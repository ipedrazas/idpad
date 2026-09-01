import { Extension } from '@tiptap/core'
import { Placeholder } from '@tiptap/extensions'
import { EditorContent, useEditor, useEditorState } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import { useCallback, useEffect, useRef, useState } from 'react'

import type { Anchor, Doc, Thread } from '../../api/types'
import { rangeToAnchor, truncate } from '../../lib/anchor'
import { cx } from '../../lib/format'
import { Button, Spinner } from '../ui'
import {
  THREAD_ID_ATTRIBUTE,
  commentHighlightKey,
  commentHighlightPlugin,
  readBlockSegments,
} from './commentHighlight'

/**
 * The editing schema, deliberately narrow: headings h1–h3, paragraphs, bullet
 * and numbered lists, bold and italic. Keeping the node set small keeps the
 * flattened block text — and therefore the anchor offsets — predictable.
 */
const editorExtensions = [
  StarterKit.configure({
    heading: { levels: [1, 2, 3] },
    blockquote: false,
    code: false,
    codeBlock: false,
    horizontalRule: false,
    strike: false,
    underline: false,
    link: false,
  }),
  Placeholder.configure({ placeholder: 'Start writing your idea…' }),
]

/** Vertical room the floating comment button needs above a selection. */
const FLOATING_OFFSET = 44

interface IdeaEditorProps {
  body: Doc
  threads: Thread[]
  activeThreadId: string | null
  onSelectThread: (threadId: string | null) => void
  onBodyChange: (body: Doc) => void
  onCreateComment: (anchor: Anchor, body: string) => Promise<void>
}

/** Where the floating comment affordance should sit, relative to the wrapper. */
interface FloatingPosition {
  top: number
  left: number
}

export function IdeaEditor({
  body,
  threads,
  activeThreadId,
  onSelectThread,
  onBodyChange,
  onCreateComment,
}: IdeaEditorProps) {
  const wrapperRef = useRef<HTMLDivElement>(null)

  // Selection state drives the floating "Comment" button.
  const [selection, setSelection] = useState<{ anchor: Anchor; position: FloatingPosition } | null>(null)
  // A captured anchor while the composer is open; the editor selection may be
  // gone by then because focus moved into the textarea.
  const [pending, setPending] = useState<{ anchor: Anchor; position: FloatingPosition } | null>(null)
  const [draft, setDraft] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [composerError, setComposerError] = useState<string | null>(null)

  // The plugin is built once, before any comment has loaded; the current
  // threads reach it through the meta transaction in the effect below.
  const [highlightExtension] = useState(() =>
    Extension.create({
      name: 'idpadCommentHighlight',
      addProseMirrorPlugins() {
        return [commentHighlightPlugin({ threads: [], activeThreadId: null })]
      },
    }),
  )

  // The initial document is captured once: after mount the editor owns the
  // document, and re-seeding it from a refetch would fight the typist.
  const [initialContent] = useState(body)

  const editor = useEditor({
    extensions: [...editorExtensions, highlightExtension],
    content: initialContent,
    editorProps: {
      attributes: { 'aria-label': 'Idea body' },
    },
    onUpdate: ({ editor: instance }) => {
      onBodyChange(instance.getJSON() as Doc)
    },
  })

  const toolbar = useEditorState({
    editor,
    selector: ({ editor: instance }) => ({
      h1: instance?.isActive('heading', { level: 1 }) ?? false,
      h2: instance?.isActive('heading', { level: 2 }) ?? false,
      h3: instance?.isActive('heading', { level: 3 }) ?? false,
      bold: instance?.isActive('bold') ?? false,
      italic: instance?.isActive('italic') ?? false,
      bulletList: instance?.isActive('bulletList') ?? false,
      orderedList: instance?.isActive('orderedList') ?? false,
      canUndo: instance?.can().undo() ?? false,
      canRedo: instance?.can().redo() ?? false,
    }),
  })

  // Push the current comment state into the decoration plugin. This is a
  // meta-only transaction, so it never marks the document dirty.
  useEffect(() => {
    if (!editor) return
    editor.view.dispatch(
      editor.state.tr.setMeta(commentHighlightKey, { state: { threads, activeThreadId } }),
    )
  }, [editor, threads, activeThreadId])

  // Track the selection to decide whether a comment can be started.
  useEffect(() => {
    if (!editor) return

    const update = () => {
      const { from, to, empty } = editor.state.selection
      if (empty) {
        setSelection(null)
        return
      }

      const anchor = rangeToAnchor(readBlockSegments(editor.state.doc), { from, to })
      if (!anchor) {
        // Cross-block or whitespace-only selections cannot be anchored in v1.
        setSelection(null)
        return
      }

      const wrapper = wrapperRef.current
      if (!wrapper) return
      const start = editor.view.coordsAtPos(from)
      const bounds = wrapper.getBoundingClientRect()

      // Sit above the selection by default, but drop below it when the line is
      // near the top of the editor, where the sticky toolbar would cover it.
      const above = start.top - bounds.top - FLOATING_OFFSET
      const top = above < 0 ? start.bottom - bounds.top + 8 : above

      setSelection({ anchor, position: { top, left: Math.max(start.left - bounds.left, 0) } })
    }

    editor.on('selectionUpdate', update)
    editor.on('transaction', update)
    return () => {
      editor.off('selectionUpdate', update)
      editor.off('transaction', update)
    }
  }, [editor])

  const handleHighlightClick = useCallback(
    (event: React.MouseEvent<HTMLDivElement>) => {
      const target = event.target instanceof HTMLElement ? event.target : null
      const threadId = target?.closest(`[${THREAD_ID_ATTRIBUTE}]`)?.getAttribute(THREAD_ID_ATTRIBUTE)
      if (threadId) onSelectThread(threadId)
    },
    [onSelectThread],
  )

  const closeComposer = useCallback(() => {
    setPending(null)
    setDraft('')
    setComposerError(null)
  }, [])

  const submitComment = useCallback(async () => {
    if (!pending) return
    const text = draft.trim()
    if (!text) {
      setComposerError('A comment needs some text.')
      return
    }

    setSubmitting(true)
    setComposerError(null)
    try {
      await onCreateComment(pending.anchor, text)
      closeComposer()
      setSelection(null)
      editor?.commands.focus()
    } catch (error) {
      setComposerError(error instanceof Error ? error.message : 'Could not save the comment.')
    } finally {
      setSubmitting(false)
    }
  }, [closeComposer, draft, editor, onCreateComment, pending])

  if (!editor) {
    return (
      <div className="flex h-64 items-center justify-center text-slate-400">
        <Spinner />
      </div>
    )
  }

  return (
    <div className="idpad-editor">
      <div className="sticky top-0 z-10 -mx-1 mb-3 flex flex-wrap items-center gap-1 rounded-lg border border-slate-200 bg-white/90 px-1.5 py-1.5 backdrop-blur dark:border-slate-800 dark:bg-slate-900/90">
        <ToolbarButton active={toolbar?.h1} label="Heading 1" onClick={() => editor.chain().focus().toggleHeading({ level: 1 }).run()}>
          H1
        </ToolbarButton>
        <ToolbarButton active={toolbar?.h2} label="Heading 2" onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}>
          H2
        </ToolbarButton>
        <ToolbarButton active={toolbar?.h3} label="Heading 3" onClick={() => editor.chain().focus().toggleHeading({ level: 3 }).run()}>
          H3
        </ToolbarButton>

        <Divider />

        <ToolbarButton active={toolbar?.bold} label="Bold" onClick={() => editor.chain().focus().toggleBold().run()}>
          <span className="font-bold">B</span>
        </ToolbarButton>
        <ToolbarButton active={toolbar?.italic} label="Italic" onClick={() => editor.chain().focus().toggleItalic().run()}>
          <span className="italic">I</span>
        </ToolbarButton>

        <Divider />

        <ToolbarButton active={toolbar?.bulletList} label="Bullet list" onClick={() => editor.chain().focus().toggleBulletList().run()}>
          • List
        </ToolbarButton>
        <ToolbarButton
          active={toolbar?.orderedList}
          label="Numbered list"
          onClick={() => editor.chain().focus().toggleOrderedList().run()}
        >
          1. List
        </ToolbarButton>

        <Divider />

        <ToolbarButton label="Undo" disabled={!toolbar?.canUndo} onClick={() => editor.chain().focus().undo().run()}>
          Undo
        </ToolbarButton>
        <ToolbarButton label="Redo" disabled={!toolbar?.canRedo} onClick={() => editor.chain().focus().redo().run()}>
          Redo
        </ToolbarButton>
      </div>

      {/*
        Highlights are decorations, so they are not React elements; clicks are
        caught here and matched by the thread id the decoration carries.
      */}
      <div ref={wrapperRef} className="relative" onClickCapture={handleHighlightClick}>
        <EditorContent editor={editor} className="text-slate-800 dark:text-slate-100" />

        {selection && !pending ? (
          <div
            className="absolute z-20"
            style={{ top: selection.position.top, left: selection.position.left }}
          >
            <Button
              variant="primary"
              size="sm"
              className="shadow-lg"
              onClick={() => {
                setPending(selection)
                setDraft('')
                setComposerError(null)
              }}
            >
              💬 Comment
            </Button>
          </div>
        ) : null}

        {pending ? (
          <div
            className="absolute z-30 w-80 rounded-xl border border-slate-200 bg-white p-3 shadow-xl dark:border-slate-700 dark:bg-slate-900"
            style={{ top: pending.position.top, left: pending.position.left }}
          >
            <p className="mb-2 truncate text-xs text-slate-500 dark:text-slate-400">
              on “{truncate(pending.anchor.snippet, 48)}”
            </p>
            <textarea
              autoFocus
              rows={3}
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Escape') closeComposer()
                if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) void submitComment()
              }}
              placeholder="Add a comment…"
              className="w-full resize-none rounded-lg border border-slate-300 bg-white px-2.5 py-2 text-sm outline-none focus:border-slate-500 dark:border-slate-700 dark:bg-slate-950"
            />
            {composerError ? <p className="mt-1 text-xs text-rose-600">{composerError}</p> : null}
            <div className="mt-2 flex items-center justify-end gap-2">
              <Button size="sm" variant="ghost" onClick={closeComposer}>
                Cancel
              </Button>
              <Button size="sm" variant="primary" disabled={submitting} onClick={() => void submitComment()}>
                {submitting ? <Spinner className="size-3" /> : null}
                Comment
              </Button>
            </div>
          </div>
        ) : null}
      </div>
    </div>
  )
}

interface ToolbarButtonProps {
  children: React.ReactNode
  label: string
  onClick: () => void
  active?: boolean
  disabled?: boolean
}

function ToolbarButton({ children, label, onClick, active = false, disabled = false }: ToolbarButtonProps) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      aria-pressed={active}
      disabled={disabled}
      // Keep DOM focus in the editor: without this the button takes focus on
      // mousedown, so the next keystroke lands on the button instead of the
      // document, and Enter re-triggers the command.
      onMouseDown={(event) => event.preventDefault()}
      onClick={onClick}
      className={cx(
        'rounded-md px-2 py-1 text-xs font-medium transition-colors disabled:opacity-40',
        active
          ? 'bg-slate-900 text-white dark:bg-slate-100 dark:text-slate-900'
          : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800',
      )}
    >
      {children}
    </button>
  )
}

function Divider() {
  return <span className="mx-1 h-5 w-px bg-slate-200 dark:bg-slate-700" aria-hidden="true" />
}
