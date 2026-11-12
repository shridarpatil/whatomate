// @vitest-environment happy-dom
import { describe, it, expect, beforeAll, afterEach } from 'vitest'
import { createApp, h, nextTick } from 'vue'
import Modal from './Modal.vue'

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
  }).mount(el)
  return { dialog: el.querySelector('dialog') as HTMLDialogElement, emitted }
}

describe('Modal', () => {
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
})
