<script setup lang="ts">
import { computed } from 'vue'
import { Clock } from 'lucide-vue-next'
import BaseNode from './BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: Record<string, any> }>()

const summary = computed(() => {
  const c = props.data?.config || {}
  const schedule = c.schedule || []
  const activeDays = schedule.filter((s: any) => s.enabled).length
  return `${activeDays}/7 days active`
})

const outputHandles = [
  { id: 'in_hours', label: 'Open', title: 'Open - within business hours' },
  { id: 'out_of_hours', label: 'Closed', title: 'Closed - outside business hours' },
]
</script>

<template>
  <BaseNode :label="data?.label || 'Timing'" header-class="bg-cyan-600" :output-handles="outputHandles" :has-input="!data?.isEntryNode">
    <template #icon><Clock class="w-4 h-4" /></template>
    <p class="truncate">{{ summary }}</p>
  </BaseNode>
</template>
