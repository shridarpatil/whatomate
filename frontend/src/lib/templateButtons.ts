// WhatsApp template button rules, in one place so the editor's inline warning and
// the detail view's save guard can't disagree.
//
// Source: Meta "Template components" doc (Buttons section) —
// https://developers.facebook.com/documentation/business-messaging/whatsapp/templates/components
// Flow buttons may be combined with other buttons per Meta's changelog
// (2024: "send message templates with a Flow and other types of buttons").

export const MAX_BUTTONS = 10

// A URL button is dynamic when its url carries a variable. The editor only ever
// creates {{1}}, but a template synced from Meta can arrive with a named parameter,
// and the backend already treats any {{…}} as dynamic (pkg/whatsapp/template.go:
// `strings.Contains(btnURL, "{{")`). Keeping the test here stops the editor and the
// save guard drifting apart, which is how a url could look dynamic on screen and
// still reach Meta with no example.
export const URL_VAR = '{{1}}'
const URL_VAR_PATTERN = /\{\{[^}]+\}\}/

export function isDynamicUrl(url: string): boolean {
  return URL_VAR_PATTERN.test(String(url || ''))
}

// The variable itself — '{{1}}' or '{{order}}'. Empty when the url is static.
export function urlVariable(url: string): string {
  return String(url || '').match(URL_VAR_PATTERN)?.[0] || ''
}

// The url without its variable: the part the user types, and the base that a
// synced full-url example is stripped back to.
export function urlBase(url: string): string {
  return String(url || '').replace(URL_VAR_PATTERN, '')
}

// URL and phone buttons are "call to action" buttons, capped at 2 combined.
export const MAX_CTA = 2

// Per-type maximums. Quick replies have no per-type cap beyond the total; the others
// are hard limits from the components doc (URL: 2, phone: 1, copy code: 1) plus the
// one-per-template rule Meta applies to FLOW and VOICE_CALL.
export const BUTTON_LIMITS: Record<string, number> = {
  URL: 2,
  PHONE_NUMBER: 1,
  COPY_CODE: 1,
  FLOW: 1,
  VOICE_CALL: 1,
}

const TYPE_LABEL: Record<string, string> = {
  URL: 'URL',
  PHONE_NUMBER: 'phone',
  COPY_CODE: 'copy code',
  FLOW: 'Flow',
  VOICE_CALL: 'call',
}

// Returns an empty string when the button set is valid, or a user-facing message for
// the first rule it breaks. OTP buttons belong to authentication templates and are
// validated on their own path, so they're ignored here.
export function validateButtonCombination(buttons: Array<{ type?: string }>): string {
  const list = buttons.filter((b) => b.type !== 'OTP')
  if (list.length === 0) return ''

  if (list.length > MAX_BUTTONS) {
    return `A template can have at most ${MAX_BUTTONS} buttons.`
  }

  const counts: Record<string, number> = {}
  for (const b of list) {
    const type = b.type || ''
    counts[type] = (counts[type] || 0) + 1
  }
  for (const type of Object.keys(BUTTON_LIMITS)) {
    if ((counts[type] || 0) > BUTTON_LIMITS[type]) {
      const max = BUTTON_LIMITS[type]
      const noun = TYPE_LABEL[type] || type
      return max === 1
        ? `A template can have only 1 ${noun} button.`
        : `A template can have at most ${max} ${noun} buttons.`
    }
  }

  // Call-to-action cap: URL and phone buttons together can't exceed 2.
  if ((counts.URL || 0) + (counts.PHONE_NUMBER || 0) > MAX_CTA) {
    return `A template can have at most ${MAX_CTA} call-to-action buttons (URL or phone) combined.`
  }

  // Buttons of the same type must be contiguous — once a type ends and another
  // begins, that first type can't reappear later. Keeps quick replies together and
  // all URL buttons together, which is how Meta groups them.
  let currentType = list[0].type
  const finishedTypes = new Set<string>()
  for (let i = 1; i < list.length; i++) {
    const type = list[i].type
    if (type !== currentType) {
      if (finishedTypes.has(type || '')) {
        return 'Buttons of the same type must be grouped together — keep all quick replies together and all URL buttons together.'
      }
      finishedTypes.add(currentType || '')
      currentType = type
    }
  }

  return ''
}
