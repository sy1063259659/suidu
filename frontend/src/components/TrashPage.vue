<script setup lang="ts">
import { File, RotateCcw, Trash2 } from '@lucide/vue'
import { NButton, NEmpty } from 'naive-ui'
import type { ClipboardItem } from '../api/clipboard'
defineProps<{ items: ClipboardItem[]; loading: boolean; busyId: number | null; error: string }>()
const emit = defineEmits<{ restore: [item: ClipboardItem]; purge: [item: ClipboardItem]; refresh: [] }>()
function date(value?: string) { return value ? new Date(value).toLocaleString('zh-CN') : '' }
</script>
<template>
  <main class="trash-page" :aria-busy="loading">
    <section class="page-intro"><div><p class="eyebrow">RECYCLE BIN</p><h1>回收站</h1><p class="intro-copy">内容和附件保留至你确认彻底删除，恢复后可继续使用。</p></div><n-button :disabled="loading" @click="emit('refresh')">刷新</n-button></section>
    <div v-if="loading" class="trash-loading" role="status">正在更新回收站…</div>
    <p v-if="error" role="alert" class="trash-error">{{ error }} <n-button text @click="emit('refresh')">重试</n-button></p>
    <n-empty v-if="!items.length && !loading && !error" description="回收站是空的" class="empty-state" />
    <article v-for="item in items" :key="item.id" class="trash-card">
      <div class="trash-item-body"><strong v-if="item.kind !== 'text'"><File :size="16" /> {{ item.fileName }}</strong><p v-else class="trash-preview">{{ item.content?.slice(0, 300) }}</p><p v-if="item.note" class="trash-note">{{ item.note }}</p><small>移入于 {{ date(item.deletedAt) }}</small><p v-if="item.purgePending" class="trash-error">彻底删除尚未完成，请重试删除；此内容已无法恢复。</p></div>
      <div class="trash-actions"><n-button secondary :disabled="busyId === item.id || item.purgePending" :aria-busy="busyId === item.id" @click="emit('restore', item)"><template #icon><RotateCcw :size="16" /></template>恢复</n-button><n-button quaternary type="error" :disabled="busyId === item.id" @click="emit('purge', item)"><template #icon><Trash2 :size="16" /></template>{{ item.purgePending ? '重试删除' : '彻底删除' }}</n-button></div>
    </article>
  </main>
</template>
<style scoped>
.trash-page { max-width: 920px; margin: 0 auto; }
.trash-card { display: flex; align-items: center; justify-content: space-between; gap: 18px; padding: 20px; margin-top: 12px; border: 1px solid #e4eaf2; border-radius: 8px; background: white; }
.trash-item-body { min-width: 0; flex: 1; }.trash-item-body strong { overflow-wrap: anywhere; }.trash-item-body small,.trash-note { color: #718096; }
.trash-preview { margin: 0 0 10px; white-space: pre-wrap; overflow-wrap: anywhere; display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden; }
.trash-note { margin: 6px 0; }.trash-actions { display: flex; flex-wrap: wrap; gap: 6px; }.trash-loading { color: #2d6cdf; min-height: 22px; }.trash-error { color: #a13046; }
@media(max-width:640px) { .trash-card { align-items: stretch; flex-direction: column; padding: 16px; }.trash-actions :deep(.n-button) { min-height: 44px; flex: 1; } }
</style>
