<script setup lang="ts">
import { computed } from 'vue'
import type { MarkerShape } from './chart-utils'

const props = withDefaults(defineProps<{ shape: MarkerShape; x: number; y: number; size?: number; color: string }>(), { size: 10 })

const h = computed(() => props.size / 2)
const d = computed(() => {
  const { x, y } = props
  const r = h.value
  switch (props.shape) {
    case 'square': return `M${x - r} ${y - r}H${x + r}V${y + r}H${x - r}Z`
    case 'triangle': return `M${x} ${y - r}L${x + r} ${y + r}H${x - r}Z`
    case 'diamond': return `M${x} ${y - r}L${x + r} ${y}L${x} ${y + r}L${x - r} ${y}Z`
    case 'cross': return `M${x - r} ${y - r}L${x + r} ${y + r}M${x - r} ${y + r}L${x + r} ${y - r}`
    default: return `M${x - r} ${y}A${r} ${r} 0 1 0 ${x + r} ${y}A${r} ${r} 0 1 0 ${x - r} ${y}Z`
  }
})
</script>

<template>
  <path
    v-if="shape === 'cross'"
    :d="d"
    fill="none"
    :stroke="color"
    stroke-width="2.5"
    stroke-linecap="round"
  />
  <path v-else :d="d" :fill="color" stroke="var(--background)" stroke-width="1" />
</template>
