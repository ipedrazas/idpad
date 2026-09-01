import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'

import { ThemeContext, type ThemeContextValue } from './context'
import { applyTheme, parseThemePreference, resolveTheme, THEME_STORAGE_KEY, type ThemePreference } from './theme'

/** The media query 'system' follows. */
const DARK_QUERY = '(prefers-color-scheme: dark)'

/**
 * Reads the stored preference. Storage can throw in a locked-down browser, so
 * a failure falls back to 'system' rather than taking the app down.
 */
function readStoredPreference(): ThemePreference {
  try {
    return parseThemePreference(window.localStorage.getItem(THEME_STORAGE_KEY))
  } catch {
    return 'system'
  }
}

function systemPrefersDark(): boolean {
  return window.matchMedia(DARK_QUERY).matches
}

/**
 * Owns the theme preference: persists it, resolves it against the OS, and
 * writes the result onto <html>.
 *
 * The first paint is already correct — the inline script in index.html applies
 * the same rule before React mounts — so this provider re-applies rather than
 * introduces the class.
 */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [preference, setPreferenceState] = useState<ThemePreference>(readStoredPreference)
  const [prefersDark, setPrefersDark] = useState<boolean>(systemPrefersDark)

  // While the preference is 'system' the theme has to keep tracking the OS,
  // so the app flips the moment the user changes it elsewhere.
  useEffect(() => {
    const query = window.matchMedia(DARK_QUERY)
    const onChange = (event: MediaQueryListEvent) => setPrefersDark(event.matches)
    query.addEventListener('change', onChange)
    return () => query.removeEventListener('change', onChange)
  }, [])

  const resolved = resolveTheme(preference, prefersDark)

  useEffect(() => {
    applyTheme(document.documentElement, resolved)
  }, [resolved])

  const setPreference = useCallback((next: ThemePreference) => {
    setPreferenceState(next)
    try {
      window.localStorage.setItem(THEME_STORAGE_KEY, next)
    } catch {
      // A preference that cannot be persisted still applies for this session.
    }
  }, [])

  const value = useMemo<ThemeContextValue>(
    () => ({ preference, resolved, setPreference }),
    [preference, resolved, setPreference],
  )

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
}
