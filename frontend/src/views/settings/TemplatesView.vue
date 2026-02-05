<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import DOMPurify from 'dompurify'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Textarea } from '@/components/ui/textarea'
import { Separator } from '@/components/ui/separator'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { PageHeader, SearchInput, DataTable, DeleteConfirmDialog, type Column } from '@/components/shared'
import { api, templatesService } from '@/services/api'
import { useOrganizationsStore } from '@/stores/organizations'
import { toast } from 'vue-sonner'
import { Plus, RefreshCw, FileText, Eye, Pencil, Trash2, Loader2, MessageSquare, Image, FileIcon, Video, X, Check, AlertCircle, Send, Upload } from 'lucide-vue-next'
import { getErrorMessage } from '@/lib/api-utils'
import { useDebounceFn } from '@vueuse/core'

const { t } = useI18n()

interface WhatsAppAccount {
  id: string
  name: string
  phone_id: string
}

interface Template {
  id: string
  whatsapp_account: string
  meta_template_id: string
  name: string
  display_name: string
  language: string
  category: string
  status: string
  header_type: string
  header_content: string
  body_content: string
  footer_content: string
  buttons: any[]
  sample_values: any[]
  created_at: string
  updated_at: string
}

const organizationsStore = useOrganizationsStore()

const templates = ref<Template[]>([])
const accounts = ref<WhatsAppAccount[]>([])
const isLoading = ref(true)
const isSyncing = ref(false)
const searchQuery = ref('')
const selectedAccount = ref<string>(localStorage.getItem('templates_selected_account') || 'all')

// Dialog state
const isDialogOpen = ref(false)
const isSubmitting = ref(false)
const editingTemplate = ref<Template | null>(null)
const isPreviewOpen = ref(false)
const previewTemplate = ref<Template | null>(null)
const deleteDialogOpen = ref(false)
const templateToDelete = ref<Template | null>(null)
const publishDialogOpen = ref(false)
const templateToPublish = ref<Template | null>(null)

// Header media upload state
const headerMediaFile = ref<File | null>(null)
const headerMediaUploading = ref(false)
const headerMediaHandle = ref('')
const headerMediaFilename = ref('')

const formData = ref({
  whatsapp_account: '',
  name: '',
  display_name: '',
  language: 'en',
  category: 'UTILITY',
  header_type: 'NONE',
  header_content: '',
  body_content: '',
  footer_content: '',
  buttons: [] as any[],
  sample_values: [] as any[]
})

// Pagination state
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 20

const columns = computed<Column<Template>[]>(() => [
  { key: 'name', label: t('templates.name'), sortable: true },
  { key: 'category', label: t('templates.category'), sortable: true },
  { key: 'status', label: t('templates.status'), sortable: true },
  { key: 'language', label: t('templates.language'), sortable: true },
  { key: 'header_type', label: t('templates.header') },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

const languages = [
  { code: 'en', name: 'English' },
  { code: 'en_US', name: 'English (US)' },
  { code: 'en_GB', name: 'English (UK)' },
  { code: 'es', name: 'Spanish' },
  { code: 'pt_BR', name: 'Portuguese (BR)' },
  { code: 'hi', name: 'Hindi' },
  { code: 'ar', name: 'Arabic' },
  { code: 'fr', name: 'French' },
  { code: 'de', name: 'German' },
]

const categories = [
  { value: 'UTILITY', label: 'Utility', description: 'Order updates, account alerts' },
  { value: 'MARKETING', label: 'Marketing', description: 'Promotions, offers' },
  { value: 'AUTHENTICATION', label: 'Authentication', description: 'OTP, verification codes' },
]

const headerTypes = [
  { value: 'NONE', label: 'None' },
  { value: 'TEXT', label: 'Text' },
  { value: 'IMAGE', label: 'Image' },
  { value: 'VIDEO', label: 'Video' },
  { value: 'DOCUMENT', label: 'Document' },
]

// Refetch data when organization changes
watch(() => organizationsStore.selectedOrgId, async () => {
  await fetchAccounts()
  await fetchTemplates()
})

onMounted(async () => {
  await fetchAccounts()
  await fetchTemplates()
})

async function fetchAccounts() {
  try {
    const response = await api.get('/accounts')
    accounts.value = response.data.data?.accounts || []
    // Validate stored account still exists, fallback to 'all' if not
    if (selectedAccount.value !== 'all' && !accounts.value.some(a => a.name === selectedAccount.value)) {
      selectedAccount.value = 'all'
      localStorage.setItem('templates_selected_account', 'all')
    }
  } catch (error) {
    console.error('Failed to fetch accounts:', error)
  }
}

function onAccountChange(value: string | number | bigint | Record<string, any> | null) {
  if (typeof value !== 'string') return
  localStorage.setItem('templates_selected_account', value)
  currentPage.value = 1
  fetchTemplates()
}

async function fetchTemplates() {
  isLoading.value = true
  try {
    const response = await templatesService.list({
      account: selectedAccount.value !== 'all' ? selectedAccount.value : undefined,
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    const data = (response.data as any).data || response.data
    templates.value = data.templates || []
    totalItems.value = data.total ?? templates.value.length
  } catch (error: any) {
    console.error('Failed to fetch templates:', error)
    toast.error(t('common.failedLoad', { resource: t('resources.templates') }))
    templates.value = []
  } finally {
    isLoading.value = false
  }
}

// Debounced search
const debouncedSearch = useDebounceFn(() => {
  currentPage.value = 1
  fetchTemplates()
}, 300)

watch(searchQuery, () => debouncedSearch())

function handlePageChange(page: number) {
  currentPage.value = page
  fetchTemplates()
}

async function syncTemplates() {
  if (!selectedAccount.value || selectedAccount.value === 'all') {
    toast.error(t('templates.selectAccountFirst'))
    return
  }

  isSyncing.value = true
  try {
    const response = await api.post('/templates/sync', {
      whatsapp_account: selectedAccount.value
    })
    toast.success(response.data.data.message || t('templates.syncSuccess'))
    await fetchTemplates()
  } catch (error) {
    toast.error(getErrorMessage(error, t('templates.syncFailed')))
  } finally {
    isSyncing.value = false
  }
}

function openCreateDialog() {
  editingTemplate.value = null
  formData.value = {
    whatsapp_account: (selectedAccount.value && selectedAccount.value !== 'all') ? selectedAccount.value : (accounts.value[0]?.name || ''),
    name: '',
    display_name: '',
    language: 'en',
    category: 'UTILITY',
    header_type: 'NONE',
    header_content: '',
    body_content: '',
    footer_content: '',
    buttons: [],
    sample_values: []
  }
  // Reset header media state
  headerMediaFile.value = null
  headerMediaHandle.value = ''
  headerMediaFilename.value = ''
  isDialogOpen.value = true
}

function openEditDialog(template: Template) {
  editingTemplate.value = template
  formData.value = {
    whatsapp_account: template.whatsapp_account,
    name: template.name,
    display_name: template.display_name,
    language: template.language,
    category: template.category,
    header_type: template.header_type || 'NONE',
    header_content: template.header_content || '',
    body_content: template.body_content,
    footer_content: template.footer_content || '',
    buttons: template.buttons || [],
    sample_values: template.sample_values || []
  }
  // Reset header media state (will show existing handle if present)
  headerMediaFile.value = null
  headerMediaHandle.value = template.header_content || ''
  headerMediaFilename.value = ''
  isDialogOpen.value = true
}

function openPreview(template: Template) {
  previewTemplate.value = template
  isPreviewOpen.value = true
}

async function saveTemplate() {
  if (!formData.value.name.trim() || !formData.value.body_content.trim()) {
    toast.error(t('templates.nameBodyRequired'))
    return
  }

  if (!formData.value.whatsapp_account) {
    toast.error(t('templates.selectAccountRequired'))
    return
  }

  isSubmitting.value = true
  try {
    if (editingTemplate.value) {
      await api.put(`/templates/${editingTemplate.value.id}`, formData.value)
      toast.success(t('common.updatedSuccess', { resource: t('resources.Template') }))
    } else {
      await api.post('/templates', formData.value)
      toast.success(t('common.createdSuccess', { resource: t('resources.Template') }))
    }
    isDialogOpen.value = false
    await fetchTemplates()
  } catch (error) {
    toast.error(getErrorMessage(error, t('common.failedSave', { resource: t('resources.template') })))
  } finally {
    isSubmitting.value = false
  }
}

function openDeleteDialog(template: Template) {
  templateToDelete.value = template
  deleteDialogOpen.value = true
}

async function confirmDeleteTemplate() {
  if (!templateToDelete.value) return

  try {
    await api.delete(`/templates/${templateToDelete.value.id}`)
    toast.success(t('common.deletedSuccess', { resource: t('resources.Template') }))
    deleteDialogOpen.value = false
    templateToDelete.value = null
    await fetchTemplates()
  } catch (error) {
    toast.error(getErrorMessage(error, t('common.failedDelete', { resource: t('resources.template') })))
  }
}

const publishingTemplateId = ref<string | null>(null)

function openPublishDialog(template: Template) {
  templateToPublish.value = template
  publishDialogOpen.value = true
}

async function confirmPublishTemplate() {
  if (!templateToPublish.value) return

  publishingTemplateId.value = templateToPublish.value.id
  try {
    const response = await api.post(`/templates/${templateToPublish.value.id}/publish`)
    toast.success(response.data.data?.message || t('templates.publishSuccess'))
    publishDialogOpen.value = false
    templateToPublish.value = null
    await fetchTemplates()
  } catch (error) {
    toast.error(getErrorMessage(error, t('templates.publishFailed')), { duration: 8000 })
  } finally {
    publishingTemplateId.value = null
  }
}

// Dark-first: default is dark mode, light: prefix for light mode
function getStatusBadgeClass(status: string) {
  switch (status) {
    case 'APPROVED':
      return 'bg-green-900 text-green-300 light:bg-green-100 light:text-green-800'
    case 'PENDING':
      return 'bg-yellow-900 text-yellow-300 light:bg-yellow-100 light:text-yellow-800'
    case 'REJECTED':
      return 'bg-red-900 text-red-300 light:bg-red-100 light:text-red-800'
    case 'DRAFT':
      return 'bg-gray-800 text-gray-300 light:bg-gray-100 light:text-gray-800'
    default:
      return 'bg-gray-800 text-gray-300 light:bg-gray-100 light:text-gray-800'
  }
}

function getCategoryBadgeClass(category: string) {
  switch (category) {
    case 'UTILITY':
      return 'bg-blue-900 text-blue-300 light:bg-blue-100 light:text-blue-800'
    case 'MARKETING':
      return 'bg-purple-900 text-purple-300 light:bg-purple-100 light:text-purple-800'
    case 'AUTHENTICATION':
      return 'bg-orange-900 text-orange-300 light:bg-orange-100 light:text-orange-800'
    default:
      return 'bg-gray-800 text-gray-300 light:bg-gray-100 light:text-gray-800'
  }
}

function getHeaderIcon(type: string) {
  switch (type) {
    case 'IMAGE':
      return Image
    case 'VIDEO':
      return Video
    case 'DOCUMENT':
      return FileIcon
    default:
      return MessageSquare
  }
}


// Extract all parameter names (both positional {{1}} and named {{name}})
function extractParamNames(content: string): string[] {
  const matches = content.match(/\{\{([^}]+)\}\}/g) || []
  const seen = new Set<string>()
  const names: string[] = []
  for (const m of matches) {
    const name = m.replace(/[{}]/g, '').trim()
    if (name && !seen.has(name)) {
      seen.add(name)
      names.push(name)
    }
  }
  return names
}

// Get variable names from body content (supports both {{1}} and {{name}})
const bodyVariables = computed(() => {
  return extractParamNames(formData.value.body_content)
})

// Get variable names from header content
const headerVariables = computed(() => {
  if (formData.value.header_type !== 'TEXT') return []
  return extractParamNames(formData.value.header_content)
})

// Button types for template
const buttonTypes = [
  { value: 'QUICK_REPLY', label: 'Quick Reply', description: 'Simple reply button' },
  { value: 'URL', label: 'URL', description: 'Opens a website' },
  { value: 'PHONE_NUMBER', label: 'Phone Number', description: 'Calls a number' },
]

function addButton() {
  if (formData.value.buttons.length >= 3) {
    toast.error(t('templates.maxButtons'))
    return
  }
  formData.value.buttons.push({
    type: 'QUICK_REPLY',
    text: ''
  })
}

function removeButton(index: number) {
  formData.value.buttons.splice(index, 1)
}

// Handle header media file selection
function onHeaderMediaFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  if (input.files && input.files.length > 0) {
    headerMediaFile.value = input.files[0]
    headerMediaFilename.value = input.files[0].name
    // Clear previous handle when new file is selected
    headerMediaHandle.value = ''
    formData.value.header_content = ''
  }
}

// Upload header media file to Meta
async function uploadHeaderMedia() {
  if (!headerMediaFile.value) {
    toast.error(t('templates.selectAccountFirst'))
    return
  }

  if (!formData.value.whatsapp_account) {
    toast.error(t('templates.selectAccountFirst'))
    return
  }

  headerMediaUploading.value = true
  try {
    const response = await templatesService.uploadMedia(formData.value.whatsapp_account, headerMediaFile.value)
    const data = response.data.data
    headerMediaHandle.value = data.handle
    formData.value.header_content = data.handle
    toast.success(t('templates.mediaUploadedSuccess'))
  } catch (error) {
    toast.error(getErrorMessage(error, t('templates.uploadFailed')))
  } finally {
    headerMediaUploading.value = false
  }
}

// Get accepted file types for header type
function getAcceptedFileTypes(): string {
  switch (formData.value.header_type) {
    case 'IMAGE':
      return 'image/jpeg,image/png'
    case 'VIDEO':
      return 'video/mp4'
    case 'DOCUMENT':
      return 'application/pdf'
    default:
      return '*/*'
  }
}

function getSampleValue(component: string, paramName: string): string {
  const sample = formData.value.sample_values.find(
    (s: any) => s.component === component && s.param_name === paramName
  )
  return sample?.value || ''
}

function setSampleValue(component: string, paramName: string, value: string) {
  const existingIndex = formData.value.sample_values.findIndex(
    (s: any) => s.component === component && s.param_name === paramName
  )
  if (existingIndex >= 0) {
    formData.value.sample_values[existingIndex].value = value
  } else {
    formData.value.sample_values.push({ component, param_name: paramName, value })
  }
}

function formatVariableLabel(paramName: string): string {
  return `{{${paramName}}}`
}

// Format template preview with sample values (sanitized to prevent XSS)
function formatPreview(text: string, samples: any[]): string {
  // Sanitize the base text first
  let result = DOMPurify.sanitize(text, { ALLOWED_TAGS: [] })

  // Handle named parameters with param_name field
  samples.forEach((sample) => {
    if (sample && sample.param_name && sample.value) {
      const sanitizedSample = DOMPurify.sanitize(String(sample.value), { ALLOWED_TAGS: [] })
      result = result.replace(`{{${sample.param_name}}}`, `<span class="bg-green-900 light:bg-green-100 px-1 rounded">${sanitizedSample}</span>`)
    }
  })

  // Replace remaining variables (both named and positional)
  result = result.replace(/\{\{([^}]+)\}\}/g, '<span class="bg-yellow-900 light:bg-yellow-100 px-1 rounded">{{$1}}</span>')
  return result
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('templates.title')" :subtitle="$t('templates.subtitle')" :icon="FileText" icon-gradient="bg-gradient-to-br from-blue-500 to-cyan-600 shadow-blue-500/20">
      <template #actions>
        <Button variant="outline" size="sm" @click="syncTemplates" :disabled="isSyncing || !selectedAccount || selectedAccount === 'all'">
          <Loader2 v-if="isSyncing" class="h-4 w-4 mr-2 animate-spin" />
          <RefreshCw v-else class="h-4 w-4 mr-2" />
          {{ $t('templates.syncFromMeta') }}
        </Button>
        <Button variant="outline" size="sm" @click="openCreateDialog">
          <Plus class="h-4 w-4 mr-2" />
          {{ $t('templates.createTemplate') }}
        </Button>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto">
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>{{ $t('templates.yourTemplates') }}</CardTitle>
                  <CardDescription>{{ $t('templates.yourTemplatesDesc') }}</CardDescription>
                </div>
                <div class="flex items-center gap-4 flex-wrap">
                  <div class="flex items-center gap-2">
                    <Label class="text-sm text-muted-foreground">{{ $t('templates.account') }}:</Label>
                    <Select v-model="selectedAccount" @update:model-value="onAccountChange">
                      <SelectTrigger class="w-[180px]">
                        <SelectValue :placeholder="$t('templates.allAccounts')" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="all">{{ $t('templates.allAccounts') }}</SelectItem>
                        <SelectItem v-for="account in accounts" :key="account.id" :value="account.name">
                          {{ account.name }}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <SearchInput v-model="searchQuery" :placeholder="$t('templates.searchTemplates') + '...'" class="w-64" />
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <DataTable
                :items="templates"
                :columns="columns"
                :is-loading="isLoading"
                :empty-icon="FileText"
                :empty-title="$t('templates.noTemplatesFound')"
                :empty-description="$t('templates.noTemplatesFoundDesc')"
                server-pagination
                :current-page="currentPage"
                :total-items="totalItems"
                :page-size="pageSize"
                item-name="templates"
                @page-change="handlePageChange"
                v-model:sort-key="sortKey"
                v-model:sort-direction="sortDirection"
              >
                <template #cell-name="{ item: template }">
                  <div>
                    <span class="font-medium">{{ template.display_name || template.name }}</span>
                    <p class="text-xs font-mono text-muted-foreground">{{ template.name }}</p>
                  </div>
                </template>
                <template #cell-category="{ item: template }">
                  <Badge :class="getCategoryBadgeClass(template.category)" class="text-xs">
                    {{ template.category }}
                  </Badge>
                </template>
                <template #cell-status="{ item: template }">
                  <Badge :class="getStatusBadgeClass(template.status)" class="text-xs">
                    {{ template.status }}
                  </Badge>
                </template>
                <template #cell-language="{ item: template }">
                  <span class="text-muted-foreground">{{ template.language }}</span>
                </template>
                <template #cell-header_type="{ item: template }">
                  <div class="flex items-center gap-1">
                    <component :is="getHeaderIcon(template.header_type)" class="h-4 w-4 text-muted-foreground" />
                    <span class="text-muted-foreground text-sm">{{ template.header_type || 'NONE' }}</span>
                  </div>
                </template>
                <template #cell-actions="{ item: template }">
                  <div class="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click="openPreview(template)">
                      <Eye class="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      class="h-8 w-8"
                      @click="openEditDialog(template)"
                      :disabled="template.status === 'PENDING'"
                    >
                      <Pencil class="h-4 w-4" />
                    </Button>
                    <Button
                      v-if="template.status === 'DRAFT' || template.status === 'REJECTED'"
                      variant="ghost"
                      size="icon"
                      class="h-8 w-8 text-blue-600 hover:text-blue-700"
                      @click="openPublishDialog(template)"
                      :disabled="publishingTemplateId === template.id"
                    >
                      <Loader2 v-if="publishingTemplateId === template.id" class="h-4 w-4 animate-spin" />
                      <Send v-else class="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="icon" class="h-8 w-8 text-destructive" @click="openDeleteDialog(template)">
                      <Trash2 class="h-4 w-4" />
                    </Button>
                  </div>
                </template>
                <template #empty-action>
                  <div class="flex items-center justify-center gap-2">
                    <Button variant="outline" size="sm" @click="syncTemplates" :disabled="!selectedAccount || selectedAccount === 'all'">
                      <RefreshCw class="h-4 w-4 mr-2" />
                      {{ $t('templates.syncFromMeta') }}
                    </Button>
                    <Button variant="outline" size="sm" @click="openCreateDialog">
                      <Plus class="h-4 w-4 mr-2" />
                      {{ $t('templates.createTemplate') }}
                    </Button>
                  </div>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <!-- Create/Edit Dialog -->
    <Dialog v-model:open="isDialogOpen">
      <DialogContent class="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{{ editingTemplate ? $t('templates.editDialogTitle') : $t('templates.createDialogTitle') }}</DialogTitle>
          <DialogDescription>
            {{ editingTemplate ? $t('templates.editDialogDesc') : $t('templates.createDialogDesc') }}
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-4">
          <!-- Account Selection -->
          <div class="space-y-2">
            <Label>{{ $t('templates.whatsappAccount') }} <span class="text-destructive">*</span></Label>
            <select
              v-model="formData.whatsapp_account"
              class="w-full h-10 rounded-md border bg-background px-3 disabled:cursor-not-allowed disabled:opacity-50"
              :disabled="!!editingTemplate"
            >
              <option value="">{{ $t('templates.selectAccount') }}...</option>
              <option v-for="account in accounts" :key="account.id" :value="account.name">
                {{ account.name }}
              </option>
            </select>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <!-- Template Name -->
            <div class="space-y-2">
              <Label>{{ $t('templates.templateName') }} <span class="text-destructive">*</span></Label>
              <Input
                v-model="formData.name"
                placeholder="order_confirmation"
                :disabled="!!editingTemplate"
              />
              <p class="text-xs text-muted-foreground">{{ $t('templates.templateNameLowercase') }}</p>
            </div>

            <!-- Display Name -->
            <div class="space-y-2">
              <Label>{{ $t('templates.displayName') }}</Label>
              <Input
                v-model="formData.display_name"
                placeholder="Order Confirmation"
              />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <!-- Language -->
            <div class="space-y-2">
              <Label>{{ $t('templates.language') }} <span class="text-destructive">*</span></Label>
              <select
                v-model="formData.language"
                class="w-full h-10 rounded-md border bg-background px-3 disabled:cursor-not-allowed disabled:opacity-50"
                :disabled="!!editingTemplate"
              >
                <option v-for="lang in languages" :key="lang.code" :value="lang.code">
                  {{ lang.name }}
                </option>
              </select>
            </div>

            <!-- Category -->
            <div class="space-y-2">
              <Label>{{ $t('templates.category') }} <span class="text-destructive">*</span></Label>
              <select
                v-model="formData.category"
                class="w-full h-10 rounded-md border bg-background px-3 disabled:cursor-not-allowed disabled:opacity-50"
                :disabled="!!editingTemplate"
              >
                <option v-for="cat in categories" :key="cat.value" :value="cat.value">
                  {{ cat.label }} - {{ cat.description }}
                </option>
              </select>
            </div>
          </div>

          <Separator />

          <!-- Header -->
          <div class="space-y-2">
            <Label>{{ $t('templates.headerType') }}</Label>
            <select v-model="formData.header_type" class="w-full h-10 rounded-md border bg-background px-3">
              <option v-for="type in headerTypes" :key="type.value" :value="type.value">
                {{ type.label }}
              </option>
            </select>
          </div>

          <div v-if="formData.header_type === 'TEXT'" class="space-y-2">
            <Label>{{ $t('templates.headerText') }}</Label>
            <Input v-model="formData.header_content" :placeholder="$t('templates.headerTextPlaceholder') + '...'" />
          </div>

          <!-- Header Media Upload for IMAGE/VIDEO/DOCUMENT -->
          <div v-else-if="['IMAGE', 'VIDEO', 'DOCUMENT'].includes(formData.header_type)" class="space-y-3">
            <Label>{{ $t('templates.headerSample') }} {{ formData.header_type.toLowerCase() }}</Label>
            <p class="text-xs text-muted-foreground">
              {{ $t('templates.uploadSampleHint', { type: formData.header_type.toLowerCase() }) }}
            </p>

            <div class="flex items-center gap-2">
              <div class="flex-1">
                <input
                  type="file"
                  :accept="getAcceptedFileTypes()"
                  @change="onHeaderMediaFileChange"
                  class="w-full text-sm file:mr-4 file:py-2 file:px-4 file:rounded-md file:border-0 file:text-sm file:font-medium file:bg-primary file:text-primary-foreground hover:file:bg-primary/90 cursor-pointer"
                />
              </div>
              <Button
                type="button"
                size="sm"
                @click="uploadHeaderMedia"
                :disabled="!headerMediaFile || headerMediaUploading || !formData.whatsapp_account"
              >
                <Loader2 v-if="headerMediaUploading" class="h-4 w-4 mr-1 animate-spin" />
                <Upload v-else class="h-4 w-4 mr-1" />
                {{ $t('templates.uploadMedia') }}
              </Button>
            </div>

            <!-- Show upload status -->
            <div v-if="headerMediaFilename && !headerMediaHandle" class="text-sm text-muted-foreground">
              {{ $t('templates.selectedFile', { filename: headerMediaFilename }) }}
            </div>

            <!-- Show uploaded handle -->
            <div v-if="headerMediaHandle" class="bg-green-950 light:bg-green-50 border border-green-800 light:border-green-200 rounded-lg p-3">
              <div class="flex items-center gap-2">
                <Check class="h-4 w-4 text-green-600" />
                <span class="text-sm text-green-200 light:text-green-800">{{ $t('templates.mediaUploadedSuccess') }}</span>
              </div>
              <p class="text-xs text-muted-foreground mt-1 font-mono truncate">
                Handle: {{ headerMediaHandle.substring(0, 40) }}...
              </p>
            </div>

            <!-- Accepted formats hint -->
            <p class="text-xs text-muted-foreground">
              <span v-if="formData.header_type === 'IMAGE'">{{ $t('templates.imageFormats') }}</span>
              <span v-else-if="formData.header_type === 'VIDEO'">{{ $t('templates.videoFormats') }}</span>
              <span v-else-if="formData.header_type === 'DOCUMENT'">{{ $t('templates.documentFormats') }}</span>
            </p>
          </div>

          <!-- Body -->
          <div class="space-y-2">
            <Label>{{ $t('templates.bodyContent') }} <span class="text-destructive">*</span></Label>
            <Textarea
              v-model="formData.body_content"
              :placeholder="$t('templates.bodyPlaceholder')"
              :rows="4"
            />
            <p class="text-xs text-muted-foreground">
              {{ $t('templates.bodyVariablesHint') }}
            </p>
          </div>

          <!-- Footer -->
          <div class="space-y-2">
            <Label>{{ $t('templates.footerOptional') }}</Label>
            <Input v-model="formData.footer_content" :placeholder="$t('templates.footerPlaceholder')" />
          </div>

          <Separator />

          <!-- Buttons -->
          <div class="space-y-3">
            <div class="flex items-center justify-between">
              <Label>{{ $t('templates.buttonsOptional') }}</Label>
              <Button
                type="button"
                variant="outline"
                size="sm"
                @click="addButton"
                :disabled="formData.buttons.length >= 3"
              >
                <Plus class="h-4 w-4 mr-1" />
                {{ $t('templates.addButton') }}
              </Button>
            </div>
            <p class="text-xs text-muted-foreground">{{ $t('templates.maxButtonsHint') }}</p>

            <div v-for="(button, index) in formData.buttons" :key="index" class="border rounded-lg p-3 space-y-3">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">{{ $t('templates.button') }} {{ index + 1 }}</span>
                <Button type="button" variant="ghost" size="sm" @click="removeButton(index)">
                  <X class="h-4 w-4 text-destructive" />
                </Button>
              </div>

              <div class="grid grid-cols-2 gap-3">
                <div class="space-y-1">
                  <Label class="text-xs">{{ $t('templates.buttonType') }}</Label>
                  <select v-model="button.type" class="w-full h-9 rounded-md border bg-background px-2 text-sm">
                    <option v-for="bt in buttonTypes" :key="bt.value" :value="bt.value">
                      {{ bt.label }}
                    </option>
                  </select>
                </div>
                <div class="space-y-1">
                  <Label class="text-xs">{{ $t('templates.buttonText') }}</Label>
                  <Input v-model="button.text" :placeholder="$t('templates.buttonTextPlaceholder')" class="h-9" />
                </div>
              </div>

              <!-- URL specific fields -->
              <div v-if="button.type === 'URL'" class="space-y-1">
                <Label class="text-xs">{{ $t('templates.buttonUrl') }}</Label>
                <Input v-model="button.url" placeholder="https://example.com/{{path}}" class="h-9" />
                <p class="text-xs text-muted-foreground">{{ $t('templates.buttonUrlHint') }}</p>
              </div>

              <!-- Phone number specific fields -->
              <div v-if="button.type === 'PHONE_NUMBER'" class="space-y-1">
                <Label class="text-xs">{{ $t('templates.buttonPhoneNumber') }}</Label>
                <Input v-model="button.phone_number" placeholder="+1234567890" class="h-9" />
              </div>
            </div>
          </div>

          <Separator />

          <!-- Sample Values for Variables -->
          <div v-if="bodyVariables.length > 0 || headerVariables.length > 0" class="space-y-3">
            <div>
              <Label>{{ $t('templates.sampleValues') }}</Label>
              <p class="text-xs text-muted-foreground mt-1">
                {{ $t('templates.sampleValuesHint') }}
              </p>
            </div>

            <!-- Header Variables -->
            <div v-if="headerVariables.length > 0" class="space-y-2">
              <p class="text-sm font-medium text-muted-foreground">{{ $t('templates.headerVariables') }}</p>
              <div v-for="paramName in headerVariables" :key="'header-' + paramName" class="flex items-center gap-2">
                <span class="text-sm font-mono bg-muted px-2 py-1 rounded min-w-[80px] text-center">{{ formatVariableLabel(paramName) }}</span>
                <input
                  type="text"
                  :value="getSampleValue('header', paramName)"
                  @input="setSampleValue('header', paramName, ($event.target as HTMLInputElement).value)"
                  :placeholder="$t('templates.exampleFor', { name: paramName }) + '...'"
                  class="flex-1 h-9 rounded-md border border-input bg-background px-3 text-sm"
                />
              </div>
            </div>

            <!-- Body Variables -->
            <div v-if="bodyVariables.length > 0" class="space-y-2">
              <p class="text-sm font-medium text-muted-foreground">{{ $t('templates.bodyVariables') }}</p>
              <div v-for="paramName in bodyVariables" :key="'body-' + paramName" class="flex items-center gap-2">
                <span class="text-sm font-mono bg-muted px-2 py-1 rounded min-w-[80px] text-center">{{ formatVariableLabel(paramName) }}</span>
                <input
                  type="text"
                  :value="getSampleValue('body', paramName)"
                  @input="setSampleValue('body', paramName, ($event.target as HTMLInputElement).value)"
                  :placeholder="$t('templates.exampleFor', { name: paramName }) + '...'"
                  class="flex-1 h-9 rounded-md border border-input bg-background px-3 text-sm"
                />
              </div>
            </div>
          </div>

          <!-- Info Box -->
          <div class="bg-blue-950 light:bg-blue-50 border border-blue-800 light:border-blue-200 rounded-lg p-4">
            <div class="flex gap-3">
              <AlertCircle class="h-5 w-5 text-blue-400 light:text-blue-600 flex-shrink-0" />
              <div class="text-sm text-blue-200 light:text-blue-800">
                <p class="font-medium">{{ $t('templates.templateSubmission') }}</p>
                <p class="mt-1">
                  {{ $t('templates.templateSubmissionHint') }}
                </p>
              </div>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" @click="isDialogOpen = false">{{ $t('common.cancel') }}</Button>
          <Button size="sm" @click="saveTemplate" :disabled="isSubmitting">
            <Loader2 v-if="isSubmitting" class="h-4 w-4 mr-2 animate-spin" />
            {{ editingTemplate ? $t('templates.updateTemplate') : $t('templates.createTemplate') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Preview Dialog -->
    <Dialog v-model:open="isPreviewOpen">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>{{ $t('templates.templatePreview') }}</DialogTitle>
          <DialogDescription>
            {{ previewTemplate?.display_name || previewTemplate?.name }}
          </DialogDescription>
        </DialogHeader>

        <div v-if="previewTemplate" class="py-4">
          <!-- WhatsApp-style preview -->
          <div class="bg-gray-800 light:bg-[#e5ddd5] rounded-lg p-4">
            <div class="bg-gray-700 light:bg-white rounded-lg shadow max-w-[280px] overflow-hidden">
              <!-- Header -->
              <div v-if="previewTemplate.header_type && previewTemplate.header_type !== 'NONE'" class="p-3 border-b">
                <div v-if="previewTemplate.header_type === 'TEXT'" class="font-semibold">
                  {{ previewTemplate.header_content }}
                </div>
                <div v-else class="h-32 bg-gray-600 light:bg-gray-200 rounded flex items-center justify-center">
                  <component :is="getHeaderIcon(previewTemplate.header_type)" class="h-8 w-8 text-gray-400" />
                </div>
              </div>

              <!-- Body -->
              <div class="p-3">
                <p class="text-sm whitespace-pre-wrap" v-html="formatPreview(previewTemplate.body_content, previewTemplate.sample_values || [])"></p>
              </div>

              <!-- Footer -->
              <div v-if="previewTemplate.footer_content" class="px-3 pb-3">
                <p class="text-xs text-gray-500">{{ previewTemplate.footer_content }}</p>
              </div>

              <!-- Buttons -->
              <div v-if="previewTemplate.buttons && previewTemplate.buttons.length > 0" class="border-t">
                <div v-for="(btn, idx) in previewTemplate.buttons" :key="idx" class="border-b last:border-b-0">
                  <button class="w-full py-2 text-sm text-blue-500 hover:bg-gray-600 light:hover:bg-gray-50">
                    {{ btn.text || btn.title || 'Button' }}
                  </button>
                </div>
              </div>
            </div>
          </div>

          <!-- Template Info -->
          <div class="mt-4 space-y-2 text-sm">
            <div class="flex justify-between">
              <span class="text-muted-foreground">{{ $t('templates.status') }}:</span>
              <span :class="['px-2 py-0.5 rounded text-xs font-medium', getStatusBadgeClass(previewTemplate.status)]">
                {{ previewTemplate.status }}
              </span>
            </div>
            <div class="flex justify-between">
              <span class="text-muted-foreground">{{ $t('templates.category') }}:</span>
              <span>{{ previewTemplate.category }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-muted-foreground">{{ $t('templates.language') }}:</span>
              <span>{{ previewTemplate.language }}</span>
            </div>
            <div v-if="previewTemplate.meta_template_id" class="flex justify-between">
              <span class="text-muted-foreground">{{ $t('templates.metaId') }}:</span>
              <span class="font-mono text-xs">{{ previewTemplate.meta_template_id }}</span>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" @click="isPreviewOpen = false">{{ $t('common.close') }}</Button>
          <Button
            v-if="previewTemplate?.status === 'DRAFT' || previewTemplate?.status === 'REJECTED'"
            size="sm"
            @click="openPublishDialog(previewTemplate!); isPreviewOpen = false"
            :disabled="publishingTemplateId === previewTemplate?.id"
          >
            <Loader2 v-if="publishingTemplateId === previewTemplate?.id" class="h-4 w-4 mr-2 animate-spin" />
            <Send v-else class="h-4 w-4 mr-2" />
            {{ previewTemplate?.meta_template_id ? $t('templates.republishToMeta') : $t('templates.publishToMeta') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <DeleteConfirmDialog
      v-model:open="deleteDialogOpen"
      :title="$t('templates.deleteTemplate')"
      :item-name="templateToDelete?.display_name || templateToDelete?.name"
      @confirm="confirmDeleteTemplate"
    />

    <!-- Publish Confirmation Dialog -->
    <AlertDialog v-model:open="publishDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ templateToPublish?.meta_template_id ? $t('templates.republishTemplate') : $t('templates.publishTemplate') }}</AlertDialogTitle>
          <AlertDialogDescription>
            <template v-if="templateToPublish?.meta_template_id">
              {{ $t('templates.republishConfirm', { name: templateToPublish?.display_name || templateToPublish?.name }) }}
            </template>
            <template v-else>
              {{ $t('templates.publishConfirm', { name: templateToPublish?.display_name || templateToPublish?.name }) }}
            </template>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('common.cancel') }}</AlertDialogCancel>
          <AlertDialogAction @click="confirmPublishTemplate">{{ templateToPublish?.meta_template_id ? $t('templates.republish') : $t('templates.publish') }}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
