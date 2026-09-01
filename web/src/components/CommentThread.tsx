import { useState } from 'react'

import type { Comment, Thread } from '../api/types'
import { HIGHLIGHT_COLORS, highlightColorAt, truncate } from '../lib/anchor'
import { absoluteTime, cx, relativeTime } from '../lib/format'
import { Button, Spinner } from './ui'

/** Left-edge accent per palette colour, matching the editor highlights. */
const ACCENT_CLASSES: Record<(typeof HIGHLIGHT_COLORS)[number], string> = {
  amber: 'bg-amber-400',
  sky: 'bg-sky-400',
  violet: 'bg-violet-400',
  emerald: 'bg-emerald-400',
  rose: 'bg-rose-400',
  teal: 'bg-teal-400',
}

export interface CommentThreadProps {
  thread: Thread
  /** Position in the sidebar, which decides the highlight colour. */
  index: number
  active: boolean
  onActivate: () => void
  onReply: (body: string) => Promise<void>
  onEdit: (commentId: string, body: string) => Promise<void>
  onDelete: (commentId: string) => Promise<void>
  onToggleResolved: () => Promise<void>
}

/**
 * One comment thread in the sidebar: the anchored root, its replies, and the
 * actions available on each. Author is a placeholder until there are users.
 */
export function CommentThread({
  thread,
  index,
  active,
  onActivate,
  onReply,
  onEdit,
  onDelete,
  onToggleResolved,
}: CommentThreadProps) {
  const [replyDraft, setReplyDraft] = useState('')
  const [replying, setReplying] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const resolved = thread.status === 'resolved'
  // Only threads that are open and still anchored own a highlight colour.
  const accent = resolved || thread.detached ? null : ACCENT_CLASSES[highlightColorAt(index)]

  const run = async (action: () => Promise<void>) => {
    setBusy(true)
    setError(null)
    try {
      await action()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Something went wrong.')
    } finally {
      setBusy(false)
    }
  }

  const submitReply = async () => {
    const body = replyDraft.trim()
    if (!body) return
    await run(async () => {
      await onReply(body)
      setReplyDraft('')
      setReplying(false)
    })
  }

  return (
    <article
      onClick={onActivate}
      className={cx(
        'relative cursor-pointer overflow-hidden rounded-xl border bg-white p-3 pl-4 transition-all dark:bg-slate-900',
        active
          ? 'border-slate-400 shadow-md dark:border-slate-500'
          : 'border-slate-200 hover:border-slate-300 dark:border-slate-800 dark:hover:border-slate-700',
        resolved && 'opacity-60',
      )}
      aria-current={active ? 'true' : undefined}
    >
      {accent ? <span className={cx('absolute inset-y-0 left-0 w-1', accent)} aria-hidden="true" /> : null}

      {thread.anchor ? (
        <p
          className={cx(
            'mb-2 border-l-2 pl-2 text-xs italic',
            thread.detached
              ? 'border-slate-300 text-slate-400 line-through dark:border-slate-700'
              : 'border-slate-300 text-slate-500 dark:border-slate-700 dark:text-slate-400',
          )}
        >
          “{truncate(thread.anchor.snippet, 70)}”
        </p>
      ) : null}

      {thread.detached ? (
        <p className="mb-2 rounded-md bg-amber-50 px-2 py-1 text-xs text-amber-800 dark:bg-amber-950/50 dark:text-amber-200">
          Text changed — this comment is no longer attached to the document.
        </p>
      ) : null}

      <CommentBody
        comment={thread}
        busy={busy}
        onEdit={(body) => run(() => onEdit(thread.id, body))}
        onDelete={() => run(() => onDelete(thread.id))}
      />

      {thread.replies.length > 0 ? (
        <ul className="mt-3 space-y-3 border-l border-slate-200 pl-3 dark:border-slate-800">
          {thread.replies.map((reply) => (
            <li key={reply.id}>
              <CommentBody
                comment={reply}
                busy={busy}
                onEdit={(body) => run(() => onEdit(reply.id, body))}
                onDelete={() => run(() => onDelete(reply.id))}
              />
            </li>
          ))}
        </ul>
      ) : null}

      {error ? <p className="mt-2 text-xs text-rose-600 dark:text-rose-400">{error}</p> : null}

      <div className="mt-3 flex items-center justify-between gap-2">
        {replying ? null : (
          <Button
            size="sm"
            variant="ghost"
            onClick={(event) => {
              event.stopPropagation()
              setReplying(true)
            }}
          >
            Reply
          </Button>
        )}
        <Button
          size="sm"
          variant="ghost"
          disabled={busy}
          onClick={(event) => {
            event.stopPropagation()
            void run(onToggleResolved)
          }}
          className="ml-auto"
        >
          {resolved ? 'Reopen' : 'Resolve'}
        </Button>
      </div>

      {replying ? (
        <div className="mt-2" onClick={(event) => event.stopPropagation()}>
          <textarea
            autoFocus
            rows={2}
            value={replyDraft}
            onChange={(event) => setReplyDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Escape') setReplying(false)
              if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) void submitReply()
            }}
            placeholder="Reply…"
            className="w-full resize-none rounded-lg border border-slate-300 bg-white px-2.5 py-1.5 text-sm outline-none focus:border-slate-500 dark:border-slate-700 dark:bg-slate-950"
          />
          <div className="mt-1.5 flex justify-end gap-2">
            <Button size="sm" variant="ghost" onClick={() => setReplying(false)}>
              Cancel
            </Button>
            <Button size="sm" variant="primary" disabled={busy || !replyDraft.trim()} onClick={() => void submitReply()}>
              {busy ? <Spinner className="size-3" /> : null}
              Reply
            </Button>
          </div>
        </div>
      ) : null}
    </article>
  )
}

interface CommentBodyProps {
  comment: Comment
  busy: boolean
  onEdit: (body: string) => Promise<void>
  onDelete: () => Promise<void>
}

/** A single comment: text, metadata, and the edit/delete affordances. */
function CommentBody({ comment, busy, onEdit, onDelete }: CommentBodyProps) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(comment.body)
  const [confirmingDelete, setConfirmingDelete] = useState(false)

  const save = async () => {
    const body = draft.trim()
    if (!body || body === comment.body) {
      setEditing(false)
      return
    }
    await onEdit(body)
    setEditing(false)
  }

  return (
    <div onClick={(event) => event.stopPropagation()}>
      <div className="flex items-baseline gap-2">
        <span className="text-sm font-medium text-slate-800 dark:text-slate-100">You</span>
        <time
          className="text-xs text-slate-400"
          dateTime={comment.created_at}
          title={absoluteTime(comment.created_at)}
        >
          {relativeTime(comment.created_at)}
        </time>
      </div>

      {editing ? (
        <div className="mt-1.5">
          <textarea
            autoFocus
            rows={2}
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Escape') {
                setDraft(comment.body)
                setEditing(false)
              }
              if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) void save()
            }}
            className="w-full resize-none rounded-lg border border-slate-300 bg-white px-2.5 py-1.5 text-sm outline-none focus:border-slate-500 dark:border-slate-700 dark:bg-slate-950"
          />
          <div className="mt-1.5 flex justify-end gap-2">
            <Button
              size="sm"
              variant="ghost"
              onClick={() => {
                setDraft(comment.body)
                setEditing(false)
              }}
            >
              Cancel
            </Button>
            <Button size="sm" variant="primary" disabled={busy} onClick={() => void save()}>
              Save
            </Button>
          </div>
        </div>
      ) : (
        <p className="mt-1 text-sm whitespace-pre-wrap text-slate-700 dark:text-slate-200">{comment.body}</p>
      )}

      {!editing ? (
        <div className="mt-1.5 flex items-center gap-1 text-xs">
          <button
            type="button"
            className="text-slate-400 transition-colors hover:text-slate-700 dark:hover:text-slate-200"
            onClick={() => {
              setDraft(comment.body)
              setEditing(true)
            }}
          >
            Edit
          </button>
          <span className="text-slate-300 dark:text-slate-700">·</span>
          {confirmingDelete ? (
            <span className="flex items-center gap-1">
              <button
                type="button"
                className="font-medium text-rose-600 hover:text-rose-500"
                onClick={() => void onDelete()}
              >
                Confirm
              </button>
              <button
                type="button"
                className="text-slate-400 hover:text-slate-700 dark:hover:text-slate-200"
                onClick={() => setConfirmingDelete(false)}
              >
                Cancel
              </button>
            </span>
          ) : (
            <button
              type="button"
              className="text-slate-400 transition-colors hover:text-rose-600"
              onClick={() => setConfirmingDelete(true)}
            >
              Delete
            </button>
          )}
        </div>
      ) : null}
    </div>
  )
}
