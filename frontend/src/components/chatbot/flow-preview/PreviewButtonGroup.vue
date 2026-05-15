<script setup lang="ts">
import type { ButtonConfig } from '@/types/flow-preview'
import { ExternalLink, Phone, PhoneCall } from 'lucide-vue-next'

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
  <div class="mt-1 space-y-1">
    <button
      v-for="btn in buttons"
      :key="btn.id"
      class="w-full bg-white dark:bg-[#202c33] text-[#00a884] text-sm font-medium py-2.5 px-4 rounded-lg shadow-sm border-0 flex items-center justify-center gap-1.5 transition-colors"
      :class="{
        'hover:bg-gray-50 dark:hover:bg-[#2a3942] cursor-pointer': !disabled,
        'opacity-50 cursor-not-allowed': disabled
      }"
      :disabled="disabled"
      @click="handleClick(btn)"
    >
      <ExternalLink v-if="btn.type === 'url'" class="h-4 w-4" />
      <Phone v-else-if="btn.type === 'phone'" class="h-4 w-4" />
      <PhoneCall v-else-if="btn.type === 'voice_call'" class="h-4 w-4" />
      {{ btn.title || 'Option' }}
    </button>
  </div>
</template>
