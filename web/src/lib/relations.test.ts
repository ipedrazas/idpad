import { describe, expect, it } from 'vitest'

import type { Link, Relation } from '../api/types'
import { groupLinks, isRelation, relationArrow, relationLabel, RELATIONS } from './relations'

function link(overrides: Partial<Link> = {}): Link {
  return {
    id: 'link-1',
    source_idea_id: 'a',
    target_idea_id: 'b',
    relation: 'references',
    note: null,
    created_at: '2026-01-01T00:00:00Z',
    direction: 'outgoing',
    other_idea_id: 'b',
    other_title: 'Other',
    ...overrides,
  }
}

describe('relationLabel', () => {
  it('reads a directed relation actively from the source', () => {
    expect(relationLabel('references', 'outgoing')).toBe('references')
    expect(relationLabel('expands', 'outgoing')).toBe('expands on')
  })

  it('reads a directed relation passively from the target', () => {
    // One stored row, two phrasings: this is what makes a link visible from
    // both ends without storing it twice.
    expect(relationLabel('references', 'incoming')).toBe('referenced by')
    expect(relationLabel('expands', 'incoming')).toBe('expanded on by')
  })

  it('reads a symmetric relation the same from both ends', () => {
    for (const relation of ['similar', 'related'] as Relation[]) {
      expect(relationLabel(relation, 'outgoing')).toBe(relationLabel(relation, 'incoming'))
    }
  })
})

describe('relationArrow', () => {
  it('points in the direction of a directed link', () => {
    expect(relationArrow('references', 'outgoing')).toBe('→')
    expect(relationArrow('references', 'incoming')).toBe('←')
  })

  it('uses a two-headed arrow for symmetric links', () => {
    // A one-way arrow would imply a direction "similar to" does not carry.
    expect(relationArrow('similar', 'outgoing')).toBe('↔')
    expect(relationArrow('similar', 'incoming')).toBe('↔')
  })
})

describe('groupLinks', () => {
  it('separates what this idea declared from what points at it', () => {
    const { outgoing, incoming } = groupLinks([
      link({ id: 'out', relation: 'references', direction: 'outgoing' }),
      link({ id: 'in', relation: 'expands', direction: 'incoming' }),
    ])
    expect(outgoing.map((l) => l.id)).toEqual(['out'])
    expect(incoming.map((l) => l.id)).toEqual(['in'])
  })

  it('groups symmetric links together whichever end typed them', () => {
    // "similar to" is not a claim either side owns, so listing it under
    // "pointing here" for one idea only would misdescribe it.
    const { outgoing, incoming } = groupLinks([
      link({ id: 'mine', relation: 'similar', direction: 'outgoing' }),
      link({ id: 'theirs', relation: 'similar', direction: 'incoming' }),
    ])
    expect(outgoing.map((l) => l.id)).toEqual(['mine', 'theirs'])
    expect(incoming).toEqual([])
  })
})

describe('isRelation', () => {
  it('accepts every relation the API stores', () => {
    for (const relation of Object.keys(RELATIONS)) {
      expect(isRelation(relation)).toBe(true)
    }
  })

  it('rejects an inverse, which is a rendering rather than a stored value', () => {
    expect(isRelation('referenced_by')).toBe(false)
    expect(isRelation('')).toBe(false)
  })
})
