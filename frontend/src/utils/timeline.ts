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

export interface TimelineGroup<T extends { createdAt: string }> {
  dateKey: string
  label: string
  count: number
  items: T[]
}

const WEEKDAY_LABELS = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'] as const

function startOfLocalDay(value: Date): Date {
  return new Date(value.getFullYear(), value.getMonth(), value.getDate())
}

function addLocalDays(value: Date, days: number): Date {
  return new Date(value.getFullYear(), value.getMonth(), value.getDate() + days)
}

function isSameLocalDay(left: Date, right: Date): boolean {
  return left.getFullYear() === right.getFullYear()
    && left.getMonth() === right.getMonth()
    && left.getDate() === right.getDate()
}

function localDateKey(value: Date): string {
  const year = String(value.getFullYear())
  const month = String(value.getMonth() + 1).padStart(2, '0')
  const day = String(value.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function formatFullLocalDate(value: Date): string {
  return `${value.getFullYear()}年${value.getMonth() + 1}月${value.getDate()}日 ${WEEKDAY_LABELS[value.getDay()]}`
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

export function formatTimelineGroupLabel(value: Date, now: Date): string {
  const groupDay = startOfLocalDay(value)
  const today = startOfLocalDay(now)
  if (isSameLocalDay(groupDay, today)) return '今天'
  if (isSameLocalDay(groupDay, addLocalDays(today, -1))) return '昨天'
  return formatFullLocalDate(groupDay)
}

export function groupTimelineItemsByLocalDay<T extends { createdAt: string }>(items: T[], now: Date): Array<TimelineGroup<T>> {
  const grouped = new Map<string, TimelineGroup<T>>()
  const groupOrder: string[] = []

  for (const item of items) {
    const groupDate = startOfLocalDay(new Date(item.createdAt))
    const dateKey = localDateKey(groupDate)
    const existingGroup = grouped.get(dateKey)

    if (existingGroup) {
      existingGroup.items.push(item)
      existingGroup.count += 1
      continue
    }

    grouped.set(dateKey, {
      dateKey,
      label: formatTimelineGroupLabel(groupDate, now),
      count: 1,
      items: [item],
    })
    groupOrder.push(dateKey)
  }

  return groupOrder.map((dateKey) => grouped.get(dateKey)!)
}
