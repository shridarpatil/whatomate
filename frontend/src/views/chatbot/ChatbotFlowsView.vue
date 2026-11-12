<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { chatbotService } from '@/services/api'
import { toast } from '@/lib/toast'
import { PageHeader, DataTable, DeleteConfirmDialog, SearchInput, IconButton, ErrorState, type Column } from '@/components/shared'
import { getErrorMessage } from '@/lib/api-utils'
import { Plus, Pencil, Trash2, Workflow } from '@/lib/icons'
import { debounce } from '@/lib/utils'

const { t } = useI18n()

interface ChatbotFlow {
  id: string
  name: string
  description: string
  trigger_keywords: string[]
  steps_count: number
  enabled: boolean
  created_at: string
}

const router = useRouter()
const flows = ref<ChatbotFlow[]>([])
const isLoading = ref(true)
const error = ref<string | null>(null)
const searchQuery = ref('')
const deleteDialogOpen = ref(false)
const isDeleting = ref(false)
const flowToDelete = ref<ChatbotFlow | null>(null)

// Pagination state
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 20

const columns = computed<Column<ChatbotFlow>[]>(() => [
  { key: 'name', label: t('chatbotFlows.name'), sortable: true },
  { key: 'description', label: t('chatbotFlows.description') },
  { key: 'trigger_keywords', label: t('chatbotFlows.keywords') },
  { key: 'steps_count', label: t('chatbotFlows.steps'), sortable: true },
  { key: 'status', label: t('chatbotFlows.status'), sortable: true, sortKey: 'enabled' },
  { key: 'actions', label: t('chatbotFlows.actions'), align: 'right' },
])

const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

onMounted(async () => {
  await fetchFlows()
})

async function fetchFlows() {
  isLoading.value = true
  error.value = null
  try {
    const response = await chatbotService.listFlows({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    const data = (response.data as any).data || response.data
    flows.value = data.flows || []
    totalItems.value = data.total ?? flows.value.length
  } catch (err) {
    console.error('Failed to load flows:', err)
    error.value = t('chatbotFlows.fetchError')
    flows.value = []
  } finally {
    isLoading.value = false
  }
}

// Debounced search
const debouncedSearch = debounce(() => {
  currentPage.value = 1
  fetchFlows()
}, 300)

watch(searchQuery, () => debouncedSearch())

function handlePageChange(page: number) {
  currentPage.value = page
  fetchFlows()
}

function createFlow() {
  router.push('/chatbot/flows/new')
}

function editFlow(flow: ChatbotFlow) {
  router.push(`/chatbot/flows/${flow.id}/edit`)
}

async function toggleFlow(flow: ChatbotFlow, event?: Event) {
  try {
    await chatbotService.updateFlow(flow.id, { enabled: !flow.enabled })
    flow.enabled = !flow.enabled
    toast.success(flow.enabled ? t('common.enabledSuccess', { resource: t('resources.Flow') }) : t('common.disabledSuccess', { resource: t('resources.Flow') }))
  } catch (error: any) {
    if (event?.target instanceof HTMLInputElement) event.target.checked = flow.enabled
    toast.error(getErrorMessage(error, t('common.failedToggle', { resource: t('resources.flow') })))
  }
}

function openDeleteDialog(flow: ChatbotFlow) {
  flowToDelete.value = flow
  deleteDialogOpen.value = true
}

async function confirmDeleteFlow() {
  if (!flowToDelete.value) return

  isDeleting.value = true
  try {
    await chatbotService.deleteFlow(flowToDelete.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.Flow') }))
    deleteDialogOpen.value = false
    flowToDelete.value = null
    await fetchFlows()
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('common.failedDelete', { resource: t('resources.flow') })))
  } finally {
    isDeleting.value = false
  }
}
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader
      :title="$t('chatbotFlows.title')"
      :icon="Workflow"
      back-link="/chatbot"
      :breadcrumbs="[{ label: $t('chatbotFlows.backToChatbot'), href: '/chatbot' }, { label: $t('nav.flows') }]"
    >
      <template #actions>
        <button type="button" class="outline small" @click="createFlow">
          <Plus :size="16" />
          {{ $t('chatbotFlows.createFlow') }}
        </button>
      </template>
    </PageHeader>

    <div class="flex-1 scroll-y p-4">
      <article class="card">
        <header class="hstack justify-between flex-wrap gap-4">
          <div>
            <h3>{{ $t('chatbotFlows.yourFlows') }}</h3>
            <p class="text-light">{{ $t('chatbotFlows.yourFlowsDesc') }}</p>
          </div>
          <SearchInput v-model="searchQuery" :placeholder="$t('chatbotFlows.searchFlows') + '...'" />
        </header>
        <ErrorState
          v-if="error"
          :title="$t('common.loadErrorTitle')"
          :description="error"
          :retry-label="$t('common.retry')"
          @retry="fetchFlows"
        />
        <DataTable
          v-else
          :items="flows"
          :columns="columns"
          :is-loading="isLoading"
          :empty-icon="Workflow"
          :empty-title="searchQuery ? $t('chatbotFlows.noMatchingFlows') : $t('chatbotFlows.noFlowsYet')"
          :empty-description="searchQuery ? $t('chatbotFlows.noMatchingFlowsDesc') : $t('chatbotFlows.noFlowsYetDesc')"
          server-pagination
          :current-page="currentPage"
          :total-items="totalItems"
          :page-size="pageSize"
          item-name="flows"
          @page-change="handlePageChange"
          v-model:sort-key="sortKey"
          v-model:sort-direction="sortDirection"
        >
          <template #cell-name="{ item: flow }">
            <strong>{{ flow.name }}</strong>
          </template>
          <template #cell-description="{ item: flow }">
            <span class="text-light truncate">{{ flow.description || $t('chatbotFlows.noDescription') }}</span>
          </template>
          <template #cell-trigger_keywords="{ item: flow }">
            <div class="hstack gap-1 flex-wrap">
              <span v-for="keyword in flow.trigger_keywords?.slice(0, 2)" :key="keyword" class="badge" data-variant="secondary">{{ keyword }}</span>
              <span v-if="flow.trigger_keywords?.length > 2" class="badge outline">+{{ flow.trigger_keywords.length - 2 }}</span>
              <small v-if="!flow.trigger_keywords?.length" class="text-light">—</small>
            </div>
          </template>
          <template #cell-steps_count="{ item: flow }">
            <span class="text-light">{{ flow.steps_count }}</span>
          </template>
          <template #cell-status="{ item: flow }">
            <label class="hstack gap-2">
              <input type="checkbox" role="switch" :checked="flow.enabled" :aria-label="$t('chatbotFlows.status')" @change="toggleFlow(flow, $event)">
              <small class="text-light">{{ flow.enabled ? $t('chatbotFlows.active') : $t('chatbotFlows.inactive') }}</small>
            </label>
          </template>
          <template #cell-actions="{ item: flow }">
            <div class="hstack gap-1 justify-end">
              <IconButton :icon="Pencil" :label="$t('chatbotFlows.editFlowLabel')" @click="editFlow(flow)" />
              <IconButton :icon="Trash2" :label="$t('chatbotFlows.deleteFlowLabel')" @click="openDeleteDialog(flow)" />
            </div>
          </template>
          <template #empty-action>
            <button v-if="!searchQuery" type="button" class="outline small" @click="createFlow">
              <Plus :size="16" />
              {{ $t('chatbotFlows.createFlow') }}
            </button>
          </template>
        </DataTable>
      </article>
    </div>

    <DeleteConfirmDialog
      v-model:open="deleteDialogOpen"
      :title="$t('chatbotFlows.deleteFlow')"
      :item-name="flowToDelete?.name"
      :is-submitting="isDeleting"
      @confirm="confirmDeleteFlow"
    />
  </div>
</template>
