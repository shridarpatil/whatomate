<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterView } from 'vue-router'
import { Toaster } from 'vue-sonner'
import { TooltipProvider } from '@/components/ui/tooltip'
import { useColorMode } from '@/composables/useColorMode'
import { startVersionWatch } from '@/services/appVersion'

// Initialize color mode early
const { colorMode } = useColorMode()

// Offer a reload once the server starts serving a newer build than this tab is
// running. Mounted here rather than in a layout so tabs parked on the login
// screen are covered too — they go stale the same way.
onMounted(startVersionWatch)
</script>

<template>
  <TooltipProvider>
    <div class="min-h-screen bg-background font-sans antialiased">
      <RouterView />
      <Toaster position="top-right" richColors :theme="colorMode" offset="72px" />
    </div>
  </TooltipProvider>
</template>
