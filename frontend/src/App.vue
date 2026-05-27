<script setup lang="ts">
import UnlockPanel from './components/vault/UnlockPanel.vue'
import VaultWorkspace from './components/vault/VaultWorkspace.vue'
import { useInteractionGuard } from './composables/useInteractionGuard'
import { useVault } from './composables/useVault'

useInteractionGuard()

const vault = useVault()
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
    :loading="vault.loading.value"
    :document-loading="vault.documentLoading.value"
    :error="vault.error.value"
    @lock="vault.lock"
    @select-document="vault.openDocument"
  />
</template>
