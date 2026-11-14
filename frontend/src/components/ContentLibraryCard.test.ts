import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ClipboardItem } from '../api/clipboard'

vi.mock('./ItemActions.vue', () => ({
  default: defineComponent({
    props: ['item', 'copied', 'favoriteUpdating'],
    emits: ['copy', 'favorite', 'edit', 'organize', 'share', 'trash'],
    setup(props, { emit }) {
      // Exercise the parent's event forwarding while rendering the stub.
      for (const event of ['copy', 'favorite', 'edit', 'organize', 'share', 'trash'] as const) emit(event, props.item)
      return () => h('div', { class: 'item-actions', 'data-copied': props.copied, 'data-favorite-updating': props.favoriteUpdating },
        ['copy', 'favorite', 'edit', 'organize', 'share', 'trash'].map((event) => h('button', { 'data-action': event, onClick: () => emit(event as 'copy', props.item) }, event)))
    },
  }),
}))
vi.mock('./ClipboardItemContent.vue', () => ({
  default: defineComponent({
    props: ['item', 'preview'], emits: ['viewDetail'],
    setup(props, { emit }) {
      return () => h('div', { class: 'rich-preview', 'data-content': props.item.content, 'data-preview': props.preview, onClick: () => emit('viewDetail') }, props.item.content)
    },
  }),
}))

import ContentLibraryCard from './ContentLibraryCard.vue'

const item: ClipboardItem = { id: 7, kind: 'text', content: ' \n**完整原文**\n\t下一行 ', note: '\n我的标题\n说明第一行\n说明第二行\n说明第三行', tags: ['工作', '链接'], source: 'web', createdAt: '2026-08-27T08:00:00Z' }

async function cardHandlers() {
  // The existing Vitest configuration compiles SFCs for SSR. Capture the real
  // setup handlers in an SSR context to check keyboard/selection behavior.
  const emit = vi.fn()
  let bindings: Record<string, any> = {}
  await renderToString(createSSRApp(defineComponent({
    setup() {
      bindings = (ContentLibraryCard as any).setup({ item, mode: 'compact' }, { emit, expose() {} })
      return () => h('div')
    },
  })))
  return { bindings, emit }
}

afterEach(() => vi.unstubAllGlobals())

describe('ContentLibraryCard', () => {
  it('renders note title, remaining note, item anchor and original rich content in preview mode', async () => {
    const html = await renderToString(createSSRApp({ render: () => h(ContentLibraryCard, { item, mode: 'preview', copied: true, favoriteUpdating: true, highlighted: true }) }))
    expect(html).toContain('data-item-id="7"')
    expect(html).toContain('is-highlighted')
    expect(html).toContain('查看详情：我的标题')
    expect(html).toContain('说明第一行\n说明第二行\n说明第三行')
    expect(html).toContain('rich-preview')
    expect(html).toContain('**完整原文**')
    expect(html).toContain('data-copied="true"')
    expect(html).toContain('data-favorite-updating="true"')
  })

  it('renders compact text safely and compact images as small attachment summaries', async () => {
    const text = await renderToString(createSSRApp({ render: () => h(ContentLibraryCard, { item: { ...item, content: '<script>alert(1)</script>' }, mode: 'compact' }) }))
    expect(text).not.toContain('rich-preview')
    expect(text).not.toContain('<script>')
    expect(text).toContain('&lt;script&gt;')
    const image = await renderToString(createSSRApp({ render: () => h(ContentLibraryCard, { item: { ...item, kind: 'image', fileName: '截图.png', mediaType: 'image/png', sizeBytes: 2048 }, mode: 'compact' }) }))
    expect(image).toContain('library-card-thumbnail')
    expect(image).toContain('/api/clipboard/7/content')
    expect(image).toContain('截图.png')
    expect(image).toContain('2.0 KiB')
    expect(image).toContain('image/png')
  })

  it('opens titles and summaries with Enter or Space without intercepting other keys', async () => {
    const { bindings, emit } = await cardHandlers()
    const title = {}
    const preventDefault = vi.fn(), stopPropagation = vi.fn()
    bindings.openWithKeyboard({ key: 'Enter', target: title, currentTarget: title, preventDefault, stopPropagation })
    bindings.openWithKeyboard({ key: ' ', target: title, currentTarget: title, preventDefault, stopPropagation })
    bindings.openWithKeyboard({ key: 'Escape', target: title, currentTarget: title, preventDefault, stopPropagation })
    bindings.openWithKeyboard({ key: 'Enter', target: {}, currentTarget: title, preventDefault, stopPropagation })
    expect(emit).toHaveBeenCalledTimes(2)
    expect(emit).toHaveBeenLastCalledWith('viewDetail', item)
    expect(preventDefault).toHaveBeenCalledTimes(2)
  })

  it('does not open detail on interactive descendants or while text is selected', async () => {
    const { bindings, emit } = await cardHandlers()
    bindings.openCard({ target: { closest: () => ({}) } })
    expect(emit).not.toHaveBeenCalled()
    vi.stubGlobal('window', { getSelection: () => ({ toString: () => 'selected text' }) })
    bindings.openCard({ target: { closest: () => null } })
    expect(emit).not.toHaveBeenCalled()
    vi.stubGlobal('window', { getSelection: () => null })
    bindings.openCard({ target: { closest: () => null } })
    expect(emit).toHaveBeenCalledWith('viewDetail', item)
  })

  it('forwards all item actions with the original item and uses native tag buttons', async () => {
    const handlers = Object.fromEntries(['Copy', 'Favorite', 'Edit', 'Organize', 'Share', 'Trash'].map((name) => [`on${name}`, vi.fn()]))
    const html = await renderToString(createSSRApp({ render: () => h(ContentLibraryCard, { item, mode: 'compact', ...handlers }) }))
    for (const action of ['copy', 'favorite', 'edit', 'organize', 'share', 'trash']) {
      expect(handlers[`on${action[0]!.toUpperCase()}${action.slice(1)}`]).toHaveBeenCalledWith(item)
    }
    expect(html).toMatch(/<button[^>]+type="button"[^>]+aria-label="搜索标签：工作"/)
  })
})
