export type TimeFilterPreset = 'all' | 'today' | 'last7Days' | 'last30Days' | 'custom'

export type DateRangeValue = [number, number] | null

export interface TimeRange {
  from: Date
  to: Date
}

export interface TimeFilterState {
  preset: TimeFilterPreset
  customRange: DateRangeValue
}

export interface ClipboardTimeParams {
  createdFrom?: string
  createdBefore?: string
}

function startOfLocalDay(value: Date): Date {
  return new Date(value.getFullYear(), value.getMonth(), value.getDate())
}

function addLocalDays(value: Date, days: number): Date {
  return new Date(value.getFullYear(), value.getMonth(), value.getDate() + days)
}

function presetRange(preset: Exclude<TimeFilterPreset, 'all' | 'custom'>, now: Date): TimeRange {
  const today = startOfLocalDay(now)
  if (preset === 'today') return { from: today, to: addLocalDays(today, 1) }
  if (preset === 'last7Days') return { from: addLocalDays(today, -6), to: addLocalDays(today, 1) }
  return { from: addLocalDays(today, -29), to: addLocalDays(today, 1) }
}

function customRange(value: DateRangeValue): TimeRange | null {
  if (!value) return null
  const [start, end] = value
  const from = startOfLocalDay(new Date(start))
  const to = addLocalDays(startOfLocalDay(new Date(end)), 1)
  return { from, to }
}

export function resolveTimeRange(preset: TimeFilterPreset, value: DateRangeValue, now: Date): TimeRange | null {
  if (preset === 'all') return null
  if (preset === 'custom') return customRange(value)
  return presetRange(preset, now)
}

export function toClipboardTimeParams(range: TimeRange | null): ClipboardTimeParams {
  if (!range) return {}
  return {
    createdFrom: range.from.toISOString(),
    createdBefore: range.to.toISOString(),
  }
}

export function applyTimePreset(preset: Exclude<TimeFilterPreset, 'custom'>): TimeFilterState {
  return {
    preset,
    customRange: null,
  }
}

export function applyCustomTimeFilter(value: DateRangeValue): TimeFilterState {
  if (!value) return { preset: 'all', customRange: null }
  return {
    preset: 'custom',
    customRange: value,
  }
}
