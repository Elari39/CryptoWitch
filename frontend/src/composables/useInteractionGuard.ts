import { onMounted, onUnmounted } from 'vue'

const blockedKeys = new Set(['c', 'a', 's', 'p', 'u'])

function prevent(event: Event) {
  event.preventDefault()
  event.stopPropagation()
}

// 划词 AI 解读功能需要在 Markdown 正文内允许文本选中、复制与右键菜单，
// 因此对落在 .markdown-body 内的 copy / contextmenu / Ctrl+C 放行，其余区域保持拦截。
function isInsideMarkdownBody(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) {
    return false
  }
  return Boolean(target.closest('.markdown-body'))
}

function onContextMenu(event: Event) {
  if (isInsideMarkdownBody(event.target)) {
    return
  }
  prevent(event)
}

function onCopy(event: Event) {
  if (isInsideMarkdownBody(event.target)) {
    return
  }
  prevent(event)
}

function onKeyDown(event: KeyboardEvent) {
  const key = event.key.toLowerCase()
  if (event.key === 'PrintScreen') {
    prevent(event)
    return
  }
  if ((event.ctrlKey || event.metaKey) && blockedKeys.has(key)) {
    // 在 Markdown 正文中允许 Ctrl+C 复制划选内容。
    if (key === 'c' && isInsideMarkdownBody(event.target)) {
      return
    }
    prevent(event)
  }
}

export function useInteractionGuard() {
  onMounted(() => {
    document.addEventListener('contextmenu', onContextMenu)
    document.addEventListener('copy', onCopy)
    document.addEventListener('cut', prevent)
    document.addEventListener('dragstart', prevent)
    document.addEventListener('keydown', onKeyDown)
  })

  onUnmounted(() => {
    document.removeEventListener('contextmenu', onContextMenu)
    document.removeEventListener('copy', onCopy)
    document.removeEventListener('cut', prevent)
    document.removeEventListener('dragstart', prevent)
    document.removeEventListener('keydown', onKeyDown)
  })
}
