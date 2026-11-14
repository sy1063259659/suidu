const DRAFT_PREFIX = 'suidu:draft:v1:'

interface StoredDraft {
  version: 1
  userID: number
  key: string
  content: string
}

function storageKey(userID: number, key: string): string | null {
  if (!Number.isSafeInteger(userID) || userID <= 0) return null
  if (key !== 'new') {
    if (!/^item:[1-9]\d*$/.test(key)) return null
    if (!Number.isSafeInteger(Number(key.slice(5)))) return null
  }
  return `${DRAFT_PREFIX}${userID}:${key}`
}

/** Distinguish an intentionally empty edit draft from a missing or invalid saved draft. */
export function readStoredDraft(storage: Storage, userID: number, key = 'new'): string | null {
  const address = storageKey(userID, key)
  if (address === null) return null
  try {
    const raw = storage.getItem(address)
    if (typeof raw !== 'string') return null
    const draft: unknown = JSON.parse(raw)
    if (typeof draft !== 'object' || draft === null || Array.isArray(draft)) return null
    const record = draft as Partial<StoredDraft>
    if (record.version !== 1 || record.userID !== userID || record.key !== key) return null
    return typeof record.content === 'string' ? record.content : null
  } catch {
    return null
  }
}

/** Read the composer draft with an empty editor fallback. */
export function readDraft(storage: Storage, userID: number, key = 'new'): string {
  return readStoredDraft(storage, userID, key) ?? ''
}

/** Preserve exact text, including empty edit drafts; an empty new composer clears its draft. */
export function writeDraft(storage: Storage, userID: number, content: string, key = 'new'): boolean {
  const address = storageKey(userID, key)
  if (address === null || typeof content !== 'string') return false
  if (content === '' && key === 'new') return clearDraft(storage, userID, key)
  try {
    const draft: StoredDraft = { version: 1, userID, key, content }
    storage.setItem(address, JSON.stringify(draft))
    return true
  } catch {
    return false
  }
}

/** Clear only the specified user's specified draft, leaving every other entry intact. */
export function clearDraft(storage: Storage, userID: number, key = 'new'): boolean {
  const address = storageKey(userID, key)
  if (address === null) return false
  try {
    storage.removeItem(address)
    return true
  } catch {
    return false
  }
}
