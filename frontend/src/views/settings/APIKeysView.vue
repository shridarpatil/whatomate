<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { apiKeysService } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { PageHeader, DataTable, SearchInput, DeleteConfirmDialog, IconButton, ErrorState, type Column } from '@/components/shared'
import { toast } from 'vue-sonner'
import { Plus, Trash2, Pencil, Key } from 'lucide-vue-next'
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
  { key: 'key', label: t('apiKeys.key') },
  { key: 'last_used', label: t('apiKeys.lastUsed'), sortable: true, sortKey: 'last_used_at' },
  { key: 'expires', label: t('apiKeys.expires'), sortable: true, sortKey: 'expires_at' },
  { key: 'status', label: t('apiKeys.status'), sortable: true, sortKey: 'is_active' },
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

onMounted(() => fetchItems())
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('apiKeys.title')" :subtitle="$t('apiKeys.subtitle')" :icon="Key" icon-gradient="bg-gradient-to-br from-amber-500 to-orange-600 shadow-amber-500/20" back-link="/settings">
      <template #actions>
        <RouterLink v-if="canWrite" to="/settings/api-keys/new">
          <Button variant="outline" size="sm"><Plus class="h-4 w-4 mr-2" />{{ $t('apiKeys.createApiKey') }}</Button>
        </RouterLink>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div>
          <ErrorState
            v-if="error && !isLoading"
            :title="$t('common.loadErrorTitle')"
            :description="error"
            :retry-label="$t('common.retry')"
            @retry="fetchItems"
          />
          <Card v-else>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>{{ $t('apiKeys.yourApiKeys') }}</CardTitle>
                  <CardDescription>{{ $t('apiKeys.yourApiKeysDesc') }}</CardDescription>
                </div>
                <SearchInput v-model="searchQuery" :placeholder="$t('apiKeys.searchApiKeys') + '...'" class="w-64" />
              </div>
            </CardHeader>
            <CardContent>
              <DataTable :items="apiKeys" :columns="columns" :is-loading="isLoading" :empty-icon="Key" :empty-title="searchQuery ? $t('apiKeys.noMatchingApiKeys') : $t('apiKeys.noApiKeysYet')" :empty-description="searchQuery ? $t('apiKeys.noMatchingApiKeysDesc') : $t('apiKeys.noApiKeysYetDesc')" v-model:sort-key="sortKey" v-model:sort-direction="sortDirection" server-pagination :current-page="currentPage" :total-items="totalItems" :page-size="pageSize" item-name="API keys" @page-change="handlePageChange">
                <template #cell-name="{ item: key }">
                  <RouterLink :to="`/settings/api-keys/${key.id}`" class="font-medium text-inherit no-underline hover:opacity-80">{{ key.name }}</RouterLink>
                </template>
                <template #cell-key="{ item: key }"><code class="bg-muted px-2 py-1 rounded text-sm">whm_{{ key.key_prefix }}...</code></template>
                <template #cell-last_used="{ item: key }">{{ formatDateTime(key.last_used_at) }}</template>
                <template #cell-expires="{ item: key }">{{ formatDateTime(key.expires_at) }}</template>
                <template #cell-status="{ item: key }">
                  <div class="flex items-center gap-2">
                    <Switch :checked="key.is_active && !isExpired(key.expires_at)" :disabled="isExpired(key.expires_at)" @update:checked="toggleActive(key)" />
                    <span class="text-sm text-muted-foreground">
                      {{ isExpired(key.expires_at) ? $t('apiKeys.expired') : key.is_active ? $t('common.active') : $t('common.inactive') }}
                    </span>
                  </div>
                </template>
                <template #cell-actions="{ item: key }">
                  <div class="flex items-center justify-end gap-1">
                    <RouterLink :to="`/settings/api-keys/${key.id}`">
                      <IconButton :icon="Pencil" :label="$t('common.edit')" class="h-8 w-8" />
                    </RouterLink>
                    <IconButton v-if="canDelete" :icon="Trash2" :label="$t('apiKeys.deleteApiKeyLabel')" variant="ghost" class="h-8 w-8 text-destructive" @click="keyToDelete = key; isDeleteDialogOpen = true" />
                  </div>
                </template>
                <template #empty-action>
                  <RouterLink v-if="canWrite" to="/settings/api-keys/new">
                    <Button variant="outline" size="sm"><Plus class="h-4 w-4 mr-2" />{{ $t('apiKeys.createApiKey') }}</Button>
                  </RouterLink>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <DeleteConfirmDialog v-model:open="isDeleteDialogOpen" :title="$t('apiKeys.deleteApiKey')" :item-name="keyToDelete?.name" :description="$t('apiKeys.deleteWarning')" :is-submitting="isDeleting" @confirm="deleteAPIKey" />
  </div>
</template>
