import axios from 'axios'
import type { ClipboardItem } from './clipboard'

export interface ClipboardShare {
  id: number
  token: string
  item: ClipboardItem
  expiresAt: string
  revokedAt?: string
  createdAt: string
}

export interface PublicClipboardShare {
  item: ClipboardItem
  expiresAt: string
  createdAt: string
}

const api = axios.create({ baseURL: '/api' })

export async function createClipboardShare(itemId: number, expiresInSeconds: number): Promise<ClipboardShare> {
  const { data } = await api.post<ClipboardShare>(`/clipboard/${itemId}/shares`, { expiresInSeconds })
  return data
}

export async function listClipboardShares(limit = 50): Promise<ClipboardShare[]> {
  const { data } = await api.get<{ shares: ClipboardShare[] }>('/shares', { params: { limit } })
  return data.shares
}

export async function revokeClipboardShare(id: number): Promise<void> {
  await api.post(`/shares/${id}/revoke`)
}

export async function getPublicClipboardShare(token: string): Promise<PublicClipboardShare> {
  const { data } = await api.get<PublicClipboardShare>(`/public/shares/${encodeURIComponent(token)}`)
  return data
}

export function publicShareUrl(token: string): string {
  return `${window.location.origin}/s/${encodeURIComponent(token)}`
}

export function publicShareContentUrl(token: string, download = false): string {
  const suffix = download ? '?download=1' : ''
  return `/api/public/shares/${encodeURIComponent(token)}/content${suffix}`
}
