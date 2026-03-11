<script setup lang="ts">
import { computed } from 'vue'
import { Globe } from 'lucide-vue-next'
import BaseNode from './BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: Record<string, any> }>()

const summary = computed(() => {
  const c = props.data?.config || {}
  const method = c.method || 'GET'
  const url = c.url || 'No URL'
  return `${method} ${url}`
})

const outputHandles = [
  { id: 'http:2xx', label: '2xx', title: 'Success - HTTP 200-299 response' },
  { id: 'http:non2xx', label: 'Error', title: 'Error - HTTP non-2xx or request failed' },
]
</script>

<template>
  <BaseNode :label="data?.label || 'HTTP Callback'" header-class="bg-orange-600" :output-handles="outputHandles" :has-input="!data?.isEntryNode">
    <template #icon><Globe class="w-3 h-3" /></template>
    <p class="truncate font-mono text-[10px]">{{ summary }}</p>
  </BaseNode>
</template>
