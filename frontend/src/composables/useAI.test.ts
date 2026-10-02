import { beforeEach, describe, expect, it, vi } from 'vitest'
import { deferred, flush } from '../test/helpers'

const mocks = vi.hoisted(() => ({
  GetAIInfo: vi.fn(), AIChat: vi.fn(), CancelAIChat: vi.fn(),
  handlers: {} as Record<string, (event: unknown) => void>,
}))
vi.mock('../../bindings/cryptowitch/internal/vault/service', () => mocks)
vi.mock('@wailsio/runtime', () => ({
  Create: { Array: (create: (value: unknown) => unknown) => (items: unknown[]) => items.map(create), Any: (value: unknown) => value },
  Events: { On: (name: string, fn: (event: unknown) => void) => { mocks.handlers[name] = fn } },
}))

let ai: ReturnType<typeof import('./useAI')['useAI']>
const info = { available: true, model: 'test', models: ['test'] }
beforeEach(async () => {
  vi.resetModules()
  vi.clearAllMocks()
  mocks.GetAIInfo.mockResolvedValue(info)
  mocks.AIChat.mockResolvedValue(undefined)
  mocks.CancelAIChat.mockResolvedValue(undefined)
  ai = (await import('./useAI')).useAI()
})

async function start() {
  ai.openWithSelection('片段 A', 'doc-a')
  await ai.ensureInfo()
  await ai.ask('问题 A')
  await flush()
  return mocks.AIChat.mock.calls.at(-1)![0].requestId as number
}
function emit(name: string, id: number, data = '') { mocks.handlers[name]({ data: { requestId: id, data } }) }

describe('AI 会话生命周期', () => {
  it('新片段须确认，取消保留旧上下文，确认后归档且不混入旧消息', async () => {
    const id = await start()
    emit('ai:chunk', id, '回答 A'); emit('ai:done', id)
    ai.openWithSelection('片段 B', 'doc-b')
    expect(ai.pendingSelection.value?.documentId).toBe('doc-b')
    expect(ai.selectedContext.value).toBe('片段 A')
    ai.cancelSelection()
    expect(ai.messages.value).toHaveLength(2)
    ai.openWithSelection('片段 B', 'doc-b')
    ai.confirmSelection()
    expect(ai.histories.value[0].selectedText).toBe('片段 A')
    expect(ai.messages.value).toHaveLength(0)
    await ai.ask('问题 B'); await flush()
    expect(mocks.AIChat.mock.calls.at(-1)![0]).toMatchObject({ documentId: 'doc-b', selectedText: '片段 B', history: [] })
  })

  it('同片段继续追问，无需切换确认', async () => {
    const id = await start()
    emit('ai:done', id)
    ai.openWithSelection(' 片段 A ', 'doc-a')
    expect(ai.pendingSelection.value).toBeNull()
    expect(ai.messages.value).toHaveLength(1)
  })

  it('确认切换停止流，丢弃部分输出与旧事件，仅归档已完成消息', async () => {
    const id = await start()
    emit('ai:chunk', id, '未完成')
    ai.openWithSelection('片段 B', 'doc-b')
    ai.confirmSelection()
    await flush()
    expect(mocks.CancelAIChat).toHaveBeenCalledWith(id)
    expect(ai.histories.value[0].messages).toHaveLength(1)
    emit('ai:chunk', id, '过期'); emit('ai:error', id, '过期错误')
    expect(ai.partial.value).toBe('')
    expect(ai.error.value).toBe('')
  })

  it('取消先于登记返回时，等待取消完成后才登记新请求', async () => {
    const registration = deferred()
    const cancellation = deferred()
    mocks.AIChat.mockImplementationOnce(() => registration.promise)
    mocks.CancelAIChat.mockImplementationOnce(() => cancellation.promise)
    const id = await start()
    ai.openWithSelection('片段 B', 'doc-b'); ai.confirmSelection()
    await ai.ask('问题 B'); await flush()
    expect(mocks.AIChat).toHaveBeenCalledTimes(1)
    registration.resolve(undefined); await flush()
    expect(mocks.CancelAIChat).toHaveBeenCalledWith(id)
    expect(mocks.AIChat).toHaveBeenCalledTimes(1)
    cancellation.resolve(undefined); await flush()
    expect(mocks.AIChat).toHaveBeenCalledTimes(2)
    expect(mocks.AIChat.mock.calls[1][0].documentId).toBe('doc-b')
  })

  it('锁定清除待确认状态，配置和等待中的提问返回后也不能恢复状态', async () => {
    const pendingInfo = deferred()
    mocks.GetAIInfo.mockReturnValue(pendingInfo.promise)
    ai.openWithSelection('片段 A', 'doc-a')
    const asking = ai.ask('不应发送')
    ai.clearOnLock()
    pendingInfo.resolve(info); await asking; await flush()
    expect(mocks.AIChat).not.toHaveBeenCalled()
    expect(ai.available.value).toBe(false)
    expect(ai.messages.value).toEqual([])
    expect(ai.open.value).toBe(false)
    expect(ai.selectedContext.value).toBe('')
  })

  it('配置等待期间更换片段不会发送旧问题；重复提问只登记一次', async () => {
    const oldInfo = deferred()
    mocks.GetAIInfo.mockReturnValueOnce(oldInfo.promise)
    ai.openWithSelection('片段 A', 'doc-a')
    const oldAsk = ai.ask('旧问题')
    ai.openWithSelection('片段 B', 'doc-b')
    await ai.ensureInfo()
    oldInfo.resolve(info); await oldAsk
    expect(mocks.AIChat).not.toHaveBeenCalled()
    await Promise.all([ai.ask('新问题'), ai.ask('重复问题')]); await flush()
    expect(mocks.AIChat).toHaveBeenCalledTimes(1)
  })

  it('恢复历史取消当前流，锁定后忽略所有旧事件', async () => {
    const first = await start()
    emit('ai:chunk', first, '回答'); emit('ai:done', first)
    ai.newConversation()
    await ai.ask('另一个问题'); await flush()
    const current = mocks.AIChat.mock.calls.at(-1)![0].requestId
    ai.loadHistory(0); await flush()
    expect(mocks.CancelAIChat).toHaveBeenCalledWith(current)
    expect(ai.messages.value[0].content).toBe('问题 A')
    ai.openWithSelection('B', 'b'); ai.clearOnLock()
    emit('ai:chunk', current, '过期'); emit('ai:done', current)
    expect(ai.pendingSelection.value).toBeNull()
    expect(ai.histories.value).toEqual([])
    expect(ai.messages.value).toEqual([])
  })

  it('断流保留部分回答，重试移除部分回答且不重复问题，完成后忽略晚到事件', async () => {
    const id = await start()
    emit('ai:chunk', id, '部分回答'); emit('ai:error', id, '响应不完整')
    expect(ai.messages.value.at(-1)?.content).toBe('部分回答')
    await ai.retry(); await flush()
    const request = mocks.AIChat.mock.calls.at(-1)![0]
    expect(request.history).toEqual([])
    expect(request.question).toBe('问题 A')
    emit('ai:chunk', request.requestId, '完整回答'); emit('ai:done', request.requestId)
    emit('ai:chunk', request.requestId, '晚到内容')
    expect(ai.partial.value).toBe('')
    await ai.regenerate(); await flush()
    expect(mocks.AIChat.mock.calls.at(-1)![0].history).toEqual([])
    expect(ai.messages.value).toHaveLength(1)
  })
})
