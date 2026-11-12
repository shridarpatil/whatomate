<script setup lang="ts">
import { RouterView } from 'vue-router'
import { useColorMode } from '@/composables/useColorMode'
// TEMPORARY bridge: unmigrated screens (ChatView, CallButton, AgentTransfersView)
// still use the shadcn Tooltip, which needs this provider. Remove in Task 19
// together with components/ui.
import { TooltipProvider } from '@/components/ui/tooltip'

// Initialize color mode early
useColorMode()
</script>

<template>
  <TooltipProvider>
    <RouterView />
  </TooltipProvider>
</template>
