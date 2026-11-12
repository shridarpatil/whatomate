// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { toast } from './toast'

const otToast = vi.fn()

beforeEach(() => {
  otToast.mockReset()
  ;(window as any).ot = { toast: otToast }
})

describe('toast', () => {
  it('re-raises the toast container popover above open dialogs', () => {
    const container = document.createElement('div')
    const el = document.createElement('div')
    container.appendChild(el)
    const calls: string[] = []
    ;(container as any).hidePopover = () => calls.push('hide')
    ;(container as any).showPopover = () => calls.push('show')
    otToast.mockReturnValue(el)
    toast.info('Hi')
    expect(calls).toEqual(['hide', 'show'])
  })

  it('error maps to danger', () => {
    toast.error('Failed')
    expect(otToast).toHaveBeenCalledWith('Failed', undefined, { variant: 'danger', placement: 'top-right' })
  })

  it('description becomes body', () => {
    toast.success('Saved', { description: 'All good', duration: 2000 })
    expect(otToast).toHaveBeenCalledWith('All good', 'Saved', {
      variant: 'success',
      duration: 2000,
      placement: 'top-right',
    })
  })

  it('plain toast is info', () => {
    toast('Hello')
    expect(otToast).toHaveBeenCalledWith('Hello', undefined, { variant: 'info', placement: 'top-right' })
  })

  it('warning and info map to their variants', () => {
    toast.warning('W')
    toast.info('I')
    expect(otToast.mock.calls[0][2].variant).toBe('warning')
    expect(otToast.mock.calls[1][2].variant).toBe('info')
  })

  it('action appends a button that calls onClick', () => {
    const el = document.createElement('output')
    otToast.mockReturnValue(el)
    const onClick = vi.fn()
    toast.info('Call', { description: 'x', action: { label: 'View', onClick } })
    const btn = el.querySelector('button')!
    expect(btn.textContent).toBe('View')
    btn.click()
    expect(onClick).toHaveBeenCalled()
  })
})
