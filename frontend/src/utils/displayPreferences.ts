export type DisplayMode = 'preview' | 'compact'

const DISPLAY_PREFIX = 'suidu:display:v1:'

function storageKey(userID: number): string | null {
  return Number.isSafeInteger(userID) && userID > 0 ? `${DISPLAY_PREFIX}${userID}` : null
}

export function readDisplayMode(storage: Storage, userID: number): DisplayMode {
  const key = storageKey(userID)
  if (!key) return 'preview'
  try {
    const mode = storage.getItem(key)
    return mode === 'compact' ? mode : 'preview'
  } catch {
    return 'preview'
  }
}

export function writeDisplayMode(storage: Storage, userID: number, mode: DisplayMode): boolean {
  const key = storageKey(userID)
  if (!key || (mode !== 'preview' && mode !== 'compact')) return false
  try {
    storage.setItem(key, mode)
    return true
  } catch {
    return false
  }
}
