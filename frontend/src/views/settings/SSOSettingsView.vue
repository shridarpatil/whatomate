<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/services/api'
import { PageHeader, ConfirmDialog, ErrorState } from '@/components/shared'
import Modal from '@/components/oat/Modal.vue'
import { toast } from '@/lib/toast'
import { ShieldCheck, Settings2, ExternalLink, Info, Copy, Check } from '@/lib/icons'
import { getErrorMessage } from '@/lib/api-utils'

const { t } = useI18n()

interface SSOProvider {
  provider: string
  client_id: string
  has_secret: boolean
  is_enabled: boolean
  allow_auto_create: boolean
  default_role: string
  allowed_domains: string
  auth_url?: string
  token_url?: string
  user_info_url?: string
}

interface ProviderConfig {
  name: string
  description: string
  icon: string
  docUrl: string
  isCustom?: boolean
}

const providerConfigs: Record<string, ProviderConfig> = {
  google: {
    name: 'Google',
    description: 'Login with Google accounts',
    icon: 'M12.545,10.239v3.821h5.445c-0.712,2.315-2.647,3.972-5.445,3.972c-3.332,0-6.033-2.701-6.033-6.032s2.701-6.032,6.033-6.032c1.498,0,2.866,0.549,3.921,1.453l2.814-2.814C17.503,2.988,15.139,2,12.545,2C7.021,2,2.543,6.477,2.543,12s4.478,10,10.002,10c8.396,0,10.249-7.85,9.426-11.748L12.545,10.239z',
    docUrl: 'https://console.cloud.google.com/apis/credentials'
  },
  microsoft: {
    name: 'Microsoft',
    description: 'Login with Microsoft work/school accounts',
    icon: 'M11 11H3V3h8v8zm10 0h-8V3h8v8zM11 21H3v-8h8v8zm10 0h-8v-8h8v8z',
    docUrl: 'https://portal.azure.com/#blade/Microsoft_AAD_RegisteredApps'
  },
  github: {
    name: 'GitHub',
    description: 'Login with GitHub accounts',
    icon: 'M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z',
    docUrl: 'https://github.com/settings/developers'
  },
  facebook: {
    name: 'Facebook',
    description: 'Login with Facebook accounts',
    icon: 'M24 12.073c0-6.627-5.373-12-12-12s-12 5.373-12 12c0 5.99 4.388 10.954 10.125 11.854v-8.385H7.078v-3.47h3.047V9.43c0-3.007 1.792-4.669 4.533-4.669 1.312 0 2.686.235 2.686.235v2.953H15.83c-1.491 0-1.956.925-1.956 1.874v2.25h3.328l-.532 3.47h-2.796v8.385C19.612 23.027 24 18.062 24 12.073z',
    docUrl: 'https://developers.facebook.com/apps'
  },
  custom: {
    name: 'Custom OIDC',
    description: 'Configure a custom OAuth2/OIDC provider',
    icon: 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm-1-13h2v6h-2zm0 8h2v2h-2z',
    docUrl: '',
    isCustom: true
  }
}

const providers = ref<SSOProvider[]>([])
const isLoading = ref(false)
const isSaving = ref(false)
const fetchError = ref(false)

// Delete provider confirmation
const deleteDialogOpen = ref(false)
const providerToDelete = ref<string>('')
const isDeleting = ref(false)

// Edit dialog
const isEditDialogOpen = ref(false)
const editingProvider = ref<string>('')
const editForm = ref({
  client_id: '',
  client_secret: '',
  is_enabled: false,
  allow_auto_create: false,
  default_role: 'agent',
  allowed_domains: '',
  auth_url: '',
  token_url: '',
  user_info_url: ''
})

const currentProviderConfig = computed(() => providerConfigs[editingProvider.value])

// Generate redirect URL for OAuth provider configuration
const redirectUrl = computed(() => {
  const baseUrl = window.location.origin
  const basePath = ((window as any).__BASE_PATH__ ?? import.meta.env.BASE_URL ?? '').replace(/\/$/, '')
  return `${baseUrl}${basePath}/api/auth/sso/${editingProvider.value}/callback`
})

const copiedRedirectUrl = ref(false)
function copyRedirectUrl() {
  navigator.clipboard.writeText(redirectUrl.value)
  copiedRedirectUrl.value = true
  toast.success(t('common.copiedToClipboard'))
  setTimeout(() => {
    copiedRedirectUrl.value = false
  }, 2000)
}

async function fetchProviders() {
  isLoading.value = true
  fetchError.value = false
  try {
    const response = await api.get('/settings/sso')
    providers.value = response.data.data || []
  } catch (error) {
    fetchError.value = true
    toast.error(getErrorMessage(error, t('common.failedLoad', { resource: t('resources.ssoProviders') })))
  } finally {
    isLoading.value = false
  }
}

function openEditDialog(providerKey: string) {
  editingProvider.value = providerKey
  const existing = providers.value.find(p => p.provider === providerKey)

  editForm.value = {
    client_id: existing?.client_id || '',
    client_secret: '',
    is_enabled: existing?.is_enabled || false,
    allow_auto_create: existing?.allow_auto_create || false,
    default_role: existing?.default_role || 'agent',
    allowed_domains: existing?.allowed_domains || '',
    auth_url: existing?.auth_url || '',
    token_url: existing?.token_url || '',
    user_info_url: existing?.user_info_url || ''
  }

  isEditDialogOpen.value = true
}

async function saveProvider() {
  if (!editForm.value.client_id.trim()) {
    toast.error(t('sso.clientIdRequired'))
    return
  }

  // For new providers, require client secret
  const existing = providers.value.find(p => p.provider === editingProvider.value)
  if (!existing && !editForm.value.client_secret.trim()) {
    toast.error(t('sso.clientSecretRequired'))
    return
  }

  // For custom providers, validate URLs
  if (editingProvider.value === 'custom') {
    if (!editForm.value.auth_url || !editForm.value.token_url || !editForm.value.user_info_url) {
      toast.error(t('sso.customUrlsRequired'))
      return
    }
  }

  isSaving.value = true
  try {
    const payload: Record<string, any> = {
      client_id: editForm.value.client_id.trim(),
      is_enabled: editForm.value.is_enabled,
      allow_auto_create: editForm.value.allow_auto_create,
      default_role: editForm.value.default_role,
      allowed_domains: editForm.value.allowed_domains.trim()
    }

    // Only send secret if provided
    if (editForm.value.client_secret.trim()) {
      payload.client_secret = editForm.value.client_secret.trim()
    }

    // Custom provider fields
    if (editingProvider.value === 'custom') {
      payload.auth_url = editForm.value.auth_url.trim()
      payload.token_url = editForm.value.token_url.trim()
      payload.user_info_url = editForm.value.user_info_url.trim()
    }

    await api.put(`/settings/sso/${editingProvider.value}`, payload)
    await fetchProviders()
    isEditDialogOpen.value = false
    toast.success(t('common.savedSuccess', { resource: t('resources.SSOProvider') }))
  } catch (error) {
    toast.error(getErrorMessage(error, t('common.failedSave', { resource: t('resources.SSOProvider') })))
  } finally {
    isSaving.value = false
  }
}

function openDeleteDialog(providerKey: string) {
  providerToDelete.value = providerKey
  deleteDialogOpen.value = true
}

async function confirmDeleteProvider() {
  if (!providerToDelete.value) return
  isDeleting.value = true
  try {
    await api.delete(`/settings/sso/${providerToDelete.value}`)
    await fetchProviders()
    toast.success(t('common.deletedSuccess', { resource: t('resources.SSOProvider') }))
    deleteDialogOpen.value = false
    providerToDelete.value = ''
  } catch (error) {
    toast.error(getErrorMessage(error, t('common.failedDelete', { resource: t('resources.SSOProvider') })))
  } finally {
    isDeleting.value = false
  }
}

function getConfiguredProvider(providerKey: string): SSOProvider | undefined {
  return providers.value.find(p => p.provider === providerKey)
}

onMounted(() => {
  fetchProviders()
})
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader :title="$t('sso.title')" :description="$t('sso.subtitle')" :icon="ShieldCheck" />

    <ErrorState
      v-if="fetchError && !isLoading"
      :title="$t('sso.fetchErrorTitle')"
      :description="$t('sso.fetchErrorDescription')"
      :retry-label="$t('common.retry')"
      class="flex-1"
      @retry="fetchProviders"
    />

    <div v-else class="flex-1 scroll-y p-4">
      <div class="container vstack gap-6">
        <!-- Info Card -->
        <div role="alert">
          <strong class="hstack gap-2"><Info :size="16" /> {{ $t('sso.configuration') }}</strong>
          {{ $t('sso.configurationDesc') }}
        </div>

        <!-- Provider Cards -->
        <div class="row">
          <div v-for="(config, key) in providerConfigs" :key="key" class="col-4">
            <article class="card">
              <header class="hstack justify-between">
                <div class="hstack gap-2">
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                    <path :d="config.icon" />
                  </svg>
                  <div>
                    <h3>{{ config.name }}</h3>
                    <small class="text-light">{{ config.description }}</small>
                  </div>
                </div>
                <span
                  v-if="getConfiguredProvider(key)"
                  class="badge"
                  :data-variant="getConfiguredProvider(key)?.is_enabled ? 'success' : 'secondary'"
                >
                  {{ getConfiguredProvider(key)?.is_enabled ? $t('sso.enabled') : $t('sso.disabled') }}
                </span>
              </header>
              <div v-if="getConfiguredProvider(key)" class="vstack gap-1">
                <small class="text-light">
                  <strong>{{ $t('sso.autoCreateUsers') }}:</strong>
                  {{ getConfiguredProvider(key)?.allow_auto_create ? $t('sso.yes') : $t('sso.no') }}
                </small>
                <small v-if="getConfiguredProvider(key)?.allowed_domains" class="text-light">
                  <strong>{{ $t('sso.allowedDomains') }}:</strong>
                  {{ getConfiguredProvider(key)?.allowed_domains }}
                </small>
              </div>
              <footer class="hstack gap-2">
                <button type="button" class="outline small flex-1" @click="openEditDialog(key)">
                  <Settings2 :size="16" />
                  {{ getConfiguredProvider(key) ? $t('sso.configure') : $t('sso.setUp') }}
                </button>
                <a
                  v-if="config.docUrl"
                  class="button ghost icon small"
                  :href="config.docUrl"
                  target="_blank"
                  rel="noopener noreferrer"
                  :title="config.name"
                  :aria-label="config.name"
                >
                  <ExternalLink :size="16" />
                </a>
              </footer>
            </article>
          </div>
        </div>
      </div>
    </div>

    <!-- Edit Dialog -->
    <Modal
      v-model:open="isEditDialogOpen"
      :title="`${$t('sso.configure')} ${currentProviderConfig?.name ?? ''}`"
      :description="$t('sso.enterCredentials', { provider: currentProviderConfig?.name })"
    >
      <div class="vstack gap-4">
        <!-- Redirect URL -->
        <div data-field>
          <label for="redirect_url">{{ $t('sso.redirectUrl') }}</label>
          <div class="hstack gap-2">
            <input id="redirect_url" :value="redirectUrl" readonly class="flex-1" />
            <button
              type="button"
              class="outline icon"
              :title="$t('common.copy', 'Copy')"
              :aria-label="$t('common.copy', 'Copy')"
              @click="copyRedirectUrl"
            >
              <Check v-if="copiedRedirectUrl" :size="16" />
              <Copy v-else :size="16" />
            </button>
          </div>
          <small data-hint>{{ $t('sso.redirectUrlHint', { provider: currentProviderConfig?.name }) }}</small>
        </div>

        <!-- Client ID -->
        <label data-field>
          {{ $t('sso.clientId') }}
          <input id="client_id" v-model="editForm.client_id" :placeholder="$t('sso.clientIdPlaceholder')" />
        </label>

        <!-- Client Secret -->
        <label data-field>
          <span>
            {{ $t('sso.clientSecret') }}
            <small v-if="getConfiguredProvider(editingProvider)?.has_secret" class="text-light">
              {{ $t('sso.clientSecretKeepExisting') }}
            </small>
          </span>
          <input id="client_secret" v-model="editForm.client_secret" type="password" :placeholder="$t('sso.clientSecretPlaceholder')" />
        </label>

        <!-- Custom Provider URLs -->
        <template v-if="editingProvider === 'custom'">
          <label data-field>
            {{ $t('sso.authUrl') }}
            <input id="auth_url" v-model="editForm.auth_url" :placeholder="$t('sso.authUrlPlaceholder')" />
          </label>
          <label data-field>
            {{ $t('sso.tokenUrl') }}
            <input id="token_url" v-model="editForm.token_url" :placeholder="$t('sso.tokenUrlPlaceholder')" />
          </label>
          <label data-field>
            {{ $t('sso.userInfoUrl') }}
            <input id="user_info_url" v-model="editForm.user_info_url" :placeholder="$t('sso.userInfoUrlPlaceholder')" />
          </label>
        </template>

        <hr>

        <!-- Enable Toggle -->
        <label data-field>
          <span class="hstack justify-between">
            <span>{{ $t('sso.enableProvider') }}</span>
            <input type="checkbox" role="switch" v-model="editForm.is_enabled" />
          </span>
          <small data-hint>{{ $t('sso.enableProviderDesc') }}</small>
        </label>

        <!-- Auto-create Toggle -->
        <label data-field>
          <span class="hstack justify-between">
            <span>{{ $t('sso.autoCreateUsersLabel') }}</span>
            <input type="checkbox" role="switch" v-model="editForm.allow_auto_create" />
          </span>
          <small data-hint>{{ $t('sso.autoCreateUsersDesc') }}</small>
        </label>

        <!-- Default Role -->
        <label v-if="editForm.allow_auto_create" data-field>
          {{ $t('sso.defaultRole') }}
          <select v-model="editForm.default_role">
            <option value="agent">{{ $t('sso.agent') }}</option>
            <option value="manager">{{ $t('sso.manager') }}</option>
            <option value="admin">{{ $t('sso.admin') }}</option>
          </select>
        </label>

        <!-- Allowed Domains -->
        <label data-field>
          {{ $t('sso.allowedEmailDomains') }}
          <input id="allowed_domains" v-model="editForm.allowed_domains" :placeholder="$t('sso.allowedDomainsPlaceholder')" />
          <small data-hint>{{ $t('sso.allowedDomainsHint') }}</small>
        </label>
      </div>

      <template #footer>
        <button
          v-if="getConfiguredProvider(editingProvider)"
          type="button"
          data-variant="danger"
          @click="openDeleteDialog(editingProvider); isEditDialogOpen = false"
        >
          {{ $t('sso.remove') }}
        </button>
        <div class="flex-1" />
        <button type="button" class="outline" @click="isEditDialogOpen = false">
          {{ $t('common.cancel') }}
        </button>
        <button type="button" :aria-busy="isSaving" :disabled="isSaving" @click="saveProvider">
          {{ $t('common.save') }}
        </button>
      </template>
    </Modal>

    <!-- Delete SSO Provider Confirmation -->
    <ConfirmDialog
      v-model:open="deleteDialogOpen"
      :title="$t('sso.confirmDeleteTitle')"
      :description="$t('sso.confirmDeleteDescription', { provider: providerConfigs[providerToDelete]?.name || '' })"
      :confirm-label="$t('common.remove')"
      variant="destructive"
      :is-submitting="isDeleting"
      @confirm="confirmDeleteProvider"
    />
  </div>
</template>
