<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { webhooksService, type Webhook, type WebhookEvent } from '@/services/api'
import { useOrganizationsStore } from '@/stores/organizations'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Checkbox } from '@/components/ui/checkbox'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { PageHeader, DataTable, SearchInput, DeleteConfirmDialog, type Column } from '@/components/shared'
import { toast } from 'vue-sonner'
import { Plus, Trash2, Pencil, Webhook as WebhookIcon, Play, Loader2 } from 'lucide-vue-next'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDate } from '@/lib/utils'
import { useDebounceFn } from '@vueuse/core'

const { t } = useI18n()

const organizationsStore = useOrganizationsStore()

const webhooks = ref<Webhook[]>([])
const availableEvents = ref<WebhookEvent[]>([])
const isLoading = ref(false)
const isSaving = ref(false)
const isTesting = ref<string | null>(null)

const isDialogOpen = ref(false)
const isEditing = ref(false)
const editingWebhookId = ref<string | null>(null)
const formData = ref({ name: '', url: '', events: [] as string[], secret: '', headers: {} as Record<string, string> })

const newHeaderKey = ref('')
const newHeaderValue = ref('')

const isDeleteDialogOpen = ref(false)
const webhookToDelete = ref<Webhook | null>(null)

// Pagination state
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 20

const columns = computed<Column<Webhook>[]>(() => [
  { key: 'name', label: t('webhooks.name'), sortable: true },
  { key: 'url', label: t('webhooks.url'), sortable: true },
  { key: 'events', label: t('webhooks.events') },
  { key: 'status', label: t('webhooks.status'), sortable: true, sortKey: 'is_active' },
  { key: 'created', label: t('webhooks.created'), sortable: true, sortKey: 'created_at' },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

// Sorting state
const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

const searchQuery = ref('')

async function fetchWebhooks() {
  isLoading.value = true
  try {
    const response = await webhooksService.list({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    const data = (response.data as any).data || response.data
    webhooks.value = data.webhooks || []
    availableEvents.value = data.available_events || []
    totalItems.value = data.total ?? webhooks.value.length
  } catch (e) { toast.error(getErrorMessage(e, t('common.failedLoad', { resource: t('resources.webhooks') }))) }
  finally { isLoading.value = false }
}

// Debounced search
const debouncedSearch = useDebounceFn(() => {
  currentPage.value = 1
  fetchWebhooks()
}, 300)

watch(searchQuery, () => debouncedSearch())

function handlePageChange(page: number) {
  currentPage.value = page
  fetchWebhooks()
}

function openCreateDialog() {
  isEditing.value = false
  editingWebhookId.value = null
  formData.value = { name: '', url: '', events: [], secret: '', headers: {} }
  isDialogOpen.value = true
}

function openEditDialog(webhook: Webhook) {
  isEditing.value = true
  editingWebhookId.value = webhook.id
  formData.value = { name: webhook.name, url: webhook.url, events: [...webhook.events], secret: '', headers: { ...webhook.headers } }
  isDialogOpen.value = true
}

async function saveWebhook() {
  if (!formData.value.name.trim()) { toast.error(t('webhooks.nameRequired')); return }
  if (!formData.value.url.trim()) { toast.error(t('webhooks.urlRequired')); return }
  if (formData.value.events.length === 0) { toast.error(t('webhooks.eventRequired')); return }

  isSaving.value = true
  try {
    const payload = { name: formData.value.name.trim(), url: formData.value.url.trim(), events: formData.value.events, headers: formData.value.headers, secret: formData.value.secret || undefined }
    if (isEditing.value && editingWebhookId.value) {
      await webhooksService.update(editingWebhookId.value, { ...payload, is_active: true })
      toast.success(t('common.updatedSuccess', { resource: t('resources.Webhook') }))
    } else {
      await webhooksService.create(payload)
      toast.success(t('common.createdSuccess', { resource: t('resources.Webhook') }))
    }
    isDialogOpen.value = false
    await fetchWebhooks()
  } catch (e) { toast.error(getErrorMessage(e, t('common.failedSave', { resource: t('resources.webhook') }))) }
  finally { isSaving.value = false }
}

async function toggleWebhook(webhook: Webhook) {
  try {
    await webhooksService.update(webhook.id, { is_active: !webhook.is_active })
    await fetchWebhooks()
    toast.success(webhook.is_active ? t('common.disabledSuccess', { resource: t('resources.Webhook') }) : t('common.enabledSuccess', { resource: t('resources.Webhook') }))
  } catch (e) { toast.error(getErrorMessage(e, t('common.failedToggle', { resource: t('resources.webhook') }))) }
}

async function testWebhook(webhook: Webhook) {
  isTesting.value = webhook.id
  try { await webhooksService.test(webhook.id); toast.success(t('webhooks.testSent')) }
  catch (e) { toast.error(getErrorMessage(e, t('webhooks.testFailed'))) }
  finally { isTesting.value = null }
}

async function deleteWebhook() {
  if (!webhookToDelete.value) return
  try { await webhooksService.delete(webhookToDelete.value.id); await fetchWebhooks(); toast.success(t('common.deletedSuccess', { resource: t('resources.Webhook') })); isDeleteDialogOpen.value = false; webhookToDelete.value = null }
  catch (e) { toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.webhook') }))) }
}

function addHeader() {
  if (newHeaderKey.value.trim() && newHeaderValue.value.trim()) {
    formData.value.headers[newHeaderKey.value.trim()] = newHeaderValue.value.trim()
    newHeaderKey.value = ''
    newHeaderValue.value = ''
  }
}

function removeHeader(key: string) { delete formData.value.headers[key] }

function toggleEvent(eventValue: string, checked: boolean | 'indeterminate') {
  if (checked === true) { if (!formData.value.events.includes(eventValue)) formData.value.events.push(eventValue) }
  else { const index = formData.value.events.indexOf(eventValue); if (index > -1) formData.value.events.splice(index, 1) }
}

function getEventLabel(eventValue: string): string { return availableEvents.value.find(e => e.value === eventValue)?.label || eventValue }

watch(() => organizationsStore.selectedOrgId, () => fetchWebhooks())
onMounted(() => fetchWebhooks())
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('webhooks.title')" :subtitle="$t('webhooks.subtitle')" :icon="WebhookIcon" icon-gradient="bg-gradient-to-br from-indigo-500 to-purple-600 shadow-indigo-500/20">
      <template #actions>
        <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />{{ $t('webhooks.addWebhook') }}</Button>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto">
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>{{ $t('webhooks.yourWebhooks') }}</CardTitle>
                  <CardDescription>{{ $t('webhooks.yourWebhooksDesc') }}</CardDescription>
                </div>
                <SearchInput v-model="searchQuery" :placeholder="$t('webhooks.searchWebhooks') + '...'" class="w-64" />
              </div>
            </CardHeader>
            <CardContent>
              <DataTable :items="webhooks" :columns="columns" :is-loading="isLoading" :empty-icon="WebhookIcon" :empty-title="searchQuery ? $t('webhooks.noMatchingWebhooks') : $t('webhooks.noWebhooksYet')" :empty-description="searchQuery ? $t('webhooks.noMatchingWebhooksDesc') : $t('webhooks.noWebhooksYetDesc')" v-model:sort-key="sortKey" v-model:sort-direction="sortDirection" server-pagination :current-page="currentPage" :total-items="totalItems" :page-size="pageSize" item-name="webhooks" @page-change="handlePageChange">
                <template #cell-name="{ item: webhook }"><span class="font-medium">{{ webhook.name }}</span></template>
                <template #cell-url="{ item: webhook }"><span class="max-w-[200px] truncate text-muted-foreground block">{{ webhook.url }}</span></template>
                <template #cell-events="{ item: webhook }">
                  <div class="flex flex-wrap gap-1">
                    <Badge v-for="event in webhook.events.slice(0, 2)" :key="event" variant="secondary" class="text-xs">{{ getEventLabel(event) }}</Badge>
                    <Badge v-if="webhook.events.length > 2" variant="outline" class="text-xs">+{{ webhook.events.length - 2 }}</Badge>
                  </div>
                </template>
                <template #cell-status="{ item: webhook }">
                  <div class="flex items-center gap-2">
                    <Switch :checked="webhook.is_active" @update:checked="toggleWebhook(webhook)" />
                    <span class="text-sm text-muted-foreground">{{ webhook.is_active ? $t('common.active') : $t('common.inactive') }}</span>
                  </div>
                </template>
                <template #cell-created="{ item: webhook }"><span class="text-muted-foreground">{{ formatDate(webhook.created_at) }}</span></template>
                <template #cell-actions="{ item: webhook }">
                  <div class="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="icon" class="h-8 w-8" :disabled="isTesting === webhook.id" @click="testWebhook(webhook)"><Loader2 v-if="isTesting === webhook.id" class="h-4 w-4 animate-spin" /><Play v-else class="h-4 w-4" /></Button>
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click="openEditDialog(webhook)"><Pencil class="h-4 w-4" /></Button>
                    <Button variant="ghost" size="icon" class="h-8 w-8 text-destructive" @click="webhookToDelete = webhook; isDeleteDialogOpen = true"><Trash2 class="h-4 w-4" /></Button>
                  </div>
                </template>
                <template #empty-action>
                  <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />{{ $t('webhooks.addWebhook') }}</Button>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <!-- Custom Dialog (has headers editor and events checkboxes) -->
    <Dialog v-model:open="isDialogOpen">
      <DialogContent class="max-w-lg">
        <DialogHeader>
          <DialogTitle>{{ isEditing ? $t('webhooks.editWebhook') : $t('webhooks.addWebhook') }}</DialogTitle>
          <DialogDescription>{{ $t('webhooks.configureWebhook') }}</DialogDescription>
        </DialogHeader>
        <div class="space-y-4 py-4">
          <div class="space-y-2"><Label for="name">{{ $t('webhooks.name') }}</Label><Input id="name" v-model="formData.name" :placeholder="$t('webhooks.namePlaceholder')" /></div>
          <div class="space-y-2"><Label for="url">{{ $t('webhooks.webhookUrl') }}</Label><Input id="url" v-model="formData.url" type="url" :placeholder="$t('webhooks.webhookUrlPlaceholder')" /></div>
          <div class="space-y-2">
            <Label>{{ $t('webhooks.events') }}</Label>
            <div class="grid grid-cols-1 gap-2 border rounded-lg p-3">
              <div v-for="event in availableEvents" :key="event.value" class="flex items-start gap-2">
                <Checkbox :id="event.value" :checked="formData.events.includes(event.value)" @update:checked="(checked) => toggleEvent(event.value, checked)" />
                <div class="grid gap-0.5"><Label :for="event.value" class="cursor-pointer">{{ event.label }}</Label><p class="text-xs text-muted-foreground">{{ event.description }}</p></div>
              </div>
            </div>
          </div>
          <div class="space-y-2">
            <Label for="secret">{{ $t('webhooks.secret') }}</Label>
            <Input id="secret" v-model="formData.secret" type="password" :placeholder="$t('webhooks.secretPlaceholder')" />
            <p class="text-xs text-muted-foreground">{{ $t('webhooks.secretHint') }}</p>
          </div>
          <div class="space-y-2">
            <Label>{{ $t('webhooks.customHeaders') }}</Label>
            <div class="space-y-2">
              <div v-for="(value, key) in formData.headers" :key="key" class="flex items-center gap-2">
                <Badge variant="secondary" class="flex-shrink-0">{{ key }}</Badge>
                <span class="text-sm truncate flex-1">{{ value }}</span>
                <Button variant="ghost" size="icon" class="h-6 w-6 flex-shrink-0" @click="removeHeader(key as string)"><Trash2 class="h-3 w-3" /></Button>
              </div>
              <div class="flex gap-2">
                <Input v-model="newHeaderKey" :placeholder="$t('webhooks.headerName')" class="flex-1" />
                <Input v-model="newHeaderValue" :placeholder="$t('webhooks.headerValue')" class="flex-1" />
                <Button variant="outline" size="sm" @click="addHeader">{{ $t('common.add') }}</Button>
              </div>
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="isDialogOpen = false">{{ $t('common.cancel') }}</Button>
          <Button @click="saveWebhook" :disabled="isSaving"><Loader2 v-if="isSaving" class="h-4 w-4 mr-2 animate-spin" />{{ isEditing ? $t('common.update') : $t('common.create') }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <DeleteConfirmDialog v-model:open="isDeleteDialogOpen" :title="$t('webhooks.deleteWebhook')" :item-name="webhookToDelete?.name" @confirm="deleteWebhook" />
  </div>
</template>
