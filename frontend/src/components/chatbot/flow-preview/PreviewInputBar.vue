<script setup lang="ts">
import { ref, computed } from 'vue'
import { Send, Mic } from '@/lib/icons'

const props = defineProps<{
  inputType: string | null
  disabled?: boolean
  placeholder?: string
}>()

const emit = defineEmits<{
  submit: [value: string]
}>()

const inputValue = ref('')

const inputPlaceholder = computed(() => {
  if (props.placeholder) return props.placeholder

  switch (props.inputType) {
    case 'email':
      return 'Enter your email address...'
    case 'phone':
      return 'Enter your phone number...'
    case 'number':
      return 'Enter a number...'
    case 'date':
      return 'Enter a date (YYYY-MM-DD)...'
    case 'text':
      return 'Type a message...'
    default:
      return 'Type a message...'
  }
})

const inputTypeAttr = computed(() => {
  switch (props.inputType) {
    case 'email':
      return 'email'
    case 'phone':
      return 'tel'
    case 'number':
      return 'number'
    case 'date':
      return 'date'
    default:
      return 'text'
  }
})

const isEnabled = computed(() => {
  return !props.disabled && props.inputType && props.inputType !== 'none' && props.inputType !== 'button'
})

// Convert input to string (handles number inputs)
const inputString = computed(() => String(inputValue.value ?? '').trim())

function handleSubmit() {
  if (!inputString.value || !isEnabled.value) return

  emit('submit', inputString.value)
  inputValue.value = ''
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    handleSubmit()
  }
}
</script>

<template>
  <form class="hstack gap-2" @submit.prevent="handleSubmit">
    <input
      v-if="isEnabled"
      v-model="inputValue"
      class="flex-1 min-w-0"
      :type="inputTypeAttr"
      :placeholder="inputPlaceholder"
      :aria-label="inputPlaceholder"
      @keydown="handleKeydown"
    >
    <input
      v-else
      class="flex-1 min-w-0"
      disabled
      :placeholder="disabled ? 'Waiting...' : 'Type a message'"
      :aria-label="disabled ? 'Waiting...' : 'Type a message'"
    >

    <button
      type="submit"
      class="icon"
      :disabled="!isEnabled || !inputString"
      :aria-label="$t('chat.send')"
      :title="$t('chat.send')"
    >
      <Send v-if="inputString" />
      <Mic v-else />
    </button>
  </form>
</template>
