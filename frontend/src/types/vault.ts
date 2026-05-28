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
}
