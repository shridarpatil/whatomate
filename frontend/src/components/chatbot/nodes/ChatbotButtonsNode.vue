<script setup lang="ts">
import { computed } from 'vue'
import { MousePointerClick } from '@/lib/icons'
import BaseNode from '@/components/calling/nodes/BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: any }>()

const buttons = computed(() => props.data?.config?.buttons || [])

const outputHandles = computed(() => {
  // Handle id becomes the edge condition on save. The graph runner routes
  // button taps via the namespaced "button:<id>" outcome (author-controlled
  // ids are prefixed so they can't collide with reserved outcomes like
  // "default" or "true"), so the handle must carry that prefix too.
  return buttons.value.map((b: any) => ({
    id: `button:${b.id}`,
    label: b.title || '—',
    title: b.title || '—',
  }))
})
</script>

<template>
  <BaseNode :label="data?.label || 'Buttons'" :output-handles="outputHandles" :has-input="!data?.isEntryNode">
    <template #icon><MousePointerClick /></template>
    <template v-if="buttons.length > 0">
      <small v-for="(btn, idx) in buttons" :key="btn.id" class="hstack gap-1 min-w-0" :title="btn.title">
        <code>{{ Number(idx) + 1 }}:</code>
        <span class="truncate">{{ btn.title || '—' }}</span>
      </small>
    </template>
    <small v-else><em>No buttons</em></small>
  </BaseNode>
</template>
