<script setup lang="ts">
import { ref } from 'vue'
import { Upload, FileText, X } from '@/lib/icons'

defineProps<{
  file: File | null
  previewUrl: string | null
  acceptTypes: string
  mediaLabel?: string
  label?: string
}>()

defineEmits<{
  (e: 'change', event: Event): void
  (e: 'clear'): void
}>()

const fileInput = ref<HTMLInputElement | null>(null)
</script>

<template>
  <div class="vstack gap-1">
    <strong v-if="label">{{ label }}</strong>
    <button v-if="!file" type="button" class="outline" @click="fileInput?.click()">
      <Upload />
      {{ $t('common.clickToUpload') }}
    </button>
    <small v-if="!file && mediaLabel" class="text-light">{{ mediaLabel }}</small>
    <div v-else-if="file" class="hstack gap-2">
      <img v-if="previewUrl" :src="previewUrl" :alt="file.name" width="48" height="48">
      <FileText v-else :size="32" />
      <span class="truncate flex-1">{{ file.name }}</span>
      <button type="button" class="ghost small icon" :title="$t('common.remove')" :aria-label="$t('common.remove')" @click="$emit('clear')">
        <X />
      </button>
    </div>
    <input ref="fileInput" type="file" :accept="acceptTypes" hidden @change="$emit('change', $event)">
  </div>
</template>
