package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/ipedrazas/idpad/api/internal/model"
)

// ListTags returns the whole tag vocabulary with how many ideas carry each
// one, most used first. Tags with no ideas are included: they are reachable
// again the moment someone types the name.
func (s *Store) ListTags(ctx context.Context) ([]model.TagSummary, error) {
	const q = `
		select t.id, t.name, t.slug, t.created_at,
		       (select count(*) from idea_tags it where it.tag_id = t.id) as idea_count
		from tags t
		order by idea_count desc, t.slug asc`

	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	tags := make([]model.TagSummary, 0)
	for rows.Next() {
		var t model.TagSummary
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.CreatedAt, &t.IdeaCount); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	return tags, nil
}

// ListTagsForIdea returns one idea's tags in slug order, which is stable and
// so keeps the chips from reshuffling between renders.
func (s *Store) ListTagsForIdea(ctx context.Context, ideaID string) ([]model.Tag, error) {
	const q = `
		select t.id, t.name, t.slug, t.created_at
		from tags t
		join idea_tags it on it.tag_id = t.id
		where it.idea_id = $1
		order by t.slug asc`

	rows, err := s.pool.Query(ctx, q, ideaID)
	if err != nil {
		return nil, fmt.Errorf("list idea tags: %w", err)
	}
	defer rows.Close()

	return collectTags(rows)
}

// SetIdeaTags replaces an idea's tag set with names, creating any tag the
// vocabulary does not have yet. The whole replacement runs in one transaction
// so a failure part-way cannot leave the idea holding half its new tags.
//
// names are display names; their slugs decide identity, so a name that folds
// onto an existing tag reuses that tag rather than creating a near-duplicate.
func (s *Store) SetIdeaTags(ctx context.Context, ideaID string, names []string) ([]model.Tag, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin set idea tags: %w", err)
	}
	// Rollback after a successful commit is a no-op, so the deferred call is
	// safe and covers every early return.
	defer func() { _ = tx.Rollback(ctx) }()

	tagIDs := make([]string, 0, len(names))
	for _, name := range names {
		id, err := upsertTag(ctx, tx, name)
		if err != nil {
			return nil, err
		}
		tagIDs = append(tagIDs, id)
	}

	// Deleting only what is no longer wanted preserves the created_at of the
	// tags that stayed, so "tagged since" survives an unrelated edit.
	if _, err := tx.Exec(ctx,
		`delete from idea_tags where idea_id = $1 and tag_id <> all ($2)`,
		ideaID, tagIDs,
	); err != nil {
		return nil, fmt.Errorf("prune idea tags: %w", err)
	}

	for _, tagID := range tagIDs {
		if _, err := tx.Exec(ctx,
			`insert into idea_tags (idea_id, tag_id) values ($1, $2) on conflict do nothing`,
			ideaID, tagID,
		); err != nil {
			return nil, fmt.Errorf("attach idea tag: %w", err)
		}
	}

	const q = `
		select t.id, t.name, t.slug, t.created_at
		from tags t
		join idea_tags it on it.tag_id = t.id
		where it.idea_id = $1
		order by t.slug asc`

	rows, err := tx.Query(ctx, q, ideaID)
	if err != nil {
		return nil, fmt.Errorf("read back idea tags: %w", err)
	}
	tags, err := collectTags(rows)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit set idea tags: %w", err)
	}
	return tags, nil
}

// upsertTag returns the id of the tag named name, inserting it if the slug is
// new. The on-conflict clause makes concurrent writers converge on one row
// instead of one of them failing.
func upsertTag(ctx context.Context, tx pgx.Tx, name string) (string, error) {
	slug := model.Slugify(name)
	display := model.NormaliseTagName(name)

	id, err := newID()
	if err != nil {
		return "", err
	}

	// `do update set slug = excluded.slug` is a no-op write whose only job is
	// to make the row visible to RETURNING; `do nothing` would return no row.
	const q = `
		insert into tags (id, name, slug)
		values ($1, $2, $3)
		on conflict (slug) do update set slug = excluded.slug
		returning id`

	var tagID string
	if err := tx.QueryRow(ctx, q, id, display, slug).Scan(&tagID); err != nil {
		return "", fmt.Errorf("upsert tag %q: %w", slug, err)
	}
	return tagID, nil
}

// DeleteUnusedTags removes tags no idea carries any more, and reports how many
// went. Tag rows are cheap, so this is a housekeeping call rather than
// something the write path runs.
func (s *Store) DeleteUnusedTags(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`delete from tags t where not exists (select 1 from idea_tags it where it.tag_id = t.id)`)
	if err != nil {
		return 0, fmt.Errorf("delete unused tags: %w", err)
	}
	return tag.RowsAffected(), nil
}

// collectTags drains a rows cursor of the shared tag column list.
func collectTags(rows pgx.Rows) ([]model.Tag, error) {
	defer rows.Close()

	tags := make([]model.Tag, 0)
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read tags: %w", err)
	}
	return tags, nil
}
