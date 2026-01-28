/**
 * Centralized constants for the application
 */

// Canned response categories
export const CANNED_RESPONSE_CATEGORIES = [
  { value: 'greeting', label: 'Greetings' },
  { value: 'support', label: 'Support' },
  { value: 'sales', label: 'Sales' },
  { value: 'closing', label: 'Closing' },
  { value: 'general', label: 'General' },
] as const

// Template categories (WhatsApp)
export const TEMPLATE_CATEGORIES = [
  { value: 'UTILITY', label: 'Utility', description: 'Order updates, account alerts' },
  { value: 'MARKETING', label: 'Marketing', description: 'Promotions, offers' },
  { value: 'AUTHENTICATION', label: 'Authentication', description: 'OTP, verification codes' },
] as const

// Team assignment strategies
export const ASSIGNMENT_STRATEGIES = [
  { value: 'round_robin', label: 'Round Robin', description: 'Distribute evenly to all team members' },
  { value: 'load_balanced', label: 'Load Balanced', description: 'Assign to agent with least open conversations' },
  { value: 'manual', label: 'Manual Queue', description: 'Agents manually pick up conversations' },
] as const

export type AssignmentStrategy = typeof ASSIGNMENT_STRATEGIES[number]['value']

// User/agent statuses
export const USER_STATUSES = [
  { value: 'active', label: 'Active' },
  { value: 'inactive', label: 'Inactive' },
] as const

// Role badge variants
export const ROLE_BADGE_VARIANTS: Record<string, 'default' | 'secondary' | 'outline'> = {
  admin: 'default',
  manager: 'secondary',
  agent: 'outline',
} as const

// Resource labels for permissions UI
export const RESOURCE_LABELS: Record<string, string> = {
  users: 'Users',
  contacts: 'Contacts',
  messages: 'Messages',
  teams: 'Teams',
  chatbot: 'Chatbot',
  campaigns: 'Campaigns',
  templates: 'Templates',
  analytics: 'Analytics',
  settings: 'Settings',
  webhooks: 'Webhooks',
  apikeys: 'API Keys',
  roles: 'Roles',
} as const

// Supported languages for templates
export const SUPPORTED_LANGUAGES = [
  { code: 'en', name: 'English' },
  { code: 'en_US', name: 'English (US)' },
  { code: 'es', name: 'Spanish' },
  { code: 'pt_BR', name: 'Portuguese (BR)' },
  { code: 'de', name: 'German' },
] as const

// Default pagination settings
export const DEFAULT_PAGE_SIZE = 20
export const PAGE_SIZE_OPTIONS = [10, 20, 50, 100] as const

// Helper function to get label from value
export function getLabelFromValue<T extends readonly { value: string; label: string }[]>(
  options: T,
  value: string
): string {
  const option = options.find(opt => opt.value === value)
  return option?.label || value
}
