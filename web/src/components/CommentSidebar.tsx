import type { Thread } from '../api/types'
import { CommentThread } from './CommentThread'
import { EmptyState, ErrorState, Skeleton } from './ui'

export interface CommentSidebarProps {
  threads: Thread[]
  loading: boolean
  error: Error | null
  activeThreadId: string | null
  onSelectThread: (threadId: string | null) => void
  onReply: (threadId: string, body: string) => Promise<void>
  onEdit: (commentId: string, body: string) => Promise<void>
  onDelete: (commentId: string) => Promise<void>
  onToggleResolved: (thread: Thread) => Promise<void>
}

/**
 * The sticky right-hand panel. Threads keep the order the API returns them in
 * (newest first), which is also the order that assigns highlight colours.
 */
export function CommentSidebar({
  threads,
  loading,
  error,
  activeThreadId,
  onSelectThread,
  onReply,
  onEdit,
  onDelete,
  onToggleResolved,
}: CommentSidebarProps) {
  const openCount = threads.filter((thread) => thread.status === 'open').length

  return (
    <section aria-label="Comments" className="flex flex-col gap-3">
      <header className="flex items-baseline justify-between">
        <h2 className="text-sm font-semibold tracking-tight text-slate-900 dark:text-slate-100">Comments</h2>
        <span className="text-xs text-slate-500 dark:text-slate-400">
          {openCount} open · {threads.length} total
        </span>
      </header>

      {loading ? (
        <div className="space-y-3">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-20 w-full" />
        </div>
      ) : error ? (
        <ErrorState title="Could not load comments" description={error.message} />
      ) : threads.length === 0 ? (
        <EmptyState
          title="No comments yet"
          description="Select any text in the idea and click Comment to start a thread."
        />
      ) : (
        <ol className="space-y-3">
          {threads.map((thread, index) => (
            <li key={thread.id}>
              <CommentThread
                thread={thread}
                index={index}
                active={thread.id === activeThreadId}
                onActivate={() => onSelectThread(thread.id === activeThreadId ? null : thread.id)}
                onReply={(body) => onReply(thread.id, body)}
                onEdit={onEdit}
                onDelete={onDelete}
                onToggleResolved={() => onToggleResolved(thread)}
              />
            </li>
          ))}
        </ol>
      )}
    </section>
  )
}
