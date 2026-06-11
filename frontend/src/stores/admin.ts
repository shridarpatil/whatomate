import { defineStore } from 'pinia'
import { ref } from 'vue'
import { adminService } from '@/services/api'
import type { UserRole } from '@/stores/users'

export interface AdminOrganization {
  id: string
  name: string
  slug?: string
  user_count: number
  account_count: number
  created_at: string
}

export interface AdminUser {
  id: string
  email: string
  full_name: string
  role_id?: string
  role?: UserRole
  is_active: boolean
  is_super_admin: boolean
  organization_id: string
  organization_name?: string
  org_count: number
  created_at: string
  updated_at: string
}

export interface AdminOrgRole {
  id: string
  name: string
  description?: string
  is_system: boolean
  is_default: boolean
}

export interface FetchAdminOrgsParams {
  search?: string
  page?: number
  limit?: number
}

export interface FetchAdminUsersParams {
  search?: string
  page?: number
  limit?: number
  organization_id?: string
  is_active?: string
}

export const useAdminStore = defineStore('admin', () => {
  const organizations = ref<AdminOrganization[]>([])
  const users = ref<AdminUser[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchOrganizations(params?: FetchAdminOrgsParams) {
    loading.value = true
    error.value = null
    try {
      const response = await adminService.listOrganizations(params)
      const data = response.data.data || response.data
      organizations.value = data.organizations || []
      return {
        organizations: organizations.value,
        total: data.total ?? organizations.value.length,
        page: data.page ?? 1,
        limit: data.limit ?? 50
      }
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to fetch organizations'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function createOrganization(data: {
    name: string
    admin_full_name: string
    admin_email: string
    admin_password: string
  }): Promise<AdminOrganization> {
    const response = await adminService.createOrganization(data)
    return response.data.data || response.data
  }

  async function updateOrganization(id: string, data: { name: string }): Promise<AdminOrganization> {
    const response = await adminService.updateOrganization(id, data)
    const updated = response.data.data || response.data
    const index = organizations.value.findIndex(o => o.id === id)
    if (index !== -1) {
      organizations.value[index] = { ...organizations.value[index], ...updated }
    }
    return updated
  }

  async function fetchOrgRoles(orgId: string): Promise<AdminOrgRole[]> {
    const response = await adminService.listOrgRoles(orgId)
    const data = response.data.data || response.data
    return data.roles || []
  }

  async function fetchUsers(params?: FetchAdminUsersParams) {
    loading.value = true
    error.value = null
    try {
      const response = await adminService.listUsers(params)
      const data = response.data.data || response.data
      users.value = data.users || []
      return {
        users: users.value,
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

  async function createUser(data: {
    organization_id: string
    email: string
    password: string
    full_name: string
    role_id?: string
  }): Promise<AdminUser> {
    const response = await adminService.createUser(data)
    return response.data.data || response.data
  }

  async function setUserStatus(id: string, isActive: boolean): Promise<AdminUser> {
    const response = await adminService.setUserStatus(id, isActive)
    const updated = response.data.data || response.data
    const index = users.value.findIndex(u => u.id === id)
    if (index !== -1) {
      users.value[index] = { ...users.value[index], is_active: updated.is_active }
    }
    return updated
  }

  async function resetUserPassword(id: string, newPassword: string): Promise<void> {
    await adminService.resetUserPassword(id, newPassword)
  }

  return {
    organizations,
    users,
    loading,
    error,
    fetchOrganizations,
    createOrganization,
    updateOrganization,
    fetchOrgRoles,
    fetchUsers,
    createUser,
    setUserStatus,
    resetUserPassword
  }
})
