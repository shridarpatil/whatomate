<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { MessageSquare, Menu } from '@/lib/icons'
import { wsService } from '@/services/websocket'
import { authService } from '@/services/api'
import OrganizationSwitcher from './OrganizationSwitcher.vue'
import UserMenu from './UserMenu.vue'
import ActiveCallPanel from '@/components/calling/ActiveCallPanel.vue'
import { ScrollToTop } from '@/components/shared'
import { navigationSections, type NavSection } from './navigation'

const { t } = useI18n()

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const layout = ref<HTMLElement | null>(null)

// Refresh user data and connect WebSocket on mount
onMounted(() => {
  if (authStore.isAuthenticated) {
    // Fetch fresh permissions in background (non-destructive — interceptor handles 401)
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

function filterItems(items: NavSection['items']) {
  return items
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
}

// Filter navigation sections based on user permissions
const navSections = computed(() => {
  return navigationSections
    .map(section => ({
      ...section,
      items: filterItems(section.items)
    }))
    .filter(section => section.items.length > 0)
})

const mainSections = computed(() => navSections.value.filter(s => !s.pinBottom))
const bottomSections = computed(() => navSections.value.filter(s => s.pinBottom))

// On phones the sidebar is an overlay: data-sidebar-open means it is shown. Close it after navigating.
const sidebarOpen = ref(false)
let observer: MutationObserver | null = null
onMounted(() => {
  if (layout.value) {
    sidebarOpen.value = layout.value.hasAttribute('data-sidebar-open')
    observer = new MutationObserver(() => {
      sidebarOpen.value = layout.value?.hasAttribute('data-sidebar-open') ?? false
    })
    observer.observe(layout.value, { attributes: true, attributeFilter: ['data-sidebar-open'] })
  }
})
onUnmounted(() => observer?.disconnect())

const closeMobileSidebar = () => {
  if (window.matchMedia('(max-width: 768px)').matches) {
    layout.value?.removeAttribute('data-sidebar-open')
  }
}

const handleLogout = async () => {
  await authStore.logout()
  router.push('/login')
}
</script>

<template>
  <div ref="layout" data-sidebar-layout>
    <!-- Skip link for accessibility -->
    <a href="#main-content" class="skip-link">{{ $t('nav.skipToMain') }}</a>

    <!-- Phones only: slim top bar with the menu toggle -->
    <header data-sidebar-header>
      <button type="button" data-sidebar-toggle :aria-label="t('nav.toggleMenu')" :aria-expanded="sidebarOpen">
        <Menu :size="16" aria-hidden="true" />
      </button>
      <RouterLink to="/" class="hstack gap-2">
        <MessageSquare :size="16" aria-hidden="true" />
        <strong>Whatomate</strong>
      </RouterLink>
    </header>

    <aside data-sidebar :aria-label="t('nav.sidebar')">
      <header>
        <RouterLink to="/" class="hstack gap-2">
          <MessageSquare :size="16" aria-hidden="true" />
          <strong>Whatomate</strong>
        </RouterLink>
      </header>

      <!-- Organization Switcher (Super Admin / multi-org only) -->
      <OrganizationSwitcher />

      <nav :aria-label="t('nav.mainNavigation')">
        <ul>
          <template v-for="section in mainSections" :key="section.label">
            <li v-if="section.label" role="presentation">
              <small class="text-lighter">{{ $t(section.label) }}</small>
            </li>
            <li v-for="item in section.items" :key="item.path">
              <RouterLink
                :to="item.path"
                :aria-current="item.active ? 'page' : undefined"
                @click="closeMobileSidebar"
              >
                <component :is="item.icon" :size="16" aria-hidden="true" />
                <span>{{ $t(item.name) }}</span>
              </RouterLink>

              <!-- Submenu items -->
              <ul v-if="item.children && item.active">
                <li v-for="child in item.children" :key="child.path">
                  <RouterLink
                    :to="child.path"
                    :aria-current="route.path === child.path ? 'page' : undefined"
                    @click="closeMobileSidebar"
                  >
                    <component :is="child.icon" :size="14" aria-hidden="true" />
                    <span>{{ $t(child.name) }}</span>
                  </RouterLink>
                </li>
              </ul>
            </li>
          </template>
        </ul>
      </nav>

      <!-- Bottom-pinned navigation (Settings) -->
      <nav v-if="bottomSections.length > 0" data-pinned :aria-label="t('nav.secondaryNavigation')">
        <ul>
          <template v-for="section in bottomSections" :key="section.label">
            <li v-for="item in section.items" :key="item.path">
              <RouterLink
                :to="item.path"
                :aria-current="item.active ? 'page' : undefined"
                @click="closeMobileSidebar"
              >
                <component :is="item.icon" :size="16" aria-hidden="true" />
                <span>{{ $t(item.name) }}</span>
              </RouterLink>

              <ul v-if="item.children && item.active">
                <li v-for="child in item.children" :key="child.path">
                  <RouterLink
                    :to="child.path"
                    :aria-current="route.path === child.path ? 'page' : undefined"
                    @click="closeMobileSidebar"
                  >
                    <component :is="child.icon" :size="14" aria-hidden="true" />
                    <span>{{ $t(child.name) }}</span>
                  </RouterLink>
                </li>
              </ul>
            </li>
          </template>
        </ul>
      </nav>

      <footer>
        <UserMenu @logout="handleLogout" />
      </footer>
    </aside>

    <!-- Main content -->
    <main id="main-content" class="scroll-y min-h-0">
      <RouterView v-slot="{ Component, route: viewRoute }">
        <Transition name="page" mode="out-in">
          <component :is="Component" :key="viewRoute.meta.stableKey ? String(viewRoute.name) : viewRoute.path" />
        </Transition>
      </RouterView>
      <ActiveCallPanel />
      <ScrollToTop />
    </main>
  </div>
</template>
