import { computed, readonly, shallowRef } from 'vue'
import * as VaultService from '../../bindings/cryptowitch/internal/vault/service'
import type { VaultDocument, VaultTreeNode } from '../types/vault'

function normalizeError(error: unknown): string {
  const message = error instanceof Error ? error.message : typeof error === 'string' ? error : ''

  if (message.includes('vault is locked')) {
    return '文档库已锁定，请重新解锁。'
  }
  if (message.includes('document not found')) {
    return '未找到这篇文档，请重新选择。'
  }
  if (message) {
    return message
  }
  return '操作失败，请稍后重试。'
}

export function useVault() {
  const unlocked = shallowRef(false)
  const tree = shallowRef<VaultTreeNode[]>([])
  const activeDocument = shallowRef<VaultDocument | null>(null)
  const loading = shallowRef(false)
  const documentLoading = shallowRef(false)
  const error = shallowRef('')
  let documentRequestID = 0

  const hasDocuments = computed(() => tree.value.length > 0)

  async function unlock(password: string) {
    documentRequestID += 1
    loading.value = true
    documentLoading.value = false
    error.value = ''
    try {
      const response = await VaultService.Unlock(password)
      tree.value = response.tree
      unlocked.value = true
      activeDocument.value = null
    } catch {
      unlocked.value = false
      tree.value = []
      activeDocument.value = null
      error.value = '密码不正确，无法解锁文档。'
    } finally {
      loading.value = false
    }
  }

  async function lock() {
    documentRequestID += 1
    loading.value = true
    documentLoading.value = false
    error.value = ''
    try {
      await VaultService.Lock()
    } finally {
      unlocked.value = false
      tree.value = []
      activeDocument.value = null
      loading.value = false
    }
  }

  async function openDocument(id: string) {
    const requestID = documentRequestID + 1
    documentRequestID = requestID
    documentLoading.value = true
    error.value = ''
    try {
      const document = await VaultService.GetDocument(id)
      if (requestID === documentRequestID) {
        activeDocument.value = document
      }
    } catch (caught) {
      if (requestID === documentRequestID) {
        error.value = normalizeError(caught)
      }
    } finally {
      if (requestID === documentRequestID) {
        documentLoading.value = false
      }
    }
  }

  return {
    unlocked: readonly(unlocked),
    tree: readonly(tree),
    activeDocument: readonly(activeDocument),
    loading: readonly(loading),
    documentLoading: readonly(documentLoading),
    error: readonly(error),
    hasDocuments,
    unlock,
    lock,
    openDocument,
  }
}
