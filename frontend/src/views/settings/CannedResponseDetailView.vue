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
import MessageButtonsEditor from '@/components/shared/MessageButtonsEditor.vue'
import PreviewButtonGroup from '@/components/chatbot/flow-preview/PreviewButtonGroup.vue'
import type { ButtonConfig } from '@/types/flow-preview'
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
  buttons: [] as ButtonConfig[],
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
    buttons: (response.value.buttons || []).map(b => ({ ...b })),
  }
}

watch(form, () => {
  hasChanges.value = true
}, { deep: true })

// Validate the button combination against WhatsApp Cloud API's free-form
// interactive-message rules. Sendable shapes:
//   - 0 buttons
//   - 1–10 reply buttons (1–3 send as reply buttons; 4–10 send as a list)
//   - exactly 1 URL button (cta_url)
//   - exactly 1 voice_call button (interactive.type:"voice_call", standalone)
// Phone buttons and multi-URL / mixed combos can't be carried by any
// free-form interactive message and would otherwise silently fall back to
// plain text on send, so we block save instead of confusing the agent.
// Keep the rules in sync with validateCannedResponseButtons in the Go
// handler — it duplicates these checks for non-UI callers.
const buttonsValidationError = computed<string | null>(() => {
  const list = form.value.buttons
  if (!list.length) return null
  const reply = list.filter(b => !b.type || b.type === 'reply')
  const url = list.filter(b => b.type === 'url')
  const phone = list.filter(b => b.type === 'phone')
  const voiceCall = list.filter(b => b.type === 'voice_call')

  if (voiceCall.length > 1) {
    return t(
      'cannedResponses.errorMultiVoiceCall',
      'Only one Call button is allowed per message.',
    )
  }
  if (voiceCall.length > 0 && list.length > voiceCall.length) {
    return t(
      'cannedResponses.errorVoiceCallExclusive',
      'A Call button cannot be combined with other button types — remove the other buttons or the Call button.',
    )
  }
  if (voiceCall.length === 1) {
    const v = voiceCall[0]
    if (!v.title?.trim()) {
      return t(
        'cannedResponses.errorVoiceCallTitle',
        'The Call button needs a label (shown on the button face).',
      )
    }
    const ttl = v.ttl_minutes ?? 0
    if (ttl < 0 || ttl > 60) {
      return t(
        'cannedResponses.errorVoiceCallTtl',
        'Call button expiry must be between 1 and 60 minutes.',
      )
    }
  }
  if (phone.length > 0) {
    return t(
      'cannedResponses.errorPhoneUnsupported',
      'Phone buttons cannot be sent in free-form WhatsApp messages — only in approved templates. Remove the phone button or convert it to a URL.',
    )
  }
  if (url.length > 1) {
    return t(
      'cannedResponses.errorMultiUrl',
      'WhatsApp allows only one URL button per message. Remove the extra URL button.',
    )
  }
  if (reply.length > 0 && url.length > 0) {
    return t(
      'cannedResponses.errorMixedButtons',
      'Reply and URL buttons cannot be mixed in a single WhatsApp message.',
    )
  }
  if (reply.length > 10) {
    return t(
      'cannedResponses.errorTooManyReply',
      'WhatsApp allows at most 10 reply buttons.',
    )
  }
  return null
})

const canSave = computed(() => !buttonsValidationError.value)

async function save() {
  if (!form.value.name.trim() || !form.value.content.trim()) {
    toast.error(t('cannedResponses.nameContentRequired'))
    return
  }
  if (buttonsValidationError.value) {
    toast.error(buttonsValidationError.value)
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
        buttons: form.value.buttons,
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
        buttons: form.value.buttons,
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
          <Button v-if="canWrite && (hasChanges || isNew)" size="sm" @click="save" :disabled="isSaving || !canSave">
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

      <!-- Buttons -->
      <Card>
        <CardHeader class="pb-3">
          <CardTitle class="text-sm font-medium">{{ $t('cannedResponses.buttons', 'Buttons') }}</CardTitle>
        </CardHeader>
        <CardContent class="space-y-4">
          <MessageButtonsEditor
            :buttons="form.buttons"
            :allowed-types="['reply', 'url', 'voice_call']"
            :disabled="!canWrite"
            @update:buttons="form.buttons = $event"
          />

          <p
            v-if="buttonsValidationError"
            class="text-xs text-destructive rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2"
          >
            {{ buttonsValidationError }}
          </p>

          <!-- WhatsApp-style preview -->
          <div v-if="form.buttons.length > 0" class="border-t pt-3">
            <p class="text-[11px] text-muted-foreground mb-2">{{ $t('common.preview', 'Preview') }}</p>
            <div class="max-w-sm bg-[#0a141a] dark:bg-[#0a141a] rounded-lg p-3 space-y-1">
              <p v-if="form.content" class="text-sm text-white whitespace-pre-wrap">{{ form.content }}</p>
              <PreviewButtonGroup :buttons="form.buttons" disabled />
            </div>
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
