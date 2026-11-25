<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { PageHeader, SearchInput, DataTable, DeleteConfirmDialog, RowMenu, ErrorState, type Column } from '@/components/shared'
import { roleLabel } from '@/lib/status'
import { useRolesStore } from '@/stores/roles'
import { useOrganizationsStore } from '@/stores/organizations'
import { useAuthStore } from '@/stores/auth'
import type { Role } from '@/services/api'
import { toast } from '@/lib/toast'
import { Plus, Trash2, Shield } from '@/lib/icons'
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
  { key: 'permissions', label: t('roles.permissions') },
  { key: 'users', label: t('roles.users'), sortable: true, sortKey: 'user_count' },
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

/** Non-super-admins can only view a system role's permissions. */
const isViewOnly = (role: Role) => role.is_system && !isSuperAdmin.value

function deleteBlockedReason(role: Role): string | undefined {
  if (role.is_system) return t('roles.cannotDeleteSystem')
  if (role.user_count > 0) return t('roles.cannotDeleteUsers')
  return undefined
}
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader :title="$t('roles.title')" :description="$t('roles.subtitle')" :icon="Shield">
      <template #actions>
        <RouterLink v-if="canWrite" to="/settings/roles/new" class="button small"><Plus :size="16" />{{ $t('roles.addRole') }}</RouterLink>
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
      <article class="card vstack gap-4">
        <div data-toolbar>
          <div></div>
          <SearchInput v-model="searchQuery" :placeholder="$t('roles.searchRoles')" />
        </div>
        <DataTable :items="roles" :columns="columns" :is-loading="isLoading" :empty-icon="Shield" :empty-title="searchQuery ? $t('roles.noMatchingRoles') : $t('roles.noRolesYet')" :empty-description="searchQuery ? $t('roles.noMatchingRolesDesc') : $t('roles.noRolesYetDesc')" v-model:sort-key="sortKey" v-model:sort-direction="sortDirection" server-pagination :current-page="currentPage" :total-items="totalItems" :page-size="pageSize" item-name="roles" @page-change="handlePageChange">
          <template #cell-role="{ item: role }">
            <div class="hstack gap-2">
              <RouterLink :to="`/settings/roles/${role.id}`"><strong>{{ roleLabel(role.name) }}</strong></RouterLink>
              <span v-if="role.is_system" class="badge" data-label>{{ $t('roles.system') }}</span>
              <span v-if="role.is_default" class="badge" data-label>{{ $t('roles.default') }}</span>
            </div>
            <small v-if="role.description" class="text-light block truncate">{{ role.description }}</small>
          </template>
          <template #cell-permissions="{ item: role }">
            {{ role.permissions.length }}
          </template>
          <template #cell-users="{ item: role }">
            {{ role.user_count }}
          </template>
          <template #cell-created="{ item: role }">
            <span class="text-light">{{ formatDate(role.created_at) }}</span>
          </template>
          <template #cell-actions="{ item: role }">
            <div class="hstack items-center justify-end gap-1 flex-nowrap">
              <RouterLink :to="`/settings/roles/${role.id}`" class="button outline small">
                {{ isViewOnly(role) ? $t('roles.view') : $t('common.edit') }}
              </RouterLink>
              <RowMenu v-if="canDelete">
                <button
                  type="button"
                  role="menuitem"
                  class="ghost"
                  data-variant="danger"
                  :disabled="!!deleteBlockedReason(role)"
                  :title="deleteBlockedReason(role)"
                  @click="openDeleteDialog(role)"
                >
                  <Trash2 :size="14" />
                  {{ $t('common.delete') }}
                </button>
              </RowMenu>
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
