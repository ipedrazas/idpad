/** Small pure formatting helpers shared by the views. */

import type { Doc, DocNode } from '../api/types'
import { truncate } from './anchor'

const RELATIVE_UNITS: { limit: number; unit: Intl.RelativeTimeFormatUnit; ms: number }[] = [
  { limit: 60_000, unit: 'second', ms: 1_000 },
  { limit: 3_600_000, unit: 'minute', ms: 60_000 },
  { limit: 86_400_000, unit: 'hour', ms: 3_600_000 },
  { limit: 604_800_000, unit: 'day', ms: 86_400_000 },
  { limit: 2_629_800_000, unit: 'week', ms: 604_800_000 },
  { limit: 31_557_600_000, unit: 'month', ms: 2_629_800_000 },
  { limit: Infinity, unit: 'year', ms: 31_557_600_000 },
]

const relativeFormatter = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' })

/** Renders an RFC 3339 timestamp as "3 minutes ago". */
export function relativeTime(timestamp: string, now: Date = new Date()): string {
  const then = new Date(timestamp)
  if (Number.isNaN(then.getTime())) return ''

  const deltaMs = then.getTime() - now.getTime()
  const magnitude = Math.abs(deltaMs)

  const scale = RELATIVE_UNITS.find((candidate) => magnitude < candidate.limit) ?? RELATIVE_UNITS.at(-1)
  if (!scale) return ''
  if (scale.unit === 'second' && magnitude < 45_000) return 'just now'

  return relativeFormatter.format(Math.round(deltaMs / scale.ms), scale.unit)
}

/** Renders a timestamp as an absolute string, used for hover titles. */
export function absoluteTime(timestamp: string): string {
  const date = new Date(timestamp)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
}

/**
 * Flattens a node for display. Unlike the anchor module's flattening — which
 * must concatenate text exactly, because offsets are measured against it —
 * this separates list items so a one-line preview stays readable.
 */
function previewText(node: DocNode): string {
  if (node.type === 'text') return node.text ?? ''

  const parts = (node.content ?? []).map(previewText).filter(Boolean)
  const separator = node.type === 'bulletList' || node.type === 'orderedList' ? ' · ' : ''
  return parts.join(separator)
}

/** A one-line preview of an idea body for the list cards. */
export function bodyPreview(body: Doc | null | undefined, max = 160): string {
  if (!body || body.type !== 'doc') return ''
  const text = (body.content ?? []).map(previewText).filter(Boolean).join(' · ')
  return text ? truncate(text, max) : ''
}

/** The host of a resource URL, shown on thumbnails and link cards. */
export function urlHost(rawUrl: string): string {
  try {
    return new URL(rawUrl).host
  } catch {
    return rawUrl
  }
}

/** Joins conditional class names, dropping the falsy ones. */
export function cx(...classes: (string | false | null | undefined)[]): string {
  return classes.filter(Boolean).join(' ')
}
