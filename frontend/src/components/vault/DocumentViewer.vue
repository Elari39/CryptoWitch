<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import renderMathInElement from 'katex/contrib/auto-render'
import type { PDFLoadState, VaultDocument } from '../../types/vault'

interface Props {
  document: VaultDocument | null
  loading: boolean
  pdfLoad: PDFLoadState
  pdfProgress: number
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (event: 'aiSelect', text: string): void
}>()
const viewerRef = shallowRef<HTMLElement | null>(null)
const markdownBodyRef = shallowRef<HTMLElement | null>(null)
const pdfLoaded = shallowRef(false)
const selectionButton = shallowRef<{ visible: boolean; top: number; left: number }>({
  visible: false,
  top: 0,
  left: 0,
})
const fileSizeLabel = computed(() => formatSize(props.document?.size))
// 正文首个一级标题：命中则从正文中移除，避免与顶部标题重复；未命中则为空，回退到文件名。
const firstHeading = ref('')
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

// 独立行图片（goldmark 输出 <p><img></p>）提升为居中 figure + alt 图注；
// 内联图片保持原位。外链图片加载失败时替换为占位提示，避免碎图标。
function enhanceImages() {
  const markdownBody = markdownBodyRef.value
  if (!markdownBody || props.document?.documentType !== 'markdown') {
    return
  }
  const images = markdownBody.querySelectorAll('img')
  images.forEach((img) => {
    if (img.dataset.enhanced === '1') return
    img.dataset.enhanced = '1'
    img.loading = 'lazy'
    img.decoding = 'async'
    img.referrerPolicy = 'no-referrer'

    const alt = (img.getAttribute('alt') || '').trim()
    const parent = img.parentElement
    const standalone = parent?.tagName === 'P' && parent.childNodes.length === 1

    if (standalone) {
      const figure = document.createElement('figure')
      figure.className = 'md-figure'
      parent.replaceWith(figure)
      figure.appendChild(img)
      if (alt) {
        const caption = document.createElement('figcaption')
        caption.className = 'md-figure-caption'
        caption.textContent = alt
        figure.appendChild(caption)
      }
    }

    img.addEventListener('error', () => {
      const holder = document.createElement('span')
      holder.className = 'md-img-broken'
      holder.textContent = `图片无法加载：${img.getAttribute('src') || ''}`
      img.replaceWith(holder)
    })
  })
}

// 取正文第一个一级标题作为顶部标题，并将其从正文中移除，避免与顶部重复。
// 代码块内的 '#' 会被 goldmark 渲染为 <code> 文本而非 <h1>，因此天然只命中真正的 Markdown H1。
function extractFirstHeading() {
  const markdownBody = markdownBodyRef.value
  if (!markdownBody || props.document?.documentType !== 'markdown') {
    firstHeading.value = ''
    return
  }
  const heading = markdownBody.querySelector('h1')
  if (heading) {
    firstHeading.value = (heading.textContent || '').trim()
    heading.remove()
  } else {
    firstHeading.value = ''
  }
}

// 顶部标题：优先取正文首个 H1，无则回退到文件名（document.title）。
const headerTitle = computed(() => firstHeading.value || props.document?.title || '')

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
    enhanceImages()
    extractFirstHeading()
  },
  { immediate: true },
)

// 划词 AI 解读：在 Markdown 正文中选中非空文本时，于选区附近显示「AI 解读」按钮。
function isInsideMarkdownBody(node: Node | null): boolean {
  if (!node || !markdownBodyRef.value) {
    return false
  }
  return markdownBodyRef.value.contains(node)
}

function refreshSelectionButton() {
  const selection = window.getSelection()
  const text = (selection?.toString() || '').trim()
  if (!text || !selection || selection.rangeCount === 0 || !isInsideMarkdownBody(selection.anchorNode)) {
    selectionButton.value = { visible: false, top: 0, left: 0 }
    return
  }
  const rect = selection.getRangeAt(0).getBoundingClientRect()
  selectionButton.value = {
    visible: true,
    top: rect.top - 44,
    left: rect.left + rect.width / 2 - 40,
  }
}

function onSelectionChange() {
  // selectionchange 在 document 上触发，只有落在 markdown 正文里才显示按钮。
  const selection = window.getSelection()
  if (!selection || selection.toString().trim() === '' || !isInsideMarkdownBody(selection.anchorNode)) {
    selectionButton.value = { visible: false, top: 0, left: 0 }
    return
  }
  refreshSelectionButton()
}

function onMouseUp(event: MouseEvent) {
  if (!isInsideMarkdownBody(event.target as Node | null)) {
    return
  }
  // 等一帧让选区稳定。
  requestAnimationFrame(refreshSelectionButton)
}

function triggerAI() {
  const text = (window.getSelection()?.toString() || '').trim()
  selectionButton.value = { visible: false, top: 0, left: 0 }
  if (text) {
    emit('aiSelect', text)
  }
}

onMounted(() => {
  document.addEventListener('selectionchange', onSelectionChange)
})

onBeforeUnmount(() => {
  document.removeEventListener('selectionchange', onSelectionChange)
})
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
        <h1 class="document-title">{{ headerTitle }}</h1>
        <p class="document-size markdown-size">{{ fileSizeLabel }}</p>
      </header>
      <div ref="markdownBodyRef" class="markdown-body" v-html="document.html" @mouseup="onMouseUp"></div>
    </article>
    <button
      v-if="selectionButton.visible"
      type="button"
      class="ai-selection-button"
      :style="{ top: `${selectionButton.top}px`, left: `${selectionButton.left}px` }"
      @click="triggerAI"
    >
      AI 解读
    </button>
  </section>
</template>

<style scoped>
.viewer {
  min-width: 0;
  height: 100%;
  overflow: auto;
  overscroll-behavior: contain;
  scroll-behavior: smooth;
  background: var(--canvas);
  color: var(--ink);
  user-select: none;
  scrollbar-color: var(--hairline-soft) var(--canvas);
  scrollbar-width: thin;
}

.viewer::-webkit-scrollbar {
  width: 12px;
}

.viewer::-webkit-scrollbar-track {
  background: var(--canvas);
}

.viewer::-webkit-scrollbar-thumb {
  border: 3px solid var(--canvas);
  border-radius: 999px;
  background: var(--hairline);
}

.viewer::-webkit-scrollbar-thumb:hover {
  background: var(--muted-soft);
}

.viewer-state {
  display: grid;
  place-content: center;
  min-height: 100%;
  padding: 32px;
  color: var(--muted);
  text-align: center;
}

.state-title {
  margin: 0 0 8px;
  color: var(--ink);
  font-family: var(--font-serif);
  font-size: 24px;
  font-weight: 500;
  line-height: 1.3;
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
  background: var(--surface-soft);
}

.pdf-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin: 0;
  padding: 18px 24px;
  border-bottom: 1px solid var(--hairline);
  background: var(--canvas);
}

.pdf-header .document-title {
  color: var(--ink);
  font-family: var(--font-serif);
  font-size: 22px;
  font-weight: 500;
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
  color: var(--muted);
  background: var(--surface-soft);
}

.pdf-progress {
  width: min(320px, 70vw);
  height: 10px;
  accent-color: var(--primary);
}

.pdf-frame {
  width: 100%;
  height: 100%;
  min-height: 0;
  border: 0;
  background: var(--surface-soft);
}

.document-header {
  margin-bottom: 28px;
  padding-bottom: 20px;
  border-bottom: 1px solid transparent;
  background-image: linear-gradient(90deg, transparent, var(--hairline), transparent);
  background-position: bottom;
  background-size: 100% 1px;
  background-repeat: no-repeat;
}

.document-kicker {
  display: inline-block;
  margin: 0 0 10px;
  padding: 3px 12px;
  border: 1px solid var(--hairline);
  border-radius: 999px;
  background: var(--surface-card);
  color: var(--ink);
  font-size: 11px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.document-title {
  margin: 0;
  color: var(--ink);
  font-family: var(--font-serif);
  font-size: 34px;
  font-weight: 500;
  line-height: 1.2;
}

.document-size {
  margin: 0;
  color: var(--muted-soft);
  font-size: 12px;
  font-weight: 500;
}

.markdown-size {
  margin-top: 8px;
  color: var(--muted);
}

.markdown-body {
  font-size: 17px;
  line-height: 1.85;
  color: var(--body);
  /* 放开划词选中，仅限 Markdown 正文；PDF 与其它区域仍由 .viewer 的 user-select:none 禁用 */
  user-select: text;
  -webkit-user-select: text;
}

/* 全局 * { user-select: none } 会直接命中正文每个子元素并阻断继承，
   这里用 :deep(*) 强制让 Markdown 正文内所有元素可选，划词才能生效。 */
.markdown-body :deep(*) {
  user-select: text;
  -webkit-user-select: text;
}

.ai-selection-button {
  position: fixed;
  z-index: 50;
  padding: 7px 14px;
  border: 1px solid var(--primary);
  border-radius: 8px;
  background: var(--primary);
  color: var(--on-primary);
  font-size: 13px;
  font-weight: 500;
  font-family: var(--font-ui);
  cursor: pointer;
  box-shadow: var(--shadow);
}

.ai-selection-button:hover {
  background: var(--primary-active);
  border-color: var(--primary-active);
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
  color: var(--ink);
  font-family: var(--font-serif);
  line-height: 1.3;
  font-weight: 600;
}

.markdown-body :deep(h1) {
  font-size: 1.7em;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--hairline);
}

.markdown-body :deep(h2) {
  font-size: 1.4em;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--hairline);
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
  color: var(--muted);
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
  color: var(--primary);
}

.markdown-body :deep(a) {
  color: var(--primary);
  text-decoration: underline;
  text-underline-offset: 2px;
}

.markdown-body :deep(a:hover) {
  color: var(--primary-active);
}

.markdown-body :deep(strong) {
  color: var(--ink);
  font-weight: 600;
}

.markdown-body :deep(blockquote) {
  margin: 20px 0;
  padding: 12px 18px;
  border-left: 3px solid var(--primary);
  border-radius: 0 8px 8px 0;
  background: var(--primary-wash);
  color: var(--muted);
  font-style: italic;
}

.markdown-body :deep(blockquote p:last-child) {
  margin-bottom: 0;
}

.markdown-body :deep(hr) {
  height: 1px;
  margin: 28px 0;
  border: 0;
  background: linear-gradient(90deg, transparent, var(--hairline), transparent);
}

.markdown-body :deep(img) {
  max-width: 100%;
  height: auto;
  border-radius: 12px;
  background: var(--surface-soft);
}

.markdown-body :deep(.md-figure) {
  margin: 24px 0;
  text-align: center;
}

.markdown-body :deep(.md-figure img) {
  display: inline-block;
}

.markdown-body :deep(.md-figure-caption) {
  margin: 10px 0 0;
  color: var(--muted-soft);
  font-size: 13px;
  font-style: italic;
  font-family: var(--font-ui);
}

.markdown-body :deep(.md-img-broken) {
  display: block;
  margin: 24px auto;
  padding: 14px 18px;
  max-width: 100%;
  box-sizing: border-box;
  border: 1px dashed var(--hairline);
  border-radius: 8px;
  background: var(--surface-soft);
  color: var(--muted-soft);
  font-size: 13px;
  font-family: var(--font-ui);
  word-break: break-all;
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
  border: 1px solid var(--hairline);
  text-align: left;
}

.markdown-body :deep(th) {
  background: var(--surface-soft);
  color: var(--ink);
  font-weight: 600;
}

.markdown-body :deep(tr:nth-child(even)) {
  background: var(--surface-soft);
}

.markdown-body :deep(code) {
  border-radius: 4px;
  padding: 2px 5px;
  background: var(--surface-soft);
  border: 1px solid var(--hairline-soft);
  color: var(--body-strong);
  font-family: var(--font-mono);
  font-size: 0.9em;
}

.markdown-body :deep(pre) {
  position: relative;
  overflow: auto;
  margin: 24px 0;
  border: 1px solid var(--surface-dark-elevated);
  border-radius: 12px;
  padding: 20px 22px;
  /* !important 覆盖 goldmark-highlighting 内联到 <pre> 的浅色背景，统一为深海军代码窗 */
  background: var(--pre-bg) !important;
  color: var(--pre-ink);
  scrollbar-color: var(--surface-dark-elevated) var(--surface-dark);
  scrollbar-width: thin;
}

.markdown-body :deep(pre::-webkit-scrollbar) {
  height: 12px;
}

.markdown-body :deep(pre::-webkit-scrollbar-track) {
  background: var(--surface-dark);
}

.markdown-body :deep(pre::-webkit-scrollbar-thumb) {
  border: 3px solid var(--surface-dark);
  border-radius: 999px;
  background: var(--surface-dark-elevated);
}

.markdown-body :deep(pre code) {
  padding: 0;
  background: transparent;
  border: 0;
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
