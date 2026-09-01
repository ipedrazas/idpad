import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { THEME_STORAGE_KEY } from '../theme/theme'
import { ThemeProvider } from '../theme/ThemeProvider'
import { ThemeToggle } from './ThemeToggle'

/** Stands in for matchMedia, which jsdom does not implement. */
function mockMatchMedia(prefersDark: boolean) {
  const listeners = new Set<(event: MediaQueryListEvent) => void>()

  vi.stubGlobal(
    'matchMedia',
    vi.fn().mockImplementation((query: string) => ({
      matches: prefersDark,
      media: query,
      addEventListener: (_type: string, listener: (event: MediaQueryListEvent) => void) =>
        listeners.add(listener),
      removeEventListener: (_type: string, listener: (event: MediaQueryListEvent) => void) =>
        listeners.delete(listener),
    })),
  )

  /** Fires a change, as the OS switching theme would. */
  return (matches: boolean) => {
    act(() => {
      for (const listener of listeners) listener({ matches } as MediaQueryListEvent)
    })
  }
}

function renderToggle() {
  return render(
    <ThemeProvider>
      <ThemeToggle />
    </ThemeProvider>,
  )
}

const isDark = () => document.documentElement.classList.contains('dark')

beforeEach(() => {
  window.localStorage.clear()
  document.documentElement.classList.remove('dark')
  document.documentElement.style.colorScheme = ''
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('ThemeToggle', () => {
  it('defaults to system and follows the OS setting', () => {
    mockMatchMedia(true)
    renderToggle()

    expect(screen.getByRole('radio', { name: 'Match system' })).toBeChecked()
    expect(isDark()).toBe(true)
  })

  it('lets an explicit light choice win over a dark OS', async () => {
    // The reason dark mode is class-driven rather than media-query-driven:
    // with the media query in the variant this case would stay dark.
    mockMatchMedia(true)
    renderToggle()
    expect(isDark()).toBe(true)

    await userEvent.click(screen.getByRole('radio', { name: 'Light' }))

    expect(isDark()).toBe(false)
    expect(document.documentElement.style.colorScheme).toBe('light')
  })

  it('lets an explicit dark choice win over a light OS', async () => {
    mockMatchMedia(false)
    renderToggle()

    await userEvent.click(screen.getByRole('radio', { name: 'Dark' }))

    expect(isDark()).toBe(true)
    expect(document.documentElement.style.colorScheme).toBe('dark')
  })

  it('persists the choice so a reload keeps it', async () => {
    mockMatchMedia(false)
    renderToggle()

    await userEvent.click(screen.getByRole('radio', { name: 'Dark' }))
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark')
  })

  it('restores a stored choice on mount', () => {
    window.localStorage.setItem(THEME_STORAGE_KEY, 'dark')
    mockMatchMedia(false)
    renderToggle()

    expect(screen.getByRole('radio', { name: 'Dark' })).toBeChecked()
    expect(isDark()).toBe(true)
  })

  it('keeps tracking the OS while the preference is system', async () => {
    const emitChange = mockMatchMedia(false)
    renderToggle()
    expect(isDark()).toBe(false)

    // 'system' is a standing instruction, not a one-off resolution.
    emitChange(true)
    expect(isDark()).toBe(true)

    emitChange(false)
    expect(isDark()).toBe(false)
  })

  it('stops tracking the OS once a choice is made', async () => {
    const emitChange = mockMatchMedia(false)
    renderToggle()

    await userEvent.click(screen.getByRole('radio', { name: 'Light' }))
    emitChange(true)

    expect(isDark()).toBe(false)
  })
})
