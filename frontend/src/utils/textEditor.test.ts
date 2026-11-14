import { describe, expect, it } from 'vitest'
import { MAX_TEXT_BYTES, textStats } from './textEditor'

describe('text editor size validation', () => {
  it('counts Unicode characters separately from UTF-8 bytes', () => {
    expect(textStats('中🙂\n ')).toEqual({ characters: 4, bytes: 9 })
  })

  it('accepts the exact byte limit and detects overflow without truncation', () => {
    const exact = 'x'.repeat(MAX_TEXT_BYTES)
    expect(textStats(exact).bytes).toBe(MAX_TEXT_BYTES)
    expect(textStats(exact + '尾').bytes).toBe(MAX_TEXT_BYTES + 3)
    expect(textStats('中'.repeat(350000)).bytes).toBeGreaterThan(MAX_TEXT_BYTES)
  })
})
