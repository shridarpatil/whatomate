<script setup lang="ts">
import Modal from '@/components/oat/Modal.vue'

defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  stay: []
  leave: []
}>()
</script>

<template>
  <Modal
    :open="open"
    alert
    :title="$t('common.unsavedChangesTitle', 'Unsaved Changes')"
    @update:open="(v) => { if (!v) emit('stay') }"
  >
    <p>{{ $t('common.unsavedChangesDesc', 'You have unsaved changes. Are you sure you want to leave? Your changes will be lost.') }}</p>
    <template #footer>
      <button type="button" class="outline" @click="emit('stay')">{{ $t('common.stay', 'Stay') }}</button>
      <button type="button" @click="emit('leave')">{{ $t('common.leave', 'Leave') }}</button>
    </template>
  </Modal>
</template>
