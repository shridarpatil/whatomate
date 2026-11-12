<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { AlertCircle } from '@/lib/icons'

withDefaults(defineProps<{
  title?: string
  description?: string
  class?: HTMLAttributes['class']
  retryLabel?: string
}>(), {
  title: 'Something went wrong',
  description: 'Failed to load data. Please try again.',
  retryLabel: 'Retry',
})

defineEmits<{
  retry: []
}>()
</script>

<template>
  <div
    class="vstack items-center justify-center p-4 align-center"
    :class="$props.class"
    role="alert"
  >
    <AlertCircle :size="32" />
    <h3>
      <slot name="title">{{ title }}</slot>
    </h3>
    <p class="text-light">
      <slot name="description">{{ description }}</slot>
    </p>
    <div>
      <slot name="action">
        <button type="button" class="outline small" @click="$emit('retry')">
          {{ retryLabel }}
        </button>
      </slot>
    </div>
  </div>
</template>
