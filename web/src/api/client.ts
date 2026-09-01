import type { Anchor, Comment, CommentStatus, Doc, Idea, IdeaSummary, Resource, ResourceType, Thread } from './types'

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

/** The typed surface of the idpad API. */
export const api = {
  listIdeas: () => request<IdeaSummary[]>('/ideas'),

  getIdea: (id: string) => request<Idea>(`/ideas/${id}`),

  createIdea: (input: IdeaInput) => request<Idea>('/ideas', { method: 'POST', ...json(input) }),

  updateIdea: (id: string, input: IdeaInput) =>
    request<Idea>(`/ideas/${id}`, { method: 'PUT', ...json(input) }),

  deleteIdea: (id: string) => request<void>(`/ideas/${id}`, { method: 'DELETE' }),

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
}
