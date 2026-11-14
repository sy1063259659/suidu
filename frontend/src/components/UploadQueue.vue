<script setup lang="ts">
import { NButton, NProgress } from 'naive-ui'

interface Entry { id: string; name: string; file?: File; status: 'uploading' | 'failed' | 'done' | 'skipped'; percent: number; error?: string }
defineProps<{ entries: Entry[] }>()
const emit = defineEmits<{ retry: [entry: Entry]; dismiss: [id: string] }>()
</script>

<template>
  <div v-if="entries.length" class="upload-queue library-upload-queue" aria-label="上传记录">
    <div v-for="entry in entries" :key="entry.id" class="upload-entry">
      <div class="upload-entry-heading"><strong>{{ entry.name }}</strong><span>{{ entry.status === 'done' ? '已上传' : entry.status === 'failed' ? '上传失败' : entry.status === 'skipped' ? '已取消重复上传' : entry.percent === 100 ? '正在保存…' : `${entry.percent}%` }}</span></div>
      <n-progress v-if="entry.status === 'uploading'" type="line" :percentage="entry.percent" :show-indicator="false" />
      <p v-if="entry.error" role="alert">{{ entry.error }}</p>
      <div v-if="entry.status !== 'uploading'" class="upload-entry-actions"><n-button v-if="entry.status === 'failed' && entry.file && entry.file.size <= 100 * 1024 * 1024" size="small" @click="emit('retry', entry)">重试上传</n-button><n-button quaternary size="small" @click="emit('dismiss', entry.id)">关闭</n-button></div>
    </div>
  </div>
</template>
