<script setup lang="ts">
import { computed } from 'vue'
import { Clock } from 'lucide-vue-next'
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
    header-class="bg-cyan-600"
    :output-handles="outputHandles"
    :has-input="!data?.isEntryNode"
  >
    <template #icon><Clock class="w-4 h-4" /></template>
    <p class="truncate">{{ summary }}</p>
  </BaseNode>
</template>
