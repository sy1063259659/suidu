import { describe, expect, it } from 'vitest'
import {
  applyCustomTimeFilter,
  applyTimePreset,
  resolveTimeRange,
  toClipboardTimeParams,
} from './timeline'

describe('timeline time filters', () => {
  it('builds a today range from the local calendar day', () => {
    const now = new Date(2026, 7, 26, 15, 45, 12, 300)

    const range = resolveTimeRange('today', null, now)

    expect(range).not.toBeNull()
    expect(range?.from.getTime()).toBe(new Date(2026, 7, 26).getTime())
    expect(range?.to.getTime()).toBe(new Date(2026, 7, 27).getTime())
  })

  it('keeps the recent seven days inclusive of today and the previous six full days', () => {
    const now = new Date(2026, 7, 26, 8, 5)

    const range = resolveTimeRange('last7Days', null, now)

    expect(range).not.toBeNull()
    expect(range?.from.getTime()).toBe(new Date(2026, 7, 20).getTime())
    expect(range?.to.getTime()).toBe(new Date(2026, 7, 27).getTime())
  })

  it('keeps the recent thirty days inclusive of today and the previous twenty-nine full days', () => {
    const now = new Date(2026, 7, 26, 23, 59, 59)

    const range = resolveTimeRange('last30Days', null, now)

    expect(range).not.toBeNull()
    expect(range?.from.getTime()).toBe(new Date(2026, 6, 28).getTime())
    expect(range?.to.getTime()).toBe(new Date(2026, 7, 27).getTime())
  })

  it('turns a custom inclusive local date range into an exclusive upper API bound', () => {
    const now = new Date(2026, 7, 26, 9, 0)
    const pickerValue: [number, number] = [
      new Date(2026, 7, 12, 18, 30).getTime(),
      new Date(2026, 7, 15, 6, 45).getTime(),
    ]

    const range = resolveTimeRange('custom', pickerValue, now)
    const params = toClipboardTimeParams(range)

    expect(range).not.toBeNull()
    expect(range?.from.getTime()).toBe(new Date(2026, 7, 12).getTime())
    expect(range?.to.getTime()).toBe(new Date(2026, 7, 16).getTime())
    expect(new Date(params.createdFrom ?? '').getTime()).toBe(new Date(2026, 7, 12).getTime())
    expect(new Date(params.createdBefore ?? '').getTime()).toBe(new Date(2026, 7, 16).getTime())
  })

  it('resets custom selection when a preset is chosen', () => {
    expect(applyTimePreset('last30Days')).toEqual({
      preset: 'last30Days',
      customRange: null,
    })
  })

  it('marks a completed custom range as custom and clears back to all time', () => {
    const pickerValue: [number, number] = [
      new Date(2026, 7, 12).getTime(),
      new Date(2026, 7, 15).getTime(),
    ]

    expect(applyCustomTimeFilter(pickerValue)).toEqual({
      preset: 'custom',
      customRange: pickerValue,
    })
    expect(applyCustomTimeFilter(null)).toEqual({
      preset: 'all',
      customRange: null,
    })
  })
})
