<script setup lang="ts">
import { PhoneOff } from '@/lib/icons'
import BaseNode from './BaseNode.vue'

defineOptions({ inheritAttrs: false })

defineProps<{ data: Record<string, any> }>()
</script>

<template>
  <BaseNode :label="data?.label || 'Hangup'" :output-handles="[]" :has-input="!data?.isEntryNode">
    <template #icon><PhoneOff /></template>
    <small v-if="data?.config?.audio_file || data?.config?.greeting_text" class="truncate">
      {{ data.config.greeting_text || data.config.audio_file }}
    </small>
    <small v-else>End call</small>
  </BaseNode>
</template>
