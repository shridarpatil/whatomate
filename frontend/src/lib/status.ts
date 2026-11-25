/**
 * One status vocabulary for every list. A tone picks the badge colour in
 * theme.css (`.badge[data-status=<tone>]`):
 *   neutral – not started or inactive (draft, scheduled, away)
 *   info    – in progress (sending, processing)
 *   success – done or healthy (completed, approved, active)
 *   warning – needs attention (paused, pending review)
 *   danger  – failed or rejected
 */
export type StatusTone = 'neutral' | 'info' | 'success' | 'warning' | 'danger'

const CAMPAIGN: Record<string, StatusTone> = {
  draft: 'neutral',
  scheduled: 'neutral',
  queued: 'info',
  running: 'info',
  processing: 'info',
  paused: 'warning',
  completed: 'success',
  cancelled: 'neutral',
  failed: 'danger',
}

const TEMPLATE: Record<string, StatusTone> = {
  APPROVED: 'success',
  PENDING: 'warning',
  IN_APPEAL: 'warning',
  PAUSED: 'warning',
  REJECTED: 'danger',
  DISABLED: 'danger',
  DRAFT: 'neutral',
}

/** Campaign tone; a completed campaign with failed messages needs attention. */
export function campaignTone(status: string, failedCount = 0): StatusTone {
  if (status === 'completed' && failedCount > 0) return 'warning'
  return CAMPAIGN[status] ?? 'neutral'
}

export function templateTone(status: string): StatusTone {
  return TEMPLATE[status?.toUpperCase()] ?? 'neutral'
}

/** Built-in role names are stored lowercase; custom role names are shown as typed. */
const SYSTEM_ROLES = new Set(['admin', 'manager', 'agent'])
export function roleLabel(name: string | null | undefined): string {
  if (!name) return ''
  return SYSTEM_ROLES.has(name) ? humanize(name) : name
}

/** "UTILITY" / "in_review" / "marketing" -> "Utility" / "In review" / "Marketing". */
export function humanize(value: string | null | undefined): string {
  if (!value) return ''
  const s = value.replace(/[_-]+/g, ' ').trim().toLowerCase()
  return s.charAt(0).toUpperCase() + s.slice(1)
}
