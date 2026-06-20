import { readonly, shallowRef } from 'vue'
import { Events } from '@wailsio/runtime'
import * as VaultService from '../../bindings/cryptowitch/internal/vault/service'
import { AIChatRequest } from '../../bindings/cryptowitch/internal/vault/models'
import type { VaultAIHistory, VaultAIMessage } from '../types/vault'

interface AIEventPayload {
  requestId: number
  data: string
}

// 模块级单例状态：保证全局只有一个 AI 会话上下文，无论 useAI() 被调用几次。
const open = shallowRef(false)
const available = shallowRef(false)
const model = shallowRef('')
const messages = shallowRef<VaultAIMessage[]>([])
const histories = shallowRef<VaultAIHistory[]>([])
const streaming = shallowRef(false)
const partial = shallowRef('')
const selectedContext = shallowRef('')
const error = shallowRef('')
const currentDocumentId = shallowRef('')
let requestId = 0
let historyCounter = 0
let listenersBound = false

function payloadOf(event: unknown): AIEventPayload {
  const data = (event as { data?: AIEventPayload })?.data
  return data ?? { requestId: -1, data: '' }
}

function bindListeners() {
  if (listenersBound) {
    return
  }
  listenersBound = true

  Events.On('ai:chunk', (event) => {
    const payload = payloadOf(event)
    if (payload.requestId !== requestId) {
      return
    }
    partial.value += payload.data
  })

  Events.On('ai:done', (event) => {
    const payload = payloadOf(event)
    if (payload.requestId !== requestId) {
      return
    }
    finalizeAssistant()
  })

  Events.On('ai:error', (event) => {
    const payload = payloadOf(event)
    if (payload.requestId !== requestId) {
      return
    }
    error.value = payload.data || 'AI 解读失败，请稍后重试。'
    // 已累积的部分内容仍保留为一条助手消息，避免丢失。
    if (partial.value) {
      finalizeAssistant()
    } else {
      streaming.value = false
    }
  })
}

function finalizeAssistant() {
  const content = partial.value
  partial.value = ''
  streaming.value = false
  if (content) {
    messages.value = [...messages.value, { role: 'assistant', content }]
  }
}

async function ensureInfo() {
  if (available.value || model.value) {
    return
  }
  try {
    const info = await VaultService.GetAIInfo()
    available.value = info.available
    model.value = info.model || ''
  } catch {
    available.value = false
  }
}

function openWithSelection(text: string, documentId: string) {
  selectedContext.value = text
  currentDocumentId.value = documentId
  open.value = true
  void ensureInfo()
}

function close() {
  open.value = false
}

function newConversation() {
  // 当前对话有内容则归档到历史。
  if (messages.value.length > 0) {
    historyCounter += 1
    const title =
      messages.value.find((message) => message.role === 'user')?.content.slice(0, 24) || '未命名对话'
    histories.value = [
      ...histories.value,
      {
        id: historyCounter,
        title,
        documentId: currentDocumentId.value,
        selectedText: selectedContext.value,
        messages: messages.value,
        createdAt: Date.now(),
      },
    ]
  }
  messages.value = []
  partial.value = ''
  error.value = ''
  streaming.value = false
  requestId += 1 // 让进行中的流式回调失效
}

function loadHistory(index: number) {
  const history = histories.value[index]
  if (!history) {
    return
  }
  // 先把当前对话归档（若有），再恢复选中的历史。
  newConversation()
  messages.value = [...history.messages]
  selectedContext.value = history.selectedText
  currentDocumentId.value = history.documentId
}

function clearOnLock() {
  messages.value = []
  histories.value = []
  partial.value = ''
  selectedContext.value = ''
  currentDocumentId.value = ''
  error.value = ''
  streaming.value = false
  open.value = false
  requestId += 1
}

async function ask(question: string) {
  const trimmed = question.trim()
  if (!trimmed || streaming.value) {
    return
  }
  if (!available.value) {
    await ensureInfo()
  }
  if (!available.value) {
    error.value = '未配置划词 AI 服务，无法解读。'
    return
  }

  error.value = ''
  requestId += 1
  const currentRequestId = requestId
  const nextMessages: VaultAIMessage[] = [...messages.value, { role: 'user', content: trimmed }]
  messages.value = nextMessages
  streaming.value = true
  partial.value = ''

  // 历史上下文：去掉最后一条（当前问题），其余作为 history 传入后端。
  const history = nextMessages.slice(0, -1).map((message) => ({ role: message.role, content: message.content }))

  try {
    await VaultService.AIChat(
      new AIChatRequest({
        requestId: currentRequestId,
        documentId: currentDocumentId.value,
        selectedText: selectedContext.value,
        question: trimmed,
        history,
      }),
    )
  } catch (caught) {
    if (currentRequestId === requestId) {
      error.value = caught instanceof Error ? caught.message : 'AI 解读请求失败。'
      streaming.value = false
    }
  }
}

export function useAI() {
  bindListeners()
  return {
    open: readonly(open),
    available: readonly(available),
    model: readonly(model),
    messages: readonly(messages),
    histories: readonly(histories),
    streaming: readonly(streaming),
    partial: readonly(partial),
    selectedContext: readonly(selectedContext),
    error: readonly(error),
    openWithSelection,
    close,
    ask,
    newConversation,
    loadHistory,
    clearOnLock,
    ensureInfo,
  }
}
