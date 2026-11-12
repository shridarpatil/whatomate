<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/services/api'
import { toast } from '@/lib/toast'
import { getErrorMessage } from '@/lib/api-utils'
import { getQualityBadgeVariant, getQualityRatingLabel, getVerificationBadgeVariant, getVerificationStatusLabel, formatLimitTier } from '@/lib/utils'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import BusinessProfileDialog from './BusinessProfileDialog.vue'
import { IconButton } from '@/components/shared'
import Modal from '@/components/oat/Modal.vue'
import {
  Phone,
  Save,
  Trash2,
  Copy,
  RefreshCw,
  AlertCircle,
  CheckCircle2,
  Bell,
  Store,
  TestTube2,
  Check,
  X,
} from '@/lib/icons'

interface WhatsAppAccount {
  id: string
  name: string
  app_id: string
  phone_id: string
  business_id: string
  webhook_verify_token: string
  api_version: string
  is_default_incoming: boolean
  is_default_outgoing: boolean
  auto_read_receipt: boolean
  business_calling_enabled: boolean
  status: string
  has_access_token: boolean
  has_app_secret: boolean
  phone_number?: string
  display_name?: string
  created_by_id?: string
  created_by_name?: string
  updated_by_id?: string
  updated_by_name?: string
  created_at: string
  updated_at: string
}

interface TestResult {
  success: boolean
  error?: string
  display_phone_number?: string
  verified_name?: string
  quality_rating?: string
  messaging_limit_tier?: string
  code_verification_status?: string
  account_mode?: string
  is_test_number?: boolean
  warning?: string
}

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const authStore = useAuthStore()

const accountId = computed(() => route.params.id as string)
const isNew = computed(() => accountId.value === 'new')
const account = ref<WhatsAppAccount | null>(null)
const isLoading = ref(true)
const isNotFound = ref(false)
const isSaving = ref(false)
const hasChanges = ref(false)
const deleteDialogOpen = ref(false)
const testResult = ref<TestResult | null>(null)
const testingConnection = ref(false)
const subscribing = ref(false)
const isProfileDialogOpen = ref(false)

const { showLeaveDialog, confirmLeave, cancelLeave } = useUnsavedChangesGuard(hasChanges)

const canWrite = computed(() => authStore.hasPermission('accounts', 'write'))
const canDelete = computed(() => authStore.hasPermission('accounts', 'delete'))

const form = ref({
  name: '',
  app_id: '',
  phone_id: '',
  business_id: '',
  access_token: '',
  app_secret: '',
  webhook_verify_token: '',
  api_version: 'v21.0',
  is_default_incoming: false,
  is_default_outgoing: false,
  auto_read_receipt: false,
  business_calling_enabled: false,
})

const breadcrumbs = computed(() => [
  { label: t('nav.settings'), href: '/settings' },
  { label: t('settings.accounts'), href: '/settings/accounts' },
  { label: isNew.value ? t('accounts.newAccount', 'New Account') : (account.value?.name || '') },
])

const basePath = ((window as any).__BASE_PATH__ ?? '').replace(/\/$/, '')
const webhookUrl = window.location.origin + basePath + '/api/webhook'

// Track form changes
watch(form, () => { hasChanges.value = true }, { deep: true })

async function loadAccount() {
  isLoading.value = true
  isNotFound.value = false
  try {
    const response = await api.get(`/accounts/${accountId.value}`)
    const data = response.data.data || response.data
    account.value = data
    syncForm()
    nextTick(() => { hasChanges.value = false })
  } catch {
    isNotFound.value = true
  } finally {
    isLoading.value = false
  }
}

function syncForm() {
  if (!account.value) return
  form.value = {
    name: account.value.name,
    app_id: account.value.app_id || '',
    phone_id: account.value.phone_id,
    business_id: account.value.business_id,
    access_token: '',
    app_secret: '',
    webhook_verify_token: account.value.webhook_verify_token || '',
    api_version: account.value.api_version,
    is_default_incoming: account.value.is_default_incoming,
    is_default_outgoing: account.value.is_default_outgoing,
    auto_read_receipt: account.value.auto_read_receipt,
    business_calling_enabled: account.value.business_calling_enabled ?? false,
  }
}

async function save() {
  if (!form.value.name.trim() || !form.value.phone_id.trim() || !form.value.business_id.trim()) {
    toast.error(t('accounts.fillRequired', 'Name, Phone ID, and Business ID are required'))
    return
  }
  if (isNew.value && !form.value.access_token.trim()) {
    toast.error(t('accounts.accessTokenRequired', 'Access token is required'))
    return
  }

  isSaving.value = true
  try {
    const payload: any = { ...form.value }
    if (!isNew.value && !payload.access_token) delete payload.access_token
    if (!isNew.value && !payload.app_secret) delete payload.app_secret

    if (isNew.value) {
      const response = await api.post('/accounts', payload)
      const created = response.data.data || response.data
      hasChanges.value = false
      toast.success(t('common.createdSuccess', { resource: t('resources.Account') }))
      router.replace(`/settings/accounts/${created.id}`)
    } else {
      await api.put(`/accounts/${account.value!.id}`, payload)
      await loadAccount()
      hasChanges.value = false
      toast.success(t('common.updatedSuccess', { resource: t('resources.Account') }))
    }
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedSave', { resource: t('resources.account') })))
  } finally {
    isSaving.value = false
  }
}

async function deleteAccount() {
  if (!account.value) return
  try {
    await api.delete(`/accounts/${account.value.id}`)
    toast.success(t('common.deletedSuccess', { resource: t('resources.Account') }))
    router.push('/settings/accounts')
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.account') })))
  }
  deleteDialogOpen.value = false
}

async function testConnection() {
  if (!account.value) return
  testingConnection.value = true
  try {
    const response = await api.post(`/accounts/${account.value.id}/test`)
    testResult.value = response.data.data
    if (testResult.value?.success) {
      toast.success(t('accounts.connectionSuccess', 'Connection successful'))
    } else {
      toast.error(t('accounts.connectionFailed', 'Connection failed') + ': ' + (testResult.value?.error || ''))
    }
  } catch (e) {
    testResult.value = { success: false, error: getErrorMessage(e, t('accounts.connectionTestFailed', 'Test failed')) }
    toast.error(testResult.value.error!)
  } finally {
    testingConnection.value = false
  }
}

async function subscribeApp() {
  if (!account.value) return
  subscribing.value = true
  try {
    const response = await api.post(`/accounts/${account.value.id}/subscribe`)
    if (response.data.data?.success) {
      toast.success(t('accounts.subscribeSuccess', 'Subscribed successfully'))
    } else {
      toast.error(t('accounts.subscribeFailed', 'Subscribe failed'))
    }
  } catch (e) {
    toast.error(getErrorMessage(e, t('accounts.subscribeError', 'Subscribe error')))
  } finally {
    subscribing.value = false
  }
}

async function copyToClipboard(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success(t('common.copiedToClipboard', 'Copied'))
  } catch {
    toast.error(t('common.clipboardFailed', 'Failed to copy'))
  }
}

onMounted(async () => {
  if (isNew.value) {
    isLoading.value = false
    hasChanges.value = false
  } else {
    await loadAccount()
  }
})
</script>

<template>
  <div class="h-full">
  <DetailPageLayout
    :title="isNew ? $t('accounts.newAccount', 'New Account') : (account?.name || '')"
    :icon="Phone"
    back-link="/settings/accounts"
    :breadcrumbs="breadcrumbs"
    :is-loading="isLoading"
    :is-not-found="isNotFound"
  >
    <template #actions>
      <div class="hstack gap-2">
        <button v-if="!isNew && account" type="button" class="outline small" :aria-busy="testingConnection" :disabled="testingConnection" @click="testConnection">
          <RefreshCw v-if="!testingConnection" :size="16" />
          {{ $t('accounts.test', 'Test') }}
        </button>
        <button v-if="!isNew && account" type="button" class="outline small" :aria-busy="subscribing" :disabled="subscribing" @click="subscribeApp">
          <Bell v-if="!subscribing" :size="16" />
          {{ $t('accounts.subscribe', 'Subscribe') }}
        </button>
        <button v-if="!isNew && account" type="button" class="outline small" @click="isProfileDialogOpen = true">
          <Store :size="16" />
          {{ $t('accounts.businessProfile', 'Profile') }}
        </button>
        <button v-if="canWrite && (hasChanges || isNew)" type="button" class="small" :aria-busy="isSaving" :disabled="isSaving" @click="save">
          <Save v-if="!isSaving" :size="16" /> {{ isSaving ? $t('common.saving', 'Saving...') : isNew ? $t('common.create') : $t('common.save') }}
        </button>
        <button v-if="canDelete && !isNew" type="button" class="small" data-variant="danger" @click="deleteDialogOpen = true">
          <Trash2 :size="16" /> {{ $t('common.delete') }}
        </button>
      </div>
    </template>

    <!-- Test Result -->
    <article v-if="testResult" class="card">
      <div v-if="testResult.success" class="vstack gap-2">
        <div class="hstack gap-2 flex-wrap">
          <CheckCircle2 :size="16" />
          <strong>{{ $t('accounts.connected', 'Connected') }}</strong>
          <span v-if="testResult.display_phone_number" class="text-light">— {{ testResult.display_phone_number }}</span>
          <span v-if="testResult.is_test_number" class="badge" data-variant="warning">
            <TestTube2 :size="12" /> {{ $t('accounts.testNumber', 'Test Number') }}
          </span>
        </div>
        <div v-if="testResult.warning" role="alert" data-variant="warning">
          <AlertCircle :size="16" />
          {{ testResult.warning }}
        </div>
        <!-- Meta Connection Details -->
        <hr>
        <h4>{{ $t('accounts.metaConnectionDetails', 'Details') }}</h4>
        <div class="row">
          <!-- Verified Name -->
          <div v-if="testResult.verified_name" class="col-3">
            <small class="text-light">{{ $t('accounts.verifiedName', 'Verified Name') }}</small>
            <p class="truncate" :title="testResult.verified_name"><strong>{{ testResult.verified_name }}</strong></p>
          </div>
          <!-- Quality Rating -->
          <div class="col-3">
            <small class="text-light">{{ $t('accounts.qualityRating', 'Quality Rating') }}</small>
            <p>
              <span class="badge" :data-variant="getQualityBadgeVariant(testResult.quality_rating || '')">
                {{ getQualityRatingLabel(testResult.quality_rating || '', t) }}
              </span>
            </p>
          </div>
          <!-- Messaging Limit Tier -->
          <div class="col-3">
            <small class="text-light">{{ $t('accounts.messagingLimitTier', 'Messaging Limit') }}</small>
            <p>
              <strong>{{ formatLimitTier(testResult.messaging_limit_tier, testResult.is_test_number || testResult.account_mode === 'SANDBOX', t) }}</strong>
            </p>
          </div>
          <!-- Verification Status -->
          <div v-if="testResult.code_verification_status" class="col-3">
            <small class="text-light">{{ $t('accounts.codeVerificationStatus', 'Verification Status') }}</small>
            <p>
              <span class="badge" :data-variant="getVerificationBadgeVariant(testResult.code_verification_status)">
                {{ getVerificationStatusLabel(testResult.code_verification_status, t) }}
              </span>
            </p>
          </div>
        </div>
      </div>
      <div v-else class="hstack gap-2">
        <X :size="16" />
        <span>{{ testResult.error }}</span>
      </div>
    </article>

    <!-- Account Details Card -->
    <article class="card">
      <header class="hstack justify-between">
        <h3>{{ $t('accounts.accountDetails', 'Account Details') }}</h3>
        <span v-if="account" class="badge" :data-variant="account.status === 'active' ? 'success' : 'secondary'">
          {{ account.status }}
        </span>
      </header>

      <label data-field>
        {{ $t('accounts.accountName', 'Account Name') }} *
        <input v-model="form.name" :disabled="!canWrite" />
      </label>

      <hr>

      <div class="row">
        <div class="col-6">
          <label data-field>
            {{ $t('accounts.metaAppId', 'Meta App ID') }}
            <input v-model="form.app_id" :disabled="!canWrite" />
          </label>
        </div>
        <div class="col-6">
          <label data-field>
            {{ $t('accounts.phoneNumberId', 'Phone Number ID') }} *
            <input v-model="form.phone_id" :disabled="!canWrite" />
          </label>
        </div>
        <div class="col-6">
          <label data-field>
            {{ $t('accounts.businessAccountId', 'Business Account ID') }} *
            <input v-model="form.business_id" :disabled="!canWrite" />
          </label>
        </div>
        <div class="col-6">
          <label data-field>
            {{ $t('accounts.apiVersion', 'API Version') }}
            <input v-model="form.api_version" :disabled="!canWrite" />
          </label>
        </div>
      </div>

      <hr>

      <div class="row">
        <div class="col-6">
          <label data-field>
            <span>
              {{ $t('accounts.accessToken', 'Access Token') }}
              <span v-if="isNew">*</span>
              <small v-else class="text-light">{{ $t('accounts.accessTokenKeepExisting', '(leave empty to keep existing)') }}</small>
            </span>
            <input v-model="form.access_token" type="password" :disabled="!canWrite" />
            <span v-if="account?.has_access_token" class="badge" data-variant="success">
              <Check :size="12" /> {{ $t('accounts.configured', 'Configured') }}
            </span>
          </label>
        </div>
        <div class="col-6">
          <label data-field>
            <span>
              {{ $t('accounts.appSecret', 'App Secret') }}
              <small v-if="!isNew" class="text-light">{{ $t('accounts.accessTokenKeepExisting', '(leave empty to keep existing)') }}</small>
            </span>
            <input v-model="form.app_secret" type="password" :disabled="!canWrite" />
            <span v-if="account?.has_app_secret" class="badge" data-variant="success">
              <Check :size="12" /> {{ $t('accounts.configured', 'Configured') }}
            </span>
          </label>
        </div>
      </div>

      <hr>

          <label data-field>
            <span class="hstack justify-between">
              <span>{{ $t('accounts.defaultIncoming', 'Default for Incoming') }}</span>
              <input type="checkbox" role="switch" v-model="form.is_default_incoming" :disabled="!canWrite" />
            </span>
          </label>
          <label data-field>
            <span class="hstack justify-between">
              <span>{{ $t('accounts.defaultOutgoing', 'Default for Outgoing') }}</span>
              <input type="checkbox" role="switch" v-model="form.is_default_outgoing" :disabled="!canWrite" />
            </span>
          </label>
          <label data-field>
            <span class="hstack justify-between">
              <span>{{ $t('accounts.autoReadReceipt', 'Auto Read Receipt') }}</span>
              <input type="checkbox" role="switch" v-model="form.auto_read_receipt" :disabled="!canWrite" />
            </span>
          </label>
          <label data-field>
            <span class="hstack justify-between">
              <span>{{ $t('accounts.businessCallingEnabled', 'Business Calling enabled') }}</span>
              <input type="checkbox" role="switch" v-model="form.business_calling_enabled" :disabled="!canWrite" />
            </span>
            <small data-hint>
              {{ $t('accounts.businessCallingEnabledDesc', 'Enable only after Meta enrolls this number in the WhatsApp Business Calling API. Required for click-to-call buttons in canned responses.') }}
            </small>
          </label>
    </article>

    <!-- Webhook Config Card -->
    <article v-if="!isNew" class="card">
      <header>
        <h3>{{ $t('accounts.webhookConfig', 'Webhook Configuration') }}</h3>
      </header>
      <div class="vstack gap-4">
        <div>
          <small class="text-light">{{ $t('accounts.webhookUrl', 'Webhook URL') }}</small>
          <div class="hstack gap-2">
            <code class="flex-1 truncate">{{ webhookUrl }}</code>
            <IconButton :icon="Copy" label="Copy" @click="copyToClipboard(webhookUrl)" />
          </div>
        </div>
        <div>
          <small class="text-light">{{ $t('accounts.verifyToken', 'Verify Token') }}</small>
          <div class="hstack gap-2">
            <code class="flex-1 truncate">{{ account?.webhook_verify_token }}</code>
            <IconButton :icon="Copy" label="Copy" @click="copyToClipboard(account?.webhook_verify_token || '')" />
          </div>
        </div>
      </div>
    </article>

    <!-- Activity Log -->
    <AuditLogPanel
      v-if="account && !isNew"
      resource-type="account"
      :resource-id="account.id"
    />

    <!-- Sidebar -->
    <template #sidebar>
      <MetadataPanel
        v-if="!isNew"
        :created-at="account?.created_at"
        :updated-at="account?.updated_at"
        :created-by-name="account?.created_by_name"
        :updated-by-name="account?.updated_by_name"
      />

      <!-- Setup Guide -->
      <article class="card">
        <header>
          <h3>{{ $t('accounts.setupGuide', 'Setup Guide') }}</h3>
        </header>
        <ol class="text-light">
          <li>{{ $t('accounts.setupStep1', 'Go to') }} <a href="https://developers.facebook.com" target="_blank" rel="noopener noreferrer">Meta Developer Console</a> {{ $t('accounts.setupStep1End', 'and create an app') }}</li>
          <li>{{ $t('accounts.setupStep2', 'Add WhatsApp product to your app') }}</li>
          <li>{{ $t('accounts.setupStep3', 'Copy') }} <strong>{{ $t('accounts.setupStep3Bold1', 'Phone Number ID') }}</strong> {{ $t('accounts.setupStep3And', 'and') }} <strong>{{ $t('accounts.setupStep3Bold2', 'Business Account ID') }}</strong></li>
          <li>{{ $t('accounts.setupStep4', 'Generate a permanent token from') }} <a href="https://business.facebook.com/settings/system-users" target="_blank" rel="noopener noreferrer">Business Settings</a></li>
          <li>{{ $t('accounts.setupStep5', 'Configure the webhook URL and verify token in Meta dashboard') }}</li>
          <li>{{ $t('accounts.setupStep6', 'Click Test Connection to verify') }}</li>
        </ol>
      </article>
    </template>
  </DetailPageLayout>

  <!-- Delete Confirmation -->
  <Modal v-model:open="deleteDialogOpen" alert :title="$t('accounts.deleteAccount', 'Delete Account')">
    <p>{{ $t('accounts.deleteAccountConfirm', 'Are you sure? This cannot be undone.') }}</p>
    <template #footer>
      <button type="button" class="outline" @click="deleteDialogOpen = false">{{ $t('common.cancel') }}</button>
      <button type="button" data-variant="danger" @click="deleteAccount">{{ $t('common.delete') }}</button>
    </template>
  </Modal>

  <!-- Business Profile -->
  <BusinessProfileDialog
    v-if="account"
    v-model:open="isProfileDialogOpen"
    :account-id="account.id"
    :account-name="account.name"
  />

  <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
