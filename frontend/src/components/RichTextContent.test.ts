import { createSSRApp, h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import RichTextContent from './RichTextContent.vue'
import { describe, expect, it } from 'vitest'

async function renderRichTextContent(props: { content: string; preview?: boolean }) {
  return renderToString(createSSRApp({
    render: () => h(RichTextContent, props),
  }))
}

describe('RichTextContent', () => {
  it('keeps long preview content collapsed and exposes a detail-page action', async () => {
    const longContent = Array.from({ length: 12 }, (_, index) => `第 ${index + 1} 行内容`).join('\n')
    const html = await renderRichTextContent({ content: longContent, preview: true })

    expect(html).toContain('is-preview-clamped')
    expect(html).toContain('preview-detail-entry')
    expect(html).toContain('content-clamp-fade')
    expect(html).toContain('aria-label="前往详情页查看完整内容"')
    expect(html).toContain('前往详情页')
    expect(html).not.toContain('>查看完整内容<')
  })

  it('does not show the detail-page action for short preview content', async () => {
    const html = await renderRichTextContent({ content: '短内容', preview: true })

    expect(html).not.toContain('is-preview-clamped')
    expect(html).not.toContain('preview-detail-entry')
    expect(html).not.toContain('aria-label="前往详情页查看完整内容"')
  })
})
