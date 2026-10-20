<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import QRCode from 'qrcode'

const props = defineProps<{ code: string }>()
const { t } = useI18n()
const source = ref('')
const failed = ref(false)

watch(() => props.code, async (code, _, onCleanup) => {
  let canceled = false
  onCleanup(() => { canceled = true })
  source.value = ''
  failed.value = false
  if (!code) return
  try {
    const image = await QRCode.toDataURL(code, {
      errorCorrectionLevel: 'M', margin: 4, width: 320,
      color: { dark: '#000000ff', light: '#ffffffff' },
    })
    if (!canceled) source.value = image
  } catch {
    if (!canceled) failed.value = true
  }
}, { immediate: true })
</script>

<template>
  <div class="space-y-2">
    <template v-if="source">
      <p class="text-sm text-muted-foreground">{{ t('saas.pix_scan') }}</p>
      <img :src="source" :alt="t('saas.pix_qr_alt')" width="320" height="320" class="max-w-full h-auto rounded bg-white" />
    </template>
    <p v-else-if="failed" role="status" class="text-sm text-destructive">{{ t('saas.pix_qr_failed') }}</p>
  </div>
</template>
