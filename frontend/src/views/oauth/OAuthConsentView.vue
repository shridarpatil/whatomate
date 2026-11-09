<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Bot, Check, Loader2, ShieldAlert } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { mcpService, usersService, type OAuthClientInfo } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { getErrorMessage, unwrapResponse } from '@/lib/api-utils'
import {
  buildConsentBody,
  navigateTo,
  pickInitialOrganization,
  readConsentParams,
  type ConsentOrganization,
} from './consent'

const { t } = useI18n()
const route = useRoute()
const authStore = useAuthStore()

const params = readConsentParams(route.query)
const isLoading = ref(true)
const errorMessage = ref('')
const clientName = ref('')
const organizations = ref<ConsentOrganization[]>([])
const organizationId = ref('')
const submitting = ref<'approve' | 'deny' | null>(null)

const appName = computed(() => clientName.value || t('oauthConsent.unknownApp'))
const userEmail = computed(() => authStore.user?.email || '')
const selectedOrgName = computed(
  () => organizations.value.find(o => o.organization_id === organizationId.value)?.name || '',
)

async function loadOrganizations() {
  try {
    const data = unwrapResponse<{ organizations?: ConsentOrganization[] }>(await usersService.listMyOrganizations())
    organizations.value = data?.organizations || []
  } catch {
    organizations.value = []
  }
  if (organizations.value.length === 0 && authStore.organizationId) {
    organizations.value = [{ organization_id: authStore.organizationId, name: '' }]
  }
  organizationId.value = pickInitialOrganization(organizations.value, [
    localStorage.getItem('selected_organization_id'),
    authStore.organizationId,
  ])
}

async function load() {
  if (!params) {
    errorMessage.value = t('oauthConsent.invalidRequest')
    isLoading.value = false
    return
  }
  try {
    const [clientRes] = await Promise.all([
      mcpService.getOAuthClient({ client_id: params.client_id, redirect_uri: params.redirect_uri }),
      loadOrganizations(),
    ])
    const client = unwrapResponse<OAuthClientInfo>(clientRes)
    clientName.value = client?.client_name || ''
    if (!client?.redirect_uri_valid) errorMessage.value = t('oauthConsent.invalidClient')
    else if (!organizationId.value) errorMessage.value = t('oauthConsent.noOrganization')
  } catch (err: any) {
    const status = err?.response?.status
    errorMessage.value = status && status < 500 ? t('oauthConsent.invalidClient') : t('oauthConsent.loadFailed')
  } finally {
    isLoading.value = false
  }
}

async function submit(approve: boolean) {
  if (!params || submitting.value) return
  submitting.value = approve ? 'approve' : 'deny'
  try {
    const res = await mcpService.submitConsent(buildConsentBody(params, organizationId.value, approve))
    const url = unwrapResponse<{ redirect_url?: string }>(res)?.redirect_url
    if (!url) throw new Error('missing redirect_url')
    navigateTo(url)
  } catch (err) {
    toast.error(getErrorMessage(err, t('oauthConsent.submitFailed')))
    submitting.value = null
  }
}

onMounted(load)
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-[#0a0a0b] light:bg-gradient-to-br light:from-gray-50 light:to-gray-100 p-4">
    <div class="w-full max-w-md rounded-2xl border border-white/[0.08] bg-white/[0.02] backdrop-blur light:bg-white light:border-gray-200 light:shadow-xl">
      <div v-if="isLoading" class="p-10 flex justify-center" data-testid="consent-loading">
        <Loader2 class="h-6 w-6 animate-spin text-white/50 light:text-gray-400" />
      </div>

      <div v-else-if="errorMessage" class="p-8 text-center space-y-3" data-testid="consent-error">
        <div class="flex justify-center">
          <div class="h-12 w-12 rounded-xl bg-red-500/15 flex items-center justify-center">
            <ShieldAlert class="h-7 w-7 text-red-400 light:text-red-600" />
          </div>
        </div>
        <h2 class="text-xl font-bold text-white light:text-gray-900">{{ $t('oauthConsent.invalidTitle') }}</h2>
        <p class="text-white/60 light:text-gray-600">{{ errorMessage }}</p>
      </div>

      <template v-else>
        <div class="p-8 pb-4 space-y-2 text-center">
          <div class="flex justify-center mb-4">
            <div class="h-12 w-12 rounded-xl bg-gradient-to-br from-emerald-500 to-green-600 flex items-center justify-center shadow-lg shadow-emerald-500/20">
              <Bot class="h-7 w-7 text-white" />
            </div>
          </div>
          <h2 class="text-2xl font-bold text-white light:text-gray-900" data-testid="consent-app">{{ appName }}</h2>
          <p class="text-white/60 light:text-gray-600">
            {{ $t('oauthConsent.wantsAccess', { app: appName, email: userEmail }) }}
          </p>
        </div>

        <div class="px-8 pb-4 space-y-4">
          <div v-if="organizations.length > 1" class="space-y-2">
            <Label for="consent-org" class="text-white/70 light:text-gray-700">{{ $t('oauthConsent.organisation') }}</Label>
            <select
              id="consent-org"
              v-model="organizationId"
              :disabled="!!submitting"
              class="w-full h-10 rounded-md border border-white/[0.1] bg-white/[0.04] px-3 text-sm text-white light:bg-white light:border-gray-300 light:text-gray-900"
            >
              <option v-for="org in organizations" :key="org.organization_id" :value="org.organization_id">{{ org.name }}</option>
            </select>
          </div>
          <p v-else-if="selectedOrgName" class="text-sm text-white/60 light:text-gray-600">
            {{ $t('oauthConsent.organisation') }}: <span class="font-medium text-white light:text-gray-900">{{ selectedOrgName }}</span>
          </p>

          <div class="rounded-lg border border-white/[0.08] bg-white/[0.02] p-4 light:border-gray-200 light:bg-gray-50">
            <p class="text-sm font-medium text-white/80 light:text-gray-800 mb-2">{{ $t('oauthConsent.canDo') }}</p>
            <ul class="space-y-2 text-sm text-white/60 light:text-gray-600">
              <li v-for="key in ['bulletRole', 'bulletManage', 'bulletSend', 'bulletDisconnect']" :key="key" class="flex gap-2">
                <Check class="h-4 w-4 mt-0.5 shrink-0 text-emerald-400 light:text-emerald-600" />
                <span>{{ $t(`oauthConsent.${key}`) }}</span>
              </li>
            </ul>
          </div>
        </div>

        <div class="px-8 pb-8 flex gap-3">
          <Button variant="outline" class="flex-1" :disabled="!!submitting" data-testid="consent-deny" @click="submit(false)">
            <Loader2 v-if="submitting === 'deny'" class="mr-2 h-4 w-4 animate-spin" />
            {{ $t('oauthConsent.deny') }}
          </Button>
          <Button
            class="flex-1 bg-gradient-to-r from-emerald-500 to-green-600 hover:from-emerald-600 hover:to-green-700 text-white"
            :disabled="!!submitting"
            data-testid="consent-approve"
            @click="submit(true)"
          >
            <Loader2 v-if="submitting === 'approve'" class="mr-2 h-4 w-4 animate-spin" />
            {{ $t('oauthConsent.approve') }}
          </Button>
        </div>
      </template>
    </div>
  </div>
</template>
