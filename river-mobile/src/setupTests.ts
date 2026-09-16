import '@testing-library/jest-dom/vitest'

// Node 25 ships an experimental global `localStorage` that shadows jsdom's and
// throws on use ("getItem is not a function"). The api client reads it at
// construction, so install a plain in-memory Storage for the test environment.
const store = new Map<string, string>()
const memoryStorage: Storage = {
  get length() { return store.size },
  clear: () => store.clear(),
  getItem: (k: string) => (store.has(k) ? store.get(k)! : null),
  key: (i: number) => Array.from(store.keys())[i] ?? null,
  removeItem: (k: string) => { store.delete(k) },
  setItem: (k: string, v: string) => { store.set(k, String(v)) },
}
Object.defineProperty(globalThis, 'localStorage', { value: memoryStorage, configurable: true })
