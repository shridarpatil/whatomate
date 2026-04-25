<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { webhooksService, type Webhook, type WebhookEvent } from '@/services/api'
import { toast } from 'vue-sonner'
import { getErrorMessage } from '@/lib/api-utils'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Checkbox } from '@/components/ui/checkbox'
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
  Webhook as WebhookIcon,
  Trash2,
  Save,
  Play,
  Plus,
} from 'lucide-vue-next'

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

function toggleEvent(eventValue: string, checked: boolean | 'indeterminate') {
  if (checked === true) {
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
      icon-gradient="bg-gradient-to-br from-indigo-500 to-purple-600 shadow-indigo-500/20"
      back-link="/settings/webhooks"
      :breadcrumbs="breadcrumbs"
      :is-loading="isLoading"
      :is-not-found="isNotFound"
      :not-found-title="$t('webhooks.notFound', 'Webhook not found')"
    >
      <template #actions>
        <div class="flex items-center gap-2">
          <Button v-if="canWrite && (hasChanges || isNew)" size="sm" @click="save" :disabled="isSaving">
            <Save class="h-4 w-4 mr-1" /> {{ isSaving ? $t('common.saving', 'Saving...') : isNew ? $t('common.create') : $t('common.save') }}
          </Button>
          <Button v-if="!isNew" variant="outline" size="sm" @click="testWebhook" :disabled="isTesting">
            <Play class="h-4 w-4 mr-1" /> {{ isTesting ? $t('webhooks.testing', 'Testing...') : $t('webhooks.testWebhook') }}
          </Button>
          <Button v-if="canDelete && !isNew" variant="destructive" size="sm" @click="deleteDialogOpen = true">
            <Trash2 class="h-4 w-4 mr-1" /> {{ $t('common.delete') }}
          </Button>
        </div>
      </template>

      <Card>
        <CardHeader class="pb-3">
          <div class="flex items-center justify-between">
            <CardTitle class="text-sm font-medium">{{ $t('teams.details', 'Details') }}</CardTitle>
            <Badge v-if="!isNew" :variant="(webhook?.is_active ?? true) ? 'default' : 'secondary'">
              {{ (webhook?.is_active ?? true) ? $t('common.active') : $t('common.inactive') }}
            </Badge>
          </div>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('webhooks.name') }} <span class="text-destructive">*</span></Label>
            <Input v-model="form.name" :placeholder="$t('webhooks.namePlaceholder')" :disabled="!canWrite" />
          </div>
          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('webhooks.webhookUrl', 'URL') }} <span class="text-destructive">*</span></Label>
            <Input v-model="form.url" type="url" :placeholder="$t('webhooks.webhookUrlPlaceholder')" :disabled="!canWrite" />
          </div>
          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('webhooks.events') }} <span class="text-destructive">*</span></Label>
            <div class="grid grid-cols-1 gap-2 border rounded-lg p-3">
              <div v-for="event in availableEvents" :key="event.value" class="flex items-start gap-2">
                <Checkbox
                  :id="event.value"
                  :checked="form.events.includes(event.value)"
                  @update:checked="(checked) => toggleEvent(event.value, checked)"
                  :disabled="!canWrite"
                />
                <div class="grid gap-0.5">
                  <Label :for="event.value" class="cursor-pointer">{{ event.label }}</Label>
                  <p class="text-xs text-muted-foreground">{{ event.description }}</p>
                </div>
              </div>
            </div>
          </div>
          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('webhooks.secret') }}</Label>
            <Input v-model="form.secret" type="password" :placeholder="$t('webhooks.secretPlaceholder')" :disabled="!canWrite" />
            <p class="text-xs text-muted-foreground">{{ $t('webhooks.secretHint') }}</p>
          </div>
          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('webhooks.customHeaders') }}</Label>
            <div class="space-y-2">
              <div v-for="(value, key) in form.headers" :key="key" class="flex items-center gap-2">
                <Badge variant="secondary" class="flex-shrink-0">{{ key }}</Badge>
                <span class="text-sm truncate flex-1">{{ value }}</span>
                <Button v-if="canWrite" variant="ghost" size="icon" class="h-6 w-6 flex-shrink-0" @click="removeHeader(key as string)">
                  <Trash2 class="h-3 w-3" />
                </Button>
              </div>
              <div v-if="canWrite" class="flex gap-2">
                <Input v-model="newHeaderKey" :placeholder="$t('webhooks.headerName')" class="flex-1" />
                <Input v-model="newHeaderValue" :placeholder="$t('webhooks.headerValue')" class="flex-1" />
                <Button variant="outline" size="sm" @click="addHeader">
                  <Plus class="h-3 w-3 mr-1" /> {{ $t('common.add') }}
                </Button>
              </div>
            </div>
          </div>
          <div v-if="!isNew" class="flex items-center justify-between">
            <Label class="text-xs font-normal cursor-pointer">{{ $t('common.active') }}</Label>
            <Switch
              :checked="form.is_active"
              @update:checked="form.is_active = $event"
              :disabled="!canWrite"
            />
          </div>
        </CardContent>
      </Card>

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

    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('webhooks.deleteWebhook') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('teams.deleteConfirm', 'Are you sure? This action cannot be undone.') }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('common.cancel') }}</AlertDialogCancel>
          <AlertDialogAction @click="deleteWebhook">{{ $t('common.delete') }}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
