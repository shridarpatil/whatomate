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

export function formatLabel(key: string): string {
  return key
    .replace(/_/g, ' ')
    .replace(/([a-z])([A-Z])/g, '$1 $2')
    .replace(/\b\w/g, c => c.toUpperCase())
}

export type BadgeVariant = 'success' | 'warning' | 'danger' | 'secondary'

export function getQualityBadgeVariant(rating: string): BadgeVariant {
  switch ((rating || '').toUpperCase()) {
    case 'GREEN':
    case 'HIGH':
      return 'success'
    case 'YELLOW':
    case 'MEDIUM':
      return 'warning'
    case 'RED':
    case 'LOW':
      return 'danger'
    default:
      return 'secondary'
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

export function getVerificationBadgeVariant(status: string): BadgeVariant {
  switch ((status || '').toUpperCase()) {
    case 'VERIFIED':
    case 'VERIFIED_CODE':
      return 'success'
    case 'NOT_VERIFIED':
      return 'danger'
    case 'EXPIRED':
      return 'warning'
    default:
      return 'secondary'
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

