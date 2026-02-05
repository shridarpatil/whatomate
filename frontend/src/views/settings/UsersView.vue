<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { PageHeader, SearchInput, DataTable, CrudFormDialog, DeleteConfirmDialog, type Column } from '@/components/shared'
import { useUsersStore, type User } from '@/stores/users'
import { useAuthStore } from '@/stores/auth'
import { useRolesStore } from '@/stores/roles'
import { useOrganizationsStore } from '@/stores/organizations'
import { toast } from 'vue-sonner'
import { Plus, Pencil, Trash2, User as UserIcon, Shield, ShieldCheck, UserCog, Users } from 'lucide-vue-next'
import { useCrudState } from '@/composables/useCrudState'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDate } from '@/lib/utils'
import { ROLE_BADGE_VARIANTS } from '@/lib/constants'
import { useDebounceFn } from '@vueuse/core'

const { t } = useI18n()

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

const users = ref<User[]>([])
const searchQuery = ref('')

// Pagination state
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 20

// Debounced search
const debouncedSearch = useDebounceFn(() => {
  currentPage.value = 1
  fetchUsers()
}, 300)

watch(searchQuery, () => debouncedSearch())

function handlePageChange(page: number) {
  currentPage.value = page
  fetchUsers()
}

const columns = computed<Column<User>[]>(() => [
  { key: 'user', label: t('users.user'), width: 'w-[300px]', sortable: true, sortKey: 'full_name' },
  { key: 'role', label: t('users.role'), sortable: true, sortKey: 'role.name' },
  { key: 'status', label: t('users.status'), sortable: true, sortKey: 'is_active' },
  { key: 'created', label: t('users.created'), sortable: true, sortKey: 'created_at' },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

// Sorting state
const sortKey = ref('full_name')
const sortDirection = ref<'asc' | 'desc'>('asc')

const currentUserId = computed(() => authStore.user?.id)
const isSuperAdmin = computed(() => authStore.user?.is_super_admin || false)
const breadcrumbs = computed(() => [{ label: t('nav.settings'), href: '/settings' }, { label: t('nav.users') }])
const getDefaultRoleId = () => rolesStore.roles.find(r => r.name === 'agent' && r.is_system)?.id || ''

function openCreateDialog() { formData.value.role_id = getDefaultRoleId(); baseOpenCreateDialog() }
function openEditDialog(user: User) {
  baseOpenEditDialog(user, (u) => ({ email: u.email, password: '', full_name: u.full_name, role_id: u.role_id || '', is_active: u.is_active, is_super_admin: u.is_super_admin || false }))
}

watch(() => organizationsStore.selectedOrgId, () => { fetchUsers(); rolesStore.fetchRoles() })
onMounted(() => { fetchUsers(); rolesStore.fetchRoles() })

async function fetchUsers() {
  isLoading.value = true
  try {
    const response = await usersStore.fetchUsers({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    users.value = response.users
    totalItems.value = response.total
  } catch { toast.error(t('common.failedLoad', { resource: t('resources.users') })) }
  finally { isLoading.value = false }
}

async function saveUser() {
  if (!formData.value.email.trim() || !formData.value.full_name.trim()) { toast.error(t('users.fillEmailName')); return }
  if (!editingUser.value && !formData.value.password.trim()) { toast.error(t('users.passwordRequired')); return }
  if (!formData.value.role_id) { toast.error(t('users.selectRoleRequired')); return }

  isSubmitting.value = true
  try {
    const data: Record<string, unknown> = { email: formData.value.email, full_name: formData.value.full_name, role_id: formData.value.role_id }
    if (editingUser.value) {
      data.is_active = formData.value.is_active
      if (formData.value.password) data.password = formData.value.password
      if (isSuperAdmin.value) data.is_super_admin = formData.value.is_super_admin
      await usersStore.updateUser(editingUser.value.id, data)
      toast.success(t('common.updatedSuccess', { resource: t('resources.User') }))
    } else {
      data.password = formData.value.password
      if (isSuperAdmin.value && formData.value.is_super_admin) data.is_super_admin = true
      await usersStore.createUser(data)
      toast.success(t('common.createdSuccess', { resource: t('resources.User') }))
    }
    closeDialog()
    await fetchUsers()
  } catch (e) { toast.error(getErrorMessage(e, t('common.failedSave', { resource: t('resources.user') }))) }
  finally { isSubmitting.value = false }
}

async function confirmDelete() {
  if (!userToDelete.value) return
  try { await usersStore.deleteUser(userToDelete.value.id); toast.success(t('common.deletedSuccess', { resource: t('resources.User') })); closeDeleteDialog(); await fetchUsers() }
  catch (e) { toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.user') }))) }
}

function getRoleBadgeVariant(name: string): 'default' | 'secondary' | 'outline' { return ROLE_BADGE_VARIANTS[name.toLowerCase()] || 'outline' }
function getRoleIcon(name: string) { return { admin: ShieldCheck, manager: Shield }[name.toLowerCase()] || UserCog }
function getRoleName(user: User) { return user.role?.name || t('users.noRole') }
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('users.title')" :icon="Users" icon-gradient="bg-gradient-to-br from-blue-500 to-indigo-600 shadow-blue-500/20" back-link="/settings" :breadcrumbs="breadcrumbs">
      <template #actions>
        <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />{{ $t('users.addUser') }}</Button>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto">
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>{{ $t('users.yourUsers') }}</CardTitle>
                  <CardDescription>{{ $t('users.subtitle') }}. <RouterLink to="/settings/roles" class="text-primary hover:underline">{{ $t('users.manageRoles') }}</RouterLink></CardDescription>
                </div>
                <SearchInput v-model="searchQuery" :placeholder="$t('users.searchUsers') + '...'" class="w-64" />
              </div>
            </CardHeader>
            <CardContent>
              <DataTable :items="users" :columns="columns" :is-loading="isLoading" :empty-icon="UserIcon" :empty-title="searchQuery ? $t('users.noMatchingUsers') : $t('users.noUsersFound')" :empty-description="searchQuery ? $t('users.noMatchingUsersDesc') : $t('users.noUsersFoundDesc')" v-model:sort-key="sortKey" v-model:sort-direction="sortDirection" server-pagination :current-page="currentPage" :total-items="totalItems" :page-size="pageSize" item-name="users" @page-change="handlePageChange">
                <template #cell-user="{ item: user }">
                  <div class="flex items-center gap-3">
                    <div class="h-9 w-9 rounded-full bg-primary/10 flex items-center justify-center flex-shrink-0">
                      <component :is="getRoleIcon(getRoleName(user))" class="h-4 w-4 text-primary" />
                    </div>
                    <div class="min-w-0">
                      <div class="flex items-center gap-2">
                        <p class="font-medium truncate">{{ user.full_name }}</p>
                        <Badge v-if="user.id === currentUserId" variant="outline" class="text-xs">{{ $t('users.you') }}</Badge>
                        <Badge v-if="user.is_super_admin" variant="default" class="text-xs">{{ $t('users.superAdmin') }}</Badge>
                      </div>
                      <p class="text-sm text-muted-foreground truncate">{{ user.email }}</p>
                    </div>
                  </div>
                </template>
                <template #cell-role="{ item: user }">
                  <Badge :variant="getRoleBadgeVariant(getRoleName(user))" class="capitalize">{{ getRoleName(user) }}</Badge>
                </template>
                <template #cell-status="{ item: user }">
                  <Badge variant="outline" :class="user.is_active ? 'border-green-600 text-green-600' : ''">{{ user.is_active ? $t('common.active') : $t('common.inactive') }}</Badge>
                </template>
                <template #cell-created="{ item: user }">
                  <span class="text-muted-foreground">{{ formatDate(user.created_at) }}</span>
                </template>
                <template #cell-actions="{ item: user }">
                  <div class="flex items-center justify-end gap-1">
                    <Tooltip><TooltipTrigger as-child><Button variant="ghost" size="icon" class="h-8 w-8" @click="openEditDialog(user)"><Pencil class="h-4 w-4" /></Button></TooltipTrigger><TooltipContent>{{ $t('users.editUserTooltip') }}</TooltipContent></Tooltip>
                    <Tooltip><TooltipTrigger as-child><Button variant="ghost" size="icon" class="h-8 w-8" @click="openDeleteDialog(user)" :disabled="user.id === currentUserId"><Trash2 class="h-4 w-4 text-destructive" /></Button></TooltipTrigger><TooltipContent>{{ user.id === currentUserId ? $t('users.cantDeleteYourself') : $t('users.deleteUserTooltip') }}</TooltipContent></Tooltip>
                  </div>
                </template>
                <template #empty-action>
                  <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />{{ $t('users.addUser') }}</Button>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <CrudFormDialog v-model:open="isDialogOpen" :is-editing="!!editingUser" :is-submitting="isSubmitting" :edit-title="$t('users.editUserTitle')" :create-title="$t('users.addUserTitle')" :edit-description="$t('users.editUserDesc')" :create-description="$t('users.addUserDesc')" :edit-submit-label="$t('users.updateUser')" :create-submit-label="$t('users.createUser')" @submit="saveUser">
      <div class="space-y-4">
        <div class="space-y-2"><Label for="full_name">{{ $t('users.fullName') }} <span class="text-destructive">*</span></Label><Input id="full_name" v-model="formData.full_name" :placeholder="$t('users.fullNamePlaceholder')" /></div>
        <div class="space-y-2"><Label for="email">{{ $t('common.email') }} <span class="text-destructive">*</span></Label><Input id="email" v-model="formData.email" type="email" :placeholder="$t('users.emailPlaceholder')" /></div>
        <div class="space-y-2"><Label for="password">{{ $t('users.password') }} <span v-if="!editingUser" class="text-destructive">*</span><span v-else class="text-muted-foreground">{{ $t('users.keepExisting') }}</span></Label><Input id="password" v-model="formData.password" type="password" :placeholder="$t('users.passwordPlaceholder')" /></div>
        <div class="space-y-2">
          <Label for="role">{{ $t('users.role') }} <span class="text-destructive">*</span></Label>
          <Select v-model="formData.role_id">
            <SelectTrigger>
              <SelectValue :placeholder="$t('users.selectRole')">
                <template v-if="formData.role_id">
                  <span class="capitalize">{{ rolesStore.roles.find(r => r.id === formData.role_id)?.name }}</span>
                  <Badge v-if="rolesStore.roles.find(r => r.id === formData.role_id)?.is_system" variant="secondary" class="text-xs ml-2">{{ $t('users.system') }}</Badge>
                </template>
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="role in rolesStore.roles" :key="role.id" :value="role.id">
                <div class="flex items-center gap-2">
                  <span class="capitalize">{{ role.name }}</span>
                  <Badge v-if="role.is_system" variant="secondary" class="text-xs">{{ $t('users.system') }}</Badge>
                </div>
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div v-if="editingUser" class="flex items-center justify-between"><Label for="is_active" class="font-normal cursor-pointer">{{ $t('users.accountActive') }}</Label><Switch id="is_active" :checked="formData.is_active" @update:checked="formData.is_active = $event" :disabled="editingUser?.id === currentUserId" /></div>
        <div v-if="isSuperAdmin" class="flex items-center justify-between border-t pt-4"><div><Label for="is_super_admin" class="font-normal cursor-pointer">{{ $t('users.superAdminLabel') }}</Label><p class="text-xs text-muted-foreground">{{ $t('users.superAdminDesc') }}</p></div><Switch id="is_super_admin" :checked="formData.is_super_admin" @update:checked="formData.is_super_admin = $event" :disabled="editingUser?.id === currentUserId && editingUser?.is_super_admin" /></div>
      </div>
    </CrudFormDialog>

    <DeleteConfirmDialog v-model:open="deleteDialogOpen" :title="$t('users.deleteUser')" :item-name="userToDelete?.full_name" @confirm="confirmDelete" />
  </div>
</template>
