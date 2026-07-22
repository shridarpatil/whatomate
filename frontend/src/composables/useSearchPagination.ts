import { ref, watch } from 'vue'
import { useDebounceFn } from '@vueuse/core'

interface UseSearchPaginationOptions {
  /** Items per page (default: 20) */
  pageSize?: number
  /** Debounce delay in ms (default: 300) */
  debounceMs?: number
  /** Function to call when search/page changes */
  fetchFn: () => void | Promise<void>
}

export function useSearchPagination(options: UseSearchPaginationOptions) {
  const { pageSize = 20, debounceMs = 300, fetchFn } = options

  const searchQuery = ref('')
  const currentPage = ref(1)
  const totalItems = ref(0)

  const debouncedSearch = useDebounceFn(() => {
    currentPage.value = 1
    fetchFn()
  }, debounceMs)

  watch(searchQuery, () => debouncedSearch())

  function handlePageChange(page: number) {
    currentPage.value = page
    fetchFn()
  }

  /** Reset to page 1 and fetch (useful for filter changes) */
  function resetAndFetch() {
    currentPage.value = 1
    fetchFn()
  }

  return {
    searchQuery,
    currentPage,
    totalItems,
    pageSize,
    handlePageChange,
    resetAndFetch,
  }
}
