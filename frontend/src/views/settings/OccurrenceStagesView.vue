<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { PageHeader, DataTable, CrudFormDialog, DeleteConfirmDialog, IconButton, ErrorState, type Column } from '@/components/shared'
import { occurrencesService, type OccurrenceStage } from '@/services/api'
import { useOccurrencesStore } from '@/stores/occurrences'
import { useCrudState } from '@/composables/useCrudState'
import { toast } from 'vue-sonner'
import { Plus, ClipboardList, Pencil, Trash2 } from 'lucide-vue-next'
import { getErrorMessage } from '@/lib/api-utils'

const { t } = useI18n()
const store = useOccurrencesStore()

interface StageFormData {
  name: string
  color: string
  position: number
  is_initial: boolean
  is_closing: boolean
}

const defaultFormData: StageFormData = { name: '', color: '#6b7280', position: 0, is_initial: false, is_closing: false }

const isLoading = ref(false)
const isDeleting = ref(false)
const error = ref(false)

const {
  isSubmitting, isDialogOpen, editingItem: editingStage, deleteDialogOpen, itemToDelete: stageToDelete,
  formData, openCreateDialog: baseOpenCreateDialog, openEditDialog: baseOpenEditDialog, openDeleteDialog, closeDialog, closeDeleteDialog,
} = useCrudState<OccurrenceStage, StageFormData>(defaultFormData)

const columns = computed<Column<OccurrenceStage>[]>(() => [
  { key: 'color', label: t('occurrences.columnColor'), width: 'w-[70px]' },
  { key: 'name', label: t('occurrences.stageName') },
  { key: 'position', label: t('occurrences.columnPosition'), width: 'w-[100px]' },
  { key: 'initial', label: t('occurrences.columnInitial'), align: 'center' },
  { key: 'closing', label: t('occurrences.columnClosing'), align: 'center' },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

function openCreateDialog() {
  baseOpenCreateDialog()
  formData.value.position = store.stages.length
}

function openEditDialog(stage: OccurrenceStage) {
  baseOpenEditDialog(stage, (s) => ({
    name: s.name,
    color: s.color || '#6b7280',
    position: s.position,
    is_initial: s.is_initial,
    is_closing: s.is_closing,
  }))
}

async function fetchStages() {
  isLoading.value = true
  error.value = false
  try {
    await store.fetchStages()
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedLoad', { resource: t('resources.stages') })))
    error.value = true
  } finally {
    isLoading.value = false
  }
}

onMounted(() => fetchStages())

async function saveStage() {
  if (!formData.value.name.trim()) {
    toast.error(t('occurrences.stageNameRequired'))
    return
  }
  isSubmitting.value = true
  try {
    const payload = { ...formData.value, name: formData.value.name.trim() }
    if (editingStage.value) {
      await occurrencesService.updateStage(editingStage.value.id, payload)
      toast.success(t('common.updatedSuccess', { resource: t('resources.Stage') }))
    } else {
      await occurrencesService.createStage(payload)
      toast.success(t('common.createdSuccess', { resource: t('resources.Stage') }))
    }
    closeDialog()
    await fetchStages()
  } catch (e) {
    // 409s here carry the backend's exact integrity-rule message (duplicate
    // name, or — on edit — trying to unmark the only initial/closing stage).
    toast.error(getErrorMessage(e, t('common.failedSave', { resource: t('resources.stage') })))
  } finally {
    isSubmitting.value = false
  }
}

async function confirmDelete() {
  if (!stageToDelete.value) return
  isDeleting.value = true
  try {
    await occurrencesService.deleteStage(stageToDelete.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.Stage') }))
    closeDeleteDialog()
    await fetchStages()
  } catch (e) {
    // 409s here explain exactly why: in use by occurrences, the initial
    // stage, or the last closing stage.
    toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.stage') })))
  } finally {
    isDeleting.value = false
  }
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('occurrences.stagesTitle')" :description="$t('occurrences.stagesSubtitle')" :icon="ClipboardList" icon-gradient="bg-gradient-to-br from-violet-500 to-purple-600 shadow-violet-500/20" back-link="/settings">
      <template #actions>
        <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />{{ $t('occurrences.addStage') }}</Button>
      </template>
    </PageHeader>

    <ErrorState
      v-if="error && !isLoading"
      :title="$t('common.loadErrorTitle')"
      :description="$t('common.loadErrorDescription')"
      :retry-label="$t('common.retryLoad')"
      class="flex-1"
      @retry="fetchStages"
    />

    <ScrollArea v-else orientation="vertical" class="flex-1">
      <div class="p-6">
        <div class="max-w-4xl mx-auto">
          <Card>
            <CardHeader>
              <CardTitle>{{ $t('occurrences.stagesCardTitle') }}</CardTitle>
              <CardDescription>{{ $t('occurrences.stagesCardDesc') }}</CardDescription>
            </CardHeader>
            <CardContent>
              <DataTable
                :items="store.stages"
                :columns="columns"
                :is-loading="isLoading"
                :empty-icon="ClipboardList"
                :empty-title="$t('occurrences.noStagesYet')"
                :empty-description="$t('occurrences.noStagesYetDesc')"
                item-name="stages"
              >
                <template #cell-color="{ item: stage }">
                  <span class="inline-block h-4 w-4 rounded-full border border-white/20 light:border-gray-300" :style="{ backgroundColor: stage.color }" />
                </template>
                <template #cell-name="{ item: stage }">
                  <span class="font-medium">{{ stage.name }}</span>
                </template>
                <template #cell-position="{ item: stage }">
                  <span class="text-muted-foreground">{{ stage.position }}</span>
                </template>
                <template #cell-initial="{ item: stage }">
                  <Badge :variant="stage.is_initial ? 'success' : 'secondary'">{{ stage.is_initial ? $t('common.yes') : $t('common.no') }}</Badge>
                </template>
                <template #cell-closing="{ item: stage }">
                  <Badge :variant="stage.is_closing ? 'info' : 'secondary'">{{ stage.is_closing ? $t('common.yes') : $t('common.no') }}</Badge>
                </template>
                <template #cell-actions="{ item: stage }">
                  <div class="flex items-center justify-end gap-1">
                    <IconButton :icon="Pencil" :label="$t('occurrences.editStage')" class="h-8 w-8" @click="openEditDialog(stage)" />
                    <IconButton :label="$t('occurrences.deleteStageTitle')" class="h-8 w-8" @click="openDeleteDialog(stage)">
                      <Trash2 class="h-4 w-4 text-destructive" />
                    </IconButton>
                  </div>
                </template>
                <template #empty-action>
                  <Button variant="outline" size="sm" @click="openCreateDialog">
                    <Plus class="h-4 w-4 mr-2" />
                    {{ $t('occurrences.addStage') }}
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
      :is-editing="!!editingStage"
      :is-submitting="isSubmitting"
      :edit-title="$t('occurrences.editStageTitle')"
      :create-title="$t('occurrences.createStageTitle')"
      :edit-description="$t('occurrences.editStageDesc')"
      :create-description="$t('occurrences.createStageDesc')"
      max-width="max-w-md"
      @submit="saveStage"
    >
      <div class="space-y-4">
        <div class="space-y-2">
          <Label>{{ $t('occurrences.stageName') }} <span class="text-destructive">*</span></Label>
          <Input v-model="formData.name" :placeholder="$t('occurrences.stageNamePlaceholder')" maxlength="100" />
        </div>
        <div class="space-y-2">
          <Label>{{ $t('occurrences.stageColor') }}</Label>
          <div class="flex items-center gap-2">
            <Input type="color" v-model="formData.color" class="h-10 w-16 p-1 cursor-pointer" />
            <Input v-model="formData.color" class="flex-1 font-mono" maxlength="20" />
          </div>
        </div>
        <div class="space-y-2">
          <Label>{{ $t('occurrences.stagePosition') }}</Label>
          <Input type="number" v-model.number="formData.position" :min="0" />
        </div>
        <div class="flex items-center justify-between gap-4 pt-2">
          <div>
            <Label>{{ $t('occurrences.isInitial') }}</Label>
            <p class="text-xs text-muted-foreground">{{ $t('occurrences.isInitialHint') }}</p>
          </div>
          <Switch :checked="formData.is_initial" @update:checked="formData.is_initial = $event" />
        </div>
        <div class="flex items-center justify-between gap-4">
          <div>
            <Label>{{ $t('occurrences.isClosing') }}</Label>
            <p class="text-xs text-muted-foreground">{{ $t('occurrences.isClosingHint') }}</p>
          </div>
          <Switch :checked="formData.is_closing" @update:checked="formData.is_closing = $event" />
        </div>
      </div>
    </CrudFormDialog>

    <DeleteConfirmDialog v-model:open="deleteDialogOpen" :title="$t('occurrences.deleteStageTitle')" :item-name="stageToDelete?.name" :is-submitting="isDeleting" @confirm="confirmDelete">
      <p class="text-sm text-muted-foreground">{{ $t('occurrences.deleteStageWarning') }}</p>
    </DeleteConfirmDialog>
  </div>
</template>
