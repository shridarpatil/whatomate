<script setup lang="ts">
import { computed } from 'vue'
import { MousePointerClick } from 'lucide-vue-next'
import BaseNode from '@/components/calling/nodes/BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: any }>()

const buttons = computed(() => props.data?.config?.buttons || [])

const outputHandles = computed(() => {
  return buttons.value.map((b: any) => ({
    id: b.id,
    label: b.title || '—',
    title: b.title || '—',
  }))
})
</script>

<template>
  <BaseNode :label="data?.label || 'Buttons'" header-class="bg-purple-600" :output-handles="outputHandles" :has-input="!data?.isEntryNode">
    <template #icon><MousePointerClick class="w-4 h-4" /></template>
    <div v-if="buttons.length > 0" class="space-y-0.5">
      <div v-for="(btn, idx) in buttons" :key="btn.id" class="flex gap-1" :title="btn.title">
        <span class="font-mono font-bold">{{ Number(idx) + 1 }}:</span>
        <span class="truncate">{{ btn.title || '—' }}</span>
      </div>
    </div>
    <p v-else class="text-muted-foreground italic">No buttons</p>
  </BaseNode>
</template>
