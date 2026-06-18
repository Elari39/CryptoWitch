<script setup lang="ts">
import { computed, nextTick, shallowRef, watch } from 'vue'
import renderMathInElement from 'katex/contrib/auto-render'
import type { PDFLoadState, VaultDocument } from '../../types/vault'

interface Props {
  document: VaultDocument | null
  loading: boolean
  pdfLoad: PDFLoadState
  pdfProgress: number
}

const props = defineProps<Props>()
const viewerRef = shallowRef<HTMLElement | null>(null)
const markdownBodyRef = shallowRef<HTMLElement | null>(null)
const pdfLoaded = shallowRef(false)
const fileSizeLabel = computed(() => formatSize(props.document?.size))
const pdfLoadingLabel = computed(() => {
  if (props.pdfLoad.totalChunks > 1) {
    return `正在加载 PDF ${props.pdfProgress}%（${props.pdfLoad.loadedChunks}/${props.pdfLoad.totalChunks}）`
  }
  return '正在打开 PDF...'
})

function formatSize(size?: number) {
  if (!size || size <= 0) {
    return '未知大小'
  }
  if (size < 1024) {
    return `${size} B`
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`
  }
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

function renderMarkdownMath() {
  const markdownBody = markdownBodyRef.value
  if (!markdownBody || props.document?.documentType !== 'markdown') {
    return
  }

  renderMathInElement(markdownBody, {
    delimiters: [
      { left: '$$', right: '$$', display: true },
      { left: '\\[', right: '\\]', display: true },
      { left: '\\(', right: '\\)', display: false },
      { left: '$', right: '$', display: false },
    ],
    ignoredTags: ['script', 'noscript', 'style', 'textarea', 'pre', 'code', 'option'],
    throwOnError: false,
  })
}

watch(
  () => props.document?.id,
  async (documentId) => {
    if (!documentId) {
      return
    }
    await nextTick()
    viewerRef.value?.scrollTo({ top: 0, behavior: 'smooth' })
  },
)

watch(
  () => props.document,
  () => {
    pdfLoaded.value = false
  },
  { immediate: true },
)

watch(
  () => [props.document?.id, props.document?.html] as const,
  async () => {
    await nextTick()
    renderMarkdownMath()
  },
  { immediate: true },
)
</script>

<template>
  <section ref="viewerRef" class="viewer" aria-live="polite">
    <div v-if="loading" class="viewer-state">正在加载文档...</div>
    <article v-else-if="document?.documentType === 'pdf'" class="pdf-view">
      <header class="document-header pdf-header">
        <div>
          <p class="document-kicker">PDF</p>
          <h1 class="document-title">{{ document.title }}</h1>
        </div>
        <p class="document-size">{{ fileSizeLabel }}</p>
      </header>
      <div v-if="pdfLoad.url" class="pdf-frame-wrap">
        <div v-if="!pdfLoaded" class="pdf-loading">正在打开 PDF...</div>
        <iframe class="pdf-frame" :src="pdfLoad.url" :title="document.title" @load="pdfLoaded = true"></iframe>
      </div>
      <div v-else class="viewer-state">
        <p class="state-title">{{ pdfLoadingLabel }}</p>
        <progress
          v-if="pdfLoad.totalChunks > 1"
          class="pdf-progress"
          :max="pdfLoad.totalChunks"
          :value="pdfLoad.loadedChunks"
        />
      </div>
    </article>
    <article v-else-if="document" class="markdown-view">
      <header class="document-header">
        <p class="document-kicker">Markdown</p>
        <h1 class="document-title">{{ document.title }}</h1>
        <p class="document-size markdown-size">{{ fileSizeLabel }}</p>
      </header>
      <div ref="markdownBodyRef" class="markdown-body" v-html="document.html"></div>
    </article>
    <div v-else class="viewer-state">
      <p class="state-title">请选择一篇文档或 PDF</p>
      <p class="state-copy">目录加载完成后，内容只会在你点击条目时解密渲染。</p>
    </div>
  </section>
</template>

<style scoped>
.viewer {
  min-width: 0;
  height: 100%;
  overflow: auto;
  overscroll-behavior: contain;
  scroll-behavior: smooth;
  background: var(--paper);
  color: var(--ink);
  user-select: none;
  scrollbar-color: var(--rule-strong) var(--paper);
  scrollbar-width: thin;
}

.viewer::-webkit-scrollbar {
  width: 12px;
}

.viewer::-webkit-scrollbar-track {
  background: var(--paper);
}

.viewer::-webkit-scrollbar-thumb {
  border: 3px solid var(--paper);
  border-radius: 999px;
  background: var(--rule-strong);
}

.viewer::-webkit-scrollbar-thumb:hover {
  background: var(--ink-faint);
}

.viewer-state {
  display: grid;
  place-content: center;
  min-height: 100%;
  padding: 32px;
  color: var(--ink-muted);
  text-align: center;
}

.state-title {
  margin: 0 0 8px;
  color: var(--ink-strong);
  font-size: 24px;
  font-weight: 800;
}

.state-copy {
  margin: 0;
}

.markdown-view {
  max-width: 760px;
  margin: 0 auto;
  padding: 56px 48px 80px;
  font-family: var(--font-serif);
}

.pdf-view {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  height: 100%;
  min-height: 0;
  background: var(--paper-warm);
}

.pdf-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin: 0;
  padding: 18px 24px;
  border-bottom: 1px solid var(--rule);
  background: var(--surface);
}

.pdf-header .document-kicker {
  color: var(--accent);
}

.pdf-header .document-title {
  color: var(--ink-strong);
  font-size: 22px;
}

.pdf-frame-wrap {
  position: relative;
  min-height: 0;
}

.pdf-loading {
  position: absolute;
  inset: 0;
  display: grid;
  place-content: center;
  color: var(--ink-muted);
  background: var(--paper-warm);
}

.pdf-progress {
  width: min(320px, 70vw);
  height: 10px;
  accent-color: var(--accent);
}

.pdf-frame {
  width: 100%;
  height: 100%;
  min-height: 0;
  border: 0;
  background: var(--surface-2);
}

.document-header {
  margin-bottom: 28px;
  padding-bottom: 20px;
  border-bottom: 1px solid transparent;
  background-image: linear-gradient(90deg, transparent, var(--rule), transparent);
  background-position: bottom;
  background-size: 100% 1px;
  background-repeat: no-repeat;
}

.document-kicker {
  display: inline-block;
  margin: 0 0 8px;
  padding: 2px 8px;
  border: 1px solid var(--accent);
  border-radius: 4px;
  color: var(--accent);
  font-size: 11px;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.document-title {
  margin: 0;
  color: var(--ink-strong);
  font-family: var(--font-serif);
  font-size: 34px;
  line-height: 1.2;
}

.document-size {
  margin: 0;
  color: var(--ink-faint);
  font-size: 12px;
  font-weight: 700;
}

.markdown-size {
  margin-top: 8px;
  color: var(--ink-muted);
}

.markdown-body {
  font-size: 17px;
  line-height: 1.85;
  color: var(--ink);
}

.markdown-body :deep(.katex) {
  font-size: 1.05em;
}

.markdown-body :deep(.katex-display) {
  overflow-x: auto;
  overflow-y: hidden;
  margin: 20px 0;
  padding: 4px 2px;
}

.markdown-body :deep(.katex-display > .katex) {
  white-space: nowrap;
}

.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3),
.markdown-body :deep(h4),
.markdown-body :deep(h5),
.markdown-body :deep(h6) {
  margin: 28px 0 12px;
  color: var(--ink-strong);
  font-family: var(--font-serif);
  line-height: 1.3;
  font-weight: 700;
}

.markdown-body :deep(h1) {
  font-size: 1.7em;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--rule);
}

.markdown-body :deep(h2) {
  font-size: 1.4em;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--rule);
}

.markdown-body :deep(h3) {
  font-size: 1.18em;
}

.markdown-body :deep(h4) {
  font-size: 1.04em;
}

.markdown-body :deep(h5),
.markdown-body :deep(h6) {
  font-size: 0.95em;
  color: var(--ink-muted);
}

.markdown-body :deep(p) {
  margin: 0 0 18px;
}

.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  margin: 0 0 18px;
  padding-left: 26px;
}

.markdown-body :deep(li) {
  margin: 4px 0;
}

.markdown-body :deep(li::marker) {
  color: var(--accent);
}

.markdown-body :deep(a) {
  color: var(--accent);
  text-decoration: underline;
  text-underline-offset: 2px;
}

.markdown-body :deep(a:hover) {
  color: var(--accent-strong);
}

.markdown-body :deep(strong) {
  color: var(--ink-strong);
  font-weight: 700;
}

.markdown-body :deep(blockquote) {
  margin: 20px 0;
  padding: 12px 18px;
  border-left: 3px solid var(--accent);
  border-radius: 0 6px 6px 0;
  background: var(--accent-wash);
  color: var(--ink-muted);
  font-style: italic;
}

.markdown-body :deep(blockquote p:last-child) {
  margin-bottom: 0;
}

.markdown-body :deep(hr) {
  height: 1px;
  margin: 28px 0;
  border: 0;
  background: linear-gradient(90deg, transparent, var(--rule-strong), transparent);
}

.markdown-body :deep(img) {
  max-width: 100%;
  height: auto;
  border: 1px solid var(--rule);
  border-radius: 6px;
}

.markdown-body :deep(table) {
  width: 100%;
  margin: 20px 0;
  border-collapse: collapse;
  font-size: 0.95em;
}

.markdown-body :deep(th),
.markdown-body :deep(td) {
  padding: 8px 12px;
  border: 1px solid var(--rule);
  text-align: left;
}

.markdown-body :deep(th) {
  background: var(--surface-2);
  color: var(--ink-strong);
  font-weight: 700;
}

.markdown-body :deep(tr:nth-child(even)) {
  background: var(--accent-wash);
}

.markdown-body :deep(code) {
  border-radius: 4px;
  padding: 2px 5px;
  background: var(--code-bg);
  color: var(--code-ink);
  font-family: var(--font-mono);
  font-size: 0.9em;
}

.markdown-body :deep(pre) {
  position: relative;
  overflow: auto;
  margin: 24px 0;
  border: 1px solid var(--pre-border);
  border-top: 2px solid var(--accent);
  border-radius: 8px;
  padding: 20px 22px;
  /* !important 覆盖 goldmark-highlighting 内联到 <pre> 的深色/浅灰背景，统一为水墨纸色 */
  background: var(--pre-bg) !important;
  color: var(--pre-ink);
  box-shadow: var(--shadow);
  scrollbar-color: var(--rule) var(--pre-bg);
  scrollbar-width: thin;
}

.markdown-body :deep(pre::-webkit-scrollbar) {
  height: 12px;
}

.markdown-body :deep(pre::-webkit-scrollbar-track) {
  background: var(--pre-bg);
}

.markdown-body :deep(pre::-webkit-scrollbar-thumb) {
  border: 3px solid var(--pre-bg);
  border-radius: 999px;
  background: var(--rule-strong);
}

.markdown-body :deep(pre code) {
  padding: 0;
  background: transparent;
  color: var(--pre-ink);
  font-size: 14px;
  line-height: 1.7;
  white-space: pre;
}

@media (max-width: 760px) {
  .markdown-view {
    padding: 32px 24px 56px;
  }
}
</style>
