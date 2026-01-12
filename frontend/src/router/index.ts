import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

// Role-based route meta type
declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    roles?: ('admin' | 'manager' | 'agent')[]
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
          meta: { roles: ['admin', 'manager'] }
        },
        {
          path: 'chat',
          name: 'chat',
          component: () => import('@/views/chat/ChatView.vue')
          // All roles can access chat
        },
        {
          path: 'chat/:contactId',
          name: 'chat-conversation',
          component: () => import('@/views/chat/ChatView.vue'),
          props: true
          // All roles can access chat
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
          meta: { roles: ['admin', 'manager'] }
        },
        {
          path: 'flows',
          name: 'flows',
          component: () => import('@/views/settings/FlowsView.vue'),
          meta: { roles: ['admin', 'manager'] }
        },
        {
          path: 'campaigns',
          name: 'campaigns',
          component: () => import('@/views/settings/CampaignsView.vue'),
          meta: { roles: ['admin', 'manager'] }
        },
        {
          path: 'chatbot',
          name: 'chatbot',
          component: () => import('@/views/chatbot/ChatbotView.vue'),
          meta: { roles: ['admin', 'manager'] }
        },
        {
          path: 'chatbot/settings',
          redirect: '/settings/chatbot'
        },
        {
          path: 'chatbot/keywords',
          name: 'chatbot-keywords',
          component: () => import('@/views/chatbot/KeywordsView.vue'),
          meta: { roles: ['admin', 'manager'] }
        },
        {
          path: 'chatbot/flows',
          name: 'chatbot-flows',
          component: () => import('@/views/chatbot/ChatbotFlowsView.vue'),
          meta: { roles: ['admin', 'manager'] }
        },
        {
          path: 'chatbot/flows/new',
          name: 'chatbot-flow-new',
          component: () => import('@/views/chatbot/ChatbotFlowBuilderView.vue'),
          meta: { roles: ['admin', 'manager'] }
        },
        {
          path: 'chatbot/flows/:id/edit',
          name: 'chatbot-flow-edit',
          component: () => import('@/views/chatbot/ChatbotFlowBuilderView.vue'),
          meta: { roles: ['admin', 'manager'] }
        },
        {
          path: 'chatbot/ai',
          name: 'chatbot-ai',
          component: () => import('@/views/chatbot/AIContextsView.vue'),
          meta: { roles: ['admin', 'manager'] }
        },
        {
          path: 'chatbot/transfers',
          name: 'chatbot-transfers',
          component: () => import('@/views/chatbot/AgentTransfersView.vue')
          // All roles can access transfers
        },
        {
          path: 'analytics/agents',
          name: 'agent-analytics',
          component: () => import('@/views/analytics/AgentAnalyticsView.vue')
          // All roles can access (agents see only their own stats)
        },
        {
          path: 'settings',
          name: 'settings',
          component: () => import('@/views/settings/SettingsView.vue'),
          meta: { roles: ['admin', 'manager'] }
        },
        {
          path: 'settings/chatbot',
          name: 'chatbot-settings',
          component: () => import('@/views/settings/ChatbotSettingsView.vue'),
          meta: { roles: ['admin', 'manager'] }
        },
        {
          path: 'settings/accounts',
          name: 'accounts',
          component: () => import('@/views/settings/AccountsView.vue'),
          meta: { roles: ['admin', 'manager'] }
        },
        {
          path: 'settings/canned-responses',
          name: 'canned-responses',
          component: () => import('@/views/settings/CannedResponsesView.vue'),
          meta: { roles: ['admin', 'manager'] }
        },
        {
          path: 'settings/users',
          name: 'users',
          component: () => import('@/views/settings/UsersView.vue'),
          meta: { roles: ['admin'] }
        },
        {
          path: 'settings/teams',
          name: 'teams',
          component: () => import('@/views/settings/TeamsView.vue'),
          meta: { roles: ['admin', 'manager'] }
        },
        {
          path: 'settings/api-keys',
          name: 'api-keys',
          component: () => import('@/views/settings/APIKeysView.vue'),
          meta: { roles: ['admin'] }
        },
        {
          path: 'settings/webhooks',
          name: 'webhooks',
          component: () => import('@/views/settings/WebhooksView.vue'),
          meta: { roles: ['admin'] }
        },
        {
          path: 'settings/sso',
          name: 'sso-settings',
          component: () => import('@/views/settings/SSOSettingsView.vue'),
          meta: { roles: ['admin'] }
        },
        {
          path: 'settings/custom-actions',
          name: 'custom-actions',
          component: () => import('@/views/settings/CustomActionsView.vue'),
          meta: { roles: ['admin'] }
        },
        {
          path: 'settings/embedded-signup',
          name: 'embedded-signup',
          component: () => import('@/views/settings/EmbeddedSignupView.vue'),
          meta: { roles: ['admin', 'manager'] }
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

// Navigation guard
router.beforeEach(async (to, from, next) => {
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

    // Check role-based access
    const requiredRoles = to.meta.roles
    if (requiredRoles && requiredRoles.length > 0) {
      const userRole = authStore.userRole as 'admin' | 'manager' | 'agent'
      if (!requiredRoles.includes(userRole)) {
        // Redirect based on role
        if (userRole === 'agent') {
          return next({ name: 'agent-analytics' })
        } else {
          return next({ name: 'dashboard' })
        }
      }
    }
  } else {
    // Redirect to appropriate page if already logged in
    if (authStore.isAuthenticated && (to.name === 'login' || to.name === 'register')) {
      const userRole = authStore.userRole
      if (userRole === 'agent') {
        return next({ name: 'agent-analytics' })
      } else {
        return next({ name: 'dashboard' })
      }
    }
  }

  next()
})

export default router
