<script setup lang="ts">
import { Play } from 'lucide-vue-next'
import BaseNode from '@/components/calling/nodes/BaseNode.vue'

defineOptions({ inheritAttrs: false })
defineProps<{ data: any }>()
</script>

<template>
  <BaseNode :label="data?.label || 'Start'" header-class="bg-emerald-600" :has-input="false">
    <template #icon><Play class="w-4 h-4" /></template>
    <p class="text-[10px] text-muted-foreground">Flow entry point</p>
  </BaseNode>
</template>
