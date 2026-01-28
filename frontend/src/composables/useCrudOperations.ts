import { type Ref } from 'vue'
import { toast } from 'vue-sonner'
import { getErrorMessage } from '@/lib/api-utils'

export interface CrudOperationsOptions<T, CreateData = unknown, UpdateData = unknown> {
  /** Function to fetch all items */
  fetchFn: () => Promise<T[]>
  /** Function to create a new item (optional) */
  createFn?: (data: CreateData) => Promise<T>
  /** Function to update an existing item (optional) */
  updateFn?: (id: string, data: UpdateData) => Promise<T>
  /** Function to delete an item (optional) */
  deleteFn?: (id: string) => Promise<void>
  /** Ref to the items array */
  itemsRef: Ref<T[]>
  /** Ref to the loading state */
  loadingRef: Ref<boolean>
  /** Human-readable entity name for toast messages (e.g., 'User', 'Team') */
  entityName: string
  /** Optional callback after successful fetch */
  onFetchSuccess?: (items: T[]) => void
  /** Optional callback after successful create */
  onCreateSuccess?: (item: T) => void
  /** Optional callback after successful update */
  onUpdateSuccess?: (item: T) => void
  /** Optional callback after successful delete */
  onDeleteSuccess?: (id: string) => void
}

export interface CrudOperations<T, CreateData = unknown, UpdateData = unknown> {
  fetchItems: () => Promise<void>
  createItem: (data: CreateData) => Promise<T | null>
  updateItem: (id: string, data: UpdateData) => Promise<T | null>
  deleteItem: (id: string) => Promise<boolean>
}

/**
 * Composable for handling CRUD API operations with automatic loading states and toast notifications.
 *
 * @example
 * ```ts
 * const { fetchItems, createItem, updateItem, deleteItem } = useCrudOperations({
 *   fetchFn: async () => (await usersService.list()).data.data.users,
 *   createFn: async (data) => (await usersService.create(data)).data.data,
 *   updateFn: async (id, data) => (await usersService.update(id, data)).data.data,
 *   deleteFn: async (id) => usersService.delete(id),
 *   itemsRef: items,
 *   loadingRef: isLoading,
 *   entityName: 'User'
 * })
 * ```
 */
export function useCrudOperations<T extends { id: string }, CreateData = unknown, UpdateData = unknown>(
  options: CrudOperationsOptions<T, CreateData, UpdateData>
): CrudOperations<T, CreateData, UpdateData> {
  const {
    fetchFn,
    createFn,
    updateFn,
    deleteFn,
    itemsRef,
    loadingRef,
    entityName,
    onFetchSuccess,
    onCreateSuccess,
    onUpdateSuccess,
    onDeleteSuccess,
  } = options

  /**
   * Fetches all items and updates the items ref
   */
  async function fetchItems(): Promise<void> {
    loadingRef.value = true
    try {
      const items = await fetchFn()
      itemsRef.value = items
      onFetchSuccess?.(items)
    } catch (error) {
      const message = getErrorMessage(error, `Failed to load ${entityName.toLowerCase()}s`)
      toast.error(message)
      throw error
    } finally {
      loadingRef.value = false
    }
  }

  /**
   * Creates a new item
   * @returns The created item or null if creation failed
   */
  async function createItem(data: CreateData): Promise<T | null> {
    if (!createFn) {
      console.warn(`Create function not provided for ${entityName}`)
      return null
    }

    try {
      const newItem = await createFn(data)
      itemsRef.value.unshift(newItem)
      toast.success(`${entityName} created successfully`)
      onCreateSuccess?.(newItem)
      return newItem
    } catch (error) {
      const message = getErrorMessage(error, `Failed to create ${entityName.toLowerCase()}`)
      toast.error(message)
      throw error
    }
  }

  /**
   * Updates an existing item
   * @returns The updated item or null if update failed
   */
  async function updateItem(id: string, data: UpdateData): Promise<T | null> {
    if (!updateFn) {
      console.warn(`Update function not provided for ${entityName}`)
      return null
    }

    try {
      const updatedItem = await updateFn(id, data)
      const index = itemsRef.value.findIndex((item) => item.id === id)
      if (index !== -1) {
        itemsRef.value[index] = updatedItem
      }
      toast.success(`${entityName} updated successfully`)
      onUpdateSuccess?.(updatedItem)
      return updatedItem
    } catch (error) {
      const message = getErrorMessage(error, `Failed to update ${entityName.toLowerCase()}`)
      toast.error(message)
      throw error
    }
  }

  /**
   * Deletes an item
   * @returns true if deletion was successful, false otherwise
   */
  async function deleteItem(id: string): Promise<boolean> {
    if (!deleteFn) {
      console.warn(`Delete function not provided for ${entityName}`)
      return false
    }

    try {
      await deleteFn(id)
      itemsRef.value = itemsRef.value.filter((item) => item.id !== id)
      toast.success(`${entityName} deleted`)
      onDeleteSuccess?.(id)
      return true
    } catch (error) {
      const message = getErrorMessage(error, `Failed to delete ${entityName.toLowerCase()}`)
      toast.error(message)
      throw error
    }
  }

  return {
    fetchItems,
    createItem,
    updateItem,
    deleteItem,
  }
}
