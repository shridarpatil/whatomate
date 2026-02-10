<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  MessageSquare,
  ChevronLeft,
  ChevronRight,
  Menu,
  X
} from 'lucide-vue-next'
import { wsService } from '@/services/websocket'
import { authService } from '@/services/api'
import OrganizationSwitcher from './OrganizationSwitcher.vue'
import UserMenu from './UserMenu.vue'
import { navigationItems } from './navigation'

useI18n() // Enable $t() in template

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const isCollapsed = ref(false)
const isMobileMenuOpen = ref(false)

// Connect WebSocket on mount using short-lived WS token
onMounted(() => {
  if (authStore.isAuthenticated) {
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

// Filter navigation based on user permissions
const navigation = computed(() => {
  return navigationItems
    .filter(item => {
      if (item.childPermissions) {
        return item.childPermissions.some(p => authStore.hasPermission(p, 'read'))
      }
      return !item.permission || authStore.hasPermission(item.permission, 'read')
    })
    .map(item => {
      const filteredChildren = item.children?.filter(
        child => !child.permission || authStore.hasPermission(child.permission, 'read')
      )

      let effectivePath = item.path
      if (item.childPermissions && item.permission && !authStore.hasPermission(item.permission, 'read') && filteredChildren?.length) {
        effectivePath = filteredChildren[0].path
      }

      const originalPath = item.path
      const isActive = originalPath === '/'
        ? route.name === 'dashboard'
        : originalPath === '/chat'
          ? route.name === 'chat' || route.name === 'chat-conversation'
          : route.path.startsWith(originalPath)

      return {
        ...item,
        path: effectivePath,
        active: isActive,
        children: filteredChildren
      }
    })
})

const toggleSidebar = () => {
  isCollapsed.value = !isCollapsed.value
}

const handleLogout = async () => {
  await authStore.logout()
  router.push('/login')
}
</script>

<template>
  <div class="flex h-screen bg-[#0a0a0b] light:bg-gray-50">
    <!-- Skip link for accessibility -->
    <a href="#main-content" class="skip-link">{{ $t('nav.skipToMain') }}</a>

    <!-- Mobile header -->
    <header class="fixed top-0 left-0 right-0 z-50 flex h-12 items-center justify-between border-b border-white/[0.08] light:border-gray-200 bg-[#0a0a0b]/95 light:bg-white/95 backdrop-blur-sm px-3 md:hidden">
      <RouterLink to="/" class="flex items-center gap-2">
        <div class="h-7 w-7 rounded-lg bg-gradient-to-br from-emerald-500 to-green-600 flex items-center justify-center shadow-lg shadow-emerald-500/20">
          <MessageSquare class="h-4 w-4 text-white" />
        </div>
        <span class="font-semibold text-sm text-white light:text-gray-900">Whatomate</span>
      </RouterLink>
      <Button
        variant="ghost"
        size="icon"
        class="h-8 w-8 text-white/70 hover:text-white hover:bg-white/[0.08] light:text-gray-600 light:hover:text-gray-900 light:hover:bg-gray-100"
        aria-label="Toggle menu"
        :aria-expanded="isMobileMenuOpen"
        @click="isMobileMenuOpen = !isMobileMenuOpen"
      >
        <X v-if="isMobileMenuOpen" class="h-5 w-5" />
        <Menu v-else class="h-5 w-5" />
      </Button>
    </header>

    <!-- Mobile menu overlay -->
    <div
      v-if="isMobileMenuOpen"
      class="fixed inset-0 z-40 bg-black/60 light:bg-black/30 backdrop-blur-sm md:hidden"
      @click="isMobileMenuOpen = false"
    />

    <!-- Sidebar -->
    <aside
      :class="[
        'flex flex-col border-r border-white/[0.08] light:border-gray-200 bg-[#0a0a0b] light:bg-white transition-all duration-300',
        'fixed inset-y-0 left-0 z-40 md:relative',
        'transform md:transform-none',
        isMobileMenuOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0',
        isCollapsed ? 'w-64 md:w-16' : 'w-64'
      ]"
      role="navigation"
      aria-label="Main navigation"
    >
      <!-- Logo (hidden on mobile, shown in header instead) -->
      <div class="hidden md:flex h-12 items-center justify-between px-3 border-b border-white/[0.08] light:border-gray-200">
        <RouterLink to="/" class="flex items-center gap-2">
          <div class="h-7 w-7 rounded-lg bg-gradient-to-br from-emerald-500 to-green-600 flex items-center justify-center shadow-lg shadow-emerald-500/20">
            <MessageSquare class="h-4 w-4 text-white" />
          </div>
          <span
            v-if="!isCollapsed"
            class="font-semibold text-sm text-white light:text-gray-900"
          >
            Whatomate
          </span>
        </RouterLink>
        <Button
          variant="ghost"
          size="icon"
          class="h-7 w-7 text-white/50 hover:text-white hover:bg-white/[0.08] light:text-gray-400 light:hover:text-gray-900 light:hover:bg-gray-100"
          :aria-label="isCollapsed ? $t('nav.expandSidebar') : $t('nav.collapseSidebar')"
          :aria-expanded="!isCollapsed"
          @click="toggleSidebar"
        >
          <ChevronLeft v-if="!isCollapsed" class="h-3.5 w-3.5" />
          <ChevronRight v-else class="h-3.5 w-3.5" />
        </Button>
      </div>
      <!-- Mobile logo spacer -->
      <div class="h-12 md:hidden" />

      <!-- Organization Switcher (Super Admin only) -->
      <OrganizationSwitcher :collapsed="isCollapsed" />

      <!-- Navigation -->
      <ScrollArea class="flex-1 py-2">
        <nav class="space-y-0.5 px-2" role="menubar">
          <template v-for="item in navigation" :key="item.path">
            <RouterLink
              :to="item.path"
              :class="[
                'flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-[13px] font-medium transition-all duration-200',
                item.active
                  ? 'bg-white/[0.08] text-white light:bg-gray-100 light:text-gray-900'
                  : 'text-white/50 hover:text-white hover:bg-white/[0.04] light:text-gray-500 light:hover:text-gray-900 light:hover:bg-gray-50',
                isCollapsed && 'md:justify-center md:px-2'
              ]"
              role="menuitem"
              :aria-current="item.active ? 'page' : undefined"
              @click="isMobileMenuOpen = false"
            >
              <component :is="item.icon" class="h-4 w-4 shrink-0" aria-hidden="true" />
              <span :class="isCollapsed && 'md:sr-only'">{{ $t(item.name) }}</span>
            </RouterLink>

            <!-- Submenu items -->
            <template v-if="item.children && item.active && !isCollapsed">
              <RouterLink
                v-for="child in item.children"
                :key="child.path"
                :to="child.path"
                :class="[
                  'flex items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-[13px] font-medium transition-all duration-200 ml-4',
                  route.path === child.path
                    ? 'bg-white/[0.06] text-white light:bg-gray-100 light:text-gray-900'
                    : 'text-white/40 hover:text-white/70 hover:bg-white/[0.04] light:text-gray-400 light:hover:text-gray-700 light:hover:bg-gray-50'
                ]"
                role="menuitem"
                :aria-current="route.path === child.path ? 'page' : undefined"
                @click="isMobileMenuOpen = false"
              >
                <component :is="child.icon" class="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                <span>{{ $t(child.name) }}</span>
              </RouterLink>
            </template>
          </template>
        </nav>
      </ScrollArea>

      <!-- User Menu -->
      <UserMenu :collapsed="isCollapsed" @logout="handleLogout" />
    </aside>

    <!-- Main content -->
    <main id="main-content" class="flex-1 overflow-hidden pt-12 md:pt-0 bg-[#0a0a0b] light:bg-gray-50" role="main">
      <RouterView />
    </main>
  </div>
</template>
