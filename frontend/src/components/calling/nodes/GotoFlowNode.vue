<script setup lang="ts">
import { computed } from 'vue'
import { ExternalLink } from 'lucide-vue-next'
import BaseNode from './BaseNode.vue'
import { useCallingStore } from '@/stores/calling'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: Record<string, any> }>()
const callingStore = useCallingStore()

const summary = computed(() => {
  const flowId = props.data?.config?.flow_id
  if (!flowId) return 'No flow'
  const flow = callingStore.ivrFlows.find(f => f.id === flowId)
  return flow?.name || 'No flow'
})
</script>

<template>
  <BaseNode :label="data?.label || 'Goto Flow'" header-class="bg-teal-600" :output-handles="[]" :has-input="!data?.isEntryNode">
    <template #icon><ExternalLink class="w-3 h-3" /></template>
    <p class="truncate">{{ summary }}</p>
  </BaseNode>
</template>
