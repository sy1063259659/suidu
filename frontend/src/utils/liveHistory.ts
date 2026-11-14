import type { ClipboardItem } from '../api/clipboard'

// Keep unchanged cards intact, including image previews and text selection.
export function reconcileItems(current: ClipboardItem[], incoming: ClipboardItem[]) {
  const previous = new Map(current.map(item => [item.id, item]))
  return incoming.map(item => {
    const old = previous.get(item.id)
    return old && JSON.stringify(old) === JSON.stringify(item) ? old : item
  })
}

export interface HistoryAnchor { id: string; top: number }

export function captureHistoryAnchors(root: HTMLElement): HistoryAnchor[] {
  if (window.scrollY <= 0) return []
  return Array.from(root.querySelectorAll<HTMLElement>('[data-item-id]')).flatMap(element => {
    const bounds = element.getBoundingClientRect()
    return bounds.bottom > 0 && bounds.top < window.innerHeight
      ? [{ id: element.dataset.itemId!, top: bounds.top }] : []
  })
}

export function restoreHistoryAnchor(root: HTMLElement, anchors: HistoryAnchor[]) {
  const elements = Array.from(root.querySelectorAll<HTMLElement>('[data-item-id]'))
  for (const anchor of anchors) {
    const element = elements.find(candidate => candidate.dataset.itemId === anchor.id)
    if (!element) continue
    const delta = element.getBoundingClientRect().top - anchor.top
    if (Math.abs(delta) > 1) window.scrollBy({ top: delta, behavior: 'instant' })
    break
  }
}
