import { toast } from '@/lib/toast'

/**
 * Shared toast helper taking (message, description?) positional args, for
 * components that prefer it over the `toast(message, opts)` API in '@/lib/toast'.
 * Use this instead of `useToast` from the shadcn toast component.
 *
 * Usage:
 *   import { useAppToast } from '@/composables/useAppToast'
 *   const { success, error, info, warning } = useAppToast()
 */
export function useAppToast() {
  return {
    success(message: string, description?: string) {
      toast.success(message, { description })
    },
    error(message: string, description?: string) {
      toast.error(message, { description })
    },
    info(message: string, description?: string) {
      toast.info(message, { description })
    },
    warning(message: string, description?: string) {
      toast.warning(message, { description })
    },
  }
}
