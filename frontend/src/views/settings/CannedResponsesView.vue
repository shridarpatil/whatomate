<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { PageHeader, SearchInput, DeleteConfirmDialog, DataTable, IconButton, ErrorState, type Column } from '@/components/shared'
import { cannedResponsesService, type CannedResponse } from '@/services/api'
import { useCrudState } from '@/composables/useCrudState'
import { toast } from '@/lib/toast'
import { Plus, MessageSquareText, Pencil, Trash2, Copy } from '@/lib/icons'
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
  <div class="flex flex-col h-full">
    <PageHeader :title="$t('cannedResponses.title')" :icon="MessageSquareText" back-link="/settings" :breadcrumbs="breadcrumbs">
      <template #actions>
        <button type="button" class="outline small" @click="openCreate"><Plus :size="16" />{{ $t('cannedResponses.addResponse') }}</button>
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

    <div v-else class="flex-1 scroll-y p-4">
      <article class="card">
        <header class="hstack justify-between flex-wrap gap-4">
          <div>
            <h3>{{ $t('cannedResponses.yourResponses') }}</h3>
            <p class="text-light">{{ $t('cannedResponses.yourResponsesDesc') }}</p>
          </div>
          <div class="hstack gap-2">
            <select v-model="selectedCategory" :aria-label="$t('cannedResponses.category')">
              <option value="all">{{ $t('cannedResponses.allCategories') }}</option>
              <option v-for="cat in CANNED_RESPONSE_CATEGORIES" :key="cat.value" :value="cat.value">{{ cat.label }}</option>
            </select>
            <SearchInput v-model="searchQuery" :placeholder="$t('cannedResponses.searchResponses') + '...'" />
          </div>
        </header>
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
            <RouterLink :to="`/settings/canned-responses/${response.id}`">
              <strong>{{ response.name }}</strong>
              <small v-if="response.shortcut" class="text-light"><br>/{{ response.shortcut }}</small>
            </RouterLink>
          </template>
          <template #cell-category="{ item: response }">
            <span class="badge outline">{{ getCategoryLabel(response.category) }}</span>
          </template>
          <template #cell-content="{ item: response }">
            <p class="text-light truncate">{{ response.content }}</p>
          </template>
          <template #cell-usage_count="{ item: response }">
            <span class="text-light">{{ response.usage_count }}</span>
          </template>
          <template #cell-status="{ item: response }">
            <span class="badge" :data-variant="response.is_active ? 'success' : 'secondary'">{{ response.is_active ? $t('common.active') : $t('common.inactive') }}</span>
          </template>
          <template #cell-actions="{ item: response }">
            <div class="hstack gap-1 justify-end">
              <IconButton :icon="Copy" :label="$t('cannedResponses.copyContent')" @click="copyToClipboard(response.content)" />
              <IconButton :icon="Pencil" :label="$t('cannedResponses.editResponse')" @click="openEdit(response)" />
              <IconButton :icon="Trash2" :label="$t('cannedResponses.deleteResponse')" @click="openDeleteDialog(response)" />
            </div>
          </template>
          <template #empty-action>
            <button type="button" class="outline small" @click="openCreate">
              <Plus :size="16" />{{ $t('cannedResponses.addResponse') }}
            </button>
          </template>
        </DataTable>
      </article>
    </div>

    <DeleteConfirmDialog v-model:open="deleteDialogOpen" :title="$t('cannedResponses.deleteTitle')" :item-name="responseToDelete?.name" :is-submitting="isDeleting" @confirm="confirmDelete" />
  </div>
</template>
