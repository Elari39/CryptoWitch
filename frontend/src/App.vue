<script setup lang="ts">
import { watch } from 'vue'
import UnlockPanel from './components/vault/UnlockPanel.vue'
import VaultWorkspace from './components/vault/VaultWorkspace.vue'
import { useInteractionGuard } from './composables/useInteractionGuard'
import { useAI } from './composables/useAI'
import { useVault } from './composables/useVault'

useInteractionGuard()

const vault = useVault()
const ai = useAI()

// 锁定后清空 AI 会话内上下文与历史，避免残留文档片段。
watch(
  () => vault.unlocked.value,
  (unlocked) => {
    if (!unlocked) {
      ai.clearOnLock()
    }
  },
)
</script>

<template>
  <UnlockPanel
    v-if="!vault.unlocked.value"
    :loading="vault.loading.value"
    :error="vault.error.value"
    @unlock="vault.unlock"
  />
  <VaultWorkspace
    v-else
    :tree="vault.tree.value"
    :active-document="vault.activeDocument.value"
    :pdf-load="vault.pdfLoad.value"
    :pdf-progress="vault.pdfProgress.value"
    :loading="vault.loading.value"
    :document-loading="vault.documentLoading.value"
    :error="vault.error.value"
    @lock="vault.lock"
    @select-document="vault.openDocument"
  />
</template>
