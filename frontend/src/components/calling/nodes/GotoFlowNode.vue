<script setup lang="ts">
import { computed } from 'vue'
import { ExternalLink } from '@/lib/icons'
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
  <BaseNode :label="data?.label || 'Goto Flow'" :output-handles="[]" :has-input="!data?.isEntryNode">
    <template #icon><ExternalLink /></template>
    <small class="truncate">{{ summary }}</small>
  </BaseNode>
</template>
