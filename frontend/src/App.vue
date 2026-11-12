<script setup lang="ts">
import { RouterView } from 'vue-router'
import { useColorMode } from '@/composables/useColorMode'

// Initialize color mode early
useColorMode()
</script>

<template>
  <RouterView />
</template>
