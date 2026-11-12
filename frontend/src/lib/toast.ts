export interface ToastAction {
  label: string
  onClick: () => void
}

export interface ToastOptions {
  description?: string
  duration?: number
  action?: ToastAction
}

type Variant = 'success' | 'danger' | 'warning' | 'info'

function show(variant: Variant, message: string, opts?: ToastOptions) {
  const options: { variant: Variant; duration?: number; placement: string } = { variant, placement: 'top-right' }
  if (opts?.duration !== undefined) options.duration = opts.duration
  const el = opts?.description
    ? window.ot.toast(opts.description, message, options)
    : window.ot.toast(message, undefined, options)
  if (opts?.action && el) {
    const { label, onClick } = opts.action
    const btn = document.createElement('button')
    btn.type = 'button'
    btn.className = 'small'
    btn.textContent = label
    btn.addEventListener('click', onClick)
    el.appendChild(btn)
  }
}

export const toast = Object.assign(
  (message: string, opts?: ToastOptions) => show('info', message, opts),
  {
    success: (message: string, opts?: ToastOptions) => show('success', message, opts),
    error: (message: string, opts?: ToastOptions) => show('danger', message, opts),
    warning: (message: string, opts?: ToastOptions) => show('warning', message, opts),
    info: (message: string, opts?: ToastOptions) => show('info', message, opts),
  },
)
