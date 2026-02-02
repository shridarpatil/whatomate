<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { TagBadge } from '@/components/ui/tag-badge'
import { PageHeader, SearchInput, DataTable, CrudFormDialog, DeleteConfirmDialog, type Column } from '@/components/shared'
import { tagsService, type Tag } from '@/services/api'
import { toast } from 'vue-sonner'
import { Plus, Tags, Pencil, Trash2 } from 'lucide-vue-next'
import { getErrorMessage } from '@/lib/api-utils'
import { TAG_COLORS } from '@/lib/constants'
import { formatDate } from '@/lib/utils'
import { useDebounceFn } from '@vueuse/core'

interface TagFormData {
  name: string
  color: string
}

const defaultFormData: TagFormData = { name: '', color: 'gray' }

const tags = ref<Tag[]>([])
const isLoading = ref(false)
const isSubmitting = ref(false)
const isDialogOpen = ref(false)
const editingTag = ref<Tag | null>(null)
const deleteDialogOpen = ref(false)
const tagToDelete = ref<Tag | null>(null)
const formData = ref<TagFormData>({ ...defaultFormData })
const searchQuery = ref('')

// Pagination state
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 20

// Sorting state
const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

const columns: Column<Tag>[] = [
  { key: 'name', label: 'Tag', sortable: true },
  { key: 'color', label: 'Color', sortable: true },
  { key: 'created_at', label: 'Created', sortable: true },
  { key: 'actions', label: 'Actions', align: 'right' },
]

function openCreateDialog() {
  editingTag.value = null
  formData.value = { ...defaultFormData }
  isDialogOpen.value = true
}

function openEditDialog(tag: Tag) {
  editingTag.value = tag
  formData.value = { name: tag.name, color: tag.color || 'gray' }
  isDialogOpen.value = true
}

function openDeleteDialog(tag: Tag) {
  tagToDelete.value = tag
  deleteDialogOpen.value = true
}

function closeDialog() {
  isDialogOpen.value = false
  editingTag.value = null
  formData.value = { ...defaultFormData }
}

function closeDeleteDialog() {
  deleteDialogOpen.value = false
  tagToDelete.value = null
}

async function fetchTags() {
  isLoading.value = true
  try {
    const response = await tagsService.list({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    const data = response.data as any
    // Handle both wrapped and unwrapped responses
    const responseData = data.data || data
    tags.value = responseData.tags || []
    // Use total from response if available, otherwise use items length
    totalItems.value = responseData.total ?? tags.value.length
  } catch (error) {
    toast.error(getErrorMessage(error, 'Failed to load tags'))
  } finally {
    isLoading.value = false
  }
}

// Debounced search to avoid too many API calls
const debouncedSearch = useDebounceFn(() => {
  currentPage.value = 1
  fetchTags()
}, 300)

// Watch search query changes
watch(searchQuery, () => {
  debouncedSearch()
})

function handlePageChange(page: number) {
  currentPage.value = page
  fetchTags()
}

onMounted(() => fetchTags())

async function saveTag() {
  if (!formData.value.name.trim()) {
    toast.error('Name is required')
    return
  }
  isSubmitting.value = true
  try {
    if (editingTag.value) {
      await tagsService.update(editingTag.value.name, formData.value)
      toast.success('Tag updated successfully')
    } else {
      await tagsService.create(formData.value)
      toast.success('Tag created successfully')
    }
    closeDialog()
    // Refresh from server to keep pagination in sync
    await fetchTags()
  } catch (error) {
    toast.error(getErrorMessage(error, 'Failed to save tag'))
  } finally {
    isSubmitting.value = false
  }
}

async function confirmDelete() {
  if (!tagToDelete.value) return
  try {
    await tagsService.delete(tagToDelete.value.name)
    toast.success('Tag deleted')
    closeDeleteDialog()
    // Refresh from server to keep pagination in sync
    await fetchTags()
  } catch (error) {
    toast.error(getErrorMessage(error, 'Failed to delete tag'))
  }
}

function getColorLabel(color: string): string {
  const tagColor = TAG_COLORS.find(c => c.value === color)
  return tagColor?.label || 'Gray'
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader title="Tags" subtitle="Manage organization tags for contacts" :icon="Tags" icon-gradient="bg-gradient-to-br from-indigo-500 to-purple-600 shadow-indigo-500/20" back-link="/settings">
      <template #actions>
        <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />Add Tag</Button>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto">
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>Organization Tags</CardTitle>
                  <CardDescription>Create and manage tags to organize your contacts.</CardDescription>
                </div>
                <SearchInput v-model="searchQuery" placeholder="Search tags..." class="w-64" />
              </div>
            </CardHeader>
            <CardContent>
              <DataTable
                :items="tags"
                :columns="columns"
                :is-loading="isLoading"
                :empty-icon="Tags"
                :empty-title="searchQuery ? 'No matching tags' : 'No tags created yet'"
                :empty-description="searchQuery ? 'No tags match your search.' : 'Create your first tag to start organizing contacts.'"
                v-model:sort-key="sortKey"
                v-model:sort-direction="sortDirection"
                server-pagination
                :current-page="currentPage"
                :total-items="totalItems"
                :page-size="pageSize"
                item-name="tags"
                @page-change="handlePageChange"
              >
                <template #cell-name="{ item: tag }">
                  <TagBadge :color="tag.color">{{ tag.name }}</TagBadge>
                </template>
                <template #cell-color="{ item: tag }">
                  <span class="text-muted-foreground">{{ getColorLabel(tag.color) }}</span>
                </template>
                <template #cell-created_at="{ item: tag }">
                  <span class="text-muted-foreground">{{ formatDate(tag.created_at) }}</span>
                </template>
                <template #cell-actions="{ item: tag }">
                  <div class="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click="openEditDialog(tag)">
                      <Pencil class="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click="openDeleteDialog(tag)">
                      <Trash2 class="h-4 w-4 text-destructive" />
                    </Button>
                  </div>
                </template>
                <template #empty-action>
                  <Button variant="outline" size="sm" @click="openCreateDialog">
                    <Plus class="h-4 w-4 mr-2" />
                    Add Tag
                  </Button>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <CrudFormDialog
      v-model:open="isDialogOpen"
      :is-editing="!!editingTag"
      :is-submitting="isSubmitting"
      edit-title="Edit Tag"
      create-title="Create Tag"
      edit-description="Update the tag details."
      create-description="Add a new tag for contacts."
      max-width="max-w-md"
      @submit="saveTag"
    >
      <div class="space-y-4">
        <div class="space-y-2">
          <Label>Name <span class="text-destructive">*</span></Label>
          <Input v-model="formData.name" placeholder="VIP Customer" maxlength="50" />
          <p class="text-xs text-muted-foreground">Maximum 50 characters</p>
        </div>
        <div class="space-y-2">
          <Label>Color</Label>
          <Select v-model="formData.color" :default-value="formData.color">
            <SelectTrigger>
              <SelectValue placeholder="Select color" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="color in TAG_COLORS" :key="color.value" :value="color.value" :text-value="color.label">
                <div class="flex items-center gap-2">
                  <span :class="['w-3 h-3 rounded-full', color.class.split(' ')[0]]"></span>
                  {{ color.label }}
                </div>
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="pt-2">
          <Label class="text-sm text-muted-foreground">Preview</Label>
          <div class="mt-2">
            <TagBadge :color="formData.color">{{ formData.name || 'Tag Preview' }}</TagBadge>
          </div>
        </div>
      </div>
    </CrudFormDialog>

    <DeleteConfirmDialog v-model:open="deleteDialogOpen" title="Delete Tag" :item-name="tagToDelete?.name" @confirm="confirmDelete">
      <p class="text-sm text-muted-foreground">This will remove the tag from all contacts that have it.</p>
    </DeleteConfirmDialog>
  </div>
</template>
