import { describe, expect, it, vi } from 'vitest'
import { readDisplayMode, writeDisplayMode, type DisplayMode } from './displayPreferences'

function memoryStorage(): Storage {
  const entries = new Map<string, string>()
  return {
    get length() { return entries.size },
    clear: vi.fn(() => entries.clear()),
    key: vi.fn((index: number) => Array.from(entries.keys())[index] ?? null),
    getItem: vi.fn((key: string) => entries.get(key) ?? null),
    setItem: vi.fn((key: string, value: string) => { entries.set(key, value) }),
    removeItem: vi.fn((key: string) => { entries.delete(key) }),
  }
}

describe('per-user library display preferences', () => {
  it('defaults to preview and restores only the current user preference', () => {
    const storage = memoryStorage()
    expect(readDisplayMode(storage, 1)).toBe('preview')
    expect(writeDisplayMode(storage, 1, 'compact')).toBe(true)
    expect(readDisplayMode(storage, 1)).toBe('compact')
    expect(readDisplayMode(storage, 2)).toBe('preview')
    writeDisplayMode(storage, 2, 'preview')
    expect(readDisplayMode(storage, 1)).toBe('compact')
    expect(readDisplayMode(storage, 2)).toBe('preview')
    expect(writeDisplayMode(storage, 1, 'preview')).toBe(true)
    expect(readDisplayMode(storage, 1)).toBe('preview')
  })

  it.each(['invalid', '', 'COMPACT', 'null', '{"mode":"compact"}'])('ignores malformed stored mode %s without modifying storage', (raw) => {
    const storage = memoryStorage()
    vi.mocked(storage.getItem).mockReturnValue(raw)
    expect(readDisplayMode(storage, 1)).toBe('preview')
    expect(storage.setItem).not.toHaveBeenCalled()
    expect(storage.removeItem).not.toHaveBeenCalled()
  })

  it('silently handles unavailable storage and quota failures', () => {
    const storage = memoryStorage()
    vi.mocked(storage.getItem).mockImplementation(() => { throw new Error('denied') })
    vi.mocked(storage.setItem).mockImplementation(() => { throw new Error('full') })
    expect(readDisplayMode(storage, 1)).toBe('preview')
    expect(writeDisplayMode(storage, 1, 'compact')).toBe(false)
  })

  it.each([0, -1, NaN, Infinity, 1.5, Number.MAX_SAFE_INTEGER + 1])('rejects invalid user %s without accessing storage', (userID) => {
    const storage = memoryStorage()
    expect(readDisplayMode(storage, userID)).toBe('preview')
    expect(writeDisplayMode(storage, userID, 'compact')).toBe(false)
    expect(storage.getItem).not.toHaveBeenCalled()
    expect(storage.setItem).not.toHaveBeenCalled()
  })

  it('rejects invalid runtime mode and leaves unrelated values intact', () => {
    const storage = memoryStorage()
    storage.setItem('other-setting', 'keep')
    expect(writeDisplayMode(storage, 1, 'invalid' as DisplayMode)).toBe(false)
    expect(storage.getItem('other-setting')).toBe('keep')
    expect(storage.clear).not.toHaveBeenCalled()
    expect(storage.removeItem).not.toHaveBeenCalled()
  })
})
