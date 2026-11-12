<script setup lang="ts">
import { ref } from 'vue'
import type { ButtonConfig } from '@/types/flow-preview'
import { ExternalLink, List } from '@/lib/icons'
import Modal from '@/components/oat/Modal.vue'

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
  <div class="mt-2">
    <!-- Trigger Button -->
    <button type="button" class="outline w-100" :disabled="disabled" @click="isOpen = !isOpen">
      <List />
      Select an option
    </button>

    <Modal v-model:open="isOpen" size="small" :title="$t('flowBuilder.selectOption')">
      <ul class="unstyled vstack gap-2">
        <li v-for="(btn, idx) in buttons" :key="btn.id">
          <button type="button" class="outline w-100 justify-between" @click="handleSelect(btn)">
            <span class="hstack gap-2 min-w-0">
              <ExternalLink v-if="btn.type === 'url'" />
              <span v-else class="badge outline">{{ idx + 1 }}</span>
              <span class="truncate">{{ btn.title || `Option ${idx + 1}` }}</span>
            </span>
          </button>
        </li>
      </ul>
    </Modal>
  </div>
</template>
