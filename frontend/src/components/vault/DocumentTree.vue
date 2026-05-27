<script setup lang="ts">
import { computed } from 'vue'
import type { ReadonlyVaultTreeNode } from '../../types/vault'

interface Props {
  nodes: readonly ReadonlyVaultTreeNode[]
  activeId?: string
}

interface Emits {
  select: [id: string]
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const visibleNodes = computed(() => props.nodes)

function selectNode(node: ReadonlyVaultTreeNode) {
  if (node.kind === 'document' && node.id) {
    emit('select', node.id)
  }
}
</script>

<template>
  <nav class="tree" aria-label="文档目录">
    <p v-if="visibleNodes.length === 0" class="empty-tree">没有可显示的文档。</p>
    <ul v-else class="tree-list">
      <li v-for="node in visibleNodes" :key="`${node.kind}:${node.path}`" class="tree-item">
        <button
          v-if="node.kind === 'document'"
          class="tree-document"
          :class="{ 'tree-document-active': node.id === activeId }"
          type="button"
          @click="selectNode(node)"
        >
          {{ node.title }}
        </button>

        <div v-else class="tree-folder">
          <div class="folder-label">{{ node.title }}</div>
          <DocumentTree :nodes="node.children ?? []" :active-id="activeId" @select="emit('select', $event)" />
        </div>
      </li>
    </ul>
  </nav>
</template>

<style scoped>
.tree {
  height: 100%;
  overflow: auto;
  padding: 18px 12px;
}

.tree-list {
  display: grid;
  gap: 4px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.tree-item {
  min-width: 0;
}

.tree-folder {
  display: grid;
  gap: 6px;
}

.folder-label {
  padding: 10px 10px 6px;
  color: #85c7bc;
  font-size: 12px;
  font-weight: 800;
  text-transform: uppercase;
}

.tree-folder :deep(.tree) {
  height: auto;
  overflow: visible;
  padding: 0 0 0 10px;
  border-left: 1px solid #273447;
}

.tree-document {
  width: 100%;
  min-height: 36px;
  border: 0;
  border-radius: 6px;
  padding: 8px 10px;
  background: transparent;
  color: #cbd5e1;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.tree-document:hover,
.tree-document-active {
  background: #223042;
  color: #f5f0e8;
}

.empty-tree {
  margin: 0;
  padding: 16px;
  color: #8b98aa;
}
</style>
