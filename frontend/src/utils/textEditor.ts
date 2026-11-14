export const MAX_TEXT_BYTES = 1 << 20

export function textStats(content: string) {
  let characters = 0
  for (const _character of content) characters++
  return { characters, bytes: new TextEncoder().encode(content).length }
}

export function formatTextBytes(bytes: number) {
  return bytes < 1024 ? `${bytes} B` : `${(bytes / 1024).toFixed(1)} KiB`
}

