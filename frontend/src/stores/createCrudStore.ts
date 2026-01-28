import { defineStore } from 'pinia'
import { ref, type Ref } from 'vue'
import { getErrorMessage } from '@/lib/api-utils'

export interface CrudService<_T, CreateData, UpdateData> {
  list: () => Promise<unknown>
  create?: (data: CreateData) => Promise<unknown>
  update?: (id: string, data: UpdateData) => Promise<unknown>
  delete?: (id: string) => Promise<unknown>
}

export interface CrudStoreOptions<T> {
  /** Function to extract array of items from list response */
  unwrapList: (response: unknown) => T[]
  /** Function to extract single item from create/update response */
  unwrapSingle: (response: unknown) => T
  /** Human-readable entity name for error messages */
  entityName: string
}

export interface CrudStore<T extends { id: string }, CreateData, UpdateData> {
  items: Ref<T[]>
  loading: Ref<boolean>
  error: Ref<string | null>
  fetchItems: () => Promise<void>
  createItem: (data: CreateData) => Promise<T>
  updateItem: (id: string, data: UpdateData) => Promise<T>
  deleteItem: (id: string) => Promise<void>
  getItemById: (id: string) => T | undefined
}

/**
 * Factory function for generating CRUD stores with consistent patterns.
 * Reduces boilerplate by providing standard CRUD operations.
 *
 * @param storeName - Unique name for the Pinia store
 * @param service - Object with API service methods
 * @param options - Configuration options for unwrapping responses
 *
 * @example
 * ```ts
 * export const useUsersStore = createCrudStore<User, CreateUserData, UpdateUserData>(
 *   'users',
 *   {
 *     list: () => usersService.list(),
 *     create: (data) => usersService.create(data),
 *     update: (id, data) => usersService.update(id, data),
 *     delete: (id) => usersService.delete(id),
 *   },
 *   {
 *     unwrapList: (response: any) => response.data.data.users || [],
 *     unwrapSingle: (response: any) => response.data.data,
 *     entityName: 'User',
 *   }
 * )
 * ```
 */
export function createCrudStore<
  T extends { id: string },
  CreateData = Record<string, unknown>,
  UpdateData = Record<string, unknown>
>(
  storeName: string,
  service: CrudService<T, CreateData, UpdateData>,
  options: CrudStoreOptions<T>
) {
  return defineStore(storeName, () => {
    const items = ref<T[]>([]) as Ref<T[]>
    const loading = ref(false)
    const error = ref<string | null>(null)

    async function fetchItems(): Promise<void> {
      loading.value = true
      error.value = null
      try {
        const response = await service.list()
        items.value = options.unwrapList(response)
      } catch (err: unknown) {
        error.value = getErrorMessage(err, `Failed to fetch ${options.entityName.toLowerCase()}s`)
        throw err
      } finally {
        loading.value = false
      }
    }

    async function createItem(data: CreateData): Promise<T> {
      if (!service.create) {
        throw new Error(`Create not supported for ${options.entityName}`)
      }

      loading.value = true
      error.value = null
      try {
        const response = await service.create(data)
        const newItem = options.unwrapSingle(response)
        items.value.unshift(newItem)
        return newItem
      } catch (err: unknown) {
        error.value = getErrorMessage(err, `Failed to create ${options.entityName.toLowerCase()}`)
        throw err
      } finally {
        loading.value = false
      }
    }

    async function updateItem(id: string, data: UpdateData): Promise<T> {
      if (!service.update) {
        throw new Error(`Update not supported for ${options.entityName}`)
      }

      loading.value = true
      error.value = null
      try {
        const response = await service.update(id, data)
        const updatedItem = options.unwrapSingle(response)
        const index = items.value.findIndex((item) => item.id === id)
        if (index !== -1) {
          items.value[index] = updatedItem
        }
        return updatedItem
      } catch (err: unknown) {
        error.value = getErrorMessage(err, `Failed to update ${options.entityName.toLowerCase()}`)
        throw err
      } finally {
        loading.value = false
      }
    }

    async function deleteItem(id: string): Promise<void> {
      if (!service.delete) {
        throw new Error(`Delete not supported for ${options.entityName}`)
      }

      loading.value = true
      error.value = null
      try {
        await service.delete(id)
        items.value = items.value.filter((item) => item.id !== id)
      } catch (err: unknown) {
        error.value = getErrorMessage(err, `Failed to delete ${options.entityName.toLowerCase()}`)
        throw err
      } finally {
        loading.value = false
      }
    }

    function getItemById(id: string): T | undefined {
      return items.value.find((item) => item.id === id)
    }

    return {
      items,
      loading,
      error,
      fetchItems,
      createItem,
      updateItem,
      deleteItem,
      getItemById,
    }
  })
}
