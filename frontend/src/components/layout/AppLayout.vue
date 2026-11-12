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

// On narrow screens the sidebar is an overlay: close it after navigating.
const mobileQuery = window.matchMedia('(max-width: 768px)')
const isMobile = ref(mobileQuery.matches)
const sidebarFlag = ref(false)
// Oat sets data-sidebar-open: it means "overlay shown" on mobile but "collapsed" on desktop.
const sidebarExpanded = computed(() => (isMobile.value ? sidebarFlag.value : !sidebarFlag.value))
const onMediaChange = (e: MediaQueryListEvent) => { isMobile.value = e.matches }
let observer: MutationObserver | null = null
onMounted(() => {
  mobileQuery.addEventListener('change', onMediaChange)
  if (layout.value) {
    sidebarFlag.value = layout.value.hasAttribute('data-sidebar-open')
    observer = new MutationObserver(() => {
      sidebarFlag.value = layout.value?.hasAttribute('data-sidebar-open') ?? false
    })
    observer.observe(layout.value, { attributes: true, attributeFilter: ['data-sidebar-open'] })
  }
})
onUnmounted(() => {
  mobileQuery.removeEventListener('change', onMediaChange)
  observer?.disconnect()
})

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
  <div ref="layout" data-sidebar-layout="always">
    <header data-topnav>
      <!-- Skip link for accessibility -->
      <a href="#main-content" class="skip-link">{{ $t('nav.skipToMain') }}</a>
      <button type="button" data-sidebar-toggle :aria-label="t('nav.toggleMenu')" :aria-expanded="sidebarExpanded">
        <Menu :size="16" aria-hidden="true" />
      </button>
      <RouterLink to="/" class="hstack gap-2">
        <MessageSquare :size="16" aria-hidden="true" />
        <strong>Whatomate</strong>
      </RouterLink>
    </header>

    <aside data-sidebar :aria-label="t('nav.sidebar')">
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
