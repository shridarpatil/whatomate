<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { customActionsService, type CustomAction } from '@/services/api'
import Modal from '@/components/oat/Modal.vue'
import { PageHeader, DataTable, SearchInput, DeleteConfirmDialog, ConfirmDialog, IconButton, ErrorState, type Column } from '@/components/shared'
import { toast } from '@/lib/toast'
import { Plus, Trash2, Pencil, Zap, Globe, Webhook, Code, Ticket, User, BarChart, Link, Phone, Mail, FileText, ExternalLink } from '@/lib/icons'
import { getErrorMessage } from '@/lib/api-utils'
import { debounce, formatDate } from '@/lib/utils'

const { t } = useI18n()

const actions = ref<CustomAction[]>([])
const isLoading = ref(false)
const isSaving = ref(false)
const isDeleting = ref(false)
const error = ref(false)

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

const isDisableDialogOpen = ref(false)
const actionToToggle = ref<CustomAction | null>(null)
const isToggling = ref(false)

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
  { key: 'icon', label: '' },
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
  error.value = false
  try {
    const response = await customActionsService.list({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    const data = (response.data as any).data || response.data
    actions.value = data.custom_actions || []
    totalItems.value = data.total ?? actions.value.length
  } catch (e) { error.value = true; toast.error(getErrorMessage(e, t('common.failedLoad', { resource: t('resources.customActions') }))) }
  finally { isLoading.value = false }
}

// Debounced search
const debouncedSearch = debounce(() => {
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

function handleToggleAction(action: CustomAction) {
  if (action.is_active) {
    actionToToggle.value = action
    isDisableDialogOpen.value = true
  } else {
    performToggleAction(action)
  }
}

async function performToggleAction(action: CustomAction) {
  isToggling.value = true
  try {
    await customActionsService.update(action.id, { is_active: !action.is_active })
    await fetchActions()
    toast.success(action.is_active ? t('common.disabledSuccess', { resource: t('resources.CustomAction') }) : t('common.enabledSuccess', { resource: t('resources.CustomAction') }))
    isDisableDialogOpen.value = false
    actionToToggle.value = null
  } catch (e) { toast.error(getErrorMessage(e, t('common.failedUpdate', { resource: t('resources.customAction') }))) }
  finally { isToggling.value = false }
}

function confirmDisableAction() {
  if (actionToToggle.value) performToggleAction(actionToToggle.value)
}

async function deleteAction() {
  if (!actionToDelete.value) return
  isDeleting.value = true
  try { await customActionsService.delete(actionToDelete.value.id); await fetchActions(); toast.success(t('common.deletedSuccess', { resource: t('resources.CustomAction') })); isDeleteDialogOpen.value = false; actionToDelete.value = null }
  catch (e) { toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.customAction') }))) }
  finally { isDeleting.value = false }
}

function addHeader() { if (newHeaderKey.value.trim() && newHeaderValue.value.trim()) { formData.value.config.headers[newHeaderKey.value.trim()] = newHeaderValue.value.trim(); newHeaderKey.value = ''; newHeaderValue.value = '' } }
function removeHeader(key: string) { delete formData.value.config.headers[key] }
function getActionTypeBadge(type: string) { return { webhook: { label: 'Webhook', variant: undefined, outline: false }, url: { label: 'URL', variant: 'secondary', outline: false }, javascript: { label: 'JavaScript', variant: 'secondary', outline: true } }[type] || { label: type, variant: undefined, outline: true } }

const defaultBodyTemplate = `{\n  "subject": "WhatsApp: {{contact.name}}",\n  "phone": "{{contact.phone_number}}",\n  "description": "Contact from WhatsApp",\n  "user": "{{user.name}}"\n}`

onMounted(() => fetchActions())
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader :title="$t('customActions.title')" :description="$t('customActions.subtitle')" :icon="Zap">
      <template #actions>
        <button type="button" class="outline small" @click="openCreateDialog"><Plus :size="16" />{{ $t('customActions.addAction') }}</button>
      </template>
    </PageHeader>

    <ErrorState
      v-if="error && !isLoading"
      :title="$t('customActions.fetchErrorTitle')"
      :description="$t('customActions.fetchErrorDescription')"
      :retry-label="$t('common.retry')"
      class="flex-1"
      @retry="fetchActions"
    />

    <div v-else class="flex-1 scroll-y p-4">
      <article class="card">
        <header class="hstack justify-between flex-wrap gap-4">
          <div>
            <h3>{{ $t('customActions.yourActions') }}</h3>
            <p class="text-light">{{ $t('customActions.yourActionsDesc') }}</p>
          </div>
          <SearchInput v-model="searchQuery" :placeholder="$t('customActions.searchActions') + '...'" />
        </header>
        <DataTable :items="actions" :columns="columns" :is-loading="isLoading" :empty-icon="Zap" :empty-title="searchQuery ? $t('customActions.noMatchingActions') : $t('customActions.noActionsYet')" :empty-description="searchQuery ? $t('customActions.noMatchingActionsDesc') : $t('customActions.noActionsYetDesc')" v-model:sort-key="sortKey" v-model:sort-direction="sortDirection" server-pagination :current-page="currentPage" :total-items="totalItems" :page-size="pageSize" item-name="actions" @page-change="handlePageChange">
          <template #cell-icon="{ item: action }"><component :is="getIconComponent(action.icon)" :size="20" /></template>
          <template #cell-name="{ item: action }"><strong>{{ action.name }}</strong></template>
          <template #cell-type="{ item: action }"><span class="badge" :class="{ outline: getActionTypeBadge(action.action_type).outline }" :data-variant="getActionTypeBadge(action.action_type).variant">{{ getActionTypeBadge(action.action_type).label }}</span></template>
          <template #cell-target="{ item: action }"><span class="truncate text-light">{{ action.action_type === 'javascript' ? $t('customActions.customScript') : action.config.url }}</span></template>
          <template #cell-status="{ item: action }">
            <label class="hstack gap-2">
              <input type="checkbox" role="switch" :checked="action.is_active" :aria-label="`${action.name}: ${action.is_active ? $t('common.active') : $t('common.inactive')}`" @click.prevent="handleToggleAction(action)" />
              <small class="text-light">{{ action.is_active ? $t('common.active') : $t('common.inactive') }}</small>
            </label>
          </template>
          <template #cell-created="{ item: action }"><span class="text-light">{{ formatDate(action.created_at) }}</span></template>
          <template #cell-actions="{ item: action }">
            <div class="hstack gap-1 justify-end">
              <IconButton :icon="Pencil" :label="$t('common.edit')" @click="openEditDialog(action)" />
              <IconButton :icon="Trash2" :label="$t('common.delete')" @click="actionToDelete = action; isDeleteDialogOpen = true" />
            </div>
          </template>
          <template #empty-action>
            <button type="button" class="outline small" @click="openCreateDialog"><Plus :size="16" />{{ $t('customActions.addAction') }}</button>
          </template>
        </DataTable>
      </article>
    </div>

    <!-- Custom Dialog (complex action type configuration) -->
    <Modal v-model:open="isDialogOpen" :title="isEditing ? $t('customActions.editTitle') : $t('customActions.createTitle')" :description="$t('customActions.dialogDesc')">
      <div class="vstack gap-4">
        <label data-field>
          {{ $t('customActions.name') }}
          <input id="name" v-model="formData.name" :placeholder="$t('customActions.namePlaceholder')" />
        </label>
        <div data-field>
          <label>{{ $t('customActions.icon') }}</label>
          <div class="hstack gap-2 flex-wrap">
            <button
              v-for="iconOpt in iconOptions"
              :key="iconOpt.value"
              type="button"
              class="icon"
              :class="{ outline: formData.icon !== iconOpt.value }"
              :title="iconOpt.label"
              :aria-label="iconOpt.label"
              :aria-pressed="formData.icon === iconOpt.value"
              @click="formData.icon = iconOpt.value"
            ><component :is="iconOpt.icon" :size="20" /></button>
          </div>
        </div>
        <fieldset class="vstack gap-2">
          <legend>{{ $t('customActions.actionType') }}</legend>
            <label><input id="type-webhook" v-model="formData.action_type" type="radio" name="action_type" value="webhook" /><Webhook :size="16" /> {{ $t('customActions.webhookType') }}</label>
            <label><input id="type-url" v-model="formData.action_type" type="radio" name="action_type" value="url" /><Globe :size="16" /> {{ $t('customActions.urlType') }}</label>
            <label><input id="type-javascript" v-model="formData.action_type" type="radio" name="action_type" value="javascript" /><Code :size="16" /> {{ $t('customActions.javascriptType') }}</label>
        </fieldset>

        <!-- Webhook Configuration -->
        <template v-if="formData.action_type === 'webhook'">
          <hr>
          <label data-field>
            {{ $t('customActions.webhookUrl') }}
            <input id="url" v-model="formData.config.url" type="url" :placeholder="$t('customActions.webhookUrlPlaceholder')" />
          </label>
          <label data-field>
            {{ $t('customActions.httpMethod') }}
            <select id="method" v-model="formData.config.method">
              <option value="POST">POST</option>
              <option value="GET">GET</option>
              <option value="PUT">PUT</option>
              <option value="PATCH">PATCH</option>
            </select>
          </label>
          <div class="vstack gap-2">
            <strong>{{ $t('customActions.headers') }}</strong>
            <div v-for="(value, key) in formData.config.headers" :key="key" class="hstack gap-2">
              <span class="badge" data-variant="secondary">{{ key }}</span>
              <span class="truncate flex-1">{{ value }}</span>
              <button type="button" class="ghost icon small" :title="$t('common.remove')" :aria-label="`${$t('common.remove')} ${key}`" @click="removeHeader(key as string)"><Trash2 :size="14" /></button>
            </div>
            <div class="hstack gap-2">
              <input v-model="newHeaderKey" class="flex-1" :aria-label="$t('webhooks.headerName')" :placeholder="$t('webhooks.headerName')" />
              <input v-model="newHeaderValue" class="flex-1" :aria-label="$t('webhooks.headerValue')" :placeholder="$t('webhooks.headerValue')" />
              <button type="button" class="outline small" @click="addHeader">{{ $t('common.add') }}</button>
            </div>
          </div>
          <div data-field>
            <div class="hstack justify-between">
              <label for="body">{{ $t('customActions.requestBody') }}</label>
              <button type="button" class="ghost small" @click="formData.config.body = defaultBodyTemplate">{{ $t('customActions.insertTemplate') }}</button>
            </div>
            <textarea id="body" v-model="formData.config.body" placeholder='{"subject": "{{contact.name}}"}' rows="6" />
            <small data-hint>{{ $t('customActions.bodyVariables') }}</small>
          </div>
        </template>

        <!-- URL Configuration -->
        <template v-if="formData.action_type === 'url'">
          <hr>
          <label data-field>
            {{ $t('customActions.urlLabel') }}
            <input id="url" v-model="formData.config.url" type="url" :placeholder="$t('customActions.urlPlaceholder')" />
            <small data-hint>{{ $t('customActions.urlHint') }}</small>
          </label>
          <label><input id="new-tab" v-model="formData.config.open_in_new_tab" type="checkbox" role="switch" /> {{ $t('customActions.openInNewTab') }}</label>
        </template>

        <!-- JavaScript Configuration -->
        <template v-if="formData.action_type === 'javascript'">
          <hr>
          <label data-field>
            {{ $t('customActions.jsCode') }}
            <textarea id="code" v-model="formData.config.code" :placeholder="$t('customActions.jsCodePlaceholder')" rows="10" />
            <small data-hint>{{ $t('customActions.jsReturnHint') }}</small>
          </label>
        </template>
      </div>
      <template #footer>
        <button type="button" class="outline" @click="isDialogOpen = false">{{ $t('common.cancel') }}</button>
        <button type="button" :aria-busy="isSaving" :disabled="isSaving" @click="saveAction">{{ isEditing ? $t('common.update') : $t('common.create') }}</button>
      </template>
    </Modal>

    <ConfirmDialog
      v-model:open="isDisableDialogOpen"
      :title="$t('customActions.confirmDisableTitle')"
      :description="$t('customActions.confirmDisableDescription')"
      :confirm-label="$t('common.confirm')"
      variant="destructive"
      :is-submitting="isToggling"
      @confirm="confirmDisableAction"
    />

    <DeleteConfirmDialog v-model:open="isDeleteDialogOpen" :title="$t('customActions.deleteTitle')" :item-name="actionToDelete?.name" :is-submitting="isDeleting" @confirm="deleteAction" />
  </div>
</template>
