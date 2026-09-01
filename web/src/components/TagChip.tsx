import { Link } from 'react-router-dom'

import type { Tag } from '../api/types'
import { cx } from '../lib/format'

/**
 * A tag as a chip. With `to` it navigates to the filtered list; with
 * `onRemove` it carries a remove affordance; with neither it is plain text.
 */
export function TagChip({
  tag,
  to,
  onRemove,
  count,
  active = false,
  className,
}: {
  tag: Pick<Tag, 'name' | 'slug'>
  to?: string
  onRemove?: () => void
  count?: number
  active?: boolean
  className?: string
}) {
  const base = cx(
    'inline-flex max-w-full items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium',
    active
      ? 'border-slate-900 bg-slate-900 text-white dark:border-slate-100 dark:bg-slate-100 dark:text-slate-900'
      : 'border-slate-200 bg-slate-100 text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300',
    className,
  )

  const label = (
    <>
      <span className="truncate">{tag.name}</span>
      {count === undefined ? null : (
        <span className={cx('tabular-nums', active ? 'opacity-70' : 'text-slate-400')}>{count}</span>
      )}
    </>
  )

  if (onRemove) {
    return (
      <span className={base}>
        {label}
        <button
          type="button"
          onClick={onRemove}
          aria-label={`Remove tag ${tag.name}`}
          className="-mr-0.5 rounded-full px-0.5 text-slate-400 transition-colors hover:text-rose-600 dark:hover:text-rose-400"
        >
          ×
        </button>
      </span>
    )
  }

  if (to) {
    return (
      <Link
        to={to}
        className={cx(
          base,
          'transition-colors',
          active ? 'hover:opacity-90' : 'hover:border-slate-300 hover:text-slate-900 dark:hover:text-slate-100',
        )}
      >
        {label}
      </Link>
    )
  }

  return <span className={base}>{label}</span>
}
