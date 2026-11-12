<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { PageHeader, SearchInput, DataTable, DeleteConfirmDialog, IconButton, ErrorState, type Column } from '@/components/shared'
import { useRolesStore } from '@/stores/roles'
import { useOrganizationsStore } from '@/stores/organizations'
import { useAuthStore } from '@/stores/auth'
import type { Role } from '@/services/api'
import { toast } from '@/lib/toast'
import { Plus, Pencil, Trash2, Shield, Users, Lock, Star } from '@/lib/icons'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDate } from '@/lib/utils'
import { useSearchPagination } from '@/composables/useSearchPagination'

const { t } = useI18n()

const rolesStore = useRolesStore()
const organizationsStore = useOrganizationsStore()
const authStore = useAuthStore()

const roles = ref<Role[]>([])
const isLoading = ref(true)
const error = ref(false)

const deleteDialogOpen = ref(false)
const roleToDelete = ref<Role | null>(null)
const isDeleting = ref(false)

const { searchQuery, currentPage, totalItems, pageSize, handlePageChange } = useSearchPagination({
  fetchFn: () => fetchRoles(),
})

const isSuperAdmin = computed(() => authStore.user?.is_super_admin ?? false)
const canWrite = computed(() => authStore.hasPermission('roles', 'write'))
const canDelete = computed(() => authStore.hasPermission('roles', 'delete'))

const columns = computed<Column<Role>[]>(() => [
  { key: 'role', label: t('roles.role'), sortable: true, sortKey: 'name' },
  { key: 'description', label: t('roles.description'), sortable: true },
  { key: 'permissions', label: t('roles.permissions'), align: 'center' },
  { key: 'users', label: t('roles.users'), align: 'center', sortable: true, sortKey: 'user_count' },
  { key: 'created', label: t('roles.created'), sortable: true, sortKey: 'created_at' },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

function openDeleteDialog(role: Role) {
  roleToDelete.value = role
  deleteDialogOpen.value = true
}

watch(() => organizationsStore.selectedOrgId, () => fetchRoles())
onMounted(() => fetchRoles())

async function fetchRoles() {
  isLoading.value = true
  error.value = false
  try {
    const response = await rolesStore.fetchRoles({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize,
    })
    roles.value = response.roles
    totalItems.value = response.total
  } catch {
    toast.error(t('common.failedLoad', { resource: t('resources.roles') }))
    error.value = true
  } finally {
    isLoading.value = false
  }
}

async function confirmDelete() {
  if (!roleToDelete.value) return
  isDeleting.value = true
  try {
    await rolesStore.deleteRole(roleToDelete.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.Role') }))
    deleteDialogOpen.value = false
    roleToDelete.value = null
    await fetchRoles()
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.role') })))
  } finally {
    isDeleting.value = false
  }
}

function editTooltip(role: Role): string {
  if (role.is_system) {
    return isSuperAdmin.value ? t('roles.editPermissions') : t('roles.viewPermissions')
  }
  return t('roles.editRole')
}
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader :title="$t('roles.title')" :description="$t('roles.subtitle')" :icon="Shield" back-link="/settings">
      <template #actions>
        <RouterLink v-if="canWrite" to="/settings/roles/new" class="button outline small"><Plus :size="16" />{{ $t('roles.addRole') }}</RouterLink>
      </template>
    </PageHeader>

    <ErrorState
      v-if="error && !isLoading"
      :title="$t('common.loadErrorTitle')"
      :description="$t('common.loadErrorDescription')"
      :retry-label="$t('common.retryLoad')"
      class="flex-1"
      @retry="fetchRoles"
    />

    <div v-else class="flex-1 scroll-y p-4">
      <article class="card">
        <header class="hstack justify-between flex-wrap gap-4">
          <h3>{{ $t('roles.yourRoles') }}</h3>
          <SearchInput v-model="searchQuery" :placeholder="$t('roles.searchRoles') + '...'" />
        </header>
        <DataTable :items="roles" :columns="columns" :is-loading="isLoading" :empty-icon="Shield" :empty-title="searchQuery ? $t('roles.noMatchingRoles') : $t('roles.noRolesYet')" :empty-description="searchQuery ? $t('roles.noMatchingRolesDesc') : $t('roles.noRolesYetDesc')" v-model:sort-key="sortKey" v-model:sort-direction="sortDirection" server-pagination :current-page="currentPage" :total-items="totalItems" :page-size="pageSize" item-name="roles" @page-change="handlePageChange">
          <template #cell-role="{ item: role }">
            <RouterLink :to="`/settings/roles/${role.id}`" class="hstack gap-2">
              <strong>{{ role.name }}</strong>
              <span v-if="role.is_system" class="badge" data-variant="secondary"><Lock :size="12" />{{ $t('roles.system') }}</span>
              <span v-if="role.is_default" class="badge outline"><Star :size="12" />{{ $t('roles.default') }}</span>
            </RouterLink>
          </template>
          <template #cell-description="{ item: role }">
            <span class="text-light truncate">{{ role.description || '-' }}</span>
          </template>
          <template #cell-permissions="{ item: role }">
            <span class="badge outline">{{ role.permissions.length }}</span>
          </template>
          <template #cell-users="{ item: role }">
            <div class="hstack gap-1 justify-center"><Users :size="16" /><span>{{ role.user_count }}</span></div>
          </template>
          <template #cell-created="{ item: role }">
            <span class="text-light">{{ formatDate(role.created_at) }}</span>
          </template>
          <template #cell-actions="{ item: role }">
            <div class="hstack gap-1 justify-end">
              <RouterLink :to="`/settings/roles/${role.id}`" class="button ghost icon" :title="editTooltip(role)" :aria-label="editTooltip(role)">
                <Pencil :size="16" />
              </RouterLink>
              <IconButton
                v-if="canDelete && !role.is_system"
                :icon="Trash2"
                :label="role.user_count > 0 ? $t('roles.cannotDeleteUsers') : $t('roles.deleteRole')"
                :disabled="role.user_count > 0"
                @click="openDeleteDialog(role)"
              />
            </div>
          </template>
          <template #empty-action>
            <RouterLink v-if="canWrite" to="/settings/roles/new" class="button outline small"><Plus :size="16" />{{ $t('roles.addRole') }}</RouterLink>
          </template>
        </DataTable>
      </article>
    </div>

    <DeleteConfirmDialog v-model:open="deleteDialogOpen" :title="$t('roles.deleteRole')" :item-name="roleToDelete?.name" :is-submitting="isDeleting" @confirm="confirmDelete" />
  </div>
</template>
