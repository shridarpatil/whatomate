<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Switch } from '@/components/ui/switch'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { chatbotService } from '@/services/api'
import { toast } from 'vue-sonner'
import { PageHeader, SearchInput, DataTable, DeleteConfirmDialog, IconButton, ErrorState, type Column } from '@/components/shared'
import { getErrorMessage } from '@/lib/api-utils'
import { Plus, Pencil, Trash2, Key } from 'lucide-vue-next'
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

async function toggleRule(rule: KeywordRule) {
  try {
    await chatbotService.updateKeyword(rule.id, { enabled: !rule.enabled })
    rule.enabled = !rule.enabled
    toast.success(rule.enabled ? t('common.enabledSuccess', { resource: t('resources.KeywordRule') }) : t('common.disabledSuccess', { resource: t('resources.KeywordRule') }))
  } catch (error: any) {
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
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader
      :title="$t('keywords.title')"
      :icon="Key"
      icon-gradient="bg-gradient-to-br from-blue-500 to-cyan-600 shadow-blue-500/20"
      back-link="/chatbot"
      :breadcrumbs="[{ label: $t('keywords.backToChatbot'), href: '/chatbot' }, { label: $t('nav.keywords') }]"
    >
      <template #actions>
        <RouterLink to="/chatbot/keywords/new">
          <Button variant="outline" size="sm">
            <Plus class="h-4 w-4 mr-2" />
            {{ $t('keywords.addRule') }}
          </Button>
        </RouterLink>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div>
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between">
                <div>
                  <CardTitle>{{ $t('keywords.yourRules') }}</CardTitle>
                  <CardDescription>{{ $t('keywords.yourRulesDesc') }}</CardDescription>
                </div>
                <SearchInput v-model="searchQuery" :placeholder="$t('keywords.searchKeywords') + '...'" class="w-64" />
              </div>
            </CardHeader>
            <CardContent>
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
                  <RouterLink :to="`/chatbot/keywords/${rule.id}`" class="flex flex-wrap gap-1 text-inherit no-underline hover:opacity-80">
                    <Badge v-for="keyword in rule.keywords.slice(0, 3)" :key="keyword" variant="outline" class="text-xs">
                      {{ keyword }}
                    </Badge>
                    <Badge v-if="rule.keywords.length > 3" variant="outline" class="text-xs">
                      +{{ rule.keywords.length - 3 }}
                    </Badge>
                  </RouterLink>
                </template>
                <template #cell-match_type="{ item: rule }">
                  <Badge class="text-xs capitalize bg-blue-500/20 text-blue-400 border-transparent">{{ rule.match_type }}</Badge>
                </template>
                <template #cell-response_type="{ item: rule }">
                  <Badge
                    :class="rule.response_type === 'transfer'
                      ? 'bg-red-500/20 text-red-400 border-transparent light:bg-red-100 light:text-red-700'
                      : 'bg-purple-500/20 text-purple-400 border-transparent light:bg-purple-100 light:text-purple-700'"
                    class="text-xs"
                  >
                    {{ rule.response_type === 'transfer' ? $t('keywords.transfer') : $t('keywords.text') }}
                  </Badge>
                </template>
                <template #cell-priority="{ item: rule }">
                  <span class="text-muted-foreground">{{ rule.priority }}</span>
                </template>
                <template #cell-status="{ item: rule }">
                  <div class="flex items-center gap-2">
                    <Switch :checked="rule.enabled" @update:checked="toggleRule(rule)" />
                    <span class="text-sm text-muted-foreground">{{ rule.enabled ? $t('keywords.active') : $t('keywords.inactive') }}</span>
                  </div>
                </template>
                <template #cell-actions="{ item: rule }">
                  <div class="flex items-center justify-end gap-1">
                    <RouterLink :to="`/chatbot/keywords/${rule.id}`"><IconButton :icon="Pencil" :label="$t('keywords.editRuleLabel')" class="h-8 w-8" /></RouterLink>
                    <IconButton :icon="Trash2" :label="$t('keywords.deleteRuleLabel')" class="h-8 w-8 text-destructive" @click="openDeleteDialog(rule)" />
                  </div>
                </template>
                <template #empty-action>
                  <RouterLink v-if="!searchQuery" to="/chatbot/keywords/new">
                    <Button variant="outline" size="sm">
                      <Plus class="h-4 w-4 mr-2" />
                      {{ $t('keywords.addRule') }}
                    </Button>
                  </RouterLink>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <DeleteConfirmDialog
      v-model:open="deleteDialogOpen"
      :title="$t('keywords.deleteRule')"
      :description="$t('keywords.deleteRuleDesc')"
      :is-submitting="isDeleting"
      @confirm="confirmDeleteRule"
    />
  </div>
</template>
