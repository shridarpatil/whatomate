<script setup lang="ts">
const props = withDefaults(defineProps<{
  label: string
  value: number
  max?: number
  low?: number
  high?: number
  optimum?: number
  format?: (v: number) => string
}>(), { max: 100 })

const text = () => (props.format ? props.format(props.value) : String(props.value))
</script>

<template>
  <div class="meter-row">
    <small>{{ label }}</small>
    <meter :aria-label="label" min="0" :max="max" :low="low" :high="high" :optimum="optimum" :value="value" />
    <small>{{ text() }}</small>
  </div>
</template>
