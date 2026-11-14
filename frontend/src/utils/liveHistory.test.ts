import { describe, expect, it } from 'vitest'
import { reconcileItems } from './liveHistory'
import type { ClipboardItem } from '../api/clipboard'

const item: ClipboardItem = { id: 1, kind: 'text', content: 'keep\n  whitespace', source: 'web', createdAt: '2026-01-01', tags: ['a'], favorite: true }
describe('live library reconciliation', () => {
  it('keeps unchanged card identities when a new item arrives', () => {
    const added = { ...item, id: 2 }
    const result = reconcileItems([item], [added, JSON.parse(JSON.stringify(item))])
    expect(result[0]).toBe(added)
    expect(result[1]).toBe(item)
  })
  it('applies edits, favorite changes, tags and removals without retaining stale data', () => {
    for (const change of [{ content: 'edited' }, { favorite: false }, { tags: ['b'] }, { note: 'new note' }]) {
      const updated = { ...item, ...change }
      expect(reconcileItems([item], [updated])[0]).toBe(updated)
    }
    expect(reconcileItems([item], [])).toEqual([])
  })
})
