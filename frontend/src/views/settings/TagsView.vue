<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { PageHeader, SearchInput, DataTable, CrudFormDialog, DeleteConfirmDialog, IconButton, ErrorState, type Column } from '@/components/shared'
import type { Tag } from '@/services/api'
import { useTagsStore } from '@/stores/tags'
import { useCrudState } from '@/composables/useCrudState'
import { toast } from '@/lib/toast'
import { Plus, Tags, Pencil, Trash2 } from '@/lib/icons'
import { getErrorMessage } from '@/lib/api-utils'
import { TAG_COLORS } from '@/lib/constants'
import { formatDate } from '@/lib/utils'
import { useSearchPagination } from '@/composables/useSearchPagination'

const { t } = useI18n()
const tagsStore = useTagsStore()

interface TagFormData {
  name: string
  color: string
}

const defaultFormData: TagFormData = { name: '', color: 'gray' }

const tags = ref<Tag[]>([])
const isLoading = ref(false)
const isDeleting = ref(false)
const error = ref(false)
const {
  isSubmitting, isDialogOpen, editingItem: editingTag, deleteDialogOpen, itemToDelete: tagToDelete,
  formData, openCreateDialog, openEditDialog: baseOpenEditDialog, openDeleteDialog, closeDialog, closeDeleteDialog,
} = useCrudState<Tag, TagFormData>(defaultFormData)
const { searchQuery, currentPage, totalItems, pageSize, handlePageChange } = useSearchPagination({
  fetchFn: () => fetchTags(),
})

// Sorting state
const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

const columns = computed<Column<Tag>[]>(() => [
  { key: 'name', label: t('tags.tag'), sortable: true },
  { key: 'color', label: t('tags.color'), sortable: true },
  { key: 'created_at', label: t('tags.created'), sortable: true },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

function openEditDialog(tag: Tag) {
  baseOpenEditDialog(tag, (t) => ({ name: t.name, color: t.color || 'gray' }))
}

async function fetchTags() {
  isLoading.value = true
  error.value = false
  try {
    const response = await tagsStore.fetchTags({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    tags.value = response.tags
    totalItems.value = response.total
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedLoad', { resource: t('resources.tags') })))
    error.value = true
  } finally {
    isLoading.value = false
  }
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
      await tagsStore.updateTag(editingTag.value.name, formData.value)
      toast.success(t('common.updatedSuccess', { resource: t('resources.Tag') }))
    } else {
      await tagsStore.createTag(formData.value)
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
  isDeleting.value = true
  try {
    await tagsStore.deleteTag(tagToDelete.value.name)
    toast.success(t('common.deletedSuccess', { resource: t('resources.Tag') }))
    closeDeleteDialog()
    // Refresh from server to keep pagination in sync
    await fetchTags()
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.tag') })))
  } finally {
    isDeleting.value = false
  }
}

function getColorLabel(color: string): string {
  const tagColor = TAG_COLORS.find(c => c.value === color)
  return tagColor?.label || 'Gray'
}
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader :title="$t('tags.title')" :description="$t('tags.subtitle')" :icon="Tags" back-link="/settings">
      <template #actions>
        <button type="button" class="outline small" @click="openCreateDialog"><Plus :size="16" />{{ $t('tags.addTag') }}</button>
      </template>
    </PageHeader>

    <!-- Error State -->
    <ErrorState
      v-if="error && !isLoading"
      :title="$t('common.loadErrorTitle')"
      :description="$t('common.loadErrorDescription')"
      :retry-label="$t('common.retryLoad')"
      class="flex-1"
      @retry="fetchTags"
    />

    <div v-else class="flex-1 scroll-y p-4">
      <article class="card">
        <header class="hstack justify-between flex-wrap gap-4">
          <h3>{{ $t('tags.organizationTags') }}</h3>
          <SearchInput v-model="searchQuery" :placeholder="$t('tags.searchTags') + '...'" />
        </header>
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
            <span class="badge" data-variant="secondary">{{ tag.name }}</span>
          </template>
          <template #cell-color="{ item: tag }">
            <span class="text-light">{{ getColorLabel(tag.color) }}</span>
          </template>
          <template #cell-created_at="{ item: tag }">
            <span class="text-light">{{ formatDate(tag.created_at) }}</span>
          </template>
          <template #cell-actions="{ item: tag }">
            <div class="hstack gap-1 justify-end">
              <IconButton :icon="Pencil" :label="$t('tags.editTag')" @click="openEditDialog(tag)" />
              <IconButton :icon="Trash2" :label="$t('tags.deleteTag')" @click="openDeleteDialog(tag)" />
            </div>
          </template>
          <template #empty-action>
            <button type="button" class="outline small" @click="openCreateDialog">
              <Plus :size="16" />
              {{ $t('tags.addTag') }}
            </button>
          </template>
        </DataTable>
      </article>
    </div>

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
      <div class="vstack gap-4">
        <label data-field>
          {{ $t('tags.name') }} *
          <input v-model="formData.name" :placeholder="$t('tags.namePlaceholder')" maxlength="50" />
          <small data-hint>{{ $t('tags.maxCharacters') }}</small>
        </label>
        <label data-field>
          {{ $t('tags.color') }}
          <select v-model="formData.color">
            <option v-for="color in TAG_COLORS" :key="color.value" :value="color.value">{{ color.label }}</option>
          </select>
        </label>
        <div class="vstack gap-2">
          <small class="text-light">{{ $t('tags.preview') }}</small>
          <div>
            <span class="badge" data-variant="secondary">{{ formData.name || $t('tags.tagPreview') }}</span>
          </div>
        </div>
      </div>
    </CrudFormDialog>

    <DeleteConfirmDialog v-model:open="deleteDialogOpen" :title="$t('tags.deleteTag')" :item-name="tagToDelete?.name" :description="$t('tags.deleteWarning')" :is-submitting="isDeleting" @confirm="confirmDelete" />
  </div>
</template>
