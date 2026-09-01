package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ipedrazas/idpad/api/internal/model"
)

// CreateIdea inserts an idea with a freshly generated UUIDv7.
func (s *Store) CreateIdea(ctx context.Context, title string, body json.RawMessage) (model.Idea, error) {
	id, err := newID()
	if err != nil {
		return model.Idea{}, err
	}

	const q = `
		insert into ideas (id, title, body)
		values ($1, $2, $3)
		returning id, title, body, created_at, updated_at`

	var idea model.Idea
	err = s.pool.QueryRow(ctx, q, id, title, []byte(body)).
		Scan(&idea.ID, &idea.Title, &idea.Body, &idea.CreatedAt, &idea.UpdatedAt)
	if err != nil {
		return model.Idea{}, fmt.Errorf("insert idea: %w", err)
	}
	return idea, nil
}

// ListIdeas returns the ideas matching filter, newest update first, with the
// counts and tags the list view renders. Everything the cards show comes from
// correlated subqueries so a single round trip feeds the whole page.
//
// A zero filter lists every idea.
func (s *Store) ListIdeas(ctx context.Context, filter model.IdeaFilter) ([]model.IdeaSummary, error) {
	// $1..$3 are always bound, so an unset part of the filter is a predicate
	// that is trivially true rather than a different query string.
	const q = `
		select i.id, i.title, i.body, i.created_at, i.updated_at,
		       (select count(*) from comments c where c.idea_id = i.id)  as comment_count,
		       (select count(*) from resources r where r.idea_id = i.id) as resource_count,
		       (select count(*) from idea_links l
		         where l.source_idea_id = i.id or l.target_idea_id = i.id) as link_count,
		       coalesce((
		         select jsonb_agg(jsonb_build_object(
		                  'id', t.id, 'name', t.name, 'slug', t.slug, 'created_at', t.created_at)
		                order by t.slug)
		         from tags t
		         join idea_tags it on it.tag_id = t.id
		         where it.idea_id = i.id
		       ), '[]'::jsonb) as tags
		from ideas i
		where ($1::text[] is null or (
		        select count(distinct t.slug)
		        from tags t
		        join idea_tags it on it.tag_id = t.id
		        where it.idea_id = i.id and t.slug = any ($1)
		      ) = cardinality($1))
		  and ($2::text = '' or i.title ilike '%' || $2 || '%')
		  and ($3::uuid is null or i.id <> $3)
		order by i.updated_at desc, i.id desc`

	var tagSlugs []string
	if len(filter.TagSlugs) > 0 {
		tagSlugs = filter.TagSlugs
	}
	var excludeID *string
	if filter.ExcludeID != "" {
		excludeID = &filter.ExcludeID
	}

	rows, err := s.pool.Query(ctx, q, tagSlugs, filter.Query, excludeID)
	if err != nil {
		return nil, fmt.Errorf("list ideas: %w", err)
	}
	defer rows.Close()

	ideas := make([]model.IdeaSummary, 0)
	for rows.Next() {
		var (
			it      model.IdeaSummary
			rawTags []byte
		)
		if err := rows.Scan(&it.ID, &it.Title, &it.Body, &it.CreatedAt, &it.UpdatedAt,
			&it.CommentCount, &it.ResourceCount, &it.LinkCount, &rawTags); err != nil {
			return nil, fmt.Errorf("scan idea: %w", err)
		}
		if err := json.Unmarshal(rawTags, &it.Tags); err != nil {
			return nil, fmt.Errorf("decode idea tags: %w", err)
		}
		ideas = append(ideas, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list ideas: %w", err)
	}
	return ideas, nil
}

// GetIdea returns one idea, or ErrNotFound.
func (s *Store) GetIdea(ctx context.Context, id string) (model.Idea, error) {
	const q = `
		select id, title, body, created_at, updated_at
		from ideas
		where id = $1`

	var idea model.Idea
	err := s.pool.QueryRow(ctx, q, id).
		Scan(&idea.ID, &idea.Title, &idea.Body, &idea.CreatedAt, &idea.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Idea{}, ErrNotFound
	}
	if err != nil {
		return model.Idea{}, fmt.Errorf("get idea: %w", err)
	}
	return idea, nil
}

// UpdateIdea replaces the title and body. updated_at is set by the
// application rather than a trigger so the write is visible in the query.
func (s *Store) UpdateIdea(ctx context.Context, id, title string, body json.RawMessage) (model.Idea, error) {
	const q = `
		update ideas
		set title = $2, body = $3, updated_at = $4
		where id = $1
		returning id, title, body, created_at, updated_at`

	var idea model.Idea
	err := s.pool.QueryRow(ctx, q, id, title, []byte(body), time.Now().UTC()).
		Scan(&idea.ID, &idea.Title, &idea.Body, &idea.CreatedAt, &idea.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Idea{}, ErrNotFound
	}
	if err != nil {
		return model.Idea{}, fmt.Errorf("update idea: %w", err)
	}
	return idea, nil
}

// DeleteIdea removes an idea; comments and resources cascade in the database.
func (s *Store) DeleteIdea(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `delete from ideas where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete idea: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
