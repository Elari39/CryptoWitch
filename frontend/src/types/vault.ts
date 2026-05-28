import type { DocumentResponse, TreeNode } from '../../bindings/cryptowitch/internal/vault'

export type VaultTreeNode = TreeNode
export type VaultDocument = DocumentResponse

export interface ReadonlyVaultTreeNode {
  readonly id?: string
  readonly title: string
  readonly path: string
  readonly kind: string
  readonly documentType?: string
  readonly mimeType?: string
  readonly size?: number
  readonly children?: readonly ReadonlyVaultTreeNode[]
}

export interface ReadonlyVaultDocument {
  readonly id: string
  readonly title: string
  readonly documentType: string
  readonly mimeType?: string
  readonly size: number
  readonly html?: string
  readonly contentBase64?: string
  readonly chunked?: boolean
  readonly chunkSize?: number
  readonly chunkCount?: number
}

export interface PDFLoadState {
  readonly url: string
  readonly loadedChunks: number
  readonly totalChunks: number
  readonly loadedBytes: number
  readonly totalBytes: number
}
