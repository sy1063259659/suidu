export interface ContentToolAction {
  id: string
  label: string
  result: string
}

export interface ContentToolAnalysis {
  actions: ContentToolAction[]
  color?: string
  jsonState?: 'valid' | 'invalid'
  timestamp?: { iso: string; local: string }
  url?: { host: string; origin: string; path: string; queryCount: number }
}

function normalizeLines(content: string) {
  return content.replace(/\r\n?/g, '\n').split('\n')
}

function encodeBase64(content: string) {
  const bytes = new TextEncoder().encode(content)
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary)
}

function decodeBase64(content: string): string | undefined {
  const compact = content.replace(/\s/g, '')
  if (compact.length < 8 || compact.length % 4 !== 0 || !/^[A-Za-z0-9+/]+={0,2}$/.test(compact)) return undefined
  try {
    const binary = atob(compact)
    const bytes = Uint8Array.from(binary, character => character.charCodeAt(0))
    const decoded = new TextDecoder('utf-8', { fatal: true }).decode(bytes)
    if (!decoded || Array.from(decoded).some(character => {
      const code = character.codePointAt(0) ?? 0
      return code < 32 && character !== '\n' && character !== '\r' && character !== '\t'
    })) return undefined
    return decoded
  } catch {
    return undefined
  }
}

function timestampDetails(content: string): ContentToolAnalysis['timestamp'] {
  const trimmed = content.trim()
  if (!/^\d{10}(?:\d{3})?$/.test(trimmed)) return undefined
  const milliseconds = trimmed.length === 10 ? Number(trimmed) * 1000 : Number(trimmed)
  const date = new Date(milliseconds)
  if (!Number.isFinite(milliseconds) || date.getUTCFullYear() < 2000 || date.getUTCFullYear() > 2100) return undefined
  return {
    iso: date.toISOString(),
    local: new Intl.DateTimeFormat('zh-CN', {
      year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
    }).format(date),
  }
}

function colorValue(content: string) {
  const value = content.trim()
  return /^(?:#[\da-f]{3,4}|#[\da-f]{6}|#[\da-f]{8}|rgba?\(\s*\d{1,3}\s*,\s*\d{1,3}\s*,\s*\d{1,3}(?:\s*,\s*(?:0|1|0?\.\d+))?\s*\)|hsla?\([^()]+\))$/i.test(value) ? value : undefined
}

export function analyzeContentTools(content: string): ContentToolAnalysis {
  const trimmed = content.trim()
  const actions: ContentToolAction[] = []
  let jsonState: ContentToolAnalysis['jsonState']
  if ((trimmed.startsWith('{') && trimmed.endsWith('}')) || (trimmed.startsWith('[') && trimmed.endsWith(']'))) {
    try {
      const parsed = JSON.parse(trimmed)
      jsonState = 'valid'
      actions.push(
        { id: 'json-format', label: '格式化 JSON', result: JSON.stringify(parsed, null, 2) },
        { id: 'json-minify', label: '压缩 JSON', result: JSON.stringify(parsed) },
      )
    } catch {
      jsonState = 'invalid'
    }
  }

  let url: ContentToolAnalysis['url']
  try {
    const parsed = new URL(trimmed)
    if ((parsed.protocol === 'http:' || parsed.protocol === 'https:') && !/[\r\n]/.test(trimmed)) {
      url = { host: parsed.hostname, origin: parsed.origin, path: `${parsed.pathname}${parsed.search}${parsed.hash}`, queryCount: Array.from(parsed.searchParams).length }
      actions.push(
        { id: 'url-host', label: '提取域名', result: parsed.hostname },
        { id: 'url-clean', label: '移除查询参数', result: `${parsed.origin}${parsed.pathname}${parsed.hash}` },
      )
    }
  } catch {
    // Not a standalone URL.
  }

  const decoded = decodeBase64(trimmed)
  if (decoded !== undefined) actions.push({ id: 'base64-decode', label: 'Base64 解码', result: decoded })
  if (content.length <= 256 * 1024) actions.push({ id: 'base64-encode', label: 'Base64 编码', result: encodeBase64(content) })

  const lines = normalizeLines(content)
  if (lines.length > 1) {
    const nonEmpty = lines.filter(line => line.trim() !== '')
    actions.push(
      { id: 'lines-dedupe', label: '行去重', result: Array.from(new Set(lines)).join('\n') },
      { id: 'lines-sort', label: '行排序', result: [...lines].sort((left, right) => left.localeCompare(right, 'zh-CN')).join('\n') },
      { id: 'lines-clean', label: '移除空行', result: nonEmpty.join('\n') },
    )
  }

  const timestamp = timestampDetails(content)
  if (timestamp) {
    actions.push(
      { id: 'timestamp-local', label: '转本地时间', result: timestamp.local },
      { id: 'timestamp-iso', label: '转 ISO 时间', result: timestamp.iso },
    )
  }

  return { actions, color: colorValue(content), jsonState, timestamp, url }
}
