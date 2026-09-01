/** Small presentational primitives shared across the views. */

import type { ButtonHTMLAttributes, ReactNode } from 'react'

import { cx } from '../lib/format'

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger'
type ButtonSize = 'sm' | 'md'

const VARIANT_CLASSES: Record<ButtonVariant, string> = {
  primary:
    'bg-slate-900 text-white hover:bg-slate-700 disabled:hover:bg-slate-900 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white',
  secondary:
    'border border-slate-300 bg-white text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800',
  ghost: 'text-slate-500 hover:bg-slate-200/70 hover:text-slate-900 dark:hover:bg-slate-800 dark:hover:text-slate-100',
  danger: 'bg-rose-600 text-white hover:bg-rose-500',
}

const SIZE_CLASSES: Record<ButtonSize, string> = {
  sm: 'px-2.5 py-1 text-xs',
  md: 'px-3.5 py-2 text-sm',
}

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
}

export function Button({ variant = 'secondary', size = 'md', className, type = 'button', ...props }: ButtonProps) {
  return (
    <button
      type={type}
      className={cx(
        'inline-flex items-center justify-center gap-1.5 rounded-lg font-medium transition-colors',
        'disabled:cursor-not-allowed disabled:opacity-50',
        VARIANT_CLASSES[variant],
        SIZE_CLASSES[size],
        className,
      )}
      {...props}
    />
  )
}

/** A spinning indicator sized to the surrounding text. */
export function Spinner({ className }: { className?: string }) {
  return (
    <span
      role="status"
      aria-label="Loading"
      className={cx(
        'inline-block size-4 animate-spin rounded-full border-2 border-current border-t-transparent',
        className,
      )}
    />
  )
}

/** A neutral grey block standing in for content that has not arrived yet. */
export function Skeleton({ className }: { className?: string }) {
  return <div className={cx('animate-pulse rounded-md bg-slate-200 dark:bg-slate-800', className)} />
}

interface StateProps {
  title: string
  description?: string
  action?: ReactNode
  className?: string
}

/** Shown when a collection is legitimately empty. */
export function EmptyState({ title, description, action, className }: StateProps) {
  return (
    <div
      className={cx(
        'rounded-xl border border-dashed border-slate-300 bg-white/50 px-6 py-10 text-center',
        'dark:border-slate-700 dark:bg-slate-900/40',
        className,
      )}
    >
      <p className="text-sm font-medium text-slate-700 dark:text-slate-200">{title}</p>
      {description ? <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{description}</p> : null}
      {action ? <div className="mt-4 flex justify-center">{action}</div> : null}
    </div>
  )
}

/** Shown when a request failed, with the API's message when there is one. */
export function ErrorState({ title, description, action, className }: StateProps) {
  return (
    <div
      className={cx(
        'rounded-xl border border-rose-300 bg-rose-50 px-6 py-6 text-center',
        'dark:border-rose-900 dark:bg-rose-950/40',
        className,
      )}
      role="alert"
    >
      <p className="text-sm font-medium text-rose-800 dark:text-rose-200">{title}</p>
      {description ? <p className="mt-1 text-sm text-rose-700/80 dark:text-rose-300/80">{description}</p> : null}
      {action ? <div className="mt-4 flex justify-center">{action}</div> : null}
    </div>
  )
}

/** An inline error message for form fields and composers. */
export function FieldError({ children }: { children: ReactNode }) {
  return <p className="mt-1.5 text-xs text-rose-600 dark:text-rose-400">{children}</p>
}

/** The shared card/panel surface. */
export function Panel({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div
      className={cx(
        'rounded-xl border border-slate-200 bg-white shadow-sm',
        'dark:border-slate-800 dark:bg-slate-900',
        className,
      )}
    >
      {children}
    </div>
  )
}
