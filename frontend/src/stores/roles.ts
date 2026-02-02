import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { rolesService, permissionsService, type Role, type Permission } from '@/services/api'
import { RESOURCE_LABELS } from '@/lib/constants'

export interface CreateRoleData {
  name: string
  description?: string
  is_default?: boolean
  permissions: string[]
}

export interface UpdateRoleData {
  name?: string
  description?: string
  is_default?: boolean
  permissions?: string[]
}

export interface FetchRolesParams {
  search?: string
  page?: number
  limit?: number
}

export interface FetchRolesResponse {
  roles: Role[]
  total: number
  page: number
  limit: number
}

// Group permissions by resource for the UI
export interface PermissionGroup {
  resource: string
  label: string
  permissions: Permission[]
}

export const useRolesStore = defineStore('roles', () => {
  const roles = ref<Role[]>([])
  const permissions = ref<Permission[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Group permissions by resource
  const permissionGroups = computed<PermissionGroup[]>(() => {
    const groups: Record<string, Permission[]> = {}

    for (const perm of permissions.value) {
      if (!groups[perm.resource]) {
        groups[perm.resource] = []
      }
      groups[perm.resource].push(perm)
    }

    return Object.entries(groups)
      .map(([resource, perms]) => ({
        resource,
        label: RESOURCE_LABELS[resource] || resource.charAt(0).toUpperCase() + resource.slice(1),
        permissions: perms.sort((a, b) => a.action.localeCompare(b.action))
      }))
      .sort((a, b) => a.label.localeCompare(b.label))
  })

  async function fetchRoles(params?: FetchRolesParams): Promise<FetchRolesResponse> {
    loading.value = true
    error.value = null
    try {
      const response = await rolesService.list(params)
      const data = (response.data as any).data || response.data
      roles.value = data.roles || []
      return {
        roles: data.roles || [],
        total: data.total ?? roles.value.length,
        page: data.page ?? 1,
        limit: data.limit ?? 50
      }
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to fetch roles'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function fetchPermissions(): Promise<void> {
    try {
      const response = await permissionsService.list()
      permissions.value = (response.data as any).data?.permissions || response.data?.permissions || []
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to fetch permissions'
      throw err
    }
  }

  async function createRole(data: CreateRoleData): Promise<Role> {
    loading.value = true
    error.value = null
    try {
      const response = await rolesService.create(data)
      const newRole = (response.data as any).data || response.data
      roles.value.unshift(newRole)
      return newRole
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to create role'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function updateRole(id: string, data: UpdateRoleData): Promise<Role> {
    loading.value = true
    error.value = null
    try {
      const response = await rolesService.update(id, data)
      const updatedRole = (response.data as any).data || response.data
      const index = roles.value.findIndex(r => r.id === id)
      if (index !== -1) {
        roles.value[index] = updatedRole
      }
      return updatedRole
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to update role'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function deleteRole(id: string): Promise<void> {
    loading.value = true
    error.value = null
    try {
      await rolesService.delete(id)
      roles.value = roles.value.filter(r => r.id !== id)
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to delete role'
      throw err
    } finally {
      loading.value = false
    }
  }

  function getRoleById(id: string): Role | undefined {
    return roles.value.find(r => r.id === id)
  }

  // Check if a role has a specific permission
  function roleHasPermission(role: Role, permissionKey: string): boolean {
    return role.permissions.includes(permissionKey)
  }

  return {
    roles,
    permissions,
    permissionGroups,
    loading,
    error,
    fetchRoles,
    fetchPermissions,
    createRole,
    updateRole,
    deleteRole,
    getRoleById,
    roleHasPermission
  }
})
