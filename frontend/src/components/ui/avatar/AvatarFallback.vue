<script setup lang="ts">
import { AvatarFallback as AvatarFallbackPrimitive } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<{
  class?: string
}>()
</script>

<template>
  <AvatarFallbackPrimitive
    :class="cn(
      'flex h-full w-full items-center justify-center rounded-full bg-muted',
      props.class
    )"
  >
    <slot />
  </AvatarFallbackPrimitive>
</template>
