import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import AIPanel from './AIPanel.vue'
import AiMessageContent from './AiMessageContent.vue'
import { flush, mount } from '../../test/helpers'

const mocks = vi.hoisted(() => ({ copyText: vi.fn(), ai: null as any }))
vi.mock('../../lib/clipboard', () => ({ copyText: mocks.copyText }))
vi.mock('../../composables/useAI', () => ({ useAI: () => mocks.ai }))
vi.mock('katex/contrib/auto-render', () => ({ default: vi.fn() }))
let view: ReturnType<typeof mount> | undefined

beforeEach(() => {
  vi.clearAllMocks()
  mocks.ai = {
    open: ref(true), available: ref(true), models: ref(['test']), model: ref('test'), selectedModel: ref('test'),
    streaming: ref(false), error: ref(''), lastQuestion: ref('question'), selectedContext: ref('old selection'),
    partial: ref(''), messages: ref([{ role: 'assistant', content: 'answer' }]), histories: ref([]), pendingSelection: ref(null),
    setModel: vi.fn(), ask: vi.fn(), retry: vi.fn(), regenerate: vi.fn(), newConversation: vi.fn(), loadHistory: vi.fn(), close: vi.fn(),
    cancelSelection: vi.fn(() => { mocks.ai.pendingSelection.value = null }),
    confirmSelection: vi.fn(() => { mocks.ai.pendingSelection.value = null }),
  }
})
afterEach(() => { view?.unmount(); view = undefined })

describe('AI 切换与复制反馈', () => {
  it('切换确认框的保留、新对话与 Escape 路径', async () => {
    view = mount(AIPanel); await flush()
    const dialog = view.host.querySelector('dialog')!
    mocks.ai.pendingSelection.value = { text: 'new', documentId: 'new-doc' }; await flush()
    expect(dialog.open).toBe(true)
    dialog.querySelectorAll('button')[0].click(); await flush()
    expect(mocks.ai.cancelSelection).toHaveBeenCalledTimes(1)
    expect(dialog.open).toBe(false)
    mocks.ai.pendingSelection.value = { text: 'new', documentId: 'new-doc' }; await flush()
    dialog.querySelectorAll('button')[1].click(); await flush()
    expect(mocks.ai.confirmSelection).toHaveBeenCalledTimes(1)
    mocks.ai.pendingSelection.value = { text: 'new', documentId: 'new-doc' }; await flush()
    dialog.dispatchEvent(new Event('cancel')); await flush()
    expect(mocks.ai.cancelSelection).toHaveBeenCalledTimes(2)
    expect(dialog.open).toBe(false)
  })

  it.each([true, false])('回答复制按真实返回结果显示反馈：%s', async (ok) => {
    mocks.copyText.mockResolvedValue(ok)
    view = mount(AIPanel); await flush()
    const button = view.host.querySelector<HTMLButtonElement>('button[title="复制回答（Markdown 源码）"]')!
    button.click(); await flush()
    expect(button.textContent).toBe(ok ? '已复制' : '复制失败')
    expect(mocks.copyText).toHaveBeenCalledWith('answer')
  })

  it.each([true, false])('代码复制按真实返回结果显示反馈：%s', async (ok) => {
    mocks.copyText.mockResolvedValue(ok)
    view = mount(AiMessageContent, { markdown: true, content: '```js\nconst answer = 42\n```' })
    await flush()
    const button = view.host.querySelector<HTMLElement>('.ai-copy-btn')!
    button.click(); await flush()
    expect(button.textContent).toBe(ok ? '已复制' : '复制失败')
    expect(mocks.copyText).toHaveBeenCalledWith('const answer = 42')
  })
})
