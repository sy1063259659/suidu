<script setup lang="ts">
import axios from 'axios'
import { computed, onMounted, ref } from 'vue'
import { ArrowLeft, ClipboardList, ClipboardPaste, Copy, Download, KeyRound, LogIn, Paperclip, RefreshCw, Send, ShieldCheck, Trash2, UploadCloud, UserPlus, Users } from '@lucide/vue'
import {
  NAlert, NButton, NCard, NEmpty, NInput, NLayout, NLayoutContent, NLayoutHeader,
  NList, NListItem, NPopconfirm, NProgress, NSpace, NSpin, NTag, NText, NUpload, NUploadDragger,
  type UploadCustomRequestOptions,
} from 'naive-ui'
import {
  changePassword, createUser, getCurrentUser, listUsers, login, logout, resetUserPassword,
  setUserDisabled, type User,
} from './api/auth'
import { clipboardContentUrl, createClipboard, deleteClipboard, listClipboard, uploadClipboardFile, type ClipboardItem } from './api/clipboard'
import ClipboardItemContent from './components/ClipboardItemContent.vue'

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
const uploadError = ref('')
const uploadProgress = ref<Record<string, number>>({})
const activeView = ref<'clipboard' | 'admin'>('clipboard')
const canSubmit = computed(() => draft.value.trim().length > 0 && !submitting.value)
const isAdmin = computed(() => currentUser.value?.role === 'admin')
const uploadingCount = computed(() => Object.keys(uploadProgress.value).length)
const averageUploadProgress = computed(() => {
  const values = Object.values(uploadProgress.value)
  return values.length ? Math.round(values.reduce((sum, value) => sum + value, 0) / values.length) : 0
})

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
  try { await logout() } finally { currentUser.value = null; items.value = []; adminUsers.value = []; passwordFormOpen.value = false; activeView.value = 'clipboard' }
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

function uploadFile(options: UploadCustomRequestOptions) {
  const rawFile = options.file.file
  if (!rawFile) { options.onError(); return }
  if (rawFile.size > 100 * 1024 * 1024) {
    uploadError.value = `${rawFile.name} 超过 100 MiB。`
    options.onError()
    return
  }
  uploadError.value = ''
  uploadProgress.value = { ...uploadProgress.value, [options.file.id]: 0 }
  void uploadClipboardFile(rawFile, 'web', (percent) => {
    uploadProgress.value = { ...uploadProgress.value, [options.file.id]: percent }
    options.onProgress({ percent })
  }).then((item) => {
    items.value = [item, ...items.value]
    apiStatus.value = 'online'
    options.onFinish()
  }).catch((errorValue) => {
    uploadError.value = errorMessage(errorValue, `${rawFile.name} 上传失败。`)
    options.onError()
  }).finally(() => {
    const next = { ...uploadProgress.value }
    delete next[options.file.id]
    uploadProgress.value = next
  })
}

async function copyItem(item: ClipboardItem) {
  if (!navigator.clipboard?.writeText) { error.value = '当前浏览器不支持写入剪贴板。'; return }
  try { await navigator.clipboard.writeText(item.content || ''); copiedId.value = item.id; window.setTimeout(() => { if (copiedId.value === item.id) copiedId.value = null }, 1600) } catch { error.value = '复制失败，请检查浏览器剪贴板权限。' }
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
    <div class="auth-shell">
      <section class="auth-brand-panel">
        <div class="auth-panel-top"><div class="auth-mark"><ClipboardList :size="22" /></div><span>SUIDU</span></div>
        <div class="auth-panel-content"><h1>随渡</h1></div>
      </section>
      <section class="auth-form-panel">
        <div class="auth-card-heading"><h2>登录</h2></div>
        <n-alert v-if="loginError" type="error" class="auth-alert">{{ loginError }}
        </n-alert>
        <div class="auth-field"><label>用户名</label><n-input v-model:value="loginUsername" placeholder="输入用户名" autocomplete="username" class="auth-input" @keyup.enter="submitLogin" /></div>
        <div class="auth-field"><label>密码</label><n-input v-model:value="loginPassword" type="password" show-password-on="click" placeholder="输入密码" autocomplete="current-password" class="auth-input" @keyup.enter="submitLogin" /></div>
        <n-button type="primary" block :loading="loginLoading" @click="submitLogin"><template #icon><LogIn :size="17" /></template>登录
        </n-button>
      </section>
    </div>
  </main>
  <n-layout v-else class="app-shell">
    <n-layout-header bordered class="app-header">
      <div><div class="brand">随渡 <span>SUIDU</span></div><div class="subtitle">把文字放在随手可取的地方</div></div>
      <n-space align="center" :size="12">
        <n-button v-if="isAdmin" :type="activeView === 'admin' ? 'primary' : 'default'" @click="activeView = activeView === 'admin' ? 'clipboard' : 'admin'"><template #icon><ShieldCheck v-if="activeView === 'clipboard'" :size="16" /><ArrowLeft v-else :size="16" /></template>{{ activeView === 'admin' ? '返回剪贴板' : '管理中心' }}
        </n-button>
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
        <n-card v-if="passwordFormOpen" class="account-card" :bordered="false">
          <div class="section-heading"><div><p class="eyebrow">ACCOUNT SECURITY</p><h2>修改密码</h2></div></div>
          <n-space vertical :size="10" class="form-stack"><n-input v-model:value="currentPassword" type="password" show-password-on="click" placeholder="当前密码" autocomplete="current-password" /><n-input v-model:value="newPassword" type="password" show-password-on="click" placeholder="新密码（至少 12 位）" autocomplete="new-password" /><n-space><n-button type="primary" :loading="passwordLoading" @click="submitPasswordChange">保存新密码</n-button><n-button quaternary @click="passwordFormOpen = false">取消</n-button></n-space></n-space>
          <n-alert v-if="passwordError" type="error" class="form-alert">{{ passwordError }}
          </n-alert><n-alert v-if="passwordSuccess" type="success" class="form-alert">{{ passwordSuccess }}
          </n-alert>
        </n-card>
        <main v-if="activeView === 'admin'" class="admin-page">
          <section class="admin-hero"><div><p class="eyebrow">ADMINISTRATION</p><h1>管理中心</h1><p class="intro-copy">管理随渡账号、登录权限和日常使用身份。</p></div><n-tag type="warning" :bordered="false"><template #icon><ShieldCheck :size="14" /></template>管理员</n-tag></section>
          <section class="admin-metrics"><div class="metric-item"><Users :size="18" /><div><span>用户总数</span><strong>{{ adminUsers.length }}</strong></div></div><div class="metric-item"><UserPlus :size="18" /><div><span>普通用户</span><strong>{{ adminUsers.filter((user) => user.role === 'user').length }}</strong></div></div><div class="metric-item"><KeyRound :size="18" /><div><span>当前账号</span><strong>{{ currentUser.username }}</strong></div></div></section>
          <section class="admin-grid">
            <div class="admin-panel create-panel"><div class="panel-heading"><div class="panel-icon"><UserPlus :size="18" /></div><div><h2>创建普通用户</h2><p>为日常使用创建一个非管理员账号。</p></div></div><n-space vertical :size="12" class="form-stack"><n-input v-model:value="newUsername" placeholder="用户名" autocomplete="off" /><n-input v-model:value="newUserPassword" type="password" show-password-on="click" placeholder="初始密码（至少 12 位）" autocomplete="new-password" /><n-button type="primary" block :disabled="!newUsername.trim() || !newUserPassword" @click="submitCreateUser"><template #icon><UserPlus :size="16" /></template>创建普通用户</n-button></n-space><n-alert v-if="adminError" type="error" class="form-alert">{{ adminError }}
              </n-alert><n-alert v-if="adminSuccess" type="success" class="form-alert">{{ adminSuccess }}
              </n-alert></div>
            <div class="admin-panel users-panel"><div class="panel-heading"><div class="panel-icon"><Users :size="18" /></div><div><h2>账号列表</h2><p>重置密码或调整普通用户的访问状态。</p></div><n-button quaternary circle aria-label="刷新账号列表" title="刷新账号列表" :loading="adminLoading" @click="loadAdminUsers"><template #icon><RefreshCw :size="16" /></template></n-button></div><n-empty v-if="adminLoading && !adminUsers.length" description="正在加载账号" /><n-list v-else class="user-list" bordered><n-list-item v-for="user in adminUsers" :key="user.id"><div class="user-row"><div class="user-identity"><strong>{{ user.username }}</strong><n-tag size="small" :type="user.role === 'admin' ? 'warning' : 'info'">{{ user.role === 'admin' ? '管理员' : '普通用户' }}</n-tag><n-tag v-if="user.disabled" size="small" type="error">已停用
                  </n-tag></div><n-space :size="4"><n-button v-if="user.role === 'user'" quaternary circle aria-label="重置密码" title="重置密码" @click="resetPasswordFor(user)"><template #icon><KeyRound :size="16" /></template></n-button><n-button v-if="user.role === 'user'" quaternary circle :aria-label="user.disabled ? '启用用户' : '停用用户'" :title="user.disabled ? '启用用户' : '停用用户'" @click="toggleUser(user)"><template #icon><ShieldCheck :size="16" /></template></n-button></n-space></div></n-list-item></n-list></div>
          </section>
        </main>
        <main v-else class="clipboard-page">
          <section class="page-intro"><div><p class="eyebrow">CLIPBOARD</p><h1>剪贴板</h1><p class="intro-copy">在手机和电脑之间传递文字、图片和文件。</p></div><n-tag round :bordered="false" type="info">{{ items.length }} 条记录
          </n-tag></section>
          <n-card class="composer-card" :bordered="false"><n-input v-model:value="draft" type="textarea" placeholder="输入或粘贴要传递的文字..." :autosize="{ minRows: 5, maxRows: 12 }" maxlength="1048576" show-count @keydown="handleKeydown" /><div class="composer-actions"><n-button secondary @click="readClipboard"><template #icon><ClipboardPaste :size="17" /></template>读取剪贴板</n-button><n-button type="primary" :disabled="!canSubmit" :loading="submitting" @click="submitClipboard"><template #icon><Send :size="17" /></template>提交文本</n-button></div></n-card>
          <n-card class="upload-card" :bordered="false">
            <n-upload multiple :show-file-list="false" :custom-request="uploadFile">
              <n-upload-dragger><div class="upload-drop-content"><div class="upload-icon"><UploadCloud :size="22" /></div><div><strong>上传图片或文件</strong><span>点击选择或拖到这里，单个文件不超过 100 MiB</span></div></div></n-upload-dragger>
            </n-upload>
            <div v-if="uploadingCount" class="upload-status"><n-space justify="space-between"><n-text depth="3">正在上传 {{ uploadingCount }} 个文件</n-text><n-text depth="3">{{ averageUploadProgress }}%</n-text></n-space><n-progress type="line" :percentage="averageUploadProgress" :show-indicator="false" processing /></div>
          </n-card>
          <n-alert v-if="uploadError" type="error" closable class="error-alert" @close="uploadError = ''">{{ uploadError }}
          </n-alert>
          <n-alert v-if="error" type="error" closable class="error-alert" @close="error = ''">{{ error }}
          </n-alert>
          <section class="history-section"><div class="section-heading"><div><p class="eyebrow">RECENT</p><h2>最近记录</h2></div><n-text depth="3">按时间倒序
          </n-text></div><div v-if="loading && !items.length" class="loading-state"><n-spin size="medium" /></div><n-empty v-else-if="!items.length" description="还没有剪贴板记录" class="empty-state" /><n-list v-else class="history-list" bordered><n-list-item v-for="item in items" :key="item.id"><div class="history-item"><ClipboardItemContent :item="item" /><div class="history-meta"><n-space :size="8" align="center"><n-tag size="small" :bordered="false"><template #icon><Paperclip v-if="item.kind !== 'text'" :size="12" /></template>{{ item.kind === 'image' ? '图片' : item.kind === 'file' ? '文件' : item.source || 'web' }}</n-tag><n-text depth="3">{{ formatDate(item.createdAt) }}</n-text></n-space><n-space :size="4"><n-button v-if="item.kind === 'text' || !item.kind" quaternary circle :aria-label="copiedId === item.id ? '已复制' : '复制记录'" :title="copiedId === item.id ? '已复制' : '复制记录'" @click="copyItem(item)"><template #icon><Copy :size="16" /></template></n-button><n-button v-else tag="a" :href="clipboardContentUrl(item.id, true)" quaternary circle aria-label="下载文件" title="下载文件"><template #icon><Download :size="16" /></template></n-button><n-popconfirm @positive-click="removeItem(item)"><template #trigger><n-button quaternary circle aria-label="删除记录" title="删除记录"><template #icon><Trash2 :size="16" /></template></n-button></template>确定删除这条记录吗？</n-popconfirm></n-space></div></div></n-list-item></n-list></section>
        </main>
    </n-layout-content>
  </n-layout>
</template>
