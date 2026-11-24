<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { PageHeader, DataTable, DeleteConfirmDialog, ErrorState, type Column } from '@/components/shared'
import Modal from '@/components/oat/Modal.vue'
import { api } from '@/services/api'
import { useOrganizationsStore } from '@/stores/organizations'
import { useAuthStore } from '@/stores/auth'
import { toast } from '@/lib/toast'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDate } from '@/lib/utils'
import {
  Plus,
  Pencil,
  Trash2,
  Phone,
  Check,
  Facebook,
  Smartphone,
  Network
} from '@/lib/icons'

declare global {
  interface Window {
    FB: any
  }
}

const { t } = useI18n()
const organizationsStore = useOrganizationsStore()
const authStore = useAuthStore()

interface WhatsAppAccount {
  id: string
  name: string
  app_id: string
  phone_id: string
  business_id: string
  api_version: string
  is_default_incoming: boolean
  is_default_outgoing: boolean
  status: string
  has_access_token: boolean
  has_app_secret: boolean
  created_at: string
}

const accounts = ref<WhatsAppAccount[]>([])
const isLoading = ref(true)
const fetchError = ref(false)
const deleteDialogOpen = ref(false)
const accountToDelete = ref<WhatsAppAccount | null>(null)
const isDeleting = ref(false)

// Facebook Embedded Signup State
const whatsappConfig = ref<{ app_id: string; config_id: string; api_version: string } | null>(null)
const isFBSDKLoaded = ref(false)
const isConnectingFB = ref(false)
const showOnboardingDialog = ref(false)

const canWrite = computed(() => authStore.hasPermission('accounts', 'write'))
const canDelete = computed(() => authStore.hasPermission('accounts', 'delete'))
const breadcrumbs = computed(() => [{ label: t('nav.settings'), href: '/settings' }, { label: t('settings.accounts') }])

const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

const columns = computed<Column<WhatsAppAccount>[]>(() => [
  { key: 'account', label: t('accounts.account'), sortable: true, sortKey: 'name' },
  { key: 'app_id', label: t('accounts.appId') },
  { key: 'phone_id', label: t('accounts.phoneNumberId'), sortable: true },
  { key: 'api_version', label: t('accounts.apiVersion') },
  { key: 'defaults', label: t('accounts.defaults') },
  { key: 'status', label: t('accounts.status'), sortable: true, sortKey: 'status' },
  { key: 'created', label: t('common.created'), sortable: true, sortKey: 'created_at' },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

watch(() => organizationsStore.selectedOrgId, () => {
  fetchAccounts()
  fetchWhatsAppConfig()
})
onMounted(async () => {
  await Promise.all([fetchAccounts(), fetchWhatsAppConfig()])
})

async function fetchAccounts() {
  isLoading.value = true
  fetchError.value = false
  try {
    const response = await api.get('/accounts')
    accounts.value = response.data.data?.accounts || []
  } catch {
    fetchError.value = true
    toast.error(t('common.failedLoad', { resource: t('resources.accounts') }))
  } finally {
    isLoading.value = false
  }
}

async function fetchWhatsAppConfig() {
  try {
    const response = await api.get('/embedded-signup/config')
    whatsappConfig.value = {
      app_id: response.data.data.whatsapp_app_id,
      config_id: response.data.data.whatsapp_config_id,
      api_version: response.data.data.whatsapp_api_version || 'v21.0'
    }
    if (whatsappConfig.value.app_id && whatsappConfig.value.config_id) {
      loadFacebookSDK()
    }
  } catch (error: any) {
    console.error('Failed to fetch WhatsApp config:', error)
  }
}

function loadFacebookSDK() {
  if (isFBSDKLoaded.value || !whatsappConfig.value?.app_id) return

  const script = document.createElement('script')
  script.src = 'https://connect.facebook.net/en_US/sdk.js'
  script.async = true
  script.defer = true
  script.onload = () => {
    window.FB.init({
      appId: whatsappConfig.value!.app_id,
      cookie: true,
      xfbml: true,
      version: whatsappConfig.value!.api_version
    })
    isFBSDKLoaded.value = true
  }
  document.body.appendChild(script)
}

function launchWhatsAppSignup(isCoexistence: boolean = true) {
  if (!isFBSDKLoaded.value) {
    toast.error('Facebook SDK not loaded yet. Please wait...')
    return
  }

  if (!whatsappConfig.value) {
    toast.error('WhatsApp configuration not loaded')
    return
  }

  showOnboardingDialog.value = false
  isConnectingFB.value = true

  const loginOptions: any = {
    config_id: whatsappConfig.value.config_id,
    response_type: 'code',
    override_default_response_type: true
  }

  if (isCoexistence) {
    loginOptions.extras = {
      setup: {},
      featureType: 'whatsapp_business_app_onboarding',
      sessionInfoVersion: '3',
      version: 'v3'
    }
  } else {
    loginOptions.extras = {
      setup: {}
    }
  }

  window.FB.login(
    (response: any) => {
      if (response.authResponse) {
        const code = response.authResponse.code
        const phoneNumberId = response.authResponse.phone_number_id
        const wabaId = response.authResponse.waba_id

        if (!code) {
          toast.error('Incomplete data from Facebook: missing authorization code')
          isConnectingFB.value = false
          return
        }

        exchangeCodeForToken(code, phoneNumberId, wabaId)
      } else if (response.error) {
        console.error('Facebook SDK error:', response.error)
        toast.error(`Facebook error: ${response.error.message || 'Unknown error'}`)
        isConnectingFB.value = false
      } else {
        toast.error('Facebook login was cancelled')
        isConnectingFB.value = false
      }
    },
    loginOptions
  )
}

async function exchangeCodeForToken(code: string, phoneNumberId: string, wabaId: string) {
  try {
    const response = await api.post('/accounts/exchange-token', {
      code,
      phone_id: phoneNumberId,
      waba_id: wabaId
    })

    const account = response.data.data.account
    const pin = response.data.data.pin

    if (account.status === 'pending_registration') {
      toast.warning('Account created. Phone registration required.')
    } else if (account.status === 'active') {
      toast.success('WhatsApp account connected successfully!')
      if (pin) {
        toast.info(`Your 2FA PIN: ${pin}. Please save it securely.`, { duration: 10000 })
      }
    }

    await fetchAccounts()
  } catch (error: any) {
    console.error('Failed to exchange Facebook code for access token:', error)
    toast.error(getErrorMessage(error, 'Failed to connect WhatsApp account'))
  } finally {
    isConnectingFB.value = false
  }
}

function openDeleteDialog(account: WhatsAppAccount) {
  accountToDelete.value = account
  deleteDialogOpen.value = true
}

async function confirmDelete() {
  if (!accountToDelete.value) return
  isDeleting.value = true
  try {
    await api.delete(`/accounts/${accountToDelete.value.id}`)
    toast.success(t('common.deletedSuccess', { resource: t('resources.Account') }))
    deleteDialogOpen.value = false
    accountToDelete.value = null
    await fetchAccounts()
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.account') })))
  } finally {
    isDeleting.value = false
  }
}

</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader
      :title="$t('accounts.title')"
      :icon="Phone"
      back-link="/settings"
      :breadcrumbs="breadcrumbs"
    >
      <template #actions>
        <div v-if="canWrite" class="hstack gap-2">
          <button
            v-if="whatsappConfig?.app_id && whatsappConfig?.config_id"
            type="button"
            class="small"
            :aria-busy="isConnectingFB"
            :disabled="isConnectingFB"
            @click="showOnboardingDialog = true"
          >
            <Facebook v-if="!isConnectingFB" :size="16" />
            {{ $t('accounts.connectFacebook') }}
          </button>
          <RouterLink to="/settings/accounts/new" class="button outline small">
            <Plus :size="16" />
            {{ $t('accounts.addAccount') }}
          </RouterLink>
        </div>
      </template>
    </PageHeader>

    <ErrorState
      v-if="fetchError && !isLoading"
      :title="$t('common.loadErrorTitle')"
      :description="$t('common.loadErrorDescription')"
      class="flex-1"
    >
      <template #action><button type="button" class="small" @click="fetchAccounts">{{ $t('common.retry') }}</button></template>
    </ErrorState>

    <div v-else class="flex-1 scroll-y p-4">
      <article class="card">
        <header>
          <h3>{{ $t('accounts.yourAccounts') }}</h3>
        </header>
        <DataTable
          :items="accounts"
          :columns="columns"
          :is-loading="isLoading"
          :empty-icon="Phone"
          :empty-title="$t('accounts.noAccounts')"
          :empty-description="$t('accounts.noAccountsDesc')"
          v-model:sort-key="sortKey"
          v-model:sort-direction="sortDirection"
          item-name="accounts"
        >
          <template #empty-action>
            <div v-if="canWrite" class="hstack gap-2 justify-center">
              <button
                v-if="whatsappConfig?.app_id && whatsappConfig?.config_id"
                type="button"
                :aria-busy="isConnectingFB"
                :disabled="isConnectingFB || !isFBSDKLoaded"
                @click="showOnboardingDialog = true"
              >
                <Facebook v-if="!isConnectingFB" :size="16" />
                {{ $t('accounts.connectFacebook') }}
              </button>
              <RouterLink to="/settings/accounts/new" class="button outline">
                <Plus :size="16" />
                {{ $t('accounts.addAccount') }}
              </RouterLink>
            </div>
          </template>
          <template #cell-account="{ item: account }">
            <RouterLink :to="`/settings/accounts/${account.id}`" class="hstack gap-2 unstyled">
              <Phone :size="16" />
              <strong class="truncate">{{ account.name }}</strong>
            </RouterLink>
          </template>
          <template #cell-app_id="{ item: account }">
            <code v-if="account.app_id">{{ account.app_id }}</code>
            <span v-else class="text-light">—</span>
          </template>
          <template #cell-phone_id="{ item: account }">
            <code>{{ account.phone_id }}</code>
          </template>
          <template #cell-api_version="{ item: account }">
            <span>{{ account.api_version }}</span>
          </template>
          <template #cell-defaults="{ item: account }">
            <div class="hstack gap-1 flex-wrap">
              <span v-if="account.is_default_incoming" class="badge outline">
                <Check :size="12" /> {{ $t('accounts.incoming') }}
              </span>
              <span v-if="account.is_default_outgoing" class="badge outline">
                <Check :size="12" /> {{ $t('accounts.outgoing') }}
              </span>
            </div>
          </template>
          <template #cell-status="{ item: account }">
            <span class="badge" :data-variant="account.status === 'active' ? 'success' : 'secondary'">
              {{ account.status }}
            </span>
          </template>
          <template #cell-created="{ item: account }">
            <span class="text-light">{{ formatDate(account.created_at) }}</span>
          </template>
          <template #cell-actions="{ item: account }">
            <div class="hstack gap-1 justify-end">
              <RouterLink
                :to="`/settings/accounts/${account.id}`"
                class="button ghost icon small"
                :title="$t('common.edit')"
                :aria-label="$t('common.edit')"
              >
                <Pencil :size="16" />
              </RouterLink>
              <button
                v-if="canDelete"
                type="button"
                class="ghost icon small"
                :title="$t('common.delete')"
                :aria-label="$t('common.delete')"
                @click="openDeleteDialog(account)"
              >
                <Trash2 :size="16" />
              </button>
            </div>
          </template>
        </DataTable>
      </article>
    </div>

    <DeleteConfirmDialog
      v-model:open="deleteDialogOpen"
      :title="$t('accounts.deleteAccount')"
      :item-name="accountToDelete?.name"
      :is-submitting="isDeleting"
      @confirm="confirmDelete"
    />

    <!-- Onboarding Method Selection Dialog -->
    <Modal
      v-model:open="showOnboardingDialog"
      :title="$t('accounts.connectTitle')"
      :description="$t('accounts.connectDesc')"
      size="large"
    >
      <div class="row">
        <!-- Coexistence Option Card -->
        <div class="col-6">
          <article class="card">
            <header>
              <span class="badge" data-variant="success">{{ $t('accounts.coexistenceRecommend') }}</span>
              <h3 class="hstack gap-2"><Smartphone :size="20" /> {{ $t('accounts.coexistenceTitle') }}</h3>
            </header>
            <p class="text-light">{{ $t('accounts.coexistenceDesc') }}</p>
            <footer>
              <button type="button" @click="launchWhatsAppSignup(true)">{{ $t('accounts.selectMode') }}</button>
            </footer>
          </article>
        </div>

        <!-- Classic Option Card -->
        <div class="col-6">
          <article class="card">
            <header>
              <span class="badge" data-variant="secondary">{{ $t('accounts.classicRecommend') }}</span>
              <h3 class="hstack gap-2"><Network :size="20" /> {{ $t('accounts.classicTitle') }}</h3>
            </header>
            <p class="text-light">{{ $t('accounts.classicDesc') }}</p>
            <footer>
              <button type="button" class="outline" @click="launchWhatsAppSignup(false)">{{ $t('accounts.selectMode') }}</button>
            </footer>
          </article>
        </div>
      </div>
    </Modal>
  </div>
</template>
