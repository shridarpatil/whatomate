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

defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  stay: []
  leave: []
}>()
</script>

<template>
  <AlertDialog :open="open">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ $t('common.unsavedChangesTitle', 'Unsaved Changes') }}</AlertDialogTitle>
        <AlertDialogDescription>
          {{ $t('common.unsavedChangesDesc', 'You have unsaved changes. Are you sure you want to leave? Your changes will be lost.') }}
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel @click="emit('stay')">{{ $t('common.stay', 'Stay') }}</AlertDialogCancel>
        <AlertDialogAction @click="emit('leave')">{{ $t('common.leave', 'Leave') }}</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>
