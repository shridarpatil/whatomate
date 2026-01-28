<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { PageHeader, SearchInput, CrudFormDialog, DeleteConfirmDialog } from '@/components/shared'
import { cannedResponsesService, type CannedResponse } from '@/services/api'
import { toast } from 'vue-sonner'
import { Plus, MessageSquareText, Pencil, Trash2, Loader2, Copy } from 'lucide-vue-next'
import { useCrudState } from '@/composables/useCrudState'
import { useCrudOperations } from '@/composables/useCrudOperations'
import { unwrapListResponse } from '@/lib/api-utils'
import { CANNED_RESPONSE_CATEGORIES, getLabelFromValue } from '@/lib/constants'

interface CannedResponseFormData {
  name: string
  shortcut: string
  content: string
  category: string
  is_active: boolean
}

const defaultFormData: CannedResponseFormData = { name: '', shortcut: '', content: '', category: '', is_active: true }

const {
  items: cannedResponses, isLoading, isSubmitting, isDialogOpen, editingItem: editingResponse, deleteDialogOpen, itemToDelete: responseToDelete,
  formData, searchQuery, openCreateDialog, openEditDialog: baseOpenEditDialog, openDeleteDialog, closeDialog, closeDeleteDialog,
} = useCrudState<CannedResponse, CannedResponseFormData>(defaultFormData)

const selectedCategory = computed({
  get: () => searchQuery.value.startsWith('category:') ? searchQuery.value.replace('category:', '') : 'all',
  set: (val: string) => { searchQuery.value = val === 'all' ? '' : `category:${val}` }
})

const { fetchItems, createItem, updateItem, deleteItem } = useCrudOperations({
  fetchFn: async () => { const response = await cannedResponsesService.list(); return unwrapListResponse<CannedResponse>(response, 'canned_responses') },
  createFn: async (data) => { const response = await cannedResponsesService.create(data); return response.data.data || response.data },
  updateFn: async (id, data) => { const response = await cannedResponsesService.update(id, data); return response.data.data || response.data },
  deleteFn: async (id) => { await cannedResponsesService.delete(id) },
  itemsRef: cannedResponses, loadingRef: isLoading, entityName: 'Canned response'
})

function openEditDialog(response: CannedResponse) {
  baseOpenEditDialog(response, (r) => ({ name: r.name, shortcut: r.shortcut || '', content: r.content, category: r.category || '', is_active: r.is_active }))
}

const filteredResponses = computed(() => {
  let items = cannedResponses.value
  if (selectedCategory.value !== 'all') items = items.filter(r => r.category === selectedCategory.value)
  return items
})

onMounted(() => fetchItems())

async function saveResponse() {
  if (!formData.value.name.trim() || !formData.value.content.trim()) { toast.error('Name and content are required'); return }
  isSubmitting.value = true
  try {
    if (editingResponse.value) await updateItem(editingResponse.value.id, formData.value)
    else await createItem(formData.value)
    closeDialog()
  } catch { /* Error handled by useCrudOperations */ }
  finally { isSubmitting.value = false }
}

async function confirmDelete() {
  if (!responseToDelete.value) return
  try { await deleteItem(responseToDelete.value.id); closeDeleteDialog() }
  catch { /* Error handled by useCrudOperations */ }
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

    <div class="p-4 border-b flex items-center gap-4 flex-wrap">
      <div class="flex items-center gap-2">
        <Label class="text-sm text-muted-foreground">Category:</Label>
        <Select v-model="selectedCategory"><SelectTrigger class="w-[150px]"><SelectValue placeholder="All" /></SelectTrigger><SelectContent><SelectItem value="all">All Categories</SelectItem><SelectItem v-for="cat in CANNED_RESPONSE_CATEGORIES" :key="cat.value" :value="cat.value">{{ cat.label }}</SelectItem></SelectContent></Select>
      </div>
      <SearchInput v-model="searchQuery" placeholder="Search responses..." class="flex-1 max-w-md" />
    </div>

    <div v-if="isLoading" class="flex-1 flex items-center justify-center"><Loader2 class="h-8 w-8 animate-spin text-muted-foreground" /></div>

    <ScrollArea v-else class="flex-1">
      <div class="p-6 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        <Card v-for="response in filteredResponses" :key="response.id" class="flex flex-col">
          <CardHeader class="pb-3">
            <div class="flex items-start justify-between">
              <div class="flex-1 min-w-0">
                <CardTitle class="text-base truncate">{{ response.name }}</CardTitle>
                <div class="flex items-center gap-2 mt-2">
                  <Badge variant="outline" class="text-xs">{{ getCategoryLabel(response.category) }}</Badge>
                  <span v-if="response.shortcut" class="text-xs font-mono text-muted-foreground">/{{ response.shortcut }}</span>
                </div>
              </div>
              <Badge v-if="!response.is_active" variant="secondary" class="ml-2">Inactive</Badge>
            </div>
          </CardHeader>
          <CardContent class="flex-1">
            <p class="text-sm text-muted-foreground line-clamp-3 whitespace-pre-wrap">{{ response.content }}</p>
            <p class="text-xs text-muted-foreground mt-2">Used {{ response.usage_count }} times</p>
          </CardContent>
          <div class="px-6 pb-4 flex items-center gap-1 border-t pt-3">
            <Button variant="ghost" size="sm" @click="copyToClipboard(response.content)"><Copy class="h-4 w-4" /></Button>
            <Button variant="ghost" size="sm" @click="openEditDialog(response)"><Pencil class="h-4 w-4" /></Button>
            <Button variant="ghost" size="sm" @click="openDeleteDialog(response)"><Trash2 class="h-4 w-4 text-destructive" /></Button>
          </div>
        </Card>

        <Card v-if="filteredResponses.length === 0" class="col-span-full">
          <CardContent class="py-12 text-center text-muted-foreground">
            <MessageSquareText class="h-12 w-12 mx-auto mb-4 opacity-50" />
            <p class="text-lg font-medium">No canned responses found</p>
            <p class="text-sm mb-4">Create your first canned response to get started.</p>
            <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />Add Response</Button>
          </CardContent>
        </Card>
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
