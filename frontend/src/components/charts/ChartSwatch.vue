<script setup lang="ts">
import ChartMarker from './ChartMarker.vue'
import type { MarkerShape } from './chart-utils'

defineProps<{ color: string; kind: 'line' | 'box'; dash?: string; marker?: MarkerShape }>()
</script>

<template>
  <svg v-if="kind === 'line'" width="28" height="12" viewBox="0 0 28 12" aria-hidden="true">
    <line x1="0" y1="6" x2="28" y2="6" :stroke="color" stroke-width="2" :stroke-dasharray="dash || undefined" />
    <ChartMarker v-if="marker" :shape="marker" :x="14" :y="6" :color="color" />
  </svg>
  <svg v-else width="12" height="12" viewBox="0 0 12 12" preserveAspectRatio="none" aria-hidden="true">
    <rect x="1" y="1" width="10" height="10" :fill="color" />
  </svg>
</template>
