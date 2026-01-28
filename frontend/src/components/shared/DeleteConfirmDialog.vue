<script setup lang="ts">
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

const open = defineModel<boolean>('open', { default: false })

const props = withDefaults(defineProps<{
  title?: string
  itemName?: string
  description?: string
  confirmLabel?: string
  cancelLabel?: string
}>(), {
  title: 'Delete Item',
  confirmLabel: 'Delete',
  cancelLabel: 'Cancel',
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
        <AlertDialogCancel @click="handleCancel">{{ cancelLabel }}</AlertDialogCancel>
        <AlertDialogAction
          @click="handleConfirm"
          class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
        >
          {{ confirmLabel }}
        </AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>
