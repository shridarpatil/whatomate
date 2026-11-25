<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { navigationSections } from './navigation'

/** Tabs across the chatbot pages (replaces the sidebar sub-list). */
const route = useRoute()
const authStore = useAuthStore()

const tabs = computed(() =>
  (navigationSections.flatMap(s => s.items).find(i => i.path === '/chatbot')?.children ?? [])
    .filter(i => !i.permission || authStore.hasPermission(i.permission, 'read')))
</script>

<template>
  <nav data-section-tabs :aria-label="$t('nav.chatbot')">
    <RouterLink
      v-for="tab in tabs"
      :key="tab.path"
      :to="tab.path"
      :aria-current="route.path === tab.path ? 'page' : undefined"
    >{{ $t(tab.name) }}</RouterLink>
  </nav>
</template>
