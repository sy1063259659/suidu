<script setup lang="ts">
import axios from 'axios'
import { computed, nextTick, onMounted, onUnmounted, ref, shallowRef, watch } from 'vue'
import { Archive, ArrowLeft, ClipboardList, ClipboardPaste, Copy, Download, ExternalLink, KeyRound, Link, Link2Off, LogIn, Maximize2, MoreHorizontal, Paperclip, RefreshCw, Search, Send, Share2, ShieldCheck, Star, Trash2, UploadCloud, UserPlus, Users } from '@lucide/vue'
import {
  NAlert, NButton, NButtonGroup, NCard, NDynamicTags, NEmpty, NInput, NLayout, NLayoutContent, NLayoutHeader,
  NDatePicker, NDropdown, NList, NListItem, NModal, NPopconfirm, NProgress, NSelect, NSpace, NSpin, NTag, NText, NUpload, NUploadDragger,
  type UploadCustomRequestOptions,
} from 'naive-ui'
import {
  changePassword, createUser, getCurrentUser, listUsers, login, logout, resetUserPassword,
  setUserDisabled, type User,
} from './api/auth'
import { createClipboard, deleteClipboard, getChangesVersion, getClipboard, listClipboard, listTrash, purgeClipboard, restoreClipboard, updateClipboardMetadata, uploadClipboardFile, type ClipboardItem, type ClipboardItemKind } from './api/clipboard'
import { createClipboardShare, listClipboardShares, publicShareUrl, revokeClipboardShare, type ClipboardShare } from './api/shares'
import { downloadBackup, importBackup, type ImportBackupResult } from './api/backup'
import ClipboardItemContent from './components/ClipboardItemContent.vue'
import ClipboardDetailPage from './components/ClipboardDetailPage.vue'
import PublicSharePage from './components/PublicSharePage.vue'
import ItemActions from './components/ItemActions.vue'
import TrashPage from './components/TrashPage.vue'
import { clearDraft, readDraft, readStoredDraft, writeDraft } from './utils/draftStorage'
import { readLibraryState, writeLibraryState } from './utils/libraryState'
import { captureResultsHeight, preservedResultsStyle } from './utils/historyLayout'
import { MAX_TEXT_BYTES, formatTextBytes, textStats } from './utils/textEditor'
import { createAutoSync } from './utils/autoSync'
import { captureHistoryAnchors, reconcileItems, restoreHistoryAnchor } from './utils/liveHistory'
import {
  applyCustomTimeFilter,
  applyTimePreset,
  groupTimelineItemsByLocalDay,
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
let sessionGeneration = 0
function isCurrentSession(generation: number) { return generation === sessionGeneration && currentUser.value !== null }
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
const draftRestored = ref(false)
const draftStorageFailed = ref(false)
const composeExpanded = ref(false)
const editExpanded = ref(false)
const editRestored = ref(false)
const feedback = ref<{ text: string; kind: 'success' | 'error'; undoId?: number } | null>(null)
let feedbackTimer: ReturnType<typeof setTimeout> | undefined
function notify(text: string, kind: 'success' | 'error' = 'success', undoId?: number) {
  if (feedbackTimer) clearTimeout(feedbackTimer)
  feedback.value = { text, kind, undoId }
  feedbackTimer = setTimeout(() => { feedback.value = null }, undoId ? 9000 : 4500)
}
function storage(kind: 'localStorage' | 'sessionStorage') {
  try { return window[kind] } catch { return undefined }
}
function persistDrafts() {
  const userID = currentUser.value?.id
  if (!userID) return
  const local = storage('localStorage')
  let saved = local ? writeDraft(local, userID, draft.value) : false
  if (editItem.value) {
    const itemKey = `item:${editItem.value.id}`
    const editSaved = Boolean(local && (editDraft.value === (editItem.value.content ?? '')
      ? clearDraft(local, userID, itemKey) : writeDraft(local, userID, editDraft.value, itemKey)))
    saved = editSaved && saved
  }
  draftStorageFailed.value = !saved
}
function discardComposer() {
  draft.value = ''
  draftRestored.value = false
  persistDrafts()
}
function closeEditor() {
  if (editSaving.value) return
  persistDrafts()
  editItem.value = null
}
function discardEdits() {
  editDraft.value = editItem.value?.content ?? ''
  editRestored.value = false
  persistDrafts()
}
const loading = ref(false)
const submitting = ref(false)
const error = ref('')
const apiStatus = ref<'checking' | 'online' | 'offline'>('checking')
const refreshLoading = ref(false)
const copiedId = ref<number | null>(null)
const copiedShareId = ref<number | null>(null)
const uploadError = ref('')
const editItem = shallowRef<ClipboardItem | null>(null)
const editDraft = ref('')
const editSaving = ref(false)
const editError = ref('')
const draftStats = computed(() => textStats(draft.value))
const editStats = computed(() => textStats(editDraft.value))
const draftTooLarge = computed(() => draftStats.value.bytes > MAX_TEXT_BYTES)
const editTooLarge = computed(() => editStats.value.bytes > MAX_TEXT_BYTES)
const textLimitMessage = '文本超过 1 MiB 保存上限，已完整保留在输入框中。请分段保存，或作为 .txt 文件上传。'

function openEditor(item: ClipboardItem) {
  editItem.value = item
  const local = storage('localStorage')
  const saved = local && currentUser.value ? readStoredDraft(local, currentUser.value.id, `item:${item.id}`) : null
  editRestored.value = saved !== null && saved !== (item.content ?? '')
  editDraft.value = saved ?? item.content ?? ''
  editExpanded.value = window.matchMedia('(max-width: 640px)').matches
  editError.value = ''
}

async function saveContent() {
  const item = editItem.value
  if (!item || editSaving.value || !editDraft.value.trim() || editTooLarge.value) return
  const generation = sessionGeneration
  editSaving.value = true
  editError.value = ''
  try {
    const updated = await updateClipboardMetadata(item.id, item.note ?? '', item.tags ?? [], Boolean(item.favorite), editDraft.value)
    if (!isCurrentSession(generation)) return
    replaceItem(updated)
    const local = storage('localStorage')
    if (local && currentUser.value) clearDraft(local, currentUser.value.id, `item:${item.id}`)
    editItem.value = null
    notify('内容已保存')
    if (hasActiveFilters.value) void loadItems()
  } catch (err) {
    if (!isCurrentSession(generation)) return
    editError.value = errorMessage(err, '保存失败，请稍后重试。')
  } finally { if (isCurrentSession(generation)) editSaving.value = false }
}
type UploadEntry = { id: string; name: string; file?: File; status: 'uploading' | 'failed' | 'done' | 'skipped'; percent: number; error?: string }
const uploadEntries = ref<UploadEntry[]>([])
function updateUpload(id: string, update: Partial<UploadEntry>) {
  uploadEntries.value = uploadEntries.value.map(entry => entry.id === id ? { ...entry, ...update } : entry)
}
function retryUpload(entry: UploadEntry) { if (entry.file) void submitFileUpload(entry.file, entry.id) }
const shares = ref<ClipboardShare[]>([])
const sharesLoading = ref(false)
const shareModalOpen = ref(false)
const shareCreating = ref(false)
const shareItem = shallowRef<ClipboardItem | null>(null)
const shareTTL = ref(86400)
const createdShare = shallowRef<ClipboardShare | null>(null)
const shareError = ref('')
const activeView = ref<'clipboard' | 'shares' | 'admin' | 'backup' | 'trash'>('clipboard')
const trashItems = ref<ClipboardItem[]>([])
const trashLoading = ref(false)
const trashError = ref('')
const trashBusyId = ref<number | null>(null)
const pendingTrash = shallowRef<ClipboardItem | null>(null)
const pendingPurge = shallowRef<ClipboardItem | null>(null)
const deleting = ref(false)
let trashRequestSequence = 0
function requestTrash(item: ClipboardItem) { pendingTrash.value = item }
async function loadTrash(silent = false): Promise<boolean> {
  const sequence = ++trashRequestSequence
  const generation = sessionGeneration
  const epoch = collectionEpoch
  const view = syncViewKey()
  if (!silent) { trashLoading.value = true; trashError.value = '' }
  try {
    const result = await listTrash()
    if (sequence !== trashRequestSequence || !isCurrentSession(generation) || (silent && !canApplyBackground(epoch, view))) return false
    trashItems.value = reconcileItems(trashItems.value, result)
    return true
  } catch (err) {
    if (sequence !== trashRequestSequence || !isCurrentSession(generation)) return false
    if (silent) throw err
    trashError.value = errorMessage(err, '回收站加载失败，请重试。')
    return false
  } finally { if (!silent && sequence === trashRequestSequence) trashLoading.value = false }
}
async function restoreItem(id: number) {
  if (trashBusyId.value !== null) return
  trashBusyId.value = id
  const generation = sessionGeneration
  try {
    await restoreClipboard(id)
    if (!isCurrentSession(generation)) return
    trashItems.value = trashItems.value.filter(item => item.id !== id)
    await loadItems()
    if (isCurrentSession(generation)) notify('内容已恢复，原分享链接保持失效')
  } catch (err) { if (isCurrentSession(generation)) notify(errorMessage(err, '恢复失败，请重试。'), 'error') }
  finally { if (isCurrentSession(generation)) trashBusyId.value = null }
}
async function permanentlyDelete() {
  if (!pendingPurge.value || deleting.value) return
  deleting.value = true
  const id = pendingPurge.value.id
  const generation = sessionGeneration
  try {
    await purgeClipboard(id)
    if (!isCurrentSession(generation)) return
    trashItems.value = trashItems.value.filter(item => item.id !== id)
    pendingPurge.value = null
    const local = storage('localStorage')
    if (local && currentUser.value) clearDraft(local, currentUser.value.id, `item:${id}`)
    notify('已彻底删除')
  } catch (err) { if (isCurrentSession(generation)) { await loadTrash(); if (isCurrentSession(generation)) notify(errorMessage(err, '彻底删除失败，请重试。'), 'error') } }
  finally { if (isCurrentSession(generation)) deleting.value = false }
}
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
const backupImporting = ref(false)
const backupProgress = ref(0)
const backupError = ref('')
const backupSuccess = ref('')
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
const canSubmit = computed(() => draft.value.trim().length > 0 && !submitting.value && !draftTooLarge.value)
const isAdmin = computed(() => currentUser.value?.role === 'admin')
const isDetailPage = computed(() => detailItemId.value > 0)
const hasActiveFilters = computed(() => (
  searchQuery.value.trim() !== ''
  || itemKindFilter.value !== 'all'
  || favoritesOnly.value
  || timePreset.value !== 'all'
))
const emptyHistoryDescription = computed(() => hasActiveFilters.value ? '没有找到匹配的记录' : '还没有内容记录')
const historyResultsStyle = computed(() => preservedResultsStyle(
  favoriteResultsMinHeight.value,
  favoritesOnly.value || favoritesFilterTransitioning.value,
))
const groupedHistoryItems = computed(() => groupTimelineItemsByLocalDay(items.value, new Date()))

let searchTimer: ReturnType<typeof window.setTimeout> | undefined
let listRequestSequence = 0
let collectionEpoch = 0
let restoringWorkspace = false
let libraryScrollY = 0
let draftTimer: ReturnType<typeof setTimeout> | undefined
let scrollFrame = 0
const previousScrollRestoration = window.history.scrollRestoration

function rememberLibrary() {
  if (!currentUser.value) return
  const session = storage('sessionStorage')
  if (session) writeLibraryState(session, currentUser.value.id, {
    query: searchQuery.value, kind: itemKindFilter.value, favorites: favoritesOnly.value,
    preset: timePreset.value, range: customTimeRange.value, scrollY: libraryScrollY,
  })
}
function captureLibraryPosition() {
  if (isDetailPage.value || activeView.value !== 'clipboard') return
  libraryScrollY = window.scrollY
  rememberLibrary()
}
async function restoreLibraryPosition() {
  await nextTick()
  requestAnimationFrame(() => {
    if (!isDetailPage.value && activeView.value === 'clipboard') window.scrollTo({ top: libraryScrollY, behavior: 'instant' })
  })
}
function onPageHidden() {
  persistDrafts()
  captureLibraryPosition()
}
function onScroll() {
  if (!scrollFrame) scrollFrame = requestAnimationFrame(() => { scrollFrame = 0; captureLibraryPosition() })
}
function changeView(view: typeof activeView.value) {
  captureLibraryPosition()
  if (isDetailPage.value) {
    detailItemId.value = 0
    detailItem.value = null
    window.history.replaceState({}, '', '/')
  }
  activeView.value = view
  if (view === 'clipboard') void restoreLibraryPosition()
  else window.scrollTo({ top: 0, behavior: 'instant' })
  if (view === 'trash') void loadTrash()
}
const navigationOptions = computed(() => [
  { key: 'shares', label: '分享管理' }, { key: 'backup', label: '备份迁移' }, { key: 'trash', label: '回收站' },
  ...(isAdmin.value ? [{ key: 'admin', label: '管理中心' }] : []),
  { key: 'password', label: '修改密码' }, { key: 'logout', label: '退出登录' },
])
function selectNavigation(key: string) {
  if (key === 'logout') void submitLogout()
  else if (key === 'password') passwordFormOpen.value = !passwordFormOpen.value
  else changeView(key as typeof activeView.value)
}

function errorMessage(errorValue: unknown, fallback: string) {
  if (axios.isAxiosError(errorValue) && typeof errorValue.response?.data?.error === 'string') return errorValue.response.data.error
  return fallback
}

function duplicateItemFromError(errorValue: unknown): ClipboardItem | null {
  if (!axios.isAxiosError(errorValue) || errorValue.response?.status !== 409) return null
  const duplicate = errorValue.response.data?.duplicate
  return duplicate && typeof duplicate.id === 'number' ? duplicate as ClipboardItem : null
}

function duplicateDescription(item: ClipboardItem) {
  if (item.kind === 'text') return '相同文本'
  return item.fileName ? `相同文件“${item.fileName}”` : '相同文件'
}

function confirmDuplicate(item: ClipboardItem) {
  return window.confirm(`${duplicateDescription(item)}已存在于内容库中，是否仍要创建一条新的记录？`)
}

async function loadItems(silent = false): Promise<boolean> {
  const requestSequence = ++listRequestSequence
  const epoch = collectionEpoch
  const view = syncViewKey()
  if (!silent) {
    loading.value = true
    error.value = ''
    if (apiStatus.value !== 'online') apiStatus.value = 'checking'
  }
  try {
    const timeParams = toClipboardTimeParams(resolveTimeRange(timePreset.value, customTimeRange.value, new Date()))
    const result = await listClipboard({
      query: searchQuery.value,
      kind: itemKindFilter.value === 'all' ? undefined : itemKindFilter.value,
      favoriteOnly: favoritesOnly.value,
      createdFrom: timeParams.createdFrom,
      createdBefore: timeParams.createdBefore,
    })
    if (requestSequence !== listRequestSequence || (silent && !canApplyBackground(epoch, view))) return false
    const root = historyResultsElement.value
    const anchors = silent && root ? captureHistoryAnchors(root) : []
    items.value = reconcileItems(items.value, result)
    apiStatus.value = 'online'
    if (silent && anchors.length) {
      await nextTick()
      await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())))
      if (root && requestSequence === listRequestSequence && view === syncViewKey()) restoreHistoryAnchor(root, anchors)
    }
    return true
  } catch (err) {
    if (requestSequence !== listRequestSequence) return false
    if (silent) throw err
    apiStatus.value = 'offline'
    error.value = '无法连接服务，请稍后重试。'
    return false
  } finally {
    if (!silent && requestSequence === listRequestSequence) {
      loading.value = false
      favoritesFilterTransitioning.value = false
    }
  }
}

async function loadDetailItem(silent = false): Promise<boolean> {
  if (!detailItemId.value) return false
  const generation = sessionGeneration
  const epoch = collectionEpoch
  const view = syncViewKey()
  const requestedID = detailItemId.value
  if (!silent) { detailLoading.value = true; detailUnavailable.value = false }
  try {
    const item = await getClipboard(requestedID)
    if (!isCurrentSession(generation) || detailItemId.value !== requestedID || (silent && !canApplyBackground(epoch, view))) return false
    detailItem.value = reconcileItems(detailItem.value ? [detailItem.value] : [], [item])[0]!
    detailUnavailable.value = false
    return true
  } catch (errorValue) {
    if (!isCurrentSession(generation) || detailItemId.value !== requestedID || (silent && !canApplyBackground(epoch, view))) return false
    if (silent && (!axios.isAxiosError(errorValue) || errorValue.response?.status !== 404)) throw errorValue
    detailItem.value = null
    detailUnavailable.value = axios.isAxiosError(errorValue) && errorValue.response?.status === 404
    if (!detailUnavailable.value) error.value = '无法加载内容详情，请稍后重试。'
    return detailUnavailable.value
  } finally {
    if (!silent && isCurrentSession(generation) && detailItemId.value === requestedID) detailLoading.value = false
  }
}

async function loadAdminUsers() {
  if (!isAdmin.value) return
  const generation = sessionGeneration
  adminLoading.value = true
  try { const users = await listUsers(); if (isCurrentSession(generation)) adminUsers.value = users } catch (errorValue) { if (isCurrentSession(generation)) adminError.value = errorMessage(errorValue, '无法加载用户列表。') } finally { if (isCurrentSession(generation)) adminLoading.value = false }
}

async function loadShares(silent = false): Promise<boolean> {
  const generation = sessionGeneration
  const epoch = collectionEpoch
  const view = syncViewKey()
  if (!silent) { sharesLoading.value = true; shareError.value = '' }
  try {
    const result = await listClipboardShares()
    if (!isCurrentSession(generation) || (silent && !canApplyBackground(epoch, view))) return false
    shares.value = result
    return true
  } catch (err) {
    if (!isCurrentSession(generation)) return false
    if (silent) throw err
    shareError.value = errorMessage(err, '无法加载分享链接。')
    return false
  } finally { if (!silent && isCurrentSession(generation)) sharesLoading.value = false }
}

async function exportBackup() {
  backupError.value = ''
  backupSuccess.value = ''
  try {
    await downloadBackup()
    backupSuccess.value = '备份已开始下载。'
  } catch (errorValue) {
    backupError.value = errorMessage(errorValue, '备份导出失败，请稍后重试。')
  }
}

async function uploadBackup(file: File) {
  if (!file.name.toLowerCase().endsWith('.zip')) {
    backupError.value = '请选择 .zip 格式的 Suidu 备份文件。'
    return
  }
  backupImporting.value = true
  backupProgress.value = 0
  backupError.value = ''
  backupSuccess.value = ''
  try {
    const result: ImportBackupResult = await importBackup(file, (percent) => { backupProgress.value = percent })
    backupSuccess.value = `迁移完成：导入 ${result.imported} 条，跳过重复 ${result.skippedDuplicates} 条。`
    await loadItems()
  } catch (errorValue) {
    backupError.value = errorMessage(errorValue, '备份导入失败，请确认文件完整且来自 Suidu。')
  } finally {
    backupImporting.value = false
  }
}

function handleBackupUpload(options: UploadCustomRequestOptions) {
  const file = options.file.file
  if (!file) { options.onError(); return }
  void uploadBackup(file).then(() => options.onFinish()).catch(() => options.onError())
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
  } finally { authLoading.value = false; if (currentUser.value && !isDetailPage.value) void restoreLibraryPosition() }
}

async function submitLogin() {
  if (!loginUsername.value.trim() || !loginPassword.value || loginLoading.value) return
  loginLoading.value = true
  loginError.value = ''
  try { currentUser.value = await login(loginUsername.value, loginPassword.value); loginPassword.value = ''; await Promise.all([loadItems(), loadShares(), loadDetailItem()]); await loadAdminUsers() } catch (errorValue) { loginError.value = errorMessage(errorValue, '登录失败，请稍后重试。') } finally { loginLoading.value = false; if (currentUser.value && !isDetailPage.value) void restoreLibraryPosition() }
}

async function submitLogout() {
  persistDrafts()
  rememberLibrary()
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
  const generation = sessionGeneration
  if (!navigator.clipboard?.readText) { error.value = '当前浏览器不支持读取剪贴板。'; return }
  try { const text = await navigator.clipboard.readText(); if (isCurrentSession(generation)) { draft.value = text; error.value = '' } } catch { if (isCurrentSession(generation)) error.value = '读取剪贴板失败，请允许当前页面访问剪贴板。' }
}

async function submitClipboard() {
  if (!canSubmit.value) return
  const generation = sessionGeneration
  submitting.value = true; error.value = ''
  try {
    const content = draft.value
    let item: ClipboardItem
    try {
      item = await createClipboard(content)
    } catch (errorValue) {
      if (!isCurrentSession(generation)) return
      const duplicate = duplicateItemFromError(errorValue)
      if (!duplicate) throw errorValue
      if (!confirmDuplicate(duplicate)) { apiStatus.value = 'online'; return }
      item = await createClipboard(content, 'web', true)
    }
    if (!isCurrentSession(generation)) return
    if (hasActiveFilters.value) void loadItems()
    else items.value = [item, ...items.value]
    if (draft.value === content) { draft.value = ''; draftRestored.value = false; composeExpanded.value = false; persistDrafts() }
    apiStatus.value = 'online'
    notify('内容已保存')
  } catch (err) {
    if (!isCurrentSession(generation)) return
    if (axios.isAxiosError(err) && !err.response) apiStatus.value = 'offline'
    error.value = errorMessage(err, '提交失败，内容已保留，可再次提交。')
    notify(error.value, 'error')
  } finally { if (isCurrentSession(generation)) submitting.value = false }
}

interface UploadCallbacks {
  onProgress?: (percent: number) => void
  onFinish?: () => void
  onError?: () => void
}

async function submitFileUpload(file: File, uploadId: string, callbacks: UploadCallbacks = {}) {
  const generation = sessionGeneration
  if (!uploadEntries.value.some(entry => entry.id === uploadId)) uploadEntries.value.push({ id: uploadId, name: file.name, file, status: 'uploading', percent: 0 })
  updateUpload(uploadId, { status: 'uploading', percent: 0, error: undefined })
  if (file.size > 100 * 1024 * 1024) {
    uploadError.value = `${file.name} 超过 100 MiB。`
    updateUpload(uploadId, { status: 'failed', error: uploadError.value })
    callbacks.onError?.()
    return
  }
  uploadError.value = ''
  try {
    const onProgress = (percent: number) => {
      if (!isCurrentSession(generation)) return
      updateUpload(uploadId, { percent })
      callbacks.onProgress?.(percent)
    }
    let item: ClipboardItem
    try {
      item = await uploadClipboardFile(file, 'web', onProgress)
    } catch (errorValue) {
      if (!isCurrentSession(generation)) return
      const duplicate = duplicateItemFromError(errorValue)
      if (!duplicate) throw errorValue
      if (!confirmDuplicate(duplicate)) {
        apiStatus.value = 'online'
        updateUpload(uploadId, { status: 'skipped', file: undefined })
        callbacks.onFinish?.()
        return
      }
      item = await uploadClipboardFile(file, 'web', onProgress, true)
    }
    if (!isCurrentSession(generation)) return
    if (hasActiveFilters.value) void loadItems()
    else items.value = [item, ...items.value]
    apiStatus.value = 'online'
    updateUpload(uploadId, { status: 'done', percent: 100, file: undefined })
    notify(`${file.name} 已上传`)
    callbacks.onFinish?.()
  } catch (errorValue) {
    if (!isCurrentSession(generation)) return
    uploadError.value = errorMessage(errorValue, `${file.name} 上传失败。`)
    updateUpload(uploadId, { status: 'failed', error: uploadError.value })
    callbacks.onError?.()
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
  if (!currentUser.value || activeView.value !== 'clipboard' || isDetailPage.value || editItem.value || shareModalOpen.value || tagModalOpen.value || passwordFormOpen.value) return
  const clipboard = event.clipboardData
  if (!clipboard) return
  // Keep the browser's native text paste (including undo) when a clipboard also contains files.
  if (event.target instanceof HTMLTextAreaElement && clipboard.getData('text/plain')) return
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
  const generation = sessionGeneration
  if (!navigator.clipboard?.writeText) { error.value = '当前浏览器不支持写入剪贴板。'; return }
  try { await navigator.clipboard.writeText(item.content || ''); if (!isCurrentSession(generation)) return; copiedId.value = item.id; notify('已复制'); window.setTimeout(() => { if (isCurrentSession(generation) && copiedId.value === item.id) copiedId.value = null }, 1600) } catch { if (isCurrentSession(generation)) notify('复制失败，请检查浏览器剪贴板权限。', 'error') }
}

async function removeItem(item: ClipboardItem) {
  if (deleting.value) return
  const generation = sessionGeneration
  deleting.value = true
  try {
    await deleteClipboard(item.id)
    if (!isCurrentSession(generation)) return
    items.value = items.value.filter((current) => current.id !== item.id)
    shares.value = shares.value.filter((share) => share.item.id !== item.id)
    if (detailItemId.value === item.id) closeDetail()
    pendingTrash.value = null
    notify('已移入回收站，附件已保留', 'success', item.id)
  } catch (errorValue) { if (isCurrentSession(generation)) notify(errorMessage(errorValue, '移入回收站失败，请重试。'), 'error') }
  finally { if (isCurrentSession(generation)) deleting.value = false }
}

function replaceItem(updated: ClipboardItem) {
  if (favoritesOnly.value && !updated.favorite) items.value = items.value.filter((item) => item.id !== updated.id)
  else items.value = items.value.map((item) => item.id === updated.id ? updated : item)
  shares.value = shares.value.map((share) => share.item.id === updated.id ? { ...share, item: updated } : share)
  if (detailItemId.value === updated.id) detailItem.value = updated
}

function openDetail(item: ClipboardItem) {
  captureLibraryPosition()
  detailItemId.value = item.id
  detailItem.value = item
  detailUnavailable.value = false
  window.history.pushState({ suiduDetail: true }, '', `/items/${item.id}`)
  window.scrollTo({ top: 0, behavior: 'instant' })
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
  activeView.value = 'clipboard'
  void restoreLibraryPosition()
}

function handlePopState() {
  const nextID = detailIDFromPath()
  detailItemId.value = nextID
  detailItem.value = nextID ? items.value.find((item) => item.id === nextID) ?? null : null
  detailUnavailable.value = false
  if (nextID && currentUser.value && !detailItem.value) void loadDetailItem()
  if (!nextID) { activeView.value = 'clipboard'; void restoreLibraryPosition() }
  else window.scrollTo({ top: 0, behavior: 'instant' })
}

async function toggleFavorite(item: ClipboardItem) {
  if (favoriteUpdatingId.value === item.id) return
  const generation = sessionGeneration
  favoriteUpdatingId.value = item.id
  error.value = ''
  try {
    const updated = await updateClipboardMetadata(item.id, item.note ?? '', item.tags ?? [], !item.favorite)
    if (!isCurrentSession(generation)) return
    replaceItem(updated)
  } catch (errorValue) {
    if (!isCurrentSession(generation)) return
    error.value = errorMessage(errorValue, '更新收藏状态失败。')
    notify(error.value, 'error')
  } finally {
    if (isCurrentSession(generation) && favoriteUpdatingId.value === item.id) favoriteUpdatingId.value = null
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
  const generation = sessionGeneration
  tagSaving.value = true
  tagError.value = ''
  try {
    const updated = await updateClipboardMetadata(tagItem.value.id, noteDraft.value, tagDraft.value, Boolean(tagItem.value.favorite))
    if (!isCurrentSession(generation)) return
    replaceItem(updated)
    if (searchQuery.value.trim()) void loadItems()
    tagModalOpen.value = false
    notify('备注和标签已保存')
  } catch (errorValue) {
    if (!isCurrentSession(generation)) return
    tagError.value = errorMessage(errorValue, '保存失败，请检查备注和标签长度。')
  } finally { if (isCurrentSession(generation)) tagSaving.value = false }
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
  const generation = sessionGeneration
  shareCreating.value = true
  shareError.value = ''
  try {
    const created = await createClipboardShare(shareItem.value.id, shareTTL.value)
    if (!isCurrentSession(generation)) return
    createdShare.value = created
    await loadShares()
  } catch (errorValue) {
    if (!isCurrentSession(generation)) return
    shareError.value = errorMessage(errorValue, '创建分享链接失败。')
  } finally { if (isCurrentSession(generation)) shareCreating.value = false }
}

async function copyShareLink(share: ClipboardShare) {
  const generation = sessionGeneration
  if (!navigator.clipboard?.writeText) { shareError.value = '当前浏览器不支持复制链接。'; return }
  try {
    await navigator.clipboard.writeText(publicShareUrl(share.token))
    if (!isCurrentSession(generation)) return
    copiedShareId.value = share.id
    notify('分享链接已复制')
    window.setTimeout(() => { if (isCurrentSession(generation) && copiedShareId.value === share.id) copiedShareId.value = null }, 1600)
  } catch { if (isCurrentSession(generation)) shareError.value = '复制链接失败。' }
}

async function revokeShare(share: ClipboardShare) {
  const generation = sessionGeneration
  try { await revokeClipboardShare(share.id); if (isCurrentSession(generation)) { await loadShares(); if (isCurrentSession(generation)) notify('分享链接已失效') } } catch (errorValue) { if (isCurrentSession(generation)) shareError.value = errorMessage(errorValue, '关闭分享链接失败。') }
}

function shareStatus(share: ClipboardShare): 'active' | 'expired' | 'revoked' {
  if (share.revokedAt) return 'revoked'
  return new Date(share.expiresAt).getTime() <= Date.now() ? 'expired' : 'active'
}

async function refreshActiveView() {
  if (refreshLoading.value) return
  const generation = sessionGeneration
  refreshLoading.value = true
  try {
    if (isDetailPage.value) await loadDetailItem()
    else if (activeView.value === 'shares') await loadShares()
    else if (activeView.value === 'admin') await loadAdminUsers()
    else if (activeView.value === 'trash') await loadTrash()
    else if (activeView.value === 'backup') return
    else await loadItems()
  } finally { if (isCurrentSession(generation)) refreshLoading.value = false }
}

function syncViewKey() {
  return detailItemId.value ? `detail:${detailItemId.value}` : activeView.value
}

const syncBusy = computed(() => Boolean(
  authLoading.value || loginLoading.value || loading.value || detailLoading.value || sharesLoading.value || trashLoading.value || refreshLoading.value ||
  editItem.value || editSaving.value || composeExpanded.value || tagModalOpen.value || tagSaving.value || shareModalOpen.value || shareCreating.value || passwordFormOpen.value ||
  pendingTrash.value || pendingPurge.value || deleting.value || trashBusyId.value !== null || favoriteUpdatingId.value !== null || submitting.value || backupImporting.value ||
  uploadEntries.value.some(entry => entry.status === 'uploading')
))

function canRefreshAutomatically() {
  return Boolean(currentUser.value && !publicToken && !syncBusy.value &&
    document.visibilityState === 'visible' && navigator.onLine !== false && !window.getSelection()?.toString() &&
    (isDetailPage.value || ['clipboard', 'shares', 'trash'].includes(activeView.value)))
}

function canApplyBackground(epoch: number, view: string) {
  return canRefreshAutomatically() && epoch === collectionEpoch && view === syncViewKey()
}

const autoSync = createAutoSync({
  isActive: canRefreshAutomatically,
  readVersion: async () => {
    const generation = sessionGeneration
    const version = await getChangesVersion()
    if (isCurrentSession(generation)) apiStatus.value = 'online'
    return version
  },
  refresh: async () => {
    if (!canRefreshAutomatically()) return false
    if (isDetailPage.value) return loadDetailItem(true)
    if (activeView.value === 'shares') return loadShares(true)
    if (activeView.value === 'trash') return loadTrash(true)
    return loadItems(true)
  },
  onError: (err) => {
    if (axios.isAxiosError(err) && err.response?.status === 401) {
      loginError.value = '登录已失效，请重新登录。'
      currentUser.value = null
    } else apiStatus.value = 'offline'
  },
})
function checkForChanges() { void autoSync.check() }

// Reject responses that were started before a local mutation or another fetch completed.
watch([items, shares, trashItems, detailItem], () => { collectionEpoch++ }, { flush: 'sync' })
watch(() => currentUser.value?.id, () => { autoSync.stop() }, { flush: 'sync' })
watch([currentUser, authLoading, loginLoading], () => {
  if (!publicToken && currentUser.value && !authLoading.value && !loginLoading.value) autoSync.start()
  else autoSync.stop()
})
watch(syncViewKey, () => {
  autoSync.invalidate()
  checkForChanges()
})
watch(syncBusy, (busy) => { if (!busy) checkForChanges() })

function formatDateTime(value: string) { return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value)) }
function formatHistoryItemTime(value: string) { return new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date(value)) }
function handleKeydown(event: KeyboardEvent) { if (!event.isComposing && (event.ctrlKey || event.metaKey) && event.key === 'Enter') { event.preventDefault(); void submitClipboard() } }

watch([searchQuery, itemKindFilter, favoritesOnly, timePreset, customTimeRange], () => {
  if (restoringWorkspace || !currentUser.value) return
  rememberLibrary()
  if (searchTimer) window.clearTimeout(searchTimer)
  listRequestSequence++
  loading.value = false
  searchTimer = window.setTimeout(() => {
    if (currentUser.value) void loadItems()
  }, 300)
}, { flush: 'sync' })

watch(currentUser, (user, previous) => {
  if (user?.id === previous?.id) return
  sessionGeneration++
  if (feedbackTimer) clearTimeout(feedbackTimer)
  if (searchTimer) clearTimeout(searchTimer)
  submitting.value = false
  editSaving.value = false
  deleting.value = false
  tagSaving.value = false
  shareCreating.value = false
  refreshLoading.value = false
  trashBusyId.value = null
  favoriteUpdatingId.value = null
  sharesLoading.value = false
  adminLoading.value = false
  detailLoading.value = false
  copiedId.value = null
  copiedShareId.value = null
  shareModalOpen.value = false
  shareItem.value = null
  createdShare.value = null
  shares.value = []
  tagModalOpen.value = false
  tagItem.value = null
  error.value = ''
  shareError.value = ''
  uploadError.value = ''
  if (previous) {
    items.value = []
    adminUsers.value = []
    detailItemId.value = 0
    detailItem.value = null
    activeView.value = 'clipboard'
    window.history.replaceState({}, '', '/')
  }
  listRequestSequence++
  trashRequestSequence++
  trashLoading.value = false
  trashError.value = ''
  const local = storage('localStorage')
  if (previous && local) {
    writeDraft(local, previous.id, draft.value)
    if (editItem.value) writeDraft(local, previous.id, editDraft.value, `item:${editItem.value.id}`)
  }
  editItem.value = null
  composeExpanded.value = false
  feedback.value = null
  pendingTrash.value = null
  pendingPurge.value = null
  trashItems.value = []
  uploadEntries.value = []
  draft.value = user && local ? readDraft(local, user.id) : ''
  draftRestored.value = draft.value !== ''
  draftStorageFailed.value = !local
  restoringWorkspace = true
  const session = storage('sessionStorage')
  const state = user && session ? readLibraryState(session, user.id) : null
  searchQuery.value = state?.query ?? ''
  itemKindFilter.value = state?.kind ?? 'all'
  favoritesOnly.value = state?.favorites ?? false
  timePreset.value = state?.preset ?? 'all'
  customTimeRange.value = state?.range ?? null
  libraryScrollY = state?.scrollY ?? 0
  restoringWorkspace = false
}, { flush: 'sync' })

watch([draft, editDraft], () => {
  if (draftTimer) clearTimeout(draftTimer)
  draftTimer = setTimeout(persistDrafts, 300)
})

onMounted(() => {
  if (!publicToken) {
    window.history.scrollRestoration = 'manual'
    void loadSession()
    document.addEventListener('paste', handlePaste)
    window.addEventListener('popstate', handlePopState)
    window.addEventListener('scroll', onScroll, { passive: true })
    window.addEventListener('pagehide', onPageHidden)
    document.addEventListener('visibilitychange', onPageHidden)
    document.addEventListener('visibilitychange', checkForChanges)
    window.addEventListener('focus', checkForChanges)
    window.addEventListener('online', checkForChanges)
  }
})
onUnmounted(() => {
  autoSync.stop()
  onPageHidden()
  document.removeEventListener('paste', handlePaste)
  window.removeEventListener('popstate', handlePopState)
  window.removeEventListener('scroll', onScroll)
  window.removeEventListener('pagehide', onPageHidden)
  document.removeEventListener('visibilitychange', onPageHidden)
  document.removeEventListener('visibilitychange', checkForChanges)
  window.removeEventListener('focus', checkForChanges)
  window.removeEventListener('online', checkForChanges)
  window.history.scrollRestoration = previousScrollRestoration
  cancelAnimationFrame(scrollFrame)
  if (draftTimer) clearTimeout(draftTimer)
  if (feedbackTimer) clearTimeout(feedbackTimer)
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
      <div class="header-actions">
        <n-button v-if="!isDetailPage && activeView !== 'clipboard'" secondary @click="changeView('clipboard')"><template #icon><ArrowLeft :size="16" /></template>内容库</n-button>
        <span class="connection-status" :class="apiStatus" :title="apiStatus === 'online' ? '已连接' : apiStatus === 'offline' ? '连接中断，可重试' : '连接中'">{{ currentUser.username }}</span>
        <n-button quaternary circle aria-label="刷新内容" title="刷新内容" :disabled="refreshLoading" :aria-busy="refreshLoading" @click="refreshActiveView"><template #icon><RefreshCw :size="17" /></template></n-button>
        <n-dropdown trigger="click" :options="navigationOptions" @select="selectNavigation"><n-button secondary aria-label="应用菜单"><template #icon><MoreHorizontal :size="18" /></template>菜单</n-button></n-dropdown>
      </div>
    </n-layout-header>
    <n-layout-content class="app-content">
        <n-card v-if="passwordFormOpen" class="account-card" :bordered="false">
          <div class="section-heading"><div><p class="eyebrow">ACCOUNT SECURITY</p><h2>修改密码</h2></div></div>
          <n-space vertical :size="10" class="form-stack"><n-input v-model:value="currentPassword" type="password" show-password-on="click" placeholder="当前密码" autocomplete="current-password" /><n-input v-model:value="newPassword" type="password" show-password-on="click" placeholder="新密码（至少 12 位）" autocomplete="new-password" /><n-space><n-button type="primary" :loading="passwordLoading" @click="submitPasswordChange">保存新密码</n-button><n-button quaternary @click="passwordFormOpen = false">取消</n-button></n-space></n-space>
          <n-alert v-if="passwordError" type="error" class="form-alert">{{ passwordError }}
          </n-alert><n-alert v-if="passwordSuccess" type="success" class="form-alert">{{ passwordSuccess }}
          </n-alert>
        </n-card>
        <ClipboardDetailPage v-if="isDetailPage" :item="detailItem" :loading="detailLoading" :unavailable="detailUnavailable" :favorite-updating="favoriteUpdatingId === detailItemId" :copied="copiedId === detailItemId" @back="closeDetail" @copy="copyItem" @edit="openEditor" @favorite="toggleFavorite" @organize="openTagModal" @share="openShareModal" @delete="requestTrash" />
        <main v-else-if="activeView === 'backup'" class="backup-page">
          <section class="page-intro"><div><p class="eyebrow">BACKUP &amp; MIGRATION</p><h1>备份与迁移</h1><p class="intro-copy">导出记录和附件，换服务器时可以直接恢复。</p></div><n-tag round :bordered="false" type="info"><template #icon><Archive :size="14" /></template>ZIP 备份</n-tag></section>
          <n-alert v-if="backupError" type="error" closable class="error-alert" @close="backupError = ''">{{ backupError }}
          </n-alert>
          <n-alert v-if="backupSuccess" type="success" closable class="error-alert" @close="backupSuccess = ''">{{ backupSuccess }}
          </n-alert>
          <section class="backup-grid">
            <n-card class="backup-card" :bordered="false"><div class="panel-heading"><div class="panel-icon"><Download :size="18" /></div><div><h2>导出备份</h2><p>包含文本、图片、文件、备注、标签和收藏状态。</p></div></div><n-button type="primary" block @click="exportBackup"><template #icon><Download :size="16" /></template>下载 ZIP 备份</n-button></n-card>
            <n-card class="backup-card" :bordered="false"><div class="panel-heading"><div class="panel-icon"><Archive :size="18" /></div><div><h2>导入备份</h2><p>导入只会新增记录，已有的重复内容会自动跳过。</p></div></div><n-upload accept=".zip,application/zip" :show-file-list="false" :custom-request="handleBackupUpload" :disabled="backupImporting"><n-upload-dragger><div class="upload-drop-content"><div class="upload-icon"><Archive :size="22" /></div><div><strong>选择或拖入 ZIP 备份</strong><span>不会覆盖当前数据，最大 1 GiB</span></div></div></n-upload-dragger></n-upload><div v-if="backupImporting" class="upload-status"><n-progress type="line" :percentage="backupProgress" :show-indicator="false" processing /><n-text depth="3">正在导入备份…</n-text></div></n-card>
          </section>
        </main>
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
                  <n-text depth="3">创建于 {{ formatDateTime(share.createdAt) }} · 有效期至 {{ formatDateTime(share.expiresAt) }}</n-text>
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
        <TrashPage v-else-if="activeView === 'trash'" :items="trashItems" :loading="trashLoading" :busy-id="trashBusyId" :error="trashError" @restore="(item) => restoreItem(item.id)" @purge="(item) => pendingPurge = item" @refresh="() => loadTrash()" />
        <main v-show="!isDetailPage && activeView === 'clipboard'" class="clipboard-page">
          <section class="page-intro"><div><p class="eyebrow">CONTENT LIBRARY</p><h1>内容库</h1><p class="intro-copy">在设备之间保存、整理和传递内容。</p></div><n-tag round :bordered="false" type="info">{{ items.length }} 条{{ hasActiveFilters ? '匹配' : '记录' }}
          </n-tag></section>
          <n-card class="composer-card" :bordered="false">
            <div class="composer-heading"><h2 class="composer-title">添加内容</h2><n-button quaternary @click="composeExpanded = true"><template #icon><Maximize2 :size="16" /></template>展开编辑</n-button></div>
            <textarea v-model="draft" class="content-editor" aria-label="添加内容" :aria-invalid="draftTooLarge" aria-describedby="draft-editor-status" placeholder="输入或粘贴要传递的文字..." rows="5" @keydown.stop="handleKeydown" />
            <div id="draft-editor-status" class="editor-status"><span class="editor-hint">{{ draftStorageFailed ? '草稿暂时无法保存到此设备，请勿关闭页面' : draftRestored ? '已恢复上次草稿 · 自动保存在此设备' : '草稿自动保存在此设备' }}</span><span class="editor-hint">{{ draftStats.characters.toLocaleString() }} 字符 · {{ formatTextBytes(draftStats.bytes) }} / 1 MiB</span></div>
            <n-alert v-if="draftTooLarge" type="warning" class="form-alert">{{ textLimitMessage }}</n-alert>
            <div class="composer-actions"><n-popconfirm v-if="draft" @positive-click="discardComposer"><template #trigger><n-button quaternary>清空草稿</n-button></template>确认清空此设备上的这份草稿？</n-popconfirm><n-button secondary @click="readClipboard"><template #icon><ClipboardPaste :size="17" /></template>读取剪贴板</n-button><n-button type="primary" :disabled="!canSubmit" :loading="submitting" @click="submitClipboard"><template #icon><Send :size="17" /></template>提交文本</n-button></div>
          </n-card>
          <n-card class="upload-card" :bordered="false">
            <n-upload multiple :show-file-list="false" :custom-request="uploadFile">
              <n-upload-dragger><div class="upload-drop-content"><div class="upload-icon"><UploadCloud :size="22" /></div><div><strong>上传图片或文件</strong><span>点击、拖入，或直接 Ctrl+V / ⌘V 粘贴截图，单个不超过 100 MiB</span></div></div></n-upload-dragger>
            </n-upload>
            <div v-if="uploadEntries.length" class="upload-queue" aria-label="上传记录">
              <div v-for="entry in uploadEntries" :key="entry.id" class="upload-entry">
                <div class="upload-entry-heading"><strong>{{ entry.name }}</strong><span>{{ entry.status === 'done' ? '已上传' : entry.status === 'failed' ? '上传失败' : entry.status === 'skipped' ? '已取消重复上传' : entry.percent === 100 ? '正在保存…' : `${entry.percent}%` }}</span></div>
                <n-progress v-if="entry.status === 'uploading'" type="line" :percentage="entry.percent" :show-indicator="false" />
                <p v-if="entry.error" role="alert">{{ entry.error }}</p>
                <div v-if="entry.status !== 'uploading'" class="upload-entry-actions"><n-button v-if="entry.status === 'failed' && entry.file && entry.file.size <= 100 * 1024 * 1024" size="small" @click="retryUpload(entry)">重试上传</n-button><n-button quaternary size="small" @click="uploadEntries = uploadEntries.filter(current => current.id !== entry.id)">关闭</n-button></div>
              </div>
            </div>
          </n-card>
          <n-alert v-if="uploadError" type="error" closable class="error-alert" @close="uploadError = ''">{{ uploadError }}
          </n-alert>
          <n-alert v-if="error" type="error" closable class="error-alert" @close="error = ''">{{ error }}
          </n-alert>
          <section class="history-section" :class="{ 'is-loading': loading }" :aria-busy="loading">
            <div class="history-progress-rail" aria-hidden="true"><span /></div>
            <div class="section-heading">
              <div><p class="eyebrow">CONTENT TIMELINE</p><h2>内容时间线</h2></div>
              <div class="history-heading-status"><n-text depth="3">{{ hasActiveFilters ? `找到 ${items.length} 条` : '按时间倒序' }}</n-text></div>
            </div>
            <div class="history-toolbar">
              <n-input v-model:value="searchQuery" class="history-search" clearable maxlength="200" :input-props="{ 'aria-label': '搜索内容记录' }" placeholder="搜索文本、文件名、备注或标签" aria-label="搜索内容记录">
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
              <div v-else class="history-timeline">
                <section v-for="group in groupedHistoryItems" :key="group.dateKey" class="history-group">
                  <div class="history-group-rail" aria-hidden="true">
                    <span class="history-group-dot" />
                    <span class="history-group-line" />
                  </div>
                  <div class="history-group-main">
                    <header class="history-group-header">
                      <div>
                        <h3>{{ group.label }}</h3>
                        <p>{{ group.count }} 条记录</p>
                      </div>
                    </header>
                    <n-list class="history-group-list" bordered>
                      <n-list-item v-for="item in group.items" :key="item.id">
                        <div class="history-item" :data-item-id="item.id">
                          <div class="item-card-header">
                            <n-space :size="8" align="center"><n-tag size="small" :bordered="false">{{ item.kind === 'image' ? '图片' : item.kind === 'file' ? '文件' : '文本' }}</n-tag><time :datetime="item.createdAt" :title="formatDateTime(item.createdAt)">{{ formatHistoryItemTime(item.createdAt) }}</time></n-space>
                            <ItemActions :item="item" :copied="copiedId === item.id" :favorite-updating="favoriteUpdatingId === item.id" @copy="copyItem" @favorite="toggleFavorite" @edit="openEditor" @organize="openTagModal" @share="openShareModal" @trash="requestTrash" />
                          </div>
                          <ClipboardItemContent :item="item" preview @view-detail="openDetail(item)" />
                          <p v-if="item.note" class="item-note">{{ item.note }}</p>
                          <div v-if="item.tags?.length" class="item-tags" aria-label="记录标签">
                            <n-tag v-for="tag in item.tags" :key="tag" size="small" round :bordered="false" type="info" class="item-tag" role="button" tabindex="0" @click="searchTag(tag)" @keydown.enter="searchTag(tag)">{{ tag }}</n-tag>
                          </div>

                        </div>
                      </n-list-item>
                    </n-list>
                  </div>
                </section>
              </div>
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
    <n-modal v-model:show="composeExpanded" preset="card" title="添加内容" class="workspace-editor-modal compose-expanded-modal" :mask-closable="false">
      <textarea v-model="draft" class="content-editor" aria-label="展开添加内容" :aria-invalid="draftTooLarge" rows="18" @keydown.stop="handleKeydown" />
      <div class="editor-status"><span class="editor-hint">{{ draftStorageFailed ? '草稿暂时无法保存，请勿关闭页面' : '草稿自动保存在此设备' }}</span><span class="editor-hint">{{ draftStats.characters.toLocaleString() }} 字符 · {{ formatTextBytes(draftStats.bytes) }} / 1 MiB</span></div>
      <n-alert v-if="draftTooLarge" type="warning" class="form-alert">{{ textLimitMessage }}</n-alert>
      <n-alert v-if="error" type="error" class="form-alert">{{ error }}</n-alert>
      <n-space justify="end" class="tag-modal-actions"><n-button @click="composeExpanded = false">收起</n-button><n-button type="primary" :disabled="!canSubmit" :loading="submitting" @click="submitClipboard">提交文本</n-button></n-space>
    </n-modal>
    <n-modal :show="Boolean(editItem)" preset="card" title="编辑内容" class="edit-content-modal workspace-editor-modal" :class="{ 'is-fullscreen': editExpanded }" :mask-closable="false" :close-on-esc="!editSaving" :closable="!editSaving" @update:show="(show: boolean) => { if (!show) closeEditor() }">
      <div class="editor-topline"><span>{{ editRestored ? '已恢复未提交的修改' : 'Enter 换行 · Ctrl/⌘ + Enter 保存' }}</span><n-button quaternary @click="editExpanded = !editExpanded">{{ editExpanded ? '退出全屏' : '全屏编辑' }}</n-button></div>
      <textarea v-model="editDraft" class="content-editor" aria-label="编辑内容" :aria-invalid="editTooLarge" aria-describedby="edit-editor-status" rows="14" :disabled="editSaving" @keydown.stop="(event: KeyboardEvent) => { if (!event.isComposing && (event.ctrlKey || event.metaKey) && event.key === 'Enter') { event.preventDefault(); void saveContent() } }" />
      <div id="edit-editor-status" class="editor-status"><span class="editor-hint">Enter 换行 · Ctrl/⌘ + Enter 保存</span><span class="editor-hint">{{ editStats.characters.toLocaleString() }} 字符 · {{ formatTextBytes(editStats.bytes) }} / 1 MiB</span></div>
      <p v-if="draftStorageFailed" class="draft-warning" role="status">草稿暂时无法保存到此设备，请勿关闭页面。</p>
      <n-alert v-if="editTooLarge" type="warning" class="form-alert">{{ textLimitMessage }}</n-alert>
      <n-alert v-if="editError" type="error" class="form-alert">{{ editError }}</n-alert>
      <n-space justify="end" class="tag-modal-actions"><n-popconfirm @positive-click="discardEdits"><template #trigger><n-button quaternary :disabled="editSaving">放弃修改</n-button></template>丢弃未保存的修改，恢复已提交的内容？</n-popconfirm><n-button :disabled="editSaving" @click="closeEditor">暂存并关闭</n-button><n-button type="primary" :loading="editSaving" :disabled="!editDraft.trim() || editTooLarge" @click="saveContent">保存</n-button></n-space>
    </n-modal>
    <n-modal :show="Boolean(pendingTrash)" preset="card" title="移入回收站" class="confirmation-modal" :mask-closable="!deleting" :closable="!deleting" :close-on-esc="!deleting" @update:show="(show: boolean) => { if (!show && !deleting) pendingTrash = null }">
      <p>内容和附件会保留，可从回收站恢复。已公开的分享链接将立即失效，恢复内容也不会重新开启。</p>
      <n-space justify="end"><n-button :disabled="deleting" @click="pendingTrash = null">取消</n-button><n-button type="primary" :loading="deleting" @click="pendingTrash && removeItem(pendingTrash)">确认移入</n-button></n-space>
    </n-modal>
    <n-modal :show="Boolean(pendingPurge)" preset="card" title="彻底删除" class="confirmation-modal" :mask-closable="!deleting" :closable="!deleting" :close-on-esc="!deleting" @update:show="(show: boolean) => { if (!show && !deleting) pendingPurge = null }">
      <p>确认永久删除{{ pendingPurge?.fileName ? `“${pendingPurge.fileName}”及源文件` : '这条内容' }}？此操作无法恢复。</p>
      <n-space justify="end"><n-button :disabled="deleting" @click="pendingPurge = null">取消</n-button><n-button type="error" :loading="deleting" @click="permanentlyDelete">确认彻底删除</n-button></n-space>
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
  <div v-if="feedback && currentUser" class="operation-feedback" :class="feedback.kind" :role="feedback.kind === 'error' ? 'alert' : 'status'">
    <span>{{ feedback.text }}</span><button v-if="feedback.undoId" type="button" :disabled="trashBusyId !== null" @click="restoreItem(feedback.undoId)">撤销</button><button type="button" aria-label="关闭提示" @click="feedback = null">×</button>
  </div>
</template>
