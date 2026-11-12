<script setup lang="ts">
import { Play } from '@/lib/icons'
import BaseNode from '@/components/calling/nodes/BaseNode.vue'

defineOptions({ inheritAttrs: false })
defineProps<{ data: any }>()
</script>

<template>
  <BaseNode :label="data?.label || 'Start'" :has-input="false">
    <template #icon><Play /></template>
    <small>Flow entry point</small>
  </BaseNode>
</template>
