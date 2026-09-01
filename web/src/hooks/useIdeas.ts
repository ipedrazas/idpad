import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api, type IdeaFilter, type IdeaInput } from '../api/client'
import type { Idea, IdeaSummary } from '../api/types'
import { queryKeys } from './queryKeys'

/**
 * Ideas matching the filter, newest update first, with the counts and tags the
 * cards show. An omitted filter lists everything.
 */
export function useIdeas(filter?: IdeaFilter) {
  return useQuery<IdeaSummary[]>({
    queryKey: queryKeys.ideaList(filter),
    queryFn: () => api.listIdeas(filter),
    // The previous list stays on screen while a new filter loads, so changing
    // tags does not blank the grid on every keystroke.
    placeholderData: (previous) => previous,
  })
}

/** A single idea. Disabled until an id is available. */
export function useIdea(id: string | undefined) {
  return useQuery<Idea>({
    queryKey: queryKeys.idea(id ?? ''),
    queryFn: () => api.getIdea(id as string),
    enabled: Boolean(id),
  })
}

export function useCreateIdea() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: IdeaInput) => api.createIdea(input),
    onSuccess: (idea) => {
      queryClient.setQueryData(queryKeys.idea(idea.id), idea)
      void queryClient.invalidateQueries({ queryKey: queryKeys.ideas })
    },
  })
}

export function useUpdateIdea(id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: IdeaInput) => api.updateIdea(id, input),
    onSuccess: (idea) => {
      queryClient.setQueryData(queryKeys.idea(id), idea)
      void queryClient.invalidateQueries({ queryKey: queryKeys.ideas })
      // The body may have moved under the anchors, so the detached flags the
      // server computes have to be re-read.
      void queryClient.invalidateQueries({ queryKey: queryKeys.comments(id) })
    },
  })
}

export function useDeleteIdea() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.deleteIdea(id),
    onSuccess: (_result, id) => {
      queryClient.removeQueries({ queryKey: queryKeys.idea(id) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.ideas })
    },
  })
}
