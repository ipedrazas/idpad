package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ipedrazas/idpad/api/internal/model"
)

// commentColumns is the shared select list, keeping every scan in this file
// aligned with scanComment.
const commentColumns = `id, idea_id, parent_id, block_index, start_offset, end_offset,
	snippet, body, status, created_at, updated_at`

// scanComment maps a commentColumns row onto a domain comment, folding the
// four nullable anchor columns back into an optional Anchor.
func scanComment(row pgx.Row) (model.Comment, error) {
	var (
		c           model.Comment
		blockIndex  *int
		startOffset *int
		endOffset   *int
		snippet     *string
	)
	err := row.Scan(&c.ID, &c.IdeaID, &c.ParentID, &blockIndex, &startOffset, &endOffset,
		&snippet, &c.Body, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return model.Comment{}, err
	}
	if blockIndex != nil && startOffset != nil && endOffset != nil && snippet != nil {
		c.Anchor = &model.Anchor{
			BlockIndex:  *blockIndex,
			StartOffset: *startOffset,
			EndOffset:   *endOffset,
			Snippet:     *snippet,
		}
	}
	return c, nil
}

// CreateThread inserts an anchored top-level comment.
func (s *Store) CreateThread(ctx context.Context, ideaID string, anchor model.Anchor, body string) (model.Comment, error) {
	id, err := newID()
	if err != nil {
		return model.Comment{}, err
	}

	q := `
		insert into comments (id, idea_id, block_index, start_offset, end_offset, snippet, body)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning ` + commentColumns

	row := s.pool.QueryRow(ctx, q, id, ideaID,
		anchor.BlockIndex, anchor.StartOffset, anchor.EndOffset, anchor.Snippet, body)
	c, err := scanComment(row)
	if err != nil {
		return model.Comment{}, fmt.Errorf("insert thread: %w", err)
	}
	return c, nil
}

// CreateReply inserts an unanchored reply under an existing thread root.
func (s *Store) CreateReply(ctx context.Context, ideaID, parentID, body string) (model.Comment, error) {
	id, err := newID()
	if err != nil {
		return model.Comment{}, err
	}

	q := `
		insert into comments (id, idea_id, parent_id, body)
		values ($1, $2, $3, $4)
		returning ` + commentColumns

	row := s.pool.QueryRow(ctx, q, id, ideaID, parentID, body)
	c, err := scanComment(row)
	if err != nil {
		return model.Comment{}, fmt.Errorf("insert reply: %w", err)
	}
	return c, nil
}

// ListCommentsForIdea returns every comment of an idea, roots newest first
// and replies oldest first, ready to be assembled into threads.
func (s *Store) ListCommentsForIdea(ctx context.Context, ideaID string) ([]model.Comment, error) {
	q := `
		select ` + commentColumns + `
		from comments
		where idea_id = $1
		order by (parent_id is not null), created_at desc, id desc`

	rows, err := s.pool.Query(ctx, q, ideaID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	comments := make([]model.Comment, 0)
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		comments = append(comments, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	return comments, nil
}

// GetComment returns one comment, or ErrNotFound.
func (s *Store) GetComment(ctx context.Context, id string) (model.Comment, error) {
	q := `select ` + commentColumns + ` from comments where id = $1`

	c, err := scanComment(s.pool.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Comment{}, ErrNotFound
	}
	if err != nil {
		return model.Comment{}, fmt.Errorf("get comment: %w", err)
	}
	return c, nil
}

// UpdateCommentBody edits the text of a comment or reply.
func (s *Store) UpdateCommentBody(ctx context.Context, id, body string) (model.Comment, error) {
	q := `
		update comments
		set body = $2, updated_at = $3
		where id = $1
		returning ` + commentColumns

	c, err := scanComment(s.pool.QueryRow(ctx, q, id, body, time.Now().UTC()))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Comment{}, ErrNotFound
	}
	if err != nil {
		return model.Comment{}, fmt.Errorf("update comment: %w", err)
	}
	return c, nil
}

// SetCommentStatus opens or resolves a comment.
func (s *Store) SetCommentStatus(ctx context.Context, id string, status model.CommentStatus) (model.Comment, error) {
	q := `
		update comments
		set status = $2, updated_at = $3
		where id = $1
		returning ` + commentColumns

	c, err := scanComment(s.pool.QueryRow(ctx, q, id, string(status), time.Now().UTC()))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Comment{}, ErrNotFound
	}
	if err != nil {
		return model.Comment{}, fmt.Errorf("set comment status: %w", err)
	}
	return c, nil
}

// DeleteComment removes a comment; replies of a thread root cascade.
func (s *Store) DeleteComment(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `delete from comments where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
