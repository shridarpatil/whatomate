// Looping ringtone for an incoming call waiting to be answered.
//
// One shared element, reused across calls: a fresh Audio per ring would stack
// up if a stop event were ever missed. Which sound plays is the agent's choice
// (Settings → Notifications), kept deliberately distinct from the new-message
// beep so the two can't be confused.

import { useAuthStore, type CallRingtone } from '@/stores/auth'

const SOURCES: Record<Exclude<CallRingtone, 'none'>, string> = {
  // Two-tone ring with a 2s gap, so looping sounds like a phone ringing.
  ring: '/ringtone.wav',
  // The message beep, for agents who'd rather keep the old sound.
  beep: '/notification.mp3',
}

const DEFAULT_RINGTONE: CallRingtone = 'ring'
const RINGTONE_VOLUME = 0.5

// Hard stop so a dropped "call is over" event can't leave an agent's tab
// ringing forever. Comfortably longer than any ring window the server uses.
const MAX_RING_MS = 60_000

let element: HTMLAudioElement | null = null
let loadedSrc: string | null = null
let stopTimer: ReturnType<typeof setTimeout> | null = null

function selectedRingtone(): CallRingtone {
  try {
    return useAuthStore().userSettings.call_ringtone || DEFAULT_RINGTONE
  } catch {
    // Pinia not active (tests, early boot) — fall back rather than go silent.
    return DEFAULT_RINGTONE
  }
}

export function startRinging() {
  const choice = selectedRingtone()
  if (choice === 'none') {
    return
  }

  const src = SOURCES[choice] ?? SOURCES[DEFAULT_RINGTONE as Exclude<CallRingtone, 'none'>]
  if (!element || loadedSrc !== src) {
    element?.pause()
    element = new Audio(src)
    element.loop = true
    element.volume = RINGTONE_VOLUME
    loadedSrc = src
  }

  if (stopTimer) {
    clearTimeout(stopTimer)
  }
  stopTimer = setTimeout(stopRinging, MAX_RING_MS)

  // Already ringing — don't restart from the top mid-loop.
  if (!element.paused) {
    return
  }
  element.currentTime = 0
  element.play().catch(() => {
    // Browsers block autoplay until the tab has seen a user gesture.
  })
}

export function stopRinging() {
  if (stopTimer) {
    clearTimeout(stopTimer)
    stopTimer = null
  }
  if (!element) {
    return
  }
  element.pause()
  element.currentTime = 0
}

// previewRingtone plays one pass of a ringtone for the settings picker, without
// disturbing a ring that may be in progress.
export function previewRingtone(choice: CallRingtone) {
  if (choice === 'none') {
    return
  }
  const audio = new Audio(SOURCES[choice])
  audio.volume = RINGTONE_VOLUME
  audio.play().catch(() => {})
}
