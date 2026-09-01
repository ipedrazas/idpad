import { createContext } from 'react'

import type { ResolvedTheme, ThemePreference } from './theme'

export interface ThemeContextValue {
  /** What the user chose, including 'system'. */
  preference: ThemePreference
  /** What is actually painted right now. */
  resolved: ResolvedTheme
  setPreference: (preference: ThemePreference) => void
}

/**
 * Lives apart from the provider so that file exports only a component, which
 * is what keeps fast refresh working for it.
 */
export const ThemeContext = createContext<ThemeContextValue | null>(null)
