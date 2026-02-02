<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { PageHeader, SearchInput, CrudFormDialog, DeleteConfirmDialog, DataTable, type Column } from '@/components/shared'
import { cannedResponsesService, type CannedResponse } from '@/services/api'
import { toast } from 'vue-sonner'
import { Plus, MessageSquareText, Pencil, Trash2, Copy } from 'lucide-vue-next'
import { getErrorMessage } from '@/lib/api-utils'
import { CANNED_RESPONSE_CATEGORIES, getLabelFromValue } from '@/lib/constants'
import { useDebounceFn } from '@vueuse/core'

interface CannedResponseFormData {
  name: string
  shortcut: string
  content: string
  category: string
  is_active: boolean
}

const defaultFormData: CannedResponseFormData = { name: '', shortcut: '', content: '', category: '', is_active: true }

const cannedResponses = ref<CannedResponse[]>([])
const isLoading = ref(false)
const isSubmitting = ref(false)
const isDialogOpen = ref(false)
const editingResponse = ref<CannedResponse | null>(null)
const deleteDialogOpen = ref(false)
const responseToDelete = ref<CannedResponse | null>(null)
const formData = ref<CannedResponseFormData>({ ...defaultFormData })
const searchQuery = ref('')
const selectedCategory = ref('all')

// Pagination state
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 20

const columns: Column<CannedResponse>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'category', label: 'Category', sortable: true },
  { key: 'content', label: 'Content' },
  { key: 'usage_count', label: 'Used', sortable: true },
  { key: 'status', label: 'Status', sortable: true, sortKey: 'is_active' },
  { key: 'actions', label: 'Actions', align: 'right' },
]

const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

async function fetchItems() {
  isLoading.value = true
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
  } catch (error) {
    toast.error(getErrorMessage(error, 'Failed to load canned responses'))
  } finally {
    isLoading.value = false
  }
}

// Debounced search
const debouncedSearch = useDebounceFn(() => {
  currentPage.value = 1
  fetchItems()
}, 300)

watch(searchQuery, () => debouncedSearch())
watch(selectedCategory, () => {
  currentPage.value = 1
  fetchItems()
})

function handlePageChange(page: number) {
  currentPage.value = page
  fetchItems()
}

function openCreateDialog() {
  editingResponse.value = null
  formData.value = { ...defaultFormData }
  isDialogOpen.value = true
}

function openEditDialog(response: CannedResponse) {
  editingResponse.value = response
  formData.value = { name: response.name, shortcut: response.shortcut || '', content: response.content, category: response.category || '', is_active: response.is_active }
  isDialogOpen.value = true
}

function openDeleteDialog(response: CannedResponse) {
  responseToDelete.value = response
  deleteDialogOpen.value = true
}

function closeDialog() {
  isDialogOpen.value = false
  editingResponse.value = null
  formData.value = { ...defaultFormData }
}

function closeDeleteDialog() {
  deleteDialogOpen.value = false
  responseToDelete.value = null
}

onMounted(() => fetchItems())

async function saveResponse() {
  if (!formData.value.name.trim() || !formData.value.content.trim()) { toast.error('Name and content are required'); return }
  isSubmitting.value = true
  try {
    if (editingResponse.value) {
      await cannedResponsesService.update(editingResponse.value.id, formData.value)
      toast.success('Canned response updated')
    } else {
      await cannedResponsesService.create(formData.value)
      toast.success('Canned response created')
    }
    closeDialog()
    await fetchItems()
  } catch (error) {
    toast.error(getErrorMessage(error, 'Failed to save canned response'))
  } finally {
    isSubmitting.value = false
  }
}

async function confirmDelete() {
  if (!responseToDelete.value) return
  try {
    await cannedResponsesService.delete(responseToDelete.value.id)
    toast.success('Canned response deleted')
    closeDeleteDialog()
    await fetchItems()
  } catch (error) {
    toast.error(getErrorMessage(error, 'Failed to delete canned response'))
  }
}

function copyToClipboard(content: string) { navigator.clipboard.writeText(content); toast.success('Copied to clipboard') }
function getCategoryLabel(category: string): string { return getLabelFromValue(CANNED_RESPONSE_CATEGORIES, category) || 'Uncategorized' }
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader title="Canned Responses" subtitle="Pre-defined responses for quick messaging" :icon="MessageSquareText" icon-gradient="bg-gradient-to-br from-teal-500 to-emerald-600 shadow-teal-500/20">
      <template #actions>
        <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />Add Response</Button>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto">
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>Your Canned Responses</CardTitle>
                  <CardDescription>Quick responses for common questions.</CardDescription>
                </div>
                <div class="flex items-center gap-2">
                  <Select v-model="selectedCategory">
                    <SelectTrigger class="w-[150px]"><SelectValue placeholder="All" /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">All Categories</SelectItem>
                      <SelectItem v-for="cat in CANNED_RESPONSE_CATEGORIES" :key="cat.value" :value="cat.value">{{ cat.label }}</SelectItem>
                    </SelectContent>
                  </Select>
                  <SearchInput v-model="searchQuery" placeholder="Search responses..." class="w-64" />
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <DataTable
                :items="cannedResponses"
                :columns="columns"
                :is-loading="isLoading"
                :empty-icon="MessageSquareText"
                empty-title="No canned responses found"
                empty-description="Create your first canned response to get started."
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
                  <div>
                    <span class="font-medium">{{ response.name }}</span>
                    <p v-if="response.shortcut" class="text-xs font-mono text-muted-foreground">/{{ response.shortcut }}</p>
                  </div>
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
                  <Badge v-if="response.is_active" class="bg-emerald-500/20 text-emerald-400 border-transparent text-xs">Active</Badge>
                  <Badge v-else variant="secondary" class="text-xs">Inactive</Badge>
                </template>
                <template #cell-actions="{ item: response }">
                  <div class="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click="copyToClipboard(response.content)" title="Copy">
                      <Copy class="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click="openEditDialog(response)" title="Edit">
                      <Pencil class="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="icon" class="h-8 w-8 text-destructive" @click="openDeleteDialog(response)" title="Delete">
                      <Trash2 class="h-4 w-4" />
                    </Button>
                  </div>
                </template>
                <template #empty-action>
                  <Button variant="outline" size="sm" @click="openCreateDialog">
                    <Plus class="h-4 w-4 mr-2" />Add Response
                  </Button>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <CrudFormDialog v-model:open="isDialogOpen" :is-editing="!!editingResponse" :is-submitting="isSubmitting" edit-title="Edit Canned Response" create-title="Create Canned Response" edit-description="Update the response details." create-description="Add a new quick response." max-width="max-w-lg" @submit="saveResponse">
      <div class="space-y-4">
        <div class="space-y-2"><Label>Name <span class="text-destructive">*</span></Label><Input v-model="formData.name" placeholder="Welcome Message" /></div>
        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-2">
            <Label>Shortcut</Label>
            <div class="relative"><span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">/</span><Input v-model="formData.shortcut" placeholder="welcome" class="pl-7" /></div>
            <p class="text-xs text-muted-foreground">Type /welcome to quickly find</p>
          </div>
          <div class="space-y-2">
            <Label>Category</Label>
            <Select v-model="formData.category"><SelectTrigger><SelectValue placeholder="Select category" /></SelectTrigger><SelectContent><SelectItem v-for="cat in CANNED_RESPONSE_CATEGORIES" :key="cat.value" :value="cat.value">{{ cat.label }}</SelectItem></SelectContent></Select>
          </div>
        </div>
        <div class="space-y-2">
          <Label>Content <span class="text-destructive">*</span></Label>
          <Textarea v-model="formData.content" placeholder="Hello {{contact_name}}! Thank you for reaching out. How can I help you today?" :rows="5" />
          <p class="text-xs text-muted-foreground">Placeholders: <code class="bg-muted px-1 rounded" v-pre>{{contact_name}}</code> for name, <code class="bg-muted px-1 rounded" v-pre>{{phone_number}}</code> for phone</p>
        </div>
        <div v-if="editingResponse" class="flex items-center justify-between"><Label>Active</Label><Switch v-model:checked="formData.is_active" /></div>
      </div>
    </CrudFormDialog>

    <DeleteConfirmDialog v-model:open="deleteDialogOpen" title="Delete Canned Response" :item-name="responseToDelete?.name" @confirm="confirmDelete" />
  </div>
</template>
