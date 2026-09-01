import { useContext } from 'react'

import { ThemeContext, type ThemeContextValue } from './context'

/** Reads the current theme. Throws outside a ThemeProvider, which is a bug. */
export function useTheme(): ThemeContextValue {
  const context = useContext(ThemeContext)
  if (!context) {
    throw new Error('useTheme must be used inside a ThemeProvider')
  }
  return context
}
