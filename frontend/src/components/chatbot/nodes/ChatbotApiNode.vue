<script setup lang="ts">
import { computed } from 'vue'
import { Globe } from 'lucide-vue-next'
import BaseNode from '@/components/calling/nodes/BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: any }>()

const summary = computed(() => {
  const cfg = props.data?.config?.api_config
  if (!cfg?.url) return 'No URL configured'
  const method = (cfg.method || 'GET').toUpperCase()
  const url = cfg.url.length > 40 ? cfg.url.slice(0, 40) + '...' : cfg.url
  return `${method} ${url}`
})
</script>

<template>
  <BaseNode :label="data?.label || 'API Fetch'" header-class="bg-orange-600" :has-input="!data?.isEntryNode">
    <template #icon><Globe class="w-4 h-4" /></template>
    <p class="truncate font-mono text-[10px]" :title="data?.config?.api_config?.url || ''">{{ summary }}</p>
  </BaseNode>
</template>
