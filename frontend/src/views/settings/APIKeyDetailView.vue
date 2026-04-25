<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { apiKeysService } from '@/services/api'
import { toast } from 'vue-sonner'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDateTime } from '@/lib/utils'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Switch } from '@/components/ui/switch'
import { Key, Trash2, Save, Copy, AlertTriangle } from 'lucide-vue-next'
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
  if (isExpired.value) return 'destructive'
  return 'default'
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
      icon-gradient="bg-gradient-to-br from-amber-500 to-orange-600 shadow-amber-500/20"
      back-link="/settings/api-keys"
      :breadcrumbs="breadcrumbs"
      :is-loading="isLoading"
      :is-not-found="isNotFound"
      :not-found-title="$t('apiKeys.notFound', 'API Key not found')"
    >
      <template #actions>
        <div class="flex items-center gap-2">
          <Button v-if="isNew && canWrite" size="sm" @click="create" :disabled="isSaving">
            <Save class="h-4 w-4 mr-1" /> {{ isSaving ? $t('common.saving', 'Saving...') : $t('common.create') }}
          </Button>
          <Button v-if="!isNew && canDelete" variant="destructive" size="sm" @click="deleteDialogOpen = true">
            <Trash2 class="h-4 w-4 mr-1" /> {{ $t('common.delete') }}
          </Button>
        </div>
      </template>

      <!-- Create form -->
      <Card v-if="isNew">
        <CardHeader class="pb-3">
          <CardTitle class="text-sm font-medium">{{ $t('teams.details', 'Details') }}</CardTitle>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('apiKeys.name') }} <span class="text-destructive">*</span></Label>
            <Input v-model="form.name" :placeholder="$t('apiKeys.namePlaceholder')" />
          </div>
          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('apiKeys.expiration') }}</Label>
            <Input v-model="form.expires_at" type="datetime-local" />
            <p class="text-xs text-muted-foreground">{{ $t('apiKeys.expirationHint') }}</p>
          </div>
        </CardContent>
      </Card>

      <!-- View existing key -->
      <Card v-else>
        <CardHeader class="pb-3">
          <div class="flex items-center justify-between">
            <CardTitle class="text-sm font-medium">{{ $t('teams.details', 'Details') }}</CardTitle>
            <Badge :variant="statusVariant">{{ statusLabel }}</Badge>
          </div>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="flex items-center gap-3 p-3 rounded-lg bg-muted/50">
            <div class="h-10 w-10 rounded-full bg-gradient-to-br from-amber-500 to-orange-600 flex items-center justify-center shrink-0">
              <Key class="h-5 w-5 text-white" />
            </div>
            <div class="min-w-0">
              <p class="font-medium truncate">{{ apiKey?.name }}</p>
              <code class="text-xs text-muted-foreground font-mono bg-muted px-1.5 py-0.5 rounded">whm_{{ apiKey?.key_prefix }}...</code>
            </div>
          </div>

          <div class="grid gap-4">
            <div class="space-y-1">
              <p class="text-xs text-muted-foreground">{{ $t('common.createdAt', 'Created') }}</p>
              <p class="text-sm">{{ apiKey?.created_at ? formatDateTime(apiKey.created_at) : '—' }}</p>
            </div>
            <div class="space-y-1">
              <p class="text-xs text-muted-foreground">{{ $t('apiKeys.lastUsedAt', 'Last Used') }}</p>
              <p class="text-sm">{{ apiKey?.last_used_at ? formatDateTime(apiKey.last_used_at) : '—' }}</p>
            </div>
            <div class="space-y-1">
              <p class="text-xs text-muted-foreground">{{ $t('apiKeys.expiresAt', 'Expires') }}</p>
              <div class="flex items-center gap-2">
                <p class="text-sm">{{ apiKey?.expires_at ? formatDateTime(apiKey.expires_at) : $t('apiKeys.never', 'Never') }}</p>
                <Badge v-if="isExpired" variant="destructive" class="text-xs">{{ $t('apiKeys.expired', 'Expired') }}</Badge>
              </div>
            </div>
          </div>
          <div v-if="canWrite" class="flex items-center justify-between border-t pt-4">
            <Label class="text-xs font-normal cursor-pointer">{{ $t('common.active') }}</Label>
            <Switch
              :checked="apiKey?.is_active ?? false"
              :disabled="isExpired"
              @update:checked="toggleActive"
            />
          </div>
        </CardContent>
      </Card>

      <AuditLogPanel v-if="apiKey && !isNew" resource-type="api_key" :resource-id="apiKey.id" />

      <template v-if="!isNew" #sidebar>
        <MetadataPanel :created-at="apiKey?.created_at" :updated-at="apiKey?.updated_at" />
      </template>
    </DetailPageLayout>

    <!-- Key created dialog (shown once after create) -->
    <Dialog v-model:open="isKeyDisplayOpen">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{{ $t('apiKeys.apiKeyCreated') }}</DialogTitle>
          <DialogDescription>
            <div class="flex items-center gap-2 text-amber-600 mt-2">
              <AlertTriangle class="h-4 w-4" />
              <span>{{ $t('apiKeys.apiKeyCreatedWarning') }}</span>
            </div>
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-4 py-4">
          <div class="space-y-2">
            <Label>{{ $t('apiKeys.yourApiKey') }}</Label>
            <div class="flex gap-2">
              <Input :model-value="newlyCreatedKey?.key" readonly class="font-mono text-sm" />
              <IconButton :icon="Copy" :label="$t('apiKeys.copyApiKey')" variant="outline" @click="copyToClipboard(newlyCreatedKey?.key || '')" />
            </div>
          </div>
          <div class="bg-muted p-3 rounded-lg text-sm">
            <p class="font-medium mb-1">{{ $t('apiKeys.usage') }}:</p>
            <code class="text-xs">curl -H "X-API-Key: {{ newlyCreatedKey?.key }}" https://your-api.com/api/contacts</code>
          </div>
        </div>
        <DialogFooter>
          <Button size="sm" @click="isKeyDisplayOpen = false; if (createdKeyId) router.replace(`/settings/api-keys/${createdKeyId}`)">{{ $t('common.done') }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('apiKeys.deleteKey', 'Delete API Key') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('apiKeys.deleteWarning', 'Are you sure you want to delete this API key? Any applications using this key will immediately lose access. This action cannot be undone.') }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('common.cancel') }}</AlertDialogCancel>
          <AlertDialogAction @click="deleteApiKey">{{ $t('common.delete') }}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
