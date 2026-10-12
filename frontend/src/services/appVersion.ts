import { toast } from 'vue-sonner'
import { i18n } from '@/i18n'

const basePath = ((window as any).__BASE_PATH__ ?? '').replace(/\/$/, '')

// How often to ask the server which build it is serving. Deploys are rare and
// the response is a few dozen bytes, so a slow poll is plenty; the
// visibility hook below is what catches the common case in practice.
const POLL_INTERVAL_MS = 15 * 60 * 1000

// Floor between two checks, so tab-switching cannot turn the visibility hook
// into a request loop.
const MIN_CHECK_GAP_MS = 60 * 1000

// The build this tab booted with. Stays null until the first successful fetch:
// if the very first request fails (offline, server restarting) we adopt the
// next answer as the baseline rather than reporting a bogus update.
let bootBuild: string | null = null
let lastCheckedAt = 0
let started = false

async function fetchBuild(): Promise<string | null> {
  try {
    const res = await fetch(`${basePath}/build.json`, { cache: 'no-store' })
    if (!res.ok) return null
    const data = await res.json()
    return typeof data?.build === 'string' ? data.build : null
  } catch {
    // Offline, or the server is mid-restart — try again on the next tick.
    return null
  }
}

function announceUpdate() {
  const t = i18n.global.t
  // A fixed id keeps repeated detections collapsing into the one toast instead
  // of stacking. It also means a dismissed toast comes back on the next check,
  // which is the point: the tab is running code that no longer exists.
  toast.info(t('common.updateAvailable'), {
    id: 'app-update-available',
    description: t('common.updateAvailableDescription'),
    duration: Infinity,
    action: {
      label: t('common.reload'),
      onClick: () => window.location.reload()
    }
  })
}

async function check() {
  const now = Date.now()
  if (now - lastCheckedAt < MIN_CHECK_GAP_MS) return
  lastCheckedAt = now

  const build = await fetchBuild()
  if (!build) return

  if (bootBuild === null) {
    bootBuild = build
    return
  }
  if (build !== bootBuild) announceUpdate()
}

/**
 * Watch for the server being upgraded underneath a tab that never reloads.
 *
 * Browsers increasingly keep tabs alive indefinitely (session restore, tab
 * freezing/discarding), so an agent can sit on a build from several deploys ago
 * and see none of the fixes — with no symptom other than features quietly
 * behaving the old way. Cache headers do not help: the bundle is only requested
 * on a real navigation, and this tab is not making one.
 *
 * Safe to call more than once.
 */
export function startVersionWatch() {
  if (started) return
  started = true

  void check()
  window.setInterval(() => void check(), POLL_INTERVAL_MS)

  // The trigger that matters day to day: the agent comes back to a tab they
  // left open, and finds out right then rather than 15 minutes later.
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') void check()
  })
}
