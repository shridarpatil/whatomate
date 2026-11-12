<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { apiKeysService } from '@/services/api'
import { toast } from '@/lib/toast'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDateTime } from '@/lib/utils'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import Modal from '@/components/oat/Modal.vue'
import { Key, Trash2, Save, Copy, AlertTriangle } from '@/lib/icons'
import { IconButton } from '@/components/shared'

interface APIKey {
  id: string
  name: string
  key_prefix: string
  last_used_at: string | null
  expires_at: string | null
  is_active: boolean
  created_at: string
  updated_at?: string
}

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const authStore = useAuthStore()

const keyId = computed(() => route.params.id as string)
const isNew = computed(() => keyId.value === 'new')
const apiKey = ref<APIKey | null>(null)
const isLoading = ref(true)
const isNotFound = ref(false)
const isSaving = ref(false)
const deleteDialogOpen = ref(false)

const canWrite = computed(() => authStore.hasPermission('api_keys', 'write'))
const canDelete = computed(() => authStore.hasPermission('api_keys', 'delete'))

const form = ref({ name: '', expires_at: '' })

const isKeyDisplayOpen = ref(false)
const newlyCreatedKey = ref<{ key: string } | null>(null)
const createdKeyId = ref<string | null>(null)

const isExpired = computed(() => {
  if (!apiKey.value?.expires_at) return false
  return new Date(apiKey.value.expires_at) < new Date()
})

const statusVariant = computed(() => {
  if (!apiKey.value) return 'secondary'
  if (!apiKey.value.is_active) return 'secondary'
  if (isExpired.value) return 'danger'
  return 'success'
})

const statusLabel = computed(() => {
  if (!apiKey.value) return ''
  if (!apiKey.value.is_active) return t('common.inactive', 'Inactive')
  if (isExpired.value) return t('apiKeys.expired', 'Expired')
  return t('common.active', 'Active')
})

const breadcrumbs = computed(() => [
  { label: t('nav.settings'), href: '/settings' },
  { label: t('nav.apiKeys', 'API Keys'), href: '/settings/api-keys' },
  { label: isNew.value ? t('apiKeys.newApiKey', 'New API Key') : (apiKey.value?.name || '') },
])

async function loadApiKey() {
  isLoading.value = true
  isNotFound.value = false
  try {
    const response = await apiKeysService.get(keyId.value)
    apiKey.value = (response.data as any).data || response.data
  } catch {
    isNotFound.value = true
  } finally {
    isLoading.value = false
  }
}

async function create() {
  if (!form.value.name.trim()) { toast.error(t('apiKeys.nameRequired')); return }
  isSaving.value = true
  try {
    const payload: { name: string; expires_at?: string } = { name: form.value.name.trim() }
    if (form.value.expires_at) payload.expires_at = new Date(form.value.expires_at).toISOString()
    const response = await apiKeysService.create(payload)
    const created = response.data.data
    newlyCreatedKey.value = created
    createdKeyId.value = created.id
    isKeyDisplayOpen.value = true
    toast.success(t('common.createdSuccess', { resource: t('resources.APIKey', 'API Key') }))
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedCreate', { resource: t('resources.APIKey', 'API key') })))
  } finally {
    isSaving.value = false
  }
}

async function toggleActive() {
  if (!apiKey.value) return
  try {
    await apiKeysService.update(apiKey.value.id, { is_active: !apiKey.value.is_active })
    toast.success(apiKey.value.is_active ? t('common.disabledSuccess', { resource: t('resources.APIKey', 'API Key') }) : t('common.enabledSuccess', { resource: t('resources.APIKey', 'API Key') }))
    await loadApiKey()
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedToggle', { resource: t('resources.apiKey', 'API key') })))
  }
}

async function deleteApiKey() {
  if (!apiKey.value) return
  try {
    await apiKeysService.delete(apiKey.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.apiKey', 'API Key') }))
    router.push('/settings/api-keys')
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.apiKey', 'API key') })))
  }
  deleteDialogOpen.value = false
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text)
  toast.success(t('common.copiedToClipboard'))
}

onMounted(async () => {
  if (isNew.value) {
    isLoading.value = false
  } else {
    await loadApiKey()
  }
})
</script>

<template>
  <div class="h-full">
    <DetailPageLayout
      :title="isNew ? $t('apiKeys.newApiKey', 'New API Key') : (apiKey?.name || '')"
      :icon="Key"
      back-link="/settings/api-keys"
      :breadcrumbs="breadcrumbs"
      :is-loading="isLoading"
      :is-not-found="isNotFound"
      :not-found-title="$t('apiKeys.notFound', 'API Key not found')"
    >
      <template #actions>
        <div class="hstack gap-2">
          <button v-if="isNew && canWrite" type="button" class="small" :disabled="isSaving" :aria-busy="isSaving ? 'true' : undefined" data-spinner="small" @click="create">
            <Save v-if="!isSaving" :size="16" /> {{ isSaving ? $t('common.saving', 'Saving...') : $t('common.create') }}
          </button>
          <button v-if="!isNew && canDelete" type="button" class="small" data-variant="danger" @click="deleteDialogOpen = true">
            <Trash2 :size="16" /> {{ $t('common.delete') }}
          </button>
        </div>
      </template>

      <!-- Create form -->
      <article v-if="isNew" class="card">
        <header>
          <h3>{{ $t('teams.details', 'Details') }}</h3>
        </header>
        <div class="vstack gap-4">
          <label data-field>
            {{ $t('apiKeys.name') }} *
            <input v-model="form.name" :placeholder="$t('apiKeys.namePlaceholder')" />
          </label>
          <label data-field>
            {{ $t('apiKeys.expiration') }}
            <input v-model="form.expires_at" type="datetime-local" />
            <small data-hint>{{ $t('apiKeys.expirationHint') }}</small>
          </label>
        </div>
      </article>

      <!-- View existing key -->
      <article v-else class="card">
        <header class="hstack justify-between">
          <h3>{{ $t('teams.details', 'Details') }}</h3>
          <span class="badge" :data-variant="statusVariant">{{ statusLabel }}</span>
        </header>
        <div class="vstack gap-4">
          <div class="hstack gap-2">
            <Key :size="24" />
            <div class="min-w-0">
              <p class="truncate"><strong>{{ apiKey?.name }}</strong></p>
              <code>whm_{{ apiKey?.key_prefix }}...</code>
            </div>
          </div>

          <div class="vstack gap-4">
            <div>
              <small class="text-light">{{ $t('common.createdAt', 'Created') }}</small>
              <p>{{ apiKey?.created_at ? formatDateTime(apiKey.created_at) : '—' }}</p>
            </div>
            <div>
              <small class="text-light">{{ $t('apiKeys.lastUsedAt', 'Last Used') }}</small>
              <p>{{ apiKey?.last_used_at ? formatDateTime(apiKey.last_used_at) : '—' }}</p>
            </div>
            <div>
              <small class="text-light">{{ $t('apiKeys.expiresAt', 'Expires') }}</small>
              <div class="hstack gap-2">
                <p>{{ apiKey?.expires_at ? formatDateTime(apiKey.expires_at) : $t('apiKeys.never', 'Never') }}</p>
                <span v-if="isExpired" class="badge" data-variant="danger">{{ $t('apiKeys.expired', 'Expired') }}</span>
              </div>
            </div>
          </div>
          <template v-if="canWrite">
            <hr>
            <label class="hstack justify-between">
              <span>{{ $t('common.active') }}</span>
              <input
                type="checkbox"
                role="switch"
                :checked="apiKey?.is_active ?? false"
                :disabled="isExpired"
                @click.prevent="toggleActive"
              />
            </label>
          </template>
        </div>
      </article>

      <AuditLogPanel v-if="apiKey && !isNew" resource-type="api_key" :resource-id="apiKey.id" />

      <template v-if="!isNew" #sidebar>
        <MetadataPanel :created-at="apiKey?.created_at" :updated-at="apiKey?.updated_at" />
      </template>
    </DetailPageLayout>

    <!-- Key created dialog (shown once after create) -->
    <Modal v-model:open="isKeyDisplayOpen" :title="$t('apiKeys.apiKeyCreated')">
      <div class="vstack gap-4">
        <div role="alert" data-variant="warning">
          <AlertTriangle :size="16" />
          {{ $t('apiKeys.apiKeyCreatedWarning') }}
        </div>
        <label data-field>
          {{ $t('apiKeys.yourApiKey') }}
          <span class="hstack gap-2">
            <input :value="newlyCreatedKey?.key" readonly />
            <IconButton :icon="Copy" :label="$t('apiKeys.copyApiKey')" variant="outline" @click="copyToClipboard(newlyCreatedKey?.key || '')" />
          </span>
        </label>
        <div>
          <p><strong>{{ $t('apiKeys.usage') }}:</strong></p>
          <code>curl -H "X-API-Key: {{ newlyCreatedKey?.key }}" https://your-api.com/api/contacts</code>
        </div>
      </div>
      <template #footer>
        <button type="button" class="small" @click="isKeyDisplayOpen = false; if (createdKeyId) router.replace(`/settings/api-keys/${createdKeyId}`)">{{ $t('common.done') }}</button>
      </template>
    </Modal>

    <Modal v-model:open="deleteDialogOpen" alert :title="$t('apiKeys.deleteKey', 'Delete API Key')">
      <p>{{ $t('apiKeys.deleteWarning', 'Are you sure you want to delete this API key? Any applications using this key will immediately lose access. This action cannot be undone.') }}</p>
      <template #footer>
        <button type="button" class="outline" @click="deleteDialogOpen = false">{{ $t('common.cancel') }}</button>
        <button type="button" data-variant="danger" @click="deleteApiKey">{{ $t('common.delete') }}</button>
      </template>
    </Modal>
  </div>
</template>
