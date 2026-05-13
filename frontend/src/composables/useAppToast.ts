import { toast } from 'vue-sonner'

/**
 * Shared toast utility wrapping vue-sonner.
 * Use this instead of importing `toast` from 'vue-sonner' directly
 * or `useToast` from the shadcn toast component.
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
