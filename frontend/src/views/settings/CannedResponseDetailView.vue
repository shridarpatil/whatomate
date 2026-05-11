<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { cannedResponsesService, type CannedResponse } from '@/services/api'
import { toast } from 'vue-sonner'
import { getErrorMessage } from '@/lib/api-utils'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { MessageSquareText, Trash2, Save } from 'lucide-vue-next'
import { CANNED_RESPONSE_CATEGORIES } from '@/lib/constants'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const authStore = useAuthStore()

const isNew = computed(() => route.params.id === 'new')
const responseId = computed(() => route.params.id as string)
const response = ref<CannedResponse | null>(null)
const isLoading = ref(true)
const isNotFound = ref(false)
const isSaving = ref(false)
const hasChanges = ref(false)
const deleteDialogOpen = ref(false)

const { showLeaveDialog, confirmLeave, cancelLeave } = useUnsavedChangesGuard(hasChanges)

const canWrite = computed(() => authStore.hasPermission('canned_responses', 'write'))
const canDelete = computed(() => authStore.hasPermission('canned_responses', 'delete'))

const form = ref({
  name: '',
  shortcut: '',
  content: '',
  category: '',
  is_active: true,
})

const breadcrumbs = computed(() => [
  { label: t('nav.settings'), href: '/settings' },
  { label: t('cannedResponses.title'), href: '/settings/canned-responses' },
  { label: isNew.value ? t('cannedResponses.createTitle') : response.value?.name || '' },
])

const pageTitle = computed(() =>
  isNew.value ? t('cannedResponses.createTitle') : response.value?.name || ''
)

async function loadResponse() {
  if (isNew.value) {
    isLoading.value = false
    nextTick(() => { hasChanges.value = false })
    return
  }
  isLoading.value = true
  isNotFound.value = false
  try {
    const res = await cannedResponsesService.get(responseId.value)
    const data = (res.data as any).data || res.data
    response.value = data
    syncForm()
    nextTick(() => { hasChanges.value = false })
  } catch {
    isNotFound.value = true
  } finally {
    isLoading.value = false
  }
}

function syncForm() {
  if (!response.value) return
  form.value = {
    name: response.value.name,
    shortcut: response.value.shortcut || '',
    content: response.value.content,
    category: response.value.category || '',
    is_active: response.value.is_active,
  }
}

watch(form, () => {
  hasChanges.value = true
}, { deep: true })

async function save() {
  if (!form.value.name.trim() || !form.value.content.trim()) {
    toast.error(t('cannedResponses.nameContentRequired'))
    return
  }
  isSaving.value = true
  try {
    if (isNew.value) {
      const res = await cannedResponsesService.create({
        name: form.value.name,
        shortcut: form.value.shortcut || undefined,
        content: form.value.content,
        category: form.value.category || undefined,
      })
      toast.success(t('common.createdSuccess', { resource: t('resources.CannedResponse') }))
      const created = (res.data as any).data || res.data
      // Set the response locally so the (reused) component switches to edit
      // mode without re-fetching. responseId watcher below guards against a
      // duplicate load when the route param flips from "new" to the new UUID.
      response.value = created
      await nextTick()
      hasChanges.value = false
      router.replace(`/settings/canned-responses/${created.id}`)
    } else if (response.value) {
      await cannedResponsesService.update(response.value.id, {
        name: form.value.name,
        shortcut: form.value.shortcut,
        content: form.value.content,
        category: form.value.category,
        is_active: form.value.is_active,
      })
      toast.success(t('common.updatedSuccess', { resource: t('resources.CannedResponse') }))
      await loadResponse()
    }
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedSave', { resource: t('resources.cannedResponse') })))
  } finally {
    isSaving.value = false
  }
}

async function deleteResponse() {
  if (!response.value) return
  try {
    await cannedResponsesService.delete(response.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.CannedResponse') }))
    hasChanges.value = false
    router.push('/settings/canned-responses')
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.cannedResponse') })))
  }
  deleteDialogOpen.value = false
}

// Same component handles /new and /:id, so route-param changes don't remount.
// Reload when the id genuinely changes — but skip when we just set the response
// locally (e.g. immediately after create) to avoid an unnecessary fetch and the
// race it causes with hasChanges reset.
watch(responseId, async (newId, oldId) => {
  if (!newId || newId === 'new' || newId === oldId) return
  if (response.value?.id === newId) return
  await loadResponse()
})

onMounted(() => { loadResponse() })
</script>

<template>
  <div class="h-full">
    <DetailPageLayout
      :title="pageTitle"
      :icon="MessageSquareText"
      icon-gradient="bg-gradient-to-br from-teal-500 to-emerald-600 shadow-teal-500/20"
      back-link="/settings/canned-responses"
      :breadcrumbs="breadcrumbs"
      :is-loading="isLoading"
      :is-not-found="isNotFound"
      :not-found-title="$t('cannedResponses.noResponsesFound')"
    >
      <template #actions>
        <div class="flex items-center gap-2">
          <Button v-if="canWrite && (hasChanges || isNew)" size="sm" @click="save" :disabled="isSaving">
            <Save class="h-4 w-4 mr-1" />
            {{ isSaving ? $t('common.saving', 'Saving...') : $t('common.save') }}
          </Button>
          <Button
            v-if="!isNew && canDelete"
            variant="destructive"
            size="sm"
            @click="deleteDialogOpen = true"
          >
            <Trash2 class="h-4 w-4 mr-1" />
            {{ $t('common.delete') }}
          </Button>
        </div>
      </template>

      <!-- Details Card -->
      <Card>
        <CardHeader class="pb-3">
          <div class="flex items-center justify-between">
            <CardTitle class="text-sm font-medium">{{ $t('teams.details', 'Details') }}</CardTitle>
            <Badge v-if="!isNew" :variant="form.is_active ? 'default' : 'secondary'">
              {{ form.is_active ? $t('common.active') : $t('common.inactive') }}
            </Badge>
          </div>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('cannedResponses.name') }} <span class="text-destructive">*</span></Label>
            <Input v-model="form.name" placeholder="Welcome Message" :disabled="!canWrite" />
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-1.5">
              <Label class="text-xs">{{ $t('cannedResponses.shortcut') }}</Label>
              <div class="relative">
                <span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">/</span>
                <Input v-model="form.shortcut" placeholder="welcome" class="pl-7" :disabled="!canWrite" />
              </div>
              <p class="text-[11px] text-muted-foreground">{{ $t('cannedResponses.shortcutHint') }}</p>
            </div>
            <div class="space-y-1.5">
              <Label class="text-xs">{{ $t('cannedResponses.category') }}</Label>
              <Select v-model="form.category" :disabled="!canWrite">
                <SelectTrigger>
                  <SelectValue :placeholder="$t('cannedResponses.category')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="cat in CANNED_RESPONSE_CATEGORIES" :key="cat.value" :value="cat.value">
                    {{ cat.label }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('cannedResponses.content') }} <span class="text-destructive">*</span></Label>
            <Textarea v-model="form.content" :placeholder="$t('cannedResponses.contentPlaceholder')" :rows="6" :disabled="!canWrite" />
            <p class="text-[11px] text-muted-foreground">{{ $t('cannedResponses.placeholderHint') }}</p>
          </div>
          <div v-if="!isNew" class="flex items-center justify-between border-t pt-4">
            <Label class="text-xs font-normal cursor-pointer">{{ $t('common.active') }}</Label>
            <Switch
              :checked="form.is_active"
              @update:checked="form.is_active = $event"
              :disabled="!canWrite"
            />
          </div>
        </CardContent>
      </Card>

      <!-- Activity Log -->
      <AuditLogPanel
        v-if="!isNew && response"
        resource-type="canned_response"
        :resource-id="response.id"
      />

      <!-- Sidebar -->
      <template #sidebar>
        <MetadataPanel
          :created-at="response?.created_at"
          :updated-at="response?.updated_at"
        />
      </template>
    </DetailPageLayout>

    <!-- Delete Confirmation -->
    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('cannedResponses.deleteTitle') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('teams.deleteConfirm', 'Are you sure? This action cannot be undone.') }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('common.cancel') }}</AlertDialogCancel>
          <AlertDialogAction @click="deleteResponse">{{ $t('common.delete') }}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
