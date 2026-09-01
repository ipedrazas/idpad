import { describe, expect, it } from 'vitest'

import { applyTheme, parseThemePreference, resolveTheme } from './theme'

describe('resolveTheme', () => {
  it('takes an explicit choice over the OS setting', () => {
    // The whole point of the class-driven variant: picking light on a dark
    // OS must produce a light page.
    expect(resolveTheme('light', true)).toBe('light')
    expect(resolveTheme('dark', false)).toBe('dark')
  })

  it('follows the OS when the preference is system', () => {
    expect(resolveTheme('system', true)).toBe('dark')
    expect(resolveTheme('system', false)).toBe('light')
  })
})

describe('parseThemePreference', () => {
  it('accepts the three valid preferences', () => {
    expect(parseThemePreference('light')).toBe('light')
    expect(parseThemePreference('dark')).toBe('dark')
    expect(parseThemePreference('system')).toBe('system')
  })

  it('falls back to system for anything else', () => {
    // Stored values come from localStorage, so they can be anything at all.
    for (const value of [null, undefined, '', 'DARK', 'blue', 42, {}]) {
      expect(parseThemePreference(value)).toBe('system')
    }
  })
})

describe('applyTheme', () => {
  it('toggles the class and the colour scheme together', () => {
    const root = document.createElement('html')

    applyTheme(root, 'dark')
    expect(root.classList.contains('dark')).toBe(true)
    expect(root.style.colorScheme).toBe('dark')

    applyTheme(root, 'light')
    expect(root.classList.contains('dark')).toBe(false)
    expect(root.style.colorScheme).toBe('light')
  })
})
