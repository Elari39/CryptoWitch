import { onMounted, onUnmounted } from 'vue'

const blockedKeys = new Set(['c', 'a', 's', 'p', 'u'])

function prevent(event: Event) {
  event.preventDefault()
  event.stopPropagation()
}

function onKeyDown(event: KeyboardEvent) {
  const key = event.key.toLowerCase()
  if (event.key === 'PrintScreen') {
    prevent(event)
    return
  }
  if ((event.ctrlKey || event.metaKey) && blockedKeys.has(key)) {
    prevent(event)
  }
}

export function useInteractionGuard() {
  onMounted(() => {
    document.addEventListener('contextmenu', prevent)
    document.addEventListener('copy', prevent)
    document.addEventListener('cut', prevent)
    document.addEventListener('dragstart', prevent)
    document.addEventListener('keydown', onKeyDown)
  })

  onUnmounted(() => {
    document.removeEventListener('contextmenu', prevent)
    document.removeEventListener('copy', prevent)
    document.removeEventListener('cut', prevent)
    document.removeEventListener('dragstart', prevent)
    document.removeEventListener('keydown', onKeyDown)
  })
}
