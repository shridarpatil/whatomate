<script setup lang="ts">
import { computed } from 'vue'
import Modal from '@/components/oat/Modal.vue'

const open = defineModel<boolean>('open', { default: false })

const props = withDefaults(defineProps<{
  title?: string
  editTitle?: string
  createTitle?: string
  description?: string
  editDescription?: string
  createDescription?: string
  isEditing?: boolean
  isSubmitting?: boolean
  submitLabel?: string
  editSubmitLabel?: string
  createSubmitLabel?: string
  cancelLabel?: string
  maxWidth?: string
}>(), {
  title: '',
  editTitle: 'Edit Item',
  createTitle: 'Create Item',
  description: '',
  editDescription: 'Update the item details.',
  createDescription: 'Fill in the details to create a new item.',
  isEditing: false,
  isSubmitting: false,
  submitLabel: '',
  editSubmitLabel: 'Update',
  createSubmitLabel: 'Create',
  cancelLabel: 'Cancel',
  maxWidth: 'max-w-md',
})

const emit = defineEmits<{
  submit: []
  cancel: []
}>()

function handleSubmit() {
  emit('submit')
}

function handleCancel() {
  open.value = false
  emit('cancel')
}

const computedTitle = computed(() => {
  if (props.title) return props.title
  return props.isEditing ? props.editTitle : props.createTitle
})

const computedDescription = computed(() => {
  if (props.description) return props.description
  return props.isEditing ? props.editDescription : props.createDescription
})

const computedSubmitLabel = computed(() => {
  if (props.submitLabel) return props.submitLabel
  return props.isEditing ? props.editSubmitLabel : props.createSubmitLabel
})

// Tailwind width tokens from call sites map onto the two Modal sizes.
const size = computed(() => (/max-w-(lg|xl|[2-7]xl)/.test(props.maxWidth) ? 'large' : undefined))
</script>

<template>
  <Modal v-model:open="open" :title="computedTitle" :description="computedDescription" :size="size">
    <slot />
    <template #footer>
      <button type="button" class="outline" @click="handleCancel">{{ cancelLabel }}</button>
      <button
        type="button"
        :disabled="isSubmitting"
        :aria-busy="isSubmitting ? 'true' : undefined"
        data-spinner="small"
        @click="handleSubmit"
      >
        {{ computedSubmitLabel }}
      </button>
    </template>
  </Modal>
</template>
