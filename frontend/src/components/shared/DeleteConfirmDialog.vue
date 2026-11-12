<script setup lang="ts">
import Modal from '@/components/oat/Modal.vue'

const open = defineModel<boolean>('open', { default: false })

withDefaults(defineProps<{
  title?: string
  itemName?: string
  description?: string
  confirmLabel?: string
  cancelLabel?: string
  isSubmitting?: boolean
}>(), {
  title: 'Delete Item',
  confirmLabel: 'Delete',
  cancelLabel: 'Cancel',
  isSubmitting: false,
})

const emit = defineEmits<{
  confirm: []
  cancel: []
}>()

function handleConfirm() {
  emit('confirm')
}

function handleCancel() {
  open.value = false
  emit('cancel')
}
</script>

<template>
  <Modal v-model:open="open" alert :title="title">
    <slot name="description">
      <p v-if="description">{{ description }}</p>
      <p v-else-if="itemName">
        Are you sure you want to delete "{{ itemName }}"? This action cannot be undone.
      </p>
      <p v-else>
        Are you sure you want to delete this item? This action cannot be undone.
      </p>
    </slot>
    <template #footer>
      <button type="button" class="outline" :disabled="isSubmitting" @click="handleCancel">
        {{ cancelLabel }}
      </button>
      <button
        type="button"
        data-variant="danger"
        :disabled="isSubmitting"
        :aria-busy="isSubmitting ? 'true' : undefined"
        data-spinner="small"
        @click="handleConfirm"
      >
        {{ confirmLabel }}
      </button>
    </template>
  </Modal>
</template>
