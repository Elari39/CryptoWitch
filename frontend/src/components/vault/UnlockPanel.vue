<script setup lang="ts">
import { computed, shallowRef } from 'vue'

interface Props {
  loading: boolean
  error: string
}

interface Emits {
  unlock: [password: string]
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const password = shallowRef('')
const passwordInput = shallowRef<HTMLInputElement | null>(null)
const canSubmit = computed(() => password.value.trim().length > 0 && !props.loading)

function submit() {
  if (!canSubmit.value) {
    passwordInput.value?.focus()
    return
  }
  emit('unlock', password.value)
}
</script>

<template>
  <main class="unlock-shell">
    <section class="unlock-panel" aria-labelledby="unlock-title">
      <div class="brand-mark" aria-hidden="true">CW</div>
      <p class="eyebrow">Encrypted Markdown Vault</p>
      <h1 id="unlock-title" class="title">CryptoWitch</h1>
      <p class="subtitle">输入密码后才会加载文档目录和内容。</p>

      <form class="unlock-form" @submit.prevent="submit">
        <label class="field-label" for="vault-password">访问密码</label>
        <input
          id="vault-password"
          ref="passwordInput"
          v-model="password"
          class="password-input"
          type="password"
          autocomplete="off"
          spellcheck="false"
          :disabled="loading"
        />
        <button class="unlock-button" type="submit" :disabled="!canSubmit">
          {{ loading ? '解锁中' : '解锁文档' }}
        </button>
      </form>

      <p v-if="error" class="error-message" role="alert">{{ error }}</p>
    </section>
  </main>
</template>

<style scoped>
.unlock-shell {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 32px;
  background: #0e1218;
}

.unlock-panel {
  width: min(420px, 100%);
  padding: 32px;
  border: 1px solid #253042;
  border-radius: 8px;
  background: #151b24;
  box-shadow: 0 20px 70px rgba(0, 0, 0, 0.35);
}

.brand-mark {
  display: grid;
  place-items: center;
  width: 48px;
  height: 48px;
  margin-bottom: 20px;
  border-radius: 8px;
  background: #c7a76c;
  color: #11151c;
  font-weight: 800;
}

.eyebrow {
  margin: 0 0 8px;
  color: #85c7bc;
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
}

.title {
  margin: 0;
  color: #f5f0e8;
  font-size: 34px;
  line-height: 1.15;
}

.subtitle {
  margin: 12px 0 28px;
  color: #a8b3c3;
}

.unlock-form {
  display: grid;
  gap: 12px;
}

.field-label {
  color: #d7deea;
  font-size: 14px;
  font-weight: 600;
}

.password-input {
  width: 100%;
  height: 44px;
  box-sizing: border-box;
  border: 1px solid #34445b;
  border-radius: 6px;
  padding: 0 14px;
  outline: none;
  background: #0f141b;
  color: #f4f7fb;
  font-size: 15px;
}

.password-input:focus {
  border-color: #85c7bc;
  box-shadow: 0 0 0 3px rgba(133, 199, 188, 0.16);
}

.unlock-button {
  height: 44px;
  border: 0;
  border-radius: 6px;
  background: #85c7bc;
  color: #08100f;
  font-weight: 800;
  cursor: pointer;
}

.unlock-button:disabled {
  cursor: not-allowed;
  opacity: 0.58;
}

.error-message {
  margin: 16px 0 0;
  color: #ffb0a6;
  font-size: 14px;
}
</style>
