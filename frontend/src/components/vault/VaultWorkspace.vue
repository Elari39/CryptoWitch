<script setup lang="ts">
import { computed } from 'vue'
import AIPanel from './AIPanel.vue'
import DocumentTree from './DocumentTree.vue'
import DocumentViewer from './DocumentViewer.vue'
import { useAI } from '../../composables/useAI'
import type { PDFLoadState, ReadonlyVaultDocument, ReadonlyVaultTreeNode } from '../../types/vault'

interface Props {
  tree: readonly ReadonlyVaultTreeNode[]
  activeDocument: ReadonlyVaultDocument | null
  pdfLoad: PDFLoadState
  pdfProgress: number
  loading: boolean
  documentLoading: boolean
  error: string
}

interface Emits {
  lock: []
  selectDocument: [id: string]
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const ai = useAI()

const activeId = computed(() => props.activeDocument?.id)

function onAISelect(text: string) {
  if (props.activeDocument) {
    ai.openWithSelection(text, props.activeDocument.id)
  }
}
</script>

<template>
  <main class="workspace">
    <aside class="sidebar">
      <div class="sidebar-header">
        <div>
          <p class="app-kicker">CryptoWitch</p>
          <h1 class="app-title">加密文档</h1>
        </div>
        <button class="lock-button" type="button" :disabled="loading" title="锁定" @click="emit('lock')">
          锁定
        </button>
      </div>
      <DocumentTree :nodes="tree" :active-id="activeId" @select="emit('selectDocument', $event)" />
      <footer class="sidebar-footer">
        <span class="footer-label">由</span>
        <a
          class="footer-author"
          href="https://github.com/Elari39/CryptoWitch"
          target="_blank"
          rel="noopener noreferrer"
          title="Elari39 · GitHub"
        >Elari39</a>
        <span class="footer-label">制作</span>
      </footer>
    </aside>

    <section class="content">
      <div v-if="error" class="content-error" role="alert">{{ error }}</div>
      <DocumentViewer
        :document="activeDocument"
        :loading="documentLoading"
        :pdf-load="pdfLoad"
        :pdf-progress="pdfProgress"
        @ai-select="onAISelect"
      />
    </section>

    <AIPanel />
  </main>
</template>

<style scoped>
.workspace {
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr);
  height: 100vh;
  min-height: 0;
  overflow: hidden;
  background: var(--paper-warm);
}

.sidebar {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  min-width: 0;
  min-height: 0;
  border-right: 1px solid var(--rule);
  background: var(--paper-warm);
}

.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 20px;
  border-bottom: 1px solid var(--rule);
}

.app-kicker {
  margin: 0 0 4px;
  color: var(--accent);
  font-size: 12px;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.app-title {
  margin: 0;
  color: var(--ink-strong);
  font-size: 22px;
  line-height: 1.2;
}

.lock-button {
  min-width: 64px;
  height: 34px;
  border: 1px solid var(--rule-strong);
  border-radius: 6px;
  background: var(--surface);
  color: var(--ink-muted);
  font-weight: 700;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.lock-button:hover:not(:disabled) {
  background: var(--surface-2);
  color: var(--ink);
}

.lock-button:disabled {
  cursor: not-allowed;
  opacity: 0.58;
}

.content {
  position: relative;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}

.content-error {
  position: absolute;
  z-index: 2;
  top: 16px;
  right: 18px;
  max-width: min(420px, calc(100% - 36px));
  border: 1px solid var(--accent);
  border-radius: 6px;
  padding: 10px 12px;
  background: var(--accent-wash);
  color: var(--accent-strong);
  box-shadow: var(--shadow);
}

.sidebar-footer {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 14px 16px;
  border-top: 1px solid var(--rule);
  font-size: 12px;
}

.footer-label {
  color: var(--ink-faint);
}

.footer-author {
  color: var(--accent);
  font-weight: 700;
  text-decoration: none;
  border-bottom: 1px dashed var(--accent);
  padding-bottom: 1px;
}

.footer-author:hover {
  color: var(--accent-strong);
  border-bottom-color: var(--accent-strong);
}

@media (max-width: 820px) {
  .workspace {
    grid-template-columns: 1fr;
  }

  .sidebar {
    border-right: 0;
    border-bottom: 1px solid var(--rule);
  }
}
</style>
