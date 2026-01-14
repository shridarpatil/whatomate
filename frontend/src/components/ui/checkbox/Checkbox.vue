<script setup lang="ts">
import type { HTMLAttributes } from "vue"
import { CheckIcon } from '@radix-icons/vue'
import { CheckboxIndicator, CheckboxRoot } from "reka-ui"
import { cn } from "@/lib/utils"

type CheckedState = boolean | 'indeterminate'

const props = defineProps<{
  checked?: CheckedState
  defaultChecked?: CheckedState
  disabled?: boolean
  required?: boolean
  name?: string
  value?: string
  id?: string
  class?: HTMLAttributes["class"]
}>()

const emits = defineEmits<{
  'update:checked': [value: CheckedState]
}>()

function handleChange(value: CheckedState) {
  emits('update:checked', value)
}
</script>

<template>
  <CheckboxRoot
    :model-value="props.checked"
    :default-value="props.defaultChecked"
    :disabled="props.disabled"
    :required="props.required"
    :name="props.name"
    :value="props.value"
    :id="props.id"
    @update:model-value="handleChange"
    :class="
      cn('grid place-content-center peer h-4 w-4 shrink-0 rounded-sm border border-primary shadow focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50 data-[state=checked]:bg-primary data-[state=checked]:text-primary-foreground',
         props.class)"
  >
    <CheckboxIndicator class="grid place-content-center text-current">
      <slot>
        <CheckIcon class="h-4 w-4" />
      </slot>
    </CheckboxIndicator>
  </CheckboxRoot>
</template>
