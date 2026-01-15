<script setup lang="ts">
import { Separator as SeparatorPrimitive } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<{
  orientation?: 'horizontal' | 'vertical'
  decorative?: boolean
  class?: string
}>()
</script>

<template>
  <SeparatorPrimitive
    :orientation="orientation ?? 'horizontal'"
    :decorative="decorative"
    :class="cn(
      'shrink-0 bg-border',
      orientation === 'vertical' ? 'h-full w-[1px]' : 'h-[1px] w-full',
      props.class
    )"
  />
</template>
