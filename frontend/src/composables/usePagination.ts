import { ref, computed, watch, type Ref, type ComputedRef } from 'vue'
import { DEFAULT_PAGE_SIZE } from '@/lib/constants'

export interface PaginationOptions {
  /** Number of items per page (default: 20) */
  pageSize?: number
  /** Whether to reset to page 1 when items change (default: true) */
  resetOnItemsChange?: boolean
}

export interface PaginationInfo {
  start: number
  end: number
  total: number
}

export interface PaginationResult<T> {
  /** Current page number (1-indexed) */
  currentPage: Ref<number>
  /** Number of items per page */
  pageSize: Ref<number>
  /** Items for the current page */
  paginatedItems: ComputedRef<T[]>
  /** Total number of pages */
  totalPages: ComputedRef<number>
  /** Pagination info for display (start, end, total) */
  paginationInfo: ComputedRef<PaginationInfo>
  /** Go to a specific page */
  goToPage: (page: number) => void
  /** Go to the next page */
  nextPage: () => void
  /** Go to the previous page */
  prevPage: () => void
  /** Go to the first page */
  goToFirst: () => void
  /** Go to the last page */
  goToLast: () => void
  /** Whether there's a previous page */
  hasPrevPage: ComputedRef<boolean>
  /** Whether there's a next page */
  hasNextPage: ComputedRef<boolean>
  /** Whether pagination is needed (more than one page) */
  needsPagination: ComputedRef<boolean>
  /** Reset pagination to first page */
  resetPagination: () => void
}

/**
 * Composable for pagination state and logic.
 * Provides page navigation, pagination info, and sliced items.
 *
 * @param items - Ref to the array of items to paginate
 * @param options - Optional configuration
 *
 * @example
 * ```ts
 * const {
 *   currentPage,
 *   paginatedItems,
 *   totalPages,
 *   goToPage,
 *   needsPagination
 * } = usePagination(filteredUsers, { pageSize: 20 })
 * ```
 */
export function usePagination<T>(
  items: Ref<T[]>,
  options: PaginationOptions = {}
): PaginationResult<T> {
  const { pageSize: initialPageSize = DEFAULT_PAGE_SIZE, resetOnItemsChange = true } = options

  const currentPage = ref(1)
  const pageSize = ref(initialPageSize)

  // Reset to page 1 when items change (e.g., after search/filter)
  if (resetOnItemsChange) {
    watch(
      () => items.value.length,
      () => {
        // Only reset if current page is out of bounds
        const maxPage = Math.ceil(items.value.length / pageSize.value) || 1
        if (currentPage.value > maxPage) {
          currentPage.value = maxPage
        }
      }
    )
  }

  const totalPages = computed(() => {
    return Math.ceil(items.value.length / pageSize.value) || 1
  })

  const paginatedItems = computed(() => {
    const start = (currentPage.value - 1) * pageSize.value
    const end = start + pageSize.value
    return items.value.slice(start, end)
  })

  const paginationInfo = computed<PaginationInfo>(() => {
    const total = items.value.length
    if (total === 0) {
      return { start: 0, end: 0, total: 0 }
    }
    const start = (currentPage.value - 1) * pageSize.value + 1
    const end = Math.min(currentPage.value * pageSize.value, total)
    return { start, end, total }
  })

  const hasPrevPage = computed(() => currentPage.value > 1)
  const hasNextPage = computed(() => currentPage.value < totalPages.value)
  const needsPagination = computed(() => totalPages.value > 1)

  function goToPage(page: number): void {
    const clampedPage = Math.min(Math.max(1, page), totalPages.value)
    currentPage.value = clampedPage
  }

  function nextPage(): void {
    if (hasNextPage.value) {
      currentPage.value++
    }
  }

  function prevPage(): void {
    if (hasPrevPage.value) {
      currentPage.value--
    }
  }

  function goToFirst(): void {
    currentPage.value = 1
  }

  function goToLast(): void {
    currentPage.value = totalPages.value
  }

  function resetPagination(): void {
    currentPage.value = 1
  }

  return {
    currentPage,
    pageSize,
    paginatedItems,
    totalPages,
    paginationInfo,
    goToPage,
    nextPage,
    prevPage,
    goToFirst,
    goToLast,
    hasPrevPage,
    hasNextPage,
    needsPagination,
    resetPagination,
  }
}

/**
 * Generate an array of page numbers for pagination UI.
 * Shows first, last, current, and surrounding pages with ellipses.
 *
 * @param currentPage - Current page number
 * @param totalPages - Total number of pages
 * @param siblingCount - Number of pages to show on each side of current (default: 1)
 *
 * @example
 * // For page 5 of 10: [1, '...', 4, 5, 6, '...', 10]
 * const pages = getPageNumbers(5, 10)
 */
export function getPageNumbers(
  currentPage: number,
  totalPages: number,
  siblingCount = 1
): (number | '...')[] {
  const pages: (number | '...')[] = []

  if (totalPages <= 5 + siblingCount * 2) {
    // Show all pages if total is small
    for (let i = 1; i <= totalPages; i++) {
      pages.push(i)
    }
    return pages
  }

  // Always show first page
  pages.push(1)

  // Calculate range around current page
  const leftSibling = Math.max(2, currentPage - siblingCount)
  const rightSibling = Math.min(totalPages - 1, currentPage + siblingCount)

  // Add ellipsis after first page if needed
  if (leftSibling > 2) {
    pages.push('...')
  }

  // Add pages around current
  for (let i = leftSibling; i <= rightSibling; i++) {
    if (i !== 1 && i !== totalPages) {
      pages.push(i)
    }
  }

  // Add ellipsis before last page if needed
  if (rightSibling < totalPages - 1) {
    pages.push('...')
  }

  // Always show last page
  if (totalPages > 1) {
    pages.push(totalPages)
  }

  return pages
}
