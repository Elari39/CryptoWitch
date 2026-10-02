import { beforeEach, describe, expect, it, vi } from 'vitest'
import { deferred, flush } from '../test/helpers'

const backend = vi.hoisted(() => ({ Unlock: vi.fn(), Lock: vi.fn(), GetDocument: vi.fn(), GetPDFChunk: vi.fn(), clearOnLock: vi.fn() }))
vi.mock('../../bindings/cryptowitch/internal/vault/service', () => backend)
vi.mock('./useAI', () => ({ useAI: () => ({ clearOnLock: backend.clearOnLock }) }))
let vault: ReturnType<typeof import('./useVault')['useVault']>
const md = { id: 'md', title: 'Markdown', html: '<p>secret</p>', documentType: 'markdown', size: 10 }
const pdf = { id: 'pdf', title: 'PDF', documentType: 'pdf', size: 2, chunked: true, chunkCount: 2 }
beforeEach(async () => {
  vi.resetModules(); vi.clearAllMocks()
  backend.Unlock.mockResolvedValue({ tree: [] })
  backend.Lock.mockResolvedValue(undefined)
  URL.createObjectURL = vi.fn(() => 'blob:pdf-test')
  URL.revokeObjectURL = vi.fn()
  vault = (await import('./useVault')).useVault()
})

describe('文档库异步状态', () => {
  it.each(['resolve', 'reject'] as const)('锁定后忽略旧解锁 %s', async (result) => {
    const pending = deferred()
    backend.Unlock.mockReturnValueOnce(pending.promise)
    const unlocking = vault.unlock('test')
    await vault.lock()
    if (result === 'resolve') pending.resolve({ tree: [{ id: 'old' }] })
    else pending.reject(new Error('invalid password'))
    await unlocking
    expect(vault.unlocked.value).toBe(false)
    expect(vault.tree.value).toEqual([])
    expect(vault.error.value).toBe('')
  })

  it('锁定立即清空正文与 AI，旧锁定返回不覆盖新的解锁', async () => {
    await vault.unlock('test')
    backend.GetDocument.mockResolvedValue(md)
    await vault.openDocument('md')
    const pendingLock = deferred()
    backend.Lock.mockReturnValueOnce(pendingLock.promise)
    const locking = vault.lock()
    expect(vault.activeDocument.value).toBeNull()
    expect(vault.unlocked.value).toBe(false)
    expect(backend.clearOnLock).toHaveBeenCalledTimes(2)
    await vault.unlock('new')
    pendingLock.reject(new Error('old lock error')); await locking
    expect(vault.unlocked.value).toBe(true)
    expect(vault.error.value).toBe('')
  })

  it('旧解锁 finally 不清除新操作的 loading', async () => {
    const old = deferred(), current = deferred()
    backend.Unlock.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise)
    const first = vault.unlock('old'), second = vault.unlock('new')
    old.resolve({ tree: [] }); await first
    expect(vault.loading.value).toBe(true)
    current.resolve({ tree: [] }); await second
    expect(vault.loading.value).toBe(false)
  })

  it('切换立即清空正文，忽略旧文档响应和锁定后的响应', async () => {
    await vault.unlock('test')
    backend.GetDocument.mockResolvedValueOnce(md)
    await vault.openDocument('md')
    const old = deferred(), current = deferred()
    backend.GetDocument.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise)
    const first = vault.openDocument('old')
    expect(vault.activeDocument.value).toBeNull()
    const second = vault.openDocument('new')
    old.reject(new Error('old failure')); await first
    expect(vault.error.value).toBe('')
    expect(vault.documentLoading.value).toBe(true)
    await vault.lock()
    current.resolve(md); await second
    expect(vault.activeDocument.value).toBeNull()
  })

  it('PDF 展示分块进度并在锁定时释放 URL', async () => {
    await vault.unlock('test')
    backend.GetDocument.mockResolvedValue(pdf)
    const first = deferred(), second = deferred()
    backend.GetPDFChunk.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    const opening = vault.openDocument('pdf'); await flush()
    expect(vault.pdfLoad.value.status).toBe('loading')
    first.resolve({ contentBase64: 'YQ==' }); await flush()
    expect(vault.pdfProgress.value).toBe(50)
    second.resolve({ contentBase64: 'Yg==' }); await opening
    expect(vault.pdfLoad.value).toMatchObject({ status: 'ready', loadedBytes: 2, url: 'blob:pdf-test' })
    await vault.lock()
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:pdf-test')
    expect(vault.pdfLoad.value.status).toBe('idle')
  })

  it('PDF 分块失败退出加载状态；过期分块不得创建 URL', async () => {
    await vault.unlock('test')
    backend.GetDocument.mockResolvedValue(pdf)
    backend.GetPDFChunk.mockRejectedValueOnce(new Error('broken chunk'))
    await vault.openDocument('pdf')
    expect(vault.pdfLoad.value.status).toBe('error')
    expect(vault.documentLoading.value).toBe(false)
    const pending = deferred()
    backend.GetPDFChunk.mockReturnValueOnce(pending.promise)
    const opening = vault.openDocument('pdf'); await flush()
    await vault.lock()
    pending.resolve({ contentBase64: 'YQ==' }); await opening
    expect(URL.createObjectURL).not.toHaveBeenCalled()
    expect(vault.pdfLoad.value.status).toBe('idle')
  })
})
