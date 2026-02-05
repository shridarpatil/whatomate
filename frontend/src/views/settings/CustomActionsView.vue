<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { customActionsService, type CustomAction } from '@/services/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { PageHeader, DataTable, SearchInput, DeleteConfirmDialog, type Column } from '@/components/shared'
import { toast } from 'vue-sonner'
import { Plus, Trash2, Pencil, Zap, Loader2, Globe, Webhook, Code, Ticket, User, BarChart, Link, Phone, Mail, FileText, ExternalLink } from 'lucide-vue-next'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDate } from '@/lib/utils'
import { useDebounceFn } from '@vueuse/core'

const { t } = useI18n()

const actions = ref<CustomAction[]>([])
const isLoading = ref(false)
const isSaving = ref(false)

const isDialogOpen = ref(false)
const isEditing = ref(false)
const editingActionId = ref<string | null>(null)
const formData = ref({
  name: '', icon: 'zap', action_type: 'webhook' as 'webhook' | 'url' | 'javascript', is_active: true, display_order: 0,
  config: { url: '', method: 'POST', headers: {} as Record<string, string>, body: '', open_in_new_tab: true, code: '' }
})

const newHeaderKey = ref('')
const newHeaderValue = ref('')
const isDeleteDialogOpen = ref(false)
const actionToDelete = ref<CustomAction | null>(null)

const iconOptions = [
  { value: 'ticket', label: 'Ticket', icon: Ticket }, { value: 'user', label: 'User', icon: User },
  { value: 'bar-chart', label: 'Chart', icon: BarChart }, { value: 'link', label: 'Link', icon: Link },
  { value: 'phone', label: 'Phone', icon: Phone }, { value: 'mail', label: 'Mail', icon: Mail },
  { value: 'file-text', label: 'Document', icon: FileText }, { value: 'external-link', label: 'External', icon: ExternalLink },
  { value: 'zap', label: 'Zap', icon: Zap }, { value: 'globe', label: 'Globe', icon: Globe }, { value: 'code', label: 'Code', icon: Code }
]

// Pagination state
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 20

const columns = computed<Column<CustomAction>[]>(() => [
  { key: 'icon', label: '', width: 'w-[40px]' },
  { key: 'name', label: t('customActions.name'), sortable: true },
  { key: 'type', label: t('customActions.type'), sortable: true, sortKey: 'action_type' },
  { key: 'target', label: t('customActions.target') },
  { key: 'status', label: t('customActions.status'), sortable: true, sortKey: 'is_active' },
  { key: 'created', label: t('customActions.created'), sortable: true, sortKey: 'created_at' },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

// Sorting state
const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

const searchQuery = ref('')

const getIconComponent = (iconName: string) => iconOptions.find(i => i.value === iconName)?.icon || Zap

async function fetchActions() {
  isLoading.value = true
  try {
    const response = await customActionsService.list({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    const data = (response.data as any).data || response.data
    actions.value = data.custom_actions || []
    totalItems.value = data.total ?? actions.value.length
  } catch (e) { toast.error(getErrorMessage(e, t('common.failedLoad', { resource: t('resources.customActions') }))) }
  finally { isLoading.value = false }
}

// Debounced search
const debouncedSearch = useDebounceFn(() => {
  currentPage.value = 1
  fetchActions()
}, 300)

watch(searchQuery, () => debouncedSearch())

function handlePageChange(page: number) {
  currentPage.value = page
  fetchActions()
}

function openCreateDialog() {
  isEditing.value = false
  editingActionId.value = null
  formData.value = { name: '', icon: 'zap', action_type: 'webhook', is_active: true, display_order: actions.value.length, config: { url: '', method: 'POST', headers: {}, body: '', open_in_new_tab: true, code: '' } }
  isDialogOpen.value = true
}

function openEditDialog(action: CustomAction) {
  isEditing.value = true
  editingActionId.value = action.id
  formData.value = {
    name: action.name, icon: action.icon || 'zap', action_type: action.action_type, is_active: action.is_active, display_order: action.display_order,
    config: { url: action.config.url || '', method: action.config.method || 'POST', headers: { ...(action.config.headers || {}) }, body: action.config.body || '', open_in_new_tab: action.config.open_in_new_tab !== false, code: action.config.code || '' }
  }
  isDialogOpen.value = true
}

async function saveAction() {
  if (!formData.value.name.trim()) { toast.error(t('customActions.nameRequired')); return }
  if ((formData.value.action_type === 'webhook' || formData.value.action_type === 'url') && !formData.value.config.url.trim()) { toast.error(t('customActions.urlRequired')); return }
  if (formData.value.action_type === 'javascript' && !formData.value.config.code.trim()) { toast.error(t('customActions.jsCodeRequired')); return }

  let config: Record<string, any> = {}
  switch (formData.value.action_type) {
    case 'webhook': config = { url: formData.value.config.url.trim(), method: formData.value.config.method, headers: formData.value.config.headers, body: formData.value.config.body.trim() }; break
    case 'url': config = { url: formData.value.config.url.trim(), open_in_new_tab: formData.value.config.open_in_new_tab }; break
    case 'javascript': config = { code: formData.value.config.code }; break
  }

  isSaving.value = true
  try {
    const payload = { name: formData.value.name.trim(), icon: formData.value.icon, action_type: formData.value.action_type, config, is_active: formData.value.is_active, display_order: formData.value.display_order }
    if (isEditing.value && editingActionId.value) { await customActionsService.update(editingActionId.value, payload); toast.success(t('common.updatedSuccess', { resource: t('resources.CustomAction') })) }
    else { await customActionsService.create(payload); toast.success(t('common.createdSuccess', { resource: t('resources.CustomAction') })) }
    isDialogOpen.value = false
    await fetchActions()
  } catch (e) { toast.error(getErrorMessage(e, t('common.failedSave', { resource: t('resources.customAction') }))) }
  finally { isSaving.value = false }
}

async function toggleAction(action: CustomAction) {
  try { await customActionsService.update(action.id, { is_active: !action.is_active }); await fetchActions(); toast.success(action.is_active ? t('common.disabledSuccess', { resource: t('resources.CustomAction') }) : t('common.enabledSuccess', { resource: t('resources.CustomAction') })) }
  catch (e) { toast.error(getErrorMessage(e, t('common.failedUpdate', { resource: t('resources.customAction') }))) }
}

async function deleteAction() {
  if (!actionToDelete.value) return
  try { await customActionsService.delete(actionToDelete.value.id); await fetchActions(); toast.success(t('common.deletedSuccess', { resource: t('resources.CustomAction') })); isDeleteDialogOpen.value = false; actionToDelete.value = null }
  catch (e) { toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.customAction') }))) }
}

function addHeader() { if (newHeaderKey.value.trim() && newHeaderValue.value.trim()) { formData.value.config.headers[newHeaderKey.value.trim()] = newHeaderValue.value.trim(); newHeaderKey.value = ''; newHeaderValue.value = '' } }
function removeHeader(key: string) { delete formData.value.config.headers[key] }
function getActionTypeBadge(type: string) { return { webhook: { label: 'Webhook', variant: 'default' as const }, url: { label: 'URL', variant: 'secondary' as const }, javascript: { label: 'JavaScript', variant: 'outline' as const } }[type] || { label: type, variant: 'outline' as const } }

const defaultBodyTemplate = `{\n  "subject": "WhatsApp: {{contact.name}}",\n  "phone": "{{contact.phone_number}}",\n  "description": "Contact from WhatsApp",\n  "user": "{{user.name}}"\n}`

onMounted(() => fetchActions())
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('customActions.title')" :subtitle="$t('customActions.subtitle')" :icon="Zap" icon-gradient="bg-gradient-to-br from-yellow-500 to-orange-600 shadow-yellow-500/20">
      <template #actions>
        <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />{{ $t('customActions.addAction') }}</Button>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto">
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>{{ $t('customActions.yourActions') }}</CardTitle>
                  <CardDescription>{{ $t('customActions.yourActionsDesc') }}</CardDescription>
                </div>
                <SearchInput v-model="searchQuery" :placeholder="$t('customActions.searchActions') + '...'" class="w-64" />
              </div>
            </CardHeader>
            <CardContent>
              <DataTable :items="actions" :columns="columns" :is-loading="isLoading" :empty-icon="Zap" :empty-title="searchQuery ? $t('customActions.noMatchingActions') : $t('customActions.noActionsYet')" :empty-description="searchQuery ? $t('customActions.noMatchingActionsDesc') : $t('customActions.noActionsYetDesc')" v-model:sort-key="sortKey" v-model:sort-direction="sortDirection" server-pagination :current-page="currentPage" :total-items="totalItems" :page-size="pageSize" item-name="actions" @page-change="handlePageChange">
                <template #cell-icon="{ item: action }"><component :is="getIconComponent(action.icon)" class="h-5 w-5 text-muted-foreground" /></template>
                <template #cell-name="{ item: action }"><span class="font-medium">{{ action.name }}</span></template>
                <template #cell-type="{ item: action }"><Badge :variant="getActionTypeBadge(action.action_type).variant">{{ getActionTypeBadge(action.action_type).label }}</Badge></template>
                <template #cell-target="{ item: action }"><span class="max-w-[200px] truncate text-muted-foreground block">{{ action.action_type === 'javascript' ? $t('customActions.customScript') : action.config.url }}</span></template>
                <template #cell-status="{ item: action }">
                  <div class="flex items-center gap-2"><Switch :checked="action.is_active" @update:checked="toggleAction(action)" /><span class="text-sm text-muted-foreground">{{ action.is_active ? $t('common.active') : $t('common.inactive') }}</span></div>
                </template>
                <template #cell-created="{ item: action }"><span class="text-muted-foreground">{{ formatDate(action.created_at) }}</span></template>
                <template #cell-actions="{ item: action }">
                  <div class="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click="openEditDialog(action)"><Pencil class="h-4 w-4" /></Button>
                    <Button variant="ghost" size="icon" class="h-8 w-8 text-destructive" @click="actionToDelete = action; isDeleteDialogOpen = true"><Trash2 class="h-4 w-4" /></Button>
                  </div>
                </template>
                <template #empty-action>
                  <Button variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />{{ $t('customActions.addAction') }}</Button>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <!-- Custom Dialog (complex action type configuration) -->
    <Dialog v-model:open="isDialogOpen">
      <DialogContent class="max-w-lg max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{{ isEditing ? $t('customActions.editTitle') : $t('customActions.createTitle') }}</DialogTitle>
          <DialogDescription>{{ $t('customActions.dialogDesc') }}</DialogDescription>
        </DialogHeader>
        <div class="space-y-4 py-4">
          <div class="space-y-2"><Label for="name">{{ $t('customActions.name') }}</Label><Input id="name" v-model="formData.name" :placeholder="$t('customActions.namePlaceholder')" /></div>
          <div class="space-y-2">
            <Label>{{ $t('customActions.icon') }}</Label>
            <div class="flex flex-wrap gap-2">
              <Button v-for="iconOpt in iconOptions" :key="iconOpt.value" variant="outline" size="icon" class="h-10 w-10" :class="{ 'ring-2 ring-primary': formData.icon === iconOpt.value }" @click="formData.icon = iconOpt.value"><component :is="iconOpt.icon" class="h-5 w-5" /></Button>
            </div>
          </div>
          <div class="space-y-2">
            <Label>{{ $t('customActions.actionType') }}</Label>
            <RadioGroup v-model="formData.action_type" class="flex flex-col gap-2">
              <div class="flex items-center space-x-2"><RadioGroupItem value="webhook" id="type-webhook" /><Label for="type-webhook" class="flex items-center gap-2 cursor-pointer font-normal"><Webhook class="h-4 w-4" />{{ $t('customActions.webhookType') }}</Label></div>
              <div class="flex items-center space-x-2"><RadioGroupItem value="url" id="type-url" /><Label for="type-url" class="flex items-center gap-2 cursor-pointer font-normal"><Globe class="h-4 w-4" />{{ $t('customActions.urlType') }}</Label></div>
              <div class="flex items-center space-x-2"><RadioGroupItem value="javascript" id="type-javascript" /><Label for="type-javascript" class="flex items-center gap-2 cursor-pointer font-normal"><Code class="h-4 w-4" />{{ $t('customActions.javascriptType') }}</Label></div>
            </RadioGroup>
          </div>

          <!-- Webhook Configuration -->
          <template v-if="formData.action_type === 'webhook'">
            <div class="border-t pt-4 space-y-4">
              <div class="space-y-2"><Label for="url">{{ $t('customActions.webhookUrl') }}</Label><Input id="url" v-model="formData.config.url" type="url" :placeholder="$t('customActions.webhookUrlPlaceholder')" /></div>
              <div class="space-y-2">
                <Label for="method">{{ $t('customActions.httpMethod') }}</Label>
                <Select v-model="formData.config.method"><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="POST">POST</SelectItem><SelectItem value="GET">GET</SelectItem><SelectItem value="PUT">PUT</SelectItem><SelectItem value="PATCH">PATCH</SelectItem></SelectContent></Select>
              </div>
              <div class="space-y-2">
                <Label>{{ $t('customActions.headers') }}</Label>
                <div class="space-y-2">
                  <div v-for="(value, key) in formData.config.headers" :key="key" class="flex items-center gap-2">
                    <Badge variant="secondary" class="flex-shrink-0">{{ key }}</Badge><span class="text-sm truncate flex-1">{{ value }}</span>
                    <Button variant="ghost" size="icon" class="h-6 w-6 flex-shrink-0" @click="removeHeader(key as string)"><Trash2 class="h-3 w-3" /></Button>
                  </div>
                  <div class="flex gap-2"><Input v-model="newHeaderKey" :placeholder="$t('webhooks.headerName')" class="flex-1" /><Input v-model="newHeaderValue" :placeholder="$t('webhooks.headerValue')" class="flex-1" /><Button variant="outline" size="sm" @click="addHeader">{{ $t('common.add') }}</Button></div>
                </div>
              </div>
              <div class="space-y-2">
                <div class="flex items-center justify-between"><Label for="body">{{ $t('customActions.requestBody') }}</Label><Button variant="link" size="sm" class="h-auto p-0 text-xs" @click="formData.config.body = defaultBodyTemplate">{{ $t('customActions.insertTemplate') }}</Button></div>
                <Textarea id="body" v-model="formData.config.body" placeholder='{"subject": "{{contact.name}}"}' class="font-mono text-sm min-h-[120px]" />
                <p class="text-xs text-muted-foreground">{{ $t('customActions.bodyVariables') }}</p>
              </div>
            </div>
          </template>

          <!-- URL Configuration -->
          <template v-if="formData.action_type === 'url'">
            <div class="border-t pt-4 space-y-4">
              <div class="space-y-2"><Label for="url">{{ $t('customActions.urlLabel') }}</Label><Input id="url" v-model="formData.config.url" type="url" :placeholder="$t('customActions.urlPlaceholder')" /><p class="text-xs text-muted-foreground">{{ $t('customActions.urlHint') }}</p></div>
              <div class="flex items-center space-x-2"><Switch id="new-tab" :checked="formData.config.open_in_new_tab" @update:checked="formData.config.open_in_new_tab = $event" /><Label for="new-tab" class="cursor-pointer">{{ $t('customActions.openInNewTab') }}</Label></div>
            </div>
          </template>

          <!-- JavaScript Configuration -->
          <template v-if="formData.action_type === 'javascript'">
            <div class="border-t pt-4 space-y-4">
              <div class="space-y-2">
                <Label for="code">{{ $t('customActions.jsCode') }}</Label>
                <Textarea id="code" v-model="formData.config.code" :placeholder="$t('customActions.jsCodePlaceholder')" class="font-mono text-sm min-h-[200px]" />
                <p class="text-xs text-muted-foreground">{{ $t('customActions.jsReturnHint') }}</p>
              </div>
            </div>
          </template>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="isDialogOpen = false">{{ $t('common.cancel') }}</Button>
          <Button @click="saveAction" :disabled="isSaving"><Loader2 v-if="isSaving" class="h-4 w-4 mr-2 animate-spin" />{{ isEditing ? $t('common.update') : $t('common.create') }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <DeleteConfirmDialog v-model:open="isDeleteDialogOpen" :title="$t('customActions.deleteTitle')" :item-name="actionToDelete?.name" @confirm="deleteAction" />
  </div>
</template>
