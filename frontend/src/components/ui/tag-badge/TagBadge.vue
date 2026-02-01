<script setup lang="ts">
import { getTagColorClass } from '@/lib/constants'

const props = defineProps<{
  color?: string
}>()
</script>

<template>
  <span :class="['inline-flex items-center rounded-full border border-transparent px-2.5 py-0.5 text-xs font-medium', getTagColorClass(props.color || 'gray')]">
    <slot />
  </span>
</template>
