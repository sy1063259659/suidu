<script setup lang="ts">
import { computed } from 'vue'
import { ArrowLeft, Copy, Download, Paperclip, Share2, Star, Tags, Trash2 } from '@lucide/vue'
import { NButton, NCard, NEmpty, NPopconfirm, NSpace, NSpin, NTag, NText } from 'naive-ui'
import { clipboardContentUrl, type ClipboardItem } from '../api/clipboard'
import { detectTextFormat } from '../utils/textFormat'
import ClipboardItemContent from './ClipboardItemContent.vue'

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

function formatDate(value: string) {
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit',
  }).format(new Date(value))
}
</script>

<template>
  <main class="clipboard-detail-page">
    <n-button quaternary class="detail-back" @click="emit('back')"><template #icon><ArrowLeft :size="17" /></template>返回剪贴板</n-button>
    <n-card class="detail-card" :bordered="false">
      <div v-if="loading" class="detail-state"><n-spin size="medium" /></div>
      <n-empty v-else-if="unavailable || !item" class="detail-state" description="这条记录不存在或已被删除">
        <template #extra><n-button secondary @click="emit('back')">返回剪贴板</n-button></template>
      </n-empty>
      <template v-else>
        <div class="detail-heading">
          <div><p class="eyebrow">CLIPBOARD DETAIL</p><h1>记录详情</h1></div>
          <n-space :size="8" align="center">
            <n-tag :bordered="false" type="info"><template #icon><Paperclip v-if="item.kind !== 'text'" :size="12" /></template>{{ formatLabel }}</n-tag>
            <n-tag v-if="item.favorite" :bordered="false" type="warning"><template #icon><Star :size="12" fill="currentColor" /></template>已收藏</n-tag>
          </n-space>
        </div>

        <div class="detail-content"><ClipboardItemContent :item="item" /></div>
        <section v-if="item.note" class="detail-note"><span>备注</span><p>{{ item.note }}</p></section>
        <div v-if="item.tags?.length" class="item-tags detail-tags" aria-label="记录标签"><n-tag v-for="tag in item.tags" :key="tag" round :bordered="false" type="info">{{ tag }}</n-tag></div>

        <div class="detail-footer">
          <n-text depth="3">创建于 {{ formatDate(item.createdAt) }}</n-text>
          <div class="detail-actions">
            <n-button :type="item.favorite ? 'warning' : 'default'" secondary :disabled="favoriteUpdating" :aria-busy="favoriteUpdating" @click="emit('favorite', item)"><template #icon><Star :size="16" :fill="item.favorite ? 'currentColor' : 'none'" /></template>{{ item.favorite ? '取消收藏' : '收藏' }}</n-button>
            <n-button v-if="item.kind === 'text' || !item.kind" secondary @click="emit('copy', item)"><template #icon><Copy :size="16" /></template>{{ copied ? '已复制' : '复制' }}</n-button>
            <n-button v-else tag="a" :href="clipboardContentUrl(item.id, true)" secondary><template #icon><Download :size="16" /></template>下载</n-button>
            <n-button secondary @click="emit('organize', item)"><template #icon><Tags :size="16" /></template>整理</n-button>
            <n-button secondary @click="emit('share', item)"><template #icon><Share2 :size="16" /></template>分享</n-button>
            <n-popconfirm @positive-click="emit('delete', item)"><template #trigger><n-button secondary type="error"><template #icon><Trash2 :size="16" /></template>删除</n-button></template>确定删除这条记录吗？</n-popconfirm>
          </div>
        </div>
      </template>
    </n-card>
  </main>
</template>
