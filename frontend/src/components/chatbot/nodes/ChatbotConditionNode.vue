<script setup lang="ts">
import { computed } from 'vue'
import { GitBranch } from 'lucide-vue-next'
import BaseNode from '@/components/calling/nodes/BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: any }>()

const operatorLabels: Record<string, string> = {
  eq: '=',
  equals: '=',
  neq: '≠',
  not_equals: '≠',
  contains: 'contains',
  starts_with: 'starts with',
  empty: 'is empty',
  not_empty: 'is set',
  exists: 'is set',
}

const summary = computed(() => {
  const cfg = props.data?.config?.input_config || props.data?.config || {}
  const variable = cfg.variable || '?'
  const op = operatorLabels[cfg.operator] || cfg.operator || '=='
  if (cfg.operator === 'empty' || cfg.operator === 'not_empty' || cfg.operator === 'exists') {
    return `${variable} ${op}`
  }
  const value = cfg.value ?? ''
  return `${variable} ${op} ${value}`
})

const outputHandles = [
  { id: 'true', label: 'True' },
  { id: 'false', label: 'False' },
]
</script>

<template>
  <BaseNode
    :label="data?.label || 'Condition'"
    header-class="bg-indigo-600"
    :output-handles="outputHandles"
    :has-input="!data?.isEntryNode"
  >
    <template #icon><GitBranch class="w-4 h-4" /></template>
    <p class="truncate" :title="summary">{{ summary }}</p>
  </BaseNode>
</template>
