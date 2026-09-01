import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api, type CreateResourceInput } from '../api/client'
import type { Resource } from '../api/types'
import { queryKeys } from './queryKeys'

/** An idea's attached images and links, newest first. */
export function useResources(ideaId: string | undefined) {
  return useQuery<Resource[]>({
    queryKey: queryKeys.resources(ideaId ?? ''),
    queryFn: () => api.listResources(ideaId as string),
    enabled: Boolean(ideaId),
  })
}

export function useCreateResource(ideaId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (input: CreateResourceInput) => api.createResource(ideaId, input),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.resources(ideaId) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.ideas })
    },
  })
}

export function useDeleteResource(ideaId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => api.deleteResource(id),
    onMutate: async (id) => {
      const key = queryKeys.resources(ideaId)
      await queryClient.cancelQueries({ queryKey: key })
      const previous = queryClient.getQueryData<Resource[]>(key)
      queryClient.setQueryData<Resource[]>(key, (resources) => (resources ?? []).filter((r) => r.id !== id))
      return previous
    },
    onError: (_error, _id, previous) => {
      if (previous) queryClient.setQueryData(queryKeys.resources(ideaId), previous)
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.resources(ideaId) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.ideas })
    },
  })
}
