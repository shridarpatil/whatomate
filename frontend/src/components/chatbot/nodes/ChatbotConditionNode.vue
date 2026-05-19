<script setup lang="ts">
import { computed } from 'vue'
import { GitBranch } from 'lucide-vue-next'
import BaseNode from '@/components/calling/nodes/BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: any }>()

const summary = computed(() => {
  const cfg = props.data?.config || {}
  const expression = (cfg.expression || cfg.input_config?.expression) as string | undefined
  if (!expression) return 'No expression'
  return expression.length > 60 ? expression.slice(0, 60) + '…' : expression
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
