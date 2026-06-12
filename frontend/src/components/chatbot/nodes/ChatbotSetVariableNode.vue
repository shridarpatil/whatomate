<script setup lang="ts">
import { computed } from 'vue'
import { Sliders } from 'lucide-vue-next'
import BaseNode from '@/components/calling/nodes/BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: any }>()

const summary = computed(() => {
  const cfg = props.data?.config || {}
  const set = (cfg.set || {}) as Record<string, any>
  const keys = Object.keys(set)
  if (keys.length === 0) return 'No variables set'
  return keys.map(k => `${k} = ${set[k]}`).join(', ')
})
</script>

<template>
  <BaseNode
    :label="data?.label || 'Set Variable'"
    header-class="bg-pink-600"
    :has-input="!data?.isEntryNode"
  >
    <template #icon><Sliders class="w-4 h-4" /></template>
    <p class="truncate text-xs px-1" :title="summary">{{ summary }}</p>
  </BaseNode>
</template>
