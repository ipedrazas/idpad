/**
 * A ProseMirror plugin that paints comment highlights as decorations.
 *
 * Decorations never touch the document: the TipTap JSON stored in Postgres is
 * exactly what the user typed, and highlights are recomputed from the comment
 * list on every relevant change. That is what lets an anchor detach cleanly
 * when its text is edited away — there is no orphan mark left behind.
 */

import { Plugin, PluginKey } from '@tiptap/pm/state'
import type { Node as PMNode } from '@tiptap/pm/model'
import { Decoration, DecorationSet } from '@tiptap/pm/view'

import type { BlockSegments, DocRange } from '../../lib/anchor'
import { anchorToRange, highlightColorAt, truncate } from '../../lib/anchor'
import type { Thread } from '../../api/types'

/** Identifies the plugin so the editor can push new comments into it. */
export const commentHighlightKey = new PluginKey<DecorationSet>('idpad-comment-highlight')

/** The dataset attribute a highlight carries, read back on click. */
export const THREAD_ID_ATTRIBUTE = 'data-thread-id'

/** Tailwind classes per palette colour, written out so the JIT compiler sees them. */
const COLOR_CLASSES: Record<string, string> = {
  amber: 'bg-amber-200/70 dark:bg-amber-500/30 border-b-2 border-amber-400',
  sky: 'bg-sky-200/70 dark:bg-sky-500/30 border-b-2 border-sky-400',
  violet: 'bg-violet-200/70 dark:bg-violet-500/30 border-b-2 border-violet-400',
  emerald: 'bg-emerald-200/70 dark:bg-emerald-500/30 border-b-2 border-emerald-400',
  rose: 'bg-rose-200/70 dark:bg-rose-500/30 border-b-2 border-rose-400',
  teal: 'bg-teal-200/70 dark:bg-teal-500/30 border-b-2 border-teal-400',
}

const ACTIVE_CLASSES = 'ring-2 ring-slate-900/40 dark:ring-white/40 rounded-sm'

/**
 * Reads the live document into the plain structure the pure anchor module
 * works with: one entry per top-level block, each listing its text runs with
 * the ProseMirror position they start at.
 */
export function readBlockSegments(doc: PMNode): BlockSegments[] {
  const blocks: BlockSegments[] = []

  doc.forEach((block, offset) => {
    const segments: BlockSegments['segments'] = []
    block.descendants((child, pos) => {
      if (child.isText) {
        // `pos` is relative to the start of the block's content, which begins
        // one position after the block node itself.
        segments.push({ from: offset + 1 + pos, text: child.text ?? '' })
      }
      return true
    })
    blocks.push({ from: offset, to: offset + block.nodeSize, segments })
  })

  return blocks
}

/** The state the plugin needs from the application on every render. */
export interface HighlightState {
  threads: Thread[]
  activeThreadId: string | null
}

/** A highlight ready to be drawn. */
interface ResolvedHighlight extends DocRange {
  threadId: string
  color: string
  tooltip: string
}

/**
 * Resolves each open thread's anchor against the current document. Resolved
 * and detached threads are skipped: neither has a highlight to draw.
 */
export function resolveHighlights(doc: PMNode, state: HighlightState): ResolvedHighlight[] {
  const blocks = readBlockSegments(doc)
  const highlights: ResolvedHighlight[] = []

  // Colours are assigned by thread order so a thread keeps its colour as
  // others come and go around it.
  state.threads.forEach((thread, index) => {
    if (thread.status === 'resolved') return

    const range = anchorToRange(blocks, thread.anchor)
    if (!range) return

    highlights.push({
      ...range,
      threadId: thread.id,
      color: highlightColorAt(index),
      tooltip: truncate(thread.body, 120),
    })
  })

  return highlights
}

/** Builds the decoration set for the current document and comment state. */
function buildDecorations(doc: PMNode, state: HighlightState): DecorationSet {
  const decorations = resolveHighlights(doc, state).map((highlight) =>
    Decoration.inline(highlight.from, highlight.to, {
      class: [
        'idpad-comment-highlight cursor-pointer transition-colors',
        COLOR_CLASSES[highlight.color] ?? COLOR_CLASSES.amber,
        highlight.threadId === state.activeThreadId ? ACTIVE_CLASSES : '',
      ]
        .filter(Boolean)
        .join(' '),
      [THREAD_ID_ATTRIBUTE]: highlight.threadId,
      title: highlight.tooltip,
    }),
  )

  return DecorationSet.create(doc, decorations)
}

/** The transaction meta key used to push fresh comment state into the plugin. */
interface HighlightMeta {
  state: HighlightState
}

/**
 * Creates the plugin. Highlights carry their thread id as a DOM attribute, so
 * clicks are handled in React on the container rather than through a callback
 * captured at plugin construction time.
 */
export function commentHighlightPlugin(initial: HighlightState) {
  let current = initial

  return new Plugin<DecorationSet>({
    key: commentHighlightKey,

    state: {
      init: (_config, editorState) => buildDecorations(editorState.doc, current),

      apply(tr, decorations, _oldState, newState) {
        const meta = tr.getMeta(commentHighlightKey) as HighlightMeta | undefined
        if (meta) {
          current = meta.state
          return buildDecorations(newState.doc, current)
        }
        if (tr.docChanged) {
          // The document moved under the anchors, so they are re-resolved
          // rather than mapped: an edit inside an anchor should detach it.
          return buildDecorations(newState.doc, current)
        }
        return decorations
      },
    },

    props: {
      decorations: (editorState) => commentHighlightKey.getState(editorState),
    },
  })
}
