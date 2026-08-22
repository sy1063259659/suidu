<script setup lang="ts">
import axios from 'axios'
import { computed, onMounted, ref } from 'vue'
import { ClipboardPaste, Copy, RefreshCw, Send, Trash2 } from '@lucide/vue'
import {
  NAlert, NButton, NCard, NEmpty, NInput, NLayout, NLayoutContent, NLayoutHeader,
  NList, NListItem, NPopconfirm, NSpace, NSpin, NTag, NText,
} from 'naive-ui'
import {
  changePassword, createUser, getCurrentUser, listUsers, login, logout, resetUserPassword,
  setUserDisabled, type User,
} from './api/auth'
import { createClipboard, deleteClipboard, listClipboard, type ClipboardItem } from './api/clipboard'

const items = ref<ClipboardItem[]>([])
const currentUser = ref<User | null>(null)
const authLoading = ref(true)
const loginLoading = ref(false)
const loginUsername = ref('')
const loginPassword = ref('')
const loginError = ref('')
const passwordFormOpen = ref(false)
const passwordLoading = ref(false)
const currentPassword = ref('')
const newPassword = ref('')
const passwordError = ref('')
const passwordSuccess = ref('')
const adminUsers = ref<User[]>([])
const adminLoading = ref(false)
const newUsername = ref('')
const newUserPassword = ref('')
const adminError = ref('')
const adminSuccess = ref('')
const draft = ref('')
const loading = ref(false)
const submitting = ref(false)
const error = ref('')
const apiStatus = ref<'checking' | 'online' | 'offline'>('checking')
const copiedId = ref<number | null>(null)
const canSubmit = computed(() => draft.value.trim().length > 0 && !submitting.value)
const isAdmin = computed(() => currentUser.value?.role === 'admin')

function errorMessage(errorValue: unknown, fallback: string) {
  if (axios.isAxiosError(errorValue) && typeof errorValue.response?.data?.error === 'string') return errorValue.response.data.error
  return fallback
}

async function loadItems() {
  loading.value = true
  error.value = ''
  apiStatus.value = 'checking'
  try { items.value = await listClipboard(); apiStatus.value = 'online' } catch { apiStatus.value = 'offline'; error.value = '无法连接服务，请稍后重试。' } finally { loading.value = false }
}

async function loadAdminUsers() {
  if (!isAdmin.value) return
  adminLoading.value = true
  try { adminUsers.value = await listUsers() } catch (errorValue) { adminError.value = errorMessage(errorValue, '无法加载用户列表。') } finally { adminLoading.value = false }
}

async function loadSession() {
  authLoading.value = true
  try {
    currentUser.value = await getCurrentUser()
    await loadItems()
    await loadAdminUsers()
  } catch (errorValue) {
    if (!axios.isAxiosError(errorValue) || errorValue.response?.status !== 401) loginError.value = '无法连接认证服务，请稍后重试。'
    currentUser.value = null
  } finally { authLoading.value = false }
}

async function submitLogin() {
  if (!loginUsername.value.trim() || !loginPassword.value || loginLoading.value) return
  loginLoading.value = true
  loginError.value = ''
  try { currentUser.value = await login(loginUsername.value, loginPassword.value); loginPassword.value = ''; await loadItems(); await loadAdminUsers() } catch (errorValue) { loginError.value = errorMessage(errorValue, '登录失败，请稍后重试。') } finally { loginLoading.value = false }
}

async function submitLogout() {
  try { await logout() } finally { currentUser.value = null; items.value = []; adminUsers.value = []; passwordFormOpen.value = false }
}

async function submitPasswordChange() {
  if (!currentPassword.value || !newPassword.value || passwordLoading.value) return
  passwordLoading.value = true; passwordError.value = ''; passwordSuccess.value = ''
  try {
    await changePassword(currentPassword.value, newPassword.value)
    passwordSuccess.value = '密码已修改，请重新登录。'
    currentPassword.value = ''; newPassword.value = ''
    window.setTimeout(() => { void submitLogout() }, 700)
  } catch (errorValue) { passwordError.value = errorMessage(errorValue, '修改密码失败，请检查输入。') } finally { passwordLoading.value = false }
}

async function submitCreateUser() {
  if (!newUsername.value.trim() || !newUserPassword.value) return
  adminError.value = ''; adminSuccess.value = ''
  try { await createUser(newUsername.value, newUserPassword.value); newUsername.value = ''; newUserPassword.value = ''; adminSuccess.value = '普通用户已创建。'; await loadAdminUsers() } catch (errorValue) { adminError.value = errorMessage(errorValue, '创建用户失败，请检查用户名和密码。') }
}

async function resetPasswordFor(user: User) {
  const nextPassword = window.prompt(`为 ${user.username} 设置新密码（至少 12 位）`)
  if (!nextPassword) return
  try { await resetUserPassword(user.id, nextPassword); adminSuccess.value = `${user.username} 的密码已重置。` } catch (errorValue) { adminError.value = errorMessage(errorValue, '重置密码失败。') }
}

async function toggleUser(user: User) {
  try { await setUserDisabled(user.id, !user.disabled); await loadAdminUsers() } catch (errorValue) { adminError.value = errorMessage(errorValue, '更新用户状态失败。') }
}

async function readClipboard() {
  if (!navigator.clipboard?.readText) { error.value = '当前浏览器不支持读取剪贴板。'; return }
  try { draft.value = await navigator.clipboard.readText(); error.value = '' } catch { error.value = '读取剪贴板失败，请允许当前页面访问剪贴板。' }
}

async function submitClipboard() {
  if (!canSubmit.value) return
  submitting.value = true; error.value = ''
  try { const item = await createClipboard(draft.value.trim()); items.value = [item, ...items.value]; draft.value = ''; apiStatus.value = 'online' } catch { apiStatus.value = 'offline'; error.value = '提交失败，请稍后重试。' } finally { submitting.value = false }
}

async function copyItem(item: ClipboardItem) {
  if (!navigator.clipboard?.writeText) { error.value = '当前浏览器不支持写入剪贴板。'; return }
  try { await navigator.clipboard.writeText(item.content); copiedId.value = item.id; window.setTimeout(() => { if (copiedId.value === item.id) copiedId.value = null }, 1600) } catch { error.value = '复制失败，请检查浏览器剪贴板权限。' }
}

async function removeItem(item: ClipboardItem) {
  try { await deleteClipboard(item.id); items.value = items.value.filter((current) => current.id !== item.id) } catch { error.value = '删除失败，请稍后重试。' }
}

function formatDate(value: string) { return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value)) }
function handleKeydown(event: KeyboardEvent) { if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') { event.preventDefault(); void submitClipboard() } }

onMounted(loadSession)
</script>

<template>
  <div v-if="authLoading" class="auth-loading"><n-spin size="medium" /></div>
  <main v-else-if="!currentUser" class="auth-page">
    <n-card class="auth-card" :bordered="false">
      <div class="brand auth-brand">随渡 <span>SUIDU</span></div>
      <p class="eyebrow">PRIVATE CONTENT BRIDGE</p><h1>登录随渡</h1><p class="intro-copy">使用你的随渡账号访问剪贴板内容。</p>
      <n-alert v-if="loginError" type="error" class="auth-alert">{{ loginError }}
      </n-alert>
      <n-input v-model:value="loginUsername" placeholder="用户名" autocomplete="username" class="auth-input" @keyup.enter="submitLogin" />
      <n-input v-model:value="loginPassword" type="password" show-password-on="click" placeholder="密码" autocomplete="current-password" class="auth-input" @keyup.enter="submitLogin" />
      <n-button type="primary" block :loading="loginLoading" @click="submitLogin">登录
      </n-button>
    </n-card>
  </main>
  <n-layout v-else class="app-shell">
    <n-layout-header bordered class="app-header">
      <div><div class="brand">随渡 <span>SUIDU</span></div><div class="subtitle">把文字放在随手可取的地方</div></div>
      <n-space align="center" :size="12">
        <n-tag :type="isAdmin ? 'warning' : 'info'">{{ currentUser.username }} · {{ isAdmin ? '管理员' : '普通用户' }}
        </n-tag>
        <n-button quaternary @click="passwordFormOpen = !passwordFormOpen">修改密码
        </n-button>
        <n-button quaternary @click="submitLogout">退出
        </n-button>
        <n-tag :type="apiStatus === 'online' ? 'success' : apiStatus === 'offline' ? 'error' : 'warning'">API {{ apiStatus === 'online' ? '在线' : apiStatus === 'offline' ? '离线' : '检查中' }}
        </n-tag>
        <n-button quaternary circle aria-label="刷新内容" title="刷新内容" :loading="loading" @click="loadItems"><template #icon><RefreshCw :size="17" /></template>
        </n-button>
      </n-space>
    </n-layout-header>
    <n-layout-content class="app-content">
      <main class="clipboard-page">
        <n-card v-if="passwordFormOpen" class="account-card" :bordered="false">
          <div class="section-heading"><div><p class="eyebrow">ACCOUNT</p><h2>修改密码</h2></div></div>
          <n-space vertical :size="10" class="form-stack"><n-input v-model:value="currentPassword" type="password" show-password-on="click" placeholder="当前密码" autocomplete="current-password" /><n-input v-model:value="newPassword" type="password" show-password-on="click" placeholder="新密码（至少 12 位）" autocomplete="new-password" /><n-space><n-button type="primary" :loading="passwordLoading" @click="submitPasswordChange">保存新密码</n-button><n-button quaternary @click="passwordFormOpen = false">取消</n-button></n-space></n-space>
          <n-alert v-if="passwordError" type="error" class="form-alert">{{ passwordError }}
          </n-alert><n-alert v-if="passwordSuccess" type="success" class="form-alert">{{ passwordSuccess }}
          </n-alert>
        </n-card>
        <n-card v-if="isAdmin" class="account-card" :bordered="false">
           <div class="section-heading"><div><p class="eyebrow">ADMINISTRATION</p><h2>用户管理</h2></div><n-button quaternary :loading="adminLoading" @click="loadAdminUsers">刷新
          </n-button></div>
           <n-space vertical :size="10" class="form-stack"><n-input v-model:value="newUsername" placeholder="普通用户用户名" autocomplete="off" /><n-input v-model:value="newUserPassword" type="password" show-password-on="click" placeholder="初始密码（至少 12 位）" autocomplete="new-password" /><n-button type="primary" :disabled="!newUsername.trim() || !newUserPassword" @click="submitCreateUser">创建普通用户
          </n-button></n-space>
           <n-alert v-if="adminError" type="error" class="form-alert">{{ adminError }}
          </n-alert><n-alert v-if="adminSuccess" type="success" class="form-alert">{{ adminSuccess }}
          </n-alert>
           <n-list v-if="adminUsers.length" class="user-list" bordered><n-list-item v-for="user in adminUsers" :key="user.id"><div class="user-row"><div><strong>{{ user.username }}</strong><n-tag size="small" :type="user.role === 'admin' ? 'warning' : 'info'">{{ user.role === 'admin' ? '管理员' : '普通用户' }}</n-tag><n-tag v-if="user.disabled" size="small" type="error">已停用
            </n-tag></div><n-space><n-button v-if="user.role === 'user'" quaternary @click="resetPasswordFor(user)">重置密码</n-button><n-button v-if="user.role === 'user'" quaternary @click="toggleUser(user)">{{ user.disabled ? '启用' : '停用' }}
            </n-button></n-space></div></n-list-item></n-list>
        </n-card>
        <section class="page-intro"><div><p class="eyebrow">TEXT CLIPBOARD</p><h1>剪贴板</h1><p class="intro-copy">在手机和电脑之间传递一段文字，提交后会保存在最近记录中。</p></div><n-tag round :bordered="false" type="info">{{ items.length }} 条记录</n-tag></section>
        <n-card class="composer-card" :bordered="false"><n-input v-model:value="draft" type="textarea" placeholder="输入或粘贴要传递的文字..." :autosize="{ minRows: 5, maxRows: 12 }" maxlength="1048576" show-count @keydown="handleKeydown" /><div class="composer-actions"><n-button secondary @click="readClipboard"><template #icon><ClipboardPaste :size="17" /></template>读取剪贴板</n-button><n-button type="primary" :disabled="!canSubmit" :loading="submitting" @click="submitClipboard"><template #icon><Send :size="17" /></template>提交文本</n-button></div></n-card>
         <n-alert v-if="error" type="error" closable class="error-alert" @close="error = ''">{{ error }}
        </n-alert>
        <section class="history-section"><div class="section-heading"><div><p class="eyebrow">RECENT</p><h2>最近记录</h2></div><n-text depth="3">按时间倒序</n-text></div><div v-if="loading && !items.length" class="loading-state"><n-spin size="medium" /></div><n-empty v-else-if="!items.length" description="还没有剪贴板记录" class="empty-state" /><n-list v-else class="history-list" bordered><n-list-item v-for="item in items" :key="item.id"><div class="history-item"><div class="history-content">{{ item.content }}</div><div class="history-meta"><n-space :size="8" align="center"><n-tag size="small" :bordered="false">{{ item.source || 'web' }}</n-tag><n-text depth="3">{{ formatDate(item.createdAt) }}</n-text></n-space><n-space :size="4"><n-button quaternary circle :aria-label="copiedId === item.id ? '已复制' : '复制记录'" :title="copiedId === item.id ? '已复制' : '复制记录'" @click="copyItem(item)"><template #icon><Copy :size="16" /></template></n-button><n-popconfirm @positive-click="removeItem(item)"><template #trigger><n-button quaternary circle aria-label="删除记录" title="删除记录"><template #icon><Trash2 :size="16" /></template></n-button></template>确定删除这条记录吗？</n-popconfirm></n-space></div></div></n-list-item></n-list></section>
      </main>
    </n-layout-content>
  </n-layout>
</template>
