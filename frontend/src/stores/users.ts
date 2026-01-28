import { defineStore } from 'pinia'
import { usersService } from '@/services/api'
import { createCrudStore } from './createCrudStore'

export interface UserRole {
  id: string
  name: string
  description?: string
  is_system: boolean
}

export interface User {
  id: string
  email: string
  full_name: string
  role_id?: string
  role?: UserRole
  is_active: boolean
  is_super_admin?: boolean
  organization_id: string
  created_at: string
  updated_at: string
}

export interface CreateUserData {
  email: string
  password: string
  full_name: string
  role_id?: string
  is_super_admin?: boolean
}

export interface UpdateUserData {
  email?: string
  password?: string
  full_name?: string
  role_id?: string
  is_active?: boolean
  is_super_admin?: boolean
}

// Create the base CRUD store
const useBaseCrudStore = createCrudStore<User, CreateUserData, UpdateUserData>(
  'users-crud',
  {
    list: () => usersService.list(),
    create: (data) => usersService.create(data),
    update: (id, data) => usersService.update(id, data),
    delete: (id) => usersService.delete(id),
  },
  {
    unwrapList: (response: any) => response.data.data.users || [],
    unwrapSingle: (response: any) => response.data.data,
    entityName: 'User',
  }
)

// Extend with user-specific methods
export const useUsersStore = defineStore('users', () => {
  const baseCrud = useBaseCrudStore()

  // Re-export base CRUD functionality
  const users = baseCrud.items
  const loading = baseCrud.loading
  const error = baseCrud.error
  const fetchUsers = baseCrud.fetchItems
  const createUser = baseCrud.createItem
  const updateUser = baseCrud.updateItem
  const deleteUser = baseCrud.deleteItem

  // User-specific helper methods
  function getUserById(id: string): User | undefined {
    return users.value.find(u => u.id === id)
  }

  function getUsersByRoleId(roleId: string): User[] {
    return users.value.filter(u => u.role_id === roleId)
  }

  function getUsersByRoleName(roleName: string): User[] {
    return users.value.filter(u => u.role?.name === roleName)
  }

  return {
    users,
    loading,
    error,
    fetchUsers,
    createUser,
    updateUser,
    deleteUser,
    getUserById,
    getUsersByRoleId,
    getUsersByRoleName
  }
})
