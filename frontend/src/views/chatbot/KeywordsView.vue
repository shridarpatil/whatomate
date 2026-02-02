<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
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
import { PageHeader, SearchInput, DataTable, DeleteConfirmDialog, type Column } from '@/components/shared'
import { getErrorMessage } from '@/lib/api-utils'
import { Plus, Pencil, Trash2, Key } from 'lucide-vue-next'
import { useDebounceFn } from '@vueuse/core'

interface ButtonItem {
  id: string
  title: string
}

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
const isDialogOpen = ref(false)
const isSubmitting = ref(false)
const searchQuery = ref('')
const editingRule = ref<KeywordRule | null>(null)
const deleteDialogOpen = ref(false)
const ruleToDelete = ref<KeywordRule | null>(null)

// Pagination state
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 20

const columns: Column<KeywordRule>[] = [
  { key: 'keywords', label: 'Keywords' },
  { key: 'match_type', label: 'Match Type', sortable: true },
  { key: 'response_type', label: 'Response', sortable: true },
  { key: 'priority', label: 'Priority', sortable: true },
  { key: 'status', label: 'Status', sortable: true, sortKey: 'enabled' },
  { key: 'actions', label: 'Actions', align: 'right' },
]

const sortKey = ref('priority')
const sortDirection = ref<'asc' | 'desc'>('desc')

const formData = ref({
  keywords: '',
  match_type: 'contains' as 'exact' | 'contains' | 'regex',
  response_type: 'text' as 'template' | 'text' | 'flow' | 'transfer',
  response_content: '',
  buttons: [] as ButtonItem[],
  priority: 0,
  enabled: true
})

function addButton() {
  if (formData.value.buttons.length >= 10) {
    toast.error('Maximum 10 buttons allowed')
    return
  }
  formData.value.buttons.push({ id: '', title: '' })
}

function removeButton(index: number) {
  formData.value.buttons.splice(index, 1)
}

onMounted(async () => {
  await fetchRules()
})

async function fetchRules() {
  isLoading.value = true
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
  } catch (error) {
    console.error('Failed to load keyword rules:', error)
    rules.value = []
  } finally {
    isLoading.value = false
  }
}

// Debounced search to avoid too many API calls
const debouncedSearch = useDebounceFn(() => {
  currentPage.value = 1
  fetchRules()
}, 300)

// Watch search query changes
watch(searchQuery, () => {
  debouncedSearch()
})

function handlePageChange(page: number) {
  currentPage.value = page
  fetchRules()
}

function openCreateDialog() {
  editingRule.value = null
  formData.value = {
    keywords: '',
    match_type: 'contains',
    response_type: 'text',
    response_content: '',
    buttons: [],
    priority: 0,
    enabled: true
  }
  isDialogOpen.value = true
}

function openEditDialog(rule: KeywordRule) {
  editingRule.value = rule
  formData.value = {
    keywords: rule.keywords.join(', '),
    match_type: rule.match_type,
    response_type: rule.response_type,
    response_content: rule.response_content?.body || '',
    buttons: rule.response_content?.buttons || [],
    priority: rule.priority,
    enabled: rule.enabled
  }
  isDialogOpen.value = true
}

async function saveRule() {
  if (!formData.value.keywords.trim()) {
    toast.error('Please enter at least one keyword')
    return
  }

  // Response content is required for text, optional for transfer
  if (formData.value.response_type !== 'transfer' && !formData.value.response_content.trim()) {
    toast.error('Please enter a response message')
    return
  }

  // Filter out empty buttons
  const validButtons = formData.value.buttons.filter(b => b.id.trim() && b.title.trim())

  isSubmitting.value = true
  try {
    const data = {
      keywords: formData.value.keywords.split(',').map(k => k.trim()).filter(Boolean),
      match_type: formData.value.match_type,
      response_type: formData.value.response_type,
      response_content: {
        body: formData.value.response_content,
        buttons: validButtons.length > 0 ? validButtons : undefined
      },
      priority: formData.value.priority,
      enabled: formData.value.enabled
    }

    if (editingRule.value) {
      await chatbotService.updateKeyword(editingRule.value.id, data)
      toast.success('Keyword rule updated')
    } else {
      await chatbotService.createKeyword(data)
      toast.success('Keyword rule created')
    }

    isDialogOpen.value = false
    await fetchRules()
  } catch (error: any) {
    toast.error(getErrorMessage(error, 'Failed to save keyword rule'))
  } finally {
    isSubmitting.value = false
  }
}

function openDeleteDialog(rule: KeywordRule) {
  ruleToDelete.value = rule
  deleteDialogOpen.value = true
}

async function confirmDeleteRule() {
  if (!ruleToDelete.value) return

  try {
    await chatbotService.deleteKeyword(ruleToDelete.value.id)
    toast.success('Keyword rule deleted')
    deleteDialogOpen.value = false
    ruleToDelete.value = null
    await fetchRules()
  } catch (error: any) {
    toast.error(getErrorMessage(error, 'Failed to delete keyword rule'))
  }
}

async function toggleRule(rule: KeywordRule) {
  try {
    await chatbotService.updateKeyword(rule.id, { enabled: !rule.enabled })
    rule.enabled = !rule.enabled
    toast.success(rule.enabled ? 'Rule enabled' : 'Rule disabled')
  } catch (error: any) {
    toast.error(getErrorMessage(error, 'Failed to toggle rule'))
  }
}

const emptyDescription = computed(() => {
  if (searchQuery.value) {
    return 'No keyword rules match "' + searchQuery.value + '"'
  }
  return 'Create your first keyword rule to get started.'
})
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader
      title="Keyword Rules"
      :icon="Key"
      icon-gradient="bg-gradient-to-br from-blue-500 to-cyan-600 shadow-blue-500/20"
      back-link="/chatbot"
      :breadcrumbs="[{ label: 'Chatbot', href: '/chatbot' }, { label: 'Keywords' }]"
    >
      <template #actions>
        <Button variant="outline" size="sm" @click="openCreateDialog">
          <Plus class="h-4 w-4 mr-2" />
          Add Rule
        </Button>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto">
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between">
                <div>
                  <CardTitle>Your Keyword Rules</CardTitle>
                  <CardDescription>Configure keywords that trigger automated responses.</CardDescription>
                </div>
                <SearchInput v-model="searchQuery" placeholder="Search keywords..." class="w-64" />
              </div>
            </CardHeader>
            <CardContent>
              <DataTable
                :items="rules"
                :columns="columns"
                :is-loading="isLoading"
                :empty-icon="Key"
                :empty-title="searchQuery ? 'No matching rules' : 'No keyword rules yet'"
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
                  <div class="flex flex-wrap gap-1">
                    <Badge v-for="keyword in rule.keywords.slice(0, 3)" :key="keyword" variant="outline" class="text-xs">
                      {{ keyword }}
                    </Badge>
                    <Badge v-if="rule.keywords.length > 3" variant="outline" class="text-xs">
                      +{{ rule.keywords.length - 3 }}
                    </Badge>
                  </div>
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
                    {{ rule.response_type === 'transfer' ? 'Transfer' : 'Text' }}
                  </Badge>
                </template>
                <template #cell-priority="{ item: rule }">
                  <span class="text-muted-foreground">{{ rule.priority }}</span>
                </template>
                <template #cell-status="{ item: rule }">
                  <div class="flex items-center gap-2">
                    <Switch :checked="rule.enabled" @update:checked="toggleRule(rule)" />
                    <span class="text-sm text-muted-foreground">{{ rule.enabled ? 'Active' : 'Inactive' }}</span>
                  </div>
                </template>
                <template #cell-actions="{ item: rule }">
                  <div class="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click="openEditDialog(rule)">
                      <Pencil class="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="icon" class="h-8 w-8 text-destructive" @click="openDeleteDialog(rule)">
                      <Trash2 class="h-4 w-4" />
                    </Button>
                  </div>
                </template>
                <template #empty-action>
                  <Button v-if="!searchQuery" variant="outline" size="sm" @click="openCreateDialog">
                    <Plus class="h-4 w-4 mr-2" />
                    Add Rule
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
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>{{ editingRule ? 'Edit' : 'Create' }} Keyword Rule</DialogTitle>
          <DialogDescription>
            Configure keywords that trigger automated responses.
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-4 py-4">
          <div class="space-y-2">
            <Label for="keywords">Keywords (comma-separated)</Label>
            <Input
              id="keywords"
              v-model="formData.keywords"
              placeholder="hello, hi, hey"
            />
          </div>
          <div class="space-y-2">
            <Label for="match_type">Match Type</Label>
            <Select v-model="formData.match_type">
              <SelectTrigger>
                <SelectValue placeholder="Select match type" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="contains">Contains</SelectItem>
                <SelectItem value="exact">Exact Match</SelectItem>
                <SelectItem value="regex">Regex</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="space-y-2">
            <Label for="response_type">Response Type</Label>
            <Select v-model="formData.response_type">
              <SelectTrigger>
                <SelectValue placeholder="Select response type" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="text">Text Response</SelectItem>
                <SelectItem value="transfer">Transfer to Agent</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="space-y-2">
            <Label for="response">
              {{ formData.response_type === 'transfer' ? 'Transfer Message (optional)' : 'Response Message' }}
            </Label>
            <Textarea
              id="response"
              v-model="formData.response_content"
              :placeholder="formData.response_type === 'transfer' ? 'Connecting you with a human agent...' : 'Enter the response message...'"
              :rows="3"
            />
            <p v-if="formData.response_type === 'transfer'" class="text-xs text-muted-foreground">
              This message is sent before transferring the conversation to a human agent
            </p>
          </div>

          <!-- Buttons Section (only for text responses) -->
          <div v-if="formData.response_type !== 'transfer'" class="space-y-2">
            <div class="flex items-center justify-between">
              <Label>Buttons (optional, max 10)</Label>
              <Button
                type="button"
                variant="outline"
                size="sm"
                @click="addButton"
                :disabled="formData.buttons.length >= 10"
              >
                <Plus class="h-3 w-3 mr-1" />
                Add Button
              </Button>
            </div>
            <p class="text-xs text-muted-foreground">
              Add buttons for quick replies. 3 or fewer shows as buttons, more than 3 shows as a list.
            </p>
            <div v-if="formData.buttons.length > 0" class="space-y-2 mt-2">
              <div
                v-for="(button, index) in formData.buttons"
                :key="index"
                class="flex items-center gap-2"
              >
                <Input
                  v-model="button.id"
                  placeholder="Button ID"
                  class="flex-1"
                />
                <Input
                  v-model="button.title"
                  placeholder="Button Title"
                  class="flex-1"
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  @click="removeButton(index)"
                >
                  <Trash2 class="h-4 w-4 text-destructive" />
                </Button>
              </div>
            </div>
          </div>

          <div class="space-y-2">
            <Label for="priority">Priority (higher = checked first)</Label>
            <Input
              id="priority"
              v-model.number="formData.priority"
              type="number"
              min="0"
            />
          </div>
          <div class="flex items-center gap-2">
            <Switch
              id="enabled"
              :checked="formData.enabled"
              @update:checked="formData.enabled = $event"
            />
            <Label for="enabled">Enabled</Label>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" size="sm" @click="isDialogOpen = false">Cancel</Button>
          <Button size="sm" @click="saveRule" :disabled="isSubmitting">
            {{ editingRule ? 'Update' : 'Create' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <DeleteConfirmDialog
      v-model:open="deleteDialogOpen"
      title="Delete Keyword Rule"
      description="Are you sure you want to delete this keyword rule? This action cannot be undone."
      @confirm="confirmDeleteRule"
    />
  </div>
</template>
