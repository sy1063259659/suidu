import { describe, expect, it, vi } from 'vitest'
import { readLibraryState, writeLibraryState, type LibraryState } from './libraryState'

function memoryStorage(): Storage {
  const entries = new Map<string, string>()
  return {
    get length() { return entries.size },
    clear: vi.fn(() => entries.clear()),
    getItem: vi.fn((key: string) => entries.get(key) ?? null),
    key: vi.fn((index: number) => Array.from(entries.keys())[index] ?? null),
    removeItem: vi.fn((key: string) => { entries.delete(key) }),
    setItem: vi.fn((key: string, value: string) => { entries.set(key, value) }),
  }
}

function libraryState(overrides: Partial<LibraryState> = {}): LibraryState {
  return { query: ' 搜索🙂 ', kind: 'text', favorites: true, preset: 'custom', range: [0, 1760000000000], scrollY: 1234.5, ...overrides }
}

describe('per-user library session state', () => {
  it('restores an exact loaded boundary with nanoseconds alongside scroll state', () => {
    const storage = memoryStorage()
    const state = libraryState({ loadedBoundary: { id: 150, createdAt: '2026-11-14T12:00:00.123456789Z' } })
    expect(writeLibraryState(storage, 1, state)).toBe(true)
    expect(readLibraryState(storage, 1)).toEqual(state)
  })

  it.each([{ id: 0, createdAt: '2026-11-14T12:00:00Z' }, { id: 1, createdAt: 'broken' }, 'bad'])('rejects an invalid loaded boundary %o', (loadedBoundary) => {
    const storage = memoryStorage()
    expect(writeLibraryState(storage, 1, libraryState({ loadedBoundary } as Partial<LibraryState>))).toBe(false)
  })

  it('round trips filters and an exact scroll position independently per user', () => {
    const storage = memoryStorage()
    const first = libraryState()
    const second = libraryState({ query: '', kind: 'image', favorites: false, preset: 'all', range: null, scrollY: 0 })
    expect(writeLibraryState(storage, 1, first)).toBe(true)
    expect(writeLibraryState(storage, 2, second)).toBe(true)
    expect(readLibraryState(storage, 1)).toEqual(first)
    expect(readLibraryState(storage, 2)).toEqual(second)
    expect(readLibraryState(storage, 3)).toBeNull()
    expect(storage.getItem('suidu:library:v1:1')).not.toBeNull()
  })

  it('preserves a query at the limit and a large finite scroll position', () => {
    const storage = memoryStorage()
    const state = libraryState({ query: '中'.repeat(200), scrollY: Number.MAX_VALUE })
    expect(writeLibraryState(storage, 1, state)).toBe(true)
    expect(readLibraryState(storage, 1)).toEqual(state)
  })

  it.each(['all', 'today', 'last7Days', 'last30Days'] as const)('allows null dates with %s preset', (preset) => {
    const storage = memoryStorage()
    const state = libraryState({ preset, range: null })
    expect(writeLibraryState(storage, 1, state)).toBe(true)
    expect(readLibraryState(storage, 1)).toEqual(state)
  })

  it.each([
    'not JSON', 'null', '[]', '{}',
    JSON.stringify({ version: 2, userID: 1, state: libraryState() }),
    JSON.stringify({ version: 1, userID: 2, state: libraryState() }),
    JSON.stringify({ version: 1, userID: 1, state: null }),
  ])('ignores invalid or mismatched envelopes without mutating them: %s', (raw) => {
    const storage = memoryStorage()
    storage.setItem('suidu:library:v1:1', raw)
    expect(readLibraryState(storage, 1)).toBeNull()
    expect(storage.getItem('suidu:library:v1:1')).toBe(raw)
    expect(storage.removeItem).not.toHaveBeenCalled()
  })

  it.each([
    { query: 'x'.repeat(201) }, { query: 1 }, { kind: 'unknown' }, { favorites: 'true' },
    { preset: 'tomorrow' }, { preset: 'custom', range: null },
    { range: [] }, { range: [1] }, { range: [1, 2, 3] }, { range: [2, 1] },
    { range: ['1', 2] }, { range: [0, null] }, { range: [0, 8640000000000001] },
    { scrollY: -1 }, { scrollY: '2' },
  ])('rejects invalid state on both read and write: %o', (override) => {
    const storage = memoryStorage()
    const state = { ...libraryState(), ...override } as LibraryState
    expect(writeLibraryState(storage, 1, state)).toBe(false)
    expect(storage.setItem).not.toHaveBeenCalled()
    storage.setItem('suidu:library:v1:1', JSON.stringify({ version: 1, userID: 1, state }))
    expect(readLibraryState(storage, 1)).toBeNull()
  })

  it.each([NaN, Infinity, -Infinity])('rejects non-finite scroll and range values: %s', (value) => {
    const storage = memoryStorage()
    expect(writeLibraryState(storage, 1, libraryState({ scrollY: value }))).toBe(false)
    expect(writeLibraryState(storage, 1, libraryState({ range: [0, value] }))).toBe(false)
    expect(storage.setItem).not.toHaveBeenCalled()
  })

  it('handles broken and unavailable storage without throwing', () => {
    const storage = memoryStorage()
    vi.mocked(storage.getItem).mockReturnValue(1 as unknown as string)
    expect(readLibraryState(storage, 1)).toBeNull()
    vi.mocked(storage.getItem).mockImplementation(() => { throw new DOMException('Denied', 'SecurityError') })
    vi.mocked(storage.setItem).mockImplementation(() => { throw new DOMException('Denied', 'SecurityError') })
    expect(readLibraryState(storage, 1)).toBeNull()
    expect(writeLibraryState(storage, 1, libraryState())).toBe(false)
  })

  it('preserves earlier state when quota prevents a write', () => {
    const storage = memoryStorage()
    const previous = libraryState()
    writeLibraryState(storage, 1, previous)
    vi.mocked(storage.setItem).mockImplementation(() => { throw new DOMException('Full', 'QuotaExceededError') })
    expect(writeLibraryState(storage, 1, libraryState({ scrollY: 99 }))).toBe(false)
    expect(readLibraryState(storage, 1)).toEqual(previous)
  })

  it.each([0, -1, 1.5, NaN, Infinity, Number.MAX_SAFE_INTEGER + 1])('rejects invalid user ID %s', (userID) => {
    const storage = memoryStorage()
    expect(readLibraryState(storage, userID)).toBeNull()
    expect(writeLibraryState(storage, userID, libraryState())).toBe(false)
    expect(storage.length).toBe(0)
  })
})
