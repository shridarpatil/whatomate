<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { chatbotService } from '@/services/api'
import { toast } from '@/lib/toast'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import { IconButton, DeleteConfirmDialog } from '@/components/shared'
import {
  Key,
  Trash2,
  Save,
  Plus,
} from '@/lib/icons'
import { getErrorMessage } from '@/lib/api-utils'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const authStore = useAuthStore()

const keywordId = computed(() => route.params.id as string)
const isNew = computed(() => keywordId.value === 'new')
const keyword = ref<any>(null)
const isLoading = ref(true)
const isNotFound = ref(false)
const isSaving = ref(false)
const hasChanges = ref(false)
const deleteDialogOpen = ref(false)

const { showLeaveDialog, confirmLeave, cancelLeave } = useUnsavedChangesGuard(hasChanges)

const canWrite = computed(() => authStore.hasPermission('chatbot.keywords', 'write'))
const canDelete = computed(() => authStore.hasPermission('chatbot.keywords', 'delete'))

interface ButtonItem {
  id: string
  title: string
}

const form = ref({
  keywords: '',
  match_type: 'contains' as 'contains' | 'exact' | 'regex',
  response_type: 'text' as 'text' | 'transfer',
  response_content: '',
  buttons: [] as ButtonItem[],
  priority: 0,
  enabled: true,
})

const breadcrumbs = computed(() => [
  { label: t('nav.chatbot', 'Chatbot'), href: '/chatbot' },
  { label: t('nav.keywords', 'Keywords'), href: '/chatbot/keywords' },
  { label: isNew.value ? t('keywords.newKeyword', 'New Keyword') : (keyword.value?.name || form.value.keywords.split(',')[0]?.trim() || '') },
])

async function loadKeyword() {
  isLoading.value = true
  isNotFound.value = false
  try {
    const response = await chatbotService.getKeyword(keywordId.value)
    const data = (response.data as any).data?.rule || (response.data as any).data || response.data
    keyword.value = data
    syncForm()
    nextTick(() => { hasChanges.value = false })
  } catch {
    isNotFound.value = true
  } finally {
    isLoading.value = false
  }
}

function syncForm() {
  if (!keyword.value) return
  form.value = {
    keywords: (keyword.value.keywords || []).join(', '),
    match_type: keyword.value.match_type || 'contains',
    response_type: keyword.value.response_type || 'text',
    response_content: keyword.value.response_content?.body || '',
    buttons: [...(keyword.value.response_content?.buttons || [])],
    priority: keyword.value.priority || 0,
    enabled: keyword.value.enabled ?? true,
  }
}

// Track form changes
watch(form, () => {
  hasChanges.value = true
}, { deep: true })

function addButton() {
  if (form.value.buttons.length >= 10) {
    toast.error(t('keywords.maxButtonsError', 'Maximum 10 buttons allowed'))
    return
  }
  form.value.buttons.push({ id: '', title: '' })
}

function removeButton(index: number) {
  form.value.buttons.splice(index, 1)
}

function buildPayload() {
  const validButtons = form.value.buttons.filter(b => b.title.trim())
  return {
    keywords: form.value.keywords.split(',').map(k => k.trim()).filter(Boolean),
    match_type: form.value.match_type,
    response_type: form.value.response_type,
    response_content: {
      body: form.value.response_content,
      buttons: validButtons.length > 0 ? validButtons : undefined,
    },
    priority: form.value.priority,
    enabled: form.value.enabled,
  }
}

async function save() {
  if (!form.value.keywords.trim()) {
    toast.error(t('keywords.enterKeyword', 'Please enter at least one keyword'))
    return
  }

  if (form.value.response_type !== 'transfer' && !form.value.response_content.trim()) {
    toast.error(t('keywords.enterResponse', 'Please enter a response message'))
    return
  }

  isSaving.value = true
  try {
    const data = buildPayload()

    if (isNew.value) {
      const response = await chatbotService.createKeyword(data)
      const created = (response.data as any).data?.rule || (response.data as any).data || response.data
      hasChanges.value = false
      toast.success(t('common.createdSuccess', { resource: t('resources.KeywordRule', 'Keyword Rule') }))
      router.replace(`/chatbot/keywords/${created.id}`)
    } else {
      await chatbotService.updateKeyword(keyword.value!.id, data)
      await loadKeyword()
      hasChanges.value = false
      toast.success(t('common.updatedSuccess', { resource: t('resources.KeywordRule', 'Keyword Rule') }))
    }
  } catch (error: any) {
    toast.error(getErrorMessage(error, isNew.value
      ? t('common.failedSave', { resource: t('resources.keywordRule', 'keyword rule') })
      : t('common.failedSave', { resource: t('resources.keywordRule', 'keyword rule') })
    ))
  } finally {
    isSaving.value = false
  }
}

async function deleteKeyword() {
  if (!keyword.value) return
  try {
    await chatbotService.deleteKeyword(keyword.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.KeywordRule', 'Keyword Rule') }))
    router.push('/chatbot/keywords')
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('common.failedDelete', { resource: t('resources.keywordRule', 'keyword rule') })))
  }
  deleteDialogOpen.value = false
}

onMounted(async () => {
  if (isNew.value) {
    isLoading.value = false
    hasChanges.value = false
  } else {
    await loadKeyword()
  }
})
</script>

<template>
  <div class="h-full">
  <DetailPageLayout
    :title="isNew ? $t('keywords.newKeyword', 'New Keyword Rule') : (keyword?.name || form.keywords.split(',')[0]?.trim() || '')"
    :icon="Key"
    back-link="/chatbot/keywords"
    :breadcrumbs="breadcrumbs"
    :is-loading="isLoading"
    :is-not-found="isNotFound"
    :not-found-title="$t('keywords.notFound', 'Keyword rule not found')"
  >
    <template #actions>
      <div class="hstack gap-2">
        <button v-if="canWrite && (hasChanges || isNew)" type="button" class="small" :disabled="isSaving" :aria-busy="isSaving ? 'true' : undefined" data-spinner="small" @click="save">
          <Save v-if="!isSaving" :size="16" /> {{ isSaving ? $t('common.saving', 'Saving...') : isNew ? $t('common.create') : $t('common.save') }}
        </button>
        <button v-if="canDelete && !isNew" type="button" class="small" data-variant="danger" @click="deleteDialogOpen = true">
          <Trash2 :size="16" /> {{ $t('common.delete') }}
        </button>
      </div>
    </template>

    <!-- Keyword Details Card -->
    <article class="card">
      <header class="hstack justify-between">
        <h3>{{ $t('keywords.details', 'Details') }}</h3>
        <span class="badge" :data-variant="(keyword?.enabled ?? form.enabled) ? 'success' : 'secondary'">
          {{ (keyword?.enabled ?? form.enabled) ? $t('keywords.active', 'Active') : $t('keywords.inactive', 'Inactive') }}
        </span>
      </header>
      <div class="vstack gap-4">
        <label data-field>
          {{ $t('keywords.keywordsLabel', 'Keywords') }} *
          <input
            v-model="form.keywords"
            :placeholder="$t('keywords.keywordsPlaceholder', 'hello, hi, hey')"
            :disabled="!canWrite"
          >
          <small data-hint>{{ $t('keywords.keywordsHint', 'Comma-separated list of keywords') }}</small>
        </label>
        <label data-field>
          {{ $t('keywords.matchTypeLabel', 'Match Type') }}
          <select v-model="form.match_type" :disabled="!canWrite">
            <option value="contains">{{ $t('keywords.contains', 'Contains') }}</option>
            <option value="exact">{{ $t('keywords.exact', 'Exact') }}</option>
            <option value="regex">{{ $t('keywords.regex', 'Regex') }}</option>
          </select>
        </label>
        <label data-field>
          {{ $t('keywords.responseType', 'Response Type') }}
          <select v-model="form.response_type" :disabled="!canWrite">
            <option value="text">{{ $t('keywords.textResponse', 'Text Response') }}</option>
            <option value="transfer">{{ $t('keywords.transferToAgent', 'Transfer to Agent') }}</option>
          </select>
        </label>
        <label data-field>
          {{ form.response_type === 'transfer' ? $t('keywords.transferMessage', 'Transfer Message') : $t('keywords.responseMessage', 'Response Message') }}
          <template v-if="form.response_type !== 'transfer'"> *</template>
          <textarea
            v-model="form.response_content"
            :placeholder="form.response_type === 'transfer' ? $t('keywords.transferPlaceholder', 'Optional message before transfer') + '...' : $t('keywords.responsePlaceholder', 'Enter the response message') + '...'"
            rows="3"
            :disabled="!canWrite"
          />
          <small v-if="form.response_type === 'transfer'" data-hint>
            {{ $t('keywords.transferHint', 'Optional message sent before transferring to a human agent') }}
          </small>
        </label>

        <!-- Buttons Section (only for text responses) -->
        <div v-if="form.response_type === 'text'" class="vstack gap-2">
          <div class="hstack justify-between">
            <strong>{{ $t('keywords.buttonsOptional', 'Buttons (optional)') }}</strong>
            <button
              v-if="canWrite"
              type="button"
              class="outline small"
              :disabled="form.buttons.length >= 10"
              @click="addButton"
            >
              <Plus :size="14" />
              {{ $t('keywords.addButton', 'Add Button') }}
            </button>
          </div>
          <small class="text-light">
            {{ $t('keywords.buttonsHint', 'Add quick-reply buttons to the response message') }}
          </small>
          <div v-if="form.buttons.length > 0" class="vstack gap-2">
            <div
              v-for="(button, index) in form.buttons"
              :key="index"
              class="hstack gap-2"
            >
              <input
                v-model="button.id"
                class="flex-1"
                :placeholder="$t('keywords.buttonId', 'Button ID')"
                :aria-label="$t('keywords.buttonId', 'Button ID')"
                :disabled="!canWrite"
              >
              <input
                v-model="button.title"
                class="flex-1"
                :placeholder="$t('keywords.buttonTitle', 'Button Title')"
                :aria-label="$t('keywords.buttonTitle', 'Button Title')"
                :disabled="!canWrite"
              >
              <IconButton
                v-if="canWrite"
                :icon="Trash2"
                :label="$t('keywords.removeButtonLabel', 'Remove button')"
                @click="removeButton(index)"
              />
            </div>
          </div>
        </div>

        <label data-field>
          {{ $t('keywords.priorityLabel', 'Priority') }}
          <input
            v-model.number="form.priority"
            type="number"
            min="1"
            max="100"
            :disabled="!canWrite"
          >
        </label>
        <label class="hstack gap-2">
          <input v-model="form.enabled" type="checkbox" role="switch" :disabled="!canWrite">
          {{ $t('keywords.enabled', 'Enabled') }}
        </label>
      </div>
    </article>

    <!-- Activity Log -->
    <AuditLogPanel
      v-if="keyword && !isNew"
      resource-type="keyword_rule"
      :resource-id="keyword.id"
    />

    <!-- Sidebar -->
    <template v-if="!isNew" #sidebar>
      <MetadataPanel
        :created-at="keyword?.created_at"
        :updated-at="keyword?.updated_at"
        :created-by-name="keyword?.created_by_name"
        :updated-by-name="keyword?.updated_by_name"
      />
    </template>
  </DetailPageLayout>

  <!-- Delete Confirmation -->
  <DeleteConfirmDialog
    v-model:open="deleteDialogOpen"
    :title="$t('keywords.deleteRule', 'Delete Keyword Rule')"
    :description="$t('keywords.deleteRuleDesc', 'Are you sure? This action cannot be undone.')"
    @confirm="deleteKeyword"
  />

  <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
