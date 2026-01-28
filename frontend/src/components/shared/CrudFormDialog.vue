<script setup lang="ts">
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Loader2 } from 'lucide-vue-next'

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
</script>

<script lang="ts">
import { computed } from 'vue'
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent :class="maxWidth">
      <DialogHeader>
        <DialogTitle>{{ computedTitle }}</DialogTitle>
        <DialogDescription>{{ computedDescription }}</DialogDescription>
      </DialogHeader>

      <div class="py-4">
        <slot />
      </div>

      <DialogFooter>
        <Button variant="outline" size="sm" @click="handleCancel">
          {{ cancelLabel }}
        </Button>
        <Button size="sm" @click="handleSubmit" :disabled="isSubmitting">
          <Loader2 v-if="isSubmitting" class="h-4 w-4 mr-2 animate-spin" />
          {{ computedSubmitLabel }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
