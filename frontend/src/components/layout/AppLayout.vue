<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { wsService } from '@/services/websocket'
import { authService } from '@/services/api'
import TopBar from './TopBar.vue'
import NavRail from './NavRail.vue'
import SettingsSidebar from './SettingsSidebar.vue'
import ActiveCallPanel from '@/components/calling/ActiveCallPanel.vue'
import { ScrollToTop } from '@/components/shared'

const route = useRoute()
const authStore = useAuthStore()

// Setup Mode Logic
const isSettingsMode = computed(() => route.path.startsWith('/settings'))
const isFullWidthPage = computed(() => ['/notifications', '/error'].includes(route.path))

// Refresh user data and connect WebSocket on mount
onMounted(() => {
  if (authStore.isAuthenticated) {
    authStore.refreshUserData()

    wsService.connect(async () => {
      try {
        const resp = await authService.getWSToken()
        return resp.data.data.token
      } catch {
        return null
      }
    })
  }
})
</script>

<template>
  <div class="h-screen flex flex-col overflow-hidden bg-background text-foreground font-sans selection:bg-primary/20">
    <TopBar class="h-14 bg-background/95 backdrop-blur-md border-b border-white/5 z-[60]" />

    <!-- Skip link for accessibility -->
    <a href="#main-content"
      class="sr-only focus:not-sr-only focus:absolute focus:top-4 focus:left-4 focus:z-[100] focus:px-4 focus:py-2 focus:bg-primary focus:text-primary-foreground focus:rounded-md">
      Skip to main content
    </a>

    <div class="flex-1 flex overflow-hidden relative">
      <!-- Left Navigation: High-Density Rail or Settings Sidebar -->
      <NavRail v-if="!isSettingsMode && !isFullWidthPage"
        class="hidden md:flex shrink-0 border-r border-border bg-background/50 backdrop-blur-xl" />
      <SettingsSidebar v-if="isSettingsMode && !isFullWidthPage"
        class="hidden md:flex shrink-0 border-r border-border bg-background/50 backdrop-blur-xl" />

      <!-- Main Workspace Area -->
      <main id="main-content"
        class="flex-1 flex flex-col relative overflow-hidden bg-background/20 transition-all duration-300" role="main">
        <div class="flex-1 overflow-auto relative custom-scrollbar">
          <RouterView v-slot="{ Component, route: r }">
            <Transition name="page" mode="out-in">
              <KeepAlive :max="10">
                <component :is="Component" :key="r.path" />
              </KeepAlive>
            </Transition>
          </RouterView>
        </div>

        <!-- Active Panels & UI Helpers -->
        <ActiveCallPanel />
        <ScrollToTop />
      </main>
    </div>
  </div>
</template>

<style scoped>
.page-enter-active,
.page-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.page-enter-from {
  opacity: 0;
  transform: translateY(4px);
}

.page-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

.custom-scrollbar::-webkit-scrollbar {
  width: 6px;
}

.custom-scrollbar::-webkit-scrollbar-thumb {
  background: transparent;
  border-radius: 10px;
}

.custom-scrollbar:hover::-webkit-scrollbar-thumb {
  background: rgba(var(--primary), 0.1);
}
</style>
