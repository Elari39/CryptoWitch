<script setup lang="ts">
interface Props {
  size?: number | string
}

withDefaults(defineProps<Props>(), { size: 48 })
</script>

<template>
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 64 64"
    :width="size"
    :height="size"
    role="img"
    aria-label="太极"
    focusable="false"
  >
    <circle cx="32" cy="32" r="30" fill="#1a1a1a" />
    <path d="M32,2 A30,30 0 0,1 32,62 A15,15 0 0,1 32,32 A15,15 0 0,0 32,2 Z" fill="#faf7f0" />
    <circle cx="32" cy="17" r="5" fill="#faf7f0" />
    <circle cx="32" cy="47" r="5" fill="#1a1a1a" />
  </svg>
</template>
