import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { describe, expect, it, vi } from 'vitest'

vi.mock('naive-ui', () => {
  const NButton = defineComponent({
    name: 'NButtonStub',
    props: {
      tag: String,
      href: String,
      disabled: Boolean,
      ariaBusy: [Boolean, String],
      type: String,
      secondary: Boolean,
      quaternary: Boolean,
    },
    setup(props, { attrs, slots }) {
      return () => h(props.tag === 'a' ? 'a' : 'button', {
        ...attrs,
        href: props.tag === 'a' ? props.href : undefined,
        disabled: props.tag === 'a' ? undefined : props.disabled,
        'aria-busy': props.ariaBusy,
        'data-type': props.type,
        'data-secondary': props.secondary ? 'true' : undefined,
        'data-quaternary': props.quaternary ? 'true' : undefined,
      }, slots.default?.())
    },
  })

  const NCard = defineComponent({
    name: 'NCardStub',
    setup(_, { attrs, slots }) {
      return () => h('section', attrs, slots.default?.())
    },
  })

  const NEmpty = defineComponent({
    name: 'NEmptyStub',
    props: { description: String },
    setup(props, { attrs, slots }) {
      return () => h('div', attrs, [
        h('p', props.description),
        slots.extra?.(),
      ])
    },
  })

  const NImage = defineComponent({
    name: 'NImageStub',
    props: { src: String, alt: String },
    setup(props, { attrs }) {
      return () => h('img', { ...attrs, src: props.src, alt: props.alt, 'data-image-preview': 'true' })
    },
  })

  const NPopconfirm = defineComponent({
    name: 'NPopconfirmStub',
    setup(_, { slots }) {
      return () => h('div', { class: 'n-popconfirm-stub' }, [
        slots.trigger?.(),
        slots.default?.(),
      ])
    },
  })

  const NSpin = defineComponent({
    name: 'NSpinStub',
    setup() {
      return () => h('div', { class: 'n-spin-stub' })
    },
  })

  const NSpace = defineComponent({
    name: 'NSpaceStub',
    setup(_, { attrs, slots }) {
      return () => h('div', attrs, slots.default?.())
    },
  })

  const NTag = defineComponent({
    name: 'NTagStub',
    setup(_, { attrs, slots }) {
      return () => h('span', attrs, slots.default?.())
    },
  })

  const NText = defineComponent({
    name: 'NTextStub',
    setup(_, { attrs, slots }) {
      return () => h('span', attrs, slots.default?.())
    },
  })

  return { NButton, NCard, NEmpty, NImage, NPopconfirm, NSpin, NSpace, NTag, NText }
})

import ClipboardDetailPage from './ClipboardDetailPage.vue'
import type { ClipboardItem } from '../api/clipboard'

function createItem(overrides: Partial<ClipboardItem> = {}): ClipboardItem {
  return {
    id: 42,
    kind: 'text',
    content: '这是一条很长的剪贴板内容，用来验证操作栏出现在正文之前。',
    source: 'web',
    createdAt: '2026-08-26T12:34:56.000Z',
    note: '备注内容',
    tags: ['工作', '灵感'],
    favorite: false,
    ...overrides,
  }
}

async function renderDetailPage(props: Partial<InstanceType<typeof ClipboardDetailPage>['$props']> = {}) {
  return renderToString(createSSRApp({
    render: () => h(ClipboardDetailPage, {
      item: createItem(),
      loading: false,
      unavailable: false,
      favoriteUpdating: false,
      copied: false,
      ...props,
    }),
  }))
}

describe('ClipboardDetailPage', () => {
  it('renders created time in the header and places the action bar before the content', async () => {
    const html = await renderDetailPage({
      favoriteUpdating: true,
      copied: true,
    })

    expect(html).toContain('detail-created-at')
    expect(html).toContain('创建于')
    expect(html).toContain('detail-toolbar')
    expect(html).not.toContain('detail-footer')
    expect(html).toContain('aria-busy="true"')
    expect(html).toContain('已复制')
    expect(html.indexOf('detail-created-at')).toBeLessThan(html.indexOf('detail-toolbar'))
    expect(html.indexOf('detail-toolbar')).toBeLessThan(html.indexOf('detail-content'))
  })

  it('keeps the copy/download branches and marks delete as a secondary action', async () => {
    const textHtml = await renderDetailPage()
    const fileHtml = await renderDetailPage({
      item: createItem({
        kind: 'file',
        content: undefined,
        fileName: 'archive.zip',
        mediaType: 'application/zip',
        sizeBytes: 2048,
      }),
    })

    expect(textHtml).toContain('>复制</button>')
    expect(textHtml).not.toContain('>下载</a>')
    expect(fileHtml).toContain('/api/clipboard/42/content?download=1')
    expect(fileHtml).toContain('>下载</a>')
    expect(fileHtml).not.toContain('>复制</button>')
    expect(fileHtml).toContain('detail-delete-action')
  })

  it('keeps the grouped action-bar structure needed for wrapped touch actions', async () => {
    const html = await renderDetailPage({
      item: createItem({ favorite: true }),
    })

    expect(html).toContain('class="detail-toolbar"')
    expect(html).toContain('class="detail-actions"')
    expect(html.match(/detail-action-button/g)?.length).toBe(5)
    expect(html).toContain('detail-action-button detail-delete-action')
  })
})
