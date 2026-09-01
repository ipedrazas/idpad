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
