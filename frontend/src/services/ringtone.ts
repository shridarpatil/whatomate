// Synthetic telephone ringtone via the Web Audio API — no audio asset needed.
// Plays a 440+480 Hz ringback burst (~1s) on a 3s cadence until stopped, so an
// incoming call rings continuously like a phone instead of a single short beep.

let audioCtx: AudioContext | null = null
let cadenceTimer: ReturnType<typeof setInterval> | null = null
let ringing = false

// Emit one ~1s dual-tone ring burst with a click-free amplitude envelope.
function ringBurst(ctx: AudioContext): void {
  const now = ctx.currentTime
  const gain = ctx.createGain()
  gain.connect(ctx.destination)
  gain.gain.setValueAtTime(0.0001, now)
  gain.gain.exponentialRampToValueAtTime(0.2, now + 0.04)
  gain.gain.setValueAtTime(0.2, now + 0.9)
  gain.gain.exponentialRampToValueAtTime(0.0001, now + 1.0)
  for (const freq of [440, 480]) {
    const osc = ctx.createOscillator()
    osc.type = 'sine'
    osc.frequency.value = freq
    osc.connect(gain)
    osc.start(now)
    osc.stop(now + 1.0)
  }
}

// Start ringing (idempotent). Safe to call repeatedly; only one cadence runs.
export function startRingtone(): void {
  if (ringing) return
  ringing = true
  try {
    if (!audioCtx) {
      const Ctx = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext
      audioCtx = new Ctx()
    }
    if (audioCtx.state === 'suspended') audioCtx.resume().catch(() => { /* needs a user gesture */ })
    const ctx = audioCtx
    ringBurst(ctx) // ring immediately, then repeat (1s ring + 2s pause)
    cadenceTimer = setInterval(() => ringBurst(ctx), 3000)
  } catch {
    ringing = false
  }
}

// Stop ringing (idempotent).
export function stopRingtone(): void {
  ringing = false
  if (cadenceTimer !== null) {
    clearInterval(cadenceTimer)
    cadenceTimer = null
  }
}
