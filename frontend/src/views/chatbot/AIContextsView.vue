<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Switch } from '@/components/ui/switch'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { chatbotService } from '@/services/api'
import { toast } from 'vue-sonner'
import { PageHeader, DataTable, DeleteConfirmDialog, SearchInput, IconButton, ErrorState, type Column } from '@/components/shared'
import { getErrorMessage } from '@/lib/api-utils'
import { Plus, Pencil, Trash2, Sparkles } from 'lucide-vue-next'
import { useDebounceFn } from '@vueuse/core'

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
const debouncedSearch = useDebounceFn(() => {
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

async function toggleContext(context: AIContext) {
  try {
    await chatbotService.updateAIContext(context.id, { enabled: !context.enabled })
    context.enabled = !context.enabled
    toast.success(context.enabled ? t('common.enabledSuccess', { resource: t('resources.AIContext') }) : t('common.disabledSuccess', { resource: t('resources.AIContext') }))
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('common.failedToggle', { resource: t('resources.AIContext') })))
  }
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader
      :title="$t('aiContexts.title')"
      :icon="Sparkles"
      icon-gradient="bg-gradient-to-br from-orange-500 to-amber-600 shadow-orange-500/20"
      back-link="/chatbot"
      :breadcrumbs="[{ label: $t('aiContexts.backToChatbot'), href: '/chatbot' }, { label: $t('nav.aiContexts') }]"
    >
      <template #actions>
        <RouterLink to="/chatbot/ai/new">
          <Button variant="outline" size="sm">
            <Plus class="h-4 w-4 mr-2" />
            {{ $t('aiContexts.addContext') }}
          </Button>
        </RouterLink>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div>
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>{{ $t('aiContexts.yourContexts') }}</CardTitle>
                  <CardDescription>{{ $t('aiContexts.yourContextsDesc') }}</CardDescription>
                </div>
                <SearchInput v-model="searchQuery" :placeholder="$t('aiContexts.searchContexts') + '...'" class="w-64" />
              </div>
            </CardHeader>
            <CardContent>
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
                  <RouterLink :to="`/chatbot/ai/${context.id}`" class="font-medium text-inherit no-underline hover:opacity-80">{{ context.name }}</RouterLink>
                </template>
                <template #cell-context_type="{ item: context }">
                  <Badge
                    :class="context.context_type === 'api'
                      ? 'bg-blue-500/20 text-blue-400 border-transparent'
                      : 'bg-orange-500/20 text-orange-400 border-transparent'"
                    class="text-xs"
                  >
                    {{ context.context_type === 'api' ? $t('aiContexts.apiFetch') : $t('aiContexts.static') }}
                  </Badge>
                </template>
                <template #cell-trigger_keywords="{ item: context }">
                  <div class="flex flex-wrap gap-1">
                    <Badge v-for="kw in context.trigger_keywords?.slice(0, 2)" :key="kw" variant="secondary" class="text-xs">
                      {{ kw }}
                    </Badge>
                    <Badge v-if="context.trigger_keywords?.length > 2" variant="outline" class="text-xs">
                      +{{ context.trigger_keywords.length - 2 }}
                    </Badge>
                    <span v-if="!context.trigger_keywords?.length" class="text-muted-foreground text-sm">{{ $t('aiContexts.always') }}</span>
                  </div>
                </template>
                <template #cell-priority="{ item: context }">
                  <span class="text-muted-foreground">{{ context.priority }}</span>
                </template>
                <template #cell-status="{ item: context }">
                  <div class="flex items-center gap-2">
                    <Switch :checked="context.enabled" @update:checked="toggleContext(context)" />
                    <span class="text-sm text-muted-foreground">{{ context.enabled ? $t('aiContexts.active') : $t('aiContexts.inactive') }}</span>
                  </div>
                </template>
                <template #cell-actions="{ item: context }">
                  <div class="flex items-center justify-end gap-1">
                    <RouterLink :to="`/chatbot/ai/${context.id}`"><IconButton :icon="Pencil" :label="$t('aiContexts.editContextLabel')" class="h-8 w-8" /></RouterLink>
                    <IconButton :icon="Trash2" :label="$t('aiContexts.deleteContextLabel')" class="h-8 w-8 text-destructive" @click="openDeleteDialog(context)" />
                  </div>
                </template>
                <template #empty-action>
                  <RouterLink v-if="!searchQuery" to="/chatbot/ai/new">
                    <Button variant="outline" size="sm">
                      <Plus class="h-4 w-4 mr-2" />
                      {{ $t('aiContexts.addContext') }}
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
      :title="$t('aiContexts.deleteContext')"
      :item-name="contextToDelete?.name"
      :is-submitting="isDeleting"
      @confirm="confirmDeleteContext"
    />
  </div>
</template>
