import type { IdeaStatus } from '../api/types'
import { cx } from '../lib/format'
import { STATUSES, statusLabel } from '../lib/status'

/**
 * A status as a badge. Colour carries the state at a glance, but the label is
 * always spelled out, so the badge never depends on colour alone.
 */
export function StatusBadge({
  status,
  count,
  selected = false,
  className,
}: {
  status: IdeaStatus
  count?: number
  selected?: boolean
  className?: string
}) {
  const descriptor = STATUSES[status]

  return (
    <span
      className={cx(
        'inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium',
        selected ? descriptor.solid : descriptor.quiet,
        className,
      )}
    >
      {statusLabel(status)}
      {count === undefined ? null : (
        <span className={cx('tabular-nums', selected ? 'opacity-70' : 'opacity-60')}>{count}</span>
      )}
    </span>
  )
}
