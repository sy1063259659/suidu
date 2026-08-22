<script setup lang="ts">
import { onMounted, ref } from 'vue'
import axios from 'axios'
import { NButton, NCard, NLayout, NLayoutContent, NLayoutHeader, NSpace, NTag } from 'naive-ui'

const apiStatus = ref<'checking' | 'online' | 'offline'>('checking')

async function checkApi() {
  apiStatus.value = 'checking'
  try {
    await axios.get('/api/health')
    apiStatus.value = 'online'
  } catch {
    apiStatus.value = 'offline'
  }
}

onMounted(checkApi)
</script>

<template>
  <n-layout class="app-shell">
    <n-layout-header bordered class="app-header">
      <div>
        <div class="brand">随渡 <span>SUIDU</span></div>
        <div class="subtitle">跨端内容中转与轻量记事</div>
      </div>
      <n-space align="center">
        <n-tag :type="apiStatus === 'online' ? 'success' : apiStatus === 'offline' ? 'error' : 'warning'">
          API {{ apiStatus === 'online' ? '在线' : apiStatus === 'offline' ? '离线' : '检查中' }}
        </n-tag>
        <n-button secondary @click="checkApi">刷新状态</n-button>
      </n-space>
    </n-layout-header>

    <n-layout-content class="app-content">
      <n-card title="项目已初始化" size="large" class="welcome-card">
        <p>前端、Go API、PostgreSQL、Redis 和 SFTPGo 的基础结构已经准备完成。</p>
        <p class="muted">下一步可以从设备配对和文本内容传输开始。</p>
      </n-card>
    </n-layout-content>
  </n-layout>
</template>

