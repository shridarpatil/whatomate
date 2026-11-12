<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { webhooksService, type Webhook, type WebhookEvent } from '@/services/api'
import { toast } from '@/lib/toast'
import { getErrorMessage } from '@/lib/api-utils'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import Modal from '@/components/oat/Modal.vue'
import { IconButton } from '@/components/shared'
import {
  Webhook as WebhookIcon,
  Trash2,
  Save,
  Play,
  Plus,
} from '@/lib/icons'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const authStore = useAuthStore()

const webhookId = computed(() => route.params.id as string)
const isNew = computed(() => webhookId.value === 'new')
const webhook = ref<Webhook | null>(null)
const availableEvents = ref<WebhookEvent[]>([])
const isLoading = ref(true)
const isNotFound = ref(false)
const isSaving = ref(false)
const isTesting = ref(false)
const hasChanges = ref(false)
const deleteDialogOpen = ref(false)

const { showLeaveDialog, confirmLeave, cancelLeave } = useUnsavedChangesGuard(hasChanges)

const canWrite = computed(() => authStore.hasPermission('webhooks', 'write'))
const canDelete = computed(() => authStore.hasPermission('webhooks', 'delete'))

const newHeaderKey = ref('')
const newHeaderValue = ref('')

const form = ref({
  name: '',
  url: '',
  events: [] as string[],
  secret: '',
  headers: {} as Record<string, string>,
  is_active: true,
})

const breadcrumbs = computed(() => [
  { label: t('nav.settings'), href: '/settings' },
  { label: t('nav.webhooks', 'Webhooks'), href: '/settings/webhooks' },
  { label: isNew.value ? t('webhooks.newWebhook', 'New Webhook') : (webhook.value?.name || form.value.name || '') },
])

async function fetchAvailableEvents() {
  try {
    const response = await webhooksService.list({ limit: 1 })
    const data = (response.data as any).data || response.data
    availableEvents.value = data.available_events || []
  } catch {
    // Events will remain empty; form can still be used
  }
}

async function loadWebhook() {
  isLoading.value = true
  isNotFound.value = false
  try {
    const response = await webhooksService.get(webhookId.value)
    const data = (response.data as any).data?.webhook || (response.data as any).data || response.data
    webhook.value = data
    syncForm()
    nextTick(() => { hasChanges.value = false })
  } catch {
    isNotFound.value = true
  } finally {
    isLoading.value = false
  }
}

function syncForm() {
  if (!webhook.value) return
  form.value = {
    name: webhook.value.name,
    url: webhook.value.url,
    events: [...webhook.value.events],
    secret: '',
    headers: { ...webhook.value.headers },
    is_active: webhook.value.is_active,
  }
}

watch(form, () => {
  hasChanges.value = true
}, { deep: true })

function toggleEvent(eventValue: string, checked: boolean) {
  if (checked) {
    if (!form.value.events.includes(eventValue)) form.value.events.push(eventValue)
  } else {
    const index = form.value.events.indexOf(eventValue)
    if (index > -1) form.value.events.splice(index, 1)
  }
}

function addHeader() {
  if (newHeaderKey.value.trim() && newHeaderValue.value.trim()) {
    form.value.headers[newHeaderKey.value.trim()] = newHeaderValue.value.trim()
    newHeaderKey.value = ''
    newHeaderValue.value = ''
  }
}

function removeHeader(key: string) {
  delete form.value.headers[key]
}

async function save() {
  if (!form.value.name.trim()) {
    toast.error(t('webhooks.nameRequired'))
    return
  }
  if (!form.value.url.trim()) {
    toast.error(t('webhooks.urlRequired'))
    return
  }
  if (form.value.events.length === 0) {
    toast.error(t('webhooks.eventRequired'))
    return
  }

  isSaving.value = true
  try {
    const payload = {
      name: form.value.name.trim(),
      url: form.value.url.trim(),
      events: form.value.events,
      headers: form.value.headers,
      secret: form.value.secret || undefined,
    }

    if (isNew.value) {
      const response = await webhooksService.create(payload)
      const created = (response.data as any).data?.webhook || (response.data as any).data || response.data
      hasChanges.value = false
      toast.success(t('common.createdSuccess', { resource: t('resources.Webhook') }))
      router.replace(`/settings/webhooks/${created.id}`)
    } else {
      await webhooksService.update(webhook.value!.id, {
        ...payload,
        is_active: form.value.is_active,
      })
      toast.success(t('common.updatedSuccess', { resource: t('resources.Webhook') }))
      await loadWebhook()
    }
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedSave', { resource: t('resources.webhook') })))
  } finally {
    isSaving.value = false
  }
}

async function testWebhook() {
  if (!webhook.value) return
  isTesting.value = true
  try {
    await webhooksService.test(webhook.value.id)
    toast.success(t('webhooks.testSent'))
  } catch (e) {
    toast.error(getErrorMessage(e, t('webhooks.testFailed')))
  } finally {
    isTesting.value = false
  }
}

async function deleteWebhook() {
  if (!webhook.value) return
  try {
    await webhooksService.delete(webhook.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.Webhook') }))
    router.push('/settings/webhooks')
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.webhook') })))
  }
  deleteDialogOpen.value = false
}

onMounted(async () => {
  await fetchAvailableEvents()
  if (isNew.value) {
    isLoading.value = false
    hasChanges.value = false
  } else {
    await loadWebhook()
  }
})
</script>

<template>
  <div class="h-full">
    <DetailPageLayout
      :title="isNew ? $t('webhooks.newWebhook', 'New Webhook') : (webhook?.name || '')"
      :icon="WebhookIcon"
      back-link="/settings/webhooks"
      :breadcrumbs="breadcrumbs"
      :is-loading="isLoading"
      :is-not-found="isNotFound"
      :not-found-title="$t('webhooks.notFound', 'Webhook not found')"
    >
      <template #actions>
        <div class="hstack gap-2">
          <button v-if="canWrite && (hasChanges || isNew)" type="button" class="small" :disabled="isSaving" :aria-busy="isSaving ? 'true' : undefined" data-spinner="small" @click="save">
            <Save v-if="!isSaving" :size="16" /> {{ isSaving ? $t('common.saving', 'Saving...') : isNew ? $t('common.create') : $t('common.save') }}
          </button>
          <button v-if="!isNew" type="button" class="outline small" :disabled="isTesting" :aria-busy="isTesting ? 'true' : undefined" data-spinner="small" @click="testWebhook">
            <Play v-if="!isTesting" :size="16" /> {{ isTesting ? $t('webhooks.testing', 'Testing...') : $t('webhooks.testWebhook') }}
          </button>
          <button v-if="canDelete && !isNew" type="button" class="small" data-variant="danger" @click="deleteDialogOpen = true">
            <Trash2 :size="16" /> {{ $t('common.delete') }}
          </button>
        </div>
      </template>

      <article class="card">
        <header class="hstack justify-between">
          <h3>{{ $t('teams.details', 'Details') }}</h3>
          <span v-if="!isNew" class="badge" :data-variant="(webhook?.is_active ?? true) ? 'success' : 'secondary'">
            {{ (webhook?.is_active ?? true) ? $t('common.active') : $t('common.inactive') }}
          </span>
        </header>
        <div class="vstack gap-4">
          <label data-field>
            {{ $t('webhooks.name') }} *
            <input v-model="form.name" :placeholder="$t('webhooks.namePlaceholder')" :disabled="!canWrite" />
          </label>
          <label data-field>
            {{ $t('webhooks.webhookUrl', 'URL') }} *
            <input v-model="form.url" type="url" :placeholder="$t('webhooks.webhookUrlPlaceholder')" :disabled="!canWrite" />
          </label>
          <fieldset class="vstack gap-2">
            <legend>{{ $t('webhooks.events') }} *</legend>
            <label v-for="event in availableEvents" :key="event.value" data-field>
              <span class="hstack gap-2">
                <input
                  type="checkbox"
                  :value="event.value"
                  :checked="form.events.includes(event.value)"
                  :disabled="!canWrite"
                  @change="toggleEvent(event.value, ($event.target as HTMLInputElement).checked)"
                />
                {{ event.label }}
              </span>
              <small data-hint>{{ event.description }}</small>
            </label>
          </fieldset>
          <label data-field>
            {{ $t('webhooks.secret') }}
            <input v-model="form.secret" type="password" :placeholder="$t('webhooks.secretPlaceholder')" :disabled="!canWrite" />
            <small data-hint>{{ $t('webhooks.secretHint') }}</small>
          </label>
          <div class="vstack gap-2">
            <strong>{{ $t('webhooks.customHeaders') }}</strong>
            <div v-for="(value, key) in form.headers" :key="key" class="hstack gap-2">
              <span class="badge" data-variant="secondary">{{ key }}</span>
              <span class="truncate flex-1">{{ value }}</span>
              <IconButton v-if="canWrite" :icon="Trash2" :label="$t('common.delete')" size="icon" @click="removeHeader(key as string)" />
            </div>
            <div v-if="canWrite" class="hstack gap-2">
              <input v-model="newHeaderKey" class="flex-1" :placeholder="$t('webhooks.headerName')" :aria-label="$t('webhooks.headerName')" />
              <input v-model="newHeaderValue" class="flex-1" :placeholder="$t('webhooks.headerValue')" :aria-label="$t('webhooks.headerValue')" />
              <button type="button" class="outline small" @click="addHeader">
                <Plus :size="12" /> {{ $t('common.add') }}
              </button>
            </div>
          </div>
          <label v-if="!isNew" data-field>
            <span class="hstack justify-between">
              <span>{{ $t('common.active') }}</span>
              <input v-model="form.is_active" type="checkbox" role="switch" :disabled="!canWrite" />
            </span>
          </label>
        </div>
      </article>

      <AuditLogPanel
        v-if="webhook && !isNew"
        resource-type="webhook"
        :resource-id="webhook.id"
      />

      <template v-if="!isNew" #sidebar>
        <MetadataPanel
          :created-at="webhook?.created_at"
          :updated-at="webhook?.updated_at"
        />
      </template>
    </DetailPageLayout>

    <Modal v-model:open="deleteDialogOpen" alert :title="$t('webhooks.deleteWebhook')">
      <p>{{ $t('teams.deleteConfirm', 'Are you sure? This action cannot be undone.') }}</p>
      <template #footer>
        <button type="button" class="outline" @click="deleteDialogOpen = false">{{ $t('common.cancel') }}</button>
        <button type="button" data-variant="danger" @click="deleteWebhook">{{ $t('common.delete') }}</button>
      </template>
    </Modal>

    <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
