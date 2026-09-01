package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/ipedrazas/idpad/api/internal/model"
)

// CreateLink connects two ideas. Stating a relation that already holds
// between the same pair returns ErrConflict, and a missing target surfaces as
// ErrNotFound rather than a raw foreign-key error.
func (s *Store) CreateLink(ctx context.Context, sourceID, targetID string, relation model.Relation, note *string) (model.Link, error) {
	id, err := newID()
	if err != nil {
		return model.Link{}, err
	}

	// The target's title comes back with the insert so the caller can render
	// the new row without a follow-up read.
	const q = `
		insert into idea_links (id, source_idea_id, target_idea_id, relation, note)
		values ($1, $2, $3, $4, $5)
		returning id, source_idea_id, target_idea_id, relation, note, created_at,
		          (select title from ideas where id = $3)`

	var link model.Link
	err = s.pool.QueryRow(ctx, q, id, sourceID, targetID, string(relation), note).
		Scan(&link.ID, &link.SourceIdeaID, &link.TargetIdeaID, &link.Relation,
			&link.Note, &link.CreatedAt, &link.OtherTitle)
	if isUniqueViolation(err) {
		return model.Link{}, ErrConflict
	}
	if isForeignKeyViolation(err) {
		return model.Link{}, ErrNotFound
	}
	if err != nil {
		return model.Link{}, fmt.Errorf("insert link: %w", err)
	}

	link.Direction = model.DirectionOutgoing
	link.OtherIdeaID = link.TargetIdeaID
	return link, nil
}

// ListLinksForIdea returns every link touching the idea, from that idea's
// point of view: the ones it declared (outgoing) and the ones declared at it
// (incoming). Both halves come from one query so the two directions cannot
// drift between round trips.
func (s *Store) ListLinksForIdea(ctx context.Context, ideaID string) ([]model.Link, error) {
	const q = `
		select l.id, l.source_idea_id, l.target_idea_id, l.relation, l.note, l.created_at,
		       'outgoing' as direction, l.target_idea_id as other_id, target.title as other_title
		from idea_links l
		join ideas target on target.id = l.target_idea_id
		where l.source_idea_id = $1

		union all

		select l.id, l.source_idea_id, l.target_idea_id, l.relation, l.note, l.created_at,
		       'incoming' as direction, l.source_idea_id as other_id, source.title as other_title
		from idea_links l
		join ideas source on source.id = l.source_idea_id
		where l.target_idea_id = $1

		order by direction asc, created_at desc, other_title asc`

	rows, err := s.pool.Query(ctx, q, ideaID)
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}
	defer rows.Close()

	links := make([]model.Link, 0)
	for rows.Next() {
		var l model.Link
		if err := rows.Scan(&l.ID, &l.SourceIdeaID, &l.TargetIdeaID, &l.Relation, &l.Note,
			&l.CreatedAt, &l.Direction, &l.OtherIdeaID, &l.OtherTitle); err != nil {
			return nil, fmt.Errorf("scan link: %w", err)
		}
		links = append(links, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}
	return links, nil
}

// GetLink returns one link with no direction applied, used to check ownership
// before a delete.
func (s *Store) GetLink(ctx context.Context, id string) (model.Link, error) {
	const q = `
		select id, source_idea_id, target_idea_id, relation, note, created_at
		from idea_links
		where id = $1`

	var l model.Link
	err := s.pool.QueryRow(ctx, q, id).
		Scan(&l.ID, &l.SourceIdeaID, &l.TargetIdeaID, &l.Relation, &l.Note, &l.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Link{}, ErrNotFound
	}
	if err != nil {
		return model.Link{}, fmt.Errorf("get link: %w", err)
	}
	return l, nil
}

// DeleteLink removes a link. Either end may delete it: a link is a statement
// about a pair, not a possession of the idea that typed it.
func (s *Store) DeleteLink(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `delete from idea_links where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete link: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
