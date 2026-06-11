<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { PageHeader, SearchInput, DataTable, CrudFormDialog, IconButton, ErrorState, type Column } from '@/components/shared'
import { useAdminStore, type AdminOrganization } from '@/stores/admin'
import { toast } from 'vue-sonner'
import { Plus, Pencil, Users, Building2 } from 'lucide-vue-next'
import { useCrudState } from '@/composables/useCrudState'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDate } from '@/lib/utils'
import { useSearchPagination } from '@/composables/useSearchPagination'

const { t } = useI18n()
const router = useRouter()
const adminStore = useAdminStore()

interface OrgFormData {
  name: string
  admin_full_name: string
  admin_email: string
  admin_password: string
}

const defaultFormData: OrgFormData = { name: '', admin_full_name: '', admin_email: '', admin_password: '' }

const {
  isLoading, isSubmitting, isDialogOpen, editingItem,
  formData, openCreateDialog, openEditDialog: baseOpenEditDialog, closeDialog,
} = useCrudState<AdminOrganization, OrgFormData>(defaultFormData)

const isEditing = computed(() => editingItem.value !== null)

const organizations = ref<AdminOrganization[]>([])
const error = ref(false)

const { searchQuery, currentPage, totalItems, pageSize, handlePageChange } = useSearchPagination({
  fetchFn: () => fetchOrganizations(),
})

const columns = computed<Column<AdminOrganization>[]>(() => [
  { key: 'name', label: t('admin.orgName'), sortable: true, sortKey: 'name' },
  { key: 'slug', label: t('admin.slug') },
  { key: 'users', label: t('admin.userCount'), sortable: true, sortKey: 'user_count' },
  { key: 'accounts', label: t('admin.accountCount'), sortable: true, sortKey: 'account_count' },
  { key: 'created', label: t('users.created'), sortable: true, sortKey: 'created_at' },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

onMounted(() => fetchOrganizations())

async function fetchOrganizations() {
  isLoading.value = true
  error.value = false
  try {
    const response = await adminStore.fetchOrganizations({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize,
    })
    organizations.value = response.organizations
    totalItems.value = response.total
  } catch {
    toast.error(t('common.failedLoad', { resource: t('resources.organizations') }))
    error.value = true
  } finally {
    isLoading.value = false
  }
}

function openEditDialog(org: AdminOrganization) {
  baseOpenEditDialog(org, o => ({ name: o.name, admin_full_name: '', admin_email: '', admin_password: '' }))
}

async function submitForm() {
  if (!formData.value.name.trim()) {
    toast.error(t('admin.orgNameRequired'))
    return
  }
  isSubmitting.value = true
  try {
    if (isEditing.value && editingItem.value) {
      await adminStore.updateOrganization(editingItem.value.id, { name: formData.value.name.trim() })
      toast.success(t('common.updatedSuccess', { resource: t('resources.Organization') }))
    } else {
      if (!formData.value.admin_full_name.trim() || !formData.value.admin_email.trim()) {
        toast.error(t('users.fillEmailName'))
        isSubmitting.value = false
        return
      }
      if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.value.admin_email.trim())) {
        toast.error(t('validation.email'))
        isSubmitting.value = false
        return
      }
      if (!formData.value.admin_password.trim()) {
        toast.error(t('users.passwordRequired'))
        isSubmitting.value = false
        return
      }
      await adminStore.createOrganization({
        name: formData.value.name.trim(),
        admin_full_name: formData.value.admin_full_name.trim(),
        admin_email: formData.value.admin_email.trim(),
        admin_password: formData.value.admin_password,
      })
      toast.success(t('common.createdSuccess', { resource: t('resources.Organization') }))
    }
    closeDialog()
    await fetchOrganizations()
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedSave', { resource: t('resources.organization') })))
  } finally {
    isSubmitting.value = false
  }
}

function viewOrgUsers(org: AdminOrganization) {
  router.push({ path: '/admin/users', query: { organization_id: org.id } })
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('admin.organizationsTitle')" :icon="Building2" icon-gradient="bg-gradient-to-br from-purple-500 to-indigo-600 shadow-purple-500/20">
      <template #actions>
        <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />{{ $t('admin.addOrganization') }}</Button>
      </template>
    </PageHeader>

    <ErrorState
      v-if="error && !isLoading"
      :title="$t('common.loadErrorTitle')"
      :description="$t('common.loadErrorDescription')"
      :retry-label="$t('common.retryLoad')"
      class="flex-1"
      @retry="fetchOrganizations"
    />

    <ScrollArea v-else class="flex-1">
      <div class="p-6">
        <Card>
          <CardHeader>
            <div class="flex items-center justify-between flex-wrap gap-4">
              <div>
                <CardTitle>{{ $t('admin.organizationsTitle') }}</CardTitle>
                <CardDescription>{{ $t('admin.organizationsSubtitle') }}</CardDescription>
              </div>
              <SearchInput v-model="searchQuery" :placeholder="$t('admin.searchOrganizations') + '...'" class="w-64" />
            </div>
          </CardHeader>
          <CardContent>
            <DataTable :items="organizations" :columns="columns" :is-loading="isLoading" :empty-icon="Building2" :empty-title="$t('admin.noOrganizations')" :empty-description="$t('admin.noOrganizationsDesc')" v-model:sort-key="sortKey" v-model:sort-direction="sortDirection" server-pagination :current-page="currentPage" :total-items="totalItems" :page-size="pageSize" item-name="organizations" @page-change="handlePageChange">
              <template #cell-name="{ item: org }">
                <div class="flex items-center gap-3">
                  <div class="h-9 w-9 rounded-full bg-primary/10 flex items-center justify-center flex-shrink-0">
                    <Building2 class="h-4 w-4 text-primary" />
                  </div>
                  <p class="font-medium truncate">{{ org.name }}</p>
                </div>
              </template>
              <template #cell-slug="{ item: org }">
                <span class="text-sm text-muted-foreground">{{ org.slug }}</span>
              </template>
              <template #cell-users="{ item: org }">
                <span>{{ org.user_count }}</span>
              </template>
              <template #cell-accounts="{ item: org }">
                <span>{{ org.account_count }}</span>
              </template>
              <template #cell-created="{ item: org }">
                <span class="text-muted-foreground">{{ formatDate(org.created_at) }}</span>
              </template>
              <template #cell-actions="{ item: org }">
                <div class="flex items-center justify-end gap-1">
                  <IconButton :icon="Users" :label="$t('admin.viewUsers')" class="h-8 w-8" @click="viewOrgUsers(org)" />
                  <IconButton :icon="Pencil" :label="$t('admin.renameOrganization')" class="h-8 w-8" @click="openEditDialog(org)" />
                </div>
              </template>
              <template #empty-action>
                <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />{{ $t('admin.addOrganization') }}</Button>
              </template>
            </DataTable>
          </CardContent>
        </Card>
      </div>
    </ScrollArea>

    <CrudFormDialog v-model:open="isDialogOpen" :is-editing="isEditing" :is-submitting="isSubmitting" :edit-title="$t('admin.renameOrganization')" :create-title="$t('admin.addOrganizationTitle')" :edit-description="$t('admin.renameOrganizationDesc')" :create-description="$t('admin.addOrganizationDesc')" :edit-submit-label="$t('common.save')" :create-submit-label="$t('common.create')" @submit="submitForm">
      <div class="space-y-4">
        <div class="space-y-2">
          <Label for="org_name">{{ $t('admin.orgName') }} <span class="text-destructive">*</span></Label>
          <Input id="org_name" v-model="formData.name" :placeholder="$t('admin.orgNamePlaceholder')" />
        </div>
        <template v-if="!isEditing">
          <div class="border-t pt-4">
            <p class="text-sm font-medium">{{ $t('admin.adminAccountSection') }}</p>
            <p class="text-xs text-muted-foreground">{{ $t('admin.adminAccountSectionDesc') }}</p>
          </div>
          <div class="space-y-2">
            <Label for="admin_full_name">{{ $t('users.fullName') }} <span class="text-destructive">*</span></Label>
            <Input id="admin_full_name" v-model="formData.admin_full_name" :placeholder="$t('users.fullNamePlaceholder')" />
          </div>
          <div class="space-y-2">
            <Label for="admin_email">{{ $t('common.email') }} <span class="text-destructive">*</span></Label>
            <Input id="admin_email" v-model="formData.admin_email" type="email" :placeholder="$t('users.emailPlaceholder')" />
          </div>
          <div class="space-y-2">
            <Label for="admin_password">{{ $t('users.password') }} <span class="text-destructive">*</span></Label>
            <Input id="admin_password" v-model="formData.admin_password" type="password" :placeholder="$t('users.passwordPlaceholder')" />
          </div>
        </template>
      </div>
    </CrudFormDialog>
  </div>
</template>
