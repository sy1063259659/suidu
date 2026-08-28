import { describe, expect, it } from 'vitest'

const nodeProcess = (globalThis as unknown as {
  process: {
    getBuiltinModule(name: 'fs'): {
      readFileSync(path: URL, encoding: 'utf8'): string
    }
  }
}).process
const styles = nodeProcess.getBuiltinModule('fs').readFileSync(new URL('./style.css', import.meta.url), 'utf8')

describe('history list layout', () => {
  it('allows Naive UI list content to shrink around long code lines', () => {
    expect(styles).toMatch(/\.history-group-list\s+\.n-list-item__main\s*\{[^}]*min-width:\s*0/)
  })
})
