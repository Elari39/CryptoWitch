<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, shallowRef, watch } from 'vue'
import type { VaultDocument } from '../../types/vault'

interface Props {
  document: VaultDocument | null
  loading: boolean
}

const props = defineProps<Props>()
const viewerRef = shallowRef<HTMLElement | null>(null)
const pdfUrl = shallowRef('')
const pdfLoaded = shallowRef(false)
const fileSizeLabel = computed(() => formatSize(props.document?.size))

function revokePDFURL() {
  if (pdfUrl.value) {
    URL.revokeObjectURL(pdfUrl.value)
    pdfUrl.value = ''
  }
}

function base64ToBlobURL(contentBase64: string, mimeType: string) {
  const binary = window.atob(contentBase64)
  const bytes = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index)
  }
  return URL.createObjectURL(new Blob([bytes], { type: mimeType }))
}

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
  (document) => {
    revokePDFURL()
    pdfLoaded.value = false
    if (document?.documentType === 'pdf' && document.contentBase64) {
      pdfUrl.value = base64ToBlobURL(document.contentBase64, document.mimeType || 'application/pdf')
    }
  },
  { immediate: true },
)

onBeforeUnmount(revokePDFURL)
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
      <div v-if="pdfUrl" class="pdf-frame-wrap">
        <div v-if="!pdfLoaded" class="pdf-loading">正在打开 PDF...</div>
        <iframe class="pdf-frame" :src="pdfUrl" :title="document.title" @load="pdfLoaded = true"></iframe>
      </div>
      <div v-else class="viewer-state">PDF 加载失败，请重新选择文档。</div>
    </article>
    <article v-else-if="document" class="markdown-view">
      <header class="document-header">
        <p class="document-kicker">Markdown</p>
        <h1 class="document-title">{{ document.title }}</h1>
        <p class="document-size markdown-size">{{ fileSizeLabel }}</p>
      </header>
      <div class="markdown-body" v-html="document.html"></div>
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
  background: #f8f6f1;
  color: #17202b;
  user-select: none;
  scrollbar-color: #c8bca9 #f1ece3;
  scrollbar-width: thin;
}

.viewer::-webkit-scrollbar {
  width: 12px;
}

.viewer::-webkit-scrollbar-track {
  background: #f1ece3;
}

.viewer::-webkit-scrollbar-thumb {
  border: 3px solid #f1ece3;
  border-radius: 999px;
  background: #c8bca9;
}

.viewer::-webkit-scrollbar-thumb:hover {
  background: #ac9d86;
}

.viewer-state {
  display: grid;
  place-content: center;
  min-height: 100%;
  padding: 32px;
  color: #586476;
  text-align: center;
}

.state-title {
  margin: 0 0 8px;
  color: #1f2937;
  font-size: 24px;
  font-weight: 800;
}

.state-copy {
  margin: 0;
}

.markdown-view {
  max-width: 860px;
  margin: 0 auto;
  padding: 48px 56px 72px;
}

.pdf-view {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  height: 100%;
  min-height: 0;
  background: #1b2230;
}

.pdf-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin: 0;
  padding: 18px 24px;
  border-bottom: 1px solid #313c4f;
  background: #111722;
}

.pdf-header .document-kicker {
  color: #85c7bc;
}

.pdf-header .document-title {
  color: #f5f0e8;
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
  color: #b8c6d8;
  background: #1b2230;
}

.pdf-frame {
  width: 100%;
  height: 100%;
  min-height: 0;
  border: 0;
  background: #2a3140;
}

.document-header {
  margin-bottom: 28px;
  padding-bottom: 20px;
  border-bottom: 1px solid #d8d3c8;
}

.document-kicker {
  margin: 0 0 8px;
  color: #b08547;
  font-size: 12px;
  font-weight: 800;
  text-transform: uppercase;
}

.document-title {
  margin: 0;
  color: #101820;
  font-size: 34px;
  line-height: 1.2;
}

.document-size {
  margin: 0;
  color: #9caabd;
  font-size: 12px;
  font-weight: 700;
}

.markdown-size {
  margin-top: 8px;
  color: #697386;
}

.markdown-body {
  font-size: 16px;
  line-height: 1.75;
}

.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3) {
  margin: 28px 0 12px;
  color: #111827;
  line-height: 1.25;
}

.markdown-body :deep(p) {
  margin: 0 0 16px;
}

.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  margin: 0 0 18px;
  padding-left: 24px;
}

.markdown-body :deep(code) {
  border-radius: 4px;
  padding: 2px 5px;
  background: #ebe4d8;
  color: #7c3e14;
  font-family: "SFMono-Regular", "Cascadia Code", "JetBrains Mono", Consolas, monospace;
  font-size: 0.92em;
}

.markdown-body :deep(pre) {
  position: relative;
  overflow: auto;
  margin: 24px 0;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  padding: 46px 20px 20px;
  background: #10151f;
  color: #edf2f7;
  box-shadow: 0 18px 48px rgba(16, 21, 31, 0.22);
  scrollbar-color: #586476 #10151f;
  scrollbar-width: thin;
}

.markdown-body :deep(pre::before) {
  position: absolute;
  top: 18px;
  left: 18px;
  width: 12px;
  height: 12px;
  border-radius: 999px;
  background: #ff5f57;
  box-shadow:
    20px 0 0 #ffbd2e,
    40px 0 0 #28c840;
  content: "";
}

.markdown-body :deep(pre::-webkit-scrollbar) {
  height: 12px;
}

.markdown-body :deep(pre::-webkit-scrollbar-track) {
  background: #10151f;
}

.markdown-body :deep(pre::-webkit-scrollbar-thumb) {
  border: 3px solid #10151f;
  border-radius: 999px;
  background: #586476;
}

.markdown-body :deep(pre code) {
  padding: 0;
  background: transparent;
  color: inherit;
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
