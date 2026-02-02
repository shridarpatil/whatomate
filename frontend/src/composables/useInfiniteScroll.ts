import { ref, onUnmounted, type Ref } from 'vue'
import type { ScrollArea } from '@/components/ui/scroll-area'

export interface InfiniteScrollOptions {
  /** Direction to trigger loading: 'top' for older items, 'bottom' for newer items */
  direction?: 'top' | 'bottom'
  /** Distance from edge (in pixels) to trigger loading (default: 100) */
  threshold?: number
  /** Callback when loading is triggered */
  onLoadMore: () => void | Promise<void>
  /** Ref to check if more items are available */
  hasMore: Ref<boolean>
  /** Ref to check if currently loading */
  isLoading: Ref<boolean>
  /** Optional callback on every scroll event (e.g., for sticky headers) */
  onScroll?: (event: Event) => void
}

export interface InfiniteScrollResult {
  /** Ref to attach to ScrollArea component */
  scrollAreaRef: Ref<InstanceType<typeof ScrollArea> | null>
  /** Setup the scroll listener (call after component mounts and ScrollArea renders) */
  setup: () => void
  /** Cleanup the scroll listener */
  cleanup: () => void
  /** Get the scroll viewport element (useful for scroll position manipulation) */
  getViewport: () => HTMLElement | null
  /** Preserve scroll position after prepending items (for 'top' direction) */
  preserveScrollPosition: (callback: () => Promise<void>) => Promise<void>
}

/**
 * Composable for infinite scroll functionality.
 * Works with Reka/Radix UI ScrollArea components.
 *
 * @example
 * ```ts
 * // For loading older messages (scroll to top)
 * const { scrollAreaRef, setup } = useInfiniteScroll({
 *   direction: 'top',
 *   onLoadMore: () => store.fetchOlderMessages(),
 *   hasMore: computed(() => store.hasMoreMessages),
 *   isLoading: computed(() => store.isLoadingOlderMessages)
 * })
 *
 * // For loading more contacts (scroll to bottom)
 * const { scrollAreaRef, setup } = useInfiniteScroll({
 *   direction: 'bottom',
 *   onLoadMore: () => store.loadMoreContacts(),
 *   hasMore: computed(() => store.hasMoreContacts),
 *   isLoading: computed(() => store.isLoadingMoreContacts)
 * })
 * ```
 */
export function useInfiniteScroll(options: InfiniteScrollOptions): InfiniteScrollResult {
  const {
    direction = 'bottom',
    threshold = 100,
    onLoadMore,
    hasMore,
    isLoading,
    onScroll: onScrollCallback
  } = options

  const scrollAreaRef = ref<InstanceType<typeof ScrollArea> | null>(null)
  let scrollViewport: HTMLElement | null = null

  function findScrollViewport(scrollArea: HTMLElement): HTMLElement | null {
    // Try data attributes first (Reka/Radix UI)
    let viewport = scrollArea.querySelector('[data-reka-scroll-area-viewport]') ||
                   scrollArea.querySelector('[data-radix-scroll-area-viewport]')

    if (!viewport) {
      // Fallback: find child element with overflow scroll/auto
      const children = scrollArea.querySelectorAll('*')
      for (const child of children) {
        const style = window.getComputedStyle(child)
        if (style.overflowY === 'scroll' || style.overflowY === 'auto') {
          viewport = child as HTMLElement
          break
        }
      }
    }

    return viewport as HTMLElement | null
  }

  function handleScroll(event: Event) {
    const target = event.target as HTMLElement

    // Call optional scroll callback (e.g., for sticky headers)
    onScrollCallback?.(event)

    if (!hasMore.value || isLoading.value) return

    if (direction === 'top') {
      // Trigger when scrolled near top
      if (target.scrollTop < threshold) {
        onLoadMore()
      }
    } else {
      // Trigger when scrolled near bottom
      const scrollBottom = target.scrollHeight - target.scrollTop - target.clientHeight
      if (scrollBottom < threshold) {
        onLoadMore()
      }
    }
  }

  function setup() {
    const scrollArea = scrollAreaRef.value?.$el
    if (!scrollArea) return

    scrollViewport = findScrollViewport(scrollArea)
    if (scrollViewport) {
      scrollViewport.addEventListener('scroll', handleScroll)
    }
  }

  function cleanup() {
    if (scrollViewport) {
      scrollViewport.removeEventListener('scroll', handleScroll)
      scrollViewport = null
    }
  }

  function getViewport(): HTMLElement | null {
    return scrollViewport
  }

  /**
   * Preserve scroll position when prepending items.
   * Useful for 'top' direction where new items are added above.
   */
  async function preserveScrollPosition(callback: () => Promise<void>) {
    if (!scrollViewport) return

    const currentScrollHeight = scrollViewport.scrollHeight
    const currentScrollTop = scrollViewport.scrollTop

    await callback()

    // Wait for DOM update
    await new Promise(resolve => requestAnimationFrame(resolve))

    const newScrollHeight = scrollViewport.scrollHeight
    scrollViewport.scrollTop = newScrollHeight - currentScrollHeight + currentScrollTop
  }

  // Auto cleanup on unmount
  onUnmounted(cleanup)

  return {
    scrollAreaRef,
    setup,
    cleanup,
    getViewport,
    preserveScrollPosition
  }
}
