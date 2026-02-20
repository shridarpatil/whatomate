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

// Team assignment strategies
export const ASSIGNMENT_STRATEGIES = [
  { value: 'round_robin', label: 'Round Robin', description: 'Distribute evenly to all team members' },
  { value: 'load_balanced', label: 'Load Balanced', description: 'Assign to agent with least open conversations' },
  { value: 'manual', label: 'Manual Queue', description: 'Agents manually pick up conversations' },
] as const

export type AssignmentStrategy = typeof ASSIGNMENT_STRATEGIES[number]['value']

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
  tags: 'Tags',
} as const

// Default pagination settings
export const DEFAULT_PAGE_SIZE = 20
// Tag colors with their CSS classes
export const TAG_COLORS = [
  { value: 'blue', label: 'Blue', class: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400' },
  { value: 'red', label: 'Red', class: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400' },
  { value: 'green', label: 'Green', class: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400' },
  { value: 'yellow', label: 'Yellow', class: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400' },
  { value: 'purple', label: 'Purple', class: 'bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-400' },
  { value: 'gray', label: 'Gray', class: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300' },
] as const

// Get tag color class by value
export function getTagColorClass(color: string): string {
  const tagColor = TAG_COLORS.find(c => c.value === color)
  return tagColor?.class || TAG_COLORS.find(c => c.value === 'gray')!.class
}

// Helper function to get label from value
export function getLabelFromValue<T extends readonly { value: string; label: string }[]>(
  options: T,
  value: string
): string {
  const option = options.find(opt => opt.value === value)
  return option?.label || value
}
