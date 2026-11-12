<script setup lang="ts">
import type { ButtonConfig } from '@/types/flow-preview'
import { ExternalLink, Phone, PhoneCall } from '@/lib/icons'

defineProps<{
  buttons: ButtonConfig[]
  disabled?: boolean
}>()

const emit = defineEmits<{
  select: [button: ButtonConfig]
}>()

function handleClick(button: ButtonConfig) {
  emit('select', button)
}
</script>

<template>
  <div class="vstack gap-1 mt-2">
    <button
      v-for="btn in buttons"
      :key="btn.id"
      type="button"
      class="outline w-100"
      :disabled="disabled"
      @click="handleClick(btn)"
    >
      <ExternalLink v-if="btn.type === 'url'" />
      <Phone v-else-if="btn.type === 'phone'" />
      <PhoneCall v-else-if="btn.type === 'voice_call'" />
      {{ btn.title || 'Option' }}
    </button>
  </div>
</template>
