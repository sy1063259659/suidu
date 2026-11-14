import { describe, expect, it } from 'vitest'
import type { ClipboardItem } from '../api/clipboard'
import { formatCardFileSize, getCardPresentation } from './cardPresentation'

const textItem: ClipboardItem = { id: 1, kind: 'text', content: '\r\n  原始内容\n第二行  ', source: 'web', createdAt: '2026-08-27T08:00:00Z' }

describe('content library card presentation', () => {
  it('uses the first nonempty note line as title and keeps the remaining note separate', () => {
    const item = { ...textItem, note: '\n \t\r\n  会议纪要  \r\n后续事项\r\n第三行\r\n第四行' }
    expect(getCardPresentation(item)).toMatchObject({ title: '会议纪要', note: '后续事项\n第三行\n第四行', summary: '原始内容 第二行' })
    expect(item.content).toBe(textItem.content)
    expect(item.note).toBe('\n \t\r\n  会议纪要  \r\n后续事项\r\n第三行\r\n第四行')
  })

  it('falls back to text or filename without changing the raw copy/edit content', () => {
    expect(getCardPresentation({ ...textItem, note: ' \n ' }).title).toBe('原始内容')
    expect(getCardPresentation({ ...textItem, kind: 'image', fileName: '截图.png' })).toMatchObject({ title: '截图.png', typeLabel: '图片' })
    expect(getCardPresentation({ ...textItem, kind: 'file', fileName: 'report.pdf', note: '说明' })).toMatchObject({ title: '说明', note: '', typeLabel: '文件' })
  })

  it('has usable empty and missing attachment fallbacks', () => {
    expect(getCardPresentation({ ...textItem, content: ' \r\n ' })).toMatchObject({ title: '未命名文本', summary: '空文本' })
    expect(getCardPresentation({ ...textItem, kind: 'file' }).title).toBe('未命名文件')
    expect(getCardPresentation({ ...textItem, kind: 'image' }).title).toBe('未命名图片')
  })

  it('keeps Unicode, long titles, and potentially unsafe markup as display text', () => {
    const title = '🙂中'.repeat(1000)
    expect(getCardPresentation({ ...textItem, note: title }).title).toBe(title)
    expect(getCardPresentation({ ...textItem, content: '<script>alert(1)</script>\n hello' }).summary).toBe('<script>alert(1)</script> hello')
  })

  it.each([[undefined, '0 B'], [NaN, '0 B'], [-1, '0 B'], [123, '123 B'], [1024, '1.0 KiB'], [1024 ** 2, '1.0 MiB'], [1024 ** 3, '1.0 GiB']])('formats file size %s', (value, expected) => {
    expect(formatCardFileSize(value as number | undefined)).toBe(expected)
  })
})
