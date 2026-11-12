<script setup lang="ts">
import { computed } from 'vue'
import ChartSwatch from './ChartSwatch.vue'
import type { MarkerShape } from './chart-utils'

export interface TooltipRow {
  label?: string
  value: string
  color?: string
  kind?: 'line' | 'box'
  dash?: string
  marker?: MarkerShape
}

const props = defineProps<{
  visible: boolean
  /** Anchor position as a percentage of the chart wrapper. */
  left: number
  top: number
  flip?: boolean
  title?: string
  rows: TooltipRow[]
}>()

// Offset 12px sideways so the box never covers the hovered mark.
const pos = computed(() => ({
  left: `${props.left}%`,
  top: `${props.top}%`,
  transform: props.flip ? 'translateX(calc(-100% - 12px))' : 'translateX(12px)',
}))
</script>

<template>
  <article v-if="visible" class="card chart-tooltip" aria-hidden="true" :style="pos">
    <strong v-if="title">{{ title }}</strong>
    <div v-for="(row, i) in rows" :key="i" class="hstack gap-2">
      <ChartSwatch v-if="row.color" :color="row.color" :kind="row.kind ?? 'box'" :dash="row.dash" :marker="row.marker" />
      <small v-if="row.label">{{ row.label }}</small>
      <strong>{{ row.value }}</strong>
    </div>
  </article>
</template>
