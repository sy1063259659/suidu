import { describe, expect, it, vi } from 'vitest'
import { clearDraft, readDraft, readStoredDraft, writeDraft } from './draftStorage'

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

describe('per-user draft storage', () => {
  it('restores exact whitespace, line endings, Unicode, and drafts larger than 1 MiB', () => {
    const storage = memoryStorage()
    const content = ` \r\n\t中🙂${'x'.repeat(1024 * 1024 + 1)}\n  `
    expect(writeDraft(storage, 1, content)).toBe(true)
    expect(readDraft(storage, 1)).toBe(content)
    expect(writeDraft(storage, 1, ' \t\r\n ')).toBe(true)
    expect(readDraft(storage, 1)).toBe(' \t\r\n ')
  })

  it('isolates users and separate new/edit drafts', () => {
    const storage = memoryStorage()
    writeDraft(storage, 1, 'first user')
    writeDraft(storage, 2, 'second user')
    writeDraft(storage, 1, 'editing first', 'item:3')
    writeDraft(storage, 1, 'editing second', 'item:4')
    expect(readDraft(storage, 1)).toBe('first user')
    expect(readDraft(storage, 2)).toBe('second user')
    expect(readDraft(storage, 1, 'item:3')).toBe('editing first')
    expect(readDraft(storage, 1, 'item:4')).toBe('editing second')
    expect(readDraft(storage, 3)).toBe('')
    expect(storage.getItem('suidu:draft:v1:1:new')).not.toBeNull()
  })

  it('removes only the new draft when emptied or an addressed draft when explicitly cleared', () => {
    const storage = memoryStorage()
    storage.setItem('unrelated', 'keep')
    writeDraft(storage, 1, 'new')
    writeDraft(storage, 1, 'edit', 'item:3')
    writeDraft(storage, 2, 'other user')
    expect(writeDraft(storage, 1, '')).toBe(true)
    expect(storage.getItem('suidu:draft:v1:1:new')).toBeNull()
    expect(clearDraft(storage, 1, 'item:3')).toBe(true)
    expect(storage.getItem('suidu:draft:v1:1:item:3')).toBeNull()
    expect(readDraft(storage, 2)).toBe('other user')
    expect(storage.getItem('unrelated')).toBe('keep')
    expect(storage.clear).not.toHaveBeenCalled()
  })

  it('recovers empty edits while distinguishing missing drafts and isolating users', () => {
    const storage = memoryStorage()
    expect(readStoredDraft(storage, 1, 'item:3')).toBeNull()
    expect(writeDraft(storage, 1, 'previous text', 'item:3')).toBe(true)
    expect(writeDraft(storage, 1, '', 'item:3')).toBe(true)
    expect(readStoredDraft(storage, 1, 'item:3')).toBe('')
    expect(readStoredDraft(storage, 1, 'item:3') ?? 'server text').toBe('')
    expect(readStoredDraft(storage, 2, 'item:3')).toBeNull()
    expect(storage.removeItem).not.toHaveBeenCalled()
    expect(clearDraft(storage, 1, 'item:3')).toBe(true)
    expect(readStoredDraft(storage, 1, 'item:3')).toBeNull()
    expect(readStoredDraft(storage, 1, 'item:3') ?? 'server text').toBe('server text')
  })

  it.each([
    'not JSON', 'null', '[]', '"raw text"', '{}',
    JSON.stringify({ version: 2, userID: 1, key: 'new', content: 'text' }),
    JSON.stringify({ version: 1, userID: 2, key: 'new', content: 'other user' }),
    JSON.stringify({ version: 1, userID: 1, key: 'item:3', content: 'other draft' }),
    JSON.stringify({ version: 1, userID: 1, key: 'new', content: 42 }),
    JSON.stringify({ version: 1, userID: 1, key: 'new', content: null }),
  ])('ignores malformed or mismatched data without mutating it: %s', (raw) => {
    const storage = memoryStorage()
    storage.setItem('suidu:draft:v1:1:new', raw)
    expect(readStoredDraft(storage, 1)).toBeNull()
    expect(readDraft(storage, 1)).toBe('')
    expect(storage.getItem('suidu:draft:v1:1:new')).toBe(raw)
    expect(storage.removeItem).not.toHaveBeenCalled()
  })

  it('ignores a non-string value from a broken storage implementation', () => {
    const storage = memoryStorage()
    vi.mocked(storage.getItem).mockReturnValue(123 as unknown as string)
    expect(readStoredDraft(storage, 1)).toBeNull()
    expect(readDraft(storage, 1)).toBe('')
  })

  it('reports quota failure and preserves the previously stored draft', () => {
    const storage = memoryStorage()
    writeDraft(storage, 1, 'saved earlier')
    vi.mocked(storage.setItem).mockImplementation(() => {
      throw new DOMException('Storage full', 'QuotaExceededError')
    })
    expect(writeDraft(storage, 1, 'unsaved replacement')).toBe(false)
    expect(readDraft(storage, 1)).toBe('saved earlier')
  })

  it('handles SecurityError from reads, writes, and removals', () => {
    const storage = memoryStorage()
    const denied = () => { throw new DOMException('Access denied', 'SecurityError') }
    vi.mocked(storage.getItem).mockImplementation(denied)
    vi.mocked(storage.setItem).mockImplementation(denied)
    vi.mocked(storage.removeItem).mockImplementation(denied)
    expect(readStoredDraft(storage, 1)).toBeNull()
    expect(readDraft(storage, 1)).toBe('')
    expect(writeDraft(storage, 1, 'draft')).toBe(false)
    expect(writeDraft(storage, 1, '')).toBe(false)
    expect(writeDraft(storage, 1, '', 'item:3')).toBe(false)
    expect(clearDraft(storage, 1)).toBe(false)
  })

  it.each([0, -1, NaN, Infinity, 1.5, Number.MAX_SAFE_INTEGER + 1])('rejects invalid user ID %s', (userID) => {
    const storage = memoryStorage()
    expect(readDraft(storage, userID)).toBe('')
    expect(writeDraft(storage, userID, 'text')).toBe(false)
    expect(clearDraft(storage, userID)).toBe(false)
    expect(storage.length).toBe(0)
  })

  it.each(['', '1:new', 'item:0', 'item:-1', 'item:01', 'item:1.5', 'item:9007199254740992'])('rejects invalid draft key %s', (key) => {
    const storage = memoryStorage()
    expect(readDraft(storage, 1, key)).toBe('')
    expect(writeDraft(storage, 1, 'text', key)).toBe(false)
    expect(clearDraft(storage, 1, key)).toBe(false)
    expect(storage.length).toBe(0)
  })
})
