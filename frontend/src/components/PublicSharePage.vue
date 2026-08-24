<script setup lang="ts">
import axios from 'axios'
import { ClipboardList, Copy, Download } from '@lucide/vue'
import { NButton, NCard, NSpin, NTag } from 'naive-ui'
import { onMounted, ref, shallowRef } from 'vue'
import { getPublicClipboardShare, publicShareContentUrl, type PublicClipboardShare } from '../api/shares'
import ClipboardItemContent from './ClipboardItemContent.vue'

const props = defineProps<{ token: string }>()

const share = shallowRef<PublicClipboardShare | null>(null)
const loading = ref(true)
const unavailable = ref(false)
const copied = ref(false)

function formatDate(value: string) {
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
  }).format(new Date(value))
}

function contentUrl(download = false) {
  return publicShareContentUrl(props.token, download)
}

async function copyText() {
  if (!share.value?.item.content || !navigator.clipboard?.writeText) return
  await navigator.clipboard.writeText(share.value.item.content)
  copied.value = true
  window.setTimeout(() => { copied.value = false }, 1600)
}

onMounted(async () => {
  try {
    share.value = await getPublicClipboardShare(props.token)
  } catch (error) {
    unavailable.value = axios.isAxiosError(error) && error.response?.status === 404
    if (!unavailable.value) unavailable.value = true
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <main class="public-share-page">
    <div class="public-share-brand"><span class="public-share-mark"><ClipboardList :size="19" /></span><strong>随渡</strong></div>
    <n-card class="public-share-card" :bordered="false">
      <div v-if="loading" class="public-share-state"><n-spin size="medium" /></div>
      <div v-else-if="unavailable || !share" class="public-share-state unavailable-state">
        <h1>链接已失效</h1><p>分享可能已过期或被创建者关闭。</p>
      </div>
      <template v-else>
        <div class="public-share-heading"><div><p class="eyebrow">PUBLIC SHARE</p><h1>分享内容</h1></div><n-tag type="success" :bordered="false">有效</n-tag></div>
        <div class="public-share-content"><ClipboardItemContent :item="share.item" :content-url="contentUrl" /></div>
        <div class="public-share-footer">
          <span>有效期至 {{ formatDate(share.expiresAt) }}</span>
          <div class="public-share-actions">
            <n-button v-if="share.item.kind === 'text' || !share.item.kind" secondary @click="copyText"><template #icon><Copy :size="16" /></template>{{ copied ? '已复制' : '复制文本' }}</n-button>
            <n-button v-else tag="a" :href="contentUrl(true)" type="primary"><template #icon><Download :size="16" /></template>下载文件</n-button>
          </div>
        </div>
      </template>
    </n-card>
  </main>
</template>
