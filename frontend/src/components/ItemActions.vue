<script setup lang="ts">
import { Copy, Download, MoreHorizontal, Star } from '@lucide/vue'
import { NButton, NDropdown } from 'naive-ui'
import { clipboardContentUrl, type ClipboardItem } from '../api/clipboard'

const props = defineProps<{ item: ClipboardItem; copied?: boolean; favoriteUpdating?: boolean }>()
const emit = defineEmits<{
  copy: [item: ClipboardItem]; favorite: [item: ClipboardItem]; edit: [item: ClipboardItem]
  organize: [item: ClipboardItem]; share: [item: ClipboardItem]; trash: [item: ClipboardItem]
}>()
function options() {
  return [
    ...(props.item.kind === 'text' || !props.item.kind ? [{ key: 'edit', label: '编辑内容' }] : []),
    { key: 'organize', label: '备注和标签' }, { key: 'share', label: '公开分享' },
    { type: 'divider', key: 'divider' }, { key: 'trash', label: '移入回收站' },
  ]
}
function select(key: string) {
  if (key === 'edit') emit('edit', props.item)
  if (key === 'organize') emit('organize', props.item)
  if (key === 'share') emit('share', props.item)
  if (key === 'trash') emit('trash', props.item)
}
</script>

<template>
  <div class="item-actions" @click.stop>
    <n-button quaternary circle :type="item.favorite ? 'warning' : 'default'" :disabled="favoriteUpdating" :aria-busy="favoriteUpdating" :aria-label="item.favorite ? '取消收藏' : '收藏记录'" :title="item.favorite ? '取消收藏' : '收藏记录'" @click="emit('favorite', item)"><template #icon><Star :size="17" :fill="item.favorite ? 'currentColor' : 'none'" /></template></n-button>
    <n-button v-if="item.kind === 'text' || !item.kind" quaternary :aria-label="copied ? '已复制' : '复制记录'" @click="emit('copy', item)"><template #icon><Copy :size="17" /></template>{{ copied ? '已复制' : '复制' }}</n-button>
    <n-button v-else tag="a" :href="clipboardContentUrl(item.id, true)" quaternary aria-label="下载文件"><template #icon><Download :size="17" /></template>下载</n-button>
    <n-dropdown trigger="click" :options="options()" @select="select"><n-button quaternary circle aria-label="更多操作" title="更多操作"><template #icon><MoreHorizontal :size="18" /></template></n-button></n-dropdown>
  </div>
</template>

<style scoped>
.item-actions { display: flex; align-items: center; flex-shrink: 0; gap: 2px; }
@media (max-width: 640px) { .item-actions :deep(.n-button) { min-width: 42px; min-height: 44px; } }
</style>
