<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ClipboardPaste, Copy, RefreshCw, Send, Trash2 } from '@lucide/vue'
import {
  NAlert, NButton, NCard, NEmpty, NInput, NLayout, NLayoutContent, NLayoutHeader,
  NList, NListItem, NPopconfirm, NSpace, NSpin, NTag, NText,
} from 'naive-ui'
import { createClipboard, deleteClipboard, listClipboard, type ClipboardItem } from './api/clipboard'

const items = ref<ClipboardItem[]>([])
const draft = ref('')
const loading = ref(false)
const submitting = ref(false)
const error = ref('')
const apiStatus = ref<'checking' | 'online' | 'offline'>('checking')
const copiedId = ref<number | null>(null)
const canSubmit = computed(() => draft.value.trim().length > 0 && !submitting.value)

async function loadItems() {
  loading.value = true
  error.value = ''
  apiStatus.value = 'checking'
  try {
    items.value = await listClipboard()
    apiStatus.value = 'online'
  } catch {
    apiStatus.value = 'offline'
    error.value = '无法连接服务，请确认后端已启动。'
  } finally {
    loading.value = false
  }
}

async function readClipboard() {
  if (!navigator.clipboard?.readText) {
    error.value = '当前浏览器不支持读取剪贴板。'
    return
  }
  try {
    draft.value = await navigator.clipboard.readText()
    error.value = ''
  } catch {
    error.value = '读取剪贴板失败，请允许当前页面访问剪贴板。'
  }
}

async function submitClipboard() {
  if (!canSubmit.value) return
  submitting.value = true
  error.value = ''
  try {
    const item = await createClipboard(draft.value.trim())
    items.value = [item, ...items.value]
    draft.value = ''
    apiStatus.value = 'online'
  } catch {
    apiStatus.value = 'offline'
    error.value = '提交失败，请稍后重试。'
  } finally {
    submitting.value = false
  }
}

async function copyItem(item: ClipboardItem) {
  if (!navigator.clipboard?.writeText) {
    error.value = '当前浏览器不支持写入剪贴板。'
    return
  }
  try {
    await navigator.clipboard.writeText(item.content)
    copiedId.value = item.id
    window.setTimeout(() => { if (copiedId.value === item.id) copiedId.value = null }, 1600)
  } catch {
    error.value = '复制失败，请检查浏览器剪贴板权限。'
  }
}

async function removeItem(item: ClipboardItem) {
  try {
    await deleteClipboard(item.id)
    items.value = items.value.filter((current) => current.id !== item.id)
  } catch {
    error.value = '删除失败，请稍后重试。'
  }
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value))
}

function handleKeydown(event: KeyboardEvent) {
  if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
    event.preventDefault()
    void submitClipboard()
  }
}

onMounted(loadItems)
</script>

<template>
  <n-layout class="app-shell">
    <n-layout-header bordered class="app-header">
      <div><div class="brand">随渡 <span>SUIDU</span></div><div class="subtitle">把文字放在随手可取的地方</div></div>
      <n-space align="center" :size="12">
        <n-tag :type="apiStatus === 'online' ? 'success' : apiStatus === 'offline' ? 'error' : 'warning'">API {{ apiStatus === 'online' ? '在线' : apiStatus === 'offline' ? '离线' : '检查中' }}
        </n-tag>
        <n-button quaternary circle aria-label="刷新内容" title="刷新内容" :loading="loading" @click="loadItems"><template #icon><RefreshCw :size="17" /></template>
        </n-button>
      </n-space>
    </n-layout-header>

    <n-layout-content class="app-content">
      <main class="clipboard-page">
        <section class="page-intro"><div><p class="eyebrow">TEXT CLIPBOARD</p><h1>剪贴板</h1><p class="intro-copy">在手机和电脑之间传递一段文字，提交后会保存在最近记录中。</p></div><n-tag round :bordered="false" type="info">{{ items.length }} 条记录</n-tag></section>

        <n-card class="composer-card" :bordered="false">
          <n-input v-model:value="draft" type="textarea" placeholder="输入或粘贴要传递的文字..." :autosize="{ minRows: 5, maxRows: 12 }" maxlength="1048576" show-count @keydown="handleKeydown" />
          <div class="composer-actions"><n-button secondary @click="readClipboard"><template #icon><ClipboardPaste :size="17" /></template>读取剪贴板</n-button><n-button type="primary" :disabled="!canSubmit" :loading="submitting" @click="submitClipboard"><template #icon><Send :size="17" /></template>提交文本</n-button></div>
        </n-card>

        <n-alert v-if="error" type="error" closable class="error-alert" @close="error = ''">{{ error }}
        </n-alert>

        <section class="history-section"><div class="section-heading"><div><p class="eyebrow">RECENT</p><h2>最近记录</h2></div><n-text depth="3">按时间倒序</n-text></div>
          <div v-if="loading && !items.length" class="loading-state"><n-spin size="medium" /></div>
          <n-empty v-else-if="!items.length" description="还没有剪贴板记录" class="empty-state" />
          <n-list v-else class="history-list" bordered><n-list-item v-for="item in items" :key="item.id"><div class="history-item"><div class="history-content">{{ item.content }}</div><div class="history-meta"><n-space :size="8" align="center"><n-tag size="small" :bordered="false">{{ item.source || 'web' }}</n-tag><n-text depth="3">{{ formatDate(item.createdAt) }}</n-text></n-space><n-space :size="4"><n-button quaternary circle :aria-label="copiedId === item.id ? '已复制' : '复制记录'" :title="copiedId === item.id ? '已复制' : '复制记录'" @click="copyItem(item)"><template #icon><Copy :size="16" /></template></n-button><n-popconfirm @positive-click="removeItem(item)"><template #trigger><n-button quaternary circle aria-label="删除记录" title="删除记录"><template #icon><Trash2 :size="16" /></template></n-button></template>确定删除这条记录吗？</n-popconfirm></n-space></div></div></n-list-item></n-list>
        </section>
      </main>
    </n-layout-content>
  </n-layout>
</template>
