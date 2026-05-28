<script setup lang="ts">
import { nextTick, shallowRef, watch } from 'vue'
import type { VaultDocument } from '../../types/vault'

interface Props {
  document: VaultDocument | null
  loading: boolean
}

const props = defineProps<Props>()
const viewerRef = shallowRef<HTMLElement | null>(null)

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
</script>

<template>
  <section ref="viewerRef" class="viewer" aria-live="polite">
    <div v-if="loading" class="viewer-state">正在加载文档...</div>
    <article v-else-if="document" class="markdown-view">
      <header class="document-header">
        <p class="document-kicker">Markdown</p>
        <h1 class="document-title">{{ document.title }}</h1>
      </header>
      <div class="markdown-body" v-html="document.html"></div>
    </article>
    <div v-else class="viewer-state">
      <p class="state-title">请选择一篇文档</p>
      <p class="state-copy">目录加载完成后，内容只会在你点击文档时解密渲染。</p>
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
