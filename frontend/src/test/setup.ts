import '@testing-library/jest-dom/vitest';
// Registers an afterEach hook that unmounts components rendered via
// @testing-library/svelte. Required because we run with globals:false,
// which disables the library's default auto-cleanup wiring.
import '@testing-library/svelte/vitest';

// happy-dom 20.x exposes a top-level `localStorage` stub that lacks
// Storage methods (getItem / setItem / clear). Replace it with a
// minimal in-memory Storage so modules that hydrate from localStorage
// at import time can run under vitest.
if (
  typeof globalThis.localStorage === 'undefined' ||
  typeof globalThis.localStorage.getItem !== 'function'
) {
  const store = new Map<string, string>();
  const impl: Storage = {
    get length() {
      return store.size;
    },
    clear() {
      store.clear();
    },
    getItem(k: string) {
      return store.has(k) ? store.get(k)! : null;
    },
    key(i: number) {
      return Array.from(store.keys())[i] ?? null;
    },
    removeItem(k: string) {
      store.delete(k);
    },
    setItem(k: string, v: string) {
      store.set(k, String(v));
    },
  };
  Object.defineProperty(globalThis, 'localStorage', {
    value: impl,
    writable: true,
    configurable: true,
  });
}
