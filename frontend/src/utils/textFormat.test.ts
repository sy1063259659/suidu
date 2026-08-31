import { describe, expect, it } from 'vitest'
import { detectTextFormat, isLongText } from './textFormat'

describe('detectTextFormat', () => {
  it.each([
    ['fenced code', '```go\npackage main\n```', 'code', 'go'],
    ['json', '{"name":"随渡","enabled":true}', 'json', 'json'],
    ['markdown', '# 标题\n\n- 第一项\n- 第二项', 'markdown', 'markdown'],
    ['url', 'https://example.com/docs?q=suidu', 'url', undefined],
    ['go', 'package main\n\nfunc main() {\n\tfmt.Println("hi")\n}', 'code', 'go'],
    ['typescript', 'interface User { id: number }\nconst user: User = { id: 1 }', 'code', 'typescript'],
    ['python', 'def greet(name):\n    print(f"hello {name}")', 'code', 'python'],
    ['sql', 'SELECT id, name\nFROM users\nWHERE disabled = false;', 'code', 'sql'],
    ['yaml', 'services:\n  api:\n    image: suidu:latest', 'code', 'yaml'],
    ['plain text', '明天下午记得把这份材料发给小王。', 'text', undefined],
  ])('recognizes %s', (_name, content, kind, language) => {
    const result = detectTextFormat(content)
    expect(result.kind).toBe(kind)
    expect(result.language).toBe(language)
  })
})

describe('isLongText', () => {
  it('does not collapse a few lines of ordinary text', () => {
    expect(isLongText('短内容')).toBe(false)
    expect(isLongText(Array.from({ length: 8 }, () => '这是一段比较长但仍然可以快速阅读的普通文本。'.repeat(4)).join('\n'))).toBe(false)
  })

  it('uses generous character and line limits for genuinely long text', () => {
    expect(isLongText('字'.repeat(721))).toBe(true)
    expect(isLongText(Array.from({ length: 13 }, (_, index) => `第 ${index + 1} 行`).join('\n'))).toBe(true)
  })
})
