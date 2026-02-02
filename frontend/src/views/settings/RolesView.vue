<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { PageHeader, SearchInput, DataTable, DeleteConfirmDialog, type Column } from '@/components/shared'
import { useRolesStore, type CreateRoleData, type UpdateRoleData } from '@/stores/roles'
import { useOrganizationsStore } from '@/stores/organizations'
import { useAuthStore } from '@/stores/auth'
import type { Role } from '@/services/api'
import PermissionMatrix from '@/components/roles/PermissionMatrix.vue'
import { toast } from 'vue-sonner'
import { Plus, Pencil, Trash2, Loader2, Shield, Users, Lock, Star } from 'lucide-vue-next'
import { useCrudState } from '@/composables/useCrudState'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDate } from '@/lib/utils'
import { useDebounceFn } from '@vueuse/core'

const rolesStore = useRolesStore()
const organizationsStore = useOrganizationsStore()
const authStore = useAuthStore()

interface RoleFormData {
  name: string
  description: string
  is_default: boolean
  permissions: string[]
}

const defaultFormData: RoleFormData = { name: '', description: '', is_default: false, permissions: [] }

const {
  isLoading, isSubmitting, isDialogOpen, editingItem: editingRole, deleteDialogOpen, itemToDelete: roleToDelete,
  formData, openCreateDialog, openEditDialog: baseOpenEditDialog, openDeleteDialog, closeDialog, closeDeleteDialog,
} = useCrudState<Role, RoleFormData>(defaultFormData)

const roles = ref<Role[]>([])
const searchQuery = ref('')

// Pagination state
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 20

// Debounced search
const debouncedSearch = useDebounceFn(() => {
  currentPage.value = 1
  fetchRoles()
}, 300)

watch(searchQuery, () => debouncedSearch())

function handlePageChange(page: number) {
  currentPage.value = page
  fetchRoles()
}

const isSuperAdmin = computed(() => authStore.user?.is_super_admin ?? false)
const canEditPermissions = computed(() => {
  if (!editingRole.value) return true
  if (!editingRole.value.is_system) return true
  return isSuperAdmin.value
})

const columns: Column<Role>[] = [
  { key: 'role', label: 'Role', sortable: true, sortKey: 'name' },
  { key: 'description', label: 'Description', sortable: true },
  { key: 'permissions', label: 'Permissions', align: 'center' },
  { key: 'users', label: 'Users', align: 'center', sortable: true, sortKey: 'user_count' },
  { key: 'created', label: 'Created', sortable: true, sortKey: 'created_at' },
  { key: 'actions', label: 'Actions', align: 'right' },
]

// Sorting state
const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

function openEditDialog(role: Role) {
  baseOpenEditDialog(role, (r) => ({ name: r.name, description: r.description || '', is_default: r.is_default, permissions: [...r.permissions] }))
}

watch(() => organizationsStore.selectedOrgId, () => { fetchRoles(); rolesStore.fetchPermissions() })
onMounted(() => { fetchRoles(); rolesStore.fetchPermissions() })

async function fetchRoles() {
  isLoading.value = true
  try {
    const response = await rolesStore.fetchRoles({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    roles.value = response.roles
    totalItems.value = response.total
  } catch { toast.error('Failed to load roles') }
  finally { isLoading.value = false }
}

async function saveRole() {
  if (!formData.value.name.trim()) { toast.error('Role name is required'); return }
  isSubmitting.value = true
  try {
    if (editingRole.value) {
      const updateData: UpdateRoleData = { name: formData.value.name, description: formData.value.description, is_default: formData.value.is_default, permissions: formData.value.permissions }
      await rolesStore.updateRole(editingRole.value.id, updateData)
      toast.success('Role updated successfully')
    } else {
      const createData: CreateRoleData = { name: formData.value.name, description: formData.value.description, is_default: formData.value.is_default, permissions: formData.value.permissions }
      await rolesStore.createRole(createData)
      toast.success('Role created successfully')
    }
    closeDialog()
    await fetchRoles()
  } catch (e) { toast.error(getErrorMessage(e, 'Failed to save role')) }
  finally { isSubmitting.value = false }
}

async function confirmDelete() {
  if (!roleToDelete.value) return
  try { await rolesStore.deleteRole(roleToDelete.value.id); toast.success('Role deleted'); closeDeleteDialog(); await fetchRoles() }
  catch (e) { toast.error(getErrorMessage(e, 'Failed to delete role')) }
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader title="Roles & Permissions" subtitle="Manage roles and their permissions" :icon="Shield" icon-gradient="bg-gradient-to-br from-purple-500 to-indigo-600 shadow-purple-500/20" back-link="/settings">
      <template #actions>
        <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />Add Role</Button>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto">
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>Your Roles</CardTitle>
                  <CardDescription>Create custom roles with specific permissions to control what users can access.</CardDescription>
                </div>
                <SearchInput v-model="searchQuery" placeholder="Search roles..." class="w-64" />
              </div>
            </CardHeader>
            <CardContent>
              <DataTable :items="roles" :columns="columns" :is-loading="isLoading" :empty-icon="Shield" :empty-title="searchQuery ? 'No matching roles' : 'No roles created yet'" :empty-description="searchQuery ? 'No roles match your search.' : 'Create your first role to get started.'" v-model:sort-key="sortKey" v-model:sort-direction="sortDirection" server-pagination :current-page="currentPage" :total-items="totalItems" :page-size="pageSize" item-name="roles" @page-change="handlePageChange">
                <template #cell-role="{ item: role }">
                  <div class="flex items-center gap-2">
                    <span class="font-medium">{{ role.name }}</span>
                    <Badge v-if="role.is_system" variant="secondary"><Lock class="h-3 w-3 mr-1" />System</Badge>
                    <Badge v-if="role.is_default" variant="outline"><Star class="h-3 w-3 mr-1" />Default</Badge>
                  </div>
                </template>
                <template #cell-description="{ item: role }">
                  <span class="text-muted-foreground max-w-xs truncate block">{{ role.description || '-' }}</span>
                </template>
                <template #cell-permissions="{ item: role }">
                  <Badge variant="outline">{{ role.permissions.length }}</Badge>
                </template>
                <template #cell-users="{ item: role }">
                  <div class="flex items-center justify-center gap-1"><Users class="h-4 w-4 text-muted-foreground" /><span>{{ role.user_count }}</span></div>
                </template>
                <template #cell-created="{ item: role }">
                  <span class="text-muted-foreground">{{ formatDate(role.created_at) }}</span>
                </template>
                <template #cell-actions="{ item: role }">
                  <div class="flex items-center justify-end gap-1">
                    <Tooltip><TooltipTrigger as-child><Button variant="ghost" size="icon" class="h-8 w-8" @click="openEditDialog(role)"><Pencil class="h-4 w-4" /></Button></TooltipTrigger><TooltipContent>{{ role.is_system ? (isSuperAdmin ? 'Edit permissions' : 'View permissions') : 'Edit role' }}</TooltipContent></Tooltip>
                    <Tooltip v-if="!role.is_system"><TooltipTrigger as-child><Button variant="ghost" size="icon" class="h-8 w-8" :disabled="role.user_count > 0" @click="openDeleteDialog(role)"><Trash2 class="h-4 w-4 text-destructive" /></Button></TooltipTrigger><TooltipContent>{{ role.user_count > 0 ? 'Cannot delete: users assigned' : 'Delete role' }}</TooltipContent></Tooltip>
                  </div>
                </template>
                <template #empty-action>
                  <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />Add Role</Button>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <!-- Custom Dialog for Roles (has PermissionMatrix) -->
    <Dialog v-model:open="isDialogOpen">
      <DialogContent class="max-w-2xl max-h-[90vh] overflow-hidden flex flex-col">
        <DialogHeader>
          <DialogTitle>{{ editingRole ? (editingRole.is_system && !isSuperAdmin ? 'View Role' : 'Edit Role') : 'Create Role' }}</DialogTitle>
          <DialogDescription>{{ editingRole?.is_system ? (isSuperAdmin ? 'As a super admin, you can modify permissions for this system role.' : 'System roles cannot be modified, but you can view their permissions.') : editingRole ? 'Update the role name, description, and permissions.' : 'Create a new role with custom permissions.' }}</DialogDescription>
        </DialogHeader>
        <div class="flex-1 overflow-y-auto space-y-4 py-4 pr-2">
          <div class="space-y-2"><Label for="name">Name <span class="text-destructive">*</span></Label><Input id="name" v-model="formData.name" placeholder="e.g., Support Lead" :disabled="editingRole?.is_system" /></div>
          <div class="space-y-2"><Label for="description">Description</Label><Textarea id="description" v-model="formData.description" placeholder="Describe what this role is for..." :rows="2" :disabled="editingRole?.is_system && !isSuperAdmin" /></div>
          <div v-if="!editingRole?.is_system" class="flex items-center justify-between">
            <div class="space-y-0.5"><Label for="is_default" class="font-normal cursor-pointer">Default role for new users</Label><p class="text-xs text-muted-foreground">New users will be assigned this role automatically</p></div>
            <Switch id="is_default" :checked="formData.is_default" @update:checked="formData.is_default = $event" />
          </div>
          <div class="space-y-2">
            <div class="flex items-center justify-between"><Label>Permissions</Label><span class="text-xs text-muted-foreground">{{ formData.permissions.length }} selected</span></div>
            <p class="text-sm text-muted-foreground mb-3">Select the permissions this role should have access to.</p>
            <div v-if="rolesStore.permissions.length === 0" class="text-center py-8 text-muted-foreground border rounded-lg"><Loader2 class="h-6 w-6 animate-spin mx-auto mb-2" /><p>Loading permissions...</p></div>
            <PermissionMatrix v-else :key="editingRole?.id || 'new'" :permission-groups="rolesStore.permissionGroups" v-model:selected-permissions="formData.permissions" :disabled="!canEditPermissions" />
          </div>
        </div>
        <DialogFooter class="pt-4 border-t">
          <Button variant="outline" size="sm" @click="isDialogOpen = false">{{ editingRole?.is_system && !isSuperAdmin ? 'Close' : 'Cancel' }}</Button>
          <Button v-if="!editingRole?.is_system || isSuperAdmin" size="sm" @click="saveRole" :disabled="isSubmitting"><Loader2 v-if="isSubmitting" class="h-4 w-4 mr-2 animate-spin" />{{ editingRole ? 'Update Role' : 'Create Role' }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <DeleteConfirmDialog v-model:open="deleteDialogOpen" title="Delete Role" :item-name="roleToDelete?.name" @confirm="confirmDelete" />
  </div>
</template>
