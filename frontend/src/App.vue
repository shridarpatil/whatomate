<script setup lang="ts">
import { computed } from 'vue'
import { RouterView } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Toaster } from 'vue-sonner'
import { ConfigProvider } from 'reka-ui'
import { TooltipProvider } from '@/components/ui/tooltip'
import { useColorMode } from '@/composables/useColorMode'

// Initialize color mode early
const { colorMode } = useColorMode()

const { locale } = useI18n()
// RTL locales — keep in sync with index.html and i18n/index.ts
const RTL_LOCALES = new Set(['ar', 'he', 'fa', 'ur'])
const dir = computed(() => RTL_LOCALES.has(locale.value) ? 'rtl' as const : 'ltr' as const)
const toastPosition = computed(() => dir.value === 'rtl' ? 'top-left' as const : 'top-right' as const)
</script>

<template>
  <ConfigProvider :dir="dir">
    <TooltipProvider>
      <div class="min-h-screen bg-background font-sans antialiased">
        <RouterView />
        <Toaster :position="toastPosition" richColors :theme="colorMode" :dir="dir" />
      </div>
    </TooltipProvider>
  </ConfigProvider>
</template>
