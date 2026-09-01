import type {
  Anchor,
  Comment,
  CommentStatus,
  Doc,
  Features,
  Idea,
  IdeaStatus,
  IdeaStatusSummary,
  IdeaSummary,
  Link,
  Relation,
  Resource,
  ResourceType,
  Tag,
  TagSummary,
  Thread,
} from './types'

/**
 * Base URL for the API. Same-origin by default: vite proxies /api in dev and
 * nginx proxies it in the container, so no CORS round trip is needed either way.
 */
const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? ''

/** An error carrying the code and status from the API's error envelope. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

interface SuccessEnvelope<T> {
  data: T
}

interface ErrorEnvelope {
  error: { code: string; message: string }
}

/** True when the parsed body is the API's error envelope. */
function isErrorEnvelope(value: unknown): value is ErrorEnvelope {
  if (typeof value !== 'object' || value === null || !('error' in value)) return false
  const { error } = value as { error: unknown }
  return typeof error === 'object' && error !== null && 'message' in error
}

/**
 * Performs a request and unwraps the {"data": ...} envelope, turning any
 * non-2xx response into an ApiError carrying the server's code and message.
 */
async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response
  try {
    response = await fetch(`${BASE_URL}/api/v1${path}`, {
      ...init,
      headers: {
        ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
        ...init?.headers,
      },
    })
  } catch {
    throw new ApiError(0, 'network_error', 'Could not reach the server. Is the API running?')
  }

  if (response.status === 204) {
    return undefined as T
  }

  const text = await response.text()
  let parsed: unknown
  try {
    parsed = text ? JSON.parse(text) : null
  } catch {
    throw new ApiError(response.status, 'invalid_response', 'The server returned a malformed response.')
  }

  if (!response.ok) {
    if (isErrorEnvelope(parsed)) {
      throw new ApiError(response.status, parsed.error.code, parsed.error.message)
    }
    throw new ApiError(response.status, 'unknown_error', `Request failed with status ${response.status}.`)
  }

  return (parsed as SuccessEnvelope<T>).data
}

const json = (body: unknown): RequestInit => ({ body: JSON.stringify(body) })

export interface IdeaInput {
  title: string
  body: Doc
}

export interface CreateCommentInput {
  body: string
  /** Set for a thread root; omitted for a reply. */
  anchor?: Anchor
  /** Set for a reply; omitted for a thread root. */
  parent_id?: string
}

export interface CreateResourceInput {
  type: ResourceType
  url: string
  label?: string
}

/** Narrows the idea list. Every field is optional; an empty filter lists all. */
export interface IdeaFilter {
  /** Only ideas carrying every listed tag slug, so stacking tags narrows. */
  tags?: string[]
  /**
   * Only ideas in one of the listed states. An idea sits in exactly one, so
   * listing several widens the results where stacking tags narrows them.
   */
  statuses?: IdeaStatus[]
  /** Case-insensitive title substring. */
  q?: string
  /** Drop one idea, so the link picker never offers a link to itself. */
  exclude?: string
}

export interface CreateLinkInput {
  target_idea_id: string
  relation: Relation
  note?: string
}

/** Serialises an idea filter into the list endpoint's query string. */
function ideaQuery(filter: IdeaFilter = {}): string {
  const params = new URLSearchParams()
  for (const tag of filter.tags ?? []) params.append('tag', tag)
  for (const status of filter.statuses ?? []) params.append('status', status)
  if (filter.q) params.set('q', filter.q)
  if (filter.exclude) params.set('exclude', filter.exclude)
  const query = params.toString()
  return query ? `?${query}` : ''
}

/** The typed surface of the idpad API. */
export const api = {
  listIdeas: (filter?: IdeaFilter) => request<IdeaSummary[]>(`/ideas${ideaQuery(filter)}`),

  getIdea: (id: string) => request<Idea>(`/ideas/${id}`),

  createIdea: (input: IdeaInput) => request<Idea>('/ideas', { method: 'POST', ...json(input) }),

  updateIdea: (id: string, input: IdeaInput) =>
    request<Idea>(`/ideas/${id}`, { method: 'PUT', ...json(input) }),

  deleteIdea: (id: string) => request<void>(`/ideas/${id}`, { method: 'DELETE' }),

  /**
   * Moves an idea along its lifecycle. Its own request rather than a field on
   * updateIdea, so the editor's autosave can never reset a status it was not
   * showing, and a move never overwrites unsaved text.
   */
  setIdeaStatus: (id: string, status: IdeaStatus) =>
    request<Idea>(`/ideas/${id}/status`, { method: 'PATCH', ...json({ status }) }),

  /** Every lifecycle state with its idea count, empty states included. */
  listStatuses: () => request<IdeaStatusSummary[]>('/statuses'),

  listComments: (ideaId: string) => request<Thread[]>(`/ideas/${ideaId}/comments`),

  createComment: (ideaId: string, input: CreateCommentInput) =>
    request<Comment>(`/ideas/${ideaId}/comments`, { method: 'POST', ...json(input) }),

  updateComment: (id: string, body: string) =>
    request<Comment>(`/comments/${id}`, { method: 'PUT', ...json({ body }) }),

  deleteComment: (id: string) => request<void>(`/comments/${id}`, { method: 'DELETE' }),

  setCommentStatus: (id: string, status: CommentStatus) =>
    request<Comment>(`/comments/${id}/status`, { method: 'PATCH', ...json({ status }) }),

  listResources: (ideaId: string) => request<Resource[]>(`/ideas/${ideaId}/resources`),

  createResource: (ideaId: string, input: CreateResourceInput) =>
    request<Resource>(`/ideas/${ideaId}/resources`, { method: 'POST', ...json(input) }),

  deleteResource: (id: string) => request<void>(`/resources/${id}`, { method: 'DELETE' }),

  listTags: () => request<TagSummary[]>('/tags'),

  listIdeaTags: (ideaId: string) => request<Tag[]>(`/ideas/${ideaId}/tags`),

  /** Replaces the idea's whole tag set; an empty list clears it. */
  setIdeaTags: (ideaId: string, tags: string[]) =>
    request<Tag[]>(`/ideas/${ideaId}/tags`, { method: 'PUT', ...json({ tags }) }),

  listLinks: (ideaId: string) => request<Link[]>(`/ideas/${ideaId}/links`),

  createLink: (ideaId: string, input: CreateLinkInput) =>
    request<Link>(`/ideas/${ideaId}/links`, { method: 'POST', ...json(input) }),

  deleteLink: (id: string) => request<void>(`/links/${id}`, { method: 'DELETE' }),

  listFeatures: () => request<Features>('/features'),

  /**
   * Asks the tagging service to read the idea and merges what it suggests into
   * the idea's existing tags. Returns the resulting set, as the tag endpoints do.
   */
  autoTagIdea: (ideaId: string) =>
    request<Tag[]>(`/ideas/${ideaId}/tags/auto`, { method: 'POST' }),
}
