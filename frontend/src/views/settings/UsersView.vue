<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { PageHeader, SearchInput, DataTable, CrudFormDialog, DeleteConfirmDialog, IconButton, ErrorState, type Column } from '@/components/shared'
import Modal from '@/components/oat/Modal.vue'
import { useUsersStore, type User } from '@/stores/users'
import { useAuthStore } from '@/stores/auth'
import { useRolesStore } from '@/stores/roles'
import { useOrganizationsStore } from '@/stores/organizations'
import { toast } from '@/lib/toast'
import { Plus, Pencil, Trash2, UserMinus, User as UserIcon, Shield, ShieldCheck, UserCog, Users, Link, UserPlus } from '@/lib/icons'
import { useCrudState } from '@/composables/useCrudState'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDate } from '@/lib/utils'
import { useSearchPagination } from '@/composables/useSearchPagination'

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
  isLoading, isSubmitting, isDialogOpen, deleteDialogOpen, itemToDelete: userToDelete,
  formData, openCreateDialog: baseOpenCreateDialog, openDeleteDialog, closeDialog, closeDeleteDialog,
} = useCrudState<User, UserFormData>(defaultFormData)

const users = ref<User[]>([])
const isDeleting = ref(false)
const error = ref(false)
const onlineCount = ref(0)

// Empty string means "no role filter" and is translated to undefined before hitting the API.
const roleFilter = ref<string>('')
const onlineOnly = ref(false)

const { searchQuery, currentPage, totalItems, pageSize, handlePageChange } = useSearchPagination({
  fetchFn: () => fetchUsers(),
})

// Re-fetch (and reset to page 1) whenever a filter changes.
watch([roleFilter, onlineOnly], () => {
  currentPage.value = 1
  fetchUsers()
})

const columns = computed<Column<User>[]>(() => [
  { key: 'user', label: t('users.user'), sortable: true, sortKey: 'full_name' },
  { key: 'email', label: t('common.email'), sortable: true, sortKey: 'email' },
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

watch(() => organizationsStore.selectedOrgId, () => { fetchUsers(); rolesStore.fetchRoles() })
onMounted(() => { fetchUsers(); rolesStore.fetchRoles() })

async function fetchUsers() {
  isLoading.value = true
  error.value = false
  try {
    const response = await usersStore.fetchUsers({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize,
      role_id: roleFilter.value || undefined,
      online_only: onlineOnly.value || undefined,
    })
    users.value = response.users
    totalItems.value = response.total
    onlineCount.value = response.online_count
  } catch {
    toast.error(t('common.failedLoad', { resource: t('resources.users') }))
    error.value = true
  } finally { isLoading.value = false }
}

async function createUser() {
  if (!formData.value.email.trim() || !formData.value.full_name.trim()) { toast.error(t('users.fillEmailName')); return }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.value.email.trim())) { toast.error(t('validation.email')); return }
  if (!formData.value.password.trim()) { toast.error(t('users.passwordRequired')); return }
  if (!formData.value.role_id) { toast.error(t('users.selectRoleRequired')); return }

  isSubmitting.value = true
  try {
    await usersStore.createUser({
      email: formData.value.email,
      password: formData.value.password,
      full_name: formData.value.full_name,
      role_id: formData.value.role_id || undefined,
      is_super_admin: isSuperAdmin.value && formData.value.is_super_admin ? true : undefined,
    })
    toast.success(t('common.createdSuccess', { resource: t('resources.User') }))
    closeDialog()
    await fetchUsers()
  } catch (e) { toast.error(getErrorMessage(e, t('common.failedSave', { resource: t('resources.user') }))) }
  finally { isSubmitting.value = false }
}

async function confirmDelete() {
  if (!userToDelete.value) return
  const isMemberRemoval = userToDelete.value.is_member
  isDeleting.value = true
  try {
    await usersStore.deleteUser(userToDelete.value.id)
    toast.success(isMemberRemoval ? t('users.memberRemoved') : t('common.deletedSuccess', { resource: t('resources.User') }))
    closeDeleteDialog()
    await fetchUsers()
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.user') })))
  } finally {
    isDeleting.value = false
  }
}

function getRoleIcon(name: string) { return { admin: ShieldCheck, manager: Shield }[name.toLowerCase()] || UserCog }
function getRoleName(user: User) { return user.role?.name || t('users.noRole') }

// Add existing user dialog
const isAddExistingOpen = ref(false)
const addExistingEmail = ref('')
const addExistingRoleId = ref('')
const isAddExistingSubmitting = ref(false)

function openAddExistingDialog() {
  addExistingEmail.value = ''
  addExistingRoleId.value = ''
  isAddExistingOpen.value = true
}

async function submitAddExisting() {
  if (!addExistingEmail.value.trim()) {
    toast.error(t('users.enterEmail'))
    return
  }
  isAddExistingSubmitting.value = true
  try {
    await organizationsStore.addMember({
      email: addExistingEmail.value.trim(),
      role_id: addExistingRoleId.value || undefined,
    })
    toast.success(t('users.existingUserAdded'))
    isAddExistingOpen.value = false
    await fetchUsers()
  } catch (e) {
    toast.error(getErrorMessage(e, t('users.addExistingFailed')))
  } finally {
    isAddExistingSubmitting.value = false
  }
}

async function copyInviteLink() {
  const orgId = organizationsStore.selectedOrgId || authStore.organizationId
  const basePath = ((window as any).__BASE_PATH__ ?? '').replace(/\/$/, '')
  const url = `${window.location.origin}${basePath}/register?org=${orgId}`
  try {
    await navigator.clipboard.writeText(url)
    toast.success(t('users.inviteLinkCopied'))
  } catch {
    toast.error(t('common.clipboardFailed'))
  }
}
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader :title="$t('users.title')" :icon="Users" back-link="/settings" :breadcrumbs="breadcrumbs">
      <template #actions>
        <button type="button" class="outline small" @click="copyInviteLink"><Link :size="16" />{{ $t('users.copyInviteLink') }}</button>
        <button v-if="organizationsStore.isMultiOrg && authStore.hasPermission('organizations', 'assign')" type="button" class="outline small" @click="openAddExistingDialog"><UserPlus :size="16" />{{ $t('users.addExistingUser') }}</button>
        <button type="button" class="outline small" @click="openCreateDialog"><Plus :size="16" />{{ $t('users.addUser') }}</button>
      </template>
    </PageHeader>

    <!-- Error State -->
    <ErrorState
      v-if="error && !isLoading"
      :title="$t('common.loadErrorTitle')"
      :description="$t('common.loadErrorDescription')"
      :retry-label="$t('common.retryLoad')"
      class="flex-1"
      @retry="fetchUsers"
    />

    <div v-else class="flex-1 scroll-y p-4">
      <article class="card">
        <header class="hstack justify-between flex-wrap gap-4">
          <div>
            <div class="hstack gap-2">
              <h3>{{ $t('users.yourUsers') }}</h3>
              <span class="badge" data-variant="success">{{ onlineCount }} {{ $t('users.online', 'online') }}</span>
            </div>
            <p><RouterLink to="/settings/roles">{{ $t('users.manageRoles') }}</RouterLink></p>
          </div>
          <div class="hstack gap-2 flex-wrap">
            <select class="w-auto" v-model="roleFilter" :aria-label="$t('users.allRoles', 'All roles')">
              <option value="">{{ $t('users.allRoles', 'All roles') }}</option>
              <option v-for="role in rolesStore.roles" :key="role.id" :value="role.id">{{ role.name }}</option>
            </select>
            <label class="hstack gap-2">
              <input v-model="onlineOnly" type="checkbox" role="switch" />
              <span>{{ $t('users.onlineOnly', 'Online only') }}</span>
            </label>
            <SearchInput v-model="searchQuery" :placeholder="$t('users.searchUsers') + '...'" />
          </div>
        </header>
        <DataTable :items="users" :columns="columns" :is-loading="isLoading" :empty-icon="UserIcon" :empty-title="searchQuery ? $t('users.noMatchingUsers') : $t('users.noUsersFound')" :empty-description="searchQuery ? $t('users.noMatchingUsersDesc') : $t('users.noUsersFoundDesc')" v-model:sort-key="sortKey" v-model:sort-direction="sortDirection" server-pagination :current-page="currentPage" :total-items="totalItems" :page-size="pageSize" item-name="users" @page-change="handlePageChange">
          <template #cell-user="{ item: user }">
            <RouterLink :to="`/settings/users/${user.id}`" class="hstack gap-2">
              <component :is="getRoleIcon(getRoleName(user))" :size="20" />
              <div class="hstack gap-2 min-w-0">
                <strong class="truncate">{{ user.full_name }}</strong>
                <span v-if="user.id === currentUserId" class="badge outline">{{ $t('users.you') }}</span>
                <span v-if="user.is_super_admin" class="badge">{{ $t('users.superAdmin') }}</span>
                <span v-if="user.is_member" class="badge" data-variant="secondary">{{ $t('users.member') }}</span>
              </div>
            </RouterLink>
          </template>
          <template #cell-email="{ item: user }">
            <span class="truncate text-light">{{ user.email }}</span>
          </template>
          <template #cell-role="{ item: user }">
            <span class="badge" data-variant="secondary">{{ getRoleName(user) }}</span>
          </template>
          <template #cell-status="{ item: user }">
            <span class="badge" :data-variant="user.is_active ? 'success' : 'secondary'">{{ user.is_active ? $t('common.active') : $t('common.inactive') }}</span>
          </template>
          <template #cell-created="{ item: user }">
            <span class="text-light">{{ formatDate(user.created_at) }}</span>
          </template>
          <template #cell-actions="{ item: user }">
            <div class="hstack gap-1 justify-end">
              <RouterLink :to="`/settings/users/${user.id}`" class="button ghost icon" :title="$t('users.editUserTooltip')" :aria-label="$t('users.editUserTooltip')">
                <Pencil :size="16" />
              </RouterLink>
              <IconButton
                :label="user.is_member
                  ? $t('users.removeMemberTooltip')
                  : (user.id === currentUserId ? $t('users.cantDeleteYourself') : $t('users.deleteUserTooltip'))"
                :disabled="user.id === currentUserId"
                @click="openDeleteDialog(user)"
              >
                <component :is="user.is_member ? UserMinus : Trash2" :size="16" />
              </IconButton>
            </div>
          </template>
          <template #empty-action>
            <button type="button" class="outline small" @click="openCreateDialog"><Plus :size="16" />{{ $t('users.addUser') }}</button>
          </template>
        </DataTable>
      </article>
    </div>

    <CrudFormDialog v-model:open="isDialogOpen" :is-editing="false" :is-submitting="isSubmitting" :edit-title="$t('users.editUserTitle')" :create-title="$t('users.addUserTitle')" :edit-description="$t('users.editUserDesc')" :create-description="$t('users.addUserDesc')" :edit-submit-label="$t('users.updateUser')" :create-submit-label="$t('users.createUser')" @submit="createUser">
      <div class="vstack gap-4">
        <label data-field>{{ $t('users.fullName') }} *<input id="full_name" v-model="formData.full_name" :placeholder="$t('users.fullNamePlaceholder')" /></label>
        <label data-field>{{ $t('common.email') }} *<input id="email" v-model="formData.email" type="email" :placeholder="$t('users.emailPlaceholder')" /></label>
        <label data-field>{{ $t('users.password') }} *<input id="password" v-model="formData.password" type="password" :placeholder="$t('users.passwordPlaceholder')" /></label>
        <label data-field>
          {{ $t('users.role') }} *
          <select id="role" v-model="formData.role_id">
            <option value="" disabled>{{ $t('users.selectRole') }}</option>
            <option v-for="role in rolesStore.roles" :key="role.id" :value="role.id">{{ role.name }}{{ role.is_system ? ` (${$t('users.system')})` : '' }}</option>
          </select>
        </label>
        <label v-if="isSuperAdmin" data-field>
          <span class="hstack justify-between">
            <span>{{ $t('users.superAdminLabel') }}</span>
            <input id="is_super_admin" v-model="formData.is_super_admin" type="checkbox" role="switch" />
          </span>
          <small data-hint>{{ $t('users.superAdminDesc') }}</small>
        </label>
      </div>
    </CrudFormDialog>

    <DeleteConfirmDialog v-model:open="deleteDialogOpen" :title="userToDelete?.is_member ? $t('users.removeMember') : $t('users.deleteUser')" :description="userToDelete?.is_member ? $t('users.removeMemberWarning') : undefined" :item-name="userToDelete?.full_name" :is-submitting="isDeleting" @confirm="confirmDelete" />

    <!-- Add Existing User Dialog -->
    <Modal v-model:open="isAddExistingOpen" :title="$t('users.addExistingUserTitle')" :description="$t('users.addExistingUserDesc')">
      <div class="vstack gap-4">
        <label data-field>{{ $t('common.email') }} *<input id="existing-email" v-model="addExistingEmail" type="email" :placeholder="$t('users.existingEmailPlaceholder')" /></label>
        <label data-field>
          {{ $t('users.role') }}
          <select v-model="addExistingRoleId">
            <option value="">{{ $t('users.selectRole') }}</option>
            <option v-for="role in rolesStore.roles" :key="role.id" :value="role.id">{{ role.name }}</option>
          </select>
        </label>
      </div>
      <template #footer>
        <button type="button" class="outline" @click="isAddExistingOpen = false">{{ $t('common.cancel') }}</button>
        <button type="button" :disabled="isAddExistingSubmitting || !addExistingEmail.trim()" :aria-busy="isAddExistingSubmitting ? 'true' : undefined" data-spinner="small" @click="submitAddExisting">
          {{ $t('users.addExistingUser') }}
        </button>
      </template>
    </Modal>
  </div>
</template>
