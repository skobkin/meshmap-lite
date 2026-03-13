import { cleanup } from '@testing-library/preact'
import { afterEach, beforeEach, vi } from 'vitest'

class MemoryStorage implements Storage {
  #items = new Map<string, string>()

  get length(): number {
    return this.#items.size
  }

  clear(): void {
    this.#items.clear()
  }

  getItem(key: string): string | null {
    return this.#items.get(key) ?? null
  }

  key(index: number): string | null {
    return Array.from(this.#items.keys())[index] ?? null
  }

  removeItem(key: string): void {
    this.#items.delete(key)
  }

  setItem(key: string, value: string): void {
    this.#items.set(key, value)
  }
}

beforeEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()

  if (typeof localStorage === 'undefined' || typeof localStorage.clear !== 'function') {
    vi.stubGlobal('localStorage', new MemoryStorage())
  } else {
    localStorage.clear()
  }

  if (typeof location === 'undefined') {
    vi.stubGlobal('location', {
      protocol: 'http:',
      host: 'meshmap.test'
    } satisfies Partial<Location>)
  }

  if (typeof window === 'undefined') {
    vi.stubGlobal('window', globalThis)
  }
})

afterEach(() => {
  cleanup()
  vi.useRealTimers()
})
