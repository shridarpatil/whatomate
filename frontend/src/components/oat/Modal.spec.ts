// @vitest-environment happy-dom
import { describe, it, expect, beforeAll, afterEach } from 'vitest'
import { createApp, h, nextTick } from 'vue'
import { createI18n } from 'vue-i18n'
import Modal from './Modal.vue'

const i18n = createI18n({ legacy: false, locale: 'en', messages: { en: { common: { close: 'Close' } } } })

beforeAll(() => {
  const proto = HTMLDialogElement.prototype as any
  if (!proto.showModal) proto.showModal = function () { this.setAttribute('open', '') }
  if (!proto.close) proto.close = function () { this.removeAttribute('open') }
})

let el: HTMLElement | null = null
afterEach(() => { el?.remove(); el = null })

function mount(props: Record<string, unknown>) {
  const emitted: unknown[][] = []
  el = document.createElement('div')
  document.body.appendChild(el)
  createApp({
    render: () => h(Modal, { open: false, ...props, 'onUpdate:open': (v: boolean) => emitted.push([v]) }, { default: () => 'body' }),
  }).use(i18n).mount(el)
  return { dialog: el.querySelector('dialog') as HTMLDialogElement, emitted }
}

describe('Modal', () => {
  it('does not render content while closed', () => {
    const { dialog } = mount({ open: false, title: 'T' })
    expect(dialog).toBeTruthy()
    expect(dialog.textContent).toBe('')
  })

  it('opens when open=true', async () => {
    const { dialog } = mount({ open: true, title: 'T' })
    await nextTick()
    expect(dialog.hasAttribute('open')).toBe(true)
    expect(dialog.querySelector('h3')?.textContent).toBe('T')
  })

  it('Escape syncs open state', async () => {
    const { dialog, emitted } = mount({ open: true })
    await nextTick()
    dialog.dispatchEvent(new Event('cancel'))
    expect(emitted).toEqual([[false]])
  })

  it('alert ignores backdrop click', async () => {
    const { dialog, emitted } = mount({ open: true, alert: true })
    await nextTick()
    expect(dialog.getAttribute('role')).toBe('alertdialog')
    dialog.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    expect(emitted).toEqual([])
  })

  it('backdrop click closes a normal modal but inner clicks do not', async () => {
    const { dialog, emitted } = mount({ open: true })
    await nextTick()
    dialog.querySelector('div')!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    expect(emitted).toEqual([])
    dialog.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    expect(emitted).toEqual([[false]])
  })

  it('renders a close button that emits update:open(false)', async () => {
    const { dialog, emitted } = mount({ open: true, title: 'T' })
    await nextTick()
    const btn = dialog.querySelector('header button') as HTMLButtonElement
    expect(btn.getAttribute('aria-label')).toBe('Close')
    btn.click()
    expect(emitted).toEqual([[false]])
  })

  it('alert modals have no close button', async () => {
    const { dialog } = mount({ open: true, title: 'T', alert: true })
    await nextTick()
    expect(dialog.querySelector('header button')).toBeNull()
  })
})
