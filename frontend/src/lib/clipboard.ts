// WeakSet 身份检查只信任应用创建的临时控件，文档 HTML 不能伪造标记绕过防护。
const copyTargets = new WeakSet<EventTarget>()

export function isAppCopyTarget(target: EventTarget | null): boolean {
  return target !== null && copyTargets.has(target)
}

/**
 * 复制文本到剪贴板：优先 Clipboard API，失败时回退到隐藏 textarea + execCommand。
 * 返回是否复制成功。
 */
export async function copyText(text: string): Promise<boolean> {
  if (!text) {
    return false
  }
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    // 剪贴板 API 不可用时回退（如非安全上下文或无用户手势授权）。
    const textarea = document.createElement('textarea')
    const previousFocus = document.activeElement
    const selection = window.getSelection()
    const ranges = selection ? Array.from({ length: selection.rangeCount }, (_, i) => selection.getRangeAt(i).cloneRange()) : []
    const inputSelection = previousFocus instanceof HTMLInputElement || previousFocus instanceof HTMLTextAreaElement
      ? [previousFocus.selectionStart, previousFocus.selectionEnd, previousFocus.selectionDirection] as const
      : null
    try {
      textarea.value = text
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      copyTargets.add(textarea)
      textarea.focus({ preventScroll: true })
      textarea.select()
      return document.execCommand('copy')
    } catch {
      return false
    } finally {
      copyTargets.delete(textarea)
      textarea.remove()
      if (previousFocus instanceof HTMLElement && previousFocus.isConnected) {
        previousFocus.focus({ preventScroll: true })
      }
      if (inputSelection && inputSelection[0] !== null && inputSelection[1] !== null) {
        ;(previousFocus as HTMLInputElement | HTMLTextAreaElement).setSelectionRange(
          inputSelection[0], inputSelection[1], inputSelection[2] || undefined,
        )
      } else if (selection) {
        selection.removeAllRanges()
        ranges.forEach((range) => selection.addRange(range))
      }
    }
  }
}
