<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { apiKeysService } from '@/services/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { PageHeader, DataTable, SearchInput, CrudFormDialog, DeleteConfirmDialog, type Column } from '@/components/shared'
import { toast } from 'vue-sonner'
import { Plus, Trash2, Copy, Key, AlertTriangle } from 'lucide-vue-next'
import { useCrudState } from '@/composables/useCrudState'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDate } from '@/lib/utils'
import { useDebounceFn } from '@vueuse/core'

interface APIKey {
  id: string
  name: string
  key_prefix: string
  last_used_at: string | null
  expires_at: string | null
  is_active: boolean
  created_at: string
}

interface NewAPIKeyResponse {
  id: string
  name: string
  key: string
  key_prefix: string
  expires_at: string | null
  created_at: string
}

interface APIKeyFormData {
  name: string
  expires_at: string
}

const defaultFormData: APIKeyFormData = { name: '', expires_at: '' }

const {
  items: apiKeys, isLoading, isSubmitting, isDialogOpen: isCreateDialogOpen, deleteDialogOpen: isDeleteDialogOpen, itemToDelete: keyToDelete,
  formData, openCreateDialog: openCreateDialogBase, openDeleteDialog, closeDialog: closeCreateDialog, closeDeleteDialog,
} = useCrudState<APIKey, APIKeyFormData>(defaultFormData)

const isKeyDisplayOpen = ref(false)
const newlyCreatedKey = ref<NewAPIKeyResponse | null>(null)

// Pagination state
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 20

const columns: Column<APIKey>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'key', label: 'Key' },
  { key: 'last_used', label: 'Last Used', sortable: true, sortKey: 'last_used_at' },
  { key: 'expires', label: 'Expires', sortable: true, sortKey: 'expires_at' },
  { key: 'status', label: 'Status', sortable: true, sortKey: 'is_active' },
  { key: 'actions', label: 'Actions', align: 'right' },
]

// Sorting state
const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

const searchQuery = ref('')

async function fetchItems() {
  isLoading.value = true
  try {
    const response = await apiKeysService.list({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    const data = (response.data as any).data || response.data
    apiKeys.value = data.api_keys || []
    totalItems.value = data.total ?? apiKeys.value.length
  } catch (error) {
    toast.error(getErrorMessage(error, 'Failed to load API keys'))
  } finally {
    isLoading.value = false
  }
}

// Debounced search
const debouncedSearch = useDebounceFn(() => {
  currentPage.value = 1
  fetchItems()
}, 300)

watch(searchQuery, () => debouncedSearch())

function handlePageChange(page: number) {
  currentPage.value = page
  fetchItems()
}

async function createAPIKey() {
  if (!formData.value.name.trim()) { toast.error('Name is required'); return }
  isSubmitting.value = true
  try {
    const payload: { name: string; expires_at?: string } = { name: formData.value.name.trim() }
    if (formData.value.expires_at) payload.expires_at = new Date(formData.value.expires_at).toISOString()
    const response = await apiKeysService.create(payload)
    newlyCreatedKey.value = response.data.data
    closeCreateDialog()
    isKeyDisplayOpen.value = true
    formData.value = { ...defaultFormData }
    await fetchItems()
    toast.success('API key created successfully')
  } catch (error) { toast.error(getErrorMessage(error, 'Failed to create API key')) }
  finally { isSubmitting.value = false }
}

async function deleteAPIKey() {
  if (!keyToDelete.value) return
  try { await apiKeysService.delete(keyToDelete.value.id); await fetchItems(); toast.success('API key deleted successfully'); closeDeleteDialog() }
  catch (error) { toast.error(getErrorMessage(error, 'Failed to delete API key')) }
}

function copyToClipboard(text: string) { navigator.clipboard.writeText(text); toast.success('Copied to clipboard') }
function formatDateTime(dateStr: string | null) { return dateStr ? formatDate(dateStr, { year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }) : 'Never' }
function isExpired(expiresAt: string | null) { return expiresAt ? new Date(expiresAt) < new Date() : false }

onMounted(() => fetchItems())
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader title="API Keys" subtitle="Manage API keys for programmatic access" :icon="Key" icon-gradient="bg-gradient-to-br from-amber-500 to-orange-600 shadow-amber-500/20">
      <template #actions>
        <Button variant="outline" size="sm" @click="openCreateDialogBase"><Plus class="h-4 w-4 mr-2" />Create API Key</Button>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto">
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>Your API Keys</CardTitle>
                  <CardDescription>API keys allow external applications to access your account. Keep them secure.</CardDescription>
                </div>
                <SearchInput v-model="searchQuery" placeholder="Search API keys..." class="w-64" />
              </div>
            </CardHeader>
            <CardContent>
              <DataTable :items="apiKeys" :columns="columns" :is-loading="isLoading" :empty-icon="Key" :empty-title="searchQuery ? 'No matching API keys' : 'No API keys yet'" :empty-description="searchQuery ? 'No API keys match your search.' : 'Create your first API key to get started.'" v-model:sort-key="sortKey" v-model:sort-direction="sortDirection" server-pagination :current-page="currentPage" :total-items="totalItems" :page-size="pageSize" item-name="API keys" @page-change="handlePageChange">
                <template #cell-name="{ item: key }"><span class="font-medium">{{ key.name }}</span></template>
                <template #cell-key="{ item: key }"><code class="bg-muted px-2 py-1 rounded text-sm">whm_{{ key.key_prefix }}...</code></template>
                <template #cell-last_used="{ item: key }">{{ formatDateTime(key.last_used_at) }}</template>
                <template #cell-expires="{ item: key }">{{ formatDateTime(key.expires_at) }}</template>
                <template #cell-status="{ item: key }">
                  <Badge variant="outline" :class="isExpired(key.expires_at) ? 'border-destructive text-destructive' : key.is_active ? 'border-green-600 text-green-600' : ''">
                    {{ isExpired(key.expires_at) ? 'Expired' : key.is_active ? 'Active' : 'Inactive' }}
                  </Badge>
                </template>
                <template #cell-actions="{ item: key }">
                  <Button variant="ghost" size="icon" @click="openDeleteDialog(key)"><Trash2 class="h-4 w-4 text-destructive" /></Button>
                </template>
                <template #empty-action>
                  <Button variant="outline" size="sm" @click="openCreateDialogBase"><Plus class="h-4 w-4 mr-2" />Create API Key</Button>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <CrudFormDialog v-model:open="isCreateDialogOpen" :is-editing="false" :is-submitting="isSubmitting" create-title="Create API Key" create-description="Create a new API key for programmatic access to your account." create-submit-label="Create Key" @submit="createAPIKey">
      <div class="space-y-4">
        <div class="space-y-2"><Label for="name">Name</Label><Input id="name" v-model="formData.name" placeholder="e.g., Production Integration" /></div>
        <div class="space-y-2">
          <Label for="expiry">Expiration (optional)</Label>
          <Input id="expiry" v-model="formData.expires_at" type="datetime-local" />
          <p class="text-xs text-muted-foreground">Leave empty for no expiration</p>
        </div>
      </div>
    </CrudFormDialog>

    <Dialog v-model:open="isKeyDisplayOpen">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>API Key Created</DialogTitle>
          <DialogDescription><div class="flex items-center gap-2 text-amber-600 mt-2"><AlertTriangle class="h-4 w-4" /><span>Make sure to copy your API key now. You won't be able to see it again!</span></div></DialogDescription>
        </DialogHeader>
        <div class="space-y-4 py-4">
          <div class="space-y-2">
            <Label>Your API Key</Label>
            <div class="flex gap-2"><Input :model-value="newlyCreatedKey?.key" readonly class="font-mono text-sm" /><Button variant="outline" size="icon" @click="copyToClipboard(newlyCreatedKey?.key || '')"><Copy class="h-4 w-4" /></Button></div>
          </div>
          <div class="bg-muted p-3 rounded-lg text-sm"><p class="font-medium mb-1">Usage:</p><code class="text-xs">curl -H "X-API-Key: {{ newlyCreatedKey?.key }}" https://your-api.com/api/contacts</code></div>
        </div>
        <DialogFooter><Button size="sm" @click="isKeyDisplayOpen = false">Done</Button></DialogFooter>
      </DialogContent>
    </Dialog>

    <DeleteConfirmDialog v-model:open="isDeleteDialogOpen" title="Delete API Key" :item-name="keyToDelete?.name" description="Any applications using this key will stop working." @confirm="deleteAPIKey" />
  </div>
</template>
