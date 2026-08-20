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
    try {
      const textarea = document.createElement('textarea')
      textarea.value = text
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.select()
      const ok = document.execCommand('copy')
      textarea.remove()
      return ok
    } catch {
      return false
    }
  }
}
