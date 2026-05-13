import { ref, computed, type Ref } from 'vue'

/**
 * Composable for handling template header media (IMAGE/VIDEO/DOCUMENT) uploads.
 * Shared between ChatView (template sending) and CampaignsView (campaign media).
 */
export function useHeaderMedia(headerType: Ref<string | undefined>) {
  const file = ref<File | null>(null)
  const previewUrl = ref<string | null>(null)

  const needsMedia = computed(() => {
    const ht = headerType.value
    return ht === 'IMAGE' || ht === 'VIDEO' || ht === 'DOCUMENT'
  })

  const acceptTypes = computed(() => {
    switch (headerType.value) {
      case 'IMAGE': return 'image/jpeg,image/png,image/webp'
      case 'VIDEO': return 'video/mp4,video/3gpp'
      case 'DOCUMENT': return '.pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.txt'
      default: return '*/*'
    }
  })

  const mediaLabel = computed(() => {
    switch (headerType.value) {
      case 'IMAGE': return 'JPEG, PNG, WebP'
      case 'VIDEO': return 'MP4, 3GPP'
      case 'DOCUMENT': return 'PDF, DOC, XLS, PPT'
      default: return ''
    }
  })

  function handleFileChange(event: Event) {
    const input = event.target as HTMLInputElement
    const selected = input.files?.[0]
    if (!selected) return
    file.value = selected
    if (selected.type.startsWith('image/')) {
      previewUrl.value = URL.createObjectURL(selected)
    } else {
      previewUrl.value = null
    }
  }

  function clear() {
    file.value = null
    if (previewUrl.value) {
      URL.revokeObjectURL(previewUrl.value)
      previewUrl.value = null
    }
  }

  return {
    file,
    previewUrl,
    needsMedia,
    acceptTypes,
    mediaLabel,
    handleFileChange,
    clear,
  }
}
