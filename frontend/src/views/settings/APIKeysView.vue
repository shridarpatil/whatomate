<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { apiKeysService } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { PageHeader, DataTable, SearchInput, DeleteConfirmDialog, RowMenu, ErrorState, type Column } from '@/components/shared'
import { toast } from '@/lib/toast'
import { Plus, Trash2, Key, Power } from '@/lib/icons'
import type { StatusTone } from '@/lib/status'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDate } from '@/lib/utils'
import { useSearchPagination } from '@/composables/useSearchPagination'

const { t } = useI18n()
const authStore = useAuthStore()

interface APIKey {
  id: string
  name: string
  key_prefix: string
  last_used_at: string | null
  expires_at: string | null
  is_active: boolean
  created_at: string
}

const apiKeys = ref<APIKey[]>([])
const isLoading = ref(false)
const isDeleting = ref(false)
const error = ref<string | null>(null)

const canWrite = computed(() => authStore.hasPermission('api_keys', 'write'))
const canDelete = computed(() => authStore.hasPermission('api_keys', 'delete'))

const isDeleteDialogOpen = ref(false)
const keyToDelete = ref<APIKey | null>(null)

const { searchQuery, currentPage, totalItems, pageSize, handlePageChange } = useSearchPagination({
  fetchFn: () => fetchItems(),
})

const columns = computed<Column<APIKey>[]>(() => [
  { key: 'name', label: t('apiKeys.name'), sortable: true },
  { key: 'status', label: t('apiKeys.status'), sortable: true, sortKey: 'is_active' },
  { key: 'last_used', label: t('apiKeys.lastUsed'), sortable: true, sortKey: 'last_used_at' },
  { key: 'expires', label: t('apiKeys.expires'), sortable: true, sortKey: 'expires_at' },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

async function fetchItems() {
  isLoading.value = true
  error.value = null
  try {
    const response = await apiKeysService.list({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    const data = (response.data as any).data || response.data
    apiKeys.value = data.api_keys || []
    totalItems.value = data.total ?? apiKeys.value.length
  } catch (err) {
    toast.error(getErrorMessage(err, t('common.failedLoad', { resource: t('resources.apiKeys') })))
    error.value = t('apiKeys.errorLoadingApiKeys')
  } finally {
    isLoading.value = false
  }
}

async function deleteAPIKey() {
  if (!keyToDelete.value) return
  isDeleting.value = true
  try {
    await apiKeysService.delete(keyToDelete.value.id)
    await fetchItems()
    toast.success(t('common.deletedSuccess', { resource: t('resources.APIKey') }))
    isDeleteDialogOpen.value = false
    keyToDelete.value = null
  } catch (err) {
    toast.error(getErrorMessage(err, t('common.failedDelete', { resource: t('resources.APIKey') })))
  } finally {
    isDeleting.value = false
  }
}

async function toggleActive(key: APIKey) {
  try {
    await apiKeysService.update(key.id, { is_active: !key.is_active })
    await fetchItems()
    toast.success(key.is_active ? t('common.disabledSuccess', { resource: t('resources.APIKey', 'API Key') }) : t('common.enabledSuccess', { resource: t('resources.APIKey', 'API Key') }))
  } catch (e) { toast.error(getErrorMessage(e, t('common.failedToggle', { resource: t('resources.apiKey', 'API key') }))) }
}

function formatDateTime(dateStr: string | null) { return dateStr ? formatDate(dateStr, { year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }) : t('apiKeys.never') }
function isExpired(expiresAt: string | null) { return expiresAt ? new Date(expiresAt) < new Date() : false }
function keyStatus(key: APIKey): { tone: StatusTone, label: string } {
  if (isExpired(key.expires_at)) return { tone: 'danger', label: t('apiKeys.expired') }
  return key.is_active ? { tone: 'success', label: t('common.active') } : { tone: 'neutral', label: t('common.inactive') }
}

onMounted(() => fetchItems())
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader :title="$t('apiKeys.title')" :description="$t('apiKeys.subtitle')" :icon="Key">
      <template #actions>
        <RouterLink v-if="canWrite" to="/settings/api-keys/new" class="button small"><Plus :size="16" />{{ $t('apiKeys.createApiKey') }}</RouterLink>
      </template>
    </PageHeader>

    <div class="flex-1 scroll-y p-4">
      <ErrorState
        v-if="error && !isLoading"
        :title="$t('common.loadErrorTitle')"
        :description="error"
        :retry-label="$t('common.retry')"
        @retry="fetchItems"
      />
      <article v-else class="card vstack gap-4">
        <div data-toolbar>
          <div></div>
          <SearchInput v-model="searchQuery" :placeholder="$t('apiKeys.searchApiKeys')" />
        </div>
        <DataTable :items="apiKeys" :columns="columns" :is-loading="isLoading" :empty-icon="Key" :empty-title="searchQuery ? $t('apiKeys.noMatchingApiKeys') : $t('apiKeys.noApiKeysYet')" :empty-description="searchQuery ? $t('apiKeys.noMatchingApiKeysDesc') : $t('apiKeys.noApiKeysYetDesc')" v-model:sort-key="sortKey" v-model:sort-direction="sortDirection" server-pagination :current-page="currentPage" :total-items="totalItems" :page-size="pageSize" item-name="API keys" @page-change="handlePageChange">
          <template #cell-name="{ item: key }">
            <RouterLink :to="`/settings/api-keys/${key.id}`"><strong>{{ key.name }}</strong></RouterLink>
            <small class="text-light block"><code>whm_{{ key.key_prefix }}...</code></small>
          </template>
          <template #cell-status="{ item: key }">
            <span class="badge" :data-status="keyStatus(key).tone">{{ keyStatus(key).label }}</span>
          </template>
          <template #cell-last_used="{ item: key }"><span class="text-light">{{ formatDateTime(key.last_used_at) }}</span></template>
          <template #cell-expires="{ item: key }"><span class="text-light">{{ formatDateTime(key.expires_at) }}</span></template>
          <template #cell-actions="{ item: key }">
            <div class="hstack items-center justify-end gap-1 flex-nowrap">
              <RouterLink :to="`/settings/api-keys/${key.id}`" class="button outline small">{{ $t('common.edit') }}</RouterLink>
              <RowMenu v-if="canWrite || canDelete">
                <button
                  v-if="canWrite"
                  type="button"
                  role="menuitem"
                  class="ghost"
                  :disabled="isExpired(key.expires_at)"
                  :title="isExpired(key.expires_at) ? $t('apiKeys.expiredCantEnable') : undefined"
                  @click="toggleActive(key)"
                >
                  <Power :size="14" />
                  {{ key.is_active && !isExpired(key.expires_at) ? $t('common.disable') : $t('common.enable') }}
                </button>
                <button v-if="canDelete" type="button" role="menuitem" class="ghost" data-variant="danger" @click="keyToDelete = key; isDeleteDialogOpen = true">
                  <Trash2 :size="14" />
                  {{ $t('common.delete') }}
                </button>
              </RowMenu>
            </div>
          </template>
          <template #empty-action>
            <RouterLink v-if="canWrite" to="/settings/api-keys/new" class="button outline small"><Plus :size="16" />{{ $t('apiKeys.createApiKey') }}</RouterLink>
          </template>
        </DataTable>
      </article>
    </div>

    <DeleteConfirmDialog v-model:open="isDeleteDialogOpen" :title="$t('apiKeys.deleteApiKey')" :item-name="keyToDelete?.name" :description="$t('apiKeys.deleteWarning')" :is-submitting="isDeleting" @confirm="deleteAPIKey" />
  </div>
</template>
