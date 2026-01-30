<script setup lang="ts">
import { onMounted, computed, watch } from 'vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { PageHeader, SearchInput, DataTable, PaginationControls, CrudFormDialog, DeleteConfirmDialog, type Column } from '@/components/shared'
import { useUsersStore, type User } from '@/stores/users'
import { useAuthStore } from '@/stores/auth'
import { useRolesStore } from '@/stores/roles'
import { useOrganizationsStore } from '@/stores/organizations'
import { toast } from 'vue-sonner'
import { Plus, Pencil, Trash2, User as UserIcon, Shield, ShieldCheck, UserCog, Users } from 'lucide-vue-next'
import { useCrudState } from '@/composables/useCrudState'
import { useDeepSearch } from '@/composables/useSearch'
import { usePagination } from '@/composables/usePagination'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDate } from '@/lib/utils'
import { ROLE_BADGE_VARIANTS } from '@/lib/constants'

const usersStore = useUsersStore()
const authStore = useAuthStore()
const rolesStore = useRolesStore()
const organizationsStore = useOrganizationsStore()

interface UserFormData {
  email: string
  password: string
  full_name: string
  role_id: string
  is_active: boolean
  is_super_admin: boolean
}

const defaultFormData: UserFormData = { email: '', password: '', full_name: '', role_id: '', is_active: true, is_super_admin: false }

const {
  isLoading, isSubmitting, isDialogOpen, editingItem: editingUser, deleteDialogOpen, itemToDelete: userToDelete,
  formData, openCreateDialog: baseOpenCreateDialog, openEditDialog: baseOpenEditDialog, openDeleteDialog, closeDialog, closeDeleteDialog,
} = useCrudState<User, UserFormData>(defaultFormData)

const { searchQuery, filteredItems: filteredUsers } = useDeepSearch(computed(() => usersStore.users), ['full_name', 'email', 'role.name'])
const { currentPage, paginatedItems: paginatedUsers, totalPages, pageSize, needsPagination } = usePagination(filteredUsers, { pageSize: 20 })

watch(searchQuery, () => { currentPage.value = 1 })

const columns: Column<User>[] = [
  { key: 'user', label: 'User', width: 'w-[300px]' },
  { key: 'role', label: 'Role' },
  { key: 'status', label: 'Status' },
  { key: 'created', label: 'Created' },
  { key: 'actions', label: 'Actions', align: 'right' },
]

const currentUserId = computed(() => authStore.user?.id)
const isSuperAdmin = computed(() => authStore.user?.is_super_admin || false)
const breadcrumbs = [{ label: 'Settings', href: '/settings' }, { label: 'Users' }]
const getDefaultRoleId = () => rolesStore.roles.find(r => r.name === 'agent' && r.is_system)?.id || ''

function openCreateDialog() { formData.value.role_id = getDefaultRoleId(); baseOpenCreateDialog() }
function openEditDialog(user: User) {
  baseOpenEditDialog(user, (u) => ({ email: u.email, password: '', full_name: u.full_name, role_id: u.role_id || '', is_active: u.is_active, is_super_admin: u.is_super_admin || false }))
}

watch(() => organizationsStore.selectedOrgId, () => fetchData())
onMounted(() => fetchData())

async function fetchData() {
  isLoading.value = true
  try { await Promise.all([usersStore.fetchUsers(), rolesStore.fetchRoles()]) }
  catch { toast.error('Failed to load data') }
  finally { isLoading.value = false }
}

async function saveUser() {
  if (!formData.value.email.trim() || !formData.value.full_name.trim()) { toast.error('Please fill in email and name'); return }
  if (!editingUser.value && !formData.value.password.trim()) { toast.error('Password is required for new users'); return }
  if (!formData.value.role_id) { toast.error('Please select a role'); return }

  isSubmitting.value = true
  try {
    const data: Record<string, unknown> = { email: formData.value.email, full_name: formData.value.full_name, role_id: formData.value.role_id }
    if (editingUser.value) {
      data.is_active = formData.value.is_active
      if (formData.value.password) data.password = formData.value.password
      if (isSuperAdmin.value) data.is_super_admin = formData.value.is_super_admin
      await usersStore.updateUser(editingUser.value.id, data)
      toast.success('User updated')
    } else {
      data.password = formData.value.password
      if (isSuperAdmin.value && formData.value.is_super_admin) data.is_super_admin = true
      await usersStore.createUser(data)
      toast.success('User created')
    }
    closeDialog()
  } catch (e) { toast.error(getErrorMessage(e, 'Failed to save user')) }
  finally { isSubmitting.value = false }
}

async function confirmDelete() {
  if (!userToDelete.value) return
  try { await usersStore.deleteUser(userToDelete.value.id); toast.success('User deleted'); closeDeleteDialog() }
  catch (e) { toast.error(getErrorMessage(e, 'Failed to delete user')) }
}

function getRoleBadgeVariant(name: string): 'default' | 'secondary' | 'outline' { return ROLE_BADGE_VARIANTS[name.toLowerCase()] || 'outline' }
function getRoleIcon(name: string) { return { admin: ShieldCheck, manager: Shield }[name.toLowerCase()] || UserCog }
function getRoleName(user: User) { return user.role?.name || 'No role' }
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader title="User Management" :icon="Users" icon-gradient="bg-gradient-to-br from-blue-500 to-indigo-600 shadow-blue-500/20" back-link="/settings" :breadcrumbs="breadcrumbs">
      <template #actions>
        <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />Add User</Button>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto space-y-4">
          <div class="flex items-center gap-4">
            <SearchInput v-model="searchQuery" placeholder="Search by name, email, or role..." class="flex-1 max-w-sm" />
            <div class="text-sm text-muted-foreground">{{ filteredUsers.length }} user{{ filteredUsers.length !== 1 ? 's' : '' }}</div>
          </div>

          <Card>
            <CardHeader>
              <CardTitle>Your Users</CardTitle>
              <CardDescription>Manage team members and their roles. <RouterLink to="/settings/roles" class="text-primary hover:underline">Manage roles</RouterLink></CardDescription>
            </CardHeader>
            <CardContent>
              <DataTable :items="paginatedUsers" :columns="columns" :is-loading="isLoading" :empty-icon="UserIcon" :empty-title="searchQuery ? 'No users found matching your search' : 'No users found'">
                <template #cell-user="{ item: user }">
                  <div class="flex items-center gap-3">
                    <div class="h-9 w-9 rounded-full bg-primary/10 flex items-center justify-center flex-shrink-0">
                      <component :is="getRoleIcon(getRoleName(user))" class="h-4 w-4 text-primary" />
                    </div>
                    <div class="min-w-0">
                      <div class="flex items-center gap-2">
                        <p class="font-medium truncate">{{ user.full_name }}</p>
                        <Badge v-if="user.id === currentUserId" variant="outline" class="text-xs">You</Badge>
                        <Badge v-if="user.is_super_admin" variant="default" class="text-xs">Super Admin</Badge>
                      </div>
                      <p class="text-sm text-muted-foreground truncate">{{ user.email }}</p>
                    </div>
                  </div>
                </template>
                <template #cell-role="{ item: user }">
                  <Badge :variant="getRoleBadgeVariant(getRoleName(user))" class="capitalize">{{ getRoleName(user) }}</Badge>
                </template>
                <template #cell-status="{ item: user }">
                  <Badge variant="outline" :class="user.is_active ? 'border-green-600 text-green-600' : ''">{{ user.is_active ? 'Active' : 'Inactive' }}</Badge>
                </template>
                <template #cell-created="{ item: user }">
                  <span class="text-muted-foreground">{{ formatDate(user.created_at) }}</span>
                </template>
                <template #cell-actions="{ item: user }">
                  <div class="flex items-center justify-end gap-1">
                    <Tooltip><TooltipTrigger as-child><Button variant="ghost" size="icon" class="h-8 w-8" @click="openEditDialog(user)"><Pencil class="h-4 w-4" /></Button></TooltipTrigger><TooltipContent>Edit user</TooltipContent></Tooltip>
                    <Tooltip><TooltipTrigger as-child><Button variant="ghost" size="icon" class="h-8 w-8" @click="openDeleteDialog(user)" :disabled="user.id === currentUserId"><Trash2 class="h-4 w-4 text-destructive" /></Button></TooltipTrigger><TooltipContent>{{ user.id === currentUserId ? "Can't delete yourself" : 'Delete user' }}</TooltipContent></Tooltip>
                  </div>
                </template>
              </DataTable>
            </CardContent>
          </Card>

          <PaginationControls v-if="needsPagination" v-model:current-page="currentPage" :total-pages="totalPages" :total-items="filteredUsers.length" :page-size="pageSize" item-name="users" />
        </div>
      </div>
    </ScrollArea>

    <CrudFormDialog v-model:open="isDialogOpen" :is-editing="!!editingUser" :is-submitting="isSubmitting" edit-title="Edit User" create-title="Add User" edit-description="Update user details and permissions." create-description="Create a new team member account." edit-submit-label="Update User" create-submit-label="Create User" @submit="saveUser">
      <div class="space-y-4">
        <div class="space-y-2"><Label for="full_name">Full Name <span class="text-destructive">*</span></Label><Input id="full_name" v-model="formData.full_name" placeholder="John Doe" /></div>
        <div class="space-y-2"><Label for="email">Email <span class="text-destructive">*</span></Label><Input id="email" v-model="formData.email" type="email" placeholder="john@example.com" /></div>
        <div class="space-y-2"><Label for="password">Password <span v-if="!editingUser" class="text-destructive">*</span><span v-else class="text-muted-foreground">(leave blank to keep existing)</span></Label><Input id="password" v-model="formData.password" type="password" placeholder="Enter password" /></div>
        <div class="space-y-2">
          <Label for="role">Role <span class="text-destructive">*</span></Label>
          <Select v-model="formData.role_id">
            <SelectTrigger>
              <SelectValue placeholder="Select role">
                <template v-if="formData.role_id">
                  <span class="capitalize">{{ rolesStore.roles.find(r => r.id === formData.role_id)?.name }}</span>
                  <Badge v-if="rolesStore.roles.find(r => r.id === formData.role_id)?.is_system" variant="secondary" class="text-xs ml-2">System</Badge>
                </template>
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="role in rolesStore.roles" :key="role.id" :value="role.id">
                <div class="flex items-center gap-2">
                  <span class="capitalize">{{ role.name }}</span>
                  <Badge v-if="role.is_system" variant="secondary" class="text-xs">System</Badge>
                </div>
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div v-if="editingUser" class="flex items-center justify-between"><Label for="is_active" class="font-normal cursor-pointer">Account Active</Label><Switch id="is_active" :checked="formData.is_active" @update:checked="formData.is_active = $event" :disabled="editingUser?.id === currentUserId" /></div>
        <div v-if="isSuperAdmin" class="flex items-center justify-between border-t pt-4"><div><Label for="is_super_admin" class="font-normal cursor-pointer">Super Admin</Label><p class="text-xs text-muted-foreground">Super admins can access all organizations</p></div><Switch id="is_super_admin" :checked="formData.is_super_admin" @update:checked="formData.is_super_admin = $event" :disabled="editingUser?.id === currentUserId && editingUser?.is_super_admin" /></div>
      </div>
    </CrudFormDialog>

    <DeleteConfirmDialog v-model:open="deleteDialogOpen" title="Delete User" :item-name="userToDelete?.full_name" @confirm="confirmDelete" />
  </div>
</template>
