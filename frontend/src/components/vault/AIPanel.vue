<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useAI } from '../../composables/useAI'

const ai = useAI()

const input = ref('')
const bodyRef = ref<HTMLElement | null>(null)
const contextExpanded = ref(false)

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
        <h2 class="ai-title">{{ ai.model.value || 'AI 助手' }}</h2>
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
        <p class="ai-empty-copy">请在 access.yaml 的 ai 段填写 endpoint / apiKey / model 后重新构建。</p>
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
          <p class="ai-message-role">{{ message.role === 'user' ? '我' : 'AI' }}</p>
          <div class="ai-message-content">{{ message.content }}</div>
        </div>
      </template>
      <p v-if="ai.error.value" class="ai-error" role="alert">{{ ai.error.value }}</p>
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
  width: min(420px, 92vw);
  height: 100vh;
  border-left: 1px solid var(--rule);
  background: var(--paper);
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
  border-bottom: 1px solid var(--rule);
}

.ai-kicker {
  margin: 0 0 4px;
  color: var(--accent);
  font-size: 11px;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.ai-title {
  margin: 0;
  color: var(--ink-strong);
  font-size: 18px;
  line-height: 1.2;
}

.ai-actions {
  display: flex;
  gap: 8px;
}

.ai-ghost-button {
  padding: 6px 10px;
  border: 1px solid var(--rule-strong);
  border-radius: 6px;
  background: var(--surface);
  color: var(--ink-muted);
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
}

.ai-ghost-button:hover:not(:disabled) {
  background: var(--surface-2);
  color: var(--ink);
}

.ai-ghost-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.ai-context {
  padding: 12px 20px;
  border-bottom: 1px solid var(--rule);
  background: var(--accent-wash);
}

.ai-context-toggle {
  margin-bottom: 6px;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--accent);
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
}

.ai-context-text {
  margin: 0;
  color: var(--ink-muted);
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.ai-body {
  overflow-y: auto;
  padding: 18px 20px;
  scrollbar-color: var(--rule-strong) var(--paper);
  scrollbar-width: thin;
}

.ai-empty {
  display: grid;
  place-content: center;
  height: 100%;
  text-align: center;
  color: var(--ink-muted);
}

.ai-empty-title {
  margin: 0 0 8px;
  color: var(--ink-strong);
  font-size: 16px;
  font-weight: 700;
}

.ai-empty-copy {
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
}

.ai-message {
  margin-bottom: 16px;
}

.ai-message-role {
  margin: 0 0 4px;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.ai-message.is-user .ai-message-role {
  color: var(--ink-faint);
}

.ai-message.is-assistant .ai-message-role {
  color: var(--accent);
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
  background: var(--accent-wash);
  color: var(--ink);
}

.ai-message.is-assistant .ai-message-content {
  background: var(--surface);
  color: var(--ink);
}

.ai-error {
  margin: 8px 0 0;
  padding: 10px 12px;
  border: 1px solid var(--accent);
  border-radius: 6px;
  background: var(--accent-wash);
  color: var(--accent-strong);
  font-size: 13px;
}

.ai-history {
  padding: 12px 20px;
  border-top: 1px solid var(--rule);
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}

.ai-history-title {
  margin: 0;
  width: 100%;
  color: var(--ink-faint);
  font-size: 11px;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.ai-history-item {
  max-width: 100%;
  padding: 4px 10px;
  border: 1px solid var(--rule-strong);
  border-radius: 999px;
  background: var(--surface);
  color: var(--ink-muted);
  font-size: 12px;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-history-item:hover {
  background: var(--surface-2);
  color: var(--ink);
}

.ai-footer {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
  padding: 14px 20px;
  border-top: 1px solid var(--rule);
}

.ai-input {
  resize: none;
  padding: 8px 10px;
  border: 1px solid var(--rule-strong);
  border-radius: 6px;
  background: var(--paper-warm);
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
  border-color: var(--accent);
}

.ai-input:disabled {
  opacity: 0.6;
}

.ai-send-button {
  align-self: stretch;
  padding: 0 16px;
  border: 1px solid var(--accent);
  border-radius: 6px;
  background: var(--accent);
  color: var(--paper);
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
}

.ai-send-button:hover:not(:disabled) {
  background: var(--accent-strong);
}

.ai-send-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
