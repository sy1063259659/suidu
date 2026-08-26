<script setup lang="ts">
import axios from 'axios'
import { computed, onMounted, onUnmounted, ref, shallowRef, watch } from 'vue'
import { ArrowLeft, ClipboardList, ClipboardPaste, Copy, Download, ExternalLink, Eye, KeyRound, Link, Link2Off, LogIn, Paperclip, RefreshCw, Search, Send, Share2, ShieldCheck, Star, Tags, Trash2, UploadCloud, UserPlus, Users } from '@lucide/vue'
import {
  NAlert, NButton, NButtonGroup, NCard, NDynamicTags, NEmpty, NInput, NLayout, NLayoutContent, NLayoutHeader,
  NDatePicker, NList, NListItem, NModal, NPopconfirm, NProgress, NSelect, NSpace, NSpin, NTag, NText, NUpload, NUploadDragger,
  type UploadCustomRequestOptions,
} from 'naive-ui'
import {
  changePassword, createUser, getCurrentUser, listUsers, login, logout, resetUserPassword,
  setUserDisabled, type User,
} from './api/auth'
import { clipboardContentUrl, createClipboard, deleteClipboard, getClipboard, listClipboard, updateClipboardMetadata, uploadClipboardFile, type ClipboardItem, type ClipboardItemKind } from './api/clipboard'
import { createClipboardShare, listClipboardShares, publicShareUrl, revokeClipboardShare, type ClipboardShare } from './api/shares'
import ClipboardItemContent from './components/ClipboardItemContent.vue'
import ClipboardDetailPage from './components/ClipboardDetailPage.vue'
import PublicSharePage from './components/PublicSharePage.vue'
import { captureResultsHeight, preservedResultsStyle } from './utils/historyLayout'
import {
  applyCustomTimeFilter,
  applyTimePreset,
  resolveTimeRange,
  toClipboardTimeParams,
  type DateRangeValue,
  type TimeFilterPreset,
} from './utils/timeline'

const publicToken = window.location.pathname.match(/^\/s\/([A-Za-z0-9_-]+)\/?$/)?.[1] ?? ''
function detailIDFromPath() {
  const raw = window.location.pathname.match(/^\/items\/(\d+)\/?$/)?.[1]
  const parsed = raw ? Number(raw) : 0
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : 0
}

const items = ref<ClipboardItem[]>([])
const currentUser = ref<User | null>(null)
const detailItemId = ref(detailIDFromPath())
const detailItem = shallowRef<ClipboardItem | null>(null)
const detailLoading = ref(false)
const detailUnavailable = ref(false)
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
const refreshLoading = ref(false)
const copiedId = ref<number | null>(null)
const copiedShareId = ref<number | null>(null)
const uploadError = ref('')
const uploadProgress = ref<Record<string, number>>({})
const shares = ref<ClipboardShare[]>([])
const sharesLoading = ref(false)
const shareModalOpen = ref(false)
const shareCreating = ref(false)
const shareItem = shallowRef<ClipboardItem | null>(null)
const shareTTL = ref(86400)
const createdShare = shallowRef<ClipboardShare | null>(null)
const shareError = ref('')
const activeView = ref<'clipboard' | 'shares' | 'admin'>('clipboard')
const searchQuery = ref('')
const itemKindFilter = ref<'all' | ClipboardItemKind>('all')
const favoritesOnly = ref(false)
const timePreset = ref<TimeFilterPreset>('all')
const customTimeRange = ref<DateRangeValue>(null)
const historyResultsElement = ref<HTMLElement | null>(null)
const favoriteResultsMinHeight = ref(0)
const favoritesFilterTransitioning = ref(false)
const favoriteUpdatingId = ref<number | null>(null)
const tagModalOpen = ref(false)
const tagItem = shallowRef<ClipboardItem | null>(null)
const noteDraft = ref('')
const tagDraft = ref<string[]>([])
const tagSaving = ref(false)
const tagError = ref('')
const itemKindOptions: Array<{ label: string; value: 'all' | ClipboardItemKind }> = [
  { label: '全部', value: 'all' },
  { label: '文本', value: 'text' },
  { label: '图片', value: 'image' },
  { label: '文件', value: 'file' },
]
const timePresetOptions: Array<{ label: string; value: Exclude<TimeFilterPreset, 'custom'> }> = [
  { label: '全部时间', value: 'all' },
  { label: '今天', value: 'today' },
  { label: '近 7 天', value: 'last7Days' },
  { label: '近 30 天', value: 'last30Days' },
]
const shareExpiryOptions = [
  { label: '1 小时', value: 3600 },
  { label: '1 天', value: 86400 },
  { label: '7 天', value: 604800 },
  { label: '30 天', value: 2592000 },
]
const canSubmit = computed(() => draft.value.trim().length > 0 && !submitting.value)
const isAdmin = computed(() => currentUser.value?.role === 'admin')
const isDetailPage = computed(() => detailItemId.value > 0)
const uploadingCount = computed(() => Object.keys(uploadProgress.value).length)
const averageUploadProgress = computed(() => {
  const values = Object.values(uploadProgress.value)
  return values.length ? Math.round(values.reduce((sum, value) => sum + value, 0) / values.length) : 0
})
const hasActiveFilters = computed(() => (
  searchQuery.value.trim() !== ''
  || itemKindFilter.value !== 'all'
  || favoritesOnly.value
  || timePreset.value !== 'all'
))
const emptyHistoryDescription = computed(() => hasActiveFilters.value ? '没有找到匹配的记录' : '还没有剪贴板记录')
const historyResultsStyle = computed(() => preservedResultsStyle(
  favoriteResultsMinHeight.value,
  favoritesOnly.value || favoritesFilterTransitioning.value,
))

let searchTimer: ReturnType<typeof window.setTimeout> | undefined
let listRequestSequence = 0

function errorMessage(errorValue: unknown, fallback: string) {
  if (axios.isAxiosError(errorValue) && typeof errorValue.response?.data?.error === 'string') return errorValue.response.data.error
  return fallback
}

async function loadItems() {
  const requestSequence = ++listRequestSequence
  loading.value = true
  error.value = ''
  if (apiStatus.value !== 'online') apiStatus.value = 'checking'
  try {
    const timeParams = toClipboardTimeParams(resolveTimeRange(timePreset.value, customTimeRange.value, new Date()))
    const result = await listClipboard({
      query: searchQuery.value,
      kind: itemKindFilter.value === 'all' ? undefined : itemKindFilter.value,
      favoriteOnly: favoritesOnly.value,
      createdFrom: timeParams.createdFrom,
      createdBefore: timeParams.createdBefore,
    })
    if (requestSequence !== listRequestSequence) return
    items.value = result
    apiStatus.value = 'online'
  } catch {
    if (requestSequence !== listRequestSequence) return
    apiStatus.value = 'offline'
    error.value = '无法连接服务，请稍后重试。'
  } finally {
    if (requestSequence === listRequestSequence) {
      loading.value = false
      favoritesFilterTransitioning.value = false
    }
  }
}

async function loadDetailItem() {
  if (!detailItemId.value) return
  const requestedID = detailItemId.value
  detailLoading.value = true
  detailUnavailable.value = false
  try {
    const item = await getClipboard(requestedID)
    if (detailItemId.value === requestedID) detailItem.value = item
  } catch (errorValue) {
    if (detailItemId.value !== requestedID) return
    detailItem.value = null
    detailUnavailable.value = axios.isAxiosError(errorValue) && errorValue.response?.status === 404
    if (!detailUnavailable.value) error.value = '无法加载记录详情，请稍后重试。'
  } finally {
    if (detailItemId.value === requestedID) detailLoading.value = false
  }
}

async function loadAdminUsers() {
  if (!isAdmin.value) return
  adminLoading.value = true
  try { adminUsers.value = await listUsers() } catch (errorValue) { adminError.value = errorMessage(errorValue, '无法加载用户列表。') } finally { adminLoading.value = false }
}

async function loadShares() {
  sharesLoading.value = true
  shareError.value = ''
  try { shares.value = await listClipboardShares() } catch (errorValue) { shareError.value = errorMessage(errorValue, '无法加载分享链接。') } finally { sharesLoading.value = false }
}

async function loadSession() {
  authLoading.value = true
  try {
    currentUser.value = await getCurrentUser()
    await Promise.all([loadItems(), loadShares(), loadDetailItem()])
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
  try { currentUser.value = await login(loginUsername.value, loginPassword.value); loginPassword.value = ''; await Promise.all([loadItems(), loadShares(), loadDetailItem()]); await loadAdminUsers() } catch (errorValue) { loginError.value = errorMessage(errorValue, '登录失败，请稍后重试。') } finally { loginLoading.value = false }
}

async function submitLogout() {
  try { await logout() } finally { listRequestSequence++; currentUser.value = null; items.value = []; detailItem.value = null; shares.value = []; adminUsers.value = []; passwordFormOpen.value = false; tagModalOpen.value = false; activeView.value = 'clipboard' }
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
  try {
    const item = await createClipboard(draft.value.trim())
    if (hasActiveFilters.value) void loadItems()
    else items.value = [item, ...items.value]
    draft.value = ''
    apiStatus.value = 'online'
  } catch { apiStatus.value = 'offline'; error.value = '提交失败，请稍后重试。' } finally { submitting.value = false }
}

interface UploadCallbacks {
  onProgress?: (percent: number) => void
  onFinish?: () => void
  onError?: () => void
}

async function submitFileUpload(file: File, uploadId: string, callbacks: UploadCallbacks = {}) {
  if (file.size > 100 * 1024 * 1024) {
    uploadError.value = `${file.name} 超过 100 MiB。`
    callbacks.onError?.()
    return
  }
  uploadError.value = ''
  uploadProgress.value = { ...uploadProgress.value, [uploadId]: 0 }
  try {
    const item = await uploadClipboardFile(file, 'web', (percent) => {
      uploadProgress.value = { ...uploadProgress.value, [uploadId]: percent }
      callbacks.onProgress?.(percent)
    })
    if (hasActiveFilters.value) void loadItems()
    else items.value = [item, ...items.value]
    apiStatus.value = 'online'
    callbacks.onFinish?.()
  } catch (errorValue) {
    uploadError.value = errorMessage(errorValue, `${file.name} 上传失败。`)
    callbacks.onError?.()
  } finally {
    const next = { ...uploadProgress.value }
    delete next[uploadId]
    uploadProgress.value = next
  }
}

function uploadFile(options: UploadCustomRequestOptions) {
  const rawFile = options.file.file
  if (!rawFile) { options.onError(); return }
  void submitFileUpload(rawFile, options.file.id, {
    onProgress: (percent) => options.onProgress({ percent }),
    onFinish: options.onFinish,
    onError: options.onError,
  })
}

function handlePaste(event: ClipboardEvent) {
  if (!currentUser.value || activeView.value !== 'clipboard' || isDetailPage.value || shareModalOpen.value || tagModalOpen.value || passwordFormOpen.value) return
  const clipboard = event.clipboardData
  if (!clipboard) return
  const files = Array.from(clipboard.files)
  if (!files.length) {
    for (const item of Array.from(clipboard.items)) {
      if (item.kind !== 'file') continue
      const file = item.getAsFile()
      if (file) files.push(file)
    }
  }
  if (!files.length) return
  event.preventDefault()
  const batchId = Date.now()
  files.forEach((file, index) => {
    void submitFileUpload(file, `paste-${batchId}-${index}`)
  })
}

async function copyItem(item: ClipboardItem) {
  if (!navigator.clipboard?.writeText) { error.value = '当前浏览器不支持写入剪贴板。'; return }
  try { await navigator.clipboard.writeText(item.content || ''); copiedId.value = item.id; window.setTimeout(() => { if (copiedId.value === item.id) copiedId.value = null }, 1600) } catch { error.value = '复制失败，请检查浏览器剪贴板权限。' }
}

async function removeItem(item: ClipboardItem) {
  try {
    await deleteClipboard(item.id)
    items.value = items.value.filter((current) => current.id !== item.id)
    shares.value = shares.value.filter((share) => share.item.id !== item.id)
    if (detailItemId.value === item.id) closeDetail()
  } catch (errorValue) { error.value = errorMessage(errorValue, '删除失败，请稍后重试。') }
}

function replaceItem(updated: ClipboardItem) {
  if (favoritesOnly.value && !updated.favorite) items.value = items.value.filter((item) => item.id !== updated.id)
  else items.value = items.value.map((item) => item.id === updated.id ? updated : item)
  shares.value = shares.value.map((share) => share.item.id === updated.id ? { ...share, item: updated } : share)
  if (detailItemId.value === updated.id) detailItem.value = updated
}

function openDetail(item: ClipboardItem) {
  detailItemId.value = item.id
  detailItem.value = item
  detailUnavailable.value = false
  window.history.pushState({ suiduDetail: true }, '', `/items/${item.id}`)
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

function closeDetail() {
  if (window.history.state?.suiduDetail) {
    window.history.back()
    return
  }
  detailItemId.value = 0
  detailItem.value = null
  detailUnavailable.value = false
  window.history.pushState({}, '', '/')
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

function handlePopState() {
  const nextID = detailIDFromPath()
  detailItemId.value = nextID
  detailItem.value = nextID ? items.value.find((item) => item.id === nextID) ?? null : null
  detailUnavailable.value = false
  if (nextID && currentUser.value && !detailItem.value) void loadDetailItem()
}

async function toggleFavorite(item: ClipboardItem) {
  if (favoriteUpdatingId.value === item.id) return
  favoriteUpdatingId.value = item.id
  error.value = ''
  try {
    replaceItem(await updateClipboardMetadata(item.id, item.note ?? '', item.tags ?? [], !item.favorite))
  } catch (errorValue) {
    error.value = errorMessage(errorValue, '更新收藏状态失败。')
  } finally {
    if (favoriteUpdatingId.value === item.id) favoriteUpdatingId.value = null
  }
}

function openTagModal(item: ClipboardItem) {
  tagItem.value = item
  noteDraft.value = item.note ?? ''
  tagDraft.value = [...(item.tags ?? [])]
  tagError.value = ''
  tagModalOpen.value = true
}

async function saveTags() {
  if (!tagItem.value || tagSaving.value) return
  tagSaving.value = true
  tagError.value = ''
  try {
    replaceItem(await updateClipboardMetadata(tagItem.value.id, noteDraft.value, tagDraft.value, Boolean(tagItem.value.favorite)))
    if (searchQuery.value.trim()) void loadItems()
    tagModalOpen.value = false
  } catch (errorValue) {
    tagError.value = errorMessage(errorValue, '保存失败，请检查备注和标签长度。')
  } finally { tagSaving.value = false }
}

function searchTag(tag: string) {
  searchQuery.value = tag
}

function prepareFavoritesFilterTransition() {
  favoriteResultsMinHeight.value = captureResultsHeight(historyResultsElement.value?.getBoundingClientRect().height ?? 0)
  favoritesFilterTransitioning.value = true
}

function toggleFavoritesFilter() {
  prepareFavoritesFilterTransition()
  favoritesOnly.value = !favoritesOnly.value
}

function selectTimePreset(preset: Exclude<TimeFilterPreset, 'custom'>) {
  const next = applyTimePreset(preset)
  timePreset.value = next.preset
  customTimeRange.value = next.customRange
}

function handleCustomTimeRangeChange(value: DateRangeValue) {
  const next = applyCustomTimeFilter(value)
  timePreset.value = next.preset
  customTimeRange.value = next.customRange
}

function clearFilters() {
  if (!hasActiveFilters.value) return
  if (favoritesOnly.value) prepareFavoritesFilterTransition()
  searchQuery.value = ''
  itemKindFilter.value = 'all'
  favoritesOnly.value = false
  const next = applyTimePreset('all')
  timePreset.value = next.preset
  customTimeRange.value = next.customRange
}

function openShareModal(item: ClipboardItem) {
  shareItem.value = item
  shareTTL.value = 86400
  createdShare.value = null
  shareError.value = ''
  shareModalOpen.value = true
}

async function submitShare() {
  if (!shareItem.value || shareCreating.value) return
  shareCreating.value = true
  shareError.value = ''
  try {
    createdShare.value = await createClipboardShare(shareItem.value.id, shareTTL.value)
    await loadShares()
  } catch (errorValue) {
    shareError.value = errorMessage(errorValue, '创建分享链接失败。')
  } finally { shareCreating.value = false }
}

async function copyShareLink(share: ClipboardShare) {
  if (!navigator.clipboard?.writeText) { shareError.value = '当前浏览器不支持复制链接。'; return }
  try {
    await navigator.clipboard.writeText(publicShareUrl(share.token))
    copiedShareId.value = share.id
    window.setTimeout(() => { if (copiedShareId.value === share.id) copiedShareId.value = null }, 1600)
  } catch { shareError.value = '复制链接失败。' }
}

async function revokeShare(share: ClipboardShare) {
  try { await revokeClipboardShare(share.id); await loadShares() } catch (errorValue) { shareError.value = errorMessage(errorValue, '关闭分享链接失败。') }
}

function shareStatus(share: ClipboardShare): 'active' | 'expired' | 'revoked' {
  if (share.revokedAt) return 'revoked'
  return new Date(share.expiresAt).getTime() <= Date.now() ? 'expired' : 'active'
}

async function refreshActiveView() {
  if (refreshLoading.value) return
  refreshLoading.value = true
  try {
    if (isDetailPage.value) await loadDetailItem()
    else if (activeView.value === 'shares') await loadShares()
    else if (activeView.value === 'admin') await loadAdminUsers()
    else await loadItems()
  } finally { refreshLoading.value = false }
}

function formatDate(value: string) { return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value)) }
function handleKeydown(event: KeyboardEvent) { if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') { event.preventDefault(); void submitClipboard() } }

watch([searchQuery, itemKindFilter, favoritesOnly, timePreset, customTimeRange], () => {
  if (searchTimer) window.clearTimeout(searchTimer)
  listRequestSequence++
  loading.value = false
  searchTimer = window.setTimeout(() => {
    if (currentUser.value && activeView.value === 'clipboard') void loadItems()
  }, 300)
})

onMounted(() => {
  if (!publicToken) {
    void loadSession()
    document.addEventListener('paste', handlePaste)
    window.addEventListener('popstate', handlePopState)
  }
})
onUnmounted(() => {
  document.removeEventListener('paste', handlePaste)
  window.removeEventListener('popstate', handlePopState)
  if (searchTimer) window.clearTimeout(searchTimer)
})
</script>

<template>
  <PublicSharePage v-if="publicToken" :token="publicToken" />
  <div v-else-if="authLoading" class="auth-loading"><n-spin size="medium" /></div>
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
        <n-button v-if="!isDetailPage" :type="activeView === 'shares' ? 'primary' : 'default'" @click="activeView = activeView === 'shares' ? 'clipboard' : 'shares'"><template #icon><Share2 v-if="activeView !== 'shares'" :size="16" /><ArrowLeft v-else :size="16" /></template>{{ activeView === 'shares' ? '返回剪贴板' : '分享管理' }}
        </n-button>
        <n-button v-if="isAdmin && !isDetailPage" :type="activeView === 'admin' ? 'primary' : 'default'" @click="activeView = activeView === 'admin' ? 'clipboard' : 'admin'"><template #icon><ShieldCheck v-if="activeView !== 'admin'" :size="16" /><ArrowLeft v-else :size="16" /></template>{{ activeView === 'admin' ? '返回剪贴板' : '管理中心' }}
        </n-button>
        <n-tag :type="isAdmin ? 'warning' : 'info'">{{ currentUser.username }} · {{ isAdmin ? '管理员' : '普通用户' }}
        </n-tag>
        <n-button quaternary @click="passwordFormOpen = !passwordFormOpen">修改密码
        </n-button>
        <n-button quaternary @click="submitLogout">退出
        </n-button>
        <n-tag :type="apiStatus === 'online' ? 'success' : apiStatus === 'offline' ? 'error' : 'warning'">API {{ apiStatus === 'online' ? '在线' : apiStatus === 'offline' ? '离线' : '检查中' }}
        </n-tag>
        <n-button quaternary circle aria-label="刷新内容" title="刷新内容" :loading="refreshLoading" @click="refreshActiveView"><template #icon><RefreshCw :size="17" /></template>
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
        <ClipboardDetailPage v-if="isDetailPage" :item="detailItem" :loading="detailLoading" :unavailable="detailUnavailable" :favorite-updating="favoriteUpdatingId === detailItemId" :copied="copiedId === detailItemId" @back="closeDetail" @copy="copyItem" @favorite="toggleFavorite" @organize="openTagModal" @share="openShareModal" @delete="removeItem" />
        <main v-else-if="activeView === 'admin'" class="admin-page">
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
        <main v-else-if="activeView === 'shares'" class="shares-page">
          <section class="page-intro"><div><p class="eyebrow">PUBLIC LINKS</p><h1>分享管理</h1><p class="intro-copy">查看公开链接的有效期，或随时让链接失效。</p></div><n-tag round :bordered="false" type="info">{{ shares.length }} 个链接</n-tag></section>
          <n-alert v-if="shareError" type="error" closable class="error-alert" @close="shareError = ''">{{ shareError }}</n-alert>
          <div v-if="sharesLoading && !shares.length" class="loading-state"><n-spin size="medium" /></div>
          <n-empty v-else-if="!shares.length" description="还没有公开分享" class="empty-state" />
          <n-list v-else class="share-list" bordered>
            <n-list-item v-for="share in shares" :key="share.id">
              <div class="share-row">
                <div class="share-row-heading">
                  <n-space align="center" :size="8"><Link :size="16" /><strong>{{ share.item.kind === 'text' ? '文本分享' : share.item.fileName }}</strong></n-space>
                  <n-tag size="small" :type="shareStatus(share) === 'active' ? 'success' : shareStatus(share) === 'expired' ? 'warning' : 'error'">{{ shareStatus(share) === 'active' ? '有效' : shareStatus(share) === 'expired' ? '已过期' : '已失效' }}</n-tag>
                </div>
                <div class="share-item-preview"><ClipboardItemContent :item="share.item" /></div>
                <div class="share-link-line">{{ publicShareUrl(share.token) }}</div>
                <div class="share-row-footer">
                  <n-text depth="3">创建于 {{ formatDate(share.createdAt) }} · 有效期至 {{ formatDate(share.expiresAt) }}</n-text>
                  <n-space v-if="shareStatus(share) === 'active'" :size="4">
                    <n-button quaternary circle :aria-label="copiedShareId === share.id ? '已复制' : '复制分享链接'" :title="copiedShareId === share.id ? '已复制' : '复制分享链接'" @click="copyShareLink(share)"><template #icon><Copy :size="16" /></template></n-button>
                    <n-button tag="a" :href="publicShareUrl(share.token)" target="_blank" rel="noopener" quaternary circle aria-label="打开分享链接" title="打开分享链接"><template #icon><ExternalLink :size="16" /></template></n-button>
                    <n-popconfirm @positive-click="revokeShare(share)"><template #trigger><n-button quaternary circle aria-label="让链接失效" title="让链接失效"><template #icon><Link2Off :size="16" /></template></n-button></template>确定立即关闭这个分享链接吗？</n-popconfirm>
                  </n-space>
                </div>
              </div>
            </n-list-item>
          </n-list>
        </main>
        <main v-else class="clipboard-page">
          <section class="page-intro"><div><p class="eyebrow">CLIPBOARD</p><h1>剪贴板</h1><p class="intro-copy">在手机和电脑之间传递文字、图片和文件。</p></div><n-tag round :bordered="false" type="info">{{ items.length }} 条{{ hasActiveFilters ? '匹配' : '记录' }}
          </n-tag></section>
          <n-card class="composer-card" :bordered="false"><n-input v-model:value="draft" type="textarea" placeholder="输入或粘贴要传递的文字..." :autosize="{ minRows: 5, maxRows: 12 }" maxlength="1048576" show-count @keydown="handleKeydown" /><div class="composer-actions"><n-button secondary @click="readClipboard"><template #icon><ClipboardPaste :size="17" /></template>读取剪贴板</n-button><n-button type="primary" :disabled="!canSubmit" :loading="submitting" @click="submitClipboard"><template #icon><Send :size="17" /></template>提交文本</n-button></div></n-card>
          <n-card class="upload-card" :bordered="false">
            <n-upload multiple :show-file-list="false" :custom-request="uploadFile">
              <n-upload-dragger><div class="upload-drop-content"><div class="upload-icon"><UploadCloud :size="22" /></div><div><strong>上传图片或文件</strong><span>点击、拖入，或直接 Ctrl+V / ⌘V 粘贴截图，单个不超过 100 MiB</span></div></div></n-upload-dragger>
            </n-upload>
            <div v-if="uploadingCount" class="upload-status"><n-space justify="space-between"><n-text depth="3">正在上传 {{ uploadingCount }} 个文件</n-text><n-text depth="3">{{ averageUploadProgress }}%</n-text></n-space><n-progress type="line" :percentage="averageUploadProgress" :show-indicator="false" processing /></div>
          </n-card>
          <n-alert v-if="uploadError" type="error" closable class="error-alert" @close="uploadError = ''">{{ uploadError }}
          </n-alert>
          <n-alert v-if="error" type="error" closable class="error-alert" @close="error = ''">{{ error }}
          </n-alert>
          <section class="history-section" :class="{ 'is-loading': loading }" :aria-busy="loading">
            <div class="history-progress-rail" aria-hidden="true"><span /></div>
            <div class="section-heading">
              <div><p class="eyebrow">HISTORY</p><h2>剪贴板记录</h2></div>
              <div class="history-heading-status"><n-text depth="3">{{ hasActiveFilters ? `找到 ${items.length} 条` : '按时间倒序' }}</n-text></div>
            </div>
            <div class="history-toolbar">
              <n-input v-model:value="searchQuery" class="history-search" clearable maxlength="200" placeholder="搜索文本、文件名、备注或标签" aria-label="搜索剪贴板记录">
                <template #prefix><Search :size="17" /></template>
              </n-input>
              <div class="history-filter-scroll" role="group" aria-label="按类型筛选">
                <n-button-group class="history-filter-group">
                  <n-button v-for="option in itemKindOptions" :key="option.value" :type="itemKindFilter === option.value ? 'primary' : 'default'" :secondary="itemKindFilter === option.value" @click="itemKindFilter = option.value">
                    {{ option.label }}
                  </n-button>
                </n-button-group>
              </div>
              <div class="history-filter-scroll" role="group" aria-label="按时间筛选">
                <n-button-group class="history-filter-group">
                  <n-button v-for="option in timePresetOptions" :key="option.value" :type="timePreset === option.value ? 'primary' : 'default'" :secondary="timePreset === option.value" @click="selectTimePreset(option.value)">
                    {{ option.label }}
                  </n-button>
                </n-button-group>
              </div>
              <n-date-picker :value="customTimeRange" class="history-date-range" type="daterange" clearable format="yyyy-MM-dd" start-placeholder="开始日期" end-placeholder="结束日期" @update:value="handleCustomTimeRangeChange" />
              <n-button class="favorites-filter" :type="favoritesOnly ? 'warning' : 'default'" :secondary="favoritesOnly" @click="toggleFavoritesFilter">
                <template #icon><Star :size="16" :fill="favoritesOnly ? 'currentColor' : 'none'" /></template>收藏
              </n-button>
              <n-button v-if="hasActiveFilters" quaternary class="history-reset-filters" @click="clearFilters">清除筛选
              </n-button>
            </div>
            <div ref="historyResultsElement" class="history-results" :style="historyResultsStyle">
            <n-empty v-if="!items.length" :description="loading ? '正在加载记录' : emptyHistoryDescription" class="empty-state" />
            <n-list v-else class="history-list" bordered>
              <n-list-item v-for="item in items" :key="item.id">
                <div class="history-item">
                  <ClipboardItemContent :item="item" preview @view-detail="openDetail(item)" />
                  <p v-if="item.note" class="item-note">{{ item.note }}</p>
                  <div v-if="item.tags?.length" class="item-tags" aria-label="记录标签">
                    <n-tag v-for="tag in item.tags" :key="tag" size="small" round :bordered="false" type="info" class="item-tag" role="button" tabindex="0" @click="searchTag(tag)" @keydown.enter="searchTag(tag)">{{ tag }}</n-tag>
                  </div>
                  <div class="history-meta">
                    <n-space :size="8" align="center"><n-tag size="small" :bordered="false"><template #icon><Paperclip v-if="item.kind !== 'text'" :size="12" /></template>{{ item.kind === 'image' ? '图片' : item.kind === 'file' ? '文件' : item.source || 'web' }}</n-tag><n-text depth="3">{{ formatDate(item.createdAt) }}</n-text></n-space>
                    <n-space :size="4">
                      <n-button quaternary circle :type="item.favorite ? 'warning' : 'default'" :disabled="favoriteUpdatingId === item.id" :aria-busy="favoriteUpdatingId === item.id" :aria-label="item.favorite ? '取消收藏' : '收藏记录'" :title="item.favorite ? '取消收藏' : '收藏记录'" @click="toggleFavorite(item)"><template #icon><Star :size="16" :fill="item.favorite ? 'currentColor' : 'none'" /></template></n-button>
                      <n-button v-if="item.kind === 'text' || !item.kind" quaternary circle :aria-label="copiedId === item.id ? '已复制' : '复制记录'" :title="copiedId === item.id ? '已复制' : '复制记录'" @click="copyItem(item)"><template #icon><Copy :size="16" /></template></n-button>
                      <n-button v-else tag="a" :href="clipboardContentUrl(item.id, true)" quaternary circle aria-label="下载文件" title="下载文件"><template #icon><Download :size="16" /></template></n-button>
                      <n-button quaternary circle aria-label="查看详情" title="查看完整详情" @click="openDetail(item)"><template #icon><Eye :size="16" /></template></n-button>
                      <n-button quaternary circle aria-label="整理记录" title="添加备注和标签" @click="openTagModal(item)"><template #icon><Tags :size="16" /></template></n-button>
                      <n-button quaternary circle aria-label="公开分享" title="公开分享" @click="openShareModal(item)"><template #icon><Share2 :size="16" /></template></n-button>
                      <n-popconfirm @positive-click="removeItem(item)"><template #trigger><n-button quaternary circle aria-label="删除记录" title="删除记录"><template #icon><Trash2 :size="16" /></template></n-button></template>确定删除这条记录吗？</n-popconfirm>
                    </n-space>
                  </div>
                </div>
              </n-list-item>
            </n-list>
            </div>
          </section>
        </main>
    </n-layout-content>
    <n-modal v-model:show="shareModalOpen" preset="card" title="公开分享" class="share-modal">
      <template v-if="createdShare">
        <n-alert type="success">分享链接已创建。</n-alert>
        <n-input :value="publicShareUrl(createdShare.token)" readonly class="share-link-input" />
        <n-space justify="end"><n-button secondary @click="copyShareLink(createdShare)"><template #icon><Copy :size="16" /></template>{{ copiedShareId === createdShare.id ? '已复制' : '复制链接' }}</n-button><n-button tag="a" :href="publicShareUrl(createdShare.token)" target="_blank" rel="noopener" type="primary"><template #icon><ExternalLink :size="16" /></template>打开链接</n-button></n-space>
      </template>
      <template v-else>
        <p class="share-modal-copy">任何拿到链接的人都可以查看或下载这条内容。</p>
        <label class="share-expiry-label">有效期</label>
        <n-select v-model:value="shareTTL" :options="shareExpiryOptions" />
        <n-alert v-if="shareError" type="error" class="form-alert">{{ shareError }}</n-alert>
        <n-space justify="end" class="share-modal-actions"><n-button @click="shareModalOpen = false">取消</n-button><n-button type="primary" :loading="shareCreating" @click="submitShare"><template #icon><Share2 :size="16" /></template>生成链接</n-button></n-space>
      </template>
    </n-modal>
    <n-modal v-model:show="tagModalOpen" preset="card" title="整理记录" class="tag-modal" :mask-closable="!tagSaving">
      <label class="organize-field-label" for="clipboard-note">备注</label>
      <n-input id="clipboard-note" v-model:value="noteDraft" type="textarea" maxlength="500" show-count :autosize="{ minRows: 3, maxRows: 7 }" placeholder="用一两句话说明这条内容是做什么的..." />
      <div class="tag-field-heading"><span class="organize-field-label">标签</span><n-text depth="3">最多 10 个，每个 24 字</n-text></div>
      <n-dynamic-tags v-model:value="tagDraft" :max="10" round type="info" :input-props="{ maxlength: 24, placeholder: '输入标签' }" />
      <n-alert v-if="tagError" type="error" class="form-alert">{{ tagError }}</n-alert>
      <n-space justify="end" class="tag-modal-actions"><n-button :disabled="tagSaving" @click="tagModalOpen = false">取消</n-button><n-button type="primary" :loading="tagSaving" @click="saveTags">保存</n-button></n-space>
    </n-modal>
  </n-layout>
</template>
