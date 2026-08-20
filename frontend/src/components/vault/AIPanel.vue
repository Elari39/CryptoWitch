<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import AiMessageContent from './AiMessageContent.vue'
import { useAI } from '../../composables/useAI'
import { copyText } from '../../lib/clipboard'

const ai = useAI()

const input = ref('')
const bodyRef = ref<HTMLElement | null>(null)
const contextExpanded = ref(false)
const copiedIndex = ref(-1)

// 模型下拉：双向同步到全局 AI 状态（setModel 会校验模型属于配置列表）。
const selectedModel = computed({
  get: () => ai.selectedModel.value,
  set: (value: string) => ai.setModel(value),
})

const canRetry = computed(
  () => Boolean(ai.error.value && ai.lastQuestion.value && !ai.streaming.value),
)

// 最近一条助手消息（非流式、非错误态）时显示「重新生成」。
const canRegenerate = computed(() => {
  if (ai.streaming.value || ai.error.value || ai.messages.value.length === 0) {
    return false
  }
  return ai.messages.value[ai.messages.value.length - 1].role === 'assistant'
})

async function copyMessage(content: string, index: number) {
  await copyText(content)
  copiedIndex.value = index
  window.setTimeout(() => {
    if (copiedIndex.value === index) {
      copiedIndex.value = -1
    }
  }, 1600)
}

const visibleMessages = computed(() => {
  if (ai.partial.value) {
    return [...ai.messages.value, { role: 'assistant', content: ai.partial.value }]
  }
  return ai.messages.value
})

const contextPreview = computed(() => {
  const text = ai.selectedContext.value
  if (!text) {
    return ''
  }
  return text.length > 120 ? `${text.slice(0, 120)}…` : text
})

async function send() {
  const question = input.value
  if (!question.trim() || ai.streaming.value) {
    return
  }
  input.value = ''
  await ai.ask(question)
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    void send()
  }
}

async function scrollToBottom() {
  await nextTick()
  if (bodyRef.value) {
    bodyRef.value.scrollTop = bodyRef.value.scrollHeight
  }
}

watch(() => visibleMessages.value.length, () => void scrollToBottom())
watch(() => ai.partial.value, () => void scrollToBottom())
</script>

<template>
  <aside class="ai-panel" :class="{ 'is-open': ai.open.value }" aria-label="AI 解读">
    <header class="ai-header">
      <div class="ai-titles">
        <p class="ai-kicker">划词解读</p>
        <select
          v-if="ai.models.value.length > 1"
          v-model="selectedModel"
          class="ai-model-select"
          :disabled="ai.streaming.value"
          :title="'当前模型：' + selectedModel"
        >
          <option v-for="name in ai.models.value" :key="name" :value="name">{{ name }}</option>
        </select>
        <h2 v-else class="ai-title">{{ ai.selectedModel.value || ai.model.value || 'AI 助手' }}</h2>
      </div>
      <div class="ai-actions">
        <button
          class="ai-ghost-button"
          type="button"
          :disabled="ai.streaming.value"
          title="新对话"
          @click="ai.newConversation"
        >
          新对话
        </button>
        <button class="ai-ghost-button" type="button" title="关闭" @click="ai.close">关闭</button>
      </div>
    </header>

    <div v-if="contextPreview" class="ai-context">
      <button class="ai-context-toggle" type="button" @click="contextExpanded = !contextExpanded">
        {{ contextExpanded ? '收起' : '查看' }}划选片段
      </button>
      <p v-if="contextExpanded" class="ai-context-text">{{ ai.selectedContext.value }}</p>
      <p v-else class="ai-context-text">{{ contextPreview }}</p>
    </div>

    <div ref="bodyRef" class="ai-body">
      <div v-if="!ai.available.value && !ai.streaming.value" class="ai-empty">
        <p class="ai-empty-title">未配置划词 AI</p>
        <p class="ai-empty-copy">请在 access.yaml 的 ai 段填写 endpoint / apiKey / models 后重新构建。</p>
      </div>
      <div v-else-if="visibleMessages.length === 0" class="ai-empty">
        <p class="ai-empty-title">选中正文片段开始解读</p>
        <p class="ai-empty-copy">在 Markdown 文档中划选文字，点击「AI 解读」后将片段送入上下文，可多轮追问。</p>
      </div>
      <template v-else>
        <div
          v-for="(message, index) in visibleMessages"
          :key="index"
          class="ai-message"
          :class="message.role === 'user' ? 'is-user' : 'is-assistant'"
        >
          <div class="ai-message-head">
            <p class="ai-message-role">{{ message.role === 'user' ? '我' : 'AI' }}</p>
            <div v-if="message.role === 'assistant' && !ai.streaming.value" class="ai-message-actions">
              <button
                class="ai-message-action"
                type="button"
                :title="'复制回答（Markdown 源码）'"
                @click="copyMessage(message.content, index)"
              >
                {{ copiedIndex === index ? '已复制' : '复制' }}
              </button>
              <button
                v-if="canRegenerate && index === visibleMessages.length - 1"
                class="ai-message-action"
                type="button"
                title="同一问题重新生成，覆盖本条回答"
                @click="ai.regenerate"
              >
                重新生成
              </button>
            </div>
          </div>
          <AiMessageContent :content="message.content" :markdown="message.role === 'assistant'" />
        </div>
      </template>
      <div v-if="ai.error.value" class="ai-error-row">
        <p class="ai-error" role="alert">{{ ai.error.value }}</p>
        <button v-if="canRetry" class="ai-retry-button" type="button" @click="ai.retry">重试</button>
      </div>
    </div>

    <div v-if="ai.histories.value.length > 0" class="ai-history">
      <p class="ai-history-title">历史对话</p>
      <button
        v-for="(history, index) in ai.histories.value"
        :key="history.id"
        class="ai-history-item"
        type="button"
        :title="history.title"
        @click="ai.loadHistory(index)"
      >
        {{ history.title }}
      </button>
    </div>

    <footer class="ai-footer">
      <textarea
        v-model="input"
        class="ai-input"
        rows="2"
        placeholder="输入追问，回车发送（Shift+回车换行）"
        :disabled="ai.streaming.value || !ai.available.value"
        @keydown="onKeydown"
      ></textarea>
      <button
        class="ai-send-button"
        type="button"
        :disabled="ai.streaming.value || !input.trim() || !ai.available.value"
        @click="send"
      >
        {{ ai.streaming.value ? '解读中…' : '发送' }}
      </button>
    </footer>
  </aside>
</template>

<style scoped>
.ai-panel {
  position: fixed;
  top: 0;
  right: 0;
  display: grid;
  grid-template-rows: auto auto minmax(0, 1fr) auto auto;
  width: min(var(--ai-width, 420px), 92vw);
  height: 100vh;
  border-left: 1px solid var(--hairline);
  background: var(--canvas);
  box-shadow: var(--shadow);
  transform: translateX(100%);
  transition: transform 0.24s ease;
  z-index: 60;
}

.ai-panel.is-open {
  transform: translateX(0);
}

.ai-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 18px 20px;
  border-bottom: 1px solid var(--hairline);
}

.ai-kicker {
  margin: 0 0 4px;
  color: var(--primary);
  font-size: 11px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 1.5px;
}

.ai-title {
  margin: 0;
  color: var(--ink);
  font-family: var(--font-serif);
  font-size: 18px;
  font-weight: 500;
  line-height: 1.3;
}

.ai-model-select {
  max-width: 230px;
  padding: 5px 26px 5px 10px;
  border: 1px solid var(--hairline);
  border-radius: 8px;
  background: var(--canvas)
    url("data:image/svg+xml;charset=utf-8,%3Csvg xmlns='http://www.w3.org/2000/svg' width='10' height='6' viewBox='0 0 10 6'%3E%3Cpath d='M1 1l4 4 4-4' fill='none' stroke='%23cc785c' stroke-width='1.5' stroke-linecap='round' stroke-linejoin='round'/%3E%3C/svg%3E")
    no-repeat right 8px center;
  color: var(--ink);
  font-family: var(--font-mono);
  font-size: 12px;
  cursor: pointer;
  appearance: none;
  -webkit-appearance: none;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-model-select:focus {
  outline: none;
  border-color: var(--primary);
  box-shadow: 0 0 0 3px var(--primary-soft);
}

.ai-model-select:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.ai-actions {
  display: flex;
  gap: 8px;
}

.ai-ghost-button {
  padding: 6px 12px;
  border: 1px solid var(--hairline);
  border-radius: 8px;
  background: var(--canvas);
  color: var(--muted);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
}

.ai-ghost-button:hover:not(:disabled) {
  background: var(--surface-soft);
  color: var(--ink);
}

.ai-ghost-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.ai-context {
  padding: 12px 20px;
  border-bottom: 1px solid var(--hairline);
  background: var(--surface-soft);
}

.ai-context-toggle {
  margin-bottom: 6px;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--primary);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
}

.ai-context-toggle:hover {
  color: var(--primary-active);
}

.ai-context-text {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.ai-body {
  overflow-y: auto;
  padding: 18px 20px;
  scrollbar-color: var(--hairline) var(--canvas);
  scrollbar-width: thin;
}

.ai-empty {
  display: grid;
  place-content: center;
  height: 100%;
  text-align: center;
  color: var(--muted);
}

.ai-empty-title {
  margin: 0 0 8px;
  color: var(--ink);
  font-family: var(--font-serif);
  font-size: 17px;
  font-weight: 500;
}

.ai-empty-copy {
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
}

.ai-message {
  margin-bottom: 16px;
}

.ai-message-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 4px;
}

.ai-message-role {
  margin: 0;
  font-size: 11px;
  font-weight: 500;
  letter-spacing: 1px;
  text-transform: uppercase;
}

.ai-message.is-user .ai-message-role {
  color: var(--muted-soft);
}

.ai-message.is-assistant .ai-message-role {
  color: var(--primary);
}

/* 消息行操作（复制 / 重新生成）：悬停显示 */
.ai-message-actions {
  display: flex;
  gap: 6px;
  opacity: 0;
  transition: opacity 0.15s ease;
}

.ai-message:hover .ai-message-actions {
  opacity: 1;
}

.ai-message-action {
  padding: 2px 8px;
  border: 1px solid var(--hairline);
  border-radius: 6px;
  background: var(--canvas);
  color: var(--muted);
  font-size: 11px;
  cursor: pointer;
}

.ai-message-action:hover {
  border-color: var(--primary);
  color: var(--primary);
}

.ai-error-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-top: 8px;
}

.ai-retry-button {
  flex-shrink: 0;
  padding: 4px 12px;
  border: 1px solid var(--error);
  border-radius: 8px;
  background: var(--canvas);
  color: var(--error);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
}

.ai-retry-button:hover {
  background: var(--error-wash);
}

.ai-message-content {
  padding: 10px 12px;
  border-radius: 8px;
  font-size: 14px;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
}

.ai-message.is-user .ai-message-content {
  background: var(--primary-wash);
  color: var(--body);
}

.ai-message.is-assistant .ai-message-content {
  border: 1px solid var(--hairline);
  background: var(--canvas);
  color: var(--body);
}

/* ---- AI 回复的 Markdown + LaTeX 排版（与正文阅读器风格一致） ---- */

.ai-message-markdown {
  white-space: normal;
  /* 全局 * { user-select: none } 会阻断继承，这里强制放开，便于复制回答 */
  user-select: text;
  -webkit-user-select: text;
}

.ai-message-markdown :deep(*) {
  user-select: text;
  -webkit-user-select: text;
}

.ai-message-markdown :deep(p) {
  margin: 0 0 10px;
}

.ai-message-markdown :deep(p:last-child) {
  margin-bottom: 0;
}

.ai-message-markdown :deep(ul),
.ai-message-markdown :deep(ol) {
  margin: 0 0 10px;
  padding-left: 20px;
}

.ai-message-markdown :deep(li) {
  margin: 3px 0;
}

.ai-message-markdown :deep(li > p) {
  margin-bottom: 4px;
}

.ai-message-markdown :deep(h1),
.ai-message-markdown :deep(h2),
.ai-message-markdown :deep(h3),
.ai-message-markdown :deep(h4),
.ai-message-markdown :deep(h5),
.ai-message-markdown :deep(h6) {
  margin: 14px 0 6px;
  color: var(--ink);
  font-family: var(--font-serif);
  font-weight: 600;
  line-height: 1.35;
}

.ai-message-markdown :deep(h1) {
  font-size: 1.15em;
}

.ai-message-markdown :deep(h2) {
  font-size: 1.1em;
}

.ai-message-markdown :deep(h3) {
  font-size: 1.05em;
}

.ai-message-markdown :deep(h4),
.ai-message-markdown :deep(h5),
.ai-message-markdown :deep(h6) {
  font-size: 1em;
  color: var(--muted);
}

.ai-message-markdown :deep(hr) {
  height: 1px;
  margin: 14px 0;
  border: 0;
  background: linear-gradient(90deg, transparent, var(--hairline), transparent);
}

.ai-message-markdown :deep(a) {
  color: var(--primary);
  text-decoration: underline;
  text-underline-offset: 2px;
  word-break: break-all;
}

.ai-message-markdown :deep(a:hover) {
  color: var(--primary-active);
}

.ai-message-markdown :deep(strong) {
  color: var(--ink);
  font-weight: 600;
}

.ai-message-markdown :deep(blockquote) {
  margin: 10px 0;
  padding: 8px 12px;
  border-left: 3px solid var(--primary);
  border-radius: 0 8px 8px 0;
  background: var(--primary-wash);
  color: var(--muted);
  font-style: italic;
}

.ai-message-markdown :deep(blockquote p:last-child) {
  margin-bottom: 0;
}

.ai-message-markdown :deep(code) {
  border-radius: 4px;
  padding: 1px 5px;
  border: 1px solid var(--hairline-soft);
  background: var(--surface-soft);
  color: var(--body-strong);
  font-family: var(--font-mono);
  font-size: 0.88em;
}

.ai-message-markdown :deep(pre) {
  overflow-x: auto;
  margin: 10px 0;
  padding: 14px 16px;
  border-radius: 12px;
  background: var(--surface-dark);
  color: var(--on-dark);
  scrollbar-color: var(--surface-dark-elevated) var(--surface-dark);
  scrollbar-width: thin;
}

.ai-message-markdown :deep(pre code) {
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--on-dark);
  font-size: 12.5px;
  line-height: 1.65;
  white-space: pre;
}

/* ---- AI 代码窗：深色卡片 + 语言标识 + 一键复制 ---- */

.ai-message-markdown :deep(.ai-code-window) {
  margin: 10px 0;
  border: 1px solid var(--surface-dark-elevated);
  border-radius: 12px;
  overflow: hidden;
  background: var(--surface-dark);
}

.ai-message-markdown :deep(.ai-code-header) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  background: var(--surface-dark-elevated);
}

.ai-message-markdown :deep(.ai-code-lang) {
  color: var(--on-dark-soft);
  font-family: var(--font-mono);
  font-size: 11px;
  text-transform: lowercase;
  letter-spacing: 0.03em;
}

.ai-message-markdown :deep(.ai-copy-btn) {
  padding: 3px 10px;
  border: 1px solid rgba(250, 249, 245, 0.14);
  border-radius: 6px;
  background: var(--surface-dark);
  color: var(--on-dark-soft);
  font-size: 11px;
  font-weight: 500;
  cursor: pointer;
  user-select: none;
  transition: color 0.15s ease, border-color 0.15s ease;
}

.ai-message-markdown :deep(.ai-copy-btn:hover) {
  color: var(--on-dark);
  border-color: rgba(250, 249, 245, 0.3);
}

.ai-message-markdown :deep(.ai-code-window pre) {
  overflow-x: auto;
  margin: 0;
  padding: 14px 16px;
  border: 0;
  border-radius: 0;
  background: var(--surface-dark-soft);
  color: var(--on-dark);
  scrollbar-color: var(--surface-dark-elevated) var(--surface-dark-soft);
  scrollbar-width: thin;
}

.ai-message-markdown :deep(.ai-code-window pre code) {
  padding: 0;
  border: 0;
  background: transparent;
  color: #abb2bf;
  font-size: 12.5px;
  line-height: 1.65;
  white-space: pre;
}

/* hljs token 配色：与正文阅读器的 chroma onedark 调色板一致 */

.ai-message-markdown :deep(.hljs-comment),
.ai-message-markdown :deep(.hljs-quote) {
  color: #7f848e;
  font-style: italic;
}

.ai-message-markdown :deep(.hljs-keyword),
.ai-message-markdown :deep(.hljs-selector-tag),
.ai-message-markdown :deep(.hljs-type),
.ai-message-markdown :deep(.hljs-literal) {
  color: #c678dd;
}

.ai-message-markdown :deep(.hljs-string),
.ai-message-markdown :deep(.hljs-regexp),
.ai-message-markdown :deep(.hljs-addition) {
  color: #98c379;
}

.ai-message-markdown :deep(.hljs-number),
.ai-message-markdown :deep(.hljs-symbol),
.ai-message-markdown :deep(.hljs-bullet),
.ai-message-markdown :deep(.hljs-variable),
.ai-message-markdown :deep(.hljs-template-variable) {
  color: #d19a66;
}

.ai-message-markdown :deep(.hljs-title),
.ai-message-markdown :deep(.hljs-title.function_),
.ai-message-markdown :deep(.hljs-function .hljs-title) {
  color: #61afef;
}

.ai-message-markdown :deep(.hljs-attr),
.ai-message-markdown :deep(.hljs-attribute),
.ai-message-markdown :deep(.hljs-name),
.ai-message-markdown :deep(.hljs-property) {
  color: #e06c75;
}

.ai-message-markdown :deep(.hljs-built_in),
.ai-message-markdown :deep(.hljs-builtin-name),
.ai-message-markdown :deep(.hljs-selector-id),
.ai-message-markdown :deep(.hljs-selector-class) {
  color: #e5c07b;
}

.ai-message-markdown :deep(.hljs-meta),
.ai-message-markdown :deep(.hljs-doctag) {
  color: #7f848e;
}

.ai-message-markdown :deep(.hljs-deletion) {
  color: #e06c75;
}

.ai-message-markdown :deep(.hljs-emphasis) {
  font-style: italic;
}

.ai-message-markdown :deep(.hljs-strong) {
  font-weight: 600;
}

.ai-message-markdown :deep(table) {
  display: block;
  width: 100%;
  overflow-x: auto;
  margin: 10px 0;
  border-collapse: collapse;
  font-size: 0.9em;
}

.ai-message-markdown :deep(th),
.ai-message-markdown :deep(td) {
  padding: 6px 10px;
  border: 1px solid var(--hairline);
  text-align: left;
}

.ai-message-markdown :deep(th) {
  background: var(--surface-soft);
  color: var(--ink);
  font-weight: 600;
}

.ai-message-markdown :deep(.katex) {
  font-size: 1.05em;
}

.ai-message-markdown :deep(.katex-display) {
  overflow-x: auto;
  overflow-y: hidden;
  margin: 10px 0;
  padding: 2px;
}

.ai-message-markdown :deep(.katex-display > .katex) {
  white-space: nowrap;
}

.ai-error {
  margin: 0;
  padding: 10px 12px;
  border: 1px solid var(--error);
  border-radius: 8px;
  background: var(--error-wash);
  color: var(--error);
  font-size: 13px;
}

.ai-history {
  padding: 12px 20px;
  border-top: 1px solid var(--hairline);
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}

.ai-history-title {
  margin: 0;
  width: 100%;
  color: var(--muted-soft);
  font-size: 11px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 1px;
}

.ai-history-item {
  max-width: 100%;
  padding: 4px 12px;
  border: 1px solid var(--hairline);
  border-radius: 999px;
  background: var(--surface-card);
  color: var(--ink);
  font-size: 12px;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-history-item:hover {
  background: var(--surface-cream-strong);
}

.ai-footer {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
  padding: 14px 20px;
  border-top: 1px solid var(--hairline);
}

.ai-input {
  resize: none;
  padding: 8px 12px;
  border: 1px solid var(--hairline);
  border-radius: 8px;
  background: var(--canvas);
  color: var(--ink);
  font-size: 13px;
  line-height: 1.5;
  font-family: var(--font-ui);
  /* 全局 * { user-select: none } 会禁用输入框选中，这里放开保证可编辑 */
  user-select: text;
  -webkit-user-select: text;
}

.ai-input:focus {
  outline: none;
  border-color: var(--primary);
  box-shadow: 0 0 0 3px var(--primary-soft);
}

.ai-input:disabled {
  opacity: 0.6;
}

.ai-send-button {
  align-self: stretch;
  padding: 0 16px;
  border: 1px solid var(--primary);
  border-radius: 8px;
  background: var(--primary);
  color: var(--on-primary);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
}

.ai-send-button:hover:not(:disabled) {
  background: var(--primary-active);
  border-color: var(--primary-active);
}

.ai-send-button:disabled {
  background: var(--primary-disabled);
  border-color: var(--primary-disabled);
  color: var(--muted);
  cursor: not-allowed;
}
</style>
