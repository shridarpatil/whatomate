<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
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

const { t } = useI18n()

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

const columns = computed<Column<Tag>[]>(() => [
  { key: 'name', label: t('tags.tag'), sortable: true },
  { key: 'color', label: t('tags.color'), sortable: true },
  { key: 'created_at', label: t('tags.created'), sortable: true },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

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
    toast.error(getErrorMessage(error, t('common.failedLoad', { resource: t('resources.tags') })))
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
    toast.error(t('tags.nameRequired'))
    return
  }
  isSubmitting.value = true
  try {
    if (editingTag.value) {
      await tagsService.update(editingTag.value.name, formData.value)
      toast.success(t('common.updatedSuccess', { resource: t('resources.Tag') }))
    } else {
      await tagsService.create(formData.value)
      toast.success(t('common.createdSuccess', { resource: t('resources.Tag') }))
    }
    closeDialog()
    // Refresh from server to keep pagination in sync
    await fetchTags()
  } catch (error) {
    toast.error(getErrorMessage(error, t('common.failedSave', { resource: t('resources.tag') })))
  } finally {
    isSubmitting.value = false
  }
}

async function confirmDelete() {
  if (!tagToDelete.value) return
  try {
    await tagsService.delete(tagToDelete.value.name)
    toast.success(t('common.deletedSuccess', { resource: t('resources.Tag') }))
    closeDeleteDialog()
    // Refresh from server to keep pagination in sync
    await fetchTags()
  } catch (error) {
    toast.error(getErrorMessage(error, t('common.failedDelete', { resource: t('resources.tag') })))
  }
}

function getColorLabel(color: string): string {
  const tagColor = TAG_COLORS.find(c => c.value === color)
  return tagColor?.label || 'Gray'
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('tags.title')" :subtitle="$t('tags.subtitle')" :icon="Tags" icon-gradient="bg-gradient-to-br from-indigo-500 to-purple-600 shadow-indigo-500/20" back-link="/settings">
      <template #actions>
        <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />{{ $t('tags.addTag') }}</Button>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto">
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>{{ $t('tags.organizationTags') }}</CardTitle>
                  <CardDescription>{{ $t('tags.organizationTagsDesc') }}</CardDescription>
                </div>
                <SearchInput v-model="searchQuery" :placeholder="$t('tags.searchTags') + '...'" class="w-64" />
              </div>
            </CardHeader>
            <CardContent>
              <DataTable
                :items="tags"
                :columns="columns"
                :is-loading="isLoading"
                :empty-icon="Tags"
                :empty-title="searchQuery ? $t('tags.noMatchingTags') : $t('tags.noTagsYet')"
                :empty-description="searchQuery ? $t('tags.noMatchingTagsDesc') : $t('tags.noTagsYetDesc')"
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
                    {{ $t('tags.addTag') }}
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
      :edit-title="$t('tags.editTagTitle')"
      :create-title="$t('tags.createTagTitle')"
      :edit-description="$t('tags.editTagDesc')"
      :create-description="$t('tags.createTagDesc')"
      max-width="max-w-md"
      @submit="saveTag"
    >
      <div class="space-y-4">
        <div class="space-y-2">
          <Label>{{ $t('tags.name') }} <span class="text-destructive">*</span></Label>
          <Input v-model="formData.name" :placeholder="$t('tags.namePlaceholder')" maxlength="50" />
          <p class="text-xs text-muted-foreground">{{ $t('tags.maxCharacters') }}</p>
        </div>
        <div class="space-y-2">
          <Label>{{ $t('tags.color') }}</Label>
          <Select v-model="formData.color" :default-value="formData.color">
            <SelectTrigger>
              <SelectValue :placeholder="$t('tags.selectColor')" />
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
          <Label class="text-sm text-muted-foreground">{{ $t('tags.preview') }}</Label>
          <div class="mt-2">
            <TagBadge :color="formData.color">{{ formData.name || $t('tags.tagPreview') }}</TagBadge>
          </div>
        </div>
      </div>
    </CrudFormDialog>

    <DeleteConfirmDialog v-model:open="deleteDialogOpen" :title="$t('tags.deleteTag')" :item-name="tagToDelete?.name" @confirm="confirmDelete">
      <p class="text-sm text-muted-foreground">{{ $t('tags.deleteWarning') }}</p>
    </DeleteConfirmDialog>
  </div>
</template>
