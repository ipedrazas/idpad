import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'

import type { Anchor, Doc, Thread } from '../api/types'
import { CommentSidebar } from '../components/CommentSidebar'
import { IdeaEditor } from '../components/editor/IdeaEditor'
import { LinkPanel } from '../components/LinkPanel'
import { ResourcePanel } from '../components/ResourcePanel'
import { TagEditor } from '../components/TagEditor'
import { Button, ErrorState, Panel, Skeleton, Spinner } from '../components/ui'
import {
  useCreateComment,
  useComments,
  useDeleteComment,
  useSetCommentStatus,
  useUpdateComment,
} from '../hooks/useComments'
import { useDeleteIdea, useIdea, useIdeas, useUpdateIdea } from '../hooks/useIdeas'
import { useCreateLink, useDeleteLink, useLinks } from '../hooks/useLinks'
import { useCreateResource, useDeleteResource, useResources } from '../hooks/useResources'
import { useIdeaTags, useSetIdeaTags, useTags } from '../hooks/useTags'
import { anchorMatchesDoc } from '../lib/anchor'
import { absoluteTime, relativeTime } from '../lib/format'

/** How long after the last keystroke the idea is saved. */
const AUTOSAVE_DELAY_MS = 1_200

type SaveState = 'idle' | 'dirty' | 'saving' | 'saved' | 'error'

export function IdeaDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const ideaQuery = useIdea(id)
  const commentsQuery = useComments(id)
  const resourcesQuery = useResources(id)
  const tagsQuery = useIdeaTags(id)
  const vocabularyQuery = useTags()
  const linksQuery = useLinks(id)

  const ideaId = id ?? ''
  const updateIdea = useUpdateIdea(ideaId)
  const deleteIdea = useDeleteIdea()
  const createComment = useCreateComment(ideaId)
  const updateComment = useUpdateComment(ideaId)
  const deleteComment = useDeleteComment(ideaId)
  const setCommentStatus = useSetCommentStatus(ideaId)
  const createResource = useCreateResource(ideaId)
  const deleteResource = useDeleteResource(ideaId)
  const setIdeaTags = useSetIdeaTags(ideaId)
  const createLink = useCreateLink(ideaId)
  const deleteLink = useDeleteLink(ideaId)

  // Local copies of the fields the user edits. They are seeded once from the
  // server so a background refetch never overwrites in-flight typing.
  const [title, setTitle] = useState('')
  const [body, setBody] = useState<Doc | null>(null)
  const [saveState, setSaveState] = useState<SaveState>('idle')
  const [saveError, setSaveError] = useState<string | null>(null)
  const [activeThreadId, setActiveThreadId] = useState<string | null>(null)
  const [confirmingDelete, setConfirmingDelete] = useState(false)
  const [linkSearch, setLinkSearch] = useState('')

  // Candidates for a new link: every other idea, narrowed by the search box.
  // Excluding this idea server-side is what keeps a self-link unofferable.
  const candidatesQuery = useIdeas({ q: linkSearch, exclude: ideaId })

  // Local copies are seeded once per mount, so a background refetch never
  // overwrites in-flight typing. The route keys this component by idea id, so
  // following a connection remounts it and re-seeds from the new idea.
  const seeded = useRef(false)
  const idea = ideaQuery.data

  useEffect(() => {
    if (!idea || seeded.current) return
    seeded.current = true
    setTitle(idea.title)
    setBody(idea.body)
  }, [idea])

  // The last state successfully persisted, used to tell dirty from clean.
  const lastSaved = useRef<{ title: string; body: string } | null>(null)
  useEffect(() => {
    if (idea && lastSaved.current === null) {
      lastSaved.current = { title: idea.title, body: JSON.stringify(idea.body) }
    }
  }, [idea])

  const save = useCallback(
    async (nextTitle: string, nextBody: Doc) => {
      setSaveState('saving')
      setSaveError(null)
      try {
        await updateIdea.mutateAsync({ title: nextTitle, body: nextBody })
        lastSaved.current = { title: nextTitle, body: JSON.stringify(nextBody) }
        setSaveState('saved')
      } catch (cause) {
        setSaveState('error')
        setSaveError(cause instanceof Error ? cause.message : 'Could not save.')
      }
    },
    [updateIdea],
  )

  // Debounced autosave. Every edit restarts the timer, so a burst of typing
  // produces one write.
  useEffect(() => {
    if (!body || !lastSaved.current) return

    const serialisedBody = JSON.stringify(body)
    const trimmedTitle = title.trim()
    const unchanged = lastSaved.current.title === trimmedTitle && lastSaved.current.body === serialisedBody
    if (unchanged || trimmedTitle === '') return

    setSaveState('dirty')
    const timer = setTimeout(() => void save(trimmedTitle, body), AUTOSAVE_DELAY_MS)
    return () => clearTimeout(timer)
  }, [body, title, save])

  const threads: Thread[] = useMemo(() => {
    const loaded = commentsQuery.data ?? []
    if (!body) return loaded
    // Detachment is recomputed against the document in the editor so the note
    // appears the moment the anchored text is edited, not after the save.
    return loaded.map((thread) => ({ ...thread, detached: !anchorMatchesDoc(body, thread.anchor) }))
  }, [commentsQuery.data, body])

  const handleCreateComment = useCallback(
    async (anchor: Anchor, commentBody: string) => {
      // A thread must anchor to text the server can see, so any pending body
      // edit is flushed first.
      if (body && lastSaved.current && lastSaved.current.body !== JSON.stringify(body)) {
        await save(title.trim() || 'Untitled', body)
      }
      await createComment.mutateAsync({ anchor, body: commentBody })
    },
    [body, createComment, save, title],
  )

  if (!id) {
    return <ErrorState className="m-8" title="Missing idea id" />
  }

  if (ideaQuery.isPending) {
    return (
      <div className="mx-auto w-full max-w-7xl px-4 py-8">
        <Skeleton className="h-10 w-2/3" />
        <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,1fr)_22rem]">
          <Skeleton className="h-96" />
          <Skeleton className="h-96" />
        </div>
      </div>
    )
  }

  if (ideaQuery.error) {
    return (
      <div className="mx-auto w-full max-w-2xl px-4 py-16">
        <ErrorState
          title="Could not load this idea"
          description={ideaQuery.error.message}
          action={
            <Link to="/">
              <Button>Back to all ideas</Button>
            </Link>
          }
        />
      </div>
    )
  }

  return (
    <div className="mx-auto w-full max-w-7xl px-4 py-8">
      <div className="mb-4 flex items-center gap-3">
        <Link to="/" className="text-sm text-slate-500 hover:text-slate-900 dark:hover:text-slate-100">
          ← All ideas
        </Link>
        <SaveIndicator state={saveState} error={saveError} updatedAt={ideaQuery.data.updated_at} />

        <div className="ml-auto flex items-center gap-2">
          {confirmingDelete ? (
            <>
              <span className="text-sm text-slate-500">Delete this idea and everything on it?</span>
              <Button
                variant="danger"
                size="sm"
                disabled={deleteIdea.isPending}
                onClick={() => {
                  deleteIdea.mutate(id, { onSuccess: () => void navigate('/') })
                }}
              >
                {deleteIdea.isPending ? <Spinner className="size-3" /> : null}
                Delete
              </Button>
              <Button variant="ghost" size="sm" onClick={() => setConfirmingDelete(false)}>
                Cancel
              </Button>
            </>
          ) : (
            <Button variant="ghost" size="sm" onClick={() => setConfirmingDelete(true)}>
              Delete idea
            </Button>
          )}
        </div>
      </div>

      <input
        value={title}
        maxLength={200}
        aria-label="Idea title"
        onChange={(event) => setTitle(event.target.value)}
        className="w-full border-none bg-transparent text-3xl font-semibold tracking-tight outline-none placeholder:text-slate-300"
        placeholder="Untitled idea"
      />

      <div className="mt-4 max-w-2xl">
        <TagEditor
          tags={tagsQuery.data ?? []}
          vocabulary={vocabularyQuery.data}
          loading={tagsQuery.isPending}
          error={tagsQuery.error}
          onChange={async (names) => {
            await setIdeaTags.mutateAsync(names)
          }}
        />
      </div>

      <div className="mt-6 grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_22rem]">
        <Panel className="p-6">
          {body ? (
            <IdeaEditor
              body={body}
              threads={threads}
              activeThreadId={activeThreadId}
              onSelectThread={setActiveThreadId}
              onBodyChange={setBody}
              onCreateComment={handleCreateComment}
            />
          ) : (
            <Skeleton className="h-96" />
          )}
        </Panel>

        <aside className="flex flex-col gap-6 lg:sticky lg:top-6">
          <CommentSidebar
            threads={threads}
            loading={commentsQuery.isPending}
            error={commentsQuery.error}
            activeThreadId={activeThreadId}
            onSelectThread={setActiveThreadId}
            onReply={async (threadId, replyBody) => {
              await createComment.mutateAsync({ parent_id: threadId, body: replyBody })
            }}
            onEdit={async (commentId, commentBody) => {
              await updateComment.mutateAsync({ id: commentId, body: commentBody })
            }}
            onDelete={async (commentId) => {
              await deleteComment.mutateAsync(commentId)
              setActiveThreadId((current) => (current === commentId ? null : current))
            }}
            onToggleResolved={async (thread) => {
              await setCommentStatus.mutateAsync({
                id: thread.id,
                status: thread.status === 'open' ? 'resolved' : 'open',
              })
            }}
          />

          <ResourcePanel
            resources={resourcesQuery.data ?? []}
            loading={resourcesQuery.isPending}
            error={resourcesQuery.error}
            onAdd={async (input) => {
              await createResource.mutateAsync(input)
            }}
            onRemove={async (resourceId) => {
              await deleteResource.mutateAsync(resourceId)
            }}
          />

          <LinkPanel
            links={linksQuery.data ?? []}
            loading={linksQuery.isPending}
            error={linksQuery.error}
            candidates={candidatesQuery.data ?? []}
            candidatesLoading={candidatesQuery.isPending}
            search={linkSearch}
            onSearchChange={setLinkSearch}
            onAdd={async (input) => {
              await createLink.mutateAsync(input)
            }}
            onRemove={async (linkId) => {
              await deleteLink.mutateAsync(linkId)
            }}
          />
        </aside>
      </div>
    </div>
  )
}

/** The small "Saved / Saving… / Unsaved" line next to the breadcrumb. */
function SaveIndicator({
  state,
  error,
  updatedAt,
}: {
  state: SaveState
  error: string | null
  updatedAt: string
}) {
  if (state === 'saving') {
    return (
      <span className="flex items-center gap-1.5 text-xs text-slate-400">
        <Spinner className="size-3" /> Saving…
      </span>
    )
  }
  if (state === 'dirty') {
    return <span className="text-xs text-amber-600 dark:text-amber-400">Unsaved changes</span>
  }
  if (state === 'error') {
    return <span className="text-xs text-rose-600 dark:text-rose-400">{error ?? 'Save failed'}</span>
  }
  return (
    <span className="text-xs text-slate-400" title={absoluteTime(updatedAt)}>
      {state === 'saved' ? 'Saved' : `Edited ${relativeTime(updatedAt)}`}
    </span>
  )
}
