import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatDate(date: string | Date, options?: Intl.DateTimeFormatOptions): string {
  const d = typeof date === 'string' ? new Date(date) : date
  return d.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    ...options
  })
}

export function formatTime(date: string | Date): string {
  const d = typeof date === 'string' ? new Date(date) : date
  return d.toLocaleTimeString('en-US', {
    hour: '2-digit',
    minute: '2-digit'
  })
}

export function formatDateTime(date: string | Date): string {
  return `${formatDate(date)} ${formatTime(date)}`
}

export function truncate(str: string, length: number): string {
  if (str.length <= length) return str
  return str.slice(0, length) + '...'
}

export function debounce<T extends (...args: any[]) => any>(
  fn: T,
  delay: number
): (...args: Parameters<T>) => void {
  let timeoutId: ReturnType<typeof setTimeout>
  return (...args: Parameters<T>) => {
    clearTimeout(timeoutId)
    timeoutId = setTimeout(() => fn(...args), delay)
  }
}

export function generateId(): string {
  return Math.random().toString(36).substring(2, 15)
}

export function getInitials(name: string): string {
  return name
    .split(' ')
    .map(n => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)
}

const avatarGradients = [
  'from-violet-500 to-purple-600',
  'from-blue-500 to-cyan-600',
  'from-rose-500 to-pink-600',
  'from-amber-500 to-orange-600',
  'from-emerald-500 to-teal-600',
  'from-indigo-500 to-blue-600',
  'from-fuchsia-500 to-purple-600',
  'from-cyan-500 to-blue-600',
  'from-orange-500 to-red-600',
  'from-teal-500 to-emerald-600',
]

export function getAvatarGradient(name: string): string {
  if (!name) return avatarGradients[0]
  let hash = 0
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash)
  }
  return avatarGradients[Math.abs(hash) % avatarGradients.length]
}

export function formatLabel(key: string): string {
  return key
    .replace(/_/g, ' ')
    .replace(/([a-z])([A-Z])/g, '$1 $2')
    .replace(/\b\w/g, c => c.toUpperCase())
}

export function getQualityBadgeClass(rating: string): string {
  if (!rating) return 'bg-gray-800 text-gray-400 light:bg-gray-100 light:text-gray-600'
  switch (rating.toUpperCase()) {
    case 'GREEN':
    case 'HIGH':
      return 'bg-green-950 text-green-400 border border-green-800/40 light:bg-green-100 light:text-green-800'
    case 'YELLOW':
    case 'MEDIUM':
      return 'bg-yellow-950 text-yellow-400 border border-yellow-800/40 light:bg-yellow-100 light:text-yellow-800'
    case 'RED':
    case 'LOW':
      return 'bg-red-950 text-red-400 border border-red-800/40 light:bg-red-100 light:text-red-800'
    default:
      return 'bg-gray-800 text-gray-400 light:bg-gray-100 light:text-gray-600'
  }
}

export function getQualityRatingLabel(rating: string | undefined, t: (key: string) => string): string {
  switch ((rating || '').toUpperCase()) {
    case '':
    case 'UNKNOWN':
      return t('accounts.qualityUnknown')
    case 'GREEN':
    case 'HIGH':
      return t('accounts.qualityGreen')
    case 'YELLOW':
    case 'MEDIUM':
      return t('accounts.qualityYellow')
    case 'RED':
    case 'LOW':
      return t('accounts.qualityRed')
    default:
      return rating || ''
  }
}

export function getVerificationBadgeClass(status: string): string {
  if (!status) return 'bg-gray-800 text-gray-400 light:bg-gray-100 light:text-gray-600'
  switch (status.toUpperCase()) {
    case 'VERIFIED':
    case 'VERIFIED_CODE':
      return 'bg-green-950 text-green-400 border border-green-800/40 light:bg-green-100 light:text-green-800'
    case 'NOT_VERIFIED':
      return 'bg-red-950 text-red-400 border border-red-800/40 light:bg-red-100 light:text-red-800'
    case 'EXPIRED':
      return 'bg-amber-950 text-amber-400 border border-amber-800/40 light:bg-amber-100 light:text-amber-800'
    default:
      return 'bg-gray-800 text-gray-400 light:bg-gray-100 light:text-gray-600'
  }
}

export function getVerificationStatusLabel(status: string | undefined, t: (key: string) => string): string {
  switch ((status || '').toUpperCase()) {
    case 'VERIFIED':
    case 'VERIFIED_CODE':
      return t('accounts.statusVerified')
    case 'NOT_VERIFIED':
      return t('accounts.statusNotVerified')
    case 'EXPIRED':
      return t('accounts.statusExpired')
    default:
      return status || ''
  }
}

export function formatLimitTier(
  tier: string | undefined,
  isSandbox: boolean | undefined,
  t: (key: string) => string
): string {
  if (isSandbox) {
    return t('accounts.limitTierSandbox')
  }
  if (!tier) {
    return t('accounts.limitTierDefault')
  }
  const clean = tier.toUpperCase().replace('TIER_', '')
  switch (clean) {
    case '250':
      return t('accounts.limitTier250')
    case '2K':
      return t('accounts.limitTier2K')
    case '10K':
      return t('accounts.limitTier10K')
    case '100K':
      return t('accounts.limitTier100K')
    case 'UNLIMITED':
      return t('accounts.limitTierUnlimited')
    default:
      return `${clean} msgs/day`
  }
}

