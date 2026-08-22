import axios from 'axios'

export interface ClipboardItem {
  id: number
  content: string
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

export async function deleteClipboard(id: number): Promise<void> {
  await api.delete(`/clipboard/${id}`)
}
