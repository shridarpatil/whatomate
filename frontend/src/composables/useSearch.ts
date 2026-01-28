import { ref, computed, watch, type Ref, type ComputedRef } from 'vue'
import { debounce } from '@/lib/utils'

export interface SearchOptions {
  /** Debounce time in milliseconds (default: 0 for no debounce) */
  debounceMs?: number
  /** Whether to reset search when items change (default: false) */
  resetOnItemsChange?: boolean
}

export interface SearchResult<T> {
  /** The search query string */
  searchQuery: Ref<string>
  /** The filtered items based on search query */
  filteredItems: ComputedRef<T[]>
  /** Clear the search query */
  clearSearch: () => void
  /** Check if search is active */
  isSearchActive: ComputedRef<boolean>
}

/**
 * Composable for reusable search/filter logic.
 * Filters items based on specified fields matching the search query.
 *
 * @param items - Ref to the array of items to search
 * @param searchFields - Array of field names to search within
 * @param options - Optional configuration
 *
 * @example
 * ```ts
 * const { searchQuery, filteredItems } = useSearch(
 *   users,
 *   ['full_name', 'email'],
 *   { debounceMs: 300 }
 * )
 * ```
 */
export function useSearch<T extends Record<string, unknown>>(
  items: Ref<T[]>,
  searchFields: (keyof T)[],
  options: SearchOptions = {}
): SearchResult<T> {
  const { debounceMs = 0, resetOnItemsChange = false } = options

  const searchQuery = ref('')
  const debouncedQuery = ref('')

  // Update debounced query
  if (debounceMs > 0) {
    const updateDebouncedQuery = debounce((value: string) => {
      debouncedQuery.value = value
    }, debounceMs)

    watch(searchQuery, (newValue) => {
      updateDebouncedQuery(newValue)
    })
  }

  // Reset search when items change (optional)
  if (resetOnItemsChange) {
    watch(items, () => {
      searchQuery.value = ''
      debouncedQuery.value = ''
    })
  }

  const filteredItems = computed(() => {
    const query = debounceMs > 0 ? debouncedQuery.value : searchQuery.value
    const trimmedQuery = query.trim().toLowerCase()

    if (!trimmedQuery) {
      return items.value
    }

    return items.value.filter((item) => {
      return searchFields.some((field) => {
        const value = item[field]
        if (value === null || value === undefined) {
          return false
        }

        // Handle nested objects (e.g., item.role.name)
        if (typeof value === 'object') {
          return Object.values(value).some(
            (v) => typeof v === 'string' && v.toLowerCase().includes(trimmedQuery)
          )
        }

        // Handle string values
        if (typeof value === 'string') {
          return value.toLowerCase().includes(trimmedQuery)
        }

        // Handle number values
        if (typeof value === 'number') {
          return value.toString().includes(trimmedQuery)
        }

        return false
      })
    })
  })

  const isSearchActive = computed(() => searchQuery.value.trim().length > 0)

  function clearSearch(): void {
    searchQuery.value = ''
    debouncedQuery.value = ''
  }

  return {
    searchQuery,
    filteredItems,
    clearSearch,
    isSearchActive,
  }
}

/**
 * Extended search with support for nested field paths (e.g., 'role.name')
 *
 * @param items - Ref to the array of items to search
 * @param searchPaths - Array of dot-notation paths to search within
 * @param options - Optional configuration
 *
 * @example
 * ```ts
 * const { searchQuery, filteredItems } = useDeepSearch(
 *   users,
 *   ['full_name', 'email', 'role.name'],
 *   { debounceMs: 300 }
 * )
 * ```
 */
export function useDeepSearch<T>(
  items: Ref<T[]>,
  searchPaths: string[],
  options: SearchOptions = {}
): SearchResult<T> {
  const { debounceMs = 0, resetOnItemsChange = false } = options

  const searchQuery = ref('')
  const debouncedQuery = ref('')

  if (debounceMs > 0) {
    const updateDebouncedQuery = debounce((value: string) => {
      debouncedQuery.value = value
    }, debounceMs)

    watch(searchQuery, (newValue) => {
      updateDebouncedQuery(newValue)
    })
  }

  if (resetOnItemsChange) {
    watch(items, () => {
      searchQuery.value = ''
      debouncedQuery.value = ''
    })
  }

  function getNestedValue(obj: unknown, path: string): unknown {
    return path.split('.').reduce((acc: unknown, part: string) => {
      if (acc && typeof acc === 'object' && part in acc) {
        return (acc as Record<string, unknown>)[part]
      }
      return undefined
    }, obj)
  }

  const filteredItems = computed(() => {
    const query = debounceMs > 0 ? debouncedQuery.value : searchQuery.value
    const trimmedQuery = query.trim().toLowerCase()

    if (!trimmedQuery) {
      return items.value
    }

    return items.value.filter((item) => {
      return searchPaths.some((path) => {
        const value = getNestedValue(item, path)
        if (value === null || value === undefined) {
          return false
        }

        if (typeof value === 'string') {
          return value.toLowerCase().includes(trimmedQuery)
        }

        if (typeof value === 'number') {
          return value.toString().includes(trimmedQuery)
        }

        return false
      })
    })
  })

  const isSearchActive = computed(() => searchQuery.value.trim().length > 0)

  function clearSearch(): void {
    searchQuery.value = ''
    debouncedQuery.value = ''
  }

  return {
    searchQuery,
    filteredItems,
    clearSearch,
    isSearchActive,
  }
}
