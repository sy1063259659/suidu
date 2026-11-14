<script setup lang="ts">
import { File, Image } from '@lucide/vue'
import { NImage } from 'naive-ui'
import { clipboardContentUrl, type ClipboardItem } from '../api/clipboard'
import RichTextContent from './RichTextContent.vue'

const props = defineProps<{
  item: ClipboardItem
  contentUrl?: (download?: boolean) => string
  preview?: boolean
}>()
const emit = defineEmits<{ viewDetail: [] }>()

function itemContentUrl(download = false) {
  return props.contentUrl?.(download) ?? clipboardContentUrl(props.item.id, download)
}

function formatFileSize(value = 0) {
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`
  if (value < 1024 * 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MiB`
  return `${(value / (1024 * 1024 * 1024)).toFixed(1)} GiB`
}
function openFileDetail(event: MouseEvent | KeyboardEvent) {
  if (!props.preview) return
  if (event instanceof KeyboardEvent && !['Enter', ' '].includes(event.key)) return
  if (window.getSelection()?.toString()) return
  if (event instanceof KeyboardEvent) event.preventDefault()
  emit('viewDetail')
}
</script>

<template>
  <RichTextContent v-if="item.kind === 'text' || !item.kind" :content="item.content || ''" :preview="preview" @view-detail="emit('viewDetail')" />
  <div v-else-if="item.kind === 'image'" class="attachment-content image-attachment">
    <button v-if="preview" type="button" class="image-preview-link image-detail-trigger" aria-label="查看图片详情" @click="emit('viewDetail')">
      <img :src="itemContentUrl()" :alt="item.fileName || '内容图片'" loading="lazy" />
    </button>
    <n-image v-else :src="itemContentUrl()" :alt="item.fileName || '内容图片'" object-fit="contain" lazy class="image-preview-link" />
    <div class="attachment-details"><Image :size="18" /><div><strong>{{ item.fileName }}</strong><span>{{ formatFileSize(item.sizeBytes) }}</span></div></div>
  </div>
  <div v-else class="attachment-content file-attachment" :class="{ 'file-detail-trigger': preview }" :role="preview ? 'button' : undefined" :tabindex="preview ? 0 : undefined" :aria-label="preview ? '查看文件详情' : undefined" @click="openFileDetail" @keydown="openFileDetail">
    <div class="file-icon"><File :size="24" /></div>
    <div class="attachment-details"><div><strong>{{ item.fileName }}</strong><span>{{ formatFileSize(item.sizeBytes) }}<template v-if="item.mediaType"> · {{ item.mediaType }}</template></span></div></div>
  </div>
</template>

<style scoped>
.file-detail-trigger { cursor: pointer; }
.file-detail-trigger:focus-visible { outline: 3px solid #93b6ef; outline-offset: 3px; }
</style>
