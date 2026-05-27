import type { DocumentResponse, TreeNode } from '../../bindings/cryptowitch/internal/vault'

export type VaultTreeNode = TreeNode
export type VaultDocument = DocumentResponse

export interface ReadonlyVaultTreeNode {
  readonly id?: string
  readonly title: string
  readonly path: string
  readonly kind: string
  readonly children?: readonly ReadonlyVaultTreeNode[]
}

export interface ReadonlyVaultDocument {
  readonly id: string
  readonly title: string
  readonly html: string
}
