import { ref, watch, onScopeDispose, type Ref } from 'vue'

export interface ResizablePanelOptions {
  /** localStorage key holding `{ width, collapsed }`. */
  storageKey: string
  defaultWidth: number
  minWidth: number
  maxWidth: number
}

export interface ResizablePanel {
  width: Ref<number>
  collapsed: Ref<boolean>
  isDragging: Ref<boolean>
  toggle: () => void
  expand: () => void
  onHandlePointerDown: (event: PointerEvent) => void
  onHandleKeydown: (event: KeyboardEvent) => void
}

/** Pixels added/removed per arrow-key press on the resize handle. */
const KEYBOARD_STEP = 20

/**
 * State for a right-edge panel the user can collapse and drag-resize.
 * Width and collapsed state survive reloads via localStorage; anything
 * unreadable there falls back to the defaults rather than breaking the view.
 */
export function useResizablePanel(options: ResizablePanelOptions): ResizablePanel {
  const { storageKey, defaultWidth, minWidth, maxWidth } = options

  const clamp = (value: number) => Math.min(maxWidth, Math.max(minWidth, Math.round(value)))

  const width = ref(defaultWidth)
  const collapsed = ref(false)
  const isDragging = ref(false)

  try {
    const raw = localStorage.getItem(storageKey)
    if (raw) {
      const saved = JSON.parse(raw) as { width?: unknown; collapsed?: unknown }
      if (typeof saved.width === 'number' && Number.isFinite(saved.width)) {
        width.value = clamp(saved.width)
      }
      if (typeof saved.collapsed === 'boolean') {
        collapsed.value = saved.collapsed
      }
    }
  } catch {
    // Missing or corrupted entry — the defaults above already apply.
  }

  function persist() {
    try {
      localStorage.setItem(storageKey, JSON.stringify({ width: width.value, collapsed: collapsed.value }))
    } catch {
      // Private mode or quota exceeded — resizing still works for this session.
    }
  }

  watch([width, collapsed], () => {
    // Skip persisting mid-drag: width changes on every native pointermove,
    // and localStorage.setItem is synchronous — onEnd persists the final
    // value once the drag settles instead.
    if (isDragging.value) return
    persist()
  })

  // A drag cut short by unmount never reaches onEnd, so commit the width here.
  onScopeDispose(() => {
    if (isDragging.value) persist()
  })

  function toggle() {
    // Collapsing unmounts the handle, so a drag in flight can never reach onEnd.
    isDragging.value = false
    collapsed.value = !collapsed.value
  }

  function expand() {
    collapsed.value = false
  }

  function onHandlePointerDown(event: PointerEvent) {
    // Also guard on isDragging: a second pointerdown on the handle while a
    // drag is live would register a second onMove/onEnd pair, and since
    // addEventListener('pointermove', ...) isn't filtered by pointerId, one
    // pointer's moves would fire both closures against their own stale
    // startX/startWidth — the later listener wins and width jumps.
    if (collapsed.value || isDragging.value) return

    const handle = event.currentTarget as HTMLElement
    const startX = event.clientX
    const startWidth = width.value

    isDragging.value = true
    handle.setPointerCapture(event.pointerId)
    handle.focus()

    // The panel sits on the right edge, so dragging left widens it.
    function onMove(moveEvent: PointerEvent) {
      width.value = clamp(startWidth - (moveEvent.clientX - startX))
    }

    function onEnd() {
      isDragging.value = false
      if (handle.hasPointerCapture(event.pointerId)) {
        handle.releasePointerCapture(event.pointerId)
      }
      handle.removeEventListener('pointermove', onMove)
      handle.removeEventListener('pointerup', onEnd)
      handle.removeEventListener('pointercancel', onEnd)
      // The watcher skipped every mid-drag change; write the final width now.
      persist()
    }

    handle.addEventListener('pointermove', onMove)
    handle.addEventListener('pointerup', onEnd)
    handle.addEventListener('pointercancel', onEnd)

    event.preventDefault()
  }

  function onHandleKeydown(event: KeyboardEvent) {
    if (event.key === 'ArrowLeft') {
      width.value = clamp(width.value + KEYBOARD_STEP)
      event.preventDefault()
    } else if (event.key === 'ArrowRight') {
      width.value = clamp(width.value - KEYBOARD_STEP)
      event.preventDefault()
    }
  }

  return { width, collapsed, isDragging, toggle, expand, onHandlePointerDown, onHandleKeydown }
}
