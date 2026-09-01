/**
 * Anchor math: the pure logic that maps between a comment's stored anchor
 * (block index + character offsets) and ProseMirror document positions.
 *
 * Nothing here imports the editor, so every rule is unit-testable on plain
 * data. The editor-facing plugin supplies `BlockSegments` read from the live
 * document and consumes the ranges these functions return.
 *
 * Offsets are UTF-16 code units — the unit JavaScript string indices already
 * use — which is what the Go validator measures too, so both sides agree even
 * when the text contains emoji or other astral characters.
 */

import type { Anchor, Doc, DocNode } from '../api/types'

/** A run of text inside a block, with the ProseMirror position it starts at. */
export interface TextSegment {
  /** ProseMirror position of the first character of this run. */
  from: number
  text: string
}

/** One top-level block of the document, with its text runs. */
export interface BlockSegments {
  /** ProseMirror position of the block node itself. */
  from: number
  /** ProseMirror position just past the block node. */
  to: number
  segments: TextSegment[]
}

/** A resolved ProseMirror range for a decoration. */
export interface DocRange {
  from: number
  to: number
}

/** Concatenates the text of every descendant text node, in document order. */
export function flattenNode(node: DocNode): string {
  if (node.type === 'text') return node.text ?? ''
  return (node.content ?? []).map(flattenNode).join('')
}

/**
 * Flattens a TipTap document into one plain-text string per top-level block.
 * Anchor offsets are expressed against these strings.
 */
export function flattenDoc(doc: Doc | null | undefined): string[] {
  if (!doc || doc.type !== 'doc') return []
  return (doc.content ?? []).map(flattenNode)
}

/** The flattened text of a block, rebuilt from its segments. */
export function blockText(block: BlockSegments): string {
  return block.segments.map((segment) => segment.text).join('')
}

/**
 * Reports whether an anchor still selects exactly its snippet. A false result
 * means the anchored text was edited away and the thread is detached: it stays
 * in the sidebar but renders no highlight.
 */
export function anchorMatches(blocks: string[], anchor: Anchor | null): boolean {
  if (!anchor || anchor.snippet === '') return false
  const { block_index: index, start_offset: start, end_offset: end } = anchor
  if (index < 0 || index >= blocks.length) return false
  if (start < 0 || end <= start) return false

  const text = blocks[index]
  if (text === undefined || end > text.length) return false
  return text.slice(start, end) === anchor.snippet
}

/** Convenience wrapper for checking an anchor straight against a document. */
export function anchorMatchesDoc(doc: Doc | null | undefined, anchor: Anchor | null): boolean {
  return anchorMatches(flattenDoc(doc), anchor)
}

/**
 * Maps a character offset within a block onto a ProseMirror position.
 *
 * At a boundary between two runs the offset is ambiguous; `bias` picks a side,
 * which matters only for where a decoration visually begins or ends.
 */
export function positionForOffset(block: BlockSegments, offset: number, bias: 'start' | 'end'): number | null {
  if (offset < 0) return null

  let consumed = 0
  for (const segment of block.segments) {
    const length = segment.text.length
    const withinRun = bias === 'start' ? offset < consumed + length : offset <= consumed + length
    if (withinRun) {
      return segment.from + (offset - consumed)
    }
    consumed += length
  }

  // An offset landing exactly at the end of the block resolves to its last
  // position; anything past that does not exist any more.
  if (offset === consumed) {
    const last = block.segments.at(-1)
    return last ? last.from + last.text.length : block.from + 1
  }
  return null
}

/** Maps a ProseMirror position onto a character offset within a block. */
export function offsetForPosition(block: BlockSegments, position: number): number {
  let consumed = 0
  for (const segment of block.segments) {
    const length = segment.text.length
    if (position < segment.from) return consumed
    if (position <= segment.from + length) return consumed + (position - segment.from)
    consumed += length
  }
  return consumed
}

/**
 * Resolves a stored anchor to the ProseMirror range it highlights, or null when
 * the anchored text no longer matches and the thread should render no highlight.
 */
export function anchorToRange(blocks: BlockSegments[], anchor: Anchor | null): DocRange | null {
  if (!anchor) return null

  const block = blocks[anchor.block_index]
  if (!block) return null
  if (!anchorMatches([blockText(block)], { ...anchor, block_index: 0 })) return null

  const from = positionForOffset(block, anchor.start_offset, 'start')
  const to = positionForOffset(block, anchor.end_offset, 'end')
  if (from === null || to === null || to <= from) return null

  return { from, to }
}

/**
 * Turns a ProseMirror selection into an anchor, or null when the selection is
 * empty or spans more than one top-level block. v1 anchors within a block only;
 * refusing a cross-block selection is what keeps the offsets meaningful.
 */
export function rangeToAnchor(blocks: BlockSegments[], range: DocRange): Anchor | null {
  if (range.to <= range.from) return null

  const index = blocks.findIndex((block) => range.from >= block.from && range.to <= block.to)
  if (index === -1) return null

  const block = blocks[index]
  if (!block) return null

  const startOffset = offsetForPosition(block, range.from)
  const endOffset = offsetForPosition(block, range.to)
  if (endOffset <= startOffset) return null

  const snippet = blockText(block).slice(startOffset, endOffset)
  if (snippet.trim() === '') return null

  return {
    block_index: index,
    start_offset: startOffset,
    end_offset: endOffset,
    snippet,
  }
}

/**
 * Highlight palette. The first threads get distinct colours and the palette
 * then cycles, which is enough to tell neighbouring highlights apart without
 * inventing an unbounded set of colours.
 */
export const HIGHLIGHT_COLORS = [
  'amber',
  'sky',
  'violet',
  'emerald',
  'rose',
  'teal',
] as const

export type HighlightColor = (typeof HIGHLIGHT_COLORS)[number]

/** The colour assigned to the nth highlighted thread. */
export function highlightColorAt(index: number): HighlightColor {
  const palette = HIGHLIGHT_COLORS
  const color = palette[((index % palette.length) + palette.length) % palette.length]
  // The modulo above always lands inside the palette; the fallback satisfies
  // noUncheckedIndexedAccess without pretending the branch is reachable.
  return color ?? palette[0]
}

/** Truncates text for tooltips and card previews without cutting mid-word. */
export function truncate(text: string, max: number): string {
  if (text.length <= max) return text
  const clipped = text.slice(0, max)
  const lastSpace = clipped.lastIndexOf(' ')
  const base = lastSpace > max * 0.6 ? clipped.slice(0, lastSpace) : clipped
  return `${base.trimEnd()}…`
}
