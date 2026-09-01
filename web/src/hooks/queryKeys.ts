import type { IdeaFilter } from '../api/client'

/** Query keys, kept in one place so invalidation never drifts from the fetch. */
export const queryKeys = {
  /**
   * The prefix every idea list shares. Invalidating it refetches the list
   * under whatever filter is currently applied.
   */
  ideas: ['ideas'] as const,
  /**
   * One filtered list. The filter is part of the key, so switching tags is a
   * separate cache entry rather than a refetch that clobbers the last one.
   */
  ideaList: (filter?: IdeaFilter) => ['ideas', 'list', normaliseFilter(filter)] as const,
  idea: (id: string) => ['ideas', id] as const,
  comments: (ideaId: string) => ['ideas', ideaId, 'comments'] as const,
  resources: (ideaId: string) => ['ideas', ideaId, 'resources'] as const,
  ideaTags: (ideaId: string) => ['ideas', ideaId, 'tags'] as const,
  links: (ideaId: string) => ['ideas', ideaId, 'links'] as const,
  tags: ['tags'] as const,
  statuses: ['statuses'] as const,
  features: ['features'] as const,
}

/**
 * Canonicalises a filter so two equivalent filters share a cache entry: tag
 * order is not meaningful, and an absent value and an empty one mean the same.
 */
function normaliseFilter(filter: IdeaFilter | undefined) {
  return {
    tags: [...(filter?.tags ?? [])].sort(),
    statuses: [...(filter?.statuses ?? [])].sort(),
    q: filter?.q ?? '',
    exclude: filter?.exclude ?? '',
  }
}
