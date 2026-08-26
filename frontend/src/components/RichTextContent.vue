<script setup lang="ts">
import { computed } from 'vue'
import { ArrowRight, ExternalLink } from '@lucide/vue'
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
import { detectTextFormat, isLongText } from '../utils/textFormat'

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
const long = computed(() => isLongText(props.content))
const clampableKinds = new Set(['text', 'markdown', 'code', 'json'])
const shouldClamp = computed(() => props.preview && long.value && clampableKinds.has(format.value.kind))
const detailActionLabel = '前往详情页'
const detailActionAriaLabel = '前往详情页查看完整内容'
const highlightedCode = computed(() => {
  const language = format.value.language
  if (language && hljs.getLanguage(language)) return hljs.highlight(format.value.content, { language, ignoreIllegals: true }).value
  return escapeHTML(format.value.content)
})
const renderedMarkdown = computed(() => markdownRenderer.render(format.value.content))
const urlHost = computed(() => {
  try { return new URL(format.value.content).hostname }
  catch { return '' }
})
</script>

<template>
  <div class="rich-text-content" :class="[`format-${format.kind}`, { 'is-preview-clamped': shouldClamp }]">
    <div v-if="format.kind === 'code' || format.kind === 'json'" class="code-panel">
      <div class="format-bar"><span>{{ format.label }}</span><span>{{ format.content.split('\n').length }} 行</span></div>
      <pre><code class="hljs" v-html="highlightedCode" /></pre>
    </div>
    <div v-else-if="format.kind === 'markdown'" class="markdown-content" v-html="renderedMarkdown" />
    <a v-else-if="format.kind === 'url'" :href="format.content" target="_blank" rel="noopener noreferrer" class="standalone-link">
      <span class="standalone-link-icon"><ExternalLink :size="18" /></span>
      <span><strong>{{ urlHost || '网页链接' }}</strong><small>{{ format.content }}</small></span>
    </a>
    <div v-else class="plain-text-content">{{ format.content }}</div>
    <div v-if="shouldClamp" class="preview-detail-entry">
      <div class="content-clamp-fade" aria-hidden="true" />
      <button
        type="button"
        class="view-full-content"
        :aria-label="detailActionAriaLabel"
        @click="emit('viewDetail')"
      >
        <span>{{ detailActionLabel }}</span>
        <ArrowRight :size="15" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.preview-detail-entry {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  display: flex;
  align-items: flex-end;
  justify-content: flex-end;
  min-height: 76px;
  pointer-events: none;
}

.preview-detail-entry .content-clamp-fade {
  right: 0;
  bottom: 0;
  left: 0;
  height: 88px;
}

.preview-detail-entry .view-full-content {
  position: relative;
  z-index: 1;
  margin-top: 0;
  margin-right: 12px;
  margin-bottom: 10px;
  white-space: nowrap;
  pointer-events: auto;
  box-shadow: 0 8px 20px rgba(36, 50, 71, .16);
}
</style>
