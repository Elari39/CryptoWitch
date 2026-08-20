<script setup lang="ts">
import { computed, ref } from 'vue'
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

// 左侧文档目录的收缩/展开状态（会话内有效，锁定后随组件卸载重置）。
const sidebarOpen = ref(true)

// 左右面板宽度（会话内有效，锁定后重置为默认值）；AI 宽度经 CSS 变量驱动抽屉与正文让位。
const sidebarWidth = ref(300)
const aiWidth = ref(420)

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max)
}

// 拖拽调宽：pointerdown 记录起点，window 级 pointermove 实时更新宽度，pointerup 清理。
function onResizeStart(event: PointerEvent, kind: 'sidebar' | 'ai') {
  event.preventDefault()
  const startX = event.clientX
  const startWidth = kind === 'sidebar' ? sidebarWidth.value : aiWidth.value
  const isSidebar = kind === 'sidebar'

  const onMove = (moveEvent: PointerEvent) => {
    const delta = isSidebar ? moveEvent.clientX - startX : startX - moveEvent.clientX
    const min = isSidebar ? 180 : 260
    const max = Math.min(isSidebar ? 520 : 640, window.innerWidth - 160)
    const next = clamp(startWidth + delta, min, max)
    if (isSidebar) {
      sidebarWidth.value = next
    } else {
      aiWidth.value = next
    }
  }
  const onUp = () => {
    window.removeEventListener('pointermove', onMove)
    window.removeEventListener('pointerup', onUp)
    window.removeEventListener('pointercancel', onUp)
    document.body.classList.remove('is-resizing')
  }

  window.addEventListener('pointermove', onMove)
  window.addEventListener('pointerup', onUp)
  window.addEventListener('pointercancel', onUp)
  document.body.classList.add('is-resizing')
}

function onAISelect(text: string) {
  if (props.activeDocument) {
    ai.openWithSelection(text, props.activeDocument.id)
  }
}
</script>

<template>
  <main class="workspace" :style="{ '--ai-width': aiWidth + 'px' }">
    <div
      class="sidebar-slot"
      :class="{ 'is-collapsed': !sidebarOpen }"
      :style="sidebarOpen ? { width: sidebarWidth + 'px' } : undefined"
    >
      <aside class="sidebar" aria-label="文档目录">
        <div class="sidebar-header">
          <div>
            <p class="app-kicker">CryptoWitch</p>
            <h1 class="app-title">加密文档</h1>
          </div>
          <div class="sidebar-actions">
            <button class="lock-button" type="button" :disabled="loading" title="锁定" @click="emit('lock')">
              锁定
            </button>
            <button class="collapse-button" type="button" title="收起文档目录" @click="sidebarOpen = false">
              <svg
                viewBox="0 0 24 24"
                width="14"
                height="14"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
                aria-hidden="true"
              ><polyline points="15 18 9 12 15 6" /></svg>
            </button>
          </div>
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

      <div class="sidebar-rail" aria-label="文档目录操作">
        <button class="rail-button" type="button" title="展开文档目录" @click="sidebarOpen = true">
          <svg
            viewBox="0 0 24 24"
            width="14"
            height="14"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          ><polyline points="9 18 15 12 9 6" /></svg>
        </button>
        <button class="rail-button" type="button" :disabled="loading" title="锁定" @click="emit('lock')">
          <svg
            viewBox="0 0 24 24"
            width="14"
            height="14"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          ><rect x="3" y="11" width="18" height="11" rx="2" ry="2" /><path d="M7 11V7a5 5 0 0 1 10 0v4" /></svg>
        </button>
      </div>
      <div v-if="sidebarOpen" class="sidebar-resizer" title="拖拽调整目录宽度" @pointerdown="onResizeStart($event, 'sidebar')"></div>
    </div>

    <section class="content" :class="{ 'is-ai-open': ai.open.value }">
      <div v-if="error" class="content-error" role="alert">{{ error }}</div>
      <DocumentViewer
        :document="activeDocument"
        :loading="documentLoading"
        :pdf-load="pdfLoad"
        :pdf-progress="pdfProgress"
        @ai-select="onAISelect"
      />
    </section>

    <div
      v-if="ai.open.value"
      class="ai-resizer"
      title="拖拽调整 AI 面板宽度"
      @pointerdown="onResizeStart($event, 'ai')"
    ></div>

    <AIPanel />
  </main>
</template>

<style scoped>
.workspace {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  height: 100vh;
  min-height: 0;
  overflow: hidden;
  background: var(--canvas);
}

/* 侧栏槽位：宽度动画承担收起/展开过渡，内容由 .sidebar 内部滚动 */
.sidebar-slot {
  position: relative;
  width: 300px;
  min-width: 0;
  height: 100%;
  overflow: hidden;
  border-right: 1px solid var(--hairline);
  background: var(--canvas);
  transition: width 0.24s ease;
}

.sidebar-slot.is-collapsed {
  width: 44px;
}

.sidebar {
  position: absolute;
  inset: 0;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  min-width: 0;
  min-height: 0;
  background: var(--canvas);
}

.sidebar-slot.is-collapsed .sidebar {
  /* 滑出动画结束后再移出可访问性树，避免键盘焦点落入被裁剪区域 */
  visibility: hidden;
  transition: visibility 0s linear 0.24s;
}

.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 20px;
  border-bottom: 1px solid var(--hairline);
}

.sidebar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.app-kicker {
  margin: 0 0 4px;
  color: var(--primary);
  font-size: 12px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 1.5px;
}

.app-title {
  margin: 0;
  color: var(--ink);
  font-family: var(--font-serif);
  font-size: 22px;
  font-weight: 500;
  line-height: 1.2;
}

.lock-button {
  min-width: 64px;
  height: 36px;
  border: 1px solid var(--hairline);
  border-radius: 8px;
  background: var(--canvas);
  color: var(--muted);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.lock-button:hover:not(:disabled) {
  background: var(--surface-soft);
  color: var(--ink);
}

.lock-button:disabled {
  cursor: not-allowed;
  opacity: 0.58;
}

.collapse-button {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border: 1px solid var(--hairline);
  border-radius: 8px;
  background: var(--canvas);
  color: var(--muted);
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.collapse-button:hover {
  background: var(--surface-soft);
  color: var(--ink);
}

/* 收起态窄轨：展开按钮 + 锁定按钮，覆盖在滑出的侧栏之上 */
.sidebar-rail {
  position: absolute;
  top: 0;
  left: 0;
  bottom: 0;
  width: 44px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: space-between;
  padding: 16px 0;
  background: var(--canvas);
  z-index: 1;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.18s ease;
}

.sidebar-slot.is-collapsed .sidebar-rail {
  opacity: 1;
  pointer-events: auto;
}

/* 拖拽调宽手柄：hover 显示淡珊瑚竖条 */
.sidebar-resizer {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 8px;
  z-index: 5;
  cursor: col-resize;
  touch-action: none;
}

.sidebar-resizer:hover {
  background: var(--primary-soft);
}

.ai-resizer {
  position: fixed;
  top: 0;
  bottom: 0;
  left: calc(100vw - var(--ai-width, 420px) - 4px);
  width: 8px;
  z-index: 70;
  cursor: col-resize;
  touch-action: none;
}

.ai-resizer:hover {
  background: var(--primary-soft);
}

.rail-button {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border: 1px solid var(--hairline);
  border-radius: 999px;
  background: var(--canvas);
  color: var(--ink);
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.rail-button:hover:not(:disabled) {
  background: var(--surface-soft);
  color: var(--ink);
}

.rail-button:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.content {
  position: relative;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  /* AI 抽屉打开时正文让位左移，与固定抽屉宽度保持一致 */
  transition: margin-right 0.24s ease;
}

.content.is-ai-open {
  margin-right: min(var(--ai-width, 420px), 92vw);
}

.content-error {
  position: absolute;
  z-index: 2;
  top: 16px;
  right: 18px;
  max-width: min(420px, calc(100% - 36px));
  border: 1px solid var(--hairline);
  border-radius: 8px;
  padding: 10px 12px;
  background: var(--canvas);
  color: var(--error);
  box-shadow: var(--shadow);
}

.sidebar-footer {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 14px 16px;
  border-top: 1px solid var(--hairline);
  font-size: 12px;
}

.footer-label {
  color: var(--muted-soft);
}

.footer-author {
  color: var(--primary);
  font-weight: 500;
  text-decoration: underline;
  text-underline-offset: 2px;
}

.footer-author:hover {
  color: var(--primary-active);
}

@media (max-width: 820px) {
  .workspace {
    grid-template-columns: 1fr;
  }

  .sidebar-slot {
    width: 100%;
    border-right: 0;
    border-bottom: 1px solid var(--hairline);
  }

  .sidebar-slot.is-collapsed {
    width: 44px;
  }

  .sidebar {
    position: static;
    width: 100%;
  }

  /* 小屏下 AI 抽屉保持覆盖式，不让位 */
  .content.is-ai-open {
    margin-right: 0;
  }

  /* 小屏隐藏拖拽手柄 */
  .sidebar-resizer,
  .ai-resizer {
    display: none;
  }
}
</style>
