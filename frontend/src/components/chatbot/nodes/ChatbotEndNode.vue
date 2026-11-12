<script setup lang="ts">
import { computed } from 'vue'
import { StopCircle } from '@/lib/icons'
import BaseNode from '@/components/calling/nodes/BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: any }>()

const completionMessage = computed(() => {
  const msg = props.data?.config?.message || ''
  return msg.length > 60 ? msg.slice(0, 60) + '...' : msg
})
</script>

<template>
  <BaseNode
    :label="data?.label || 'End'"
    :output-handles="[]"
    :has-input="!data?.isEntryNode"
  >
    <template #icon><StopCircle /></template>
    <small v-if="completionMessage" class="truncate" :title="data?.config?.message || ''">{{ completionMessage }}</small>
    <small v-else>End of flow</small>
  </BaseNode>
</template>
