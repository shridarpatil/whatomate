<script setup lang="ts">
import { DropdownMenuRoot } from 'reka-ui'

// `open` must default to undefined: Vue casts an absent boolean prop to false,
// which reka-ui reads as controlled mode and the menu can never open.
withDefaults(defineProps<{
  open?: boolean
}>(), {
  open: undefined,
})

const emit = defineEmits<{
  'update:open': [value: boolean]
}>()
</script>

<template>
  <DropdownMenuRoot :open="open" @update:open="emit('update:open', $event)">
    <slot />
  </DropdownMenuRoot>
</template>
