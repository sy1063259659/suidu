import axios from 'axios'

export type ClipboardItemKind = 'text' | 'image' | 'file'

export interface ClipboardItem {
  id: number
  kind: ClipboardItemKind
  content?: string
  fileName?: string
  mediaType?: string
  sizeBytes?: number
  source: string
  createdAt: string
}

const api = axios.create({ baseURL: '/api' })

export async function listClipboard(limit = 50): Promise<ClipboardItem[]> {
  const { data } = await api.get<{ items: ClipboardItem[] }>('/clipboard', { params: { limit } })
  return data.items
}

export async function createClipboard(content: string, source = 'web'): Promise<ClipboardItem> {
  const { data } = await api.post<ClipboardItem>('/clipboard', { content, source })
  return data
}

export async function uploadClipboardFile(file: File, source = 'web', onProgress?: (percent: number) => void): Promise<ClipboardItem> {
  const form = new FormData()
  form.append('file', file)
  form.append('source', source)
  const { data } = await api.post<ClipboardItem>('/clipboard/files', form, {
    onUploadProgress: (event) => {
      if (event.total) onProgress?.(Math.round((event.loaded / event.total) * 100))
    },
  })
  return data
}

export function clipboardContentUrl(id: number, download = false): string {
  return `/api/clipboard/${id}/content${download ? '?download=1' : ''}`
}

export async function deleteClipboard(id: number): Promise<void> {
  await api.delete(`/clipboard/${id}`)
}
