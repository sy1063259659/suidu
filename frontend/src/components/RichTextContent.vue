<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ExternalLink } from '@lucide/vue'
import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js/lib/core'
import bash from 'highlight.js/lib/languages/bash'
import css from 'highlight.js/lib/languages/css'
import go from 'highlight.js/lib/languages/go'
import java from 'highlight.js/lib/languages/java'
import javascript from 'highlight.js/lib/languages/javascript'
import json from 'highlight.js/lib/languages/json'
import markdown from 'highlight.js/lib/languages/markdown'
import python from 'highlight.js/lib/languages/python'
import sql from 'highlight.js/lib/languages/sql'
import typescript from 'highlight.js/lib/languages/typescript'
import xml from 'highlight.js/lib/languages/xml'
import yaml from 'highlight.js/lib/languages/yaml'
import { detectTextFormat } from '../utils/textFormat'

const props = withDefaults(defineProps<{ content: string; preview?: boolean }>(), { preview: false })
const emit = defineEmits<{ viewDetail: [] }>()

for (const [name, language] of Object.entries({ bash, css, go, java, javascript, json, markdown, python, sql, typescript, xml, yaml })) {
  hljs.registerLanguage(name, language)
}
hljs.registerAliases(['html'], { languageName: 'xml' })

function escapeHTML(content: string) {
  return content.replace(/[&<>"']/g, (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' })[character] ?? character)
}

const markdownRenderer: InstanceType<typeof MarkdownIt> = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true,
  highlight(code, language): string {
    if (language && hljs.getLanguage(language)) return hljs.highlight(code, { language, ignoreIllegals: true }).value
    return escapeHTML(code)
  },
})
const defaultLinkOpen = markdownRenderer.renderer.rules.link_open
const secureLinkOpen: NonNullable<typeof defaultLinkOpen> = (tokens, index, options, env, self) => {
  tokens[index].attrSet('target', '_blank')
  tokens[index].attrSet('rel', 'noopener noreferrer')
  return defaultLinkOpen?.(tokens, index, options, env, self) ?? self.renderToken(tokens, index, options)
}
markdownRenderer.renderer.rules.link_open = secureLinkOpen

const format = computed(() => detectTextFormat(props.content))
const previewHeight = 220
const contentElement = ref<HTMLElement | null>(null)
const renderedContent = ref<HTMLElement | null>(null)
const shouldClamp = ref(false)
const wrapCode = ref(false)
let resizeObserver: ResizeObserver | undefined
// Bound preview markup, rather than highlighting thousands of offscreen tokens.
// Huge Markdown uses a single safe text node; detail still renders the full source.
const largeMarkdown = computed(() => props.preview && format.value.kind === 'markdown' && format.value.content.length > 16000)
const codeContent = computed(() => {
  const content = format.value.content
  return props.preview ? content.split('\n').slice(0, 120).join('\n') : content
})
const lineCount = computed(() => format.value.content.split('\n').length)
const lineNumbers = computed(() => Array.from({ length: codeContent.value.split('\n').length }, (_, index) => index + 1).join('\n'))
const highlightedCode = computed(() => {
  const language = format.value.language
  if ((!props.preview || codeContent.value.length <= 16000) && language && hljs.getLanguage(language)) {
    return hljs.highlight(codeContent.value, { language, ignoreIllegals: true }).value
  }
  return escapeHTML(codeContent.value)
})
const renderedMarkdown = computed(() => largeMarkdown.value ? '' : markdownRenderer.render(format.value.content))

function measurePreview() {
  shouldClamp.value = Boolean(props.preview && format.value.kind !== 'url' && renderedContent.value && renderedContent.value.getBoundingClientRect().height > previewHeight + 1)
}

onMounted(() => {
  if (typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(measurePreview)
    if (contentElement.value) resizeObserver.observe(contentElement.value)
    if (renderedContent.value) resizeObserver.observe(renderedContent.value)
  }
  measurePreview()
})
watch(() => [props.content, props.preview, wrapCode.value], async () => {
  await nextTick()
  measurePreview()
})
onBeforeUnmount(() => resizeObserver?.disconnect())
const urlHost = computed(() => {
  try { return new URL(format.value.content).hostname }
  catch { return '' }
})

function openPreviewDetail(event: MouseEvent) {
  if (!props.preview) return
  const target = event.target
  if (target instanceof Element && target.closest('a, button, input, textarea, select, [contenteditable]')) return
  if (window.getSelection()?.toString()) return
  emit('viewDetail')
}

function onPreviewKeydown(event: KeyboardEvent) {
  if (!props.preview || event.target !== event.currentTarget || !['Enter', ' '].includes(event.key)) return
  event.preventDefault()
  if (!window.getSelection()?.toString()) emit('viewDetail')
}
</script>

<template>
  <div
    ref="contentElement"
    class="rich-text-content"
    :class="[`format-${format.kind}`, { 'is-preview-clamped': shouldClamp, 'is-preview-interactive': preview }]"
    :role="preview && format.kind !== 'url' ? 'button' : undefined"
    :tabindex="preview && format.kind !== 'url' ? 0 : undefined"
    :aria-label="preview && format.kind !== 'url' ? '查看内容详情' : undefined"
    @click="openPreviewDetail"
    @keydown="onPreviewKeydown"
  >
    <div ref="renderedContent" class="rendered-content">
      <div v-if="format.kind === 'code' || format.kind === 'json'" class="code-panel" :class="{ 'code-wrap': wrapCode }">
        <div class="format-bar">
          <span>{{ format.label }}</span>
          <div class="code-options"><span>{{ lineCount }} 行</span><button v-if="!preview" type="button" :aria-pressed="wrapCode" @click="wrapCode = !wrapCode">{{ wrapCode ? '关闭换行' : '自动换行' }}</button></div>
        </div>
        <pre class="code-scroll" :tabindex="preview ? -1 : 0" aria-label="代码内容，可横向滚动"><span class="code-line-numbers" aria-hidden="true">{{ lineNumbers }}</span><code class="hljs" v-html="highlightedCode" /></pre>
      </div>
      <div v-else-if="format.kind === 'markdown' && !largeMarkdown" class="markdown-content" v-html="renderedMarkdown" />
      <a v-else-if="format.kind === 'url'" :href="format.content" target="_blank" rel="noopener noreferrer" class="standalone-link">
        <span class="standalone-link-icon"><ExternalLink :size="18" /></span>
        <span><strong>{{ urlHost || '网页链接' }}</strong><small>{{ format.content }}</small></span>
      </a>
      <div v-else class="plain-text-content">{{ format.content }}</div>
    </div>
    <div v-if="shouldClamp" class="content-clamp-fade" aria-hidden="true" />
  </div>
</template>

<style scoped>
.rich-text-content { min-width: 0; }
.rendered-content { min-width: 0; display: flow-root; }
.is-preview-interactive { cursor: pointer; }
.is-preview-interactive:focus-visible { outline: 3px solid #93b6ef; outline-offset: 4px; border-radius: 8px; }
.is-preview-clamped { max-height: 220px; overflow: hidden; }
.rich-text-content > .content-clamp-fade { right: 0; bottom: 0; left: 0; height: 64px; }
.rich-text-content.format-code > .content-clamp-fade,
.rich-text-content.format-json > .content-clamp-fade { background: linear-gradient(to bottom, transparent, #111a2a 95%); }
.code-panel .code-scroll { display: flex; align-items: flex-start; max-width: 100%; overflow: auto; }
.code-panel .code-scroll code { flex: 1; padding: 16px; font-size: 13px; line-height: 1.65; white-space: pre; }
.code-line-numbers { flex: 0 0 auto; padding: 16px 10px 16px 14px; border-right: 1px solid #334156; color: #8290a8; background: #111a2a; font-family: "Cascadia Code", "SFMono-Regular", Consolas, monospace; font-size: 13px; line-height: 1.65; text-align: right; user-select: none; }
.code-options { display: flex; align-items: center; gap: 12px; }
.code-options button { padding: 4px 8px; border: 1px solid #4b5e79; border-radius: 4px; background: transparent; color: #dbe7f7; cursor: pointer; }
.code-options button:focus-visible { outline: 2px solid #93b6ef; outline-offset: 2px; }
.code-wrap .code-scroll code { min-width: 0; white-space: pre-wrap; overflow-wrap: anywhere; }
.code-wrap .code-line-numbers { display: none; }
.is-preview-clamped .code-scroll { overflow: hidden; }
.markdown-content :deep(img) { max-width: 100%; height: auto; }
</style>
