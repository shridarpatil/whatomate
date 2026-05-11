<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { PageHeader, SearchInput, DeleteConfirmDialog, DataTable, IconButton, ErrorState, type Column } from '@/components/shared'
import { cannedResponsesService, type CannedResponse } from '@/services/api'
import { useCrudState } from '@/composables/useCrudState'
import { toast } from 'vue-sonner'
import { Plus, MessageSquareText, Pencil, Trash2, Copy } from 'lucide-vue-next'
import { getErrorMessage } from '@/lib/api-utils'
import { CANNED_RESPONSE_CATEGORIES, getLabelFromValue } from '@/lib/constants'
import { useSearchPagination } from '@/composables/useSearchPagination'

const { t } = useI18n()
const router = useRouter()

const cannedResponses = ref<CannedResponse[]>([])
const isLoading = ref(false)
const error = ref<string | null>(null)
const {
  deleteDialogOpen, itemToDelete: responseToDelete,
  openDeleteDialog, closeDeleteDialog,
} = useCrudState<CannedResponse, Record<string, never>>({})
const selectedCategory = ref('all')

const breadcrumbs = computed(() => [{ label: t('nav.settings'), href: '/settings' }, { label: t('cannedResponses.title') }])

const columns = computed<Column<CannedResponse>[]>(() => [
  { key: 'name', label: t('cannedResponses.name'), sortable: true },
  { key: 'category', label: t('cannedResponses.category'), sortable: true },
  { key: 'content', label: t('cannedResponses.content') },
  { key: 'usage_count', label: t('cannedResponses.used'), sortable: true },
  { key: 'status', label: t('cannedResponses.status'), sortable: true, sortKey: 'is_active' },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

async function fetchItems() {
  isLoading.value = true
  error.value = null
  try {
    const response = await cannedResponsesService.list({
      search: searchQuery.value || undefined,
      category: selectedCategory.value !== 'all' ? selectedCategory.value : undefined,
      page: currentPage.value,
      limit: pageSize
    })
    const data = (response.data as any).data || response.data
    cannedResponses.value = data.canned_responses || []
    totalItems.value = data.total ?? cannedResponses.value.length
  } catch (err) {
    toast.error(getErrorMessage(err, t('common.failedLoad', { resource: t('resources.cannedResponses') })))
    error.value = t('cannedResponses.errorLoadingResponses')
  } finally {
    isLoading.value = false
  }
}

const { searchQuery, currentPage, totalItems, pageSize, handlePageChange, resetAndFetch } = useSearchPagination({
  fetchFn: () => fetchItems(),
})

watch(selectedCategory, () => {
  resetAndFetch()
})

function openCreate() { router.push('/settings/canned-responses/new') }
function openEdit(response: CannedResponse) { router.push(`/settings/canned-responses/${response.id}`) }

onMounted(() => fetchItems())

const isDeleting = ref(false)

async function confirmDelete() {
  if (!responseToDelete.value) return
  isDeleting.value = true
  try {
    await cannedResponsesService.delete(responseToDelete.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.CannedResponse') }))
    closeDeleteDialog()
    await fetchItems()
  } catch (err) {
    toast.error(getErrorMessage(err, t('common.failedDelete', { resource: t('resources.cannedResponse') })))
  } finally {
    isDeleting.value = false
  }
}

function copyToClipboard(content: string) { navigator.clipboard.writeText(content); toast.success(t('common.copiedToClipboard')) }
function getCategoryLabel(category: string): string { return getLabelFromValue(CANNED_RESPONSE_CATEGORIES, category) || t('cannedResponses.uncategorized') }
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('cannedResponses.title')" :icon="MessageSquareText" icon-gradient="bg-gradient-to-br from-teal-500 to-emerald-600 shadow-teal-500/20" back-link="/settings" :breadcrumbs="breadcrumbs">
      <template #actions>
        <Button variant="outline" size="sm" @click="openCreate"><Plus class="h-4 w-4 mr-2" />{{ $t('cannedResponses.addResponse') }}</Button>
      </template>
    </PageHeader>

    <ErrorState
      v-if="error && !isLoading"
      :title="$t('common.loadErrorTitle')"
      :description="error"
      :retry-label="$t('common.retry')"
      class="flex-1"
      @retry="fetchItems"
    />

    <ScrollArea v-else class="flex-1">
      <div class="p-6">
        <div>
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>{{ $t('cannedResponses.yourResponses') }}</CardTitle>
                  <CardDescription>{{ $t('cannedResponses.yourResponsesDesc') }}</CardDescription>
                </div>
                <div class="flex items-center gap-2">
                  <Select v-model="selectedCategory">
                    <SelectTrigger class="w-[150px]"><SelectValue :placeholder="$t('common.all')" /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{{ $t('cannedResponses.allCategories') }}</SelectItem>
                      <SelectItem v-for="cat in CANNED_RESPONSE_CATEGORIES" :key="cat.value" :value="cat.value">{{ cat.label }}</SelectItem>
                    </SelectContent>
                  </Select>
                  <SearchInput v-model="searchQuery" :placeholder="$t('cannedResponses.searchResponses') + '...'" class="w-64" />
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <DataTable
                :items="cannedResponses"
                :columns="columns"
                :is-loading="isLoading"
                :empty-icon="MessageSquareText"
                :empty-title="$t('cannedResponses.noResponsesFound')"
                :empty-description="$t('cannedResponses.noResponsesFoundDesc')"
                v-model:sort-key="sortKey"
                v-model:sort-direction="sortDirection"
                server-pagination
                :current-page="currentPage"
                :total-items="totalItems"
                :page-size="pageSize"
                item-name="responses"
                @page-change="handlePageChange"
              >
                <template #cell-name="{ item: response }">
                  <RouterLink :to="`/settings/canned-responses/${response.id}`" class="text-inherit no-underline hover:opacity-80">
                    <div>
                      <span class="font-medium">{{ response.name }}</span>
                      <p v-if="response.shortcut" class="text-xs font-mono text-muted-foreground">/{{ response.shortcut }}</p>
                    </div>
                  </RouterLink>
                </template>
                <template #cell-category="{ item: response }">
                  <Badge variant="outline" class="text-xs">{{ getCategoryLabel(response.category) }}</Badge>
                </template>
                <template #cell-content="{ item: response }">
                  <p class="text-sm text-muted-foreground max-w-[300px] truncate">{{ response.content }}</p>
                </template>
                <template #cell-usage_count="{ item: response }">
                  <span class="text-muted-foreground">{{ response.usage_count }}</span>
                </template>
                <template #cell-status="{ item: response }">
                  <Badge v-if="response.is_active" class="bg-emerald-500/20 text-emerald-400 border-transparent text-xs">{{ $t('common.active') }}</Badge>
                  <Badge v-else variant="secondary" class="text-xs">{{ $t('common.inactive') }}</Badge>
                </template>
                <template #cell-actions="{ item: response }">
                  <div class="flex items-center justify-end gap-1">
                    <IconButton
                      :icon="Copy"
                      :label="$t('cannedResponses.copyContent')"
                      class="h-8 w-8"
                      @click="copyToClipboard(response.content)"
                    />
                    <IconButton
                      :icon="Pencil"
                      :label="$t('cannedResponses.editResponse')"
                      class="h-8 w-8"
                      @click="openEdit(response)"
                    />
                    <IconButton
                      :icon="Trash2"
                      :label="$t('cannedResponses.deleteResponse')"
                      variant="ghost"
                      class="h-8 w-8 text-destructive"
                      @click="openDeleteDialog(response)"
                    />
                  </div>
                </template>
                <template #empty-action>
                  <Button variant="outline" size="sm" @click="openCreate">
                    <Plus class="h-4 w-4 mr-2" />{{ $t('cannedResponses.addResponse') }}
                  </Button>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <DeleteConfirmDialog v-model:open="deleteDialogOpen" :title="$t('cannedResponses.deleteTitle')" :item-name="responseToDelete?.name" :is-submitting="isDeleting" @confirm="confirmDelete" />
  </div>
</template>
