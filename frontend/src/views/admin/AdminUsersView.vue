<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { PageHeader, SearchInput, DataTable, CrudFormDialog, ConfirmDialog, IconButton, ErrorState, type Column } from '@/components/shared'
import { useAdminStore, type AdminUser, type AdminOrgRole } from '@/stores/admin'
import { useAuthStore } from '@/stores/auth'
import { toast } from 'vue-sonner'
import { Plus, KeyRound, UserCheck, UserX, User as UserIcon, Users, Loader2 } from 'lucide-vue-next'
import { useCrudState } from '@/composables/useCrudState'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDate } from '@/lib/utils'
import { useSearchPagination } from '@/composables/useSearchPagination'

const { t } = useI18n()
const route = useRoute()
const adminStore = useAdminStore()
const authStore = useAuthStore()

interface AdminUserFormData {
  organization_id: string
  email: string
  password: string
  full_name: string
  role_id: string
}

const defaultFormData: AdminUserFormData = { organization_id: '', email: '', password: '', full_name: '', role_id: '' }

const {
  isLoading, isSubmitting, isDialogOpen,
  formData, openCreateDialog, closeDialog,
} = useCrudState<AdminUser, AdminUserFormData>(defaultFormData)

const users = ref<AdminUser[]>([])
const error = ref(false)

// Filters. Radix's <SelectItem> can't take value="", so use sentinels.
const ALL_ORGS = '__all'
const ALL_STATUSES = '__all'
const orgFilter = ref<string>((route.query.organization_id as string) || ALL_ORGS)
const statusFilter = ref<string>(ALL_STATUSES)

const { searchQuery, currentPage, totalItems, pageSize, handlePageChange } = useSearchPagination({
  fetchFn: () => fetchUsers(),
})

watch([orgFilter, statusFilter], () => {
  currentPage.value = 1
  fetchUsers()
})

const columns = computed<Column<AdminUser>[]>(() => [
  { key: 'user', label: t('users.user'), width: 'w-[260px]', sortable: true, sortKey: 'full_name' },
  { key: 'email', label: t('common.email'), sortable: true, sortKey: 'email' },
  { key: 'organization', label: t('admin.organization') },
  { key: 'status', label: t('users.status'), sortable: true, sortKey: 'is_active' },
  { key: 'created', label: t('users.created'), sortable: true, sortKey: 'created_at' },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

const sortKey = ref('full_name')
const sortDirection = ref<'asc' | 'desc'>('asc')

const currentUserId = computed(() => authStore.user?.id)

onMounted(async () => {
  await Promise.all([fetchUsers(), fetchOrganizations()])
})

async function fetchOrganizations() {
  try {
    await adminStore.fetchOrganizations({ limit: 100 })
  } catch {
    // Org filter dropdown stays empty; the users list still works.
  }
}

async function fetchUsers() {
  isLoading.value = true
  error.value = false
  try {
    const response = await adminStore.fetchUsers({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize,
      organization_id: orgFilter.value === ALL_ORGS ? undefined : orgFilter.value,
      is_active: statusFilter.value === ALL_STATUSES ? undefined : statusFilter.value,
    })
    users.value = response.users
    totalItems.value = response.total
  } catch {
    toast.error(t('common.failedLoad', { resource: t('resources.users') }))
    error.value = true
  } finally {
    isLoading.value = false
  }
}

// --- Create user ---
const orgRoles = ref<AdminOrgRole[]>([])
const rolesLoading = ref(false)

watch(() => formData.value.organization_id, async (orgId) => {
  formData.value.role_id = ''
  orgRoles.value = []
  if (!orgId) return
  rolesLoading.value = true
  try {
    orgRoles.value = await adminStore.fetchOrgRoles(orgId)
    const defaultRole = orgRoles.value.find(r => r.is_default)
    if (defaultRole) formData.value.role_id = defaultRole.id
  } catch {
    toast.error(t('common.failedLoad', { resource: t('resources.roles') }))
  } finally {
    rolesLoading.value = false
  }
})

async function createUser() {
  if (!formData.value.organization_id) { toast.error(t('admin.selectOrgRequired')); return }
  if (!formData.value.email.trim() || !formData.value.full_name.trim()) { toast.error(t('users.fillEmailName')); return }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.value.email.trim())) { toast.error(t('validation.email')); return }
  if (!formData.value.password.trim()) { toast.error(t('users.passwordRequired')); return }

  isSubmitting.value = true
  try {
    await adminStore.createUser({
      organization_id: formData.value.organization_id,
      email: formData.value.email.trim(),
      password: formData.value.password,
      full_name: formData.value.full_name.trim(),
      role_id: formData.value.role_id || undefined,
    })
    toast.success(t('common.createdSuccess', { resource: t('resources.User') }))
    closeDialog()
    await fetchUsers()
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedSave', { resource: t('resources.user') })))
  } finally {
    isSubmitting.value = false
  }
}

// --- Activate / deactivate ---
const statusDialogOpen = ref(false)
const statusTarget = ref<AdminUser | null>(null)
const isTogglingStatus = ref(false)

function openStatusDialog(user: AdminUser) {
  statusTarget.value = user
  statusDialogOpen.value = true
}

async function confirmToggleStatus() {
  if (!statusTarget.value) return
  isTogglingStatus.value = true
  try {
    await adminStore.setUserStatus(statusTarget.value.id, !statusTarget.value.is_active)
    toast.success(statusTarget.value.is_active ? t('admin.userDeactivated') : t('admin.userActivated'))
    statusDialogOpen.value = false
    statusTarget.value = null
    await fetchUsers()
  } catch (e) {
    toast.error(getErrorMessage(e, t('admin.statusChangeFailed')))
  } finally {
    isTogglingStatus.value = false
  }
}

// --- Reset password ---
const resetDialogOpen = ref(false)
const resetTarget = ref<AdminUser | null>(null)
const newPassword = ref('')
const confirmPassword = ref('')
const isResetting = ref(false)

function openResetDialog(user: AdminUser) {
  resetTarget.value = user
  newPassword.value = ''
  confirmPassword.value = ''
  resetDialogOpen.value = true
}

async function submitResetPassword() {
  if (newPassword.value.length < 6) { toast.error(t('admin.passwordTooShort')); return }
  if (newPassword.value !== confirmPassword.value) { toast.error(t('admin.passwordMismatch')); return }
  if (!resetTarget.value) return

  isResetting.value = true
  try {
    await adminStore.resetUserPassword(resetTarget.value.id, newPassword.value)
    toast.success(t('admin.passwordResetSuccess', { name: resetTarget.value.full_name }))
    resetDialogOpen.value = false
    resetTarget.value = null
  } catch (e) {
    toast.error(getErrorMessage(e, t('admin.passwordResetFailed')))
  } finally {
    isResetting.value = false
  }
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('admin.usersTitle')" :icon="Users" icon-gradient="bg-gradient-to-br from-purple-500 to-indigo-600 shadow-purple-500/20">
      <template #actions>
        <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />{{ $t('users.addUser') }}</Button>
      </template>
    </PageHeader>

    <ErrorState
      v-if="error && !isLoading"
      :title="$t('common.loadErrorTitle')"
      :description="$t('common.loadErrorDescription')"
      :retry-label="$t('common.retryLoad')"
      class="flex-1"
      @retry="fetchUsers"
    />

    <ScrollArea v-else class="flex-1">
      <div class="p-6">
        <Card>
          <CardHeader>
            <div class="flex items-center justify-between flex-wrap gap-4">
              <div>
                <CardTitle>{{ $t('admin.usersTitle') }}</CardTitle>
                <CardDescription>{{ $t('admin.usersSubtitle') }}</CardDescription>
              </div>
              <div class="flex items-center gap-3 flex-wrap">
                <Select v-model="orgFilter">
                  <SelectTrigger class="w-52 h-9">
                    <SelectValue :placeholder="$t('admin.allOrganizations')" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem :value="ALL_ORGS">{{ $t('admin.allOrganizations') }}</SelectItem>
                    <SelectItem v-for="org in adminStore.organizations" :key="org.id" :value="org.id">
                      {{ org.name }}
                    </SelectItem>
                  </SelectContent>
                </Select>
                <Select v-model="statusFilter">
                  <SelectTrigger class="w-36 h-9">
                    <SelectValue :placeholder="$t('admin.allStatuses')" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem :value="ALL_STATUSES">{{ $t('admin.allStatuses') }}</SelectItem>
                    <SelectItem value="true">{{ $t('common.active') }}</SelectItem>
                    <SelectItem value="false">{{ $t('common.inactive') }}</SelectItem>
                  </SelectContent>
                </Select>
                <SearchInput v-model="searchQuery" :placeholder="$t('users.searchUsers') + '...'" class="w-64" />
              </div>
            </div>
          </CardHeader>
          <CardContent>
            <DataTable :items="users" :columns="columns" :is-loading="isLoading" :empty-icon="UserIcon" :empty-title="searchQuery ? $t('users.noMatchingUsers') : $t('users.noUsersFound')" :empty-description="searchQuery ? $t('users.noMatchingUsersDesc') : $t('admin.noUsersDesc')" v-model:sort-key="sortKey" v-model:sort-direction="sortDirection" server-pagination :current-page="currentPage" :total-items="totalItems" :page-size="pageSize" item-name="users" @page-change="handlePageChange">
              <template #cell-user="{ item: user }">
                <div class="flex items-center gap-3">
                  <div class="h-9 w-9 rounded-full bg-primary/10 flex items-center justify-center flex-shrink-0">
                    <UserIcon class="h-4 w-4 text-primary" />
                  </div>
                  <div class="min-w-0">
                    <div class="flex items-center gap-2">
                      <p class="font-medium truncate">{{ user.full_name }}</p>
                      <Badge v-if="user.id === currentUserId" variant="outline" class="text-xs">{{ $t('users.you') }}</Badge>
                      <Badge v-if="user.is_super_admin" variant="default" class="text-xs">{{ $t('users.superAdmin') }}</Badge>
                    </div>
                  </div>
                </div>
              </template>
              <template #cell-email="{ item: user }">
                <span class="text-sm text-muted-foreground truncate">{{ user.email }}</span>
              </template>
              <template #cell-organization="{ item: user }">
                <div class="flex items-center gap-2">
                  <span class="text-sm truncate">{{ user.organization_name || '—' }}</span>
                  <Badge v-if="user.org_count > 1" variant="secondary" class="text-xs">+{{ user.org_count - 1 }}</Badge>
                </div>
              </template>
              <template #cell-status="{ item: user }">
                <Badge variant="outline" :class="user.is_active ? 'border-green-600 text-green-600' : ''">{{ user.is_active ? $t('common.active') : $t('common.inactive') }}</Badge>
              </template>
              <template #cell-created="{ item: user }">
                <span class="text-muted-foreground">{{ formatDate(user.created_at) }}</span>
              </template>
              <template #cell-actions="{ item: user }">
                <div class="flex items-center justify-end gap-1">
                  <IconButton :icon="KeyRound" :label="$t('admin.resetPassword')" class="h-8 w-8" @click="openResetDialog(user)" />
                  <IconButton
                    :label="user.is_active ? $t('admin.deactivateUser') : $t('admin.activateUser')"
                    class="h-8 w-8"
                    :disabled="user.id === currentUserId || (user.is_super_admin && user.is_active)"
                    @click="openStatusDialog(user)"
                  >
                    <component :is="user.is_active ? UserX : UserCheck" class="h-4 w-4" :class="user.is_active ? 'text-destructive' : 'text-green-600'" />
                  </IconButton>
                </div>
              </template>
              <template #empty-action>
                <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />{{ $t('users.addUser') }}</Button>
              </template>
            </DataTable>
          </CardContent>
        </Card>
      </div>
    </ScrollArea>

    <!-- Create user -->
    <CrudFormDialog v-model:open="isDialogOpen" :is-editing="false" :is-submitting="isSubmitting" :edit-title="$t('users.editUserTitle')" :create-title="$t('admin.addUserTitle')" :edit-description="''" :create-description="$t('admin.addUserDesc')" :edit-submit-label="$t('users.updateUser')" :create-submit-label="$t('users.createUser')" @submit="createUser">
      <div class="space-y-4">
        <div class="space-y-2">
          <Label>{{ $t('admin.organization') }} <span class="text-destructive">*</span></Label>
          <Select v-model="formData.organization_id">
            <SelectTrigger>
              <SelectValue :placeholder="$t('admin.selectOrganization')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="org in adminStore.organizations" :key="org.id" :value="org.id">
                {{ org.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="space-y-2"><Label for="admin_full_name">{{ $t('users.fullName') }} <span class="text-destructive">*</span></Label><Input id="admin_full_name" v-model="formData.full_name" :placeholder="$t('users.fullNamePlaceholder')" /></div>
        <div class="space-y-2"><Label for="admin_email">{{ $t('common.email') }} <span class="text-destructive">*</span></Label><Input id="admin_email" v-model="formData.email" type="email" :placeholder="$t('users.emailPlaceholder')" /></div>
        <div class="space-y-2"><Label for="admin_password">{{ $t('users.password') }} <span class="text-destructive">*</span></Label><Input id="admin_password" v-model="formData.password" type="password" :placeholder="$t('users.passwordPlaceholder')" /></div>
        <div class="space-y-2">
          <Label>{{ $t('users.role') }}</Label>
          <Select v-model="formData.role_id" :disabled="!formData.organization_id || rolesLoading">
            <SelectTrigger>
              <SelectValue :placeholder="rolesLoading ? $t('common.loading') : $t('users.selectRole')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="role in orgRoles" :key="role.id" :value="role.id">
                <div class="flex items-center gap-2">
                  <span class="capitalize">{{ role.name }}</span>
                  <Badge v-if="role.is_system" variant="secondary" class="text-xs">{{ $t('users.system') }}</Badge>
                </div>
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
    </CrudFormDialog>

    <!-- Activate / deactivate confirmation -->
    <ConfirmDialog
      v-model:open="statusDialogOpen"
      :title="statusTarget?.is_active ? $t('admin.deactivateUser') : $t('admin.activateUser')"
      :description="statusTarget?.is_active
        ? $t('admin.deactivateUserConfirm', { name: statusTarget?.full_name })
        : $t('admin.activateUserConfirm', { name: statusTarget?.full_name })"
      :confirm-label="statusTarget?.is_active ? $t('admin.deactivate') : $t('admin.activate')"
      :variant="statusTarget?.is_active ? 'destructive' : 'default'"
      :is-submitting="isTogglingStatus"
      @confirm="confirmToggleStatus"
    />

    <!-- Reset password -->
    <Dialog v-model:open="resetDialogOpen">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>{{ $t('admin.resetPassword') }}</DialogTitle>
          <DialogDescription>{{ $t('admin.resetPasswordDesc', { name: resetTarget?.full_name }) }}</DialogDescription>
        </DialogHeader>
        <div class="space-y-4 py-4">
          <div class="space-y-2">
            <Label for="new-password">{{ $t('admin.newPassword') }} <span class="text-destructive">*</span></Label>
            <Input id="new-password" v-model="newPassword" type="password" :placeholder="$t('admin.newPasswordPlaceholder')" />
          </div>
          <div class="space-y-2">
            <Label for="confirm-password">{{ $t('admin.confirmPassword') }} <span class="text-destructive">*</span></Label>
            <Input id="confirm-password" v-model="confirmPassword" type="password" :placeholder="$t('admin.confirmPasswordPlaceholder')" />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="resetDialogOpen = false">{{ $t('common.cancel') }}</Button>
          <Button @click="submitResetPassword" :disabled="isResetting || !newPassword || !confirmPassword">
            <Loader2 v-if="isResetting" class="h-4 w-4 mr-2 animate-spin" />
            {{ $t('admin.resetPassword') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
