<script setup lang="ts">
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'

const open = defineModel<boolean>('open', { default: false })

const props = withDefaults(defineProps<{
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
  <AlertDialog v-model:open="open">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ title }}</AlertDialogTitle>
        <AlertDialogDescription>
          <slot name="description">
            <template v-if="description">{{ description }}</template>
            <template v-else-if="itemName">
              Are you sure you want to delete "{{ itemName }}"? This action cannot be undone.
            </template>
            <template v-else>
              Are you sure you want to delete this item? This action cannot be undone.
            </template>
          </slot>
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel :disabled="isSubmitting" @click="handleCancel">{{ cancelLabel }}</AlertDialogCancel>
        <Button
          variant="destructive"
          :loading="isSubmitting"
          @click="handleConfirm"
        >
          {{ confirmLabel }}
        </Button>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>
