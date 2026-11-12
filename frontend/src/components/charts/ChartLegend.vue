<script setup lang="ts">
import ChartSwatch from './ChartSwatch.vue'
import type { MarkerShape } from './chart-utils'

export interface LegendItem {
  label: string
  color: string
  kind: 'line' | 'box'
  dash?: string
  marker?: MarkerShape
}

defineProps<{ items: LegendItem[] }>()
</script>

<template>
  <ul v-if="items.length > 1" class="unstyled hstack flex-wrap gap-4">
    <li v-for="(item, i) in items" :key="i" class="hstack gap-2">
      <ChartSwatch :color="item.color" :kind="item.kind" :dash="item.dash" :marker="item.marker" />
      <small>{{ item.label }}</small>
    </li>
  </ul>
</template>
