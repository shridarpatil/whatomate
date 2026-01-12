<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import DOMPurify from 'dompurify'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Textarea } from '@/components/ui/textarea'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { api, templatesService } from '@/services/api'
import { toast } from 'vue-sonner'
import {
  Plus,
  Search,
  RefreshCw,
  FileText,
  Eye,
  Pencil,
  Trash2,
  Loader2,
  MessageSquare,
  Image,
  FileIcon,
  Video,
  X,
  Check,
  AlertCircle,
  Send,
  Upload
} from 'lucide-vue-next'

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

function onAccountChange(value: string) {
  localStorage.setItem('templates_selected_account', value)
  fetchTemplates()
}

async function fetchTemplates() {
  isLoading.value = true
  try {
    const params = selectedAccount.value && selectedAccount.value !== 'all' ? `?account=${selectedAccount.value}` : ''
    const response = await api.get(`/templates${params}`)
    templates.value = response.data.data?.templates || []
  } catch (error: any) {
    console.error('Failed to fetch templates:', error)
    toast.error('Failed to load templates')
    templates.value = []
  } finally {
    isLoading.value = false
  }
}

async function syncTemplates() {
  if (!selectedAccount.value || selectedAccount.value === 'all') {
    toast.error('Please select a WhatsApp account first')
    return
  }

  isSyncing.value = true
  try {
    const response = await api.post('/templates/sync', {
      whatsapp_account: selectedAccount.value
    })
    toast.success(response.data.data.message || 'Templates synced successfully')
    await fetchTemplates()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to sync templates'
    toast.error(message)
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
    toast.error('Template name and body content are required')
    return
  }

  if (!formData.value.whatsapp_account) {
    toast.error('Please select a WhatsApp account')
    return
  }

  isSubmitting.value = true
  try {
    console.log('Saving template with data:', JSON.stringify(formData.value, null, 2))
    if (editingTemplate.value) {
      await api.put(`/templates/${editingTemplate.value.id}`, formData.value)
      toast.success('Template updated successfully')
    } else {
      await api.post('/templates', formData.value)
      toast.success('Template created successfully')
    }
    isDialogOpen.value = false
    await fetchTemplates()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to save template'
    toast.error(message)
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
    toast.success('Template deleted')
    deleteDialogOpen.value = false
    templateToDelete.value = null
    await fetchTemplates()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to delete template'
    toast.error(message)
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
    toast.success(response.data.data?.message || 'Template submitted to Meta for approval')
    publishDialogOpen.value = false
    templateToPublish.value = null
    await fetchTemplates()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to publish template'
    toast.error(message, { duration: 8000 })
  } finally {
    publishingTemplateId.value = null
  }
}

function getStatusBadgeClass(status: string) {
  switch (status) {
    case 'APPROVED':
      return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300'
    case 'PENDING':
      return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300'
    case 'REJECTED':
      return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300'
    case 'DRAFT':
      return 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300'
    default:
      return 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300'
  }
}

function getCategoryBadgeClass(category: string) {
  switch (category) {
    case 'UTILITY':
      return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300'
    case 'MARKETING':
      return 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-300'
    case 'AUTHENTICATION':
      return 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-300'
    default:
      return 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300'
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

const filteredTemplates = computed(() => {
  if (!searchQuery.value) return templates.value
  const query = searchQuery.value.toLowerCase()
  return templates.value.filter(t =>
    t.name.toLowerCase().includes(query) ||
    t.display_name?.toLowerCase().includes(query) ||
    t.body_content.toLowerCase().includes(query)
  )
})

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
    toast.error('Maximum 3 buttons allowed')
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
    toast.error('Please select a file first')
    return
  }

  if (!formData.value.whatsapp_account) {
    toast.error('Please select a WhatsApp account first')
    return
  }

  headerMediaUploading.value = true
  try {
    const response = await templatesService.uploadMedia(formData.value.whatsapp_account, headerMediaFile.value)
    const data = response.data.data
    headerMediaHandle.value = data.handle
    formData.value.header_content = data.handle
    toast.success(`Media uploaded: ${data.filename}`)
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to upload media'
    toast.error(message)
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
      result = result.replace(`{{${sample.param_name}}}`, `<span class="bg-green-100 dark:bg-green-900 px-1 rounded">${sanitizedSample}</span>`)
    }
  })

  // Replace remaining variables (both named and positional)
  result = result.replace(/\{\{([^}]+)\}\}/g, '<span class="bg-yellow-100 dark:bg-yellow-900 px-1 rounded">{{$1}}</span>')
  return result
}
</script>

<template>
  <div class="flex flex-col h-full">
    <!-- Header -->
    <header class="border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div class="flex h-16 items-center px-6">
        <FileText class="h-5 w-5 mr-3" />
        <div class="flex-1">
          <h1 class="text-xl font-semibold">Message Templates</h1>
          <p class="text-sm text-muted-foreground">Create and manage WhatsApp message templates</p>
        </div>
        <div class="flex items-center gap-2">
          <Button variant="outline" size="sm" @click="syncTemplates" :disabled="isSyncing || !selectedAccount || selectedAccount === 'all'">
            <Loader2 v-if="isSyncing" class="h-4 w-4 mr-2 animate-spin" />
            <RefreshCw v-else class="h-4 w-4 mr-2" />
            Sync from Meta
          </Button>
          <Button variant="outline" size="sm" @click="openCreateDialog">
            <Plus class="h-4 w-4 mr-2" />
            Create Template
          </Button>
        </div>
      </div>
    </header>

    <!-- Filters -->
    <div class="p-4 border-b flex items-center gap-4 flex-wrap">
      <div class="flex items-center gap-2">
        <Label class="text-sm text-muted-foreground">Account:</Label>
        <Select v-model="selectedAccount" @update:model-value="onAccountChange">
          <SelectTrigger class="w-[180px]">
            <SelectValue placeholder="All Accounts" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Accounts</SelectItem>
            <SelectItem v-for="account in accounts" :key="account.id" :value="account.name">
              {{ account.name }}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div class="relative flex-1 max-w-md">
        <Search class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input v-model="searchQuery" placeholder="Search templates..." class="pl-9" />
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="isLoading" class="flex-1 flex items-center justify-center">
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <!-- Templates Grid -->
    <ScrollArea v-else class="flex-1">
      <div class="p-6 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        <Card v-for="template in filteredTemplates" :key="template.id" class="flex flex-col">
          <CardHeader class="pb-3">
            <div class="flex items-start justify-between">
              <div class="flex-1 min-w-0">
                <CardTitle class="text-base truncate">{{ template.display_name || template.name }}</CardTitle>
                <p class="text-xs font-mono text-muted-foreground truncate mt-1">{{ template.name }}</p>
                <div class="flex items-center gap-2 mt-2 flex-wrap">
                  <span :class="['px-2 py-0.5 rounded text-xs font-medium', getCategoryBadgeClass(template.category)]">
                    {{ template.category }}
                  </span>
                  <span :class="['px-2 py-0.5 rounded text-xs font-medium', getStatusBadgeClass(template.status)]">
                    {{ template.status }}
                  </span>
                  <span class="text-xs text-muted-foreground">{{ template.language }}</span>
                </div>
              </div>
              <component :is="getHeaderIcon(template.header_type)" class="h-5 w-5 text-muted-foreground flex-shrink-0" />
            </div>
          </CardHeader>
          <CardContent class="flex-1">
            <p class="text-sm text-muted-foreground line-clamp-3">
              {{ template.body_content }}
            </p>
            <div v-if="template.footer_content" class="mt-2 text-xs text-muted-foreground italic">
              {{ template.footer_content }}
            </div>
          </CardContent>
          <div class="px-6 pb-4 flex items-center gap-1 border-t pt-3">
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="sm" @click="openPreview(template)">
                  <Eye class="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Preview</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="sm"
                  @click="openEditDialog(template)"
                  :disabled="template.status === 'APPROVED' || template.status === 'PENDING'"
                >
                  <Pencil class="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Edit</TooltipContent>
            </Tooltip>
            <Tooltip v-if="template.status === 'DRAFT' || template.status === 'REJECTED'">
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="sm"
                  @click="openPublishDialog(template)"
                  :disabled="publishingTemplateId === template.id"
                  class="text-blue-600 hover:text-blue-700"
                >
                  <Loader2 v-if="publishingTemplateId === template.id" class="h-4 w-4 animate-spin" />
                  <Send v-else class="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Publish to Meta</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="sm" @click="openDeleteDialog(template)">
                  <Trash2 class="h-4 w-4 text-destructive" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Delete</TooltipContent>
            </Tooltip>
          </div>
        </Card>

        <!-- Empty State -->
        <Card v-if="filteredTemplates.length === 0" class="col-span-full">
          <CardContent class="py-12 text-center text-muted-foreground">
            <FileText class="h-12 w-12 mx-auto mb-4 opacity-50" />
            <p class="text-lg font-medium">No templates found</p>
            <p class="text-sm mb-4">Create a new template or sync from Meta.</p>
            <div class="flex items-center justify-center gap-2">
              <Button variant="outline" size="sm" @click="syncTemplates" :disabled="!selectedAccount || selectedAccount === 'all'">
                <RefreshCw class="h-4 w-4 mr-2" />
                Sync from Meta
              </Button>
              <Button variant="outline" size="sm" @click="openCreateDialog">
                <Plus class="h-4 w-4 mr-2" />
                Create Template
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </ScrollArea>

    <!-- Create/Edit Dialog -->
    <Dialog v-model:open="isDialogOpen">
      <DialogContent class="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{{ editingTemplate ? 'Edit' : 'Create' }} Template</DialogTitle>
          <DialogDescription>
            {{ editingTemplate ? 'Update your message template.' : 'Create a new WhatsApp message template.' }}
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-4">
          <!-- Account Selection -->
          <div class="space-y-2">
            <Label>WhatsApp Account <span class="text-destructive">*</span></Label>
            <select
              v-model="formData.whatsapp_account"
              class="w-full h-10 rounded-md border bg-background px-3"
              :disabled="!!editingTemplate"
            >
              <option value="">Select account...</option>
              <option v-for="account in accounts" :key="account.id" :value="account.name">
                {{ account.name }}
              </option>
            </select>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <!-- Template Name -->
            <div class="space-y-2">
              <Label>Template Name <span class="text-destructive">*</span></Label>
              <Input
                v-model="formData.name"
                placeholder="order_confirmation"
                :disabled="!!editingTemplate"
              />
              <p class="text-xs text-muted-foreground">Lowercase, underscores only</p>
            </div>

            <!-- Display Name -->
            <div class="space-y-2">
              <Label>Display Name</Label>
              <Input
                v-model="formData.display_name"
                placeholder="Order Confirmation"
              />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <!-- Language -->
            <div class="space-y-2">
              <Label>Language <span class="text-destructive">*</span></Label>
              <select v-model="formData.language" class="w-full h-10 rounded-md border bg-background px-3">
                <option v-for="lang in languages" :key="lang.code" :value="lang.code">
                  {{ lang.name }}
                </option>
              </select>
            </div>

            <!-- Category -->
            <div class="space-y-2">
              <Label>Category <span class="text-destructive">*</span></Label>
              <select v-model="formData.category" class="w-full h-10 rounded-md border bg-background px-3">
                <option v-for="cat in categories" :key="cat.value" :value="cat.value">
                  {{ cat.label }} - {{ cat.description }}
                </option>
              </select>
            </div>
          </div>

          <Separator />

          <!-- Header -->
          <div class="space-y-2">
            <Label>Header Type</Label>
            <select v-model="formData.header_type" class="w-full h-10 rounded-md border bg-background px-3">
              <option v-for="type in headerTypes" :key="type.value" :value="type.value">
                {{ type.label }}
              </option>
            </select>
          </div>

          <div v-if="formData.header_type === 'TEXT'" class="space-y-2">
            <Label>Header Text</Label>
            <Input v-model="formData.header_content" placeholder="Enter header text..." />
          </div>

          <!-- Header Media Upload for IMAGE/VIDEO/DOCUMENT -->
          <div v-else-if="['IMAGE', 'VIDEO', 'DOCUMENT'].includes(formData.header_type)" class="space-y-3">
            <Label>Header Sample {{ formData.header_type.toLowerCase() }}</Label>
            <p class="text-xs text-muted-foreground">
              Upload a sample {{ formData.header_type.toLowerCase() }} for Meta to review. This helps with template approval.
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
                Upload
              </Button>
            </div>

            <!-- Show upload status -->
            <div v-if="headerMediaFilename && !headerMediaHandle" class="text-sm text-muted-foreground">
              Selected: {{ headerMediaFilename }} (click Upload to get handle)
            </div>

            <!-- Show uploaded handle -->
            <div v-if="headerMediaHandle" class="bg-green-50 dark:bg-green-950 border border-green-200 dark:border-green-800 rounded-lg p-3">
              <div class="flex items-center gap-2">
                <Check class="h-4 w-4 text-green-600" />
                <span class="text-sm text-green-800 dark:text-green-200">Media uploaded successfully</span>
              </div>
              <p class="text-xs text-muted-foreground mt-1 font-mono truncate">
                Handle: {{ headerMediaHandle.substring(0, 40) }}...
              </p>
            </div>

            <!-- Accepted formats hint -->
            <p class="text-xs text-muted-foreground">
              <span v-if="formData.header_type === 'IMAGE'">Accepted: JPEG, PNG (max 5MB)</span>
              <span v-else-if="formData.header_type === 'VIDEO'">Accepted: MP4 (max 16MB)</span>
              <span v-else-if="formData.header_type === 'DOCUMENT'">Accepted: PDF (max 100MB)</span>
            </p>
          </div>

          <!-- Body -->
          <div class="space-y-2">
            <Label>Body Content <span class="text-destructive">*</span></Label>
            <Textarea
              v-model="formData.body_content"
              placeholder="Hi {{1}}, your order #{{2}} has been confirmed... (or use named: {{name}}, {{order_id}})"
              rows="4"
            />
            <p class="text-xs text-muted-foreground">
              Use <span v-pre>{{name}}</span>, <span v-pre>{{order_id}}</span> for named variables or <span v-pre>{{1}}</span>, <span v-pre>{{2}}</span> for positional variables
            </p>
          </div>

          <!-- Footer -->
          <div class="space-y-2">
            <Label>Footer (optional)</Label>
            <Input v-model="formData.footer_content" placeholder="Thank you for your business!" />
          </div>

          <Separator />

          <!-- Buttons -->
          <div class="space-y-3">
            <div class="flex items-center justify-between">
              <Label>Buttons (optional)</Label>
              <Button
                type="button"
                variant="outline"
                size="sm"
                @click="addButton"
                :disabled="formData.buttons.length >= 3"
              >
                <Plus class="h-4 w-4 mr-1" />
                Add Button
              </Button>
            </div>
            <p class="text-xs text-muted-foreground">Add up to 3 buttons to your template</p>

            <div v-for="(button, index) in formData.buttons" :key="index" class="border rounded-lg p-3 space-y-3">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium">Button {{ index + 1 }}</span>
                <Button type="button" variant="ghost" size="sm" @click="removeButton(index)">
                  <X class="h-4 w-4 text-destructive" />
                </Button>
              </div>

              <div class="grid grid-cols-2 gap-3">
                <div class="space-y-1">
                  <Label class="text-xs">Type</Label>
                  <select v-model="button.type" class="w-full h-9 rounded-md border bg-background px-2 text-sm">
                    <option v-for="bt in buttonTypes" :key="bt.value" :value="bt.value">
                      {{ bt.label }}
                    </option>
                  </select>
                </div>
                <div class="space-y-1">
                  <Label class="text-xs">Button Text</Label>
                  <Input v-model="button.text" placeholder="Button text" class="h-9" />
                </div>
              </div>

              <!-- URL specific fields -->
              <div v-if="button.type === 'URL'" class="space-y-1">
                <Label class="text-xs">URL</Label>
                <Input v-model="button.url" placeholder="https://example.com/{{path}}" class="h-9" />
                <p class="text-xs text-muted-foreground">Use <span v-pre>{{path}}</span> for dynamic URL suffix</p>
              </div>

              <!-- Phone number specific fields -->
              <div v-if="button.type === 'PHONE_NUMBER'" class="space-y-1">
                <Label class="text-xs">Phone Number</Label>
                <Input v-model="button.phone_number" placeholder="+1234567890" class="h-9" />
              </div>
            </div>
          </div>

          <Separator />

          <!-- Sample Values for Variables -->
          <div v-if="bodyVariables.length > 0 || headerVariables.length > 0" class="space-y-3">
            <div>
              <Label>Sample Values for Variables</Label>
              <p class="text-xs text-muted-foreground mt-1">
                Provide example values for your variables. This helps Meta review and approve your template faster.
              </p>
            </div>

            <!-- Header Variables -->
            <div v-if="headerVariables.length > 0" class="space-y-2">
              <p class="text-sm font-medium text-muted-foreground">Header Variables</p>
              <div v-for="paramName in headerVariables" :key="'header-' + paramName" class="flex items-center gap-2">
                <span class="text-sm font-mono bg-muted px-2 py-1 rounded min-w-[80px] text-center">{{ formatVariableLabel(paramName) }}</span>
                <input
                  type="text"
                  :value="getSampleValue('header', paramName)"
                  @input="setSampleValue('header', paramName, ($event.target as HTMLInputElement).value)"
                  :placeholder="'Example for ' + paramName + '...'"
                  class="flex-1 h-9 rounded-md border border-input bg-background px-3 text-sm"
                />
              </div>
            </div>

            <!-- Body Variables -->
            <div v-if="bodyVariables.length > 0" class="space-y-2">
              <p class="text-sm font-medium text-muted-foreground">Body Variables</p>
              <div v-for="paramName in bodyVariables" :key="'body-' + paramName" class="flex items-center gap-2">
                <span class="text-sm font-mono bg-muted px-2 py-1 rounded min-w-[80px] text-center">{{ formatVariableLabel(paramName) }}</span>
                <input
                  type="text"
                  :value="getSampleValue('body', paramName)"
                  @input="setSampleValue('body', paramName, ($event.target as HTMLInputElement).value)"
                  :placeholder="'Example for ' + paramName + '...'"
                  class="flex-1 h-9 rounded-md border border-input bg-background px-3 text-sm"
                />
              </div>
            </div>
          </div>

          <!-- Info Box -->
          <div class="bg-blue-50 dark:bg-blue-950 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
            <div class="flex gap-3">
              <AlertCircle class="h-5 w-5 text-blue-600 dark:text-blue-400 flex-shrink-0" />
              <div class="text-sm text-blue-800 dark:text-blue-200">
                <p class="font-medium">Template Submission</p>
                <p class="mt-1">
                  This creates a local draft. After saving, click the <Send class="h-3 w-3 inline" /> publish button
                  on the template card to submit it to Meta for approval.
                </p>
              </div>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" @click="isDialogOpen = false">Cancel</Button>
          <Button size="sm" @click="saveTemplate" :disabled="isSubmitting">
            <Loader2 v-if="isSubmitting" class="h-4 w-4 mr-2 animate-spin" />
            {{ editingTemplate ? 'Update' : 'Create' }} Template
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Preview Dialog -->
    <Dialog v-model:open="isPreviewOpen">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>Template Preview</DialogTitle>
          <DialogDescription>
            {{ previewTemplate?.display_name || previewTemplate?.name }}
          </DialogDescription>
        </DialogHeader>

        <div v-if="previewTemplate" class="py-4">
          <!-- WhatsApp-style preview -->
          <div class="bg-[#e5ddd5] dark:bg-gray-800 rounded-lg p-4">
            <div class="bg-white dark:bg-gray-700 rounded-lg shadow max-w-[280px] overflow-hidden">
              <!-- Header -->
              <div v-if="previewTemplate.header_type && previewTemplate.header_type !== 'NONE'" class="p-3 border-b">
                <div v-if="previewTemplate.header_type === 'TEXT'" class="font-semibold">
                  {{ previewTemplate.header_content }}
                </div>
                <div v-else class="h-32 bg-gray-200 dark:bg-gray-600 rounded flex items-center justify-center">
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
                  <button class="w-full py-2 text-sm text-blue-500 hover:bg-gray-50 dark:hover:bg-gray-600">
                    {{ btn.text || btn.title || 'Button' }}
                  </button>
                </div>
              </div>
            </div>
          </div>

          <!-- Template Info -->
          <div class="mt-4 space-y-2 text-sm">
            <div class="flex justify-between">
              <span class="text-muted-foreground">Status:</span>
              <span :class="['px-2 py-0.5 rounded text-xs font-medium', getStatusBadgeClass(previewTemplate.status)]">
                {{ previewTemplate.status }}
              </span>
            </div>
            <div class="flex justify-between">
              <span class="text-muted-foreground">Category:</span>
              <span>{{ previewTemplate.category }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-muted-foreground">Language:</span>
              <span>{{ previewTemplate.language }}</span>
            </div>
            <div v-if="previewTemplate.meta_template_id" class="flex justify-between">
              <span class="text-muted-foreground">Meta ID:</span>
              <span class="font-mono text-xs">{{ previewTemplate.meta_template_id }}</span>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" @click="isPreviewOpen = false">Close</Button>
          <Button
            v-if="previewTemplate?.status === 'DRAFT' || previewTemplate?.status === 'REJECTED'"
            size="sm"
            @click="openPublishDialog(previewTemplate!); isPreviewOpen = false"
            :disabled="publishingTemplateId === previewTemplate?.id"
          >
            <Loader2 v-if="publishingTemplateId === previewTemplate?.id" class="h-4 w-4 mr-2 animate-spin" />
            <Send v-else class="h-4 w-4 mr-2" />
            {{ previewTemplate?.status === 'REJECTED' ? 'Resubmit to Meta' : 'Publish to Meta' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Delete Confirmation Dialog -->
    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete Template</AlertDialogTitle>
          <AlertDialogDescription>
            Are you sure you want to delete "{{ templateToDelete?.display_name || templateToDelete?.name }}"? This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction @click="confirmDeleteTemplate">Delete</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <!-- Publish Confirmation Dialog -->
    <AlertDialog v-model:open="publishDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Publish Template</AlertDialogTitle>
          <AlertDialogDescription>
            Publish "{{ templateToPublish?.display_name || templateToPublish?.name }}" to Meta for approval? Once submitted, you won't be able to edit it until it's approved or rejected.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction @click="confirmPublishTemplate">Publish</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
