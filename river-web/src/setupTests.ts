// Registers @testing-library/jest-dom matchers (e.g. toBeInTheDocument) on
// Vitest's expect and augments its types. Loaded via setupFiles in
// vitest.config.ts, so every test file gets the matchers automatically.
import '@testing-library/jest-dom/vitest'

// jsdom only enables Web Storage for documents with a non-opaque origin, and
// the default test document doesn't qualify — so `localStorage` exists as an
// inert stub with no methods. The app (and its data-access layer) relies on
// real localStorage, so install a minimal in-memory implementation when the
// environment doesn't provide a working one.
function installMemoryStorage(name: 'localStorage' | 'sessionStorage') {
  const existing = (globalThis as Record<string, unknown>)[name] as Storage | undefined
  if (existing && typeof existing.setItem === 'function') return

  const store = new Map<string, string>()
  const storage: Storage = {
    get length() { return store.size },
    clear: () => { store.clear() },
    getItem: (key) => (store.has(key) ? store.get(key)! : null),
    key: (index) => Array.from(store.keys())[index] ?? null,
    removeItem: (key) => { store.delete(key) },
    setItem: (key, value) => { store.set(key, String(value)) },
  }
  Object.defineProperty(globalThis, name, { value: storage, configurable: true, writable: true })
}

installMemoryStorage('localStorage')
installMemoryStorage('sessionStorage')
