<script setup lang="ts">
import { computed } from 'vue'
import { Volume2 } from '@/lib/icons'
import BaseNode from './BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: Record<string, any> }>()

const summary = computed(() => {
  const c = props.data?.config || {}
  if (c.greeting_text) return c.greeting_text.substring(0, 40) + (c.greeting_text.length > 40 ? '...' : '')
  if (c.audio_file) return c.audio_file
  return 'No audio configured'
})

const outputHandles = [{ id: 'default', label: 'next' }]
</script>

<template>
  <BaseNode :label="data?.label || 'Greeting'" :output-handles="outputHandles" :has-input="!data?.isEntryNode">
    <template #icon><Volume2 /></template>
    <small class="truncate">{{ summary }}</small>
    <span v-if="data?.config?.interruptible"><span class="badge" data-variant="success">Interruptible</span></span>
  </BaseNode>
</template>
