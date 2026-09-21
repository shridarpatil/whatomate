import { describe, it, expect } from 'vitest'
import { nextButtonId, validateWhatsAppButtons } from './whatsappButtons'
import type { ButtonConfig } from '@/types/flow-preview'

// The real callers pass vue-i18n's `t`, which falls back to the second
// argument when the key is missing. Returning the fallback keeps the
// assertions readable and independent of the locale files.
const t = (_key: string, fallback: string) => fallback

const reply = (id: string, title = 'Option'): ButtonConfig => ({ id, title, type: 'reply' })
const url = (id: string, u = 'https://example.com'): ButtonConfig => ({
  id,
  title: 'Docs',
  type: 'url',
  url: u
})

describe('nextButtonId', () => {
  it('starts at btn_1 for an empty list', () => {
    expect(nextButtonId([])).toBe('btn_1')
  })

  it('increments past the highest existing suffix', () => {
    expect(nextButtonId([reply('btn_1'), reply('btn_2')])).toBe('btn_3')
  })

  // The length-based scheme this replaces produced a duplicate here: after
  // deleting btn_1, `buttons.length + 1` is 2 again, so the new button
  // collided with btn_2 and Meta rejected the whole send.
  it('does not reuse an id after an earlier button is deleted', () => {
    const afterDelete = [reply('btn_2')]
    expect(nextButtonId(afterDelete)).toBe('btn_3')
  })

  it('survives repeated add/delete cycles without ever colliding', () => {
    let buttons: ButtonConfig[] = []
    const issued = new Set<string>()

    for (let i = 0; i < 25; i++) {
      const id = nextButtonId(buttons)
      expect(issued.has(id)).toBe(false)
      issued.add(id)
      buttons = [...buttons, reply(id)]
      // Drop the first button every other round to churn the indices.
      if (i % 2 === 1) buttons = buttons.slice(1)
    }
  })

  it('ignores ids that do not follow the btn_<n> shape', () => {
    expect(nextButtonId([{ id: 'custom-id', title: 'Hi', type: 'reply' }])).toBe('btn_1')
  })

  it('ignores a malformed numeric suffix', () => {
    expect(nextButtonId([{ id: 'btn_abc', title: 'Hi', type: 'reply' }])).toBe('btn_1')
  })
})

describe('validateWhatsAppButtons', () => {
  it('accepts an empty list', () => {
    expect(validateWhatsAppButtons([], t)).toBeNull()
  })

  it('accepts a single reply button', () => {
    expect(validateWhatsAppButtons([reply('btn_1')], t)).toBeNull()
  })

  it('accepts a single absolute https url button', () => {
    expect(validateWhatsAppButtons([url('btn_1')], t)).toBeNull()
  })

  it('rejects a url button missing its scheme', () => {
    expect(validateWhatsAppButtons([url('btn_1', 'example.com')], t)).toMatch(/https?:\/\//)
  })

  it('rejects a url button with an empty url', () => {
    expect(validateWhatsAppButtons([url('btn_1', '')], t)).toBeTruthy()
  })

  it('rejects two url buttons', () => {
    expect(validateWhatsAppButtons([url('btn_1'), url('btn_2')], t)).toMatch(/one URL button/i)
  })

  it('rejects mixing reply and url buttons', () => {
    expect(validateWhatsAppButtons([reply('btn_1'), url('btn_2')], t)).toMatch(/mixed/i)
  })

  it('rejects a button with no title', () => {
    expect(validateWhatsAppButtons([{ id: 'btn_1', title: '', type: 'reply' }], t)).toMatch(/label/i)
  })

  it('rejects a title over 20 characters', () => {
    expect(validateWhatsAppButtons([reply('btn_1', 'a'.repeat(21))], t)).toMatch(/20/)
  })

  it('accepts a title of exactly 20 characters', () => {
    expect(validateWhatsAppButtons([reply('btn_1', 'a'.repeat(20))], t)).toBeNull()
  })

  it('measures titles in characters, not bytes', () => {
    expect(validateWhatsAppButtons([reply('btn_1', 'न'.repeat(20))], t)).toBeNull()
    expect(validateWhatsAppButtons([reply('btn_1', 'न'.repeat(21))], t)).toMatch(/20/)
  })

  it('rejects duplicate button ids', () => {
    expect(validateWhatsAppButtons([reply('btn_2', 'A'), reply('btn_2', 'B')], t)).toMatch(
      /same id/i
    )
  })

  it('rejects phone buttons', () => {
    const phone: ButtonConfig = { id: 'btn_1', title: 'Call', type: 'phone', phone_number: '+1' }
    expect(validateWhatsAppButtons([phone], t)).toMatch(/phone/i)
  })

  it('rejects more than 10 reply buttons', () => {
    const many = Array.from({ length: 11 }, (_, i) => reply(`btn_${i + 1}`))
    expect(validateWhatsAppButtons(many, t)).toMatch(/10/)
  })

  it('accepts exactly 10 reply buttons', () => {
    const ten = Array.from({ length: 10 }, (_, i) => reply(`btn_${i + 1}`))
    expect(validateWhatsAppButtons(ten, t)).toBeNull()
  })

  it('rejects a voice_call button combined with a reply button', () => {
    const voice: ButtonConfig = { id: 'btn_1', title: 'Call', type: 'voice_call', ttl_minutes: 15 }
    expect(validateWhatsAppButtons([voice, reply('btn_2')], t)).toMatch(/combined/i)
  })

  it('rejects a flow button without a flow id', () => {
    const flow: ButtonConfig = { id: 'btn_1', title: 'Open', type: 'flow' }
    expect(validateWhatsAppButtons([flow], t)).toMatch(/flow/i)
  })
})
