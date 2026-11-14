import type { ClipboardItem, ClipboardPage } from '../api/clipboard'

export interface LibraryBoundary { id: number; createdAt: string }

export interface LibraryRequestEpoch { mutation: number; activity: number }

/** Foreground loads do not alter editor drafts, so typing only blocks background/append updates. */
export function canApplyLibraryRequest(started: LibraryRequestEpoch, current: LibraryRequestEpoch, protectEdits: boolean): boolean {
  return started.mutation === current.mutation && (!protectEdits || started.activity === current.activity)
}

/** Separate whole seconds from the fractional part so nanoseconds never pass through Date. */
function timestampParts(timestamp: string): [number, string] {
  const match = /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,9}))?(Z|[+-]\d{2}:\d{2})$/.exec(timestamp)
  if (!match) throw new Error('Invalid clipboard timestamp')
  const seconds = Date.parse(`${match[1]}${match[3]}`) / 1000
  if (!Number.isFinite(seconds)) throw new Error('Invalid clipboard timestamp')
  return [seconds, (match[2] ?? '').padEnd(9, '0')]
}

export function isLibraryBoundary(value: unknown): value is LibraryBoundary {
  if (!value || typeof value !== 'object') return false
  const boundary = value as LibraryBoundary
  if (!Number.isSafeInteger(boundary.id) || boundary.id <= 0 || typeof boundary.createdAt !== 'string') return false
  try { timestampParts(boundary.createdAt); return true } catch { return false }
}

/** Negative means a comes first in the server's newest-first timestamp/ID order. */
export function compareLibraryItems(a: LibraryBoundary, b: LibraryBoundary): number {
  const [aSeconds, aFraction] = timestampParts(a.createdAt)
  const [bSeconds, bFraction] = timestampParts(b.createdAt)
  if (aSeconds !== bSeconds) return bSeconds - aSeconds
  if (aFraction !== bFraction) return aFraction > bFraction ? -1 : 1
  return b.id - a.id
}

export function uniqueLibraryItems(items: ClipboardItem[]): ClipboardItem[] {
  return Array.from(new Map(items.map(item => [item.id, item])).values()).sort(compareLibraryItems)
}

export function oldestLibraryBoundary(items: ClipboardItem[], previous: LibraryBoundary | null = null): LibraryBoundary | null {
  const oldest = items.reduce<LibraryBoundary | null>((boundary, item) => !boundary || compareLibraryItems(item, boundary) > 0
    ? { id: item.id, createdAt: item.createdAt } : boundary, previous)
  return oldest
}

/** Match the server's RFC3339Nano/newline/ID token without rounding timestamp fractions. */
export function libraryCursorForBoundary(boundary: LibraryBoundary): string {
  if (!isLibraryBoundary(boundary)) throw new Error('Invalid clipboard boundary')
  const [seconds, fraction] = timestampParts(boundary.createdAt)
  const canonicalFraction = fraction.replace(/0+$/, '')
  const timestamp = new Date(seconds * 1000).toISOString().replace('.000Z', '')
    + (canonicalFraction ? `.${canonicalFraction}` : '') + 'Z'
  return btoa(`${timestamp}\n${boundary.id}`).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

/** Stage a complete range before applying it, including when its oldest row was deleted. */
export async function fetchLibraryRange(
  fetchPage: (cursor?: string) => Promise<ClipboardPage>,
  boundary: LibraryBoundary | null,
  isCurrent: () => boolean = () => true,
): Promise<ClipboardPage | null> {
  const staged: ClipboardItem[] = []
  const seenCursors = new Set<string>()
  let cursor: string | undefined
  for (;;) {
    const page = await fetchPage(cursor)
    if (!isCurrent()) return null
    staged.push(...page.items)
    const oldest = oldestLibraryBoundary(page.items)
    if (!page.hasMore || !boundary || (oldest && compareLibraryItems(oldest, boundary) >= 0)) {
      const all = uniqueLibraryItems(staged)
      const retained = boundary ? all.filter(item => compareLibraryItems(item, boundary) <= 0) : all
      const hasMore = page.hasMore || retained.length < all.length
      return { items: retained,
        nextCursor: hasMore && boundary ? libraryCursorForBoundary(boundary) : page.nextCursor,
        hasMore }
    }
    if (!page.nextCursor || seenCursors.has(page.nextCursor)) throw new Error('Invalid clipboard continuation')
    seenCursors.add(page.nextCursor)
    cursor = page.nextCursor
  }
}

export function newlySyncedLibraryIds(previous: ClipboardItem[], incoming: ClipboardItem[], locallyCreatedIds: ReadonlySet<number> = new Set()): number[] {
  const newest = uniqueLibraryItems(previous)[0]
  if (!newest) return uniqueLibraryItems(incoming).filter(item => !locallyCreatedIds.has(item.id)).map(item => item.id)
  const known = new Set(previous.map(item => item.id))
  return incoming.filter(item => !known.has(item.id) && !locallyCreatedIds.has(item.id) && compareLibraryItems(item, newest) < 0).map(item => item.id)
}
