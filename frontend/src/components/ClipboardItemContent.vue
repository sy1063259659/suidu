<script setup lang="ts">
import { File, Image } from '@lucide/vue'
import { clipboardContentUrl, type ClipboardItem } from '../api/clipboard'

const props = defineProps<{
  item: ClipboardItem
  contentUrl?: (download?: boolean) => string
}>()

function itemContentUrl(download = false) {
  return props.contentUrl?.(download) ?? clipboardContentUrl(props.item.id, download)
}

function formatFileSize(value = 0) {
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`
  if (value < 1024 * 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MiB`
  return `${(value / (1024 * 1024 * 1024)).toFixed(1)} GiB`
}
</script>

<template>
  <div v-if="item.kind === 'text' || !item.kind" class="history-content">{{ item.content }}</div>
  <div v-else-if="item.kind === 'image'" class="attachment-content image-attachment">
    <a :href="itemContentUrl()" target="_blank" rel="noopener" class="image-preview-link">
      <img :src="itemContentUrl()" :alt="item.fileName || '剪贴板图片'" loading="lazy" />
    </a>
    <div class="attachment-details"><Image :size="18" /><div><strong>{{ item.fileName }}</strong><span>{{ formatFileSize(item.sizeBytes) }}</span></div></div>
  </div>
  <div v-else class="attachment-content file-attachment">
    <div class="file-icon"><File :size="24" /></div>
    <div class="attachment-details"><div><strong>{{ item.fileName }}</strong><span>{{ formatFileSize(item.sizeBytes) }}<template v-if="item.mediaType"> · {{ item.mediaType }}</template></span></div></div>
  </div>
</template>
