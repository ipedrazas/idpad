/**
 * How each lifecycle state reads and looks.
 *
 * Status is a closed set with exactly one value per idea, so unlike tags it has
 * a fixed vocabulary the UI can spell out: a label to read, a hint for the
 * picker, and a colour that is never the only signal — every chip and badge
 * carries its label too.
 */

import type { IdeaStatus } from '../api/types'

interface StatusDescriptor {
  /** How the state is written wherever it is shown. */
  label: string
  /** A short line explaining the state in the picker. */
  hint: string
  /** The quiet form, used for the badge and an unselected filter chip. */
  quiet: string
  /** The solid form, used for a selected filter chip and the picker. */
  solid: string
}

export const STATUSES: Record<IdeaStatus, StatusDescriptor> = {
  draft: {
    label: 'Draft',
    hint: 'Written down, not started on',
    quiet: 'border-slate-200 bg-slate-100 text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300',
    solid: 'border-slate-900 bg-slate-900 text-white dark:border-slate-100 dark:bg-slate-100 dark:text-slate-900',
  },
  in_progress: {
    label: 'In progress',
    hint: 'Being worked on now',
    quiet:
      'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950/60 dark:text-amber-300',
    solid: 'border-amber-600 bg-amber-600 text-white dark:border-amber-500 dark:bg-amber-500 dark:text-amber-950',
  },
  done: {
    label: 'Done',
    hint: 'Finished, or acted on',
    quiet:
      'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/60 dark:text-emerald-300',
    solid:
      'border-emerald-600 bg-emerald-600 text-white dark:border-emerald-500 dark:bg-emerald-500 dark:text-emerald-950',
  },
  rejected: {
    label: 'Rejected',
    hint: 'Decided against, kept for the record',
    quiet: 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900 dark:bg-rose-950/60 dark:text-rose-300',
    solid: 'border-rose-600 bg-rose-600 text-white dark:border-rose-500 dark:bg-rose-500 dark:text-rose-950',
  },
}

/** The statuses in lifecycle order, which is how every control renders them. */
export const STATUS_OPTIONS = Object.keys(STATUSES) as IdeaStatus[]

/** True when the value is a status the API accepts. */
export function isIdeaStatus(value: string): value is IdeaStatus {
  return value in STATUSES
}

/** The label for a status, or the raw value if the API grows one first. */
export function statusLabel(status: IdeaStatus): string {
  return STATUSES[status]?.label ?? status
}
