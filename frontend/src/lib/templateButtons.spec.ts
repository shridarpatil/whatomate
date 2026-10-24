import { describe, it, expect } from 'vitest'
import {
  validateButtonCombination,
  MAX_BUTTONS,
  MAX_CTA,
  BUTTON_LIMITS,
  URL_VAR,
  isDynamicUrl,
  urlVariable,
  urlBase
} from './templateButtons'

// These rules govern message *templates* (submitted to Meta for approval), not
// free-form interactive messages — see whatsappButtons.ts for those, whose rules
// deliberately differ (phone buttons are illegal there, URL is capped at 1).

const btn = (type: string) => ({ type })
const many = (type: string, n: number) => Array.from({ length: n }, () => btn(type))

describe('validateButtonCombination', () => {
  it('accepts an empty list', () => {
    expect(validateButtonCombination([])).toBe('')
  })

  it('accepts a list that is only OTP buttons', () => {
    // OTP belongs to authentication templates and is validated on its own path,
    // so it is filtered out before any of these rules run.
    expect(validateButtonCombination([btn('OTP')])).toBe('')
  })

  describe('total cap', () => {
    it(`accepts ${MAX_BUTTONS} quick replies`, () => {
      expect(validateButtonCombination(many('QUICK_REPLY', MAX_BUTTONS))).toBe('')
    })

    it(`rejects ${MAX_BUTTONS + 1} buttons`, () => {
      expect(validateButtonCombination(many('QUICK_REPLY', MAX_BUTTONS + 1))).toMatch(
        /at most 10 buttons/
      )
    })

    it('does not count OTP buttons toward the total', () => {
      const buttons = [...many('QUICK_REPLY', MAX_BUTTONS), btn('OTP')]
      expect(validateButtonCombination(buttons)).toBe('')
    })
  })

  describe('per-type caps', () => {
    it('accepts 2 URL buttons', () => {
      expect(validateButtonCombination(many('URL', 2))).toBe('')
    })

    it('rejects 3 URL buttons', () => {
      expect(validateButtonCombination(many('URL', 3))).toMatch(/at most 2 URL buttons/)
    })

    it.each([
      ['PHONE_NUMBER', 'phone'],
      ['COPY_CODE', 'copy code'],
      ['FLOW', 'Flow'],
      ['VOICE_CALL', 'call']
    ])('allows one %s button but not two', (type, noun) => {
      expect(BUTTON_LIMITS[type]).toBe(1)
      expect(validateButtonCombination([btn(type)])).toBe('')
      expect(validateButtonCombination(many(type, 2))).toBe(
        `A template can have only 1 ${noun} button.`
      )
    })
  })

  // The cap the owner's review and Meta's components doc both describe: URL and
  // phone are one budget of 2 between them, on top of the per-type limits. This
  // is the case that is easy to get wrong, because 2 URL and 1 phone each pass
  // their own per-type check and only the combined count catches them.
  describe(`combined call-to-action cap (${MAX_CTA})`, () => {
    it('accepts 2 URL buttons (CTA total 2)', () => {
      expect(validateButtonCombination(many('URL', 2))).toBe('')
    })

    it('accepts 1 URL + 1 phone (CTA total 2)', () => {
      expect(validateButtonCombination([btn('URL'), btn('PHONE_NUMBER')])).toBe('')
    })

    it('rejects 2 URL + 1 phone even though each type is within its own cap', () => {
      const buttons = [btn('URL'), btn('URL'), btn('PHONE_NUMBER')]
      expect(validateButtonCombination(buttons)).toMatch(
        /at most 2 call-to-action buttons/
      )
    })

    it('does not count copy code or Flow as call-to-action buttons', () => {
      const buttons = [btn('URL'), btn('URL'), btn('COPY_CODE')]
      expect(validateButtonCombination(buttons)).toBe('')
    })
  })

  describe('grouping', () => {
    it('accepts quick replies followed by a URL button', () => {
      const buttons = [btn('QUICK_REPLY'), btn('QUICK_REPLY'), btn('URL')]
      expect(validateButtonCombination(buttons)).toBe('')
    })

    it('accepts a URL button followed by quick replies', () => {
      const buttons = [btn('URL'), btn('QUICK_REPLY'), btn('QUICK_REPLY')]
      expect(validateButtonCombination(buttons)).toBe('')
    })

    it('rejects a quick reply after the group has already ended', () => {
      // Buttons send in array order, so QR,URL,QR would reach Meta interleaved.
      const buttons = [btn('QUICK_REPLY'), btn('URL'), btn('QUICK_REPLY')]
      expect(validateButtonCombination(buttons)).toMatch(/must be grouped together/)
    })

    it('rejects a URL button that reappears after another type', () => {
      const buttons = [btn('URL'), btn('QUICK_REPLY'), btn('URL')]
      expect(validateButtonCombination(buttons)).toMatch(/must be grouped together/)
    })
  })

  it('accepts a Flow button combined with other button types', () => {
    // Per Meta's changelog, not the components doc: a Flow button may be sent
    // alongside other buttons in a template (unlike a free-form message).
    const buttons = [btn('QUICK_REPLY'), btn('FLOW')]
    expect(validateButtonCombination(buttons)).toBe('')
  })
})

// The editor only ever creates {{1}} urls, so every {{1}} case below must behave
// exactly as the old literal checks did — these are the equivalence tests. The
// named cases only arrive by syncing a template authored outside whatomate, where
// the backend already treats any {{…}} as dynamic.
describe('dynamic url helpers', () => {
  describe('isDynamicUrl', () => {
    it.each([
      ['', false],
      ['https://example.com', false],
      ['https://example.com/o/', false],
      ['https://example.com/o/{{1}}', true],
      ['{{1}}', true],
      ['https://example.com/o/{{order}}', true],
      ['{{order}}', true]
    ])('%s -> %s', (url, expected) => {
      expect(isDynamicUrl(url)).toBe(expected)
    })

    it('matches the old includes({{1}}) check on every positional url', () => {
      for (const url of ['', 'https://x', 'https://x/{{1}}', '{{1}}', 'https://x/{{1}}/y']) {
        expect(isDynamicUrl(url)).toBe(url.includes(URL_VAR))
      }
    })

    it('tolerates a null or undefined url', () => {
      expect(isDynamicUrl(undefined as never)).toBe(false)
      expect(isDynamicUrl(null as never)).toBe(false)
    })
  })

  describe('urlBase', () => {
    it('matches the old replace({{1}}, "") on every positional url', () => {
      for (const url of ['', 'https://x', 'https://x/{{1}}', '{{1}}', 'https://x/{{1}}/y']) {
        expect(urlBase(url)).toBe(url.replace(URL_VAR, ''))
      }
    })

    it('strips a named variable too', () => {
      expect(urlBase('https://x/o/{{order}}')).toBe('https://x/o/')
    })

    it('is empty when the url is only a variable — what the save guard rejects', () => {
      expect(urlBase('{{1}}')).toBe('')
      expect(urlBase('{{order}}')).toBe('')
    })

    it('leaves a static url untouched', () => {
      expect(urlBase('https://example.com')).toBe('https://example.com')
    })
  })

  describe('urlVariable', () => {
    it('returns the variable the url actually carries, not an assumed {{1}}', () => {
      expect(urlVariable('https://x/{{1}}')).toBe('{{1}}')
      expect(urlVariable('https://x/{{order}}')).toBe('{{order}}')
    })

    it('is empty for a static url, so the chip never renders a phantom variable', () => {
      expect(urlVariable('https://example.com')).toBe('')
      expect(urlVariable('')).toBe('')
    })

    // setUrlPrefix appends `urlVariable(url) || URL_VAR`, so a freshly created
    // dynamic url and an edited one must both come back as {{1}}.
    it('falls back to {{1}} for a url that has no variable yet', () => {
      expect(urlVariable('https://x/') || URL_VAR).toBe(URL_VAR)
      expect(urlVariable('https://x/{{1}}') || URL_VAR).toBe(URL_VAR)
    })
  })
})
