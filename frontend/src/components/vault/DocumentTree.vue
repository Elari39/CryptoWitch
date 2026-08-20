<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { ReadonlyVaultTreeNode } from '../../types/vault'

interface Props {
  nodes: readonly ReadonlyVaultTreeNode[]
  activeId?: string
  nested?: boolean
}

interface Emits {
  select: [id: string]
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const query = shallowRef('')
const expandedPaths = shallowRef(new Set<string>())
const normalizedQuery = computed(() => query.value.trim().toLowerCase())
const visibleNodes = computed(() => {
  if (props.nested || !normalizedQuery.value) {
    return props.nodes
  }
  return filterNodes(props.nodes, normalizedQuery.value)
})
const documentCount = computed(() => countDocuments(props.nodes))

watch(
  () => [props.nodes, normalizedQuery.value] as const,
  ([nodes, filter]) => {
    expandedPaths.value = collectFolderPaths(nodes)
    if (filter) {
      expandedPaths.value = collectFolderPaths(visibleNodes.value)
    }
  },
  { immediate: true },
)

function collectFolderPaths(nodes: readonly ReadonlyVaultTreeNode[]) {
  const paths = new Set<string>()
  for (const node of nodes) {
    if (node.kind !== 'folder') {
      continue
    }
    paths.add(node.path)
    for (const childPath of collectFolderPaths(node.children ?? [])) {
      paths.add(childPath)
    }
  }
  return paths
}

function filterNodes(nodes: readonly ReadonlyVaultTreeNode[], filter: string): readonly ReadonlyVaultTreeNode[] {
  const matches: ReadonlyVaultTreeNode[] = []
  for (const node of nodes) {
    if (node.kind === 'document') {
      if (node.title.toLowerCase().includes(filter) || node.path.toLowerCase().includes(filter)) {
        matches.push(node)
      }
      continue
    }

    const titleMatches = node.title.toLowerCase().includes(filter)
    const children = titleMatches ? (node.children ?? []) : filterNodes(node.children ?? [], filter)
    if (children.length > 0 || titleMatches) {
      matches.push({
        ...node,
        children,
      })
    }
  }
  return matches
}

function countDocuments(nodes: readonly ReadonlyVaultTreeNode[]) {
  let count = 0
  for (const node of nodes) {
    if (node.kind === 'document') {
      count += 1
    } else {
      count += countDocuments(node.children ?? [])
    }
  }
  return count
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

function documentTypeLabel(node: ReadonlyVaultTreeNode) {
  return node.documentType === 'pdf' ? 'PDF' : 'MD'
}

function isExpanded(node: ReadonlyVaultTreeNode) {
  return expandedPaths.value.has(node.path)
}

function toggleFolder(node: ReadonlyVaultTreeNode) {
  const next = new Set(expandedPaths.value)
  if (next.has(node.path)) {
    next.delete(node.path)
  } else {
    next.add(node.path)
  }
  expandedPaths.value = next
}

function selectNode(node: ReadonlyVaultTreeNode) {
  if (node.kind === 'document' && node.id) {
    emit('select', node.id)
  }
}
</script>

<template>
  <nav class="tree" aria-label="文档目录">
    <div v-if="!nested" class="tree-tools">
      <input
        v-model="query"
        class="tree-search"
        type="search"
        autocomplete="off"
        spellcheck="false"
        placeholder="搜索文档"
        aria-label="搜索文档"
      />
      <p class="tree-count">{{ documentCount }} 个文档</p>
    </div>
    <p v-if="visibleNodes.length === 0" class="empty-tree">
      {{ normalizedQuery ? '没有匹配的文档。' : '没有可显示的文档。' }}
    </p>
    <ul v-else class="tree-list">
      <li v-for="node in visibleNodes" :key="`${node.kind}:${node.path}`" class="tree-item">
        <button
          v-if="node.kind === 'document'"
          class="tree-document"
          :class="{ 'tree-document-active': node.id === activeId }"
          type="button"
          @click="selectNode(node)"
        >
          <span class="document-title">{{ node.title }}</span>
          <span class="document-meta">
            <span class="document-type">{{ documentTypeLabel(node) }}</span>
            <span>{{ formatSize(node.size) }}</span>
          </span>
        </button>

        <div v-else class="tree-folder">
          <button
            class="folder-label"
            type="button"
            :aria-expanded="isExpanded(node)"
            @click="toggleFolder(node)"
          >
            <span class="folder-chevron" aria-hidden="true">{{ isExpanded(node) ? '▾' : '▸' }}</span>
            <span class="folder-title">{{ node.title }}</span>
          </button>
          <DocumentTree
            v-if="isExpanded(node)"
            :nodes="node.children ?? []"
            :active-id="activeId"
            nested
            @select="emit('select', $event)"
          />
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

.tree-tools {
  display: grid;
  gap: 8px;
  margin-bottom: 14px;
  padding: 0 2px;
}

.tree-search {
  width: 100%;
  height: 36px;
  border: 1px solid var(--hairline);
  border-radius: 8px;
  padding: 0 12px;
  outline: none;
  background: var(--canvas);
  color: var(--ink);
}

.tree-search:focus {
  border-color: var(--primary);
  box-shadow: 0 0 0 3px var(--primary-soft);
}

.tree-count {
  margin: 0;
  color: var(--muted-soft);
  font-size: 12px;
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
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr);
  align-items: center;
  width: 100%;
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  padding: 8px 10px;
  background: transparent;
  color: var(--muted);
  font-size: 12px;
  font-weight: 600;
  font-family: inherit;
  text-align: left;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  cursor: pointer;
}

.folder-label:hover {
  background: var(--surface-soft);
}

.folder-chevron {
  color: var(--muted-soft);
  font-size: 13px;
}

.folder-title {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tree-folder :deep(.tree) {
  height: auto;
  overflow: visible;
  padding: 0 0 0 10px;
  border-left: 1px solid var(--hairline);
}

.tree-document {
  display: grid;
  gap: 4px;
  width: 100%;
  min-height: 46px;
  border: 0;
  border-radius: 8px;
  padding: 8px 10px;
  background: transparent;
  color: var(--body);
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.document-title {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.document-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  color: var(--muted-soft);
  font-size: 11px;
}

.document-type {
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--surface-card);
  color: var(--ink);
  font-size: 11px;
  font-weight: 500;
}

.tree-document:hover {
  background: var(--surface-soft);
}

.tree-document-active {
  background: var(--surface-card);
  color: var(--ink);
}

.empty-tree {
  margin: 0;
  padding: 16px;
  color: var(--muted-soft);
}
</style>
