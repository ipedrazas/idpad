import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api } from '../api/client'
import type { Tag, TagSummary } from '../api/types'
import { queryKeys } from './queryKeys'

/** The whole tag vocabulary with usage counts, most used first. */
export function useTags() {
  return useQuery<TagSummary[]>({
    queryKey: queryKeys.tags,
    queryFn: api.listTags,
  })
}

/** One idea's tags. Disabled until an id is available. */
export function useIdeaTags(ideaId: string | undefined) {
  return useQuery<Tag[]>({
    queryKey: queryKeys.ideaTags(ideaId ?? ''),
    queryFn: () => api.listIdeaTags(ideaId as string),
    enabled: Boolean(ideaId),
  })
}

/**
 * Replaces an idea's tag set. The response is authoritative — the server folds
 * spellings onto existing tags — so it is written straight into the cache
 * rather than merged with what was sent.
 */
export function useSetIdeaTags(ideaId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (tags: string[]) => api.setIdeaTags(ideaId, tags),
    onSuccess: (tags) => {
      queryClient.setQueryData(queryKeys.ideaTags(ideaId), tags)
      // The cards show tags, and a new tag changes the vocabulary and its counts.
      void queryClient.invalidateQueries({ queryKey: queryKeys.ideas })
      void queryClient.invalidateQueries({ queryKey: queryKeys.tags })
    },
  })
}

/**
 * Sends the idea to the tagging service and merges the suggestions into its
 * tags. The response is the full resulting set, so it replaces the cache entry
 * outright rather than being merged again on the client.
 */
export function useAutoTagIdea(ideaId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: () => api.autoTagIdea(ideaId),
    onSuccess: (tags) => {
      queryClient.setQueryData(queryKeys.ideaTags(ideaId), tags)
      void queryClient.invalidateQueries({ queryKey: queryKeys.ideas })
      void queryClient.invalidateQueries({ queryKey: queryKeys.tags })
    },
  })
}
