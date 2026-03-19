<script setup lang="ts">
import { Button } from '@/components/ui/button'
import { Upload, FileText, X } from 'lucide-vue-next'

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
</script>

<template>
  <div class="space-y-1">
    <label v-if="label" class="text-sm font-medium">{{ label }}</label>
    <div v-if="!file" class="border-2 border-dashed rounded-lg p-4 text-center cursor-pointer hover:border-primary/50 transition-colors" @click="($refs.fileInput as HTMLInputElement)?.click()">
      <Upload class="h-6 w-6 mx-auto text-muted-foreground mb-1" />
      <p class="text-xs text-muted-foreground">Click to upload file</p>
      <p v-if="mediaLabel" class="text-xs text-muted-foreground mt-0.5">{{ mediaLabel }}</p>
    </div>
    <div v-else class="flex items-center gap-2 p-2 bg-muted rounded-lg">
      <img v-if="previewUrl" :src="previewUrl" class="h-12 w-12 object-cover rounded" />
      <FileText v-else class="h-8 w-8 text-muted-foreground shrink-0" />
      <span class="text-sm truncate flex-1">{{ file.name }}</span>
      <Button variant="ghost" size="icon" class="h-6 w-6 shrink-0" @click="$emit('clear')">
        <X class="h-4 w-4" />
      </Button>
    </div>
    <input ref="fileInput" type="file" :accept="acceptTypes" class="hidden" @change="$emit('change', $event)" />
  </div>
</template>
