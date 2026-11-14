<script setup lang="ts">
import { computed, ref } from 'vue'
import { ArrowLeft, Check, Copy, Download, MoreHorizontal, Paperclip, Pencil, Star } from '@lucide/vue'
import { NButton, NCard, NDropdown, NEmpty, NSpace, NSpin, NTag, NText } from 'naive-ui'
import { clipboardContentUrl, type ClipboardItem } from '../api/clipboard'
import { detectTextFormat } from '../utils/textFormat'
import ClipboardItemContent from './ClipboardItemContent.vue'
import ContentToolsPanel from './ContentToolsPanel.vue'

const props = defineProps<{
  item: ClipboardItem | null
  loading: boolean
  unavailable: boolean
  favoriteUpdating: boolean
  copied: boolean
}>()
const emit = defineEmits<{
  back: []
  copy: [item: ClipboardItem]
  edit: [item: ClipboardItem]
  favorite: [item: ClipboardItem]
  organize: [item: ClipboardItem]
  share: [item: ClipboardItem]
  delete: [item: ClipboardItem]
}>()

const formatLabel = computed(() => {
  if (!props.item) return ''
  if (props.item.kind === 'image') return '图片'
  if (props.item.kind === 'file') return '文件'
  return detectTextFormat(props.item.content ?? '').label
})

const moreOpen = ref(false)
const moreOptions = computed(() => [
  { key: 'favorite', label: props.item?.favorite ? '取消收藏' : '收藏', disabled: props.favoriteUpdating },
  { key: 'organize', label: '整理标签与备注' },
  { key: 'share', label: '分享' },
  { type: 'divider', key: 'divider' },
  { key: 'delete', label: '移入回收站' },
])
function selectMore(key: string | number) {
  if (!props.item) return
  if (key === 'favorite') emit('favorite', props.item)
  if (key === 'organize') emit('organize', props.item)
  if (key === 'share') emit('share', props.item)
  if (key === 'delete') emit('delete', props.item)
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit',
  }).format(new Date(value))
}
</script>

<template>
  <main class="clipboard-detail-page">
    <div class="detail-toolbar" aria-label="内容操作">
      <n-button quaternary class="detail-back" aria-label="返回内容库" @click="emit('back')"><template #icon><ArrowLeft :size="17" /></template><span class="back-label">返回内容库</span></n-button>
      <div v-if="item && !loading && !unavailable" class="detail-actions">
        <n-button v-if="item.kind === 'text' || !item.kind" class="detail-action-button" :type="copied ? 'success' : 'primary'" @click="emit('copy', item)"><template #icon><Check v-if="copied" :size="16" /><Copy v-else :size="16" /></template>{{ copied ? '已复制' : '复制全部' }}</n-button>
        <n-button v-else tag="a" class="detail-action-button" :href="clipboardContentUrl(item.id, true)" type="primary"><template #icon><Download :size="16" /></template>下载</n-button>
        <n-button v-if="item.kind === 'text' || !item.kind" class="detail-action-button" secondary @click="emit('edit', item)"><template #icon><Pencil :size="16" /></template>编辑</n-button>
        <n-dropdown trigger="click" placement="bottom-end" :options="moreOptions" @select="selectMore" @update:show="moreOpen = $event">
          <n-button secondary class="detail-action-button detail-more" aria-label="更多内容操作" aria-haspopup="menu" :aria-expanded="moreOpen" :aria-busy="favoriteUpdating"><template #icon><MoreHorizontal :size="18" /></template><span class="more-label">更多</span></n-button>
        </n-dropdown>
      </div>
      <span class="copy-announcement" role="status" aria-live="polite">{{ copied ? '内容已复制' : '' }}</span>
    </div>
    <n-card class="detail-card" :bordered="false">
      <div v-if="loading" class="detail-state"><n-spin size="medium" /></div>
      <n-empty v-else-if="unavailable || !item" class="detail-state" description="这条记录不存在或已被删除">
        <template #extra><n-button secondary @click="emit('back')">返回内容库</n-button></template>
      </n-empty>
      <template v-else>
        <section class="detail-header">
          <div class="detail-heading">
            <div class="detail-heading-copy">
              <p class="eyebrow">CONTENT DETAIL</p>
              <h1>内容详情</h1>
              <n-text depth="3" class="detail-created-at">创建于 {{ formatDate(item.createdAt) }}</n-text>
            </div>
            <n-space class="detail-heading-status" :size="8" align="center">
              <n-tag :bordered="false" type="info"><template #icon><Paperclip v-if="item.kind !== 'text'" :size="12" /></template>{{ formatLabel }}</n-tag>
              <n-tag v-if="item.favorite" :bordered="false" type="warning"><template #icon><Star :size="12" fill="currentColor" /></template>已收藏</n-tag>
            </n-space>
          </div>

        </section>

        <div class="detail-content"><ClipboardItemContent :item="item" /></div>
        <ContentToolsPanel v-if="item.kind === 'text' || !item.kind" :content="item.content || ''" />
        <section v-if="item.note" class="detail-note"><span>备注</span><p>{{ item.note }}</p></section>
        <div v-if="item.tags?.length" class="item-tags detail-tags" aria-label="记录标签"><n-tag v-for="tag in item.tags" :key="tag" round :bordered="false" type="info">{{ tag }}</n-tag></div>
      </template>
    </n-card>
  </main>
</template>

<style scoped>
.clipboard-detail-page { min-width: 0; }
.detail-toolbar { position: sticky; top: 8px; z-index: 25; display: flex; align-items: center; gap: 8px; margin: 0 0 16px; padding: 10px; border: 1px solid #e0e7f0; border-radius: 12px; background: rgba(255, 255, 255, .97); box-shadow: 0 4px 20px rgba(34, 52, 84, .08); }
.detail-toolbar .detail-back { flex: 0 0 auto; margin: 0; min-height: 44px; }
.detail-toolbar .detail-actions { justify-content: flex-end; flex-wrap: nowrap; gap: 8px; }
.detail-action-button { min-height: 44px; flex: 0 0 auto; }
.copy-announcement { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
.detail-content { min-width: 0; }
@media (max-width: 520px) {
  .detail-toolbar { top: 4px; gap: 4px; padding: 6px; border-radius: 10px; }
  .detail-toolbar .detail-actions { gap: 4px; }
  .detail-toolbar :deep(.n-button) { padding-right: 10px; padding-left: 10px; }
  .back-label, .more-label { display: none; }
  .detail-back :deep(.n-button__icon), .detail-more :deep(.n-button__icon) { margin-right: 0; }
}
</style>
