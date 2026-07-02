<script setup lang="ts">
import { RouterView } from 'vue-router'
import { Toaster } from 'vue-sonner'
import { TooltipProvider } from '@/components/ui/tooltip'
import { useColorMode } from '@/composables/useColorMode'
import AppTour from '@/components/shared/AppTour.vue'

// Initialize color mode early
const { colorMode } = useColorMode()
</script>

<template>
  <TooltipProvider>
    <div class="min-h-screen bg-background font-sans antialiased">
      <RouterView />
      <Toaster position="top-right" richColors :theme="colorMode" offset="72px" />
      <!-- Quick Start Guide — auto-starts on first login, re-openable via ? button -->
      <AppTour />
    </div>
  </TooltipProvider>
</template>
