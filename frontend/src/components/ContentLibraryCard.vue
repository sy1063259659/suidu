<script setup lang="ts">
import { computed } from 'vue'
import { File } from '@lucide/vue'
import { clipboardContentUrl, type ClipboardItem } from '../api/clipboard'
import { getCardPresentation } from '../utils/cardPresentation'
import type { DisplayMode } from '../utils/displayPreferences'
import ClipboardItemContent from './ClipboardItemContent.vue'
import ItemActions from './ItemActions.vue'

const props = defineProps<{
  item: ClipboardItem
  mode: DisplayMode
  copied?: boolean
  favoriteUpdating?: boolean
  highlighted?: boolean
}>()
const emit = defineEmits<{
  viewDetail: [item: ClipboardItem]
  copy: [item: ClipboardItem]
  favorite: [item: ClipboardItem]
  edit: [item: ClipboardItem]
  organize: [item: ClipboardItem]
  share: [item: ClipboardItem]
  trash: [item: ClipboardItem]
  searchTag: [tag: string]
}>()
const presentation = computed(() => getCardPresentation(props.item))
const createdTime = computed(() => {
  const date = new Date(props.item.createdAt)
  return Number.isFinite(date.getTime())
    ? new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false }).format(date)
    : '时间未知'
})
const fullCreatedTime = computed(() => {
  const date = new Date(props.item.createdAt)
  return Number.isFinite(date.getTime()) ? date.toLocaleString('zh-CN', { hour12: false }) : props.item.createdAt
})

function viewDetail() {
  if (typeof window !== 'undefined' && window.getSelection()?.toString()) return
  emit('viewDetail', props.item)
}

function openCard(event: MouseEvent) {
  const target = event.target as Element | null
  if (target?.closest?.('a, button, input, textarea, select, [role="button"], [contenteditable]')) return
  viewDetail()
}

function openWithKeyboard(event: KeyboardEvent) {
  if (event.target !== event.currentTarget || !['Enter', ' '].includes(event.key)) return
  event.preventDefault()
  event.stopPropagation()
  viewDetail()
}
</script>

<template>
  <article class="history-item content-library-card" :class="{ 'is-compact': mode === 'compact', 'is-highlighted': highlighted }" :data-item-id="item.id" @click="openCard">
    <header class="library-card-heading">
      <h3 class="library-card-title" role="button" tabindex="0" :aria-label="`查看详情：${presentation.title}`" :title="presentation.title" @click.stop="viewDetail" @keydown="openWithKeyboard">{{ presentation.title }}</h3>
      <ItemActions :item="item" :copied="copied" :favorite-updating="favoriteUpdating" @copy="emit('copy', $event)" @favorite="emit('favorite', $event)" @edit="emit('edit', $event)" @organize="emit('organize', $event)" @share="emit('share', $event)" @trash="emit('trash', $event)" />
    </header>
    <div class="library-card-metadata">
      <span class="library-card-kind">{{ presentation.typeLabel }}</span>
      <time :datetime="item.createdAt" :title="fullCreatedTime">{{ createdTime }}</time>
      <span v-if="item.kind === 'file' || item.kind === 'image'">{{ presentation.fileSize }}</span>
    </div>
    <p v-if="presentation.note" class="library-card-note">{{ presentation.note }}</p>
    <ClipboardItemContent v-if="mode === 'preview'" :item="item" preview @view-detail="viewDetail" />
    <div v-else class="library-card-summary" role="button" tabindex="0" :aria-label="`查看${presentation.typeLabel}详情`" @click.stop="viewDetail" @keydown="openWithKeyboard">
      <p v-if="item.kind === 'text' || !item.kind" class="library-card-text-summary">{{ presentation.summary }}</p>
      <template v-else>
        <img v-if="item.kind === 'image'" class="library-card-thumbnail" :src="clipboardContentUrl(item.id)" :alt="item.fileName || '内容图片'" loading="lazy" />
        <span v-else class="library-card-file-icon" aria-hidden="true"><File :size="22" /></span>
        <div class="library-card-file-summary"><strong>{{ item.fileName || `未命名${presentation.typeLabel}` }}</strong><span v-if="item.mediaType">{{ item.mediaType }}</span></div>
      </template>
    </div>
    <div v-if="item.tags?.length" class="item-tags library-card-tags" aria-label="记录标签" @click.stop>
      <button v-for="tag in item.tags" :key="tag" type="button" class="library-card-tag" :aria-label="`搜索标签：${tag}`" @click="emit('searchTag', tag)">{{ tag }}</button>
    </div>
  </article>
</template>

<style scoped>
.content-library-card { position: relative; min-width: 0; color: #25344b; cursor: pointer; border-radius: 8px; }
.library-card-heading { display: flex; align-items: flex-start; gap: 14px; min-width: 0; }
.library-card-title { display: -webkit-box; flex: 1; min-width: 0; margin: 0; overflow: hidden; color: #172033; font-size: 16px; font-weight: 650; line-height: 1.55; overflow-wrap: anywhere; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.library-card-title, .library-card-summary { cursor: pointer; border-radius: 4px; }
.library-card-title:focus-visible, .library-card-summary:focus-visible, .library-card-tag:focus-visible { outline: 3px solid #93b6ef; outline-offset: 3px; }
.library-card-metadata { display: flex; flex-wrap: wrap; align-items: center; gap: 9px; margin: 7px 0 14px; color: #78869a; font-size: 12px; }
.library-card-kind { padding: 2px 7px; border-radius: 4px; color: #526076; background: #f1f4f9; }
.library-card-note, .library-card-text-summary { display: -webkit-box; min-width: 0; overflow: hidden; white-space: pre-wrap; overflow-wrap: anywhere; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.library-card-note { margin: 0 0 14px; color: #718096; font-size: 13px; line-height: 1.65; }
.library-card-summary { display: flex; align-items: center; gap: 12px; min-width: 0; }
.library-card-text-summary { margin: 0; line-height: 1.65; }
.library-card-thumbnail, .library-card-file-icon { flex: 0 0 44px; width: 44px; height: 44px; border: 1px solid #e4eaf2; border-radius: 6px; background: #f4f7fb; }
.library-card-thumbnail { object-fit: cover; }
.library-card-file-icon { display: grid; place-items: center; color: #718096; }
.library-card-file-summary { display: grid; min-width: 0; gap: 4px; overflow-wrap: anywhere; }
.library-card-file-summary strong { display: -webkit-box; overflow: hidden; font-size: 14px; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.library-card-file-summary span { color: #78869a; font-size: 12px; }
.library-card-tag { max-width: 100%; padding: 3px 9px; border: 0; border-radius: 99px; color: #4268aa; background: #edf3ff; font: inherit; font-size: 12px; line-height: 1.5; overflow-wrap: anywhere; cursor: pointer; }
.library-card-tag:hover { background: #e1ebff; }
.is-compact .library-card-metadata { margin-bottom: 10px; }
.is-compact .library-card-note { margin-bottom: 9px; }
.is-highlighted { outline: 2px solid #8db0ee; outline-offset: 6px; background: #f2f7ff; animation: library-card-highlight 3s ease-out; }
@keyframes library-card-highlight { from { background: #dfeaff; box-shadow: 0 0 0 7px #dfeaff; } to { background: #f2f7ff; box-shadow: 0 0 0 0 transparent; } }
@media (prefers-reduced-motion: reduce) { .is-highlighted { animation: none; } }
@media (max-width: 640px) {
  .library-card-heading { flex-wrap: wrap; gap: 6px; }
  .library-card-title { flex-basis: 100%; }
  .library-card-heading :deep(.item-actions) { margin-left: auto; }
  .library-card-tag { min-height: 36px; }
}
</style>
