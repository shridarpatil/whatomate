<script setup lang="ts">
import { computed } from 'vue'
import { Handle, Position, useNode } from '@vue-flow/core'

withDefaults(
  defineProps<{
    label: string
    hasInput?: boolean
    outputHandles?: { id: string; label: string; title?: string }[]
  }>(),
  { hasInput: true },
)

// vue-flow toggles `selected` on the node when it is clicked / box-selected.
// The selected card gets Oat's own focus ring (outline in --ring), bound at
// runtime because no Oat class reflects canvas selection.
const { node } = useNode()
const selectedStyle = computed(() =>
  node.selected ? { outline: '2px solid var(--ring)', outlineOffset: '2px' } : undefined,
)

function handleLeft(idx: number, count: number) {
  return count === 1 ? '50%' : `${((idx + 1) / (count + 1)) * 100}%`
}
</script>

<template>
  <article class="card p-4 relative flow-node" :data-node-type="node.type" :style="selectedStyle">
    <!-- Input handle (top) -->
    <Handle v-if="hasInput !== false" id="input" type="target" :position="Position.Top" />

    <header class="hstack gap-2 mb-2 min-w-0">
      <span data-node-icon><slot name="icon" /></span>
      <strong class="truncate">{{ label }}</strong>
    </header>

    <div class="text-light vstack gap-1 min-w-0">
      <slot />
    </div>

    <!-- Output handles (bottom) -->
    <template v-if="outputHandles && outputHandles.length > 0">
      <Handle
        v-for="(handle, idx) in outputHandles"
        :id="handle.id"
        :key="handle.id"
        type="source"
        :position="Position.Bottom"
        :title="handle.title || handle.label"
        :style="{ left: handleLeft(idx, outputHandles.length) }"
      />
      <small
        v-for="(handle, idx) in outputHandles"
        :key="'num-' + handle.id"
        class="handle-label text-light"
        :style="{ left: handleLeft(idx, outputHandles.length) }"
      >{{ idx + 1 }}</small>
    </template>
    <Handle v-else-if="!outputHandles" id="default" type="source" :position="Position.Bottom" />
  </article>
</template>
