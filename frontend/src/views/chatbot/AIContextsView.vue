<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Switch } from '@/components/ui/switch'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { chatbotService } from '@/services/api'
import { toast } from 'vue-sonner'
import { PageHeader, DataTable, DeleteConfirmDialog, SearchInput, type Column } from '@/components/shared'
import { getErrorMessage } from '@/lib/api-utils'
import { Plus, Pencil, Trash2, Sparkles } from 'lucide-vue-next'
import { useDebounceFn } from '@vueuse/core'

const { t } = useI18n()

interface ApiConfig {
  url: string
  method: string
  headers: Record<string, string>
  body: string
  response_path: string
}

interface AIContext {
  id: string
  name: string
  context_type: string
  trigger_keywords: string[]
  static_content: string
  api_config: ApiConfig
  priority: number
  enabled: boolean
  created_at: string
}

const contexts = ref<AIContext[]>([])
const isLoading = ref(true)
const searchQuery = ref('')
const isDialogOpen = ref(false)
const isSubmitting = ref(false)
const editingContext = ref<AIContext | null>(null)
const deleteDialogOpen = ref(false)
const contextToDelete = ref<AIContext | null>(null)

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

const formData = ref({
  name: '',
  context_type: 'static',
  trigger_keywords: '',
  static_content: '',
  api_url: '',
  api_method: 'GET',
  api_headers: '',
  api_response_path: '',
  priority: 10,
  enabled: true
})

// Helper to display variable placeholders without Vue parsing issues
const variableExample = (name: string) => `{{${name}}}`

onMounted(async () => {
  await fetchContexts()
})

async function fetchContexts() {
  isLoading.value = true
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
  } catch (error) {
    console.error('Failed to load AI contexts:', error)
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

function openCreateDialog() {
  editingContext.value = null
  formData.value = {
    name: '',
    context_type: 'static',
    trigger_keywords: '',
    static_content: '',
    api_url: '',
    api_method: 'GET',
    api_headers: '',
    api_response_path: '',
    priority: 10,
    enabled: true
  }
  isDialogOpen.value = true
}

function openEditDialog(context: AIContext) {
  editingContext.value = context
  const apiConfig = context.api_config || {} as ApiConfig
  formData.value = {
    name: context.name,
    context_type: context.context_type || 'static',
    trigger_keywords: (context.trigger_keywords || []).join(', '),
    static_content: context.static_content || '',
    api_url: apiConfig.url || '',
    api_method: apiConfig.method || 'GET',
    api_headers: apiConfig.headers ? JSON.stringify(apiConfig.headers, null, 2) : '',
    api_response_path: apiConfig.response_path || '',
    priority: context.priority || 10,
    enabled: context.enabled
  }
  isDialogOpen.value = true
}

async function saveContext() {
  if (!formData.value.name.trim()) {
    toast.error(t('aiContexts.enterName'))
    return
  }

  if (formData.value.context_type === 'api' && !formData.value.api_url.trim()) {
    toast.error(t('aiContexts.enterApiUrl'))
    return
  }

  isSubmitting.value = true
  try {
    // Parse headers JSON if provided
    let headers = {}
    if (formData.value.api_headers.trim()) {
      try {
        headers = JSON.parse(formData.value.api_headers)
      } catch (e) {
        toast.error(t('aiContexts.invalidHeaders'))
        isSubmitting.value = false
        return
      }
    }

    const data: any = {
      name: formData.value.name,
      context_type: formData.value.context_type,
      trigger_keywords: formData.value.trigger_keywords.split(',').map(k => k.trim()).filter(Boolean),
      static_content: formData.value.static_content,
      api_config: formData.value.context_type === 'api' ? {
        url: formData.value.api_url,
        method: formData.value.api_method,
        headers: headers,
        response_path: formData.value.api_response_path
      } : null,
      priority: formData.value.priority,
      enabled: formData.value.enabled
    }

    if (editingContext.value) {
      await chatbotService.updateAIContext(editingContext.value.id, data)
      toast.success(t('common.updatedSuccess', { resource: t('resources.AIContext') }))
    } else {
      await chatbotService.createAIContext(data)
      toast.success(t('common.createdSuccess', { resource: t('resources.AIContext') }))
    }

    isDialogOpen.value = false
    await fetchContexts()
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('common.failedSave', { resource: t('resources.AIContext') })))
  } finally {
    isSubmitting.value = false
  }
}

function openDeleteDialog(context: AIContext) {
  contextToDelete.value = context
  deleteDialogOpen.value = true
}

async function confirmDeleteContext() {
  if (!contextToDelete.value) return

  try {
    await chatbotService.deleteAIContext(contextToDelete.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.AIContext') }))
    deleteDialogOpen.value = false
    contextToDelete.value = null
    await fetchContexts()
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('common.failedDelete', { resource: t('resources.AIContext') })))
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
        <Button variant="outline" size="sm" @click="openCreateDialog">
          <Plus class="h-4 w-4 mr-2" />
          {{ $t('aiContexts.addContext') }}
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
                  <CardTitle>{{ $t('aiContexts.yourContexts') }}</CardTitle>
                  <CardDescription>{{ $t('aiContexts.yourContextsDesc') }}</CardDescription>
                </div>
                <SearchInput v-model="searchQuery" :placeholder="$t('aiContexts.searchContexts') + '...'" class="w-64" />
              </div>
            </CardHeader>
            <CardContent>
              <DataTable
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
                  <span class="font-medium">{{ context.name }}</span>
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
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click="openEditDialog(context)">
                      <Pencil class="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="icon" class="h-8 w-8 text-destructive" @click="openDeleteDialog(context)">
                      <Trash2 class="h-4 w-4" />
                    </Button>
                  </div>
                </template>
                <template #empty-action>
                  <Button v-if="!searchQuery" variant="outline" size="sm" @click="openCreateDialog">
                    <Plus class="h-4 w-4 mr-2" />
                    {{ $t('aiContexts.addContext') }}
                  </Button>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <!-- Create/Edit Dialog -->
    <Dialog v-model:open="isDialogOpen">
      <DialogContent class="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{{ editingContext ? $t('aiContexts.editContext') : $t('aiContexts.createContext') }} {{ $t('aiContexts.aiContext') }}</DialogTitle>
          <DialogDescription>
            {{ $t('aiContexts.dialogDesc') }}
          </DialogDescription>
        </DialogHeader>
        <div class="grid gap-4 py-4 max-h-[60vh] overflow-y-auto">
          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-2">
              <Label for="name">{{ $t('aiContexts.nameRequired') }}</Label>
              <Input
                id="name"
                v-model="formData.name"
                :placeholder="$t('aiContexts.namePlaceholder')"
              />
            </div>
            <div class="space-y-2">
              <Label for="context_type">{{ $t('aiContexts.contextType') }}</Label>
              <Select v-model="formData.context_type">
                <SelectTrigger>
                  <SelectValue :placeholder="$t('aiContexts.selectType')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="static">{{ $t('aiContexts.staticContent') }}</SelectItem>
                  <SelectItem value="api">{{ $t('aiContexts.apiFetch') }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div class="space-y-2">
            <Label for="trigger_keywords">{{ $t('aiContexts.triggerKeywords') }}</Label>
            <Input
              id="trigger_keywords"
              v-model="formData.trigger_keywords"
              :placeholder="$t('aiContexts.triggerKeywordsPlaceholder')"
            />
            <p class="text-xs text-muted-foreground">
              {{ $t('aiContexts.triggerKeywordsHint') }}
            </p>
          </div>

          <div class="space-y-2">
            <Label for="static_content">{{ $t('aiContexts.contentPrompt') }}</Label>
            <Textarea
              id="static_content"
              v-model="formData.static_content"
              :placeholder="$t('aiContexts.contentPlaceholder') + '...'"
              :rows="6"
            />
            <p class="text-xs text-muted-foreground">
              {{ $t('aiContexts.contentHint') }}
            </p>
          </div>

          <div v-if="formData.context_type === 'api'" class="space-y-4 border-t pt-4">
            <p class="text-sm font-medium">{{ $t('aiContexts.apiConfiguration') }}</p>
            <p class="text-xs text-muted-foreground">{{ $t('aiContexts.apiConfigHint') }}</p>

            <div class="grid grid-cols-4 gap-4">
              <div class="col-span-1 space-y-2">
                <Label for="api_method">{{ $t('aiContexts.method') }}</Label>
                <Select v-model="formData.api_method">
                  <SelectTrigger>
                    <SelectValue :placeholder="$t('aiContexts.method')" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="GET">GET</SelectItem>
                    <SelectItem value="POST">POST</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div class="col-span-3 space-y-2">
                <Label for="api_url">{{ $t('aiContexts.apiUrl') }}</Label>
                <Input
                  id="api_url"
                  v-model="formData.api_url"
                  :placeholder="$t('aiContexts.apiUrlPlaceholder')"
                />
              </div>
            </div>
            <p class="text-xs text-muted-foreground">
              {{ $t('aiContexts.variables') }}: <code class="bg-muted px-1 rounded">{{ variableExample('phone_number') }}</code>, <code class="bg-muted px-1 rounded">{{ variableExample('user_message') }}</code>
            </p>

            <div class="space-y-2">
              <Label for="api_headers">{{ $t('aiContexts.headersOptional') }}</Label>
              <Textarea
                id="api_headers"
                v-model="formData.api_headers"
                :placeholder="$t('aiContexts.headersPlaceholder')"
                :rows="2"
              />
              <p class="text-xs text-muted-foreground">
                {{ $t('aiContexts.headersHint') }}
              </p>
            </div>

            <div class="space-y-2">
              <Label for="api_response_path">{{ $t('aiContexts.responsePath') }}</Label>
              <Input
                id="api_response_path"
                v-model="formData.api_response_path"
                :placeholder="$t('aiContexts.responsePathPlaceholder')"
              />
              <p class="text-xs text-muted-foreground">{{ $t('aiContexts.responsePathHint') }}</p>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-2">
              <Label for="priority">{{ $t('aiContexts.priorityLabel') }}</Label>
              <Input
                id="priority"
                v-model.number="formData.priority"
                type="number"
                min="1"
                max="100"
              />
              <p class="text-xs text-muted-foreground">{{ $t('aiContexts.priorityHint') }}</p>
            </div>
            <div class="flex items-center gap-2 pt-8">
              <Switch
                id="enabled"
                :checked="formData.enabled"
                @update:checked="formData.enabled = $event"
              />
              <Label for="enabled">{{ $t('aiContexts.enabled') }}</Label>
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" size="sm" @click="isDialogOpen = false">{{ $t('common.cancel') }}</Button>
          <Button size="sm" @click="saveContext" :disabled="isSubmitting">
            {{ editingContext ? $t('common.update') : $t('common.create') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <DeleteConfirmDialog
      v-model:open="deleteDialogOpen"
      :title="$t('aiContexts.deleteContext')"
      :item-name="contextToDelete?.name"
      @confirm="confirmDeleteContext"
    />
  </div>
</template>
