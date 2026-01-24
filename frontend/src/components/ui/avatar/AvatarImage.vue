<script setup lang="ts">
import { AvatarImage as AvatarImagePrimitive } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<{
  src?: string
  alt?: string
  class?: string
}>()
</script>

<template>
  <AvatarImagePrimitive
    v-if="src"
    :src="src"
    :alt="alt || ''"
    :class="cn('aspect-square h-full w-full', props.class)"
  />
</template>
