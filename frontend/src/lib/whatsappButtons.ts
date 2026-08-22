import type { ButtonConfig } from '@/types/flow-preview'

/** Meta's cap on a button label. Counted in characters, not bytes. */
export const MAX_BUTTON_TITLE_LENGTH = 20

/** The most reply buttons one free-form interactive message can carry. */
export const MAX_REPLY_BUTTONS = 10

/** vue-i18n's `t` narrowed to the (key, fallback) form used here. */
type Translate = (key: string, fallback: string) => string

const BUTTON_ID_PATTERN = /^btn_(\d+)$/

/**
 * Mint an id no existing button is using.
 *
 * Deriving the id from `buttons.length` collides as soon as a button is
 * deleted: removing btn_1 from [btn_1, btn_2] leaves length 1, so the next
 * add produces a second btn_2. Meta rejects an interactive message whose
 * reply ids are not unique, and the send fails with only a log line — the
 * customer never sees the message. Scanning for the highest suffix in use
 * keeps ids unique across any add/delete order.
 */
export function nextButtonId(buttons: ButtonConfig[]): string {
  let highest = 0
  for (const btn of buttons) {
    const match = BUTTON_ID_PATTERN.exec(btn.id ?? '')
    if (!match) continue
    const n = Number(match[1])
    if (Number.isFinite(n) && n > highest) highest = n
  }
  return `btn_${highest + 1}`
}

/**
 * Validate a button combination against WhatsApp Cloud API's free-form
 * interactive-message rules. Returns a user-facing message, or null when the
 * combination is sendable.
 *
 * Sendable shapes:
 *   - 0 buttons
 *   - 1-10 reply buttons (1-3 send as reply buttons, 4-10 as a list)
 *   - exactly 1 URL button (cta_url)
 *   - exactly 1 voice_call button (standalone)
 *   - exactly 1 flow button (standalone)
 *
 * Anything else is silently dropped at send time, so callers block save
 * instead. Keep in sync with validateInteractiveButtons in
 * internal/handlers/interactive_buttons.go, which enforces the same rules for
 * non-UI callers.
 */
export function validateWhatsAppButtons(
  buttons: ButtonConfig[],
  t: Translate
): string | null {
  if (!buttons.length) return null

  const reply = buttons.filter(b => !b.type || b.type === 'reply')
  const url = buttons.filter(b => b.type === 'url')
  const phone = buttons.filter(b => b.type === 'phone')
  const voiceCall = buttons.filter(b => b.type === 'voice_call')
  const flow = buttons.filter(b => b.type === 'flow')

  for (const btn of buttons) {
    const title = btn.title?.trim() ?? ''
    if (!title) {
      return t(
        'whatsappButtons.errorMissingTitle',
        'Every button needs a label (shown on the button face).'
      )
    }
    // [...str] counts code points, so multi-byte labels are measured the way
    // Meta measures them rather than by byte length.
    if ([...title].length > MAX_BUTTON_TITLE_LENGTH) {
      return t(
        'whatsappButtons.errorTitleTooLong',
        `Button labels are limited to ${MAX_BUTTON_TITLE_LENGTH} characters.`
      )
    }
  }

  const seen = new Set<string>()
  for (const btn of buttons) {
    if (!btn.id) continue
    if (seen.has(btn.id)) {
      return t(
        'whatsappButtons.errorDuplicateId',
        'Two buttons share the same id. Remove one and add it again.'
      )
    }
    seen.add(btn.id)
  }

  if (voiceCall.length > 1) {
    return t('whatsappButtons.errorMultiVoiceCall', 'Only one Call button is allowed per message.')
  }
  if (voiceCall.length > 0 && buttons.length > voiceCall.length) {
    return t(
      'whatsappButtons.errorVoiceCallExclusive',
      'A Call button cannot be combined with other button types — remove the other buttons or the Call button.'
    )
  }
  if (voiceCall.length === 1) {
    const ttl = voiceCall[0].ttl_minutes ?? 0
    if (ttl < 0 || ttl > 60) {
      return t(
        'whatsappButtons.errorVoiceCallTtl',
        'Call button expiry must be between 1 and 60 minutes.'
      )
    }
  }

  if (flow.length > 1) {
    return t('whatsappButtons.errorMultiFlow', 'Only one Flow button is allowed per message.')
  }
  if (flow.length > 0 && buttons.length > flow.length) {
    return t(
      'whatsappButtons.errorFlowExclusive',
      'A Flow button cannot be combined with other button types — remove the other buttons or the Flow button.'
    )
  }
  if (flow.length === 1 && !flow[0].flow_id) {
    return t('whatsappButtons.errorFlowId', 'Select a published flow for the Flow button.')
  }

  if (phone.length > 0) {
    return t(
      'whatsappButtons.errorPhoneUnsupported',
      'Phone buttons cannot be sent in free-form WhatsApp messages — only in approved templates. Remove the phone button or convert it to a URL.'
    )
  }

  for (const btn of url) {
    if (!isAbsoluteHttpUrl(btn.url)) {
      return t(
        'whatsappButtons.errorInvalidUrl',
        'URL buttons need a full web address starting with http:// or https://'
      )
    }
  }
  if (url.length > 1) {
    return t(
      'whatsappButtons.errorMultiUrl',
      'WhatsApp allows only one URL button per message. Remove the extra URL button.'
    )
  }
  if (reply.length > 0 && url.length > 0) {
    return t(
      'whatsappButtons.errorMixedButtons',
      'Reply and URL buttons cannot be mixed in a single WhatsApp message.'
    )
  }
  if (reply.length > MAX_REPLY_BUTTONS) {
    return t(
      'whatsappButtons.errorTooManyReply',
      `WhatsApp allows at most ${MAX_REPLY_BUTTONS} reply buttons.`
    )
  }
  return null
}

/**
 * A bare host like "example.com" is the easy mistake to make in the UI, and
 * Meta rejects it — the button needs an absolute http(s) URL.
 */
function isAbsoluteHttpUrl(raw?: string): boolean {
  const value = raw?.trim()
  if (!value) return false
  let parsed: URL
  try {
    parsed = new URL(value)
  } catch {
    return false
  }
  return (parsed.protocol === 'http:' || parsed.protocol === 'https:') && !!parsed.host
}
