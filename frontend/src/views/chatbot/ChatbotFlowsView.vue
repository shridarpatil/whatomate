<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Switch } from '@/components/ui/switch'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { chatbotService } from '@/services/api'
import { toast } from 'vue-sonner'
import { PageHeader, DataTable, DeleteConfirmDialog, SearchInput, type Column } from '@/components/shared'
import { getErrorMessage } from '@/lib/api-utils'
import { Plus, Pencil, Trash2, Workflow } from 'lucide-vue-next'
import { useDebounceFn } from '@vueuse/core'

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
const searchQuery = ref('')
const deleteDialogOpen = ref(false)
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
  try {
    const response = await chatbotService.listFlows({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    const data = (response.data as any).data || response.data
    flows.value = data.flows || []
    totalItems.value = data.total ?? flows.value.length
  } catch (error) {
    console.error('Failed to load flows:', error)
    flows.value = []
  } finally {
    isLoading.value = false
  }
}

// Debounced search
const debouncedSearch = useDebounceFn(() => {
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

async function toggleFlow(flow: ChatbotFlow) {
  try {
    await chatbotService.updateFlow(flow.id, { enabled: !flow.enabled })
    flow.enabled = !flow.enabled
    toast.success(flow.enabled ? t('common.enabledSuccess', { resource: t('resources.Flow') }) : t('common.disabledSuccess', { resource: t('resources.Flow') }))
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('common.failedToggle', { resource: t('resources.flow') })))
  }
}

function openDeleteDialog(flow: ChatbotFlow) {
  flowToDelete.value = flow
  deleteDialogOpen.value = true
}

async function confirmDeleteFlow() {
  if (!flowToDelete.value) return

  try {
    await chatbotService.deleteFlow(flowToDelete.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.Flow') }))
    deleteDialogOpen.value = false
    flowToDelete.value = null
    await fetchFlows()
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('common.failedDelete', { resource: t('resources.flow') })))
  }
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader
      :title="$t('chatbotFlows.title')"
      :icon="Workflow"
      icon-gradient="bg-gradient-to-br from-purple-500 to-pink-600 shadow-purple-500/20"
      back-link="/chatbot"
      :breadcrumbs="[{ label: $t('chatbotFlows.backToChatbot'), href: '/chatbot' }, { label: $t('nav.flows') }]"
    >
      <template #actions>
        <Button variant="outline" size="sm" @click="createFlow">
          <Plus class="h-4 w-4 mr-2" />
          {{ $t('chatbotFlows.createFlow') }}
        </Button>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto">
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>{{ $t('chatbotFlows.yourFlows') }}</CardTitle>
                  <CardDescription>{{ $t('chatbotFlows.yourFlowsDesc') }}</CardDescription>
                </div>
                <SearchInput v-model="searchQuery" :placeholder="$t('chatbotFlows.searchFlows') + '...'" class="w-64" />
              </div>
            </CardHeader>
            <CardContent>
              <DataTable
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
                  <span class="font-medium">{{ flow.name }}</span>
                </template>
                <template #cell-description="{ item: flow }">
                  <span class="text-muted-foreground max-w-[200px] truncate block">{{ flow.description || $t('chatbotFlows.noDescription') }}</span>
                </template>
                <template #cell-trigger_keywords="{ item: flow }">
                  <div class="flex flex-wrap gap-1">
                    <Badge v-for="keyword in flow.trigger_keywords?.slice(0, 2)" :key="keyword" variant="secondary" class="text-xs">
                      {{ keyword }}
                    </Badge>
                    <Badge v-if="flow.trigger_keywords?.length > 2" variant="outline" class="text-xs">
                      +{{ flow.trigger_keywords.length - 2 }}
                    </Badge>
                    <span v-if="!flow.trigger_keywords?.length" class="text-muted-foreground text-sm">—</span>
                  </div>
                </template>
                <template #cell-steps_count="{ item: flow }">
                  <span class="text-muted-foreground">{{ flow.steps_count }}</span>
                </template>
                <template #cell-status="{ item: flow }">
                  <div class="flex items-center gap-2">
                    <Switch :checked="flow.enabled" @update:checked="toggleFlow(flow)" />
                    <span class="text-sm text-muted-foreground">{{ flow.enabled ? $t('chatbotFlows.active') : $t('chatbotFlows.inactive') }}</span>
                  </div>
                </template>
                <template #cell-actions="{ item: flow }">
                  <div class="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click="editFlow(flow)">
                      <Pencil class="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="icon" class="h-8 w-8 text-destructive" @click="openDeleteDialog(flow)">
                      <Trash2 class="h-4 w-4" />
                    </Button>
                  </div>
                </template>
                <template #empty-action>
                  <Button v-if="!searchQuery" variant="outline" size="sm" @click="createFlow">
                    <Plus class="h-4 w-4 mr-2" />
                    {{ $t('chatbotFlows.createFlow') }}
                  </Button>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <DeleteConfirmDialog
      v-model:open="deleteDialogOpen"
      :title="$t('chatbotFlows.deleteFlow')"
      :item-name="flowToDelete?.name"
      @confirm="confirmDeleteFlow"
    />
  </div>
</template>
