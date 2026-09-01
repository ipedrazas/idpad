import type { IdeaStatus } from '../api/types'
import { cx } from '../lib/format'
import { STATUS_OPTIONS, STATUSES } from '../lib/status'
import { Spinner } from './ui'

/**
 * The control that moves an idea along its lifecycle. It is a radio group
 * rather than a menu because there are only four states and seeing the ones
 * not chosen is half the point: the row doubles as a reminder of where the
 * idea could go next.
 */
export function StatusPicker({
  status,
  onChange,
  pending = false,
  className,
}: {
  status: IdeaStatus
  onChange: (next: IdeaStatus) => void
  pending?: boolean
  className?: string
}) {
  return (
    <div
      role="radiogroup"
      aria-label="Idea status"
      className={cx(
        'inline-flex items-center gap-0.5 rounded-lg border border-slate-200 p-0.5 dark:border-slate-800',
        className,
      )}
    >
      {STATUS_OPTIONS.map((option) => {
        const descriptor = STATUSES[option]
        const selected = status === option
        return (
          <button
            key={option}
            type="button"
            role="radio"
            aria-checked={selected}
            title={descriptor.hint}
            disabled={pending}
            // Re-picking the current status is a no-op on the server, but not
            // asking at all keeps the button honest while a move is in flight.
            onClick={() => !selected && onChange(option)}
            className={cx(
              'flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-medium transition-colors',
              'disabled:cursor-not-allowed disabled:opacity-60',
              selected
                ? cx('border', descriptor.solid)
                : 'border border-transparent text-slate-500 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-100',
            )}
          >
            {selected && pending ? <Spinner className="size-3" /> : null}
            {descriptor.label}
          </button>
        )
      })}
    </div>
  )
}
