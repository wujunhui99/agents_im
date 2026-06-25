import '@testing-library/jest-dom/vitest';

// Node.js 25+ ships a built-in global localStorage that requires --localstorage-file
// to work.  Without a valid file path it provides a stub with null prototype and no
// Storage methods (clear, getItem, setItem, …).  Replace it with a plain in-memory
// implementation so tests can call localStorage.clear() and friends normally.
if (typeof localStorage !== 'undefined' && typeof localStorage.clear !== 'function') {
  const store = new Map<string, string>();
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    writable: true,
    value: {
      clear(): void { store.clear(); },
      getItem(key: string): string | null { return store.has(key) ? store.get(key)! : null; },
      setItem(key: string, value: string): void { store.set(String(key), String(value)); },
      removeItem(key: string): void { store.delete(String(key)); },
      key(index: number): string | null { return [...store.keys()][index] ?? null; },
      get length(): number { return store.size; },
    },
  });
}
