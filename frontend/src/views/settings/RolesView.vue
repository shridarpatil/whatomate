<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from '@/components/ui/alert-dialog'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger
} from '@/components/ui/tooltip'
import { toast } from 'vue-sonner'
import {
  ArrowLeft,
  Plus,
  Pencil,
  Trash2,
  Loader2,
  Search,
  Shield,
  Users,
  Lock,
  Star
} from 'lucide-vue-next'
import { useRolesStore, type CreateRoleData, type UpdateRoleData } from '@/stores/roles'
import { useOrganizationsStore } from '@/stores/organizations'
import { useAuthStore } from '@/stores/auth'
import type { Role } from '@/services/api'
import PermissionMatrix from '@/components/roles/PermissionMatrix.vue'

const rolesStore = useRolesStore()
const organizationsStore = useOrganizationsStore()
const authStore = useAuthStore()

// Check if current user is super admin
const isSuperAdmin = computed(() => authStore.user?.is_super_admin ?? false)

// Check if we can edit the current role's permissions
const canEditPermissions = computed(() => {
  if (!editingRole.value) return true // New role
  if (!editingRole.value.is_system) return true // Custom role
  return isSuperAdmin.value // System role - only super admin
})

// State
const isLoading = ref(true)
const isDialogOpen = ref(false)
const isSubmitting = ref(false)
const editingRole = ref<Role | null>(null)
const searchQuery = ref('')
const deleteDialogOpen = ref(false)
const roleToDelete = ref<Role | null>(null)

// Form data
const formData = ref<{
  name: string
  description: string
  is_default: boolean
  permissions: string[]
}>({
  name: '',
  description: '',
  is_default: false,
  permissions: []
})

// Computed
const filteredRoles = computed(() => {
  if (!searchQuery.value.trim()) return rolesStore.roles
  const query = searchQuery.value.toLowerCase()
  return rolesStore.roles.filter(
    role =>
      role.name.toLowerCase().includes(query) ||
      role.description.toLowerCase().includes(query)
  )
})

// Watch for dialog close to reset form
watch(isDialogOpen, (open) => {
  if (!open) {
    editingRole.value = null
    resetForm()
  }
})

// Refetch data when organization changes
watch(() => organizationsStore.selectedOrgId, () => {
  fetchData()
})

// Lifecycle
onMounted(async () => {
  await fetchData()
})

async function fetchData() {
  isLoading.value = true
  try {
    await Promise.all([rolesStore.fetchRoles(), rolesStore.fetchPermissions()])
  } catch (error) {
    toast.error('Failed to load roles')
  } finally {
    isLoading.value = false
  }
}

function resetForm() {
  formData.value = {
    name: '',
    description: '',
    is_default: false,
    permissions: []
  }
}

function openCreateDialog() {
  editingRole.value = null
  resetForm()
  isDialogOpen.value = true
}

function openEditDialog(role: Role) {
  editingRole.value = role
  formData.value = {
    name: role.name,
    description: role.description || '',
    is_default: role.is_default,
    permissions: [...role.permissions]
  }
  isDialogOpen.value = true
}

async function saveRole() {
  if (!formData.value.name.trim()) {
    toast.error('Role name is required')
    return
  }

  isSubmitting.value = true
  try {
    if (editingRole.value) {
      const updateData: UpdateRoleData = {
        name: formData.value.name,
        description: formData.value.description,
        is_default: formData.value.is_default,
        permissions: formData.value.permissions
      }
      await rolesStore.updateRole(editingRole.value.id, updateData)
      toast.success('Role updated successfully')
    } else {
      const createData: CreateRoleData = {
        name: formData.value.name,
        description: formData.value.description,
        is_default: formData.value.is_default,
        permissions: formData.value.permissions
      }
      await rolesStore.createRole(createData)
      toast.success('Role created successfully')
    }
    isDialogOpen.value = false
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to save role')
  } finally {
    isSubmitting.value = false
  }
}

function openDeleteDialog(role: Role) {
  roleToDelete.value = role
  deleteDialogOpen.value = true
}

async function confirmDelete() {
  if (!roleToDelete.value) return

  try {
    await rolesStore.deleteRole(roleToDelete.value.id)
    toast.success('Role deleted successfully')
    deleteDialogOpen.value = false
    roleToDelete.value = null
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to delete role')
  }
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  })
}
</script>

<template>
  <TooltipProvider>
    <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
      <!-- Header -->
      <header class="border-b border-white/[0.08] light:border-gray-200 bg-[#0a0a0b]/95 light:bg-white/95 backdrop-blur">
        <div class="flex h-16 items-center px-6">
          <RouterLink to="/settings">
            <Button variant="ghost" size="icon" class="mr-2">
              <ArrowLeft class="h-5 w-5" />
            </Button>
          </RouterLink>
          <div class="h-8 w-8 rounded-lg bg-gradient-to-br from-purple-500 to-indigo-600 flex items-center justify-center mr-3 shadow-lg shadow-purple-500/20">
            <Shield class="h-4 w-4 text-white" />
          </div>
          <div class="flex-1">
            <h1 class="text-xl font-semibold text-white light:text-gray-900">Roles & Permissions</h1>
            <p class="text-sm text-white/50 light:text-gray-500">Manage roles and their permissions</p>
          </div>
          <Button @click="openCreateDialog">
            <Plus class="h-4 w-4 mr-2" />
            Add Role
          </Button>
        </div>
      </header>

      <ScrollArea class="flex-1">
        <div class="p-6">
          <div class="max-w-6xl mx-auto space-y-4">
            <!-- Search -->
            <div class="flex items-center gap-4">
              <div class="relative flex-1 max-w-sm">
                <Search class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input
                  v-model="searchQuery"
                  placeholder="Search roles..."
                  class="pl-9"
                />
              </div>
              <div class="text-sm text-muted-foreground">
                {{ filteredRoles.length }} role{{ filteredRoles.length !== 1 ? 's' : '' }}
              </div>
            </div>

            <!-- Roles Table -->
            <Card>
              <CardHeader>
                <CardTitle>Your Roles</CardTitle>
                <CardDescription>
                  Create custom roles with specific permissions to control what users can access.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Role</TableHead>
                    <TableHead>Description</TableHead>
                    <TableHead class="text-center">Permissions</TableHead>
                    <TableHead class="text-center">Users</TableHead>
                    <TableHead>Created</TableHead>
                    <TableHead class="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow v-if="isLoading">
                    <TableCell colspan="6" class="h-24 text-center">
                      <Loader2 class="h-6 w-6 animate-spin mx-auto text-muted-foreground" />
                    </TableCell>
                  </TableRow>
                  <TableRow v-else-if="filteredRoles.length === 0">
                    <TableCell colspan="6" class="h-24 text-center text-muted-foreground">
                      {{ searchQuery ? 'No roles found matching your search' : 'No roles created yet' }}
                    </TableCell>
                  </TableRow>
                  <TableRow v-else v-for="role in filteredRoles" :key="role.id">
                    <TableCell>
                      <div class="flex items-center gap-2">
                        <span class="font-medium">{{ role.name }}</span>
                        <Badge v-if="role.is_system" variant="secondary">
                          <Lock class="h-3 w-3 mr-1" />
                          System
                        </Badge>
                        <Badge v-if="role.is_default" variant="outline">
                          <Star class="h-3 w-3 mr-1" />
                          Default
                        </Badge>
                      </div>
                    </TableCell>
                    <TableCell class="text-muted-foreground max-w-xs truncate">
                      {{ role.description || '-' }}
                    </TableCell>
                    <TableCell class="text-center">
                      <Badge variant="outline">
                        {{ role.permissions.length }}
                      </Badge>
                    </TableCell>
                    <TableCell class="text-center">
                      <div class="flex items-center justify-center gap-1">
                        <Users class="h-4 w-4 text-muted-foreground" />
                        <span>{{ role.user_count }}</span>
                      </div>
                    </TableCell>
                    <TableCell class="text-muted-foreground">
                      {{ formatDate(role.created_at) }}
                    </TableCell>
                    <TableCell class="text-right">
                      <div class="flex items-center justify-end gap-1">
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              variant="ghost"
                              size="icon"
                              @click="openEditDialog(role)"
                            >
                              <Pencil class="h-4 w-4" />
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>
                            {{ role.is_system ? (isSuperAdmin ? 'Edit permissions' : 'View permissions') : 'Edit role' }}
                          </TooltipContent>
                        </Tooltip>
                        <Tooltip v-if="!role.is_system">
                          <TooltipTrigger asChild>
                            <Button
                              variant="ghost"
                              size="icon"
                              :disabled="role.user_count > 0"
                              @click="openDeleteDialog(role)"
                            >
                              <Trash2 class="h-4 w-4 text-destructive" />
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>
                            {{ role.user_count > 0 ? 'Cannot delete: users assigned' : 'Delete role' }}
                          </TooltipContent>
                        </Tooltip>
                      </div>
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
              </CardContent>
            </Card>
          </div>
        </div>
      </ScrollArea>

      <!-- Create/Edit Dialog -->
      <Dialog v-model:open="isDialogOpen">
        <DialogContent class="max-w-2xl max-h-[90vh] overflow-hidden flex flex-col">
          <DialogHeader>
            <DialogTitle>
              {{ editingRole ? (editingRole.is_system && !isSuperAdmin ? 'View Role' : 'Edit Role') : 'Create Role' }}
            </DialogTitle>
            <DialogDescription>
              {{ editingRole?.is_system
                ? (isSuperAdmin
                  ? 'As a super admin, you can modify permissions for this system role.'
                  : 'System roles cannot be modified, but you can view their permissions.')
                : editingRole
                  ? 'Update the role name, description, and permissions.'
                  : 'Create a new role with custom permissions.'
              }}
            </DialogDescription>
          </DialogHeader>

          <div class="flex-1 overflow-y-auto space-y-4 py-4 pr-2">
            <!-- Name -->
            <div class="space-y-2">
              <Label for="name">
                Name <span class="text-destructive">*</span>
              </Label>
              <Input
                id="name"
                v-model="formData.name"
                placeholder="e.g., Support Lead"
                :disabled="editingRole?.is_system"
              />
            </div>

            <!-- Description -->
            <div class="space-y-2">
              <Label for="description">Description</Label>
              <Textarea
                id="description"
                v-model="formData.description"
                placeholder="Describe what this role is for..."
                rows="2"
                :disabled="editingRole?.is_system && !isSuperAdmin"
              />
            </div>

            <!-- Default Role Toggle -->
            <div v-if="!editingRole?.is_system" class="flex items-center justify-between">
              <div class="space-y-0.5">
                <Label for="is_default" class="font-normal cursor-pointer">
                  Default role for new users
                </Label>
                <p class="text-xs text-muted-foreground">
                  New users will be assigned this role automatically
                </p>
              </div>
              <Switch
                id="is_default"
                :checked="formData.is_default"
                @update:checked="formData.is_default = $event"
              />
            </div>

            <!-- Permissions Matrix -->
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <Label>Permissions</Label>
                <span class="text-xs text-muted-foreground">
                  {{ formData.permissions.length }} selected
                </span>
              </div>
              <p class="text-sm text-muted-foreground mb-3">
                Select the permissions this role should have access to.
              </p>
              <div v-if="rolesStore.permissions.length === 0" class="text-center py-8 text-muted-foreground border rounded-lg">
                <Loader2 class="h-6 w-6 animate-spin mx-auto mb-2" />
                <p>Loading permissions...</p>
              </div>
              <PermissionMatrix
                v-else
                :key="editingRole?.id || 'new'"
                :permission-groups="rolesStore.permissionGroups"
                v-model:selected-permissions="formData.permissions"
                :disabled="!canEditPermissions"
              />
            </div>
          </div>

          <DialogFooter class="pt-4 border-t">
            <Button variant="outline" size="sm" @click="isDialogOpen = false">
              {{ editingRole?.is_system && !isSuperAdmin ? 'Close' : 'Cancel' }}
            </Button>
            <Button
              v-if="!editingRole?.is_system || isSuperAdmin"
              size="sm"
              @click="saveRole"
              :disabled="isSubmitting"
            >
              <Loader2 v-if="isSubmitting" class="h-4 w-4 mr-2 animate-spin" />
              {{ editingRole ? 'Update Role' : 'Create Role' }}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <!-- Delete Confirmation -->
      <AlertDialog v-model:open="deleteDialogOpen">
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Role</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete the role "{{ roleToDelete?.name }}"?
              This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              @click="confirmDelete"
              class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  </TooltipProvider>
</template>
