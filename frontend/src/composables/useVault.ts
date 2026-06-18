import { computed, readonly, shallowRef } from 'vue'
import * as VaultService from '../../bindings/cryptowitch/internal/vault/service'
import type { PDFLoadState, VaultDocument, VaultTreeNode } from '../types/vault'

function normalizeError(error: unknown): string {
  const message = error instanceof Error ? error.message : typeof error === 'string' ? error : ''

  if (message.includes('vault is locked')) {
    return '文档库已锁定，请重新解锁。'
  }
  if (message.includes('document not found')) {
    return '未找到这篇文档，请重新选择。'
  }
  if (message.includes('invalid pdf chunk')) {
    return 'PDF 分块加载失败，请重新选择文档。'
  }
  if (message) {
    return message
  }
  return '操作失败，请稍后重试。'
}

function base64ToBytes(contentBase64: string) {
  const binary = window.atob(contentBase64)
  const bytes = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index)
  }
  return bytes
}

export function useVault() {
  const unlocked = shallowRef(false)
  const tree = shallowRef<VaultTreeNode[]>([])
  const activeDocument = shallowRef<VaultDocument | null>(null)
  const pdfLoad = shallowRef<PDFLoadState>({
    url: '',
    loadedChunks: 0,
    totalChunks: 0,
    loadedBytes: 0,
    totalBytes: 0,
  })
  const loading = shallowRef(false)
  const documentLoading = shallowRef(false)
  const error = shallowRef('')
  let documentRequestID = 0

  const hasDocuments = computed(() => tree.value.length > 0)
  const pdfProgress = computed(() => {
    if (pdfLoad.value.totalChunks <= 0) {
      return 0
    }
    return Math.round((pdfLoad.value.loadedChunks / pdfLoad.value.totalChunks) * 100)
  })

  function resetPDFLoad() {
    if (pdfLoad.value.url) {
      URL.revokeObjectURL(pdfLoad.value.url)
    }
    pdfLoad.value = {
      url: '',
      loadedChunks: 0,
      totalChunks: 0,
      loadedBytes: 0,
      totalBytes: 0,
    }
  }

  function setPDFURLFromBytes(parts: BlobPart[], mimeType: string, loadedBytes: number, totalChunks: number) {
    resetPDFLoad()
    pdfLoad.value = {
      url: URL.createObjectURL(new Blob(parts, { type: mimeType })),
      loadedChunks: totalChunks,
      totalChunks,
      loadedBytes,
      totalBytes: loadedBytes,
    }
  }

  async function loadChunkedPDF(document: VaultDocument, requestID: number) {
    const totalChunks = document.chunkCount ?? 0
    if (totalChunks <= 0) {
      throw new Error('invalid pdf chunk')
    }

    const parts: BlobPart[] = []
    let loadedBytes = 0
    pdfLoad.value = {
      url: '',
      loadedChunks: 0,
      totalChunks,
      loadedBytes: 0,
      totalBytes: document.size,
    }

    for (let index = 0; index < totalChunks; index += 1) {
      const chunk = await VaultService.GetPDFChunk(document.id, index)
      if (requestID !== documentRequestID) {
        return
      }
      const bytes = base64ToBytes(chunk.contentBase64)
      parts.push(bytes)
      loadedBytes += bytes.byteLength
      pdfLoad.value = {
        url: '',
        loadedChunks: index + 1,
        totalChunks,
        loadedBytes,
        totalBytes: document.size,
      }
    }

    if (requestID === documentRequestID) {
      setPDFURLFromBytes(parts, document.mimeType || 'application/pdf', loadedBytes, totalChunks)
    }
  }

  async function loadLegacyPDF(document: VaultDocument) {
    if (!document.contentBase64) {
      throw new Error('PDF 加载失败，请重新选择文档。')
    }
    const bytes = base64ToBytes(document.contentBase64)
    setPDFURLFromBytes([bytes], document.mimeType || 'application/pdf', bytes.byteLength, 1)
  }

  async function unlock(password: string) {
    documentRequestID += 1
    loading.value = true
    documentLoading.value = false
    error.value = ''
    resetPDFLoad()
    try {
      const response = await VaultService.Unlock(password)
      tree.value = response.tree
      unlocked.value = true
      activeDocument.value = null
    } catch (caught) {
      unlocked.value = false
      tree.value = []
      activeDocument.value = null
      const message = caught instanceof Error ? caught.message : ''
      error.value = message.includes('device not authorized')
        ? '本机未授权，无法查看文档。'
        : '密码不正确，无法解锁文档。'
    } finally {
      loading.value = false
    }
  }

  async function lock() {
    documentRequestID += 1
    loading.value = true
    documentLoading.value = false
    error.value = ''
    resetPDFLoad()
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
    resetPDFLoad()
    try {
      const document = await VaultService.GetDocument(id)
      if (requestID === documentRequestID) {
        activeDocument.value = document
      }
      if (requestID === documentRequestID && document.documentType === 'pdf') {
        if (document.chunked) {
          await loadChunkedPDF(document, requestID)
        } else {
          await loadLegacyPDF(document)
        }
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
    pdfLoad: readonly(pdfLoad),
    pdfProgress,
    loading: readonly(loading),
    documentLoading: readonly(documentLoading),
    error: readonly(error),
    hasDocuments,
    unlock,
    lock,
    openDocument,
    resetPDFLoad,
  }
}
