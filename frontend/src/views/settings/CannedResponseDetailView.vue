<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { cannedResponsesService, type CannedResponse } from '@/services/api'
import { toast } from '@/lib/toast'
import { getErrorMessage } from '@/lib/api-utils'
import { validateWhatsAppButtons } from '@/lib/whatsappButtons'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import MessageButtonsEditor from '@/components/shared/MessageButtonsEditor.vue'
import PreviewButtonGroup from '@/components/chatbot/flow-preview/PreviewButtonGroup.vue'
import type { ButtonConfig } from '@/types/flow-preview'
import Modal from '@/components/oat/Modal.vue'
import { MessageSquareText, Trash2, Save } from '@/lib/icons'
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

// The button combination rules live in @/lib/whatsappButtons so the chatbot
// greeting/fallback editor enforces exactly the same set.
const buttonsValidationError = computed(() => validateWhatsAppButtons(form.value.buttons, t))

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
      back-link="/settings/canned-responses"
      :breadcrumbs="breadcrumbs"
      :is-loading="isLoading"
      :is-not-found="isNotFound"
      :not-found-title="$t('cannedResponses.noResponsesFound')"
    >
      <template #actions>
        <div class="hstack gap-2">
          <button v-if="canWrite && (hasChanges || isNew)" type="button" class="small" :disabled="isSaving || !canSave" :aria-busy="isSaving ? 'true' : undefined" data-spinner="small" @click="save">
            <Save v-if="!isSaving" :size="16" />
            {{ isSaving ? $t('common.saving', 'Saving...') : $t('common.save') }}
          </button>
          <button v-if="!isNew && canDelete" type="button" class="small" data-variant="danger" @click="deleteDialogOpen = true">
            <Trash2 :size="16" />
            {{ $t('common.delete') }}
          </button>
        </div>
      </template>

      <!-- Details Card -->
      <article class="card">
        <header class="hstack justify-between">
          <h3>{{ $t('teams.details', 'Details') }}</h3>
          <span v-if="!isNew" class="badge" :data-variant="form.is_active ? 'success' : 'secondary'">
            {{ form.is_active ? $t('common.active') : $t('common.inactive') }}
          </span>
        </header>
        <div class="vstack gap-4">
          <label data-field>
            {{ $t('cannedResponses.name') }} *
            <input v-model="form.name" placeholder="Welcome Message" :disabled="!canWrite" />
          </label>
          <div class="row">
            <div class="col-6">
              <label data-field>
                {{ $t('cannedResponses.shortcut') }}
                <input v-model="form.shortcut" placeholder="welcome" :disabled="!canWrite" />
                <small data-hint>{{ $t('cannedResponses.shortcutHint') }}</small>
              </label>
            </div>
            <div class="col-6">
              <label data-field>
                {{ $t('cannedResponses.category') }}
                <select v-model="form.category" :disabled="!canWrite">
                  <option value="">{{ $t('cannedResponses.category') }}</option>
                  <option v-for="cat in CANNED_RESPONSE_CATEGORIES" :key="cat.value" :value="cat.value">{{ cat.label }}</option>
                </select>
              </label>
            </div>
          </div>
          <label data-field>
            {{ $t('cannedResponses.content') }} *
            <textarea v-model="form.content" :placeholder="$t('cannedResponses.contentPlaceholder')" rows="6" :disabled="!canWrite" />
            <small data-hint>{{ $t('cannedResponses.placeholderHint') }}</small>
          </label>
          <template v-if="!isNew">
            <hr>
            <label data-field>
              <span class="hstack justify-between">
                <span>{{ $t('common.active') }}</span>
                <input v-model="form.is_active" type="checkbox" role="switch" :disabled="!canWrite" />
              </span>
            </label>
          </template>
        </div>
      </article>

      <!-- Buttons -->
      <article class="card">
        <header>
          <h3>{{ $t('cannedResponses.buttons', 'Buttons') }}</h3>
        </header>
        <div class="vstack gap-4">
          <MessageButtonsEditor
            :buttons="form.buttons"
            :allowed-types="['reply', 'url', 'voice_call', 'flow']"
            :disabled="!canWrite"
            @update:buttons="form.buttons = $event"
          />

          <div v-if="buttonsValidationError" role="alert" data-variant="danger">
            {{ buttonsValidationError }}
          </div>

          <!-- WhatsApp-style preview -->
          <template v-if="form.buttons.length > 0">
            <hr>
            <small class="text-light">{{ $t('common.preview', 'Preview') }}</small>
            <div class="card vstack gap-2">
              <template v-if="form.content">
                <p v-for="(line, idx) in form.content.split('\n')" :key="idx">{{ line }}</p>
              </template>
              <PreviewButtonGroup :buttons="form.buttons" disabled />
            </div>
          </template>
        </div>
      </article>

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
    <Modal v-model:open="deleteDialogOpen" alert :title="$t('cannedResponses.deleteTitle')">
      <p>{{ $t('teams.deleteConfirm', 'Are you sure? This action cannot be undone.') }}</p>
      <template #footer>
        <button type="button" class="outline" @click="deleteDialogOpen = false">{{ $t('common.cancel') }}</button>
        <button type="button" data-variant="danger" @click="deleteResponse">{{ $t('common.delete') }}</button>
      </template>
    </Modal>

    <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
