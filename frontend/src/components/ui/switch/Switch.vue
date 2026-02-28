<script setup lang="ts">
import type { HTMLAttributes } from "vue"
import {
  SwitchRoot,
  SwitchThumb,
} from "reka-ui"
import { cn } from "@/lib/utils"

const props = defineProps<{
  checked?: boolean
  defaultChecked?: boolean
  disabled?: boolean
  required?: boolean
  name?: string
  value?: string
  class?: HTMLAttributes["class"]
}>()

const emits = defineEmits<{
  'update:checked': [value: boolean]
}>()

function handleChange(value: boolean) {
  emits('update:checked', value)
}
</script>

<template>
  <SwitchRoot
    :model-value="props.checked"
    :default-value="props.defaultChecked"
    :disabled="props.disabled"
    :required="props.required"
    :name="props.name"
    :value="props.value"
    @update:model-value="handleChange"
    :class="cn(
      'peer inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50 data-[state=checked]:bg-primary data-[state=unchecked]:bg-input [direction:ltr]',
      props.class,
    )"
  >
    <SwitchThumb
      :class="cn('pointer-events-none block h-4 w-4 rounded-full bg-background shadow-lg ring-0 transition-transform data-[state=checked]:translate-x-4 data-[state=unchecked]:translate-x-0')"
    >
      <slot name="thumb" />
    </SwitchThumb>
  </SwitchRoot>
</template>
