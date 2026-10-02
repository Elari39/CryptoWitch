import { afterEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import DocumentViewer from './DocumentViewer.vue'
import { flush, mount } from '../../test/helpers'

// happy-dom 的 compatMode 固定为 BackCompat；此处验证调用与 DOM 生命周期，
// KaTeX 的实际输出由浏览器/生产构建检查。
vi.mock('katex/contrib/auto-render', () => ({ default: (body: HTMLElement) => {
  body.querySelectorAll('p').forEach((p) => { p.innerHTML = '<span class="katex">x</span>' })
} }))

let view: ReturnType<typeof mount> | undefined
afterEach(() => { view?.unmount(); view = undefined })
const state = () => reactive({
  document: { id: 'md', title: 'filename', html: '<h1>Heading</h1><p>$x$</p>', documentType: 'markdown', size: 10 },
  loading: false,
  pdfLoad: { status: 'idle', url: '', loadedChunks: 0, totalChunks: 0, loadedBytes: 0, totalBytes: 0 },
  pdfProgress: 0,
})

describe('阅读器重新挂载与 PDF 状态', () => {
  it('重复打开相同 Markdown 仍渲染公式并仅显示一个标题', async () => {
    const props = state()
    view = mount(DocumentViewer, props); await flush()
    expect(view.host.querySelectorAll('.katex')).toHaveLength(1)
    expect(view.host.querySelector('.document-title')?.textContent).toBe('Heading')
    props.loading = true; await flush()
    props.loading = false; await flush()
    expect(view.host.querySelectorAll('.katex')).toHaveLength(1)
    expect(view.host.querySelector('.markdown-body h1')).toBeNull()
    expect(view.host.querySelector('.document-title')?.textContent).toBe('Heading')
  })

  it('分块期间展示真实进度，失败后不再显示加载提示', async () => {
    const props = state()
    props.document.documentType = 'pdf'; props.loading = true
    props.pdfLoad = { status: 'loading', url: '', loadedChunks: 4, totalChunks: 10, loadedBytes: 4, totalBytes: 10 }
    props.pdfProgress = 40
    view = mount(DocumentViewer, props); await flush()
    expect(view.host.textContent).toContain('40%')
    expect(view.host.querySelector('progress')?.value).toBe(4)
    props.loading = false; props.pdfLoad.status = 'error'; await flush()
    expect(view.host.textContent).toContain('PDF 加载失败')
    expect(view.host.textContent).not.toContain('正在')
  })
})
