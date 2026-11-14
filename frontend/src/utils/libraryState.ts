import type { DateRangeValue, TimeFilterPreset } from './timeline'
import { isLibraryBoundary, type LibraryBoundary } from './libraryPagination'

export interface LibraryState {
  query: string
  kind: 'all' | 'text' | 'image' | 'file'
  favorites: boolean
  preset: TimeFilterPreset
  range: DateRangeValue
  scrollY: number
  loadedBoundary?: LibraryBoundary | null
}

function storageKey(userID: number): string | null {
  return Number.isSafeInteger(userID) && userID > 0 ? `suidu:library:v1:${userID}` : null
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function isDateRange(value: unknown): value is DateRangeValue {
  if (value === null) return true
  if (!Array.isArray(value) || value.length !== 2) return false
  return value.every((timestamp: unknown) => typeof timestamp === 'number'
    && Number.isFinite(timestamp)
    && Number.isFinite(new Date(timestamp).getTime()))
    && value[0] <= value[1]
}

function isLibraryState(value: unknown): value is LibraryState {
  if (!isRecord(value)) return false
  return typeof value.query === 'string' && value.query.length <= 200
    && ['all', 'text', 'image', 'file'].includes(value.kind as string)
    && typeof value.favorites === 'boolean'
    && ['all', 'today', 'last7Days', 'last30Days', 'custom'].includes(value.preset as string)
    && isDateRange(value.range)
    && (value.preset !== 'custom' || value.range !== null)
    && typeof value.scrollY === 'number' && Number.isFinite(value.scrollY) && value.scrollY >= 0
    && (value.loadedBoundary === undefined || value.loadedBoundary === null || isLibraryBoundary(value.loadedBoundary))
}

/** Restore only a valid state belonging to this user from the supplied session storage. */
export function readLibraryState(storage: Storage, userID: number): LibraryState | null {
  const address = storageKey(userID)
  if (address === null) return null
  try {
    const raw = storage.getItem(address)
    if (typeof raw !== 'string') return null
    const saved: unknown = JSON.parse(raw)
    if (!isRecord(saved) || saved.version !== 1 || saved.userID !== userID) return null
    return isLibraryState(saved.state) ? saved.state : null
  } catch {
    return null
  }
}

/** Save validated library navigation state without interrupting navigation on storage failure. */
export function writeLibraryState(storage: Storage, userID: number, state: LibraryState): boolean {
  const address = storageKey(userID)
  if (address === null || !isLibraryState(state)) return false
  try {
    storage.setItem(address, JSON.stringify({ version: 1, userID, state }))
    return true
  } catch {
    return false
  }
}
