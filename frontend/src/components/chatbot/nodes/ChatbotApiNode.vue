<script setup lang="ts">
import { computed } from 'vue'
import { Globe } from '@/lib/icons'
import BaseNode from '@/components/calling/nodes/BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: any }>()

const summary = computed(() => {
  const cfg = props.data?.config || {}
  const url = cfg.url || cfg.api_config?.url
  if (!url) return 'No URL configured'
  const method = (cfg.method || cfg.api_config?.method || 'GET').toUpperCase()
  const trimmed = url.length > 40 ? url.slice(0, 40) + '...' : url
  return `${method} ${trimmed}`
})

const urlTitle = computed(() => props.data?.config?.url || props.data?.config?.api_config?.url || '')
</script>

<template>
  <BaseNode :label="data?.label || 'API'" :has-input="!data?.isEntryNode">
    <template #icon><Globe /></template>
    <small class="truncate" :title="urlTitle"><code>{{ summary }}</code></small>
  </BaseNode>
</template>
