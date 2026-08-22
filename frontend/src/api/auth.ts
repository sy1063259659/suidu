import axios from 'axios'

export type UserRole = 'admin' | 'user'

export interface User {
  id: number
  username: string
  role: UserRole
  createdAt: string
  disabled: boolean
}

const api = axios.create({ baseURL: '/api', withCredentials: true })

export async function getCurrentUser() {
  const { data } = await api.get<{ user: User }>('/auth/me')
  return data.user
}

export async function login(username: string, password: string) {
  const { data } = await api.post<{ user: User }>('/auth/login', { username, password })
  return data.user
}

export async function logout() { await api.post('/auth/logout') }

export async function changePassword(currentPassword: string, newPassword: string) {
  await api.post('/auth/password', { currentPassword, newPassword })
}

export async function listUsers() {
  const { data } = await api.get<{ users: User[] }>('/admin/users')
  return data.users
}

export async function createUser(username: string, password: string) {
  const { data } = await api.post<{ user: User }>('/admin/users', { username, password, role: 'user' })
  return data.user
}

export async function resetUserPassword(id: number, newPassword: string) {
  await api.post(`/admin/users/${id}/password`, { newPassword })
}

export async function setUserDisabled(id: number, disabled: boolean) {
  await api.post(`/admin/users/${id}/${disabled ? 'disable' : 'enable'}`)
}
