import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { describe, expect, it, vi } from 'vitest'

vi.mock('naive-ui', () => ({
  NImage: defineComponent({
    props: { src: String, alt: String },
    setup(props, { attrs }) {
      return () => h('img', { ...attrs, src: props.src, alt: props.alt, 'data-image-preview': 'true' })
    },
  }),
}))

import ClipboardItemContent from './ClipboardItemContent.vue'
import type { ClipboardItem } from '../api/clipboard'

const imageItem: ClipboardItem = {
  id: 42,
  kind: 'image',
  fileName: 'screenshot.png',
  mediaType: 'image/png',
  sizeBytes: 2048,
  source: 'web',
  createdAt: '2026-08-27T00:00:00.000Z',
}

async function renderContent(preview: boolean) {
  return renderToString(createSSRApp({
    render: () => h(ClipboardItemContent, { item: imageItem, preview }),
  }))
}

describe('ClipboardItemContent image interaction', () => {
  it('uses a detail button for history previews instead of navigating to the raw image endpoint', async () => {
    const html = await renderContent(true)
    expect(html).toContain('aria-label="查看图片详情"')
    expect(html).toContain('<button')
    expect(html).not.toContain('target="_blank"')
    expect(html).not.toContain('<a ')
  })

  it('uses the in-page image preview on the detail page', async () => {
    const html = await renderContent(false)
    expect(html).toContain('data-image-preview="true"')
    expect(html).not.toContain('target="_blank"')
    expect(html).not.toContain('<a ')
  })
})


describe('ClipboardItemContent file interaction', () => {
  it('makes file previews keyboard accessible while keeping detail metadata passive', async () => {
    const item: ClipboardItem = { ...imageItem, kind: 'file', fileName: 'report.pdf', mediaType: 'application/pdf' }
    const preview = await renderToString(createSSRApp({ render: () => h(ClipboardItemContent, { item, preview: true }) }))
    const detail = await renderToString(createSSRApp({ render: () => h(ClipboardItemContent, { item }) }))
    expect(preview).toContain('aria-label="查看文件详情"')
    expect(preview).toContain('role="button"')
    expect(preview).toContain('tabindex="0"')
    expect(detail).not.toContain('role="button"')
  })
})
