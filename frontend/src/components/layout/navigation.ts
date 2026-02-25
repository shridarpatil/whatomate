import {
  LayoutDashboard,
  MessageSquare,
  Bot,
  FileText,
  Megaphone,
  Settings,
  Users,
  Contact,
  Workflow,
  Sparkles,
  Key,
  UserX,
  MessageSquareText,
  Webhook,
  BarChart3,
  ShieldCheck,
  Zap,
  Shield,
  LineChart,
  Tags,
  Phone,
  PhoneCall,
  PhoneForwarded
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
    name: 'nav.dashboard',
    path: '/',
    icon: LayoutDashboard,
    permission: 'analytics'
  },
  {
    name: 'nav.chat',
    path: '/chat',
    icon: MessageSquare,
    permission: 'chat'
  },
  {
    name: 'nav.chatbot',
    path: '/chatbot',
    icon: Bot,
    permission: 'settings.chatbot',
    childPermissions: ['settings.chatbot', 'chatbot.keywords', 'flows.chatbot', 'chatbot.ai'],
    children: [
      { name: 'nav.overview', path: '/chatbot', icon: Bot, permission: 'settings.chatbot' },
      { name: 'nav.keywords', path: '/chatbot/keywords', icon: Key, permission: 'chatbot.keywords' },
      { name: 'nav.flows', path: '/chatbot/flows', icon: Workflow, permission: 'flows.chatbot' },
      { name: 'nav.aiContexts', path: '/chatbot/ai', icon: Sparkles, permission: 'chatbot.ai' }
    ]
  },
  {
    name: 'nav.transfers',
    path: '/chatbot/transfers',
    icon: UserX,
    permission: 'transfers'
  },
  {
    name: 'nav.agentAnalytics',
    path: '/analytics/agents',
    icon: BarChart3,
    permission: 'analytics.agents'
  },
  {
    name: 'nav.metaInsights',
    path: '/analytics/meta-insights',
    icon: LineChart,
    permission: 'analytics'
  },
  {
    name: 'nav.templates',
    path: '/templates',
    icon: FileText,
    permission: 'templates'
  },
  {
    name: 'nav.flows',
    path: '/flows',
    icon: Workflow,
    permission: 'flows.whatsapp'
  },
  {
    name: 'nav.campaigns',
    path: '/campaigns',
    icon: Megaphone,
    permission: 'campaigns'
  },
  {
    name: 'nav.calling',
    path: '/calling',
    icon: Phone,
    permission: 'call_logs',
    childPermissions: ['call_logs', 'ivr_flows', 'call_transfers'],
    children: [
      { name: 'nav.callLogs', path: '/calling/logs', icon: PhoneCall, permission: 'call_logs' },
      { name: 'nav.ivrFlows', path: '/calling/ivr-flows', icon: Workflow, permission: 'ivr_flows' },
      { name: 'nav.callTransfers', path: '/calling/transfers', icon: PhoneForwarded, permission: 'call_transfers' }
    ]
  },
  {
    name: 'nav.settings',
    path: '/settings',
    icon: Settings,
    permission: 'settings.general',
    childPermissions: ['settings.general', 'settings.chatbot', 'accounts', 'contacts', 'canned_responses', 'tags', 'teams', 'users', 'roles', 'api_keys', 'webhooks', 'custom_actions', 'settings.sso'],
    children: [
      { name: 'nav.general', path: '/settings', icon: Settings, permission: 'settings.general' },
      { name: 'nav.chatbot', path: '/settings/chatbot', icon: Bot, permission: 'settings.chatbot' },
      { name: 'nav.accounts', path: '/settings/accounts', icon: Users, permission: 'accounts' },
      { name: 'nav.contacts', path: '/settings/contacts', icon: Contact, permission: 'contacts' },
      { name: 'nav.cannedResponses', path: '/settings/canned-responses', icon: MessageSquareText, permission: 'canned_responses' },
      { name: 'nav.tags', path: '/settings/tags', icon: Tags, permission: 'tags' },
      { name: 'nav.teams', path: '/settings/teams', icon: Users, permission: 'teams' },
      { name: 'nav.users', path: '/settings/users', icon: Users, permission: 'users' },
      { name: 'nav.roles', path: '/settings/roles', icon: Shield, permission: 'roles' },
      { name: 'nav.apiKeys', path: '/settings/api-keys', icon: Key, permission: 'api_keys' },
      { name: 'nav.webhooks', path: '/settings/webhooks', icon: Webhook, permission: 'webhooks' },
      { name: 'nav.customActions', path: '/settings/custom-actions', icon: Zap, permission: 'custom_actions' },
      { name: 'nav.sso', path: '/settings/sso', icon: ShieldCheck, permission: 'settings.sso' }
    ]
  }
]
