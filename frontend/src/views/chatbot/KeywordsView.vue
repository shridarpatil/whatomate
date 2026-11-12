<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { chatbotService } from '@/services/api'
import { toast } from '@/lib/toast'
import { PageHeader, SearchInput, DataTable, DeleteConfirmDialog, IconButton, ErrorState, type Column } from '@/components/shared'
import { getErrorMessage } from '@/lib/api-utils'
import { Plus, Pencil, Trash2, Key } from '@/lib/icons'
import { useSearchPagination } from '@/composables/useSearchPagination'

const { t } = useI18n()

interface KeywordRule {
  id: string
  keywords: string[]
  match_type: 'exact' | 'contains' | 'regex'
  response_type: 'text' | 'template' | 'flow' | 'transfer'
  response_content: any
  priority: number
  enabled: boolean
  created_at: string
}

const rules = ref<KeywordRule[]>([])
const isLoading = ref(true)
const isDeleting = ref(false)
const error = ref<string | null>(null)
const deleteDialogOpen = ref(false)
const ruleToDelete = ref<KeywordRule | null>(null)

function openDeleteDialog(rule: KeywordRule) {
  ruleToDelete.value = rule
  deleteDialogOpen.value = true
}

function closeDeleteDialog() {
  deleteDialogOpen.value = false
  ruleToDelete.value = null
}

const { searchQuery, currentPage, totalItems, pageSize, handlePageChange } = useSearchPagination({
  fetchFn: () => fetchRules(),
})

const columns = computed<Column<KeywordRule>[]>(() => [
  { key: 'keywords', label: t('keywords.keywordsColumn') },
  { key: 'match_type', label: t('keywords.matchType'), sortable: true },
  { key: 'response_type', label: t('keywords.response'), sortable: true },
  { key: 'priority', label: t('keywords.priority'), sortable: true },
  { key: 'status', label: t('keywords.status'), sortable: true, sortKey: 'enabled' },
  { key: 'actions', label: t('keywords.actions'), align: 'right' },
])

const sortKey = ref('priority')
const sortDirection = ref<'asc' | 'desc'>('desc')

onMounted(async () => {
  await fetchRules()
})

async function fetchRules() {
  isLoading.value = true
  error.value = null
  try {
    const response = await chatbotService.listKeywords({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    // API response is wrapped in { status: "success", data: { rules: [...] } }
    const data = (response.data as any).data || response.data
    rules.value = data.rules || []
    totalItems.value = data.total ?? rules.value.length
  } catch (err) {
    console.error('Failed to load keyword rules:', err)
    error.value = t('keywords.fetchError')
    rules.value = []
  } finally {
    isLoading.value = false
  }
}

async function confirmDeleteRule() {
  if (!ruleToDelete.value) return

  isDeleting.value = true
  try {
    await chatbotService.deleteKeyword(ruleToDelete.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.KeywordRule') }))
    closeDeleteDialog()
    await fetchRules()
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('common.failedDelete', { resource: t('resources.keywordRule') })))
  } finally {
    isDeleting.value = false
  }
}

async function toggleRule(rule: KeywordRule, event?: Event) {
  try {
    await chatbotService.updateKeyword(rule.id, { enabled: !rule.enabled })
    rule.enabled = !rule.enabled
    toast.success(rule.enabled ? t('common.enabledSuccess', { resource: t('resources.KeywordRule') }) : t('common.disabledSuccess', { resource: t('resources.KeywordRule') }))
  } catch (error: any) {
    if (event?.target instanceof HTMLInputElement) event.target.checked = rule.enabled
    toast.error(getErrorMessage(error, t('common.failedToggle', { resource: t('resources.keywordRule') })))
  }
}

const emptyDescription = computed(() => {
  if (searchQuery.value) {
    return t('keywords.noMatchingRulesDesc', { query: searchQuery.value })
  }
  return t('keywords.noRulesYetDesc')
})
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader
      :title="$t('keywords.title')"
      :icon="Key"
      back-link="/chatbot"
      :breadcrumbs="[{ label: $t('keywords.backToChatbot'), href: '/chatbot' }, { label: $t('nav.keywords') }]"
    >
      <template #actions>
        <RouterLink to="/chatbot/keywords/new" class="button outline small">
          <Plus :size="16" />
          {{ $t('keywords.addRule') }}
        </RouterLink>
      </template>
    </PageHeader>

    <div class="flex-1 scroll-y p-4">
      <article class="card">
        <header class="hstack justify-between flex-wrap gap-4">
          <h3>{{ $t('keywords.yourRules') }}</h3>
          <SearchInput v-model="searchQuery" :placeholder="$t('keywords.searchKeywords') + '...'" />
        </header>
        <ErrorState
          v-if="error"
          :title="$t('common.loadErrorTitle')"
          :description="error"
          :retry-label="$t('common.retry')"
          @retry="fetchRules"
        />
        <DataTable
          v-else
          :items="rules"
          :columns="columns"
          :is-loading="isLoading"
          :empty-icon="Key"
          :empty-title="searchQuery ? $t('keywords.noMatchingRules') : $t('keywords.noRulesYet')"
          :empty-description="emptyDescription"
          v-model:sort-key="sortKey"
          v-model:sort-direction="sortDirection"
          server-pagination
          :current-page="currentPage"
          :total-items="totalItems"
          :page-size="pageSize"
          item-name="rules"
          @page-change="handlePageChange"
        >
          <template #cell-keywords="{ item: rule }">
            <RouterLink :to="`/chatbot/keywords/${rule.id}`" class="hstack gap-1 flex-wrap">
              <span v-for="keyword in rule.keywords.slice(0, 3)" :key="keyword" class="badge outline">{{ keyword }}</span>
              <span v-if="rule.keywords.length > 3" class="badge outline">+{{ rule.keywords.length - 3 }}</span>
            </RouterLink>
          </template>
          <template #cell-match_type="{ item: rule }">
            <span class="badge" data-variant="secondary">{{ rule.match_type }}</span>
          </template>
          <template #cell-response_type="{ item: rule }">
            <span class="badge" :data-variant="rule.response_type === 'transfer' ? 'danger' : 'secondary'">
              {{ rule.response_type === 'transfer' ? $t('keywords.transfer') : $t('keywords.text') }}
            </span>
          </template>
          <template #cell-priority="{ item: rule }">
            <span class="text-light">{{ rule.priority }}</span>
          </template>
          <template #cell-status="{ item: rule }">
            <label class="hstack gap-2">
              <input type="checkbox" role="switch" :checked="rule.enabled" :aria-label="$t('keywords.status')" @change="toggleRule(rule, $event)">
              <small class="text-light">{{ rule.enabled ? $t('keywords.active') : $t('keywords.inactive') }}</small>
            </label>
          </template>
          <template #cell-actions="{ item: rule }">
            <div class="hstack gap-1 justify-end">
              <RouterLink :to="`/chatbot/keywords/${rule.id}`" class="button ghost icon" :title="$t('keywords.editRuleLabel')" :aria-label="$t('keywords.editRuleLabel')"><Pencil :size="16" /></RouterLink>
              <IconButton :icon="Trash2" :label="$t('keywords.deleteRuleLabel')" @click="openDeleteDialog(rule)" />
            </div>
          </template>
          <template #empty-action>
            <RouterLink v-if="!searchQuery" to="/chatbot/keywords/new" class="button outline small">
              <Plus :size="16" />
              {{ $t('keywords.addRule') }}
            </RouterLink>
          </template>
        </DataTable>
      </article>
    </div>

    <DeleteConfirmDialog
      v-model:open="deleteDialogOpen"
      :title="$t('keywords.deleteRule')"
      :description="$t('keywords.deleteRuleDesc')"
      :is-submitting="isDeleting"
      @confirm="confirmDeleteRule"
    />
  </div>
</template>
