<script setup lang="ts">
import { computed } from 'vue'
import { Globe } from 'lucide-vue-next'
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
  <BaseNode :label="data?.label || 'API'" header-class="bg-orange-600" :has-input="!data?.isEntryNode">
    <template #icon><Globe class="w-4 h-4" /></template>
    <p class="truncate font-mono text-[10px]" :title="urlTitle">{{ summary }}</p>
  </BaseNode>
</template>
