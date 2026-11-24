<script setup lang="ts">
import { computed } from 'vue'
import { Grid3X3 } from '@/lib/icons'
import BaseNode from './BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: Record<string, any> }>()

const options = computed(() => {
  const opts = props.data?.config?.options || {}
  return Object.entries(opts).sort(([a], [b]) => a.localeCompare(b))
})

const outputHandles = computed(() => {
  const handles: { id: string; label: string; title?: string }[] = []
  for (const [digit] of options.value) {
    handles.push({ id: `digit:${digit}`, label: digit })
  }
  handles.push({ id: 'timeout', label: 'T/O', title: 'Timed Out - no input received' })
  handles.push({ id: 'max_retries', label: 'Max', title: 'Max Retries - too many invalid attempts' })
  return handles
})
</script>

<template>
  <BaseNode :label="data?.label || 'Menu'" :output-handles="outputHandles" :has-input="!data?.isEntryNode">
    <template #icon><Grid3X3 /></template>
    <template v-if="options.length > 0">
      <small v-for="[digit, opt] in options" :key="digit" class="hstack gap-1 min-w-0">
        <code>{{ digit }}:</code>
        <span class="truncate">{{ (opt as any)?.label || '—' }}</span>
      </small>
    </template>
    <small v-else>No options</small>
  </BaseNode>
</template>
