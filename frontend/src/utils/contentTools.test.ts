import { describe, expect, it } from 'vitest'
import { analyzeContentTools } from './contentTools'

function result(content: string, id: string) {
  return analyzeContentTools(content).actions.find(action => action.id === id)?.result
}

describe('analyzeContentTools', () => {
  it('formats, minifies, and validates JSON', () => {
    const analysis = analyzeContentTools('{"name":"随渡","enabled":true}')
    expect(analysis.jsonState).toBe('valid')
    expect(result('{"name":"随渡"}', 'json-format')).toContain('\n  "name"')
    expect(result('{ "name": "随渡" }', 'json-minify')).toBe('{"name":"随渡"}')
    expect(analyzeContentTools('{broken}').jsonState).toBe('invalid')
  })

  it('extracts useful URL variants', () => {
    const analysis = analyzeContentTools('https://example.com/docs?q=suidu#start')
    expect(analysis.url).toEqual({ host: 'example.com', origin: 'https://example.com', path: '/docs?q=suidu#start', queryCount: 1 })
    expect(result('https://example.com/docs?q=suidu#start', 'url-clean')).toBe('https://example.com/docs#start')
  })

  it('encodes and decodes Unicode Base64', () => {
    const encoded = result('随渡', 'base64-encode')
    expect(encoded).toBe('6ZqP5rih')
    expect(result(encoded ?? '', 'base64-decode')).toBe('随渡')
  })

  it('converts second and millisecond timestamps', () => {
    expect(analyzeContentTools('1735689600').timestamp?.iso).toBe('2025-01-01T00:00:00.000Z')
    expect(analyzeContentTools('1735689600000').timestamp?.iso).toBe('2025-01-01T00:00:00.000Z')
    expect(result('1735689600', 'timestamp-iso')).toBe('2025-01-01T00:00:00.000Z')
  })

  it('provides multiline cleanup and color previews', () => {
    const content = '香蕉\n\n苹果\n香蕉'
    expect(result(content, 'lines-dedupe')).toBe('香蕉\n\n苹果')
    expect(result(content, 'lines-clean')).toBe('香蕉\n苹果\n香蕉')
    expect(analyzeContentTools('#2d6cdf').color).toBe('#2d6cdf')
  })
})
