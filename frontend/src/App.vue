<script setup lang="ts">
import { ref } from 'vue'
import { RouterView } from 'vue-router'
import { Toaster } from 'vue-sonner'
import { TooltipProvider } from '@/components/ui/tooltip'
import { useColorMode } from '@/composables/useColorMode'
import AppTour from '@/components/shared/AppTour.vue'

// Initialize color mode early
const { colorMode } = useColorMode()

// Tour ref — exposed to AppLayout via provide so UserMenu can trigger it
const tourRef = ref<InstanceType<typeof AppTour> | null>(null)

import { provide } from 'vue'
provide('openTour', () => tourRef.value?.open())
</script>

<template>
  <TooltipProvider>
    <div class="min-h-screen bg-background font-sans antialiased">
      <RouterView />
      <Toaster position="top-right" richColors :theme="colorMode" offset="72px" />
      <!-- Quick Start Guide — auto-starts on first login, triggered via UserMenu -->
      <AppTour ref="tourRef" />
    </div>
  </TooltipProvider>
</template>
