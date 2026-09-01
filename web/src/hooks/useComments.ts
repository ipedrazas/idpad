import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api, type CreateCommentInput } from '../api/client'
import type { Comment, CommentStatus, Thread } from '../api/types'
import { queryKeys } from './queryKeys'

/** Every thread on an idea, newest first, each with its replies. */
export function useComments(ideaId: string | undefined) {
  return useQuery<Thread[]>({
    queryKey: queryKeys.comments(ideaId ?? ''),
    queryFn: () => api.listComments(ideaId as string),
    enabled: Boolean(ideaId),
  })
}

/** The shape optimistic updates operate on. */
type ThreadList = Thread[]

/**
 * Applies `update` to the cached thread list and returns the previous value so
 * the caller can roll back. Every optimistic mutation below shares this.
 */
function useOptimisticThreads(ideaId: string) {
  const queryClient = useQueryClient()
  const key = queryKeys.comments(ideaId)

  return {
    queryClient,
    key,
    async begin(update: (threads: ThreadList) => ThreadList) {
      await queryClient.cancelQueries({ queryKey: key })
      const previous = queryClient.getQueryData<ThreadList>(key)
      queryClient.setQueryData<ThreadList>(key, (threads) => update(threads ?? []))
      return previous
    },
    rollback(previous: ThreadList | undefined) {
      if (previous) queryClient.setQueryData(key, previous)
    },
    settle() {
      void queryClient.invalidateQueries({ queryKey: key })
      // Comment counts live on the idea cards.
      void queryClient.invalidateQueries({ queryKey: queryKeys.ideas })
    },
  }
}

/** Builds the placeholder shown while a create request is in flight. */
function draftComment(ideaId: string, input: CreateCommentInput): Comment {
  const now = new Date().toISOString()
  return {
    id: `optimistic-${crypto.randomUUID()}`,
    idea_id: ideaId,
    parent_id: input.parent_id ?? null,
    anchor: input.anchor ?? null,
    body: input.body,
    status: 'open',
    created_at: now,
    updated_at: now,
  }
}

/** Creates a thread root or a reply, showing it before the server confirms. */
export function useCreateComment(ideaId: string) {
  const optimistic = useOptimisticThreads(ideaId)

  return useMutation({
    mutationFn: (input: CreateCommentInput) => api.createComment(ideaId, input),

    onMutate: (input) => {
      const draft = draftComment(ideaId, input)
      return optimistic.begin((threads) => {
        if (!input.parent_id) {
          return [{ ...draft, detached: false, replies: [] }, ...threads]
        }
        return threads.map((thread) =>
          thread.id === input.parent_id ? { ...thread, replies: [...thread.replies, draft] } : thread,
        )
      })
    },

    onError: (_error, _input, previous) => optimistic.rollback(previous),
    onSettled: () => optimistic.settle(),
  })
}

/** Edits the text of a comment or reply. */
export function useUpdateComment(ideaId: string) {
  const optimistic = useOptimisticThreads(ideaId)

  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: string }) => api.updateComment(id, body),

    onMutate: ({ id, body }) =>
      optimistic.begin((threads) =>
        threads.map((thread) =>
          thread.id === id
            ? { ...thread, body }
            : { ...thread, replies: thread.replies.map((reply) => (reply.id === id ? { ...reply, body } : reply)) },
        ),
      ),

    onError: (_error, _input, previous) => optimistic.rollback(previous),
    onSettled: () => optimistic.settle(),
  })
}

/** Resolves or reopens a thread. */
export function useSetCommentStatus(ideaId: string) {
  const optimistic = useOptimisticThreads(ideaId)

  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: CommentStatus }) => api.setCommentStatus(id, status),

    onMutate: ({ id, status }) =>
      optimistic.begin((threads) => threads.map((thread) => (thread.id === id ? { ...thread, status } : thread))),

    onError: (_error, _input, previous) => optimistic.rollback(previous),
    onSettled: () => optimistic.settle(),
  })
}

/** Deletes a thread (with its replies) or a single reply. */
export function useDeleteComment(ideaId: string) {
  const optimistic = useOptimisticThreads(ideaId)

  return useMutation({
    mutationFn: (id: string) => api.deleteComment(id),

    onMutate: (id) =>
      optimistic.begin((threads) =>
        threads
          .filter((thread) => thread.id !== id)
          .map((thread) => ({ ...thread, replies: thread.replies.filter((reply) => reply.id !== id) })),
      ),

    onError: (_error, _id, previous) => optimistic.rollback(previous),
    onSettled: () => optimistic.settle(),
  })
}
