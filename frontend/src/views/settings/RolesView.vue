<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { PageHeader, SearchInput, DataTable, DeleteConfirmDialog, IconButton, ErrorState, type Column } from '@/components/shared'
import { useRolesStore } from '@/stores/roles'
import { useOrganizationsStore } from '@/stores/organizations'
import { useAuthStore } from '@/stores/auth'
import type { Role } from '@/services/api'
import { toast } from 'vue-sonner'
import { Plus, Pencil, Trash2, Shield, Users, Lock, Star } from 'lucide-vue-next'
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
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('roles.title')" :subtitle="$t('roles.subtitle')" :icon="Shield" icon-gradient="bg-gradient-to-br from-purple-500 to-indigo-600 shadow-purple-500/20" back-link="/settings">
      <template #actions>
        <RouterLink v-if="canWrite" to="/settings/roles/new">
          <Button variant="outline" size="sm"><Plus class="h-4 w-4 mr-2" />{{ $t('roles.addRole') }}</Button>
        </RouterLink>
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

    <ScrollArea v-else class="flex-1">
      <div class="p-6">
        <div>
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>{{ $t('roles.yourRoles') }}</CardTitle>
                  <CardDescription>{{ $t('roles.yourRolesDesc') }}</CardDescription>
                </div>
                <SearchInput v-model="searchQuery" :placeholder="$t('roles.searchRoles') + '...'" class="w-64" />
              </div>
            </CardHeader>
            <CardContent>
              <DataTable :items="roles" :columns="columns" :is-loading="isLoading" :empty-icon="Shield" :empty-title="searchQuery ? $t('roles.noMatchingRoles') : $t('roles.noRolesYet')" :empty-description="searchQuery ? $t('roles.noMatchingRolesDesc') : $t('roles.noRolesYetDesc')" v-model:sort-key="sortKey" v-model:sort-direction="sortDirection" server-pagination :current-page="currentPage" :total-items="totalItems" :page-size="pageSize" item-name="roles" @page-change="handlePageChange">
                <template #cell-role="{ item: role }">
                  <RouterLink :to="`/settings/roles/${role.id}`" class="flex items-center gap-2 text-inherit no-underline hover:opacity-80">
                    <span class="font-medium">{{ role.name }}</span>
                    <Badge v-if="role.is_system" variant="secondary"><Lock class="h-3 w-3 mr-1" />{{ $t('roles.system') }}</Badge>
                    <Badge v-if="role.is_default" variant="outline"><Star class="h-3 w-3 mr-1" />{{ $t('roles.default') }}</Badge>
                  </RouterLink>
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
                    <RouterLink :to="`/settings/roles/${role.id}`">
                      <IconButton :icon="Pencil" :label="editTooltip(role)" class="h-8 w-8" />
                    </RouterLink>
                    <IconButton
                      v-if="canDelete && !role.is_system"
                      :label="role.user_count > 0 ? $t('roles.cannotDeleteUsers') : $t('roles.deleteRole')"
                      class="h-8 w-8"
                      :disabled="role.user_count > 0"
                      @click="openDeleteDialog(role)"
                    >
                      <Trash2 class="h-4 w-4 text-destructive" />
                    </IconButton>
                  </div>
                </template>
                <template #empty-action>
                  <RouterLink v-if="canWrite" to="/settings/roles/new">
                    <Button variant="outline" size="sm"><Plus class="h-4 w-4 mr-2" />{{ $t('roles.addRole') }}</Button>
                  </RouterLink>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <DeleteConfirmDialog v-model:open="deleteDialogOpen" :title="$t('roles.deleteRole')" :item-name="roleToDelete?.name" :is-submitting="isDeleting" @confirm="confirmDelete" />
  </div>
</template>
