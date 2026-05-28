<script setup lang="ts">
import { computed } from 'vue'
import DocumentTree from './DocumentTree.vue'
import DocumentViewer from './DocumentViewer.vue'
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

const activeId = computed(() => props.activeDocument?.id)
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
    </aside>

    <section class="content">
      <div v-if="error" class="content-error" role="alert">{{ error }}</div>
      <DocumentViewer
        :document="activeDocument"
        :loading="documentLoading"
        :pdf-load="pdfLoad"
        :pdf-progress="pdfProgress"
      />
    </section>
  </main>
</template>

<style scoped>
.workspace {
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr);
  height: 100vh;
  min-height: 0;
  overflow: hidden;
  background: #0e1218;
}

.sidebar {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  min-width: 0;
  min-height: 0;
  border-right: 1px solid #253042;
  background: #151b24;
}

.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 20px;
  border-bottom: 1px solid #253042;
}

.app-kicker {
  margin: 0 0 4px;
  color: #85c7bc;
  font-size: 12px;
  font-weight: 800;
  text-transform: uppercase;
}

.app-title {
  margin: 0;
  color: #f5f0e8;
  font-size: 22px;
  line-height: 1.2;
}

.lock-button {
  min-width: 64px;
  height: 34px;
  border: 1px solid #3a4a62;
  border-radius: 6px;
  background: #1e2938;
  color: #dce5f2;
  font-weight: 700;
  cursor: pointer;
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
  border: 1px solid #f2a39a;
  border-radius: 6px;
  padding: 10px 12px;
  background: #fff4f1;
  color: #8a2f25;
  box-shadow: 0 8px 28px rgba(0, 0, 0, 0.18);
}

@media (max-width: 820px) {
  .workspace {
    grid-template-columns: 1fr;
  }

  .sidebar {
    border-right: 0;
    border-bottom: 1px solid #253042;
  }
}
</style>
