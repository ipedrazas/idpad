import { cx } from '../lib/format'
import { THEME_PREFERENCES, type ThemePreference } from '../theme/theme'
import { useTheme } from '../theme/useTheme'

/** The glyph and accessible name for each preference. */
const OPTIONS: Record<ThemePreference, { icon: string; label: string }> = {
  light: { icon: '☀', label: 'Light' },
  system: { icon: '◐', label: 'Match system' },
  dark: { icon: '☾', label: 'Dark' },
}

/**
 * A three-way theme control. It is a radio group rather than a two-state
 * switch because "match system" is a real choice, not the absence of one:
 * a user on system wants the page to follow the OS from now on.
 */
export function ThemeToggle({ className }: { className?: string }) {
  const { preference, setPreference } = useTheme()

  return (
    <div
      role="radiogroup"
      aria-label="Colour theme"
      className={cx(
        'inline-flex items-center gap-0.5 rounded-lg border border-slate-200 p-0.5',
        'dark:border-slate-800',
        className,
      )}
    >
      {THEME_PREFERENCES.map((option) => {
        const { icon, label } = OPTIONS[option]
        const selected = preference === option
        return (
          <button
            key={option}
            type="button"
            role="radio"
            aria-checked={selected}
            aria-label={label}
            title={label}
            onClick={() => setPreference(option)}
            className={cx(
              'flex size-7 items-center justify-center rounded-md text-sm transition-colors',
              selected
                ? 'bg-slate-900 text-white dark:bg-slate-100 dark:text-slate-900'
                : 'text-slate-500 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-100',
            )}
          >
            <span aria-hidden="true">{icon}</span>
          </button>
        )
      })}
    </div>
  )
}
