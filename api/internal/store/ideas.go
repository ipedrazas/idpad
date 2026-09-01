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

// ideaColumns is the select list every single-idea query returns, kept in one
// place so a new column cannot be added to one query and forgotten in another.
const ideaColumns = `id, title, body, status, status_changed_at, created_at, updated_at`

// scanIdea reads a row shaped like ideaColumns.
func scanIdea(row pgx.Row) (model.Idea, error) {
	var idea model.Idea
	err := row.Scan(&idea.ID, &idea.Title, &idea.Body, &idea.Status,
		&idea.StatusChangedAt, &idea.CreatedAt, &idea.UpdatedAt)
	return idea, err
}

// CreateIdea inserts an idea with a freshly generated UUIDv7. Status is left
// to the column default: a new idea is a draft, and moving it on is a separate,
// deliberate act.
func (s *Store) CreateIdea(ctx context.Context, title string, body json.RawMessage) (model.Idea, error) {
	id, err := newID()
	if err != nil {
		return model.Idea{}, err
	}

	const q = `
		insert into ideas (id, title, body)
		values ($1, $2, $3)
		returning ` + ideaColumns

	idea, err := scanIdea(s.pool.QueryRow(ctx, q, id, title, []byte(body)))
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
	// $1..$4 are always bound, so an unset part of the filter is a predicate
	// that is trivially true rather than a different query string.
	const q = `
		select i.id, i.title, i.body, i.status, i.status_changed_at, i.created_at, i.updated_at,
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
		  and ($4::text[] is null or i.status = any ($4))
		order by i.updated_at desc, i.id desc`

	var tagSlugs []string
	if len(filter.TagSlugs) > 0 {
		tagSlugs = filter.TagSlugs
	}
	var excludeID *string
	if filter.ExcludeID != "" {
		excludeID = &filter.ExcludeID
	}
	var statuses []string
	for _, status := range filter.Statuses {
		statuses = append(statuses, string(status))
	}

	rows, err := s.pool.Query(ctx, q, tagSlugs, filter.Query, excludeID, statuses)
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
		if err := rows.Scan(&it.ID, &it.Title, &it.Body, &it.Status, &it.StatusChangedAt,
			&it.CreatedAt, &it.UpdatedAt,
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
		select ` + ideaColumns + `
		from ideas
		where id = $1`

	idea, err := scanIdea(s.pool.QueryRow(ctx, q, id))
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
// Status is not touched here: editing an idea is not a move along its
// lifecycle, and the editor never sends one.
func (s *Store) UpdateIdea(ctx context.Context, id, title string, body json.RawMessage) (model.Idea, error) {
	const q = `
		update ideas
		set title = $2, body = $3, updated_at = $4
		where id = $1
		returning ` + ideaColumns

	idea, err := scanIdea(s.pool.QueryRow(ctx, q, id, title, []byte(body), time.Now().UTC()))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Idea{}, ErrNotFound
	}
	if err != nil {
		return model.Idea{}, fmt.Errorf("update idea: %w", err)
	}
	return idea, nil
}

// SetIdeaStatus moves an idea along its lifecycle. updated_at is deliberately
// left alone, so it keeps meaning "the body was last edited" and a status
// change does not reshuffle the list. status_changed_at only moves when the
// status actually differs, which makes re-setting the current status a no-op
// rather than a way to refresh the timestamp.
func (s *Store) SetIdeaStatus(ctx context.Context, id string, status model.IdeaStatus) (model.Idea, error) {
	const q = `
		update ideas
		set status = $2,
		    status_changed_at = case when status = $2 then status_changed_at else $3 end
		where id = $1
		returning ` + ideaColumns

	idea, err := scanIdea(s.pool.QueryRow(ctx, q, id, string(status), time.Now().UTC()))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Idea{}, ErrNotFound
	}
	if err != nil {
		return model.Idea{}, fmt.Errorf("set idea status: %w", err)
	}
	return idea, nil
}

// CountIdeasByStatus returns every status with how many ideas are in it,
// in lifecycle order. The statuses drive the query rather than the rows, so an
// empty state still comes back with a count of zero and its filter chip neither
// appears nor disappears as ideas move.
func (s *Store) CountIdeasByStatus(ctx context.Context) ([]model.IdeaStatusSummary, error) {
	const q = `
		select s.status, (select count(*) from ideas i where i.status = s.status) as idea_count
		from unnest($1::text[]) with ordinality as s (status, position)
		order by s.position`

	known := make([]string, 0, len(model.IdeaStatuses))
	for _, status := range model.IdeaStatuses {
		known = append(known, string(status))
	}

	rows, err := s.pool.Query(ctx, q, known)
	if err != nil {
		return nil, fmt.Errorf("count ideas by status: %w", err)
	}
	defer rows.Close()

	counts := make([]model.IdeaStatusSummary, 0, len(known))
	for rows.Next() {
		var c model.IdeaStatusSummary
		if err := rows.Scan(&c.Status, &c.IdeaCount); err != nil {
			return nil, fmt.Errorf("scan status count: %w", err)
		}
		counts = append(counts, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("count ideas by status: %w", err)
	}
	return counts, nil
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
