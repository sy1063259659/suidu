export type TextFormatKind = 'text' | 'url' | 'markdown' | 'json' | 'code'

export interface TextFormat {
  kind: TextFormatKind
  label: string
  language?: string
  content: string
}

const languageLabels: Record<string, string> = {
  bash: 'Shell', css: 'CSS', go: 'Go', html: 'HTML', java: 'Java', javascript: 'JavaScript',
  json: 'JSON', markdown: 'Markdown', python: 'Python', sql: 'SQL', text: '文本',
  typescript: 'TypeScript', xml: 'XML', yaml: 'YAML',
}

function codeFormat(content: string, language: string): TextFormat {
  return { kind: language === 'json' ? 'json' : 'code', language, label: languageLabels[language] ?? language.toUpperCase(), content }
}

function fencedCode(content: string): TextFormat | undefined {
  const match = content.match(/^```([\w+-]*)[ \t]*\n([\s\S]*?)\n?```[ \t]*$/)
  if (!match) return undefined
  const language = (match[1] || 'text').toLowerCase()
  return codeFormat(match[2], language)
}

function codeLanguage(content: string): string | undefined {
  if (/^\s*(SELECT|INSERT|UPDATE|DELETE|CREATE|ALTER|WITH)\b[\s\S]*\b(FROM|INTO|TABLE|SET|AS)\b/im.test(content)) return 'sql'
  if (/^\s*package\s+\w+/m.test(content) && /\bfunc\s+\w+\s*\(/.test(content)) return 'go'
  if (/^\s*(def|class)\s+\w+.*:\s*$/m.test(content) || /^\s*(from\s+\S+\s+import|import\s+\w+)/m.test(content) && /:\s*$/m.test(content)) return 'python'
  if (/\b(interface|type)\s+\w+|:\s*(string|number|boolean|unknown|Record|Array)\b/.test(content)) return 'typescript'
  if (/^\s*(import|export)\s+.*from\s+['"]|\b(const|let|var|function|async function)\s+\w+/.test(content)) return 'javascript'
  if (/\b(public|private|protected)\s+(static\s+)?(class|void|String|int)\b|^\s*package\s+[\w.]+;/m.test(content)) return 'java'
  if (/^\s*#!.*\b(bash|sh)\b|^\s*(sudo\s+)?(cd|ls|docker|git|npm|pnpm|yarn|curl)\s+\S+/m.test(content)) return 'bash'
  if (/^\s*<[A-Za-z][^>]*>[\s\S]*<\/[A-Za-z][^>]*>\s*$/.test(content)) return 'html'
  if (/^\s*[.#]?[\w-]+(?:\s+[.#]?[\w-]+)*\s*\{[\s\S]*:[^;{}]+;[\s\S]*\}/.test(content)) return 'css'
  if (/^(?:[\w.-]+:\s*(?:[^\n]+)?\n)+(?:\s{2,}[\w.-]+:\s*[^\n]*\n?)+$/m.test(content)) return 'yaml'
  return undefined
}

function looksLikeMarkdown(content: string): boolean {
  const signals = [
    /^#{1,6}\s+\S/m,
    /^\s*[-*+]\s+\S/m,
    /^\s*\d+\.\s+\S/m,
    /^>\s+\S/m,
    /\[[^\]]+\]\(https?:\/\/[^)]+\)/,
    /^\|.+\|\s*\n\|?\s*:?-{3,}/m,
    /(?:^|\s)(?:\*\*|__)[^\n]+(?:\*\*|__)(?:\s|$)/,
  ]
  return signals.some((signal) => signal.test(content))
}

export function detectTextFormat(rawContent: string): TextFormat {
  const content = rawContent.trim()
  const fenced = fencedCode(content)
  if (fenced) return fenced

  if ((content.startsWith('{') && content.endsWith('}')) || (content.startsWith('[') && content.endsWith(']'))) {
    try {
      const parsed = JSON.parse(content)
      return codeFormat(JSON.stringify(parsed, null, 2), 'json')
    } catch {
      // Continue with the remaining deterministic detectors.
    }
  }

  try {
    const url = new URL(content)
    if ((url.protocol === 'http:' || url.protocol === 'https:') && !/[\r\n]/.test(content)) {
      return { kind: 'url', label: '链接', content }
    }
  } catch {
    // Not a standalone URL.
  }

  const language = codeLanguage(content)
  if (language) return codeFormat(content, language)
  if (looksLikeMarkdown(content)) return { kind: 'markdown', language: 'markdown', label: 'Markdown', content }
  return { kind: 'text', label: '文本', content: rawContent }
}

export function isLongText(content = ''): boolean {
  return Array.from(content).length > 360 || content.split(/\r?\n/).length > 8
}
