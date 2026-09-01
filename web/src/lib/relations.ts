/**
 * How a stored link reads from each end.
 *
 * The API stores one row, directed source → target. The idea viewing it sees
 * either the active phrasing (it is the source) or the passive one (it is the
 * target). Symmetric relations read the same either way, so they have no
 * distinct passive form.
 */

import type { Link, LinkDirection, Relation } from '../api/types'

interface RelationDescriptor {
  /** How the source idea reads the link: "this idea <active> that one". */
  active: string
  /** How the target idea reads it. Equal to active for symmetric relations. */
  passive: string
  /** Whether the relation reads the same from both ends. */
  symmetric: boolean
  /** A short hint shown in the relation picker. */
  hint: string
}

export const RELATIONS: Record<Relation, RelationDescriptor> = {
  references: {
    active: 'references',
    passive: 'referenced by',
    symmetric: false,
    hint: 'This idea draws on that one',
  },
  expands: {
    active: 'expands on',
    passive: 'expanded on by',
    symmetric: false,
    hint: 'This idea takes that one further',
  },
  similar: {
    active: 'similar to',
    passive: 'similar to',
    symmetric: true,
    hint: 'Two takes on the same thing',
  },
  related: {
    active: 'related to',
    passive: 'related to',
    symmetric: true,
    hint: 'Worth reading together',
  },
}

/** The relations the picker offers, in the order it renders them. */
export const RELATION_OPTIONS = Object.keys(RELATIONS) as Relation[]

/** True when the value is a relation the API accepts. */
export function isRelation(value: string): value is Relation {
  return value in RELATIONS
}

/**
 * The phrase to render for a link, from the point of view of the idea whose
 * page it is on. An outgoing link reads actively, an incoming one passively.
 */
export function relationLabel(relation: Relation, direction: LinkDirection): string {
  const descriptor = RELATIONS[relation]
  return direction === 'outgoing' ? descriptor.active : descriptor.passive
}

/**
 * The arrow shown beside a link. Symmetric relations get a plain dash: an
 * arrow would imply a direction the relation does not carry.
 */
export function relationArrow(relation: Relation, direction: LinkDirection): string {
  if (RELATIONS[relation].symmetric) return '↔'
  return direction === 'outgoing' ? '→' : '←'
}

/**
 * Splits links into the ones this idea declared and the ones declared at it.
 * Symmetric links are grouped with the outgoing ones regardless of which side
 * typed them, because "similar to" is not a claim either end owns.
 */
export function groupLinks(links: Link[]): { outgoing: Link[]; incoming: Link[] } {
  const outgoing: Link[] = []
  const incoming: Link[] = []

  for (const link of links) {
    if (link.direction === 'outgoing' || RELATIONS[link.relation].symmetric) {
      outgoing.push(link)
    } else {
      incoming.push(link)
    }
  }
  return { outgoing, incoming }
}
