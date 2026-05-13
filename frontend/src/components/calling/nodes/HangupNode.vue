<script setup lang="ts">
import { PhoneOff } from 'lucide-vue-next'
import BaseNode from './BaseNode.vue'

defineOptions({ inheritAttrs: false })

defineProps<{ data: Record<string, any> }>()
</script>

<template>
  <BaseNode :label="data?.label || 'Hangup'" header-class="bg-red-600" :output-handles="[]" :has-input="!data?.isEntryNode">
    <template #icon><PhoneOff class="w-4 h-4" /></template>
    <p v-if="data?.config?.audio_file || data?.config?.greeting_text" class="truncate">
      {{ data.config.greeting_text || data.config.audio_file }}
    </p>
    <p v-else>End call</p>
  </BaseNode>
</template>
