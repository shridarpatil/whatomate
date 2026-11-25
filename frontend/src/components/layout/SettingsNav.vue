<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { navigationSections, settingsSections } from './navigation'

/** Grouped menu shown beside every Settings page (replaces the sidebar sub-list). */
const route = useRoute()
const authStore = useAuthStore()

const settingsItems = navigationSections.flatMap(s => s.items).find(i => i.path === '/settings')?.children ?? []

const groups = computed(() => settingsSections
  .map(g => ({
    label: g.label,
    items: g.paths
      .map(p => settingsItems.find(i => i.path === p))
      .filter((i): i is NonNullable<typeof i> => !!i && (!i.permission || authStore.hasPermission(i.permission, 'read'))),
  }))
  .filter(g => g.items.length > 0))

const isCurrent = (path: string) =>
  path === '/settings' ? route.path === '/settings' : route.path === path || route.path.startsWith(`${path}/`)
</script>

<template>
  <nav data-subnav :aria-label="$t('nav.settingsMenu')">
    <template v-for="g in groups" :key="g.label">
      <small>{{ $t(g.label) }}</small>
      <RouterLink
        v-for="item in g.items"
        :key="item.path"
        :to="item.path"
        :aria-current="isCurrent(item.path) ? 'page' : undefined"
      >
        <component :is="item.icon" :size="14" aria-hidden="true" />
        <span>{{ $t(item.name) }}</span>
      </RouterLink>
    </template>
  </nav>
</template>
