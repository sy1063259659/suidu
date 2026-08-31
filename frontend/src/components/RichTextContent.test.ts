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
  it('keeps long preview content collapsed without a redundant detail action', async () => {
    const longContent = Array.from({ length: 13 }, (_, index) => `第 ${index + 1} 行内容`).join('\n')
    const html = await renderRichTextContent({ content: longContent, preview: true })

    expect(html).toContain('is-preview-clamped')
    expect(html).toContain('content-clamp-fade')
    expect(html).toContain('is-preview-interactive')
    expect(html).not.toContain('preview-detail-entry')
    expect(html).not.toContain('前往详情页')
  })

  it('does not show the detail-page action for short preview content', async () => {
    const html = await renderRichTextContent({ content: '短内容', preview: true })

    expect(html).not.toContain('is-preview-clamped')
    expect(html).not.toContain('preview-detail-entry')
    expect(html).not.toContain('content-clamp-fade')
  })

  it('keeps short code previews clickable without a separate detail action', async () => {
    const html = await renderRichTextContent({
      content: 'const greeting = "hello"\nconsole.log(greeting)',
      preview: true,
    })

    expect(html).toContain('format-code')
    expect(html).toContain('is-preview-interactive')
    expect(html).not.toContain('aria-label="查看代码详情"')
    expect(html).not.toContain('>查看详情<')
  })

  it('does not show clamp affordances for long standalone urls in preview', async () => {
    const longUrl = `https://example.com/${'path-segment/'.repeat(40)}?q=${'search-term-'.repeat(20)}`
    const html = await renderRichTextContent({ content: longUrl, preview: true })

    expect(html).toContain('class="standalone-link"')
    expect(html).not.toContain('is-preview-clamped')
    expect(html).not.toContain('preview-detail-entry')
    expect(html).not.toContain('content-clamp-fade')
    expect(html).not.toContain('前往详情页')
  })
})
