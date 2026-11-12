<script setup lang="ts">
import type { Component, HTMLAttributes } from 'vue'
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  icon?: Component
  label: string
  variant?: 'default' | 'destructive' | 'outline' | 'secondary' | 'ghost' | 'link'
  size?: 'default' | 'sm' | 'lg' | 'icon'
  // Kept for call-site compatibility; Tailwind classes no longer apply.
  class?: HTMLAttributes['class']
  disabled?: boolean
  loading?: boolean
}>(), {
  variant: 'ghost',
  size: 'icon',
  disabled: false,
  loading: false,
})

defineEmits<{
  click: [e: MouseEvent]
}>()

const classes = computed(() => [
  props.variant === 'outline' || props.variant === 'ghost' ? props.variant : undefined,
  props.size === 'sm' ? 'small' : props.size === 'lg' ? 'large' : undefined,
  props.size === 'icon' ? 'icon' : undefined,
])
const dataVariant = computed(() =>
  props.variant === 'destructive' ? 'danger' : props.variant === 'secondary' ? 'secondary' : undefined,
)
</script>

<template>
  <button
    type="button"
    :class="classes"
    :data-variant="dataVariant"
    :title="label"
    :aria-label="label"
    :disabled="disabled || loading"
    :aria-busy="loading ? 'true' : undefined"
    data-spinner="small"
    @click="$emit('click', $event)"
  >
    <slot>
      <component :is="icon" v-if="icon" />
    </slot>
  </button>
</template>
