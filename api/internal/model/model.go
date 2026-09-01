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
// and tags the cards display so the UI needs a single request.
type IdeaSummary struct {
	Idea
	CommentCount  int   `json:"comment_count"`
	ResourceCount int   `json:"resource_count"`
	LinkCount     int   `json:"link_count"`
	Tags          []Tag `json:"tags"`
}

// IdeaFilter narrows the idea list. A zero filter lists everything.
type IdeaFilter struct {
	// TagSlugs keeps only ideas carrying every listed tag, so stacking tags
	// narrows the list rather than widening it.
	TagSlugs []string
	// Query is a case-insensitive substring match against the title.
	Query string
	// ExcludeID drops one idea from the results, which is what the link
	// picker needs so an idea is never offered a link to itself.
	ExcludeID string
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

// Tag is a label shared across ideas. Slug is the folded identity used for
// uniqueness and lookup; Name is what the user typed.
type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
}

// TagSummary is a tag as listed in the tag index, carrying how many ideas
// currently reference it.
type TagSummary struct {
	Tag
	IdeaCount int `json:"idea_count"`
}

// Relation is the kind of connection one idea has to another.
type Relation string

// Link relations. references and expands are directed and read differently
// from each end; similar and related are symmetric.
const (
	RelationReferences Relation = "references"
	RelationExpands    Relation = "expands"
	RelationSimilar    Relation = "similar"
	RelationRelated    Relation = "related"
)

// Relations lists every relation the API accepts, in the order the UI offers
// them.
var Relations = []Relation{RelationReferences, RelationExpands, RelationSimilar, RelationRelated}

// Valid reports whether r is a relation the API accepts.
func (r Relation) Valid() bool {
	for _, candidate := range Relations {
		if r == candidate {
			return true
		}
	}
	return false
}

// Symmetric reports whether the relation reads the same from both ends, in
// which case it has no distinct inverse.
func (r Relation) Symmetric() bool {
	return r == RelationSimilar || r == RelationRelated
}

// Inverse returns the relation as seen from the target idea. Symmetric
// relations are their own inverse; the directed ones name the passive side.
func (r Relation) Inverse() Relation {
	switch r {
	case RelationReferences:
		return "referenced_by"
	case RelationExpands:
		return "expanded_by"
	default:
		return r
	}
}

// LinkDirection says which end of a stored link an idea sits on: outgoing
// when it is the source, incoming when it is the target.
type LinkDirection string

// Link directions, relative to the idea being viewed.
const (
	DirectionOutgoing LinkDirection = "outgoing"
	DirectionIncoming LinkDirection = "incoming"
)

// Link is a typed connection between two ideas, rendered from the point of
// view of one of them. Relation is always the stored relation; Direction says
// whether the viewing idea is the source or the target, and Other identifies
// the idea at the far end.
type Link struct {
	ID           string        `json:"id"`
	SourceIdeaID string        `json:"source_idea_id"`
	TargetIdeaID string        `json:"target_idea_id"`
	Relation     Relation      `json:"relation"`
	Note         *string       `json:"note"`
	CreatedAt    time.Time     `json:"created_at"`
	Direction    LinkDirection `json:"direction"`
	// OtherIdeaID and OtherTitle describe the idea at the far end of the link,
	// so the UI can render a row without a second request.
	OtherIdeaID string `json:"other_idea_id"`
	OtherTitle  string `json:"other_title"`
}
