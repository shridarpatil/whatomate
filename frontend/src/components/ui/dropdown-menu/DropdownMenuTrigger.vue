<script setup lang="ts">
import { DropdownMenuTrigger as DropdownMenuTriggerPrimitive } from 'reka-ui'

defineProps<{
  asChild?: boolean
}>()
</script>

<template>
  <DropdownMenuTriggerPrimitive :as-child="asChild">
    <slot />
  </DropdownMenuTriggerPrimitive>
</template>
