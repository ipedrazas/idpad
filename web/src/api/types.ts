/** Wire types mirroring the Go domain model. Keys are snake_case, as the API sends them. */

/** A TipTap/ProseMirror node. The API stores the document verbatim. */
export interface DocNode {
  type: string
  text?: string
  attrs?: Record<string, unknown>
  marks?: { type: string; attrs?: Record<string, unknown> }[]
  content?: DocNode[]
}

/** The root of a TipTap document. */
export interface Doc extends DocNode {
  type: 'doc'
  content?: DocNode[]
}

export interface Idea {
  id: string
  title: string
  body: Doc
  created_at: string
  updated_at: string
}

export interface IdeaSummary extends Idea {
  comment_count: number
  resource_count: number
  link_count: number
  tags: Tag[]
}

/**
 * A label shared across ideas. `slug` is the folded identity the API matches
 * on, so "Machine Learning" and "machine-learning" are the same tag; `name` is
 * the spelling to display.
 */
export interface Tag {
  id: string
  name: string
  slug: string
  created_at: string
}

/** A tag in the global index, with how many ideas currently carry it. */
export interface TagSummary extends Tag {
  idea_count: number
}

/** The relations one idea can declare about another, as stored. */
export type Relation = 'references' | 'expands' | 'similar' | 'related'

/** Which end of a stored link the idea being viewed sits on. */
export type LinkDirection = 'outgoing' | 'incoming'

/**
 * A typed connection between two ideas, as seen from one of them. `relation`
 * is always the stored relation; `direction` says whether this idea is the
 * source or the target, and `other_*` describes the idea at the far end.
 */
export interface Link {
  id: string
  source_idea_id: string
  target_idea_id: string
  relation: Relation
  note: string | null
  created_at: string
  direction: LinkDirection
  other_idea_id: string
  other_title: string
}

/**
 * Where a thread is attached: a character range in the flattened text of one
 * top-level block. Offsets are UTF-16 code units, the unit JS string indices
 * already use, so the browser and the Go validator agree exactly.
 */
export interface Anchor {
  block_index: number
  start_offset: number
  end_offset: number
  snippet: string
}

export type CommentStatus = 'open' | 'resolved'

export interface Comment {
  id: string
  idea_id: string
  parent_id: string | null
  anchor: Anchor | null
  body: string
  status: CommentStatus
  created_at: string
  updated_at: string
}

/** A thread root with its replies, plus whether its anchored text still exists. */
export interface Thread extends Comment {
  detached: boolean
  replies: Comment[]
}

export type ResourceType = 'image' | 'link'

export interface Resource {
  id: string
  idea_id: string
  type: ResourceType
  url: string
  label: string | null
  created_at: string
}
