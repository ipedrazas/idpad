import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api, type CreateLinkInput } from '../api/client'
import type { Link } from '../api/types'
import { queryKeys } from './queryKeys'

/** Every link touching the idea, in both directions. */
export function useLinks(ideaId: string | undefined) {
  return useQuery<Link[]>({
    queryKey: queryKeys.links(ideaId ?? ''),
    queryFn: () => api.listLinks(ideaId as string),
    enabled: Boolean(ideaId),
  })
}

export function useCreateLink(ideaId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (input: CreateLinkInput) => api.createLink(ideaId, input),
    onSuccess: (link) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.links(ideaId) })
      // A link exists for both ideas, so the other end's cached list is stale too.
      void queryClient.invalidateQueries({ queryKey: queryKeys.links(link.other_idea_id) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.ideas })
    },
  })
}

export function useDeleteLink(ideaId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => api.deleteLink(id),
    onMutate: async (id) => {
      const key = queryKeys.links(ideaId)
      await queryClient.cancelQueries({ queryKey: key })
      const previous = queryClient.getQueryData<Link[]>(key)
      queryClient.setQueryData<Link[]>(key, (links) => (links ?? []).filter((l) => l.id !== id))
      // Returned so the row can be put back if the delete fails, and so the
      // other end of the link can be invalidated once it settles.
      return { previous, removed: previous?.find((l) => l.id === id) }
    },
    onError: (_error, _id, context) => {
      if (context?.previous) queryClient.setQueryData(queryKeys.links(ideaId), context.previous)
    },
    onSettled: (_data, _error, _id, context) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.links(ideaId) })
      if (context?.removed) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.links(context.removed.other_idea_id) })
      }
      void queryClient.invalidateQueries({ queryKey: queryKeys.ideas })
    },
  })
}
