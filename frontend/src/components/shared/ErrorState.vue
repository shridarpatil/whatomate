<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { AlertCircle } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

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
    :class="cn('flex flex-col items-center justify-center py-12 px-4 text-center', $props.class)"
    role="alert"
  >
    <div class="mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-gradient-to-br from-destructive/15 to-destructive/5 ring-1 ring-destructive/10">
      <AlertCircle class="h-7 w-7 text-destructive" />
    </div>
    <h3 class="text-lg font-semibold text-foreground">
      <slot name="title">{{ title }}</slot>
    </h3>
    <p class="mt-1 max-w-sm text-sm text-muted-foreground">
      <slot name="description">{{ description }}</slot>
    </p>
    <div class="mt-4">
      <slot name="action">
        <Button variant="outline" size="sm" @click="$emit('retry')">
          {{ retryLabel }}
        </Button>
      </slot>
    </div>
  </div>
</template>
