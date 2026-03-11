<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'

withDefaults(
  defineProps<{
    label: string
    headerClass: string
    hasInput?: boolean
    outputHandles?: { id: string; label: string; title?: string }[]
  }>(),
  { hasInput: true },
)
</script>

<template>
  <div class="relative bg-background border rounded-lg shadow-sm min-w-[180px] max-w-[240px] overflow-visible">
    <!-- Input handle (top) -->
    <Handle
      v-if="hasInput !== false"
      id="input"
      type="target"
      :position="Position.Top"
      class="!w-3 !h-3 !rounded-full !bg-slate-400 !border-2 !border-background"
      style="z-index: 10;"
    />

    <!-- Header -->
    <div :class="['px-3 py-1.5 rounded-t-lg text-white text-xs font-medium flex items-center gap-1.5 overflow-hidden', headerClass]">
      <slot name="icon" />
      <span class="truncate">{{ label }}</span>
    </div>

    <!-- Body -->
    <div class="px-3 py-2 text-xs text-muted-foreground">
      <slot />
    </div>

    <!-- Output handles (bottom) -->
    <template v-if="outputHandles && outputHandles.length > 0">
      <!-- Handle labels -->
      <div v-if="outputHandles.length > 1" class="flex px-1 pt-1 border-t border-dashed">
        <div
          v-for="handle in outputHandles"
          :key="'label-' + handle.id"
          class="flex-1 text-center"
        >
          <span class="text-[9px] font-medium text-muted-foreground cursor-default" :title="handle.title">{{ handle.label }}</span>
        </div>
      </div>
      <Handle
        v-for="(handle, idx) in outputHandles"
        :key="handle.id"
        type="source"
        :id="handle.id"
        :position="Position.Bottom"
        :style="{
          left: outputHandles.length === 1 ? '50%' : `${((idx + 1) / (outputHandles.length + 1)) * 100}%`,
          zIndex: 10,
        }"
        class="!w-3 !h-3 !rounded-full !bg-primary !border-2 !border-background"
      />
    </template>
    <template v-else-if="!outputHandles">
      <Handle
        id="default"
        type="source"
        :position="Position.Bottom"
        class="!w-3 !h-3 !rounded-full !bg-primary !border-2 !border-background"
        style="z-index: 10;"
      />
    </template>
  </div>
</template>
