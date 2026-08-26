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
  note?: string
  tags?: string[]
  favorite?: boolean
}

export interface ListClipboardOptions {
  limit?: number
  query?: string
  kind?: ClipboardItemKind
  favoriteOnly?: boolean
  createdFrom?: string
  createdBefore?: string
}

const api = axios.create({ baseURL: '/api' })

export async function listClipboard(options: ListClipboardOptions = {}): Promise<ClipboardItem[]> {
  const { data } = await api.get<{ items: ClipboardItem[] }>('/clipboard', {
    params: {
      limit: options.limit ?? 50,
      q: options.query?.trim() || undefined,
      kind: options.kind,
      favorite: options.favoriteOnly ? true : undefined,
      from: options.createdFrom,
      to: options.createdBefore,
    },
  })
  return data.items
}

export async function getClipboard(id: number): Promise<ClipboardItem> {
  const { data } = await api.get<ClipboardItem>(`/clipboard/${id}`)
  return data
}

export async function updateClipboardMetadata(id: number, note: string, tags: string[], favorite: boolean): Promise<ClipboardItem> {
  const { data } = await api.patch<ClipboardItem>(`/clipboard/${id}`, { note, tags, favorite })
  return data
}

export async function createClipboard(content: string, source = 'web', allowDuplicate = false): Promise<ClipboardItem> {
  const { data } = await api.post<ClipboardItem>('/clipboard', { content, source, allowDuplicate })
  return data
}

export async function uploadClipboardFile(file: File, source = 'web', onProgress?: (percent: number) => void, allowDuplicate = false): Promise<ClipboardItem> {
  const form = new FormData()
  form.append('file', file)
  form.append('source', source)
  form.append('allowDuplicate', String(allowDuplicate))
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
