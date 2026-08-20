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
const models = shallowRef<string[]>([])
const selectedModel = shallowRef('')
const messages = shallowRef<VaultAIMessage[]>([])
const histories = shallowRef<VaultAIHistory[]>([])
const streaming = shallowRef(false)
const partial = shallowRef('')
const selectedContext = shallowRef('')
const error = shallowRef('')
const currentDocumentId = shallowRef('')
const lastQuestion = shallowRef('')
const failedPartial = shallowRef(false)
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
    // 已累积的部分内容仍保留为一条助手消息，避免丢失；记录标记以便重试时移除。
    if (partial.value) {
      failedPartial.value = true
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
    models.value =
      info.models && info.models.length > 0 ? info.models : info.model ? [info.model] : []
    if (!selectedModel.value && models.value.length > 0) {
      selectedModel.value = models.value[0]
    }
  } catch {
    available.value = false
  }
}

function setModel(name: string) {
  if (models.value.includes(name)) {
    selectedModel.value = name
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
  lastQuestion.value = ''
  failedPartial.value = false
  requestId += 1 // 让进行中的流式回调失效
}

function loadHistory(index: number) {
  const history = histories.value[index]
  if (!history) {
    return
  }
  // 先把当前对话归档（若有），再恢复选中的历史。
  newConversation()
  // 被恢复的对话从历史列表移出（转入当前对话），避免下次「新对话」重复归档。
  histories.value = histories.value.filter((item) => item.id !== history.id)
  messages.value = [...history.messages]
  selectedContext.value = history.selectedText
  currentDocumentId.value = history.documentId
  // 恢复历史后，重试/重新生成以其中最近一条用户问题为准。
  for (let i = messages.value.length - 1; i >= 0; i--) {
    if (messages.value[i].role === 'user') {
      lastQuestion.value = messages.value[i].content
      break
    }
  }
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
  lastQuestion.value = ''
  failedPartial.value = false
  selectedModel.value = models.value[0] || ''
  requestId += 1
}

/**
 * 发送历史：若末尾用户消息与当前问题相同则去掉，
 * 避免同一条问题在 history 尾部与 question 参数中各出现一次。
 * ask() 已用 slice(0,-1) 保证不变式，retry/regenerate 复用本函数兜底。
 */
function historyWithoutTrailingQuestion(historyMessages: VaultAIMessage[], question: string): VaultAIMessage[] {
  const last = historyMessages[historyMessages.length - 1]
  if (last && last.role === 'user' && last.content === question) {
    return historyMessages.slice(0, -1)
  }
  return historyMessages
}

/**
 * 发起流式请求的核心：直接携带给定历史发送，不再追加用户消息。
 * ask / retry / regenerate 共用，保证模型选择与错误处理一致。
 */
function startStream(question: string, historyMessages: VaultAIMessage[]) {
  error.value = ''
  requestId += 1
  const currentRequestId = requestId
  streaming.value = true
  partial.value = ''

  const history = historyMessages.map((message) => ({
    role: message.role,
    content: message.content,
  }))

  VaultService.AIChat(
    new AIChatRequest({
      requestId: currentRequestId,
      documentId: currentDocumentId.value,
      selectedText: selectedContext.value,
      question,
      history,
      model: selectedModel.value,
    }),
  ).catch((caught) => {
    if (currentRequestId === requestId) {
      error.value = caught instanceof Error ? caught.message : 'AI 解读请求失败。'
      streaming.value = false
    }
  })
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

  lastQuestion.value = trimmed
  failedPartial.value = false
  const nextMessages: VaultAIMessage[] = [...messages.value, { role: 'user', content: trimmed }]
  messages.value = nextMessages
  startStream(trimmed, nextMessages.slice(0, -1))
}

/** 重试：重新发送最后一条问题（可先切换模型再重试）。 */
async function retry() {
  if (streaming.value || !lastQuestion.value) {
    return
  }
  if (!available.value) {
    await ensureInfo()
  }
  if (!available.value) {
    error.value = '未配置划词 AI 服务，无法解读。'
    return
  }
  // 失败时若已落成一条不完整的助手消息（部分内容），先移除再重发。
  if (failedPartial.value) {
    const last = messages.value[messages.value.length - 1]
    if (last && last.role === 'assistant') {
      messages.value = messages.value.slice(0, -1)
    }
    failedPartial.value = false
  }
  // 历史末尾若恰是当前问题（失败时未落助手消息），发送时去掉，避免重复发送。
  startStream(lastQuestion.value, historyWithoutTrailingQuestion(messages.value, lastQuestion.value))
}

/** 重新生成：覆盖最后一条助手回答（同一问题再问一次，不追加新的用户消息）。 */
async function regenerate() {
  if (streaming.value || error.value) {
    return
  }
  const last = messages.value[messages.value.length - 1]
  if (!last || last.role !== 'assistant') {
    return
  }
  if (!available.value) {
    await ensureInfo()
  }
  if (!available.value) {
    error.value = '未配置划词 AI 服务，无法解读。'
    return
  }
  // 回溯最近一条用户消息作为重发问题。
  let question = ''
  for (let i = messages.value.length - 1; i >= 0; i--) {
    if (messages.value[i].role === 'user') {
      question = messages.value[i].content
      break
    }
  }
  if (!question) {
    return
  }
  messages.value = messages.value.slice(0, -1) // 弹出被覆盖的助手回答
  // 弹出后末尾是最近一条用户问题，发送历史时去掉它，由 question 参数单独携带。
  startStream(question, historyWithoutTrailingQuestion(messages.value, question))
}

export function useAI() {
  bindListeners()
  return {
    open: readonly(open),
    available: readonly(available),
    model: readonly(model),
    models: readonly(models),
    selectedModel: readonly(selectedModel),
    messages: readonly(messages),
    histories: readonly(histories),
    streaming: readonly(streaming),
    partial: readonly(partial),
    selectedContext: readonly(selectedContext),
    error: readonly(error),
    lastQuestion: readonly(lastQuestion),
    openWithSelection,
    close,
    ask,
    retry,
    regenerate,
    setModel,
    newConversation,
    loadHistory,
    clearOnLock,
    ensureInfo,
  }
}
