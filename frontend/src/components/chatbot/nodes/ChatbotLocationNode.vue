<script setup lang="ts">
import { computed } from 'vue'
import { MapPin } from 'lucide-vue-next'
import BaseNode from '@/components/calling/nodes/BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: any }>()

const body = computed(() => {
  const b = props.data?.config?.body || props.data?.config?.message || ''
  return b.length > 55 ? b.slice(0, 55) + '...' : b || 'Solicitar localização'
})
</script>

<template>
  <BaseNode :label="data?.label || 'Localização'" header-class="bg-rose-600" :has-input="!data?.isEntryNode">
    <template #icon><MapPin class="w-4 h-4" /></template>
    <p class="truncate text-muted-foreground" :title="body">{{ body }}</p>
    <span class="inline-block mt-1 px-1.5 py-0.5 bg-rose-500/15 text-rose-400 rounded text-[10px] font-medium">
      📍 Pede localização ao usuário
    </span>
  </BaseNode>
</template>
