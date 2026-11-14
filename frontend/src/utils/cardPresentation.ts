import type { ClipboardItem } from '../api/clipboard'

export interface CardPresentation {
  title: string
  note: string
  summary: string
  typeLabel: string
  fileSize: string
}

export function formatCardFileSize(value = 0): string {
  const size = Number.isFinite(value) && value > 0 ? value : 0
  if (size < 1024) return `${size} B`
  if (size < 1024 ** 2) return `${(size / 1024).toFixed(1)} KiB`
  if (size < 1024 ** 3) return `${(size / 1024 ** 2).toFixed(1)} MiB`
  return `${(size / 1024 ** 3).toFixed(1)} GiB`
}

/** Derive display strings without modifying the content used by copy/edit/detail. */
export function getCardPresentation(item: ClipboardItem): CardPresentation {
  const noteLines = (item.note ?? '').split(/\r\n|\n|\r/)
  const titleIndex = noteLines.findIndex((line) => line.trim().length > 0)
  const firstContentLine = (item.content ?? '').split(/\r\n|\n|\r/).find((line) => line.trim())?.trim()
  const typeLabel = item.kind === 'image' ? '图片' : item.kind === 'file' ? '文件' : '文本'
  const fallback = item.kind === 'text' || !item.kind ? firstContentLine || '未命名文本' : item.fileName?.trim() || `未命名${typeLabel}`
  return {
    title: titleIndex >= 0 ? noteLines[titleIndex]!.trim() : fallback,
    note: titleIndex >= 0 ? noteLines.slice(titleIndex + 1).join('\n').trim() : '',
    summary: (item.content ?? '').replace(/\s+/g, ' ').trim() || '空文本',
    typeLabel,
    fileSize: formatCardFileSize(item.sizeBytes),
  }
}
