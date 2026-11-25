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
  PhoneCall,
  PhoneForwarded,
  ScrollText
} from '@/lib/icons'
import type { Component } from 'vue'

export interface NavItem {
  name: string
  path: string
  icon: Component
  permission?: string
  childPermissions?: string[]
  children?: NavItem[]
}

export interface NavSection {
  label: string
  items: NavItem[]
  /** Permissions needed to show section — at least one must pass */
  permissions: string[]
  /** Pin to bottom of sidebar */
  pinBottom?: boolean
}

export const navigationSections: NavSection[] = [
  {
    label: 'nav.sectionMain',
    permissions: ['analytics', 'chat', 'contacts'],
    items: [
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
        name: 'nav.contacts',
        path: '/contacts',
        icon: Contact,
        permission: 'contacts'
      },
    ]
  },
  {
    label: 'nav.sectionMessaging',
    permissions: ['settings.chatbot', 'chatbot.keywords', 'flows.chatbot', 'chatbot.ai', 'transfers', 'campaigns', 'templates', 'flows.whatsapp'],
    items: [
      {
        name: 'nav.chatbot',
        path: '/chatbot',
        icon: Bot,
        permission: 'settings.chatbot',
        childPermissions: ['settings.chatbot', 'chatbot.keywords', 'flows.chatbot', 'chatbot.ai', 'transfers'],
        children: [
          { name: 'nav.overview', path: '/chatbot', icon: Bot, permission: 'settings.chatbot' },
          { name: 'nav.keywords', path: '/chatbot/keywords', icon: Key, permission: 'chatbot.keywords' },
          { name: 'nav.botFlows', path: '/chatbot/flows', icon: Workflow, permission: 'flows.chatbot' },
          { name: 'nav.aiContexts', path: '/chatbot/ai', icon: Sparkles, permission: 'chatbot.ai' },
          { name: 'nav.transfers', path: '/chatbot/transfers', icon: UserX, permission: 'transfers' }
        ]
      },
      {
        name: 'nav.campaigns',
        path: '/campaigns',
        icon: Megaphone,
        permission: 'campaigns'
      },
      {
        name: 'nav.templates',
        path: '/templates',
        icon: FileText,
        permission: 'templates'
      },
      {
        name: 'nav.whatsappFlows',
        path: '/flows',
        icon: Workflow,
        permission: 'flows.whatsapp'
      },
    ]
  },
  {
    label: 'nav.sectionCalling',
    permissions: ['call_logs', 'ivr_flows', 'call_transfers'],
    items: [
      { name: 'nav.callLogs', path: '/calling/logs', icon: PhoneCall, permission: 'call_logs' },
      { name: 'nav.ivrFlows', path: '/calling/ivr-flows', icon: Workflow, permission: 'ivr_flows' },
      { name: 'nav.callTransfers', path: '/calling/transfers', icon: PhoneForwarded, permission: 'call_transfers' },
    ]
  },
  {
    label: 'nav.sectionAnalytics',
    permissions: ['analytics.agents', 'analytics'],
    items: [
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
    ]
  },
  {
    label: '',
    permissions: ['settings.general', 'settings.chatbot', 'accounts', 'canned_responses', 'tags', 'teams', 'users', 'roles', 'api_keys', 'webhooks', 'custom_actions', 'settings.sso', 'audit_logs'],
    pinBottom: true,
    items: [
      {
        name: 'nav.settings',
        path: '/settings',
        icon: Settings,
        permission: 'settings.general',
        childPermissions: ['settings.general', 'settings.chatbot', 'accounts', 'canned_responses', 'tags', 'teams', 'users', 'roles', 'api_keys', 'webhooks', 'custom_actions', 'settings.sso', 'audit_logs'],
        // Settings pages get their own grouped menu inside the page (settingsSections),
        // so the sidebar stays short. `children` is kept for permission fallback only.
        children: [
          { name: 'nav.general', path: '/settings', icon: Settings, permission: 'settings.general' },
          { name: 'nav.accounts', path: '/settings/accounts', icon: Users, permission: 'accounts' },
          { name: 'nav.users', path: '/settings/users', icon: Users, permission: 'users' },
          { name: 'nav.teams', path: '/settings/teams', icon: Users, permission: 'teams' },
          { name: 'nav.roles', path: '/settings/roles', icon: Shield, permission: 'roles' },
          { name: 'nav.cannedResponses', path: '/settings/canned-responses', icon: MessageSquareText, permission: 'canned_responses' },
          { name: 'nav.tags', path: '/settings/tags', icon: Tags, permission: 'tags' },
          { name: 'nav.chatbot', path: '/settings/chatbot', icon: Bot, permission: 'settings.chatbot' },
          { name: 'nav.apiKeys', path: '/settings/api-keys', icon: Key, permission: 'api_keys' },
          { name: 'nav.webhooks', path: '/settings/webhooks', icon: Webhook, permission: 'webhooks' },
          { name: 'nav.customActions', path: '/settings/custom-actions', icon: Zap, permission: 'custom_actions' },
          { name: 'nav.sso', path: '/settings/sso', icon: ShieldCheck, permission: 'settings.sso' },
          { name: 'nav.auditLogs', path: '/settings/audit-logs', icon: ScrollText, permission: 'audit_logs' }
        ]
      }
    ]
  }
]

/** Groups for the in-page Settings menu (SettingsNav.vue). */
export const settingsSections: { label: string; paths: string[] }[] = [
  { label: 'nav.settingsWorkspace', paths: ['/settings', '/settings/accounts', '/settings/users', '/settings/teams', '/settings/roles'] },
  { label: 'nav.settingsMessaging', paths: ['/settings/canned-responses', '/settings/tags', '/settings/chatbot'] },
  { label: 'nav.settingsIntegrations', paths: ['/settings/api-keys', '/settings/webhooks', '/settings/custom-actions', '/settings/sso'] },
  { label: 'nav.settingsSecurity', paths: ['/settings/audit-logs'] },
]

// Flat list for backward compatibility (used by AppLayout computed)
export const navigationItems: NavItem[] = navigationSections.flatMap(s => s.items)
