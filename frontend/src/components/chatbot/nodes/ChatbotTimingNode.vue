<script setup lang="ts">
import { computed } from 'vue'
import { Clock } from '@/lib/icons'
import BaseNode from '@/components/calling/nodes/BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: any }>()

const summary = computed(() => {
  const cfg = props.data?.config?.input_config || props.data?.config || {}
  const schedule = (cfg.schedule as any[]) || []
  const active = schedule.filter((s) => s?.enabled).length
  return `${active}/7 days active`
})

const outputHandles = [
  { id: 'in_hours', label: 'Open', title: 'Within business hours' },
  { id: 'out_of_hours', label: 'Closed', title: 'Outside business hours' },
]
</script>

<template>
  <BaseNode
    :label="data?.label || 'Timing'"
    :output-handles="outputHandles"
    :has-input="!data?.isEntryNode"
  >
    <template #icon><Clock /></template>
    <small class="truncate">{{ summary }}</small>
  </BaseNode>
</template>
