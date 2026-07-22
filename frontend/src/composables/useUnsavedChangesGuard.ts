import { type Ref, ref, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'

/**
 * Guards against navigation when there are unsaved changes.
 * Uses a global beforeEach guard with next() callback to match
 * the existing auth guard pattern in router/index.ts.
 */
export function useUnsavedChangesGuard(hasChanges: Ref<boolean>) {
  const router = useRouter()
  const showLeaveDialog = ref(false)
  let pendingRoute: string | null = null
  let guardRemoved = false

  const removeGuard = router.beforeEach((to, from, next) => {
    if (hasChanges.value && to.fullPath !== from.fullPath) {
      pendingRoute = to.fullPath
      showLeaveDialog.value = true
      next(false)
      return
    }
    next()
  })

  function cleanup() {
    if (!guardRemoved) {
      removeGuard()
      guardRemoved = true
    }
  }

  onBeforeUnmount(cleanup)

  function confirmLeave() {
    const target = pendingRoute
    pendingRoute = null
    showLeaveDialog.value = false
    hasChanges.value = false
    cleanup()
    if (target) {
      window.location.href = target
    }
  }

  function cancelLeave() {
    showLeaveDialog.value = false
    pendingRoute = null
  }

  return { showLeaveDialog, confirmLeave, cancelLeave }
}
