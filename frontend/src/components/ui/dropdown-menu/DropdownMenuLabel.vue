<script setup lang="ts">
import { DropdownMenuLabel as DropdownMenuLabelPrimitive } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<{
  class?: string
}>()
</script>

<template>
  <DropdownMenuLabelPrimitive :class="cn('px-2 py-1.5 text-sm font-semibold', props.class)">
    <slot />
  </DropdownMenuLabelPrimitive>
</template>
