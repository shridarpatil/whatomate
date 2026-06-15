<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { PageHeader, DataTable, DeleteConfirmDialog, ErrorState, type Column } from '@/components/shared'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { api } from '@/services/api'
import { useOrganizationsStore } from '@/stores/organizations'
import { useAuthStore } from '@/stores/auth'
import { toast } from 'vue-sonner'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDate } from '@/lib/utils'
import {
  Plus,
  Pencil,
  Trash2,
  Phone,
  Check,
  Loader2,
  Facebook,
  Smartphone,
  Network
} from 'lucide-vue-next'

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
  { key: 'account', label: t('accounts.account'), width: 'w-[250px]', sortable: true, sortKey: 'name' },
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
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader
      :title="$t('accounts.title')"
      :icon="Phone"
      icon-gradient="bg-gradient-to-br from-emerald-500 to-green-600 shadow-emerald-500/20"
      back-link="/settings"
      :breadcrumbs="breadcrumbs"
    >
      <template #actions>
        <div v-if="canWrite" class="flex items-center gap-2">
          <Button
            v-if="whatsappConfig?.app_id && whatsappConfig?.config_id"
            size="sm"
            @click="showOnboardingDialog = true"
            :disabled="isConnectingFB"
            class="bg-gradient-to-br from-facebook to-facebook-dark hover:from-facebook-hover hover:to-facebook-hoverDark text-white border-none shadow-none"
          >
            <Loader2 v-if="isConnectingFB" class="h-4 w-4 mr-2 animate-spin" />
            <Facebook v-else class="h-4 w-4 mr-2" />
            {{ $t('accounts.connectFacebook') }}
          </Button>
          <RouterLink to="/settings/accounts/new">
            <Button variant="outline" size="sm">
              <Plus class="h-4 w-4 mr-2" />
              {{ $t('accounts.addAccount') }}
            </Button>
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
      <template #action><Button size="sm" @click="fetchAccounts">{{ $t('common.retry') }}</Button></template>
    </ErrorState>

    <ScrollArea v-else class="flex-1">
      <div class="p-6">
        <div>
          <Card>
            <CardHeader>
              <div>
                <CardTitle>{{ $t('accounts.yourAccounts') }}</CardTitle>
                <CardDescription>{{ $t('accounts.yourAccountsDesc') }}</CardDescription>
              </div>
            </CardHeader>
            <CardContent>
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
                  <div v-if="canWrite" class="flex gap-3 justify-center">
                    <Button
                      v-if="whatsappConfig?.app_id && whatsappConfig?.config_id"
                      size="lg"
                      @click="showOnboardingDialog = true"
                      :disabled="isConnectingFB || !isFBSDKLoaded"
                      class="bg-gradient-to-br from-facebook to-facebook-dark hover:from-facebook-hover hover:to-facebook-hoverDark text-white border-none shadow-none"
                    >
                      <Facebook v-if="!isConnectingFB" class="mr-2 h-5 w-5" />
                      <Loader2 v-else class="mr-2 h-5 w-5 animate-spin" />
                      {{ $t('accounts.connectFacebook') }}
                    </Button>
                    <RouterLink to="/settings/accounts/new">
                      <Button variant="outline" size="lg">
                        <Plus class="mr-2 h-5 w-5" />
                        {{ $t('accounts.addAccount') }}
                      </Button>
                    </RouterLink>
                  </div>
                </template>
                <template #cell-account="{ item: account }">
                  <RouterLink :to="`/settings/accounts/${account.id}`" class="flex items-center gap-3 text-inherit no-underline hover:opacity-80">
                    <div class="h-9 w-9 rounded-full bg-emerald-500/10 flex items-center justify-center flex-shrink-0">
                      <Phone class="h-4 w-4 text-emerald-500" />
                    </div>
                    <p class="font-medium truncate">{{ account.name }}</p>
                  </RouterLink>
                </template>
                <template #cell-app_id="{ item: account }">
                  <code v-if="account.app_id" class="text-xs bg-muted px-1.5 py-0.5 rounded">{{ account.app_id }}</code>
                  <span v-else class="text-muted-foreground">—</span>
                </template>
                <template #cell-phone_id="{ item: account }">
                  <code class="text-xs bg-muted px-1.5 py-0.5 rounded">{{ account.phone_id }}</code>
                </template>
                <template #cell-api_version="{ item: account }">
                  <span class="text-sm">{{ account.api_version }}</span>
                </template>
                <template #cell-defaults="{ item: account }">
                  <div class="flex items-center gap-1.5 flex-wrap">
                    <Badge v-if="account.is_default_incoming" variant="outline" class="text-[10px]">
                      <Check class="h-2.5 w-2.5 mr-0.5" /> {{ $t('accounts.incoming') }}
                    </Badge>
                    <Badge v-if="account.is_default_outgoing" variant="outline" class="text-[10px]">
                      <Check class="h-2.5 w-2.5 mr-0.5" /> {{ $t('accounts.outgoing') }}
                    </Badge>
                  </div>
                </template>
                <template #cell-status="{ item: account }">
                  <Badge variant="outline" :class="account.status === 'active' ? 'border-green-600 text-green-600' : ''">
                    {{ account.status }}
                  </Badge>
                </template>
                <template #cell-created="{ item: account }">
                  <span class="text-muted-foreground">{{ formatDate(account.created_at) }}</span>
                </template>
                <template #cell-actions="{ item: account }">
                  <div class="flex items-center justify-end gap-1">
                    <Tooltip>
                      <TooltipTrigger as-child>
                        <RouterLink :to="`/settings/accounts/${account.id}`">
                          <Button variant="ghost" size="icon" class="h-8 w-8"><Pencil class="h-4 w-4" /></Button>
                        </RouterLink>
                      </TooltipTrigger>
                      <TooltipContent>{{ $t('common.edit') }}</TooltipContent>
                    </Tooltip>
                    <Tooltip v-if="canDelete">
                      <TooltipTrigger as-child>
                        <Button variant="ghost" size="icon" class="h-8 w-8" @click="openDeleteDialog(account)">
                          <Trash2 class="h-4 w-4 text-destructive" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>{{ $t('common.delete') }}</TooltipContent>
                    </Tooltip>
                  </div>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <DeleteConfirmDialog
      v-model:open="deleteDialogOpen"
      :title="$t('accounts.deleteAccount')"
      :item-name="accountToDelete?.name"
      :is-submitting="isDeleting"
      @confirm="confirmDelete"
    />

    <!-- Onboarding Method Selection Dialog -->
    <Dialog v-model:open="showOnboardingDialog">
      <DialogContent class="sm:max-w-2xl bg-[#0e0e11] border-[#222227] text-white light:bg-white light:border-gray-200 light:text-gray-900 p-6 shadow-2xl rounded-xl">
        <DialogHeader class="mb-4">
          <DialogTitle class="text-xl font-bold bg-gradient-to-r from-emerald-400 to-green-400 light:from-emerald-600 light:to-green-600 bg-clip-text text-transparent flex items-center gap-2">
            {{ $t('accounts.connectTitle') }}
          </DialogTitle>
          <DialogDescription class="text-gray-400 light:text-gray-500 mt-1">
            {{ $t('accounts.connectDesc') }}
          </DialogDescription>
        </DialogHeader>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-4 my-4">
          <!-- Coexistence Option Card -->
          <div
            @click="launchWhatsAppSignup(true)"
            class="relative group cursor-pointer flex flex-col p-5 rounded-xl border border-emerald-500/20 bg-[#141419] hover:bg-[#181822] hover:border-emerald-500/50 hover:shadow-[0_0_20px_rgba(16,185,129,0.1)] light:bg-gray-50/50 light:border-emerald-200 light:hover:bg-gray-100/70 light:hover:border-emerald-400 light:hover:shadow-[0_0_20px_rgba(16,185,129,0.05)] transition-all duration-300 overflow-hidden"
          >
            <!-- Badge -->
            <div class="absolute top-3 right-3">
              <span class="text-[10px] bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 px-2 py-0.5 rounded-full font-medium light:bg-emerald-50 light:text-emerald-600 light:border-emerald-200">
                {{ $t('accounts.coexistenceRecommend') }}
              </span>
            </div>

            <div class="h-10 w-10 rounded-lg bg-emerald-500/10 light:bg-emerald-100/60 flex items-center justify-center mb-4 group-hover:scale-110 transition-transform duration-300">
              <Smartphone class="h-5 w-5 text-emerald-400 light:text-emerald-600" />
            </div>

            <h3 class="text-base font-semibold text-white light:text-gray-900 group-hover:text-emerald-400 light:group-hover:text-emerald-600 transition-colors duration-200">
              {{ $t('accounts.coexistenceTitle') }}
            </h3>
            <p class="text-xs text-gray-400 light:text-gray-600 mt-2 flex-grow leading-relaxed">
              {{ $t('accounts.coexistenceDesc') }}
            </p>

            <div class="mt-5 flex items-center justify-between text-xs font-medium text-emerald-400 light:text-emerald-600">
              <span>{{ $t('accounts.selectMode') }}</span>
              <span class="group-hover:translate-x-1 transition-transform duration-200">→</span>
            </div>
          </div>

          <!-- Classic Option Card -->
          <div
            @click="launchWhatsAppSignup(false)"
            class="relative group cursor-pointer flex flex-col p-5 rounded-xl border border-[#222227] bg-[#141419] hover:bg-[#181822] hover:border-blue-500/50 hover:shadow-[0_0_20px_rgba(59,130,246,0.1)] light:bg-gray-50/50 light:border-gray-200 light:hover:bg-gray-100/70 light:hover:border-blue-400 light:hover:shadow-[0_0_20px_rgba(59,130,246,0.05)] transition-all duration-300 overflow-hidden"
          >
            <!-- Badge -->
            <div class="absolute top-3 right-3">
              <span class="text-[10px] bg-blue-500/10 text-blue-400 border border-blue-500/20 px-2 py-0.5 rounded-full font-medium light:bg-blue-50 light:text-blue-600 light:border-blue-200">
                {{ $t('accounts.classicRecommend') }}
              </span>
            </div>

            <div class="h-10 w-10 rounded-lg bg-blue-500/10 light:bg-blue-100/60 flex items-center justify-center mb-4 group-hover:scale-110 transition-transform duration-300">
              <Network class="h-5 w-5 text-blue-400 light:text-blue-600" />
            </div>

            <h3 class="text-base font-semibold text-white light:text-gray-900 group-hover:text-blue-400 light:group-hover:text-blue-600 transition-colors duration-200">
              {{ $t('accounts.classicTitle') }}
            </h3>
            <p class="text-xs text-gray-400 light:text-gray-600 mt-2 flex-grow leading-relaxed">
              {{ $t('accounts.classicDesc') }}
            </p>

            <div class="mt-5 flex items-center justify-between text-xs font-medium text-blue-400 light:text-blue-600">
              <span>{{ $t('accounts.selectMode') }}</span>
              <span class="group-hover:translate-x-1 transition-transform duration-200">→</span>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>
