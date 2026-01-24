<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { api } from '@/services/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { toast } from 'vue-sonner'
import { ShieldCheck, Settings2, ExternalLink, Info, Copy, Check } from 'lucide-vue-next'

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
  toast.success('Redirect URL copied to clipboard')
  setTimeout(() => {
    copiedRedirectUrl.value = false
  }, 2000)
}

async function fetchProviders() {
  isLoading.value = true
  try {
    const response = await api.get('/settings/sso')
    providers.value = response.data.data || []
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to load SSO settings')
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
    toast.error('Client ID is required')
    return
  }

  // For new providers, require client secret
  const existing = providers.value.find(p => p.provider === editingProvider.value)
  if (!existing && !editForm.value.client_secret.trim()) {
    toast.error('Client Secret is required for new providers')
    return
  }

  // For custom providers, validate URLs
  if (editingProvider.value === 'custom') {
    if (!editForm.value.auth_url || !editForm.value.token_url || !editForm.value.user_info_url) {
      toast.error('Auth URL, Token URL, and User Info URL are required for custom providers')
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
    toast.success('SSO provider saved successfully')
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to save SSO provider')
  } finally {
    isSaving.value = false
  }
}

async function deleteProvider(providerKey: string) {
  try {
    await api.delete(`/settings/sso/${providerKey}`)
    await fetchProviders()
    toast.success('SSO provider removed')
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to remove SSO provider')
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
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <!-- Header -->
    <header class="border-b border-white/[0.08] light:border-gray-200 bg-[#0a0a0b]/95 light:bg-white/95 backdrop-blur">
      <div class="flex h-16 items-center px-6">
        <div class="h-8 w-8 rounded-lg bg-gradient-to-br from-emerald-500 to-teal-600 flex items-center justify-center mr-3 shadow-lg shadow-emerald-500/20">
          <ShieldCheck class="h-4 w-4 text-white" />
        </div>
        <div class="flex-1">
          <h1 class="text-xl font-semibold text-white light:text-gray-900">Single Sign-On (SSO)</h1>
          <p class="text-sm text-white/50 light:text-gray-500">Configure SSO providers for your organization</p>
        </div>
      </div>
    </header>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto space-y-6">
        <!-- Info Card -->
        <Card class="bg-blue-950/30 light:bg-blue-50 border-blue-800 light:border-blue-200">
          <CardContent class="flex items-start gap-3 pt-6">
            <Info class="h-5 w-5 text-blue-400 light:text-blue-600 shrink-0 mt-0.5" />
            <div class="text-sm text-blue-200 light:text-blue-800">
              <p class="font-medium mb-1">SSO Configuration</p>
              <p class="text-blue-300 light:text-blue-700">
                Configure OAuth providers to allow users to sign in with their existing accounts.
                Enabled providers will appear as login options on the sign-in page.
              </p>
            </div>
          </CardContent>
        </Card>

        <!-- Provider Cards -->
        <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          <Card
            v-for="(config, key) in providerConfigs"
            :key="key"
            :class="'relative overflow-hidden transition-all hover:shadow-md' + (getConfiguredProvider(key)?.is_enabled ? ' ring-2 ring-primary' : '')"
          >
            <CardHeader class="pb-3">
              <div class="flex items-start justify-between">
                <div class="flex items-center gap-3">
                  <div class="h-10 w-10 rounded-lg bg-muted flex items-center justify-center">
                    <svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
                      <path :d="config.icon" />
                    </svg>
                  </div>
                  <div>
                    <CardTitle class="text-base">{{ config.name }}</CardTitle>
                    <CardDescription class="text-xs mt-0.5">
                      {{ config.description }}
                    </CardDescription>
                  </div>
                </div>
                <Badge
                  v-if="getConfiguredProvider(key)"
                  :variant="getConfiguredProvider(key)?.is_enabled ? 'default' : 'secondary'"
                >
                  {{ getConfiguredProvider(key)?.is_enabled ? 'Enabled' : 'Disabled' }}
                </Badge>
              </div>
            </CardHeader>
            <CardContent class="space-y-3">
              <div v-if="getConfiguredProvider(key)" class="text-xs text-muted-foreground space-y-1">
                <p>
                  <span class="font-medium">Auto-create users:</span>
                  {{ getConfiguredProvider(key)?.allow_auto_create ? 'Yes' : 'No' }}
                </p>
                <p v-if="getConfiguredProvider(key)?.allowed_domains">
                  <span class="font-medium">Allowed domains:</span>
                  {{ getConfiguredProvider(key)?.allowed_domains }}
                </p>
              </div>
              <div class="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  class="flex-1"
                  @click="openEditDialog(key)"
                >
                  <Settings2 class="h-4 w-4 mr-2" />
                  {{ getConfiguredProvider(key) ? 'Configure' : 'Set Up' }}
                </Button>
                <Button
                  v-if="config.docUrl"
                  variant="ghost"
                  size="icon"
                  class="shrink-0"
                  as="a"
                  :href="config.docUrl"
                  target="_blank"
                >
                  <ExternalLink class="h-4 w-4" />
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
        </div>
      </div>
    </ScrollArea>

    <!-- Edit Dialog -->
    <Dialog v-model:open="isEditDialogOpen">
      <DialogContent class="max-w-lg">
        <DialogHeader>
          <DialogTitle class="flex items-center gap-2">
            <svg v-if="currentProviderConfig" class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
              <path :d="currentProviderConfig.icon" />
            </svg>
            Configure {{ currentProviderConfig?.name }}
          </DialogTitle>
          <DialogDescription>
            Enter your OAuth credentials from {{ currentProviderConfig?.name }}.
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-4 max-h-[60vh] overflow-y-auto">
          <!-- Redirect URL -->
          <div class="space-y-2">
            <Label>Redirect URL</Label>
            <p class="text-xs text-muted-foreground mb-1">
              Add this URL to your {{ currentProviderConfig?.name }} OAuth app settings
            </p>
            <div class="flex gap-2">
              <Input
                :model-value="redirectUrl"
                readonly
                class="font-mono text-xs bg-muted"
              />
              <Button
                variant="outline"
                size="icon"
                class="shrink-0"
                @click="copyRedirectUrl"
              >
                <Check v-if="copiedRedirectUrl" class="h-4 w-4 text-green-500" />
                <Copy v-else class="h-4 w-4" />
              </Button>
            </div>
          </div>

          <!-- Client ID -->
          <div class="space-y-2">
            <Label for="client_id">Client ID</Label>
            <Input
              id="client_id"
              v-model="editForm.client_id"
              placeholder="Enter client ID"
            />
          </div>

          <!-- Client Secret -->
          <div class="space-y-2">
            <Label for="client_secret">
              Client Secret
              <span v-if="getConfiguredProvider(editingProvider)?.has_secret" class="text-xs text-muted-foreground ml-1">
                (leave blank to keep existing)
              </span>
            </Label>
            <Input
              id="client_secret"
              v-model="editForm.client_secret"
              type="password"
              placeholder="Enter client secret"
            />
          </div>

          <!-- Custom Provider URLs -->
          <template v-if="editingProvider === 'custom'">
            <div class="space-y-2">
              <Label for="auth_url">Authorization URL</Label>
              <Input
                id="auth_url"
                v-model="editForm.auth_url"
                placeholder="https://provider.com/oauth/authorize"
              />
            </div>
            <div class="space-y-2">
              <Label for="token_url">Token URL</Label>
              <Input
                id="token_url"
                v-model="editForm.token_url"
                placeholder="https://provider.com/oauth/token"
              />
            </div>
            <div class="space-y-2">
              <Label for="user_info_url">User Info URL</Label>
              <Input
                id="user_info_url"
                v-model="editForm.user_info_url"
                placeholder="https://provider.com/userinfo"
              />
            </div>
          </template>

          <div class="border-t pt-4 space-y-4">
            <!-- Enable Toggle -->
            <div class="flex items-center justify-between">
              <div>
                <Label>Enable Provider</Label>
                <p class="text-xs text-muted-foreground">Allow users to sign in with this provider</p>
              </div>
              <Switch v-model:checked="editForm.is_enabled" />
            </div>

            <!-- Auto-create Toggle -->
            <div class="flex items-center justify-between">
              <div>
                <Label>Auto-create Users</Label>
                <p class="text-xs text-muted-foreground">Automatically create accounts for new SSO users</p>
              </div>
              <Switch v-model:checked="editForm.allow_auto_create" />
            </div>

            <!-- Default Role -->
            <div v-if="editForm.allow_auto_create" class="space-y-2">
              <Label>Default Role for New Users</Label>
              <Select v-model="editForm.default_role">
                <SelectTrigger>
                  <SelectValue placeholder="Select role" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="agent">Agent</SelectItem>
                  <SelectItem value="manager">Manager</SelectItem>
                  <SelectItem value="admin">Admin</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <!-- Allowed Domains -->
            <div class="space-y-2">
              <Label for="allowed_domains">Allowed Email Domains (optional)</Label>
              <Input
                id="allowed_domains"
                v-model="editForm.allowed_domains"
                placeholder="example.com, company.org"
              />
              <p class="text-xs text-muted-foreground">
                Comma-separated list. Leave empty to allow all domains.
              </p>
            </div>
          </div>
        </div>

        <DialogFooter class="flex gap-2">
          <Button
            v-if="getConfiguredProvider(editingProvider)"
            variant="destructive"
            size="sm"
            @click="deleteProvider(editingProvider); isEditDialogOpen = false"
          >
            Remove
          </Button>
          <div class="flex-1" />
          <Button variant="outline" size="sm" @click="isEditDialogOpen = false">
            Cancel
          </Button>
          <Button size="sm" @click="saveProvider" :disabled="isSaving">
            {{ isSaving ? 'Saving...' : 'Save' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
