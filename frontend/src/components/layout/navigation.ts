import {
  LayoutDashboard,
  MessageSquare,
  Bot,
  FileText,
  Megaphone,
  Settings,
  Users,
  Workflow,
  Sparkles,
  Key,
  UserX,
  MessageSquareText,
  Webhook,
  BarChart3,
  ShieldCheck,
  Zap,
  Shield
} from 'lucide-vue-next'
import type { Component } from 'vue'

export interface NavItem {
  name: string
  path: string
  icon: Component
  permission?: string
  childPermissions?: string[]
  children?: NavItem[]
}

export const navigationItems: NavItem[] = [
  {
    name: 'Dashboard',
    path: '/',
    icon: LayoutDashboard,
    permission: 'analytics'
  },
  {
    name: 'Chat',
    path: '/chat',
    icon: MessageSquare,
    permission: 'chat'
  },
  {
    name: 'Chatbot',
    path: '/chatbot',
    icon: Bot,
    permission: 'settings.chatbot',
    childPermissions: ['settings.chatbot', 'chatbot.keywords', 'flows.chatbot', 'chatbot.ai'],
    children: [
      { name: 'Overview', path: '/chatbot', icon: Bot, permission: 'settings.chatbot' },
      { name: 'Keywords', path: '/chatbot/keywords', icon: Key, permission: 'chatbot.keywords' },
      { name: 'Flows', path: '/chatbot/flows', icon: Workflow, permission: 'flows.chatbot' },
      { name: 'AI Contexts', path: '/chatbot/ai', icon: Sparkles, permission: 'chatbot.ai' }
    ]
  },
  {
    name: 'Transfers',
    path: '/chatbot/transfers',
    icon: UserX,
    permission: 'transfers'
  },
  {
    name: 'Agent Analytics',
    path: '/analytics/agents',
    icon: BarChart3,
    permission: 'analytics.agents'
  },
  {
    name: 'Templates',
    path: '/templates',
    icon: FileText,
    permission: 'templates'
  },
  {
    name: 'Flows',
    path: '/flows',
    icon: Workflow,
    permission: 'flows.whatsapp'
  },
  {
    name: 'Campaigns',
    path: '/campaigns',
    icon: Megaphone,
    permission: 'campaigns'
  },
  {
    name: 'Settings',
    path: '/settings',
    icon: Settings,
    permission: 'settings.general',
    childPermissions: ['settings.general', 'settings.chatbot', 'accounts', 'canned_responses', 'teams', 'users', 'roles', 'api_keys', 'webhooks', 'custom_actions', 'settings.sso'],
    children: [
      { name: 'General', path: '/settings', icon: Settings, permission: 'settings.general' },
      { name: 'Chatbot', path: '/settings/chatbot', icon: Bot, permission: 'settings.chatbot' },
      { name: 'Accounts', path: '/settings/accounts', icon: Users, permission: 'accounts' },
      { name: 'Canned Responses', path: '/settings/canned-responses', icon: MessageSquareText, permission: 'canned_responses' },
      { name: 'Teams', path: '/settings/teams', icon: Users, permission: 'teams' },
      { name: 'Users', path: '/settings/users', icon: Users, permission: 'users' },
      { name: 'Roles', path: '/settings/roles', icon: Shield, permission: 'roles' },
      { name: 'API Keys', path: '/settings/api-keys', icon: Key, permission: 'api_keys' },
      { name: 'Webhooks', path: '/settings/webhooks', icon: Webhook, permission: 'webhooks' },
      { name: 'Custom Actions', path: '/settings/custom-actions', icon: Zap, permission: 'custom_actions' },
      { name: 'SSO', path: '/settings/sso', icon: ShieldCheck, permission: 'settings.sso' }
    ]
  }
]
