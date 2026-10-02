import { afterEach, describe, expect, it, vi } from 'vitest'
import { h } from 'vue'
import { copyText } from './clipboard'
import { useInteractionGuard } from '../composables/useInteractionGuard'
import { mount } from '../test/helpers'

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); document.body.innerHTML = '' })

describe('复制回退与交互防护', () => {
  it('Clipboard API 成功时不创建临时控件', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', { clipboard: { writeText } })
    expect(await copyText('text')).toBe(true)
    expect(writeText).toHaveBeenCalledWith('text')
    expect(document.querySelector('textarea')).toBeNull()
  })

  it.each([false, true])('仅放行临时复制控件，成功或异常均恢复焦点并清理（异常：%s）', async (throws) => {
    vi.stubGlobal('navigator', { clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) } })
    const view = mount({ setup() { useInteractionGuard(); return () => h('input', { value: 'original' }) } })
    try {
      const input = view.host.querySelector('input')!
      input.focus(); input.setSelectionRange(1, 4)
      const normalCopy = new Event('copy', { bubbles: true, cancelable: true })
      input.dispatchEvent(normalCopy)
      expect(normalCopy.defaultPrevented).toBe(true)
      document.execCommand = vi.fn(() => {
        const temporary = document.querySelector('textarea')!
        const event = new Event('copy', { bubbles: true, cancelable: true })
        temporary.dispatchEvent(event)
        expect(event.defaultPrevented).toBe(false)
        if (throws) throw new Error('copy failed')
        return true
      })
      expect(await copyText('answer')).toBe(!throws)
      expect(document.querySelector('textarea')).toBeNull()
      expect(document.activeElement).toBe(input)
      expect([input.selectionStart, input.selectionEnd]).toEqual([1, 4])
    } finally { view.unmount() }
  })
})
