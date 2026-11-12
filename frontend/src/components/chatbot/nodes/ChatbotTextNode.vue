<script setup lang="ts">
import { computed } from 'vue'
import { MessageSquare } from '@/lib/icons'
import BaseNode from '@/components/calling/nodes/BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: any }>()

const message = computed(() => {
  const cfg = props.data?.config || {}
  const msg = cfg.message || cfg.body || ''
  return msg.length > 60 ? msg.slice(0, 60) + '...' : msg || 'No message'
})

const inputType = computed(() => props.data?.config?.input_type)
</script>

<template>
  <BaseNode :label="data?.label || 'Text'" :has-input="!data?.isEntryNode">
    <template #icon><MessageSquare /></template>
    <small class="truncate" :title="data?.config?.message || ''">{{ message }}</small>
    <span v-if="inputType && inputType !== 'none'"><span class="badge" data-variant="secondary">{{ inputType }}</span></span>
  </BaseNode>
</template>
