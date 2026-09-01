package store

import (
	"context"
	"fmt"

	"github.com/ipedrazas/idpad/api/internal/model"
)

// CreateResource attaches an image or link to an idea. Attaching a URL the
// idea already carries returns ErrConflict.
func (s *Store) CreateResource(ctx context.Context, ideaID string, kind model.ResourceType, url string, label *string) (model.Resource, error) {
	id, err := newID()
	if err != nil {
		return model.Resource{}, err
	}

	const q = `
		insert into resources (id, idea_id, type, url, label)
		values ($1, $2, $3, $4, $5)
		returning id, idea_id, type, url, label, created_at`

	var r model.Resource
	err = s.pool.QueryRow(ctx, q, id, ideaID, string(kind), url, label).
		Scan(&r.ID, &r.IdeaID, &r.Type, &r.URL, &r.Label, &r.CreatedAt)
	if isUniqueViolation(err) {
		return model.Resource{}, ErrConflict
	}
	if err != nil {
		return model.Resource{}, fmt.Errorf("insert resource: %w", err)
	}
	return r, nil
}

// ListResourcesForIdea returns an idea's resources, newest first.
func (s *Store) ListResourcesForIdea(ctx context.Context, ideaID string) ([]model.Resource, error) {
	const q = `
		select id, idea_id, type, url, label, created_at
		from resources
		where idea_id = $1
		order by created_at desc, id desc`

	rows, err := s.pool.Query(ctx, q, ideaID)
	if err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}
	defer rows.Close()

	resources := make([]model.Resource, 0)
	for rows.Next() {
		var r model.Resource
		if err := rows.Scan(&r.ID, &r.IdeaID, &r.Type, &r.URL, &r.Label, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan resource: %w", err)
		}
		resources = append(resources, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}
	return resources, nil
}

// DeleteResource removes one resource.
func (s *Store) DeleteResource(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `delete from resources where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete resource: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// IdeaExists reports whether an idea row is present, used to distinguish a
// missing parent (404) from an empty child collection (200 with []).
func (s *Store) IdeaExists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `select exists (select 1 from ideas where id = $1)`, id).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("idea exists: %w", err)
	}
	return exists, nil
}
