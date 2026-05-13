<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/services/api'
import { toast } from 'vue-sonner'
import { getErrorMessage } from '@/lib/api-utils'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import BusinessProfileDialog from './BusinessProfileDialog.vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Separator } from '@/components/ui/separator'
import { IconButton } from '@/components/shared'
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
import {
  Phone,
  Save,
  Trash2,
  Copy,
  RefreshCw,
  Loader2,
  AlertCircle,
  CheckCircle2,
  Bell,
  Store,
  TestTube2,
  Check,
  X,
} from 'lucide-vue-next'

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
    icon-gradient="bg-gradient-to-br from-emerald-500 to-green-600 shadow-emerald-500/20"
    back-link="/settings/accounts"
    :breadcrumbs="breadcrumbs"
    :is-loading="isLoading"
    :is-not-found="isNotFound"
  >
    <template #actions>
      <div class="flex items-center gap-2">
        <Button v-if="!isNew && account" variant="outline" size="sm" :disabled="testingConnection" @click="testConnection">
          <Loader2 v-if="testingConnection" class="h-4 w-4 animate-spin mr-1" />
          <RefreshCw v-else class="h-4 w-4 mr-1" />
          {{ $t('accounts.test', 'Test') }}
        </Button>
        <Button v-if="!isNew && account" variant="outline" size="sm" :disabled="subscribing" @click="subscribeApp">
          <Loader2 v-if="subscribing" class="h-4 w-4 animate-spin mr-1" />
          <Bell v-else class="h-4 w-4 mr-1" />
          {{ $t('accounts.subscribe', 'Subscribe') }}
        </Button>
        <Button v-if="!isNew && account" variant="outline" size="sm" @click="isProfileDialogOpen = true">
          <Store class="h-4 w-4 mr-1" />
          {{ $t('accounts.businessProfile', 'Profile') }}
        </Button>
        <Button v-if="canWrite && (hasChanges || isNew)" size="sm" @click="save" :disabled="isSaving">
          <Save class="h-4 w-4 mr-1" /> {{ isSaving ? $t('common.saving', 'Saving...') : isNew ? $t('common.create') : $t('common.save') }}
        </Button>
        <Button v-if="canDelete && !isNew" variant="destructive" size="sm" @click="deleteDialogOpen = true">
          <Trash2 class="h-4 w-4 mr-1" /> {{ $t('common.delete') }}
        </Button>
      </div>
    </template>

    <!-- Test Result -->
    <Card v-if="testResult">
      <CardContent class="p-4">
        <div v-if="testResult.success" class="space-y-2">
          <div class="flex items-center gap-2 text-green-400 light:text-green-600">
            <CheckCircle2 class="h-4 w-4" />
            <span class="text-sm font-medium">{{ $t('accounts.connected', 'Connected') }}</span>
            <span v-if="testResult.display_phone_number" class="text-sm text-muted-foreground">— {{ testResult.display_phone_number }}</span>
            <Badge v-if="testResult.is_test_number" variant="outline" class="border-amber-600 text-amber-600">
              <TestTube2 class="h-3 w-3 mr-1" /> {{ $t('accounts.testNumber', 'Test Number') }}
            </Badge>
          </div>
          <div v-if="testResult.warning" class="flex items-start gap-2 p-2 rounded-lg bg-amber-950/50 light:bg-amber-50 border border-amber-800 light:border-amber-200">
            <AlertCircle class="h-4 w-4 text-amber-400 light:text-amber-600 mt-0.5 shrink-0" />
            <span class="text-sm text-amber-300 light:text-amber-700">{{ testResult.warning }}</span>
          </div>
        </div>
        <div v-else class="flex items-center gap-2 text-red-400 light:text-red-600">
          <X class="h-4 w-4" />
          <span class="text-sm">{{ testResult.error }}</span>
        </div>
      </CardContent>
    </Card>

    <!-- Account Details Card -->
    <Card>
      <CardHeader class="pb-3">
        <div class="flex items-center justify-between">
          <CardTitle class="text-sm font-medium">{{ $t('accounts.accountDetails', 'Account Details') }}</CardTitle>
          <Badge v-if="account" :variant="account.status === 'active' ? 'default' : 'secondary'">
            {{ account.status }}
          </Badge>
        </div>
      </CardHeader>
      <CardContent class="space-y-4">
        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('accounts.accountName', 'Account Name') }} *</Label>
          <Input v-model="form.name" :disabled="!canWrite" />
        </div>

        <Separator />

        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('accounts.metaAppId', 'Meta App ID') }}</Label>
            <Input v-model="form.app_id" :disabled="!canWrite" />
          </div>
          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('accounts.phoneNumberId', 'Phone Number ID') }} *</Label>
            <Input v-model="form.phone_id" :disabled="!canWrite" />
          </div>
          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('accounts.businessAccountId', 'Business Account ID') }} *</Label>
            <Input v-model="form.business_id" :disabled="!canWrite" />
          </div>
          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('accounts.apiVersion', 'API Version') }}</Label>
            <Input v-model="form.api_version" :disabled="!canWrite" />
          </div>
        </div>

        <Separator />

        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-1.5">
            <Label class="text-xs">
              {{ $t('accounts.accessToken', 'Access Token') }}
              <span v-if="isNew" class="text-destructive">*</span>
              <span v-else class="text-muted-foreground text-[10px]">{{ $t('accounts.accessTokenKeepExisting', '(leave empty to keep existing)') }}</span>
            </Label>
            <Input v-model="form.access_token" type="password" :disabled="!canWrite" />
            <Badge v-if="account?.has_access_token" variant="outline" class="border-green-600 text-green-600">
              <Check class="h-3 w-3 mr-1" /> {{ $t('accounts.configured', 'Configured') }}
            </Badge>
          </div>
          <div class="space-y-1.5">
            <Label class="text-xs">
              {{ $t('accounts.appSecret', 'App Secret') }}
              <span v-if="!isNew" class="text-muted-foreground text-[10px]">{{ $t('accounts.accessTokenKeepExisting', '(leave empty to keep existing)') }}</span>
            </Label>
            <Input v-model="form.app_secret" type="password" :disabled="!canWrite" />
            <Badge v-if="account?.has_app_secret" variant="outline" class="border-green-600 text-green-600">
              <Check class="h-3 w-3 mr-1" /> {{ $t('accounts.configured', 'Configured') }}
            </Badge>
          </div>
        </div>

        <Separator />

        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <Label class="text-xs">{{ $t('accounts.defaultIncoming', 'Default for Incoming') }}</Label>
            <Switch :checked="form.is_default_incoming" @update:checked="form.is_default_incoming = $event" :disabled="!canWrite" />
          </div>
          <div class="flex items-center justify-between">
            <Label class="text-xs">{{ $t('accounts.defaultOutgoing', 'Default for Outgoing') }}</Label>
            <Switch :checked="form.is_default_outgoing" @update:checked="form.is_default_outgoing = $event" :disabled="!canWrite" />
          </div>
          <div class="flex items-center justify-between">
            <Label class="text-xs">{{ $t('accounts.autoReadReceipt', 'Auto Read Receipt') }}</Label>
            <Switch :checked="form.auto_read_receipt" @update:checked="form.auto_read_receipt = $event" :disabled="!canWrite" />
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Webhook Config Card -->
    <Card v-if="!isNew">
      <CardHeader class="pb-3">
        <CardTitle class="text-sm font-medium">{{ $t('accounts.webhookConfig', 'Webhook Configuration') }}</CardTitle>
      </CardHeader>
      <CardContent class="space-y-3">
        <div>
          <Label class="text-xs text-muted-foreground">{{ $t('accounts.webhookUrl', 'Webhook URL') }}</Label>
          <div class="flex items-center gap-2 mt-1">
            <code class="px-2 py-1 bg-muted rounded text-xs font-mono flex-1 truncate">{{ webhookUrl }}</code>
            <IconButton :icon="Copy" label="Copy" @click="copyToClipboard(webhookUrl)" />
          </div>
        </div>
        <div>
          <Label class="text-xs text-muted-foreground">{{ $t('accounts.verifyToken', 'Verify Token') }}</Label>
          <div class="flex items-center gap-2 mt-1">
            <code class="px-2 py-1 bg-muted rounded text-xs font-mono flex-1 truncate">{{ account?.webhook_verify_token }}</code>
            <IconButton :icon="Copy" label="Copy" @click="copyToClipboard(account?.webhook_verify_token || '')" />
          </div>
        </div>
      </CardContent>
    </Card>

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
      <Card>
        <CardHeader class="pb-3">
          <CardTitle class="text-sm font-medium">{{ $t('accounts.setupGuide', 'Setup Guide') }}</CardTitle>
        </CardHeader>
        <CardContent>
          <ol class="list-decimal list-inside space-y-2.5 text-sm text-muted-foreground">
            <li>{{ $t('accounts.setupStep1', 'Go to') }} <a href="https://developers.facebook.com" target="_blank" class="text-primary hover:underline">Meta Developer Console</a> {{ $t('accounts.setupStep1End', 'and create an app') }}</li>
            <li>{{ $t('accounts.setupStep2', 'Add WhatsApp product to your app') }}</li>
            <li>{{ $t('accounts.setupStep3', 'Copy') }} <strong>{{ $t('accounts.setupStep3Bold1', 'Phone Number ID') }}</strong> {{ $t('accounts.setupStep3And', 'and') }} <strong>{{ $t('accounts.setupStep3Bold2', 'Business Account ID') }}</strong></li>
            <li>{{ $t('accounts.setupStep4', 'Generate a permanent token from') }} <a href="https://business.facebook.com/settings/system-users" target="_blank" class="text-primary hover:underline">Business Settings</a></li>
            <li>{{ $t('accounts.setupStep5', 'Configure the webhook URL and verify token in Meta dashboard') }}</li>
            <li>{{ $t('accounts.setupStep6', 'Click Test Connection to verify') }}</li>
          </ol>
        </CardContent>
      </Card>
    </template>
  </DetailPageLayout>

  <!-- Delete Confirmation -->
  <AlertDialog v-model:open="deleteDialogOpen">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ $t('accounts.deleteAccount', 'Delete Account') }}</AlertDialogTitle>
        <AlertDialogDescription>
          {{ $t('accounts.deleteAccountConfirm', 'Are you sure? This cannot be undone.') }}
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>{{ $t('common.cancel') }}</AlertDialogCancel>
        <AlertDialogAction @click="deleteAccount">{{ $t('common.delete') }}</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>

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
