<script setup lang="ts">
import { computed } from 'vue'
import { MessageSquare } from 'lucide-vue-next'
import BaseNode from '@/components/calling/nodes/BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: any }>()

const message = computed(() => {
  const msg = props.data?.config?.message || ''
  return msg.length > 60 ? msg.slice(0, 60) + '...' : msg || 'No message'
})

const inputType = computed(() => props.data?.config?.input_type)
</script>

<template>
  <BaseNode :label="data?.label || 'Text'" header-class="bg-blue-600" :has-input="!data?.isEntryNode">
    <template #icon><MessageSquare class="w-4 h-4" /></template>
    <p class="truncate" :title="data?.config?.message || ''">{{ message }}</p>
    <span v-if="inputType && inputType !== 'none'" class="inline-block mt-1 px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-300 rounded text-[10px] font-medium">
      {{ inputType }}
    </span>
  </BaseNode>
</template>
