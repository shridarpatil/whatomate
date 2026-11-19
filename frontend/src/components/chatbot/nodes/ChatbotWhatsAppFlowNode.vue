<script setup lang="ts">
import { computed } from 'vue'
import { MessageCircle } from '@/lib/icons'
import BaseNode from '@/components/calling/nodes/BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: any }>()

const summary = computed(() => {
  const cfg = props.data?.config || {}
  return cfg.cta || cfg.cta_text || cfg.flow_name || cfg.body || 'WhatsApp Flow'
})
</script>

<template>
  <BaseNode :label="data?.label || 'WhatsApp Flow'" :has-input="!data?.isEntryNode">
    <template #icon><MessageCircle /></template>
    <small class="truncate" :title="summary">{{ summary }}</small>
  </BaseNode>
</template>
