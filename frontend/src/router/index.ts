import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

// Permission-based route meta type
declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    permission?: string // Resource permission required (e.g., 'analytics', 'chat')
  }
}

// Get base path from server-injected config or fallback to Vite's BASE_URL
const basePath = (window as any).__BASE_PATH__ ?? import.meta.env.BASE_URL ?? '/'
const normalizedBasePath = basePath.endsWith('/') ? basePath : basePath + '/'

const router = createRouter({
  history: createWebHistory(normalizedBasePath),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/auth/LoginView.vue'),
      meta: { requiresAuth: false }
    },
    {
      path: '/register',
      name: 'register',
      component: () => import('@/views/auth/RegisterView.vue'),
      meta: { requiresAuth: false }
    },
    {
      path: '/auth/sso/callback',
      name: 'sso-callback',
      component: () => import('@/views/auth/SSOCallbackView.vue'),
      meta: { requiresAuth: false }
    },
    {
      path: '/',
      component: () => import('@/components/layout/AppLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        {
          path: '',
          name: 'dashboard',
          component: () => import('@/views/dashboard/DashboardView.vue'),
          meta: { permission: 'analytics' }
        },
        {
          path: 'chat',
          name: 'chat',
          component: () => import('@/views/chat/ChatView.vue'),
          meta: { permission: 'chat' }
        },
        {
          path: 'chat/:contactId',
          name: 'chat-conversation',
          component: () => import('@/views/chat/ChatView.vue'),
          props: true,
          meta: { permission: 'chat' }
        },
        {
          path: 'profile',
          name: 'profile',
          component: () => import('@/views/profile/ProfileView.vue')
          // All roles can access profile
        },
        {
          path: 'templates',
          name: 'templates',
          component: () => import('@/views/settings/TemplatesView.vue'),
          meta: { permission: 'templates' }
        },
        {
          path: 'flows',
          name: 'flows',
          component: () => import('@/views/settings/FlowsView.vue'),
          meta: { permission: 'flows.whatsapp' }
        },
        {
          path: 'campaigns',
          name: 'campaigns',
          component: () => import('@/views/settings/CampaignsView.vue'),
          meta: { permission: 'campaigns' }
        },
        {
          path: 'chatbot',
          name: 'chatbot',
          component: () => import('@/views/chatbot/ChatbotView.vue'),
          meta: { permission: 'settings.chatbot' }
        },
        {
          path: 'chatbot/settings',
          redirect: '/settings/chatbot'
        },
        {
          path: 'chatbot/keywords',
          name: 'chatbot-keywords',
          component: () => import('@/views/chatbot/KeywordsView.vue'),
          meta: { permission: 'chatbot.keywords' }
        },
        {
          path: 'chatbot/flows',
          name: 'chatbot-flows',
          component: () => import('@/views/chatbot/ChatbotFlowsView.vue'),
          meta: { permission: 'flows.chatbot' }
        },
        {
          path: 'chatbot/flows/new',
          name: 'chatbot-flow-new',
          component: () => import('@/views/chatbot/ChatbotFlowBuilderView.vue'),
          meta: { permission: 'flows.chatbot' }
        },
        {
          path: 'chatbot/flows/:id/edit',
          name: 'chatbot-flow-edit',
          component: () => import('@/views/chatbot/ChatbotFlowBuilderView.vue'),
          meta: { permission: 'flows.chatbot' }
        },
        {
          path: 'chatbot/ai',
          name: 'chatbot-ai',
          component: () => import('@/views/chatbot/AIContextsView.vue'),
          meta: { permission: 'chatbot.ai' }
        },
        {
          path: 'chatbot/transfers',
          name: 'chatbot-transfers',
          component: () => import('@/views/chatbot/AgentTransfersView.vue'),
          meta: { permission: 'transfers' }
        },
        {
          path: 'analytics/agents',
          name: 'agent-analytics',
          component: () => import('@/views/analytics/AgentAnalyticsView.vue'),
          meta: { permission: 'analytics.agents' }
        },
        {
          path: 'analytics/meta-insights',
          name: 'meta-insights',
          component: () => import('@/views/analytics/MetaInsightsView.vue'),
          meta: { permission: 'analytics' }
        },
        {
          path: 'settings',
          name: 'settings',
          component: () => import('@/views/settings/SettingsView.vue'),
          meta: { permission: 'settings.general' }
        },
        {
          path: 'settings/chatbot',
          name: 'chatbot-settings',
          component: () => import('@/views/settings/ChatbotSettingsView.vue'),
          meta: { permission: 'settings.chatbot' }
        },
        {
          path: 'settings/accounts',
          name: 'accounts',
          component: () => import('@/views/settings/AccountsView.vue'),
          meta: { permission: 'accounts' }
        },
        {
          path: 'settings/canned-responses',
          name: 'canned-responses',
          component: () => import('@/views/settings/CannedResponsesView.vue'),
          meta: { permission: 'canned_responses' }
        },
        {
          path: 'settings/contacts',
          name: 'contacts',
          component: () => import('@/views/settings/ContactsView.vue'),
          meta: { permission: 'contacts' }
        },
        {
          path: 'settings/tags',
          name: 'tags',
          component: () => import('@/views/settings/TagsView.vue'),
          meta: { permission: 'tags' }
        },
        {
          path: 'settings/users',
          name: 'users',
          component: () => import('@/views/settings/UsersView.vue'),
          meta: { permission: 'users' }
        },
        {
          path: 'settings/roles',
          name: 'roles',
          component: () => import('@/views/settings/RolesView.vue'),
          meta: { permission: 'roles' }
        },
        {
          path: 'settings/teams',
          name: 'teams',
          component: () => import('@/views/settings/TeamsView.vue'),
          meta: { permission: 'teams' }
        },
        {
          path: 'settings/api-keys',
          name: 'api-keys',
          component: () => import('@/views/settings/APIKeysView.vue'),
          meta: { permission: 'api_keys' }
        },
        {
          path: 'settings/webhooks',
          name: 'webhooks',
          component: () => import('@/views/settings/WebhooksView.vue'),
          meta: { permission: 'webhooks' }
        },
        {
          path: 'settings/sso',
          name: 'sso-settings',
          component: () => import('@/views/settings/SSOSettingsView.vue'),
          meta: { permission: 'settings.sso' }
        },
        {
          path: 'settings/custom-actions',
          name: 'custom-actions',
          component: () => import('@/views/settings/CustomActionsView.vue'),
          meta: { permission: 'custom_actions' }
        },
        {
          path: 'calling',
          redirect: '/calling/logs'
        },
        {
          path: 'calling/logs',
          name: 'call-logs',
          component: () => import('@/views/calling/CallLogsView.vue'),
          meta: { permission: 'call_logs' }
        },
        {
          path: 'calling/ivr-flows',
          name: 'ivr-flows',
          component: () => import('@/views/calling/IVRFlowsView.vue'),
          meta: { permission: 'ivr_flows' }
        },
        {
          path: 'calling/transfers',
          name: 'call-transfers',
          component: () => import('@/views/calling/CallTransfersView.vue'),
          meta: { permission: 'call_transfers' }
        }
      ]
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('@/views/NotFoundView.vue')
    }
  ]
})

// Navigation items with permissions in priority order (matches AppLayout.vue)
// Used to find the first accessible route for a user
const navigationOrder = [
  { path: '/', permission: 'analytics' },
  { path: '/chat', permission: 'chat' },
  { path: '/chatbot', permission: 'settings.chatbot', childPaths: [
    { path: '/chatbot', permission: 'settings.chatbot' },
    { path: '/chatbot/keywords', permission: 'chatbot.keywords' },
    { path: '/chatbot/flows', permission: 'flows.chatbot' },
    { path: '/chatbot/ai', permission: 'chatbot.ai' }
  ]},
  { path: '/chatbot/transfers', permission: 'transfers' },
  { path: '/analytics/agents', permission: 'analytics.agents' },
  { path: '/analytics/meta-insights', permission: 'analytics' },
  { path: '/templates', permission: 'templates' },
  { path: '/flows', permission: 'flows.whatsapp' },
  { path: '/campaigns', permission: 'campaigns' },
  { path: '/calling/logs', permission: 'call_logs', childPaths: [
    { path: '/calling/logs', permission: 'call_logs' },
    { path: '/calling/ivr-flows', permission: 'ivr_flows' },
    { path: '/calling/transfers', permission: 'call_transfers' }
  ]},
  { path: '/settings', permission: 'settings.general', childPaths: [
    { path: '/settings', permission: 'settings.general' },
    { path: '/settings/chatbot', permission: 'settings.chatbot' },
    { path: '/settings/accounts', permission: 'accounts' },
    { path: '/settings/canned-responses', permission: 'canned_responses' },
    { path: '/settings/contacts', permission: 'contacts' },
    { path: '/settings/tags', permission: 'tags' },
    { path: '/settings/teams', permission: 'teams' },
    { path: '/settings/users', permission: 'users' },
    { path: '/settings/roles', permission: 'roles' },
    { path: '/settings/api-keys', permission: 'api_keys' },
    { path: '/settings/webhooks', permission: 'webhooks' },
    { path: '/settings/custom-actions', permission: 'custom_actions' },
    { path: '/settings/sso', permission: 'settings.sso' }
  ]}
]

// Find the first accessible route for the user
function getFirstAccessibleRoute(authStore: ReturnType<typeof useAuthStore>): string {
  for (const item of navigationOrder) {
    // Check if user has permission for this item
    if (authStore.hasPermission(item.permission, 'read')) {
      return item.path
    }
    // Check child paths if available
    if (item.childPaths) {
      for (const child of item.childPaths) {
        if (authStore.hasPermission(child.permission, 'read')) {
          return child.path
        }
      }
    }
  }
  // Fallback to profile (always accessible)
  return '/profile'
}

// Navigation guard
router.beforeEach(async (to, _from, next) => {
  const authStore = useAuthStore()

  // Check if route requires auth
  if (to.meta.requiresAuth !== false) {
    if (!authStore.isAuthenticated) {
      // Try to restore session from localStorage
      const restored = authStore.restoreSession()
      if (!restored) {
        return next({ name: 'login', query: { redirect: to.fullPath } })
      }
    }

    // Check permission-based access
    const requiredPermission = to.meta.permission
    if (requiredPermission) {
      if (!authStore.hasPermission(requiredPermission, 'read')) {
        // Redirect to first accessible page
        return next({ path: getFirstAccessibleRoute(authStore) })
      }
    }
  } else {
    // Redirect to appropriate page if already logged in
    if (authStore.isAuthenticated && (to.name === 'login' || to.name === 'register')) {
      return next({ path: getFirstAccessibleRoute(authStore) })
    }
  }

  next()
})

export default router
