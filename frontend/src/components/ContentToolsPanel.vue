<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { Check, Copy, WandSparkles } from '@lucide/vue'
import { NAlert, NButton, NCard, NTag } from 'naive-ui'
import { analyzeContentTools, type ContentToolAction } from '../utils/contentTools'

const props = defineProps<{ content: string }>()
const analysis = computed(() => analyzeContentTools(props.content))
const activeAction = shallowRef<ContentToolAction | null>(null)
const copied = shallowRef(false)
const copyError = shallowRef(false)

watch(() => props.content, () => {
  activeAction.value = null
  copied.value = false
  copyError.value = false
})

function run(action: ContentToolAction) {
  activeAction.value = action
  copied.value = false
  copyError.value = false
}

async function copyResult() {
  if (!activeAction.value || !navigator.clipboard?.writeText) {
    copyError.value = true
    return
  }
  try {
    await navigator.clipboard.writeText(activeAction.value.result)
    copied.value = true
    copyError.value = false
    window.setTimeout(() => { copied.value = false }, 1600)
  } catch {
    copyError.value = true
  }
}
</script>

<template>
  <n-card class="content-tools-panel" :bordered="false">
    <div class="content-tools-heading">
      <div class="content-tools-title"><span><WandSparkles :size="17" /></span><div><h2>快捷转换</h2><p>全部在当前浏览器中处理，不会修改原内容。</p></div></div>
      <n-tag :bordered="false" type="info">{{ analysis.actions.length }} 个工具</n-tag>
    </div>

    <div v-if="analysis.url || analysis.timestamp || analysis.color" class="content-insights">
      <div v-if="analysis.url" class="content-insight"><span>链接域名</span><strong>{{ analysis.url.host }}</strong><small>{{ analysis.url.queryCount }} 个查询参数</small></div>
      <div v-if="analysis.timestamp" class="content-insight"><span>时间戳</span><strong>{{ analysis.timestamp.local }}</strong><small>{{ analysis.timestamp.iso }}</small></div>
      <div v-if="analysis.color" class="content-insight color-insight"><span class="color-swatch" :style="{ backgroundColor: analysis.color }" /><div><span>颜色值</span><strong>{{ analysis.color }}</strong></div></div>
    </div>

    <n-alert v-if="analysis.jsonState === 'invalid'" type="warning" class="content-tool-alert">内容看起来像 JSON，但格式校验未通过。</n-alert>

    <div class="content-tool-actions">
      <n-button v-for="action in analysis.actions" :key="action.id" size="small" :type="activeAction?.id === action.id ? 'primary' : 'default'" :secondary="activeAction?.id === action.id" @click="run(action)">{{ action.label }}</n-button>
    </div>

    <section v-if="activeAction" class="content-tool-result">
      <header><strong>{{ activeAction.label }}结果</strong><n-button size="small" secondary @click="copyResult"><template #icon><Check v-if="copied" :size="15" /><Copy v-else :size="15" /></template>{{ copied ? '已复制' : '复制结果' }}</n-button></header>
      <pre>{{ activeAction.result }}</pre>
      <n-alert v-if="copyError" type="error" class="content-tool-alert">复制失败，请检查浏览器剪贴板权限。</n-alert>
    </section>
  </n-card>
</template>

<style scoped>
.content-tools-panel { margin-top: 18px; border: 1px solid #dfe7f3; background: linear-gradient(145deg, #fff, #f8faff); box-shadow: 0 8px 24px rgba(34, 52, 84, .04); }
.content-tools-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.content-tools-title { display: flex; align-items: flex-start; gap: 11px; }
.content-tools-title > span { display: grid; flex: 0 0 34px; place-items: center; width: 34px; height: 34px; border-radius: 8px; color: #2d6cdf; background: #eaf1ff; }
.content-tools-title h2 { margin: 0; font-size: 18px; }
.content-tools-title p { margin: 5px 0 0; color: #78869a; font-size: 12px; line-height: 1.5; }
.content-insights { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: 10px; margin-top: 16px; }
.content-insight { display: grid; gap: 4px; min-width: 0; padding: 12px 14px; border: 1px solid #e3eaf4; border-radius: 8px; background: rgba(255, 255, 255, .82); }
.content-insight span, .content-insight small { overflow: hidden; color: #78869a; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.content-insight strong { overflow: hidden; color: #25344b; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.color-insight { display: flex; align-items: center; }
.color-insight > div { display: grid; gap: 4px; min-width: 0; }
.color-swatch { flex: 0 0 38px; width: 38px; height: 38px; border: 1px solid rgba(23, 32, 51, .12); border-radius: 8px; }
.content-tool-actions { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 16px; }
.content-tool-result { margin-top: 16px; overflow: hidden; border: 1px solid #dfe7f3; border-radius: 8px; background: #fff; }
.content-tool-result header { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 10px 12px; border-bottom: 1px solid #e5ebf3; background: #f7f9fc; }
.content-tool-result pre { max-height: 360px; margin: 0; overflow: auto; padding: 14px; color: #243247; font: 12px/1.65 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; white-space: pre-wrap; overflow-wrap: anywhere; }
.content-tool-alert { margin-top: 14px; }
@media (max-width: 640px) {
  .content-tools-heading { align-items: flex-start; flex-direction: column; }
  .content-tool-actions .n-button { min-height: 40px; }
  .content-tool-result header { align-items: flex-start; flex-direction: column; }
}
</style>
