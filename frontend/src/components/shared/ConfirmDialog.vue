<script setup lang="ts">
import Modal from '@/components/oat/Modal.vue'

const open = defineModel<boolean>('open', { default: false })

withDefaults(defineProps<{
  title: string
  description?: string
  confirmLabel?: string
  cancelLabel?: string
  variant?: 'default' | 'destructive'
  isSubmitting?: boolean
}>(), {
  confirmLabel: 'Confirm',
  cancelLabel: 'Cancel',
  variant: 'default',
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
      <p>{{ description }}</p>
    </slot>
    <template #footer>
      <button type="button" class="outline" :disabled="isSubmitting" @click="handleCancel">
        {{ cancelLabel }}
      </button>
      <button
        type="button"
        :data-variant="variant === 'destructive' ? 'danger' : undefined"
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
