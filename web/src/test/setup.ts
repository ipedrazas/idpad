import '@testing-library/jest-dom/vitest'
import { beforeEach } from 'vitest'

/**
 * jsdom's localStorage is shadowed here by Node's own experimental one, which
 * is disabled unless the runtime is started with --localstorage-file. Tests
 * that exercise persistence need a working Storage, so install an in-memory
 * one when nothing usable is present.
 */
if (typeof window !== 'undefined' && !window.localStorage) {
  const store = new Map<string, string>()
  const memoryStorage: Storage = {
    get length() {
      return store.size
    },
    key: (index) => [...store.keys()][index] ?? null,
    getItem: (key) => store.get(key) ?? null,
    setItem: (key, value) => void store.set(key, String(value)),
    removeItem: (key) => void store.delete(key),
    clear: () => store.clear(),
  }
  Object.defineProperty(window, 'localStorage', { value: memoryStorage, configurable: true })
}

// Storage is shared across a file's tests, so it is cleared between them.
beforeEach(() => {
  window.localStorage?.clear()
})
