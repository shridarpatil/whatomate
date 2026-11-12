<script setup lang="ts">
import { ref } from 'vue'
import type { ButtonConfig } from '@/types/flow-preview'
import { ExternalLink, X, List } from '@/lib/icons'

defineProps<{
  buttons: ButtonConfig[]
  disabled?: boolean
}>()

const emit = defineEmits<{
  select: [button: ButtonConfig]
}>()

const isOpen = ref(false)

function handleSelect(button: ButtonConfig) {
  emit('select', button)
  isOpen.value = false
}
</script>

<template>
  <div class="mt-1">
    <!-- Trigger Button -->
    <button
      class="w-full bg-white dark:bg-[#202c33] text-[#00a884] text-sm font-medium py-2.5 px-4 rounded-lg shadow-sm border-0 flex items-center justify-center gap-2 transition-colors"
      :class="{
        'hover:bg-gray-50 dark:hover:bg-[#2a3942] cursor-pointer': !disabled,
        'opacity-50 cursor-not-allowed': disabled
      }"
      :disabled="disabled"
      @click="isOpen = !isOpen"
    >
      <List class="h-4 w-4" />
      Select an option
    </button>

    <!-- List Picker Overlay - renders via slot in parent -->
    <Teleport to="#preview-phone-frame" :disabled="!isOpen">
      <div
        v-if="isOpen"
        class="absolute inset-0 z-10 flex flex-col"
      >
        <!-- Backdrop -->
        <div
          class="flex-1 bg-black/50"
          @click="isOpen = false"
        />

        <!-- Panel -->
        <div class="bg-white dark:bg-[#1f2c34] rounded-t-2xl overflow-hidden animate-slide-up">
          <!-- Header -->
          <div class="bg-[#075e54] dark:bg-[#00a884] text-white px-4 py-3 flex items-center justify-between">
            <button
              class="p-1 hover:bg-white/10 rounded transition-colors"
              @click="isOpen = false"
            >
              <X class="h-5 w-5" />
            </button>
            <span class="font-medium text-sm">Select an option</span>
            <div class="w-7" />
          </div>

          <!-- Options List -->
          <div class="max-h-[250px] overflow-y-auto">
            <div
              v-for="(btn, idx) in buttons"
              :key="btn.id"
              class="px-4 py-3 border-b border-gray-100 dark:border-gray-700 last:border-0 hover:bg-gray-50 dark:hover:bg-[#2a3942] cursor-pointer flex items-center gap-3 transition-colors"
              @click="handleSelect(btn)"
            >
              <div
                v-if="btn.type === 'url'"
                class="w-5 h-5 flex items-center justify-center flex-shrink-0 text-[#00a884]"
              >
                <ExternalLink class="h-4 w-4" />
              </div>
              <div
                v-else
                class="w-5 h-5 rounded-full border-2 border-[#00a884] flex items-center justify-center flex-shrink-0"
              >
                <span class="text-[10px] text-[#00a884] font-medium">{{ idx + 1 }}</span>
              </div>
              <span class="text-sm text-gray-800 dark:text-gray-200 flex-1">
                {{ btn.title || `Option ${idx + 1}` }}
              </span>
              <ExternalLink v-if="btn.type === 'url'" class="h-3 w-3 text-gray-400" />
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
@keyframes slide-up {
  from {
    transform: translateY(100%);
  }
  to {
    transform: translateY(0);
  }
}

.animate-slide-up {
  animation: slide-up 0.2s ease-out;
}
</style>
