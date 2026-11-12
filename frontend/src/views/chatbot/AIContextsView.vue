<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { chatbotService } from '@/services/api'
import { toast } from '@/lib/toast'
import { PageHeader, DataTable, DeleteConfirmDialog, SearchInput, IconButton, ErrorState, type Column } from '@/components/shared'
import { getErrorMessage } from '@/lib/api-utils'
import { Plus, Pencil, Trash2, Sparkles } from '@/lib/icons'
import { debounce } from '@/lib/utils'

const { t } = useI18n()

interface AIContext {
  id: string
  name: string
  context_type: string
  trigger_keywords: string[]
  static_content: string
  api_config: {
    url: string
    method: string
    headers: Record<string, string>
    body: string
    response_path: string
  }
  priority: number
  enabled: boolean
  created_at: string
}

const contexts = ref<AIContext[]>([])
const isLoading = ref(true)
const isDeleting = ref(false)
const error = ref<string | null>(null)
const searchQuery = ref('')
const deleteDialogOpen = ref(false)
const contextToDelete = ref<AIContext | null>(null)

function openDeleteDialog(context: AIContext) {
  contextToDelete.value = context
  deleteDialogOpen.value = true
}

function closeDeleteDialog() {
  deleteDialogOpen.value = false
  contextToDelete.value = null
}

// Pagination state
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 20

const columns = computed<Column<AIContext>[]>(() => [
  { key: 'name', label: t('aiContexts.name'), sortable: true },
  { key: 'context_type', label: t('aiContexts.type'), sortable: true },
  { key: 'trigger_keywords', label: t('aiContexts.keywords') },
  { key: 'priority', label: t('aiContexts.priority'), sortable: true },
  { key: 'status', label: t('aiContexts.status'), sortable: true, sortKey: 'enabled' },
  { key: 'actions', label: t('aiContexts.actions'), align: 'right' },
])

const sortKey = ref('priority')
const sortDirection = ref<'asc' | 'desc'>('desc')

onMounted(async () => {
  await fetchContexts()
})

async function fetchContexts() {
  isLoading.value = true
  error.value = null
  try {
    const response = await chatbotService.listAIContexts({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    // API response is wrapped in { status: "success", data: { contexts: [...] } }
    const data = (response.data as any).data || response.data
    contexts.value = data.contexts || []
    totalItems.value = data.total ?? contexts.value.length
  } catch (err) {
    console.error('Failed to load AI contexts:', err)
    error.value = t('aiContexts.fetchError')
    contexts.value = []
  } finally {
    isLoading.value = false
  }
}

// Debounced search to avoid too many API calls
const debouncedSearch = debounce(() => {
  currentPage.value = 1
  fetchContexts()
}, 300)

// Watch search query changes
watch(searchQuery, () => {
  debouncedSearch()
})

function handlePageChange(page: number) {
  currentPage.value = page
  fetchContexts()
}


async function confirmDeleteContext() {
  if (!contextToDelete.value) return

  isDeleting.value = true
  try {
    await chatbotService.deleteAIContext(contextToDelete.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.AIContext') }))
    closeDeleteDialog()
    await fetchContexts()
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('common.failedDelete', { resource: t('resources.AIContext') })))
  } finally {
    isDeleting.value = false
  }
}

async function toggleContext(context: AIContext, event?: Event) {
  try {
    await chatbotService.updateAIContext(context.id, { enabled: !context.enabled })
    context.enabled = !context.enabled
    toast.success(context.enabled ? t('common.enabledSuccess', { resource: t('resources.AIContext') }) : t('common.disabledSuccess', { resource: t('resources.AIContext') }))
  } catch (error: any) {
    if (event?.target instanceof HTMLInputElement) event.target.checked = context.enabled
    toast.error(getErrorMessage(error, t('common.failedToggle', { resource: t('resources.AIContext') })))
  }
}
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader
      :title="$t('aiContexts.title')"
      :icon="Sparkles"
      back-link="/chatbot"
      :breadcrumbs="[{ label: $t('aiContexts.backToChatbot'), href: '/chatbot' }, { label: $t('nav.aiContexts') }]"
    >
      <template #actions>
        <RouterLink to="/chatbot/ai/new" class="button outline small">
          <Plus :size="16" />
          {{ $t('aiContexts.addContext') }}
        </RouterLink>
      </template>
    </PageHeader>

    <div class="flex-1 scroll-y p-4">
      <article class="card">
        <header class="hstack justify-between flex-wrap gap-4">
          <h3>{{ $t('aiContexts.yourContexts') }}</h3>
          <SearchInput v-model="searchQuery" :placeholder="$t('aiContexts.searchContexts') + '...'" />
        </header>
        <ErrorState
          v-if="error"
          :title="$t('common.loadErrorTitle')"
          :description="error"
          :retry-label="$t('common.retry')"
          @retry="fetchContexts"
        />
        <DataTable
          v-else
          :items="contexts"
          :columns="columns"
          :is-loading="isLoading"
          :empty-icon="Sparkles"
          :empty-title="searchQuery ? $t('aiContexts.noMatchingContexts') : $t('aiContexts.noContextsYet')"
          :empty-description="searchQuery ? $t('aiContexts.noMatchingContextsDesc') : $t('aiContexts.noContextsYetDesc')"
          v-model:sort-key="sortKey"
          v-model:sort-direction="sortDirection"
          server-pagination
          :current-page="currentPage"
          :total-items="totalItems"
          :page-size="pageSize"
          item-name="contexts"
          @page-change="handlePageChange"
        >
          <template #cell-name="{ item: context }">
            <RouterLink :to="`/chatbot/ai/${context.id}`"><strong>{{ context.name }}</strong></RouterLink>
          </template>
          <template #cell-context_type="{ item: context }">
            <span class="badge" data-variant="secondary">
              {{ context.context_type === 'api' ? $t('aiContexts.apiFetch') : $t('aiContexts.static') }}
            </span>
          </template>
          <template #cell-trigger_keywords="{ item: context }">
            <div class="hstack gap-1 flex-wrap">
              <span v-for="kw in context.trigger_keywords?.slice(0, 2)" :key="kw" class="badge" data-variant="secondary">{{ kw }}</span>
              <span v-if="context.trigger_keywords?.length > 2" class="badge outline">+{{ context.trigger_keywords.length - 2 }}</span>
              <small v-if="!context.trigger_keywords?.length" class="text-light">{{ $t('aiContexts.always') }}</small>
            </div>
          </template>
          <template #cell-priority="{ item: context }">
            <span class="text-light">{{ context.priority }}</span>
          </template>
          <template #cell-status="{ item: context }">
            <label class="hstack gap-2">
              <input type="checkbox" role="switch" :checked="context.enabled" :aria-label="$t('aiContexts.status')" @change="toggleContext(context, $event)">
              <small class="text-light">{{ context.enabled ? $t('aiContexts.active') : $t('aiContexts.inactive') }}</small>
            </label>
          </template>
          <template #cell-actions="{ item: context }">
            <div class="hstack gap-1 justify-end">
              <RouterLink :to="`/chatbot/ai/${context.id}`" class="button ghost icon" :title="$t('aiContexts.editContextLabel')" :aria-label="$t('aiContexts.editContextLabel')"><Pencil :size="16" /></RouterLink>
              <IconButton :icon="Trash2" :label="$t('aiContexts.deleteContextLabel')" @click="openDeleteDialog(context)" />
            </div>
          </template>
          <template #empty-action>
            <RouterLink v-if="!searchQuery" to="/chatbot/ai/new" class="button outline small">
              <Plus :size="16" />
              {{ $t('aiContexts.addContext') }}
            </RouterLink>
          </template>
        </DataTable>
      </article>
    </div>

    <DeleteConfirmDialog
      v-model:open="deleteDialogOpen"
      :title="$t('aiContexts.deleteContext')"
      :item-name="contextToDelete?.name"
      :is-submitting="isDeleting"
      @confirm="confirmDeleteContext"
    />
  </div>
</template>
