import { describe, expect, it } from 'vitest'

import type { Anchor, Doc } from '../api/types'
import {
  anchorMatches,
  anchorMatchesDoc,
  anchorToRange,
  flattenDoc,
  highlightColorAt,
  rangeToAnchor,
  type BlockSegments,
} from './anchor'

const doc: Doc = {
  type: 'doc',
  content: [
    { type: 'heading', attrs: { level: 1 }, content: [{ type: 'text', text: 'Ship it' }] },
    {
      type: 'paragraph',
      content: [
        { type: 'text', text: 'We should ' },
        { type: 'text', marks: [{ type: 'bold' }], text: 'definitely' },
        { type: 'text', text: ' ship this.' },
      ],
    },
  ],
}

/**
 * The paragraph above as ProseMirror sees it: the heading occupies positions
 * 0–8, so the paragraph node starts at 9 and its content at 10.
 */
const paragraphBlock: BlockSegments = {
  from: 9,
  to: 42,
  segments: [
    { from: 10, text: 'We should ' },
    { from: 20, text: 'definitely' },
    { from: 30, text: ' ship this.' },
  ],
}

const headingBlock: BlockSegments = {
  from: 0,
  to: 9,
  segments: [{ from: 1, text: 'Ship it' }],
}

const blocks = [headingBlock, paragraphBlock]

const definitely: Anchor = { block_index: 1, start_offset: 10, end_offset: 20, snippet: 'definitely' }

describe('flattenDoc', () => {
  it('joins the text of every descendant, ignoring marks and nesting', () => {
    expect(flattenDoc(doc)).toEqual(['Ship it', 'We should definitely ship this.'])
  })

  it('returns no blocks for a missing or non-doc value', () => {
    expect(flattenDoc(null)).toEqual([])
    expect(flattenDoc({ type: 'paragraph' } as unknown as Doc)).toEqual([])
  })
})

describe('anchorMatches', () => {
  const texts = flattenDoc(doc)

  it('matches while the snippet still sits at its offsets', () => {
    expect(anchorMatches(texts, definitely)).toBe(true)
  })

  it('detaches once the anchored text is edited away', () => {
    const edited: Doc = {
      type: 'doc',
      content: [
        doc.content![0]!,
        { type: 'paragraph', content: [{ type: 'text', text: 'We should ship this.' }] },
      ],
    }
    expect(anchorMatchesDoc(edited, definitely)).toBe(false)
  })

  it('detaches when text shifts, rather than silently highlighting the wrong words', () => {
    const shifted: Doc = {
      type: 'doc',
      content: [
        doc.content![0]!,
        { type: 'paragraph', content: [{ type: 'text', text: 'I think we should definitely ship this.' }] },
      ],
    }
    expect(anchorMatchesDoc(shifted, definitely)).toBe(false)
  })

  it('rejects out-of-range, inverted and empty anchors', () => {
    expect(anchorMatches(texts, { ...definitely, block_index: 9 })).toBe(false)
    expect(anchorMatches(texts, { ...definitely, end_offset: 999 })).toBe(false)
    expect(anchorMatches(texts, { ...definitely, end_offset: 2 })).toBe(false)
    expect(anchorMatches(texts, null)).toBe(false)
  })

  it('measures offsets in UTF-16 code units so it agrees with the Go validator', () => {
    // The rocket is a surrogate pair: two code units, one code point.
    const emoji = ['🚀 launch']
    expect(anchorMatches(emoji, { block_index: 0, start_offset: 3, end_offset: 9, snippet: 'launch' })).toBe(true)
    expect(anchorMatches(emoji, { block_index: 0, start_offset: 2, end_offset: 8, snippet: 'launch' })).toBe(false)
  })
})

describe('anchorToRange', () => {
  it('resolves an anchor spanning a single text run', () => {
    expect(anchorToRange(blocks, definitely)).toEqual({ from: 20, to: 30 })
  })

  it('resolves an anchor spanning several runs', () => {
    const across: Anchor = { block_index: 1, start_offset: 3, end_offset: 15, snippet: 'should defin' }
    expect(anchorToRange(blocks, across)).toEqual({ from: 13, to: 25 })
  })

  it('returns no range for a detached anchor, so no highlight is drawn', () => {
    expect(anchorToRange(blocks, { ...definitely, snippet: 'absolutely' })).toBeNull()
    expect(anchorToRange(blocks, { ...definitely, block_index: 7 })).toBeNull()
    expect(anchorToRange(blocks, null)).toBeNull()
  })
})

describe('rangeToAnchor', () => {
  it('turns a selection into offsets plus the literal snippet', () => {
    expect(rangeToAnchor(blocks, { from: 20, to: 30 })).toEqual(definitely)
  })

  it('anchors a selection that starts inside one run and ends in another', () => {
    expect(rangeToAnchor(blocks, { from: 13, to: 25 })).toEqual({
      block_index: 1,
      start_offset: 3,
      end_offset: 15,
      snippet: 'should defin',
    })
  })

  it('refuses empty, whitespace-only and cross-block selections', () => {
    expect(rangeToAnchor(blocks, { from: 20, to: 20 })).toBeNull()
    expect(rangeToAnchor(blocks, { from: 19, to: 20 })).toBeNull()
    expect(rangeToAnchor(blocks, { from: 3, to: 25 })).toBeNull()
  })
})

describe('highlightColorAt', () => {
  it('gives the first threads distinct colours, then cycles', () => {
    expect(highlightColorAt(0)).not.toBe(highlightColorAt(1))
    expect(highlightColorAt(0)).toBe(highlightColorAt(6))
  })
})
