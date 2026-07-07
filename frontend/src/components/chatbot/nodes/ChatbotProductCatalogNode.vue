<script setup lang="ts">
import { computed } from 'vue'
import { ShoppingBag } from 'lucide-vue-next'
import BaseNode from '@/components/calling/nodes/BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: any }>()

const body = computed(() => {
  const b = props.data?.config?.body || props.data?.config?.message || ''
  return b.length > 55 ? b.slice(0, 55) + '...' : b || 'Exibir catálogo'
})

const catalogId = computed(() => props.data?.config?.catalog_id || '')
</script>

<template>
  <BaseNode :label="data?.label || 'Catálogo'" header-class="bg-lime-600" :has-input="!data?.isEntryNode">
    <template #icon><ShoppingBag class="w-4 h-4" /></template>
    <p class="truncate text-muted-foreground" :title="body">{{ body }}</p>
    <span
      v-if="catalogId"
      class="inline-block mt-1 px-1.5 py-0.5 bg-lime-500/15 text-lime-400 rounded text-[10px] font-mono"
    >
      ID: {{ catalogId }}
    </span>
    <span
      v-else
      class="inline-block mt-1 px-1.5 py-0.5 bg-yellow-500/15 text-yellow-400 rounded text-[10px]"
    >
      ⚠ Configure o Catalog ID
    </span>
  </BaseNode>
</template>
