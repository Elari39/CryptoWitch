<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import TaijiIcon from '../icons/TaijiIcon.vue'

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
      <TaijiIcon class="brand-mark" :size="48" aria-hidden="true" />
      <p class="eyebrow">Encrypted Markdown Vault</p>
      <h1 id="unlock-title" class="title">CryptoWitch</h1>
      <p class="subtitle">输入密码后才会加载文档目录和内容，且需在本机授权设备上使用。</p>

      <p class="author-line">
        由
        <a
          class="author-link"
          href="https://github.com/Elari39/CryptoWitch"
          target="_blank"
          rel="noopener noreferrer"
          title="Elari39 · GitHub"
        >Elari39</a>
        制作
      </p>

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
  background: linear-gradient(160deg, var(--paper), var(--paper-warm));
}

.unlock-panel {
  width: min(420px, 100%);
  padding: 32px;
  border: 1px solid var(--rule);
  border-radius: 8px;
  background: var(--surface);
  box-shadow: var(--shadow);
}

.brand-mark {
  display: block;
  width: 48px;
  height: 48px;
  margin-bottom: 20px;
  border-radius: 8px;
}

.eyebrow {
  margin: 0 0 8px;
  color: var(--accent);
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.title {
  margin: 0;
  color: var(--ink-strong);
  font-size: 34px;
  line-height: 1.15;
}

.subtitle {
  margin: 12px 0 12px;
  color: var(--ink-muted);
}

.author-line {
  margin: 0 0 28px;
  color: var(--ink-faint);
  font-size: 13px;
}

.author-link {
  color: var(--accent);
  font-weight: 700;
  text-decoration: none;
  border-bottom: 1px dashed var(--accent);
  padding-bottom: 1px;
}

.author-link:hover {
  color: var(--accent-strong);
  border-bottom-color: var(--accent-strong);
}

.unlock-form {
  display: grid;
  gap: 12px;
}

.field-label {
  color: var(--ink);
  font-size: 14px;
  font-weight: 600;
}

.password-input {
  width: 100%;
  height: 44px;
  box-sizing: border-box;
  border: 1px solid var(--rule-strong);
  border-radius: 6px;
  padding: 0 14px;
  outline: none;
  background: var(--surface);
  color: var(--ink);
  font-size: 15px;
}

.password-input:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.unlock-button {
  height: 44px;
  border: 0;
  border-radius: 6px;
  background: var(--accent);
  color: var(--surface);
  font-weight: 800;
  cursor: pointer;
  transition: background 0.15s ease;
}

.unlock-button:hover:not(:disabled) {
  background: var(--accent-strong);
}

.unlock-button:disabled {
  cursor: not-allowed;
  opacity: 0.58;
}

.error-message {
  margin: 16px 0 0;
  padding: 10px 12px;
  border-radius: 6px;
  background: var(--accent-wash);
  color: var(--accent-strong);
  font-size: 14px;
}
</style>
