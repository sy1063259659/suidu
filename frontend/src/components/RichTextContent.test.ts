import richTextSource from './RichTextContent.vue?raw'
import { compileScript, compileTemplate, parse } from '@vue/compiler-sfc'
import * as VueRuntime from 'vue'
import { createRenderer, createSSRApp, h, nextTick } from 'vue'
import { renderToString } from 'vue/server-renderer'
import RichTextContent from './RichTextContent.vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

async function renderRichTextContent(props: { content: string; preview?: boolean }) {
  return renderToString(createSSRApp({
    render: () => h(RichTextContent, props),
  }))
}

describe('RichTextContent', () => {
  it('waits for actual layout before deciding whether a preview needs clamping', async () => {
    const longContent = Array.from({ length: 13 }, (_, index) => `第 ${index + 1} 行内容`).join('\n')
    const html = await renderRichTextContent({ content: longContent, preview: true })

    expect(html).not.toContain('is-preview-clamped')
    expect(html).not.toContain('content-clamp-fade')
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


type TestNode = { type: string; props: Record<string, any>; children: TestNode[]; parent?: TestNode; text?: string; getBoundingClientRect: () => { height: number } }
function mountMeasuredPreview(content: string) {
  let height = 100
  let observerCallback = () => {}
  const disconnect = vi.fn()
  vi.stubGlobal('ResizeObserver', class {
    constructor(callback: () => void) { observerCallback = callback }
    observe() {}
    disconnect = disconnect
  })
  vi.stubGlobal('window', { getSelection: () => ({ toString: () => '' }) })
  const node = (type: string): TestNode => ({ type, props: {}, children: [], getBoundingClientRect: () => ({ height }) })
  const renderer = createRenderer<TestNode, TestNode>({
    createElement: node,
    createText: (text) => ({ ...node('text'), text }),
    createComment: (text) => ({ ...node('comment'), text }),
    setText: (element, text) => { element.text = text },
    setElementText: (element, text) => { element.text = text },
    patchProp: (element, key, _old, value) => { element.props[key] = value },
    parentNode: (element) => element.parent ?? null,
    nextSibling: () => null,
    insert(element, parent) { parent.children.push(element); element.parent = parent },
    remove(element) { if (element.parent) element.parent.children = element.parent.children.filter((child) => child !== element) },
  })
  const viewDetail = vi.fn()
  // Vitest loads Vue SFCs for SSR here. Compile the same template for the
  // custom renderer so layout and keyboard behavior can be exercised without a browser.
  const source = richTextSource
  const template = source.split('<template>')[1]!.split('</template>')[0]!
  const compiled = compileTemplate({ source: template, filename: 'RichTextContent.vue', id: 'test', compilerOptions: { bindingMetadata: compileScript(parse(source).descriptor, { id: 'test' }).bindings } }).code
    .replace(/import \{([\s\S]*?)\} from "vue"/g, (_match, imports: string) => `const { ${imports.replace(/ as /g, ': ')} } = Vue`)
    .replace('export function render', 'return function render')
  const render = new Function('Vue', compiled)(VueRuntime)
  const app = renderer.createApp({ ...RichTextContent, render, ssrRender: undefined }, { content, preview: true, onViewDetail: viewDetail })
  app.provide(VueRuntime.ssrContextKey, { modules: new Set() })
  const container = node('root')
  app.mount(container)
  return { root: container.children[0]!, viewDetail, disconnect, unmount: () => app.unmount(), resize(value: number) { height = value; observerCallback() } }
}

afterEach(() => vi.unstubAllGlobals())

describe('measured preview behavior', () => {
  it('clamps only above 220 px and removes the clamp when the rendered content becomes shorter', async () => {
    const preview = mountMeasuredPreview('A'.repeat(2000))
    await nextTick()
    expect(preview.root.props.class).not.toContain('is-preview-clamped')
    preview.resize(300)
    await nextTick()
    expect(preview.root.props.class).toContain('is-preview-clamped')
    preview.resize(180)
    await nextTick()
    expect(preview.root.props.class).not.toContain('is-preview-clamped')
    preview.unmount()
    expect(preview.disconnect).toHaveBeenCalledOnce()
  })

  it('opens on Enter and Space without intercepting nested controls or selected text', () => {
    const preview = mountMeasuredPreview('short content')
    const preventDefault = vi.fn()
    preview.root.props.onKeydown({ key: 'Enter', target: preview.root, currentTarget: preview.root, preventDefault })
    preview.root.props.onKeydown({ key: ' ', target: preview.root, currentTarget: preview.root, preventDefault })
    preview.root.props.onKeydown({ key: 'Enter', target: {}, currentTarget: preview.root, preventDefault })
    vi.stubGlobal('window', { getSelection: () => ({ toString: () => 'selected' }) })
    preview.root.props.onKeydown({ key: 'Enter', target: preview.root, currentTarget: preview.root, preventDefault })
    expect(preview.viewDetail).toHaveBeenCalledTimes(2)
    preview.unmount()
  })

  it('leaves links and text selection alone when the content is clicked', () => {
    class TestElement {
      constructor(private interactive: boolean) {}
      closest() { return this.interactive ? this : null }
    }
    vi.stubGlobal('Element', TestElement)
    const preview = mountMeasuredPreview('short content')
    preview.root.props.onClick({ target: new TestElement(true) })
    expect(preview.viewDetail).not.toHaveBeenCalled()
    preview.root.props.onClick({ target: new TestElement(false) })
    expect(preview.viewDetail).toHaveBeenCalledOnce()
    vi.stubGlobal('window', { getSelection: () => ({ toString: () => 'selected' }) })
    preview.root.props.onClick({ target: new TestElement(false) })
    expect(preview.viewDetail).toHaveBeenCalledOnce()
    preview.unmount()
  })

  it('bounds code preview markup while preserving every line in full detail', async () => {
    const content = '```javascript\n' + Array.from({ length: 500 }, (_, index) => `const line${index} = ${index}`).join('\n') + '\n```'
    const preview = await renderRichTextContent({ content, preview: true })
    const detail = await renderRichTextContent({ content })
    expect(preview).not.toContain('line499')
    expect(detail).toContain('line499')
    expect(detail).toContain('code-line-numbers')
    expect(detail).toContain('自动换行')
    expect(detail).toContain('可横向滚动')
  })

  it('renders huge Markdown previews as safe text and never executes embedded HTML', async () => {
    const content = '# Heading\n' + '- entry\n'.repeat(2500) + '<script>alert(1)</script>'
    const preview = await renderRichTextContent({ content, preview: true })
    expect(preview).not.toContain('<li>')
    expect(preview).toContain('&lt;script&gt;')
    const detail = await renderRichTextContent({ content: '# Heading\n<script>alert(1)</script>' })
    expect(detail).not.toContain('<script>')
  })
})
