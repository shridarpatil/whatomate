import { defineStore } from 'pinia'
import { ref } from 'vue'
import { usersService } from '@/services/api'

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
  is_member?: boolean
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

export interface FetchUsersParams {
  search?: string
  page?: number
  limit?: number
}

export interface FetchUsersResponse {
  users: User[]
  total: number
  page: number
  limit: number
}

export const useUsersStore = defineStore('users', () => {
  const users = ref<User[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchUsers(params?: FetchUsersParams): Promise<FetchUsersResponse> {
    loading.value = true
    error.value = null
    try {
      const response = await usersService.list(params)
      const data = response.data.data || response.data
      users.value = data.users || []
      return {
        users: data.users || [],
        total: data.total ?? users.value.length,
        page: data.page ?? 1,
        limit: data.limit ?? 50
      }
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to fetch users'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function createUser(data: CreateUserData): Promise<User> {
    loading.value = true
    error.value = null
    try {
      const response = await usersService.create(data)
      const newUser = response.data.data
      users.value.unshift(newUser)
      return newUser
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to create user'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function updateUser(id: string, data: UpdateUserData): Promise<User> {
    loading.value = true
    error.value = null
    try {
      const response = await usersService.update(id, data)
      const updatedUser = response.data.data
      const index = users.value.findIndex(u => u.id === id)
      if (index !== -1) {
        users.value[index] = updatedUser
      }
      return updatedUser
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to update user'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function deleteUser(id: string): Promise<void> {
    loading.value = true
    error.value = null
    try {
      await usersService.delete(id)
      users.value = users.value.filter(u => u.id !== id)
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to delete user'
      throw err
    } finally {
      loading.value = false
    }
  }

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
