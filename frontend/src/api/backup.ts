import axios from 'axios'

export interface ImportBackupResult {
  total: number
  imported: number
  skippedDuplicates: number
}

const api = axios.create({ baseURL: '/api' })

export async function downloadBackup(): Promise<void> {
  const response = await api.get<Blob>('/backup/export', { responseType: 'blob' })
  const disposition = response.headers['content-disposition'] as string | undefined
  const filename = disposition?.match(/filename="?([^";]+)"?/i)?.[1] ?? `suidu-backup-${new Date().toISOString().slice(0, 10)}.zip`
  const url = window.URL.createObjectURL(response.data)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  window.URL.revokeObjectURL(url)
}

export async function importBackup(file: File, onProgress?: (percent: number) => void): Promise<ImportBackupResult> {
  const form = new FormData()
  form.append('backup', file)
  const { data } = await api.post<ImportBackupResult>('/backup/import', form, {
    onUploadProgress: (event) => {
      if (event.total) onProgress?.(Math.round((event.loaded / event.total) * 100))
    },
  })
  return data
}
