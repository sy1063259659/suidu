import { describe, expect, it, vi } from 'vitest'
import type { ClipboardItem, ClipboardPage } from '../api/clipboard'
import { canApplyLibraryRequest, compareLibraryItems, fetchLibraryRange, libraryCursorForBoundary, newlySyncedLibraryIds, oldestLibraryBoundary, uniqueLibraryItems } from './libraryPagination'

function item(id: number, createdAt = '2026-11-14T12:00:00.123456789Z'): ClipboardItem {
  return { id, createdAt, kind: 'text', source: 'test', content: String(id) }
}
function page(items: ClipboardItem[], nextCursor = '', hasMore = false): ClipboardPage { return { items, nextCursor, hasMore } }
function pagedFetch(items: ClipboardItem[]) {
  return vi.fn(async (cursor?: string) => {
    const after = cursor ? Number(atob(cursor.replace(/-/g, '+').replace(/_/g, '/')).split('\n')[1]) : Infinity
    const remaining = items.filter(entry => entry.id < after)
    const entries = remaining.slice(0, 50)
    return page(entries, remaining.length > 50 ? libraryCursorForBoundary(entries.at(-1)!) : '', remaining.length > 50)
  })
}

describe('library pagination', () => {
  it('allows foreground filter/initial loads after draft edits while protecting background and append requests', () => {
    const started = { mutation: 2, activity: 5 }
    const afterTyping = { mutation: 2, activity: 6 }
    expect(canApplyLibraryRequest(started, afterTyping, false)).toBe(true)
    expect(canApplyLibraryRequest(started, afterTyping, true)).toBe(false)
    const afterMutation = { mutation: 3, activity: 5 }
    expect(canApplyLibraryRequest(started, afterMutation, false)).toBe(false)
    expect(canApplyLibraryRequest(started, afterMutation, true)).toBe(false)
  })

  it('orders fractional timestamps with nanosecond precision before comparing IDs', () => {
    const earlier = item(100, '2026-11-14T12:00:00.123456788Z')
    const later = item(1, '2026-11-14T12:00:00.123456789Z')
    expect(uniqueLibraryItems([earlier, later]).map(entry => entry.id)).toEqual([1, 100])
    expect(compareLibraryItems(item(2), item(1))).toBeLessThan(0)
    expect(compareLibraryItems(item(1, '2026-11-14T20:00:00.123456789+08:00'), later)).toBe(0)
  })

  it('encodes the backend exact timestamp cursor without losing nanoseconds', () => {
    expect(atob(libraryCursorForBoundary(item(12)))).toBe('2026-11-14T12:00:00.123456789Z\n12')
    expect(atob(libraryCursorForBoundary(item(12, '2026-11-14T20:00:00.120000000+08:00')))).toBe('2026-11-14T12:00:00.12Z\n12')
  })

  it('refreshes through a deleted oldest boundary and keeps overflow available to load more', async () => {
    const all = Array.from({ length: 160 }, (_, index) => item(200 - index)).filter(entry => entry.id !== 101)
    const fetch = pagedFetch(all)
    const result = await fetchLibraryRange(fetch, item(101))
    expect(fetch).toHaveBeenCalledTimes(2)
    expect(result?.items).toHaveLength(99)
    expect(result?.items.at(-1)?.id).toBe(102)
    expect(result?.hasMore).toBe(true)
    expect(result?.nextCursor).toBe(libraryCursorForBoundary(item(101)))
    expect((await fetch(result?.nextCursor)).items[0]?.id).toBe(100)
    expect(oldestLibraryBoundary(result!.items, item(101))?.id).toBe(101)
  })

  it('does not grow the loaded range by full pages after repeated remote prepends', async () => {
    let all = Array.from({ length: 250 }, (_, index) => item(250 - index))
    const boundary = item(151)
    for (let index = 1; index <= 6; index++) {
      all = [item(250 + index), ...all]
      const result = await fetchLibraryRange(pagedFetch(all), boundary)
      expect(result?.items).toHaveLength(100 + index)
      expect(result?.items.at(-1)?.id).toBe(151)
      expect(result?.nextCursor).toBe(libraryCursorForBoundary(boundary))
    }
  })

  it('deduplicates overlapping pages, stages atomically, and propagates a failed later page', async () => {
    const fetch = vi.fn().mockResolvedValueOnce(page([item(3), item(2)], 'next', true))
      .mockResolvedValueOnce(page([item(2), item(1)]))
    expect((await fetchLibraryRange(fetch, item(1)))?.items.map(entry => entry.id)).toEqual([3, 2, 1])
    const fail = vi.fn().mockResolvedValueOnce(page([item(3)], 'next', true)).mockRejectedValueOnce(new Error('offline'))
    await expect(fetchLibraryRange(fail, item(1))).rejects.toThrow('offline')
  })

  it('stops staging invalidated responses and rejects looping continuations', async () => {
    const fetch = vi.fn(async () => page([item(3)], 'next', true))
    expect(await fetchLibraryRange(fetch, item(1), () => false)).toBeNull()
    expect(fetch).toHaveBeenCalledTimes(1)
    await expect(fetchLibraryRange(fetch, item(1))).rejects.toThrow('continuation')
  })

  it('announces only remote arrivals above the previous newest item', () => {
    expect(newlySyncedLibraryIds([item(5), item(4)], [item(6), item(5), item(4), item(3)])).toEqual([6])
    expect(newlySyncedLibraryIds([item(6), item(5)], [item(6), item(5)])).toEqual([])
  })

  it('announces the first remote arrival after an established empty baseline', () => {
    expect(newlySyncedLibraryIds([], [item(8), item(8)])).toEqual([8])
    expect(newlySyncedLibraryIds([], [])).toEqual([])
  })

  it('never announces this session’s local creations after a delayed or failed filtered refresh', () => {
    const local = new Set([8])
    expect(newlySyncedLibraryIds([item(5)], [item(9), item(8), item(5)], local)).toEqual([9])
    expect(newlySyncedLibraryIds([], [item(9), item(8)], local)).toEqual([9])
    expect(newlySyncedLibraryIds([], [item(8)], local)).toEqual([])
  })
})
