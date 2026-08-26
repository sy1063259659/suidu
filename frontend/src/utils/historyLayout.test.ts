import { describe, expect, it } from 'vitest'
import { captureResultsHeight, preservedResultsStyle } from './historyLayout'

describe('history result layout preservation', () => {
  it('captures the current height instead of retaining a historical maximum', () => {
    expect(captureResultsHeight(410.2)).toBe(411)
  })

  it('keeps the height while favorites are active or the filter request is pending', () => {
    expect(preservedResultsStyle(1255, true)).toEqual({ minHeight: '1255px' })
  })

  it('releases the height after returning to the full result set', () => {
    expect(preservedResultsStyle(1255, false)).toBeUndefined()
    expect(preservedResultsStyle(0, true)).toBeUndefined()
  })
})
