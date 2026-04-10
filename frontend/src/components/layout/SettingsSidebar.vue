<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { ScrollArea } from '@/components/ui/scroll-area'
import { navigationSections } from './navigation'

const route = useRoute()
const { t } = useI18n()
const authStore = useAuthStore()

// Filter for the settings section (usually pinned to bottom)
const settingsMainItem = computed(() => {
  const section = navigationSections.find(s => s.pinBottom)
  return section?.items.find(i => i.path === '/settings')
})

const groups = computed(() => {
  if (!settingsMainItem.value?.children) return []
  
  const items = settingsMainItem.value.children
  
  // Grouping logic for Settings Sidebar
  return [
    {
      label: 'setup.general',
      items: items.filter(i => ['/settings', '/settings/canned-responses', '/settings/tags'].includes(i.path))
    },
    {
      label: 'setup.channels',
      items: items.filter(i => ['/settings/accounts', '/settings/chatbot', '/settings/webhooks', '/settings/api-keys'].includes(i.path))
    },
    {
      label: 'setup.usersControl',
      items: items.filter(i => ['/settings/users', '/settings/roles', '/settings/teams'].includes(i.path))
    },
    {
      label: 'setup.system',
      items: items.filter(i => ['/settings/sso', '/settings/audit-logs', '/settings/custom-actions'].includes(i.path))
    }
  ]
})

// Filter out items the user doesn't have permission for
const filteredGroups = computed(() => {
  return groups.value.map(group => ({
    ...group,
    items: group.items.filter(item => 
      !item.permission || authStore.hasPermission(item.permission, 'read')
    )
  })).filter(group => group.items.length > 0)
})

const isActive = (path: string) => {
  if (path === '/settings') return route.path === '/settings'
  return route.path.startsWith(path)
}
</script>

<template>
  <aside class="w-60 border-r border-white/5 bg-background flex flex-col h-full shrink-0 animate-in slide-in-from-left duration-300">
    <ScrollArea class="flex-1">
      <div class="p-3 space-y-6 pt-6">
        <div v-for="group in filteredGroups" :key="group.label" class="space-y-1.5">
          <div class="px-3 py-1 text-[9px] font-bold text-muted-foreground/30 uppercase tracking-[0.2em] mb-1">
            {{ t(group.label) }}
          </div>
          <div class="space-y-0.5">
            <RouterLink
              v-for="item in group.items"
              :key="item.path"
              :to="item.path"
              class="flex items-center gap-3 px-3 py-2 text-[13px] font-bold rounded-lg transition-all group/item overflow-hidden relative nav-active-indicator"
              :data-active="isActive(item.path)"
              :class="isActive(item.path) 
                ? 'bg-primary/10 text-primary' 
                : 'text-muted-foreground hover:bg-white/5 hover:text-foreground'"
            >
              <component :is="item.icon" class="h-4 w-4 shrink-0 transition-all relative z-10" :class="isActive(item.path) ? 'text-primary scale-105' : 'text-muted-foreground/40 group-hover/item:text-primary group-hover/item:scale-105'" />
              <span class="truncate leading-none relative z-10 tracking-tight">{{ t(item.name) }}</span>
            </RouterLink>
          </div>
        </div>
      </div>
    </ScrollArea>
  </aside>
</template>

<style scoped>
:deep(.lucide) {
  stroke-width: 2.2px;
}
</style>
