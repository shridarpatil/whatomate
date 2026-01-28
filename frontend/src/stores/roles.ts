import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { rolesService, permissionsService, type Role, type Permission } from '@/services/api'
import { createCrudStore } from './createCrudStore'
import { getErrorMessage } from '@/lib/api-utils'
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

// Group permissions by resource for the UI
export interface PermissionGroup {
  resource: string
  label: string
  permissions: Permission[]
}

// Create the base CRUD store
const useBaseCrudStore = createCrudStore<Role, CreateRoleData, UpdateRoleData>(
  'roles-crud',
  {
    list: () => rolesService.list(),
    create: (data) => rolesService.create(data),
    update: (id, data) => rolesService.update(id, data),
    delete: (id) => rolesService.delete(id),
  },
  {
    unwrapList: (response: any) => (response.data as any).data?.roles || response.data?.roles || [],
    unwrapSingle: (response: any) => (response.data as any).data || response.data,
    entityName: 'Role',
  }
)

// Extend with role-specific methods
export const useRolesStore = defineStore('roles', () => {
  const baseCrud = useBaseCrudStore()

  // Re-export base CRUD functionality
  const roles = baseCrud.items
  const loading = baseCrud.loading
  const error = baseCrud.error
  const fetchRoles = baseCrud.fetchItems
  const createRole = baseCrud.createItem
  const updateRole = baseCrud.updateItem
  const deleteRole = baseCrud.deleteItem

  // Additional state for permissions
  const permissions = ref<Permission[]>([])

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

  async function fetchPermissions(): Promise<void> {
    try {
      const response = await permissionsService.list()
      permissions.value = (response.data as any).data?.permissions || response.data?.permissions || []
    } catch (err: any) {
      error.value = getErrorMessage(err, 'Failed to fetch permissions')
      throw err
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
