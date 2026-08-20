<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import renderMathInElement from 'katex/contrib/auto-render'
import { AI_MATH_DELIMITERS, renderAIMarkdown } from '../../lib/aiMarkdown'
import { copyText } from '../../lib/clipboard'

interface Props {
  content: string
  markdown?: boolean
}

const props = withDefaults(defineProps<Props>(), { markdown: false })

const el = ref<HTMLElement | null>(null)
const html = computed(() => (props.markdown ? renderAIMarkdown(props.content) : props.content))

let mathTimer: ReturnType<typeof setTimeout> | undefined

function renderMath() {
  if (!props.markdown || !el.value) {
    return
  }
  renderMathInElement(el.value, {
    delimiters: AI_MATH_DELIMITERS,
    ignoredTags: ['script', 'noscript', 'style', 'textarea', 'pre', 'code', 'option'],
    throwOnError: false,
  })
}

// 流式输出时内容高频变化：防抖后再跑 KaTeX，避免每个 chunk 全量渲染导致卡顿。
watch(
  () => props.content,
  () => {
    window.clearTimeout(mathTimer)
    mathTimer = window.setTimeout(() => void nextTick(renderMath), 90)
  },
)

onMounted(() => void nextTick(renderMath))
onBeforeUnmount(() => window.clearTimeout(mathTimer))

// 点击委托：命中「复制」按钮时，把同代码窗内 code 的文本复制到剪贴板。
async function copyCode(button: HTMLElement, code: string) {
  if (!code) {
    return
  }
  await copyText(code)
  button.textContent = '已复制'
  window.setTimeout(() => {
    button.textContent = '复制'
  }, 1600)
}

function onContentClick(event: MouseEvent) {
  const target = event.target as Element | null
  const button = target?.closest?.('.ai-copy-btn') as HTMLElement | null
  if (!button) {
    return
  }
  const code = button.closest('.ai-code-window')?.querySelector('code')?.textContent ?? ''
  void copyCode(button, code)
}
</script>

<template>
  <div
    v-if="markdown"
    ref="el"
    class="ai-message-content ai-message-markdown"
    v-html="html"
    @click="onContentClick"
  ></div>
  <div v-else class="ai-message-content">{{ content }}</div>
</template>
