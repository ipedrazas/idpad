/**
 * Theme preference: the pure part, shared by the pre-paint script in
 * index.html and the React provider. Keeping the storage key and the
 * resolution rule in one place is what stops the two from drifting and
 * flashing the wrong colours on load.
 */

/** What the user chose. 'system' defers to the OS setting, and keeps deferring. */
export type ThemePreference = 'light' | 'dark' | 'system'

/** What is actually painted once 'system' has been resolved. */
export type ResolvedTheme = 'light' | 'dark'

/** The localStorage key. Must match the inline script in index.html. */
export const THEME_STORAGE_KEY = 'idpad-theme'

/** The preferences the toggle offers, in the order it renders them. */
export const THEME_PREFERENCES: ThemePreference[] = ['light', 'system', 'dark']

/** Narrows an unknown stored value to a preference, defaulting to 'system'. */
export function parseThemePreference(value: unknown): ThemePreference {
  return value === 'light' || value === 'dark' || value === 'system' ? value : 'system'
}

/**
 * Resolves a preference against the OS setting. `systemPrefersDark` is passed
 * in rather than read here so this stays pure and testable.
 */
export function resolveTheme(preference: ThemePreference, systemPrefersDark: boolean): ResolvedTheme {
  if (preference === 'system') return systemPrefersDark ? 'dark' : 'light'
  return preference
}

/**
 * Applies a resolved theme to the document element: the `dark` class drives
 * Tailwind's variant, and `color-scheme` makes the browser's own widgets
 * (scrollbars, form controls, the canvas behind the page) match.
 */
export function applyTheme(root: HTMLElement, theme: ResolvedTheme): void {
  root.classList.toggle('dark', theme === 'dark')
  root.style.colorScheme = theme
}
