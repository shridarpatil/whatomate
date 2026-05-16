<script setup lang="ts">
import { computed } from 'vue'
import { StopCircle } from 'lucide-vue-next'
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
    header-class="bg-slate-600"
    :output-handles="[]"
    :has-input="!data?.isEntryNode"
  >
    <template #icon><StopCircle class="w-4 h-4" /></template>
    <p v-if="completionMessage" class="truncate" :title="data?.config?.message || ''">{{ completionMessage }}</p>
    <p v-else>End of flow</p>
  </BaseNode>
</template>
