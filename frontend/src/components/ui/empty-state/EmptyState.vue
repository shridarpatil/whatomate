<script setup lang="ts">
import type { HTMLAttributes } from "vue"
import type { Component } from "vue"
import { cn } from "@/lib/utils"

interface EmptyStateProps {
  class?: HTMLAttributes["class"]
  icon?: Component
  title?: string
  description?: string
}

const props = defineProps<EmptyStateProps>()
</script>

<template>
  <div
    :class="cn('flex flex-col items-center justify-center py-12 px-4 text-center', props.class)"
    role="status"
    aria-live="polite"
  >
    <div
      v-if="props.icon || $slots.icon"
      class="mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-gradient-to-br from-primary/10 to-primary/5 ring-1 ring-primary/10"
    >
      <slot name="icon">
        <component :is="props.icon" class="h-7 w-7 text-primary/60" />
      </slot>
    </div>
    <h3 v-if="props.title || $slots.title" class="text-lg font-semibold text-foreground">
      <slot name="title">{{ props.title }}</slot>
    </h3>
    <p
      v-if="props.description || $slots.description"
      class="mt-1 max-w-sm text-sm text-muted-foreground"
    >
      <slot name="description">{{ props.description }}</slot>
    </p>
    <div v-if="$slots.action" class="mt-4">
      <slot name="action" />
    </div>
  </div>
</template>
