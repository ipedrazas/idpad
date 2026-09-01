// Package model holds the domain types shared by the store and HTTP layers,
// together with the pure logic for comment anchoring.
package model

import (
	"encoding/json"
	"time"
)

// CommentStatus is the lifecycle state of a comment thread.
type CommentStatus string

// Comment thread states.
const (
	StatusOpen     CommentStatus = "open"
	StatusResolved CommentStatus = "resolved"
)

// Valid reports whether s is a status the API accepts.
func (s CommentStatus) Valid() bool {
	return s == StatusOpen || s == StatusResolved
}

// ResourceType distinguishes the two kinds of attachment an idea can carry.
type ResourceType string

// Resource kinds.
const (
	ResourceImage ResourceType = "image"
	ResourceLink  ResourceType = "link"
)

// Valid reports whether t is a resource type the API accepts.
func (t ResourceType) Valid() bool {
	return t == ResourceImage || t == ResourceLink
}

// Idea is a note with a rich-text body. Body holds the TipTap/ProseMirror
// document JSON verbatim so no mark or node is ever lost in a round-trip.
type Idea struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Body      json.RawMessage `json:"body"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// IdeaSummary is an idea as rendered in the list view, carrying the counts
// the cards display so the UI needs a single request.
type IdeaSummary struct {
	Idea
	CommentCount  int `json:"comment_count"`
	ResourceCount int `json:"resource_count"`
}

// Anchor ties a thread root to a range of the idea's flattened block text.
// Offsets are character (rune) offsets into BlockText of the block at
// BlockIndex; Snippet is the literal text that was selected.
type Anchor struct {
	BlockIndex  int    `json:"block_index"`
	StartOffset int    `json:"start_offset"`
	EndOffset   int    `json:"end_offset"`
	Snippet     string `json:"snippet"`
}

// Comment is a thread root (Anchor set, ParentID nil) or a reply
// (Anchor nil, ParentID set).
type Comment struct {
	ID        string        `json:"id"`
	IdeaID    string        `json:"idea_id"`
	ParentID  *string       `json:"parent_id"`
	Anchor    *Anchor       `json:"anchor"`
	Body      string        `json:"body"`
	Status    CommentStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// Thread is a root comment with its replies, as returned by the comments
// endpoint. Detached is true when the anchored text no longer matches the
// current body, in which case the UI shows the thread without a highlight.
type Thread struct {
	Comment
	Detached bool      `json:"detached"`
	Replies  []Comment `json:"replies"`
}

// Resource is an image or link attached to an idea. v1 stores URLs only.
type Resource struct {
	ID        string       `json:"id"`
	IdeaID    string       `json:"idea_id"`
	Type      ResourceType `json:"type"`
	URL       string       `json:"url"`
	Label     *string      `json:"label"`
	CreatedAt time.Time    `json:"created_at"`
}
