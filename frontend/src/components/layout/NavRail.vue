<script setup lang="ts">
import { ref, computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { navigationSections } from './navigation'
import {
  Popover,
  PopoverContent,
  PopoverTrigger
} from '@/components/ui/popover'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Settings, ChevronRight, ChevronLeft } from 'lucide-vue-next'

const route = useRoute()
const authStore = useAuthStore()
const { t } = useI18n()

const isExpanded = ref(false)

const toggleExpand = () => {
  isExpanded.value = !isExpanded.value
}

// Filter sections based on permissions
const activeSections = computed(() => {
  return navigationSections.filter(section => 
    section.permissions.some(p => authStore.hasPermission(p, 'read'))
  ).map(section => ({
    ...section,
    items: section.items.filter(item => 
      !item.permission || authStore.hasPermission(item.permission, 'read')
    )
  })).filter(section => section.items.length > 0)
})

const mainSections = computed(() => activeSections.value.filter(s => !s.pinBottom))
const settingsSection = computed(() => activeSections.value.find(s => s.pinBottom))

const isItemActive = (item: any) => {
  if (item.path === '/') return route.name === 'dashboard'
  if (item.path === '/chat') return route.path === '/chat' || route.path.startsWith('/chat/')
  return route.path.startsWith(item.path)
}


</script>

<template>
  <nav 
    :class="[
      'border-r border-white/5 bg-background/50 backdrop-blur-3xl flex flex-col pt-4 gap-2 z-40 transition-all duration-300 ease-in-out relative group/rail shrink-0',
      isExpanded ? 'w-64 px-4' : 'w-16 items-center px-0'
    ]"
  >
    <!-- Toggle Button -->
    <Button
      variant="ghost"
      size="icon"
      class="h-7 w-7 absolute -right-3.5 top-6 bg-black border border-white/10 shadow-xl rounded-full z-50 hover:bg-zinc-900 hover:text-primary transition-all opacity-0 group-hover/rail:opacity-100 focus-visible:opacity-100"
      :class="isExpanded ? 'opacity-100' : ''"
      @click="toggleExpand"
    >
      <ChevronRight v-if="!isExpanded" class="h-3 w-3" />
      <ChevronLeft v-else class="h-3 w-3" />
    </Button>

    <!-- Main Navigation Sections -->
    <div class="flex-1 flex flex-col gap-1 overflow-y-auto no-scrollbar pb-4 pt-4">
      <template v-for="section in mainSections" :key="section.label">
        <div v-for="item in section.items" :key="item.path" class="w-full flex justify-center px-1">
          <!-- Collapsed: Popover Flyout -->
          <Popover v-if="!isExpanded && item.children && item.children.length > 0">
            <PopoverTrigger as-child>
              <Button
                variant="ghost"
                size="icon"
                :class="[
                  'h-10 w-10 transition-all rounded-xl hover:bg-white/5 shrink-0 relative group/btn',
                  isItemActive(item) ? 'bg-primary/10 text-primary border border-primary/20' : 'text-muted-foreground'
                ]"
              >
                <component :is="item.icon" class="h-5 w-5 transition-transform group-hover/btn:scale-110" />
                <div v-if="isItemActive(item)" class="absolute -left-1 top-2 bottom-2 w-0.5 bg-primary rounded-full shadow-[0_0_8px_rgba(16,185,129,0.5)]"></div>
              </Button>
            </PopoverTrigger>
            <PopoverContent side="right" align="start" class="w-56 p-1.5 ml-3 shadow-3xl border-white/10 bg-black/95 backdrop-blur-xl rounded-xl">
              <div class="px-3 py-2 text-[9px] font-black text-primary uppercase tracking-[0.2em] opacity-60">
                {{ t(item.name) }}
              </div>
              <Separator class="bg-white/5 mb-1.5" />
              <div class="space-y-0.5">
                <RouterLink
                  v-for="child in item.children"
                  :key="child.path"
                  :to="child.path"
                  class="flex items-center gap-3 px-3 py-2 text-[13px] font-bold rounded-lg transition-all group/child"
                  :class="route.path === child.path ? 'bg-primary/10 text-primary' : 'text-muted-foreground hover:bg-white/5 hover:text-foreground'"
                >
                  <component :is="child.icon" class="h-4 w-4 shrink-0 transition-colors" :class="route.path === child.path ? 'text-primary' : 'text-muted-foreground/50 group-hover/child:text-primary'" />
                  <span>{{ t(child.name) }}</span>
                </RouterLink>
              </div>
            </PopoverContent>
          </Popover>

          <!-- Expanded Item with Children (Collapsible Section) -->
          <div v-else-if="isExpanded && item.children && item.children.length > 0" class="w-full space-y-1.5 py-1">
            <div class="flex items-center gap-3 px-3 py-2 text-[10px] font-black text-muted-foreground uppercase tracking-[0.25em] opacity-40">
              {{ t(item.name) }}
            </div>
            <div class="space-y-1">
              <RouterLink
                v-for="child in item.children"
                :key="child.path"
                :to="child.path"
                class="flex items-center gap-3 px-3 py-2 text-[13px] font-bold rounded-xl transition-all group/expand"
                :class="route.path === child.path ? 'bg-primary text-primary-foreground shadow-xl shadow-primary/20' : 'text-muted-foreground hover:bg-white/5 hover:text-foreground'"
              >
                <component :is="child.icon" class="h-4.5 w-4.5 shrink-0 transition-transform group-hover/expand:scale-110" :class="route.path === child.path ? 'text-primary-foreground' : 'text-muted-foreground/60 group-hover/expand:text-primary'" />
                <span class="tracking-tight">{{ t(child.name) }}</span>
              </RouterLink>
            </div>
          </div>

          <!-- Collapsed Standard Link -->
          <RouterLink 
            v-else-if="!isExpanded"
            :to="item.path"
            class="shrink-0"
          >
            <Button
              variant="ghost"
              size="icon"
              :class="[
                'h-10 w-10 transition-all rounded-xl hover:bg-white/5 relative group/btn',
                isItemActive(item) ? 'bg-primary/10 text-primary border border-primary/20' : 'text-muted-foreground'
              ]"
            >
              <component :is="item.icon" class="h-5 w-5 transition-transform group-hover/btn:scale-110" />
              <div v-if="isItemActive(item)" class="absolute -left-1 top-2 bottom-2 w-0.5 bg-primary rounded-full shadow-[0_0_8px_rgba(16,185,129,0.5)]"></div>
            </Button>
          </RouterLink>

          <!-- Expanded Standard Link -->
          <RouterLink 
            v-else 
            :to="item.path"
            class="block w-full group/main"
          >
            <div
              :class="[
                'flex items-center gap-3 px-4 py-2.5 text-[14px] font-bold rounded-xl transition-all',
                isItemActive(item) 
                  ? 'bg-primary text-primary-foreground shadow-xl shadow-primary/20' 
                  : 'text-muted-foreground group-hover/main:bg-white/5 group-hover/main:text-foreground'
              ]"
            >
              <component :is="item.icon" :class="['h-5 w-5 transition-transform group-hover/main:scale-110', isItemActive(item) ? 'text-primary-foreground' : 'text-muted-foreground/60 group-hover/main:text-primary']" />
              <span class="truncate tracking-tight">{{ t(item.name) }}</span>
            </div>
          </RouterLink>
        </div>
      </template>
    </div>

    <!-- Bottom Settings Section (Dedicated Setup Link) -->
    <div v-if="settingsSection" class="mt-auto shrink-0 pb-4 w-full">
      <Separator class="bg-white/5 w-8 mx-auto mb-4" :class="isExpanded ? 'w-full' : ''" />
      
      <div class="w-full flex justify-center px-1">
        <RouterLink 
          to="/settings"
          class="block w-full"
        >
          <Button
            variant="ghost"
            :class="[
              'h-11 transition-all rounded-xl hover:bg-white/5 shrink-0 group/settings',
              isExpanded ? 'w-full justify-start px-4 gap-3' : 'w-10 justify-center',
              route.path.startsWith('/settings') ? 'bg-orange-500 text-white shadow-xl shadow-orange-500/20' : 'text-muted-foreground'
            ]"
          >
            <Settings class="h-5 w-5 shrink-0 transition-transform group-hover/settings:rotate-90" :class="route.path.startsWith('/settings') ? 'text-white' : 'text-muted-foreground/60 group-hover/settings:text-primary'" />
            <span v-if="isExpanded" class="truncate text-[14px] font-bold tracking-tight transition-all">{{ t('nav.settings') }}</span>
            <ChevronRight v-if="isExpanded" class="ml-auto h-3 w-3 opacity-20" />
          </Button>
        </RouterLink>
      </div>
    </div>
  </nav>
</template>

<style scoped>
.no-scrollbar::-webkit-scrollbar {
  display: none;
}
.no-scrollbar {
  -ms-overflow-style: none;
  scrollbar-width: none;
}
</style>
