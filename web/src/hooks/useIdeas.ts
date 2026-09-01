import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api, type IdeaFilter, type IdeaInput } from '../api/client'
import type { Idea, IdeaStatus, IdeaStatusSummary, IdeaSummary } from '../api/types'
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
      // A new idea is a new draft, so the status counts have moved too.
      void queryClient.invalidateQueries({ queryKey: queryKeys.statuses })
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

/**
 * Moves an idea along its lifecycle. The idea is written straight into the
 * cache so the picker settles immediately, and both the list and the status
 * counts are refetched because a move changes what each of them shows.
 */
export function useSetIdeaStatus(id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (status: IdeaStatus) => api.setIdeaStatus(id, status),
    onSuccess: (idea) => {
      queryClient.setQueryData(queryKeys.idea(id), idea)
      void queryClient.invalidateQueries({ queryKey: queryKeys.ideas })
      void queryClient.invalidateQueries({ queryKey: queryKeys.statuses })
    },
  })
}

/** Every lifecycle state with its idea count, for the list view's filter. */
export function useIdeaStatuses() {
  return useQuery<IdeaStatusSummary[]>({
    queryKey: queryKeys.statuses,
    queryFn: () => api.listStatuses(),
  })
}

export function useDeleteIdea() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.deleteIdea(id),
    onSuccess: (_result, id) => {
      queryClient.removeQueries({ queryKey: queryKeys.idea(id) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.ideas })
      void queryClient.invalidateQueries({ queryKey: queryKeys.statuses })
    },
  })
}
