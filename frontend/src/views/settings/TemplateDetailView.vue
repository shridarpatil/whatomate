<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { api, templatesService, flowsService } from '@/services/api'
import { toast } from 'vue-sonner'
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
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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
  FileText,
  Trash2,
  Save,
  Upload,
  Loader2,
  Check,
  Eye,
  Send,
  Plus,
  X,
  ChevronDown,
  Info,
} from 'lucide-vue-next'
import { getErrorMessage } from '@/lib/api-utils'

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
  add_security_recommendation: boolean
  code_expiration_minutes: number
  created_by_name: string
  updated_by_name: string
  created_at: string
  updated_at: string
}

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const authStore = useAuthStore()

const bodyHint = 'Use {{1}}, {{2}} for positional or {{name}}, {{email}} for named parameters.'
const mixedVariablesHint = 'Cannot mix positional ({{1}}, {{2}}) and named ({{name}}) variables. Use one type only.'
const duplicateVariablesHint = 'Duplicate variables found. Each variable should appear only once in the template.'
const variablePositionHint = 'Variables cannot be at the very start or end of the template body.'

const templateId = computed(() => route.params.id as string)
const isNew = computed(() => templateId.value === 'new')
const isAuthentication = computed(() => form.value.category === 'AUTHENTICATION')

// OTP type for auth templates — derived from the buttons array
const authOtpType = computed(() => {
  if (!isAuthentication.value) return 'COPY_CODE'
  const otpBtn = form.value.buttons.find((b: any) => b.type === 'OTP')
  return otpBtn?.otp_type || 'COPY_CODE'
})

const zeroTapAccepted = ref(false)

function setAuthOtpType(type: string | number | bigint | Record<string, any> | null) {
  if (!type || typeof type !== 'string') return
  zeroTapAccepted.value = false
  const existing = form.value.buttons.find((b: any) => b.type === 'OTP')
  if (existing) {
    existing.otp_type = type
    if (type === 'ONE_TAP' || type === 'ZERO_TAP') {
      if (!existing.supported_apps?.length) {
        existing.supported_apps = [{ package_name: '', signature_hash: '' }]
      }
    }
  } else {
    const btn: any = { type: 'OTP', text: 'Copy code', otp_type: type }
    if (type === 'ONE_TAP' || type === 'ZERO_TAP') {
      btn.supported_apps = [{ package_name: '', signature_hash: '' }]
    }
    form.value.buttons = [btn]
  }
}

function addSupportedApp() {
  const otpBtn = form.value.buttons.find((b: any) => b.type === 'OTP')
  if (otpBtn && (!otpBtn.supported_apps || otpBtn.supported_apps.length < 5)) {
    if (!otpBtn.supported_apps) otpBtn.supported_apps = []
    otpBtn.supported_apps.push({ package_name: '', signature_hash: '' })
  }
}

function removeSupportedApp(index: number) {
  const otpBtn = form.value.buttons.find((b: any) => b.type === 'OTP')
  if (otpBtn?.supported_apps?.length > 1) {
    otpBtn.supported_apps.splice(index, 1)
  }
}

const template = ref<Template | null>(null)
const accounts = ref<WhatsAppAccount[]>([])
const isLoading = ref(true)
const isNotFound = ref(false)
const isSaving = ref(false)
const hasChanges = ref(false)
const auditRefreshKey = ref(0)
const deleteDialogOpen = ref(false)
const publishDialogOpen = ref(false)
const isPublishing = ref(false)
const isPreviewOpen = ref(false)
const isDetailsOpen = ref(true)

// Header media upload state
const headerMediaFile = ref<File | null>(null)
const headerMediaUploading = ref(false)
const headerMediaHandle = ref('')
const headerMediaFilename = ref('')

// WhatsApp Flows for FLOW button type
const whatsappFlows = ref<any[]>([])

const { showLeaveDialog, confirmLeave, cancelLeave } = useUnsavedChangesGuard(hasChanges)

const canWrite = computed(() => authStore.hasPermission('templates', 'write'))
const canDelete = computed(() => authStore.hasPermission('templates', 'delete'))

const isEditable = computed(() => {
  if (isNew.value) return true
  if (!template.value) return false
  const status = template.value.status?.toUpperCase()
  // Meta allows editing: APPROVED, REJECTED, PAUSED, DRAFT. Not PENDING (under review).
  return status === 'APPROVED' || status === 'REJECTED' || status === 'PAUSED' || status === 'DRAFT' || !status
})

const form = ref({
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
  sample_values: [] as any[],
  add_security_recommendation: false,
  code_expiration_minutes: 0,
})

// Detect variables in body and header content
const bodyVariables = computed(() => {
  const matches = form.value.body_content.match(/\{\{([^}]+)\}\}/g) || []
  return matches.map(m => m.replace(/\{\{|\}\}/g, '').trim())
})

const headerVariables = computed(() => {
  if (form.value.header_type !== 'TEXT') return []
  const matches = form.value.header_content.match(/\{\{([^}]+)\}\}/g) || []
  return matches.map(m => m.replace(/\{\{|\}\}/g, '').trim())
})

const allVariables = computed(() => {
  const vars: { component: string; name: string; label: string; index: number }[] = []
  const wrap = (v: string) => '\u007B\u007B' + v + '\u007D\u007D'
  headerVariables.value.forEach((v, i) => vars.push({ component: 'header', name: v, label: wrap(v), index: i + 1 }))
  bodyVariables.value.forEach((v, i) => vars.push({ component: 'body', name: v, label: wrap(v), index: i + 1 }))
  return vars
})

// Detect mixed variable types (positional + named) which are not allowed
const hasMixedVariables = computed(() => {
  const vars = allVariables.value
  if (vars.length === 0) return false
  const hasPositional = vars.some(v => /^\d+$/.test(v.name))
  const hasNamed = vars.some(v => !/^\d+$/.test(v.name))
  return hasPositional && hasNamed
})

// Detect duplicate positional variables (e.g. {{1}} used twice) — named params can repeat
const hasDuplicateVariables = computed(() => {
  const isPositional = (v: string) => /^\d+$/.test(v)
  const bodyNums = bodyVariables.value.filter(isPositional)
  const headerNums = headerVariables.value.filter(isPositional)
  return bodyNums.length !== new Set(bodyNums).size
    || headerNums.length !== new Set(headerNums).size
})

// Detect variables at the start or end of body content (Meta restriction)
const hasVariableAtEdge = computed(() => {
  const body = form.value.body_content.trim()
  if (!body) return false
  return /^\{\{[^}]+\}\}/.test(body) || /\{\{[^}]+\}\}$/.test(body)
})

// Build sample_values array from form inputs
function getSampleValueForVar(component: string, index: number): string {
  const sv = form.value.sample_values.find(
    (s: any) => s.component === component && s.index === index
  )
  return sv?.value || ''
}

function setSampleValueForVar(component: string, index: number, value: string) {
  const existing = form.value.sample_values.findIndex(
    (s: any) => s.component === component && s.index === index
  )
  if (existing >= 0) {
    form.value.sample_values[existing].value = value
  } else {
    form.value.sample_values.push({ component, index, value })
  }
}

// Sync sample_values when variables change — remove stale entries
watch(allVariables, (vars) => {
  form.value.sample_values = form.value.sample_values.filter((sv: any) =>
    vars.some(v => v.component === sv.component && v.index === sv.index)
  )
})

const buttonTypes = [
  { value: 'QUICK_REPLY', label: 'Quick Reply' },
  { value: 'URL', label: 'URL' },
  { value: 'PHONE_NUMBER', label: 'Phone Number' },
  { value: 'COPY_CODE', label: 'Copy Code' },
  { value: 'FLOW', label: 'Flow' },
  { value: 'VOICE_CALL', label: 'Call on WhatsApp' },
  { value: 'OTP', label: 'OTP' },
]

function addButton() {
  if (form.value.buttons.length >= 3) {
    toast.error(t('templates.maxButtons', 'Maximum 3 buttons allowed'))
    return
  }
  form.value.buttons.push({ type: 'QUICK_REPLY', text: '' })
}

function removeButton(index: number) {
  form.value.buttons.splice(index, 1)
}

const breadcrumbs = computed(() => [
  { label: t('nav.templates', 'Templates'), href: '/templates' },
  { label: isNew.value ? t('templates.newTemplate', 'New Template') : (template.value?.display_name || template.value?.name || '') },
])

const languages = [
  { code: 'en', name: 'English' },
  { code: 'en_GB', name: 'English (UK)' },
  { code: 'en_US', name: 'English (US)' },
  { code: 'es', name: 'Spanish' },
  { code: 'es_AR', name: 'Spanish (ARG)' },
  { code: 'es_MX', name: 'Spanish (MEX)' },
  { code: 'pt_BR', name: 'Portuguese (BR)' },
  { code: 'pt_PT', name: 'Portuguese (POR)' },
  { code: 'hi', name: 'Hindi' },
  { code: 'ta', name: 'Tamil' },
  { code: 'te', name: 'Telugu' },
  { code: 'kn', name: 'Kannada' },
  { code: 'ml', name: 'Malayalam' },
  { code: 'mr', name: 'Marathi' },
  { code: 'gu', name: 'Gujarati' },
  { code: 'bn', name: 'Bengali' },
  { code: 'pa', name: 'Punjabi' },
  { code: 'ur', name: 'Urdu' },
  { code: 'ar', name: 'Arabic' },
  { code: 'fr', name: 'French' },
  { code: 'de', name: 'German' },
  { code: 'it', name: 'Italian' },
  { code: 'nl', name: 'Dutch' },
  { code: 'ja', name: 'Japanese' },
  { code: 'ko', name: 'Korean' },
  { code: 'zh_CN', name: 'Chinese (CHN)' },
  { code: 'zh_HK', name: 'Chinese (HKG)' },
  { code: 'zh_TW', name: 'Chinese (TAI)' },
  { code: 'ru', name: 'Russian' },
  { code: 'tr', name: 'Turkish' },
  { code: 'id', name: 'Indonesian' },
  { code: 'ms', name: 'Malay' },
  { code: 'th', name: 'Thai' },
  { code: 'vi', name: 'Vietnamese' },
  { code: 'sw', name: 'Swahili' },
  { code: 'fil', name: 'Filipino' },
  { code: 'pl', name: 'Polish' },
  { code: 'uk', name: 'Ukrainian' },
  { code: 'ro', name: 'Romanian' },
  { code: 'sv', name: 'Swedish' },
  { code: 'da', name: 'Danish' },
  { code: 'fi', name: 'Finnish' },
  { code: 'he', name: 'Hebrew' },
  { code: 'fa', name: 'Persian' },
  { code: 'af', name: 'Afrikaans' },
  { code: 'zu', name: 'Zulu' },
]

const categories = [
  { value: 'MARKETING', label: 'Marketing' },
  { value: 'UTILITY', label: 'Utility' },
  { value: 'AUTHENTICATION', label: 'Authentication' },
]

const headerTypes = [
  { value: 'NONE', label: 'None' },
  { value: 'TEXT', label: 'Text' },
  { value: 'IMAGE', label: 'Image' },
  { value: 'VIDEO', label: 'Video' },
  { value: 'DOCUMENT', label: 'Document' },
]

const statusVariant = computed(() => {
  if (!template.value) return 'secondary' as const
  switch (template.value.status?.toUpperCase()) {
    case 'APPROVED': return 'default' as const
    case 'REJECTED': return 'destructive' as const
    case 'PENDING': return 'outline' as const
    default: return 'secondary' as const
  }
})

async function loadTemplate() {
  isLoading.value = true
  isNotFound.value = false
  try {
    const response = await templatesService.get(templateId.value)
    const data = (response.data as any).data
    template.value = data
    syncForm()
    isDetailsOpen.value = false
    nextTick(() => { hasChanges.value = false })
  } catch {
    isNotFound.value = true
  } finally {
    isLoading.value = false
  }
}

async function loadAccounts() {
  try {
    const response = await api.get('/accounts')
    accounts.value = (response.data as any).data?.accounts || []
  } catch (err) {
    console.error('Failed to load accounts:', err)
  }
}

function syncForm() {
  if (!template.value) return
  form.value = {
    whatsapp_account: template.value.whatsapp_account || '',
    name: template.value.name || '',
    display_name: template.value.display_name || '',
    language: template.value.language || 'en',
    category: template.value.category || 'UTILITY',
    header_type: template.value.header_type || 'NONE',
    header_content: template.value.header_content || '',
    body_content: template.value.body_content || '',
    footer_content: template.value.footer_content || '',
    buttons: (template.value.buttons || []).map((b: any) => ({
      ...b,
      example: Array.isArray(b.example) ? b.example[0] ?? '' : b.example,
    })),
    sample_values: template.value.sample_values || [],
    add_security_recommendation: template.value.add_security_recommendation || false,
    code_expiration_minutes: template.value.code_expiration_minutes || 0,
  }
  // Restore media handle for existing media headers
  headerMediaFile.value = null
  headerMediaFilename.value = ''
  if (['IMAGE', 'VIDEO', 'DOCUMENT'].includes(template.value.header_type || '')) {
    headerMediaHandle.value = template.value.header_content || ''
  } else {
    headerMediaHandle.value = ''
  }
}

// Track form changes
watch(form, () => {
  if (isNew.value) {
    hasChanges.value = true
    return
  }
  if (!template.value) return
  hasChanges.value = true
}, { deep: true })

// Auto-configure form when switching to/from AUTHENTICATION category
watch(() => form.value.category, (newCat, oldCat) => {
  if (newCat === 'AUTHENTICATION' && oldCat !== 'AUTHENTICATION') {
    form.value.header_type = 'NONE'
    form.value.header_content = ''
    form.value.body_content = ''
    form.value.footer_content = ''
    form.value.buttons = [{ type: 'OTP', text: 'Copy code', otp_type: 'COPY_CODE' }]
  } else if (newCat !== 'AUTHENTICATION' && oldCat === 'AUTHENTICATION') {
    form.value.add_security_recommendation = false
    form.value.code_expiration_minutes = 0
  }
})

async function save() {
  if (!form.value.name.trim()) {
    toast.error(t('templates.nameRequired', 'Template name is required'))
    return
  }
  if (!isAuthentication.value && !form.value.body_content.trim()) {
    toast.error(t('templates.bodyRequired', 'Body content is required'))
    return
  }
  if (!isAuthentication.value) {
    if (hasMixedVariables.value) {
      toast.error(t('templates.mixedVariables', 'Cannot mix positional ({{1}}, {{2}}) and named ({{name}}) variables. Use one type only.'))
      return
    }
    if (hasDuplicateVariables.value) {
      toast.error(t('templates.duplicateVariables', 'Duplicate variables found. Each variable should appear only once.'))
      return
    }
    if (hasVariableAtEdge.value) {
      toast.error(t('templates.variableAtEdge', 'Variables cannot be at the very start or end of the template body.'))
      return
    }
  }
  if (isAuthentication.value && form.value.code_expiration_minutes && (form.value.code_expiration_minutes < 1 || form.value.code_expiration_minutes > 90)) {
    toast.error(t('templates.invalidExpiration', 'Code expiration must be between 1 and 90 minutes'))
    return
  }
  if (isAuthentication.value && authOtpType.value === 'ZERO_TAP' && !zeroTapAccepted.value) {
    toast.error(t('templates.zeroTapTosRequired', 'You must accept the Terms of Service to use zero-tap authentication'))
    return
  }
  if (isAuthentication.value && (authOtpType.value === 'ONE_TAP' || authOtpType.value === 'ZERO_TAP')) {
    const apps = form.value.buttons[0]?.supported_apps || []
    if (apps.length === 0 || apps.some((a: any) => !a.package_name?.trim() || !a.signature_hash?.trim())) {
      toast.error(t('templates.supportedAppsRequired', 'Package name and signature hash are required for all supported apps'))
      return
    }
  }
  isSaving.value = true
  try {
    const payload: Record<string, any> = {
      whatsapp_account: form.value.whatsapp_account,
      name: form.value.name,
      display_name: form.value.display_name,
      language: form.value.language,
      category: form.value.category,
      header_type: isAuthentication.value ? 'NONE' : form.value.header_type,
      header_content: form.value.header_type === 'TEXT' && !isAuthentication.value ? form.value.header_content : '',
      body_content: isAuthentication.value ? '{{1}} is your verification code.' : form.value.body_content,
      footer_content: isAuthentication.value ? '' : form.value.footer_content,
      buttons: form.value.buttons,
      sample_values: form.value.sample_values,
      add_security_recommendation: form.value.add_security_recommendation,
      code_expiration_minutes: form.value.code_expiration_minutes || 0,
    }

    if (isNew.value) {
      const response = await api.post('/templates', payload)
      const created = (response.data as any).data
      hasChanges.value = false
      toast.success(t('templates.created', 'Template created'))
      router.replace(`/templates/${created.id}`)
    } else {
      await api.put(`/templates/${templateId.value}`, payload)
      hasChanges.value = false
      toast.success(t('templates.updated', 'Template updated'))
      await loadTemplate()
      auditRefreshKey.value++
    }
  } catch {
    toast.error(
      isNew.value
        ? t('templates.createFailed', 'Failed to create template')
        : t('templates.updateFailed', 'Failed to update template')
    )
  } finally {
    isSaving.value = false
  }
}

function getAcceptedFileTypes(): string {
  switch (form.value.header_type) {
    case 'IMAGE': return 'image/jpeg,image/png'
    case 'VIDEO': return 'video/mp4'
    case 'DOCUMENT': return 'application/pdf'
    default: return '*/*'
  }
}

function onHeaderMediaFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  if (input.files && input.files.length > 0) {
    headerMediaFile.value = input.files[0]
    headerMediaFilename.value = input.files[0].name
    headerMediaHandle.value = ''
    form.value.header_content = ''
  }
}

async function uploadHeaderMedia() {
  if (!headerMediaFile.value) return
  if (!form.value.whatsapp_account) {
    toast.error(t('templates.selectAccountFirst', 'Select an account first'))
    return
  }
  headerMediaUploading.value = true
  try {
    const response = await templatesService.uploadMedia(form.value.whatsapp_account, headerMediaFile.value)
    const data = (response.data as any).data
    headerMediaHandle.value = data.handle
    form.value.header_content = data.handle
    toast.success(t('templates.mediaUploadedSuccess', 'Media uploaded successfully'))
  } catch (err) {
    toast.error(getErrorMessage(err, t('templates.uploadFailed', 'Upload failed')))
  } finally {
    headerMediaUploading.value = false
  }
}

async function deleteTemplate() {
  if (!template.value) return
  try {
    await api.delete(`/templates/${template.value.id}`)
    toast.success(t('templates.deleted', 'Template deleted'))
    router.push('/templates')
  } catch {
    toast.error(t('templates.deleteFailed', 'Failed to delete template'))
  }
  deleteDialogOpen.value = false
}

const canPublish = computed(() => {
  if (!template.value || isNew.value) return false
  const status = template.value.status?.toUpperCase()
  return status === 'DRAFT' || status === 'REJECTED'
})

async function confirmPublish() {
  if (!template.value) return
  isPublishing.value = true
  try {
    const response = await api.post(`/templates/${template.value.id}/publish`)
    toast.success((response.data as any).data?.message || t('templates.publishSuccess', 'Template published'))
    publishDialogOpen.value = false
    await loadTemplate()
  } catch (err) {
    toast.error(getErrorMessage(err, t('templates.publishFailed', 'Failed to publish template')))
  } finally {
    isPublishing.value = false
  }
}

// Replace template variables with sample values for preview
function replaceVariablesWithSamples(text: string, component: string): string {
  if (!text) return text
  const samples = form.value.sample_values || []
  return text.replace(/\{\{([^}]+)\}\}/g, (_match, varName: string) => {
    const trimmed = varName.trim()
    const isPositional = /^\d+$/.test(trimmed)
    const index = isPositional ? parseInt(trimmed) : 0
    const sv = samples.find((s: any) => {
      if (s.component !== component) return false
      if (isPositional) return s.index === index
      return s.param_name === trimmed || s.index === index
    })
    return sv?.value || `[${trimmed}]`
  })
}

const previewBody = computed(() => replaceVariablesWithSamples(form.value.body_content, 'body'))
const previewHeader = computed(() => replaceVariablesWithSamples(form.value.header_content, 'header'))

async function loadFlows() {
  try {
    const response = await flowsService.list({ limit: 100 })
    const data = (response.data as any).data || response.data
    whatsappFlows.value = (data.flows || []).filter((f: any) => f.status === 'PUBLISHED')
  } catch {
    // non-critical
  }
}

function getFlowScreens(flowId: string): string[] {
  const flow = whatsappFlows.value.find((f: any) => f.meta_flow_id === flowId || f.id === flowId)
  if (!flow?.screens) return []
  return flow.screens
    .map((s: any) => (typeof s === 'string' ? s : s?.id || s?.name))
    .filter(Boolean)
}

onMounted(async () => {
  await Promise.all([loadAccounts(), loadFlows()])
  if (isNew.value) {
    isLoading.value = false
    hasChanges.value = false
  } else {
    await loadTemplate()
  }
})
</script>

<template>
  <div class="h-full">
  <DetailPageLayout
    :title="isNew ? $t('templates.newTemplate', 'New Template') : (template?.display_name || template?.name || '')"
    :icon="FileText"
    icon-gradient="bg-gradient-to-br from-blue-500 to-indigo-600 shadow-blue-500/20"
    back-link="/templates"
    :breadcrumbs="breadcrumbs"
    :is-loading="isLoading"
    :is-not-found="isNotFound"
    :not-found-title="$t('templates.notFound', 'Template not found')"
  >
    <template #actions>
      <div class="flex items-center gap-2">
        <Button v-if="!isNew" variant="outline" size="sm" @click="isPreviewOpen = true">
          <Eye class="h-4 w-4 mr-1" /> {{ $t('templates.preview', 'Preview') }}
        </Button>
        <Button v-if="canPublish" variant="outline" size="sm" @click="publishDialogOpen = true" :disabled="isPublishing">
          <Loader2 v-if="isPublishing" class="h-4 w-4 mr-1 animate-spin" />
          <Send v-else class="h-4 w-4 mr-1" />
          {{ template?.meta_template_id ? $t('templates.republish', 'Republish') : $t('templates.publish', 'Publish') }}
        </Button>
        <Button v-if="canWrite && (hasChanges || isNew)" size="sm" @click="save" :disabled="isSaving">
          <Save class="h-4 w-4 mr-1" /> {{ isSaving ? $t('common.saving', 'Saving...') : isNew ? $t('common.create') : $t('common.save') }}
        </Button>
        <Button v-if="canDelete && !isNew" variant="destructive" size="sm" @click="deleteDialogOpen = true">
          <Trash2 class="h-4 w-4 mr-1" /> {{ $t('common.delete') }}
        </Button>
      </div>
    </template>

    <!-- Details Card -->
    <Card>
      <CardHeader class="pb-3 cursor-pointer" @click="isDetailsOpen = !isDetailsOpen">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <CardTitle class="text-sm font-medium">{{ $t('templates.details', 'Details') }}</CardTitle>
            <Badge v-if="!isNew && template?.status" :variant="statusVariant">
              {{ template.status }}
            </Badge>
          </div>
          <ChevronDown class="h-4 w-4 text-muted-foreground transition-transform" :class="isDetailsOpen && 'rotate-180'" />
        </div>
      </CardHeader>
      <CardContent v-show="isDetailsOpen" class="space-y-4">
        <!-- Edit limits info for approved templates -->
        <div v-if="template?.status?.toUpperCase() === 'APPROVED'" class="flex items-start gap-2 rounded-md bg-blue-500/10 border border-blue-500/20 px-3 py-2 text-xs text-blue-400 light:text-blue-600">
          <Info class="h-3.5 w-3.5 shrink-0 mt-0.5" />
          <span>{{ $t('templates.editLimitsInfo', 'Approved templates can be edited up to 10 times in 30 days (1 edit per 24 hours). Editing triggers a new review which may take up to 24 hours. Name, language, and category cannot be changed.') }}</span>
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('templates.whatsappAccount', 'WhatsApp Account') }}</Label>
          <Select v-model="form.whatsapp_account" :disabled="!canWrite || !!template?.meta_template_id">
            <SelectTrigger><SelectValue :placeholder="$t('templates.selectAccount', 'Select account')" /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="account in accounts" :key="account.id" :value="account.name">
                {{ account.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('templates.name', 'Name') }} *</Label>
          <Input v-model="form.name" :disabled="!canWrite || !!template?.meta_template_id" />
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('templates.displayName', 'Display Name') }}</Label>
          <Input v-model="form.display_name" :disabled="!canWrite || !isEditable" />
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('templates.language', 'Language') }}</Label>
          <Select v-model="form.language" :disabled="!canWrite || !!template?.meta_template_id">
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="lang in languages" :key="lang.code" :value="lang.code">
                {{ lang.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('templates.category', 'Category') }}</Label>
          <Select v-model="form.category" :disabled="!canWrite || !isEditable || (!!template?.meta_template_id && template?.status?.toUpperCase() === 'APPROVED')">
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="cat in categories" :key="cat.value" :value="cat.value">
                {{ cat.label }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
      </CardContent>
    </Card>

    <!-- Content Card -->
    <Card>
      <CardHeader class="pb-3">
        <CardTitle class="text-sm font-medium">{{ $t('templates.content', 'Content') }}</CardTitle>
      </CardHeader>
      <CardContent class="space-y-4">
        <div v-if="!isAuthentication" class="space-y-1.5">
          <Label class="text-xs">{{ $t('templates.headerType', 'Header Type') }}</Label>
          <Select v-model="form.header_type" :disabled="!canWrite || !isEditable">
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="ht in headerTypes" :key="ht.value" :value="ht.value">
                {{ ht.label }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div v-if="form.header_type === 'TEXT'" class="space-y-1.5">
          <Label class="text-xs" for="header-content">{{ $t('templates.headerContent', 'Header Content') }}</Label>
          <Input id="header-content" v-model="form.header_content" :disabled="!canWrite || !isEditable" />
        </div>

        <!-- Header Media Upload for IMAGE/VIDEO/DOCUMENT -->
        <div v-else-if="['IMAGE', 'VIDEO', 'DOCUMENT'].includes(form.header_type)" class="space-y-3">
          <Label class="text-xs">{{ $t('templates.headerSample', 'Header') }} {{ form.header_type.toLowerCase() }}</Label>
          <div class="flex items-center gap-2">
            <div class="flex-1">
              <input
                type="file"
                :accept="getAcceptedFileTypes()"
                :disabled="!canWrite || !isEditable"
                @change="onHeaderMediaFileChange"
                class="w-full text-sm file:mr-4 file:py-1.5 file:px-3 file:rounded-md file:border-0 file:text-xs file:font-medium file:bg-primary file:text-primary-foreground hover:file:bg-primary/90 cursor-pointer"
              />
            </div>
            <Button
              type="button"
              size="sm"
              @click="uploadHeaderMedia"
              :disabled="!headerMediaFile || headerMediaUploading || !form.whatsapp_account"
            >
              <Loader2 v-if="headerMediaUploading" class="h-3.5 w-3.5 mr-1 animate-spin" />
              <Upload v-else class="h-3.5 w-3.5 mr-1" />
              {{ $t('templates.uploadMedia', 'Upload') }}
            </Button>
          </div>
          <div v-if="headerMediaFilename && !headerMediaHandle" class="text-xs text-muted-foreground">
            {{ headerMediaFilename }}
          </div>
          <div v-if="headerMediaHandle" class="bg-green-950 light:bg-green-50 border border-green-800 light:border-green-200 rounded-lg p-2.5">
            <div class="flex items-center gap-2">
              <Check class="h-3.5 w-3.5 text-green-600" />
              <span class="text-xs text-green-200 light:text-green-800">{{ $t('templates.mediaUploadedSuccess', 'Media uploaded') }}</span>
            </div>
            <p class="text-xs text-muted-foreground mt-1 font-mono truncate">
              Handle: {{ headerMediaHandle.substring(0, 40) }}...
            </p>
          </div>
          <p class="text-xs text-muted-foreground">
            <span v-if="form.header_type === 'IMAGE'">JPEG or PNG, max 5MB</span>
            <span v-else-if="form.header_type === 'VIDEO'">MP4, max 16MB</span>
            <span v-else-if="form.header_type === 'DOCUMENT'">PDF, max 100MB</span>
          </p>
        </div>

        <!-- Authentication template: fixed body & options -->
        <div v-if="isAuthentication" class="space-y-4">
          <!-- OTP Code Delivery Method -->
          <div class="space-y-2">
            <Label class="text-xs">{{ $t('templates.codeDelivery', 'Code Delivery Method') }}</Label>
            <Select :model-value="authOtpType" @update:model-value="setAuthOtpType" :disabled="!canWrite || !isEditable">
              <SelectTrigger class="h-8 text-xs">
                <SelectValue placeholder="Select delivery method" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="COPY_CODE">Copy Code</SelectItem>
                <SelectItem value="ONE_TAP">One-Tap Autofill (Android only)</SelectItem>
                <SelectItem value="ZERO_TAP">Zero-Tap (Android only)</SelectItem>
              </SelectContent>
            </Select>
            <p class="text-xs text-muted-foreground">
              <span v-if="authOtpType === 'COPY_CODE'">User taps a button to copy the code to clipboard.</span>
              <span v-else-if="authOtpType === 'ONE_TAP'">User taps a button to autofill the code in your app. Requires app configuration.</span>
              <span v-else-if="authOtpType === 'ZERO_TAP'">Code is delivered to your app automatically. Requires app configuration.</span>
            </p>
          </div>

          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('templates.bodyContent', 'Body Content') }}</Label>
            <div class="rounded-md border bg-muted/50 p-3 text-sm text-muted-foreground">
              <span class="font-mono">{'{{1}}'}</span> is your verification code.
              <span v-if="form.add_security_recommendation" class="block mt-1">For your security, do not share this code.</span>
            </div>
            <p class="text-xs text-muted-foreground">Authentication templates use fixed preset text defined by Meta.</p>
          </div>
          <div class="flex items-center gap-2">
            <input
              id="security-rec"
              type="checkbox"
              v-model="form.add_security_recommendation"
              :disabled="!canWrite || !isEditable"
              class="h-4 w-4 rounded border-gray-300"
            />
            <Label for="security-rec" class="text-xs cursor-pointer">{{ $t('templates.addSecurityRecommendation', 'Add security recommendation') }}</Label>
          </div>
          <div class="space-y-2">
            <div class="flex items-center gap-2">
              <input
                id="code-expiration"
                type="checkbox"
                :checked="form.code_expiration_minutes > 0"
                @change="form.code_expiration_minutes = ($event.target as HTMLInputElement).checked ? 10 : 0"
                :disabled="!canWrite || !isEditable"
                class="h-4 w-4 rounded border-gray-300"
              />
              <Label for="code-expiration" class="text-xs cursor-pointer">{{ $t('templates.addCodeExpiration', 'Add expiration time for the code') }}</Label>
            </div>
            <div v-if="form.code_expiration_minutes > 0" class="flex items-center gap-2 ml-6">
              <Input
                type="number"
                :model-value="form.code_expiration_minutes"
                @update:model-value="(val: string) => form.code_expiration_minutes = val ? parseInt(val) : 0"
                min="1"
                max="90"
                class="h-8 text-xs w-24"
                :disabled="!canWrite || !isEditable"
              />
              <span class="text-xs text-muted-foreground">minutes (1-90)</span>
            </div>
            <p v-if="form.code_expiration_minutes > 0" class="text-xs text-muted-foreground ml-6">
              Footer: "This code expires in {{ form.code_expiration_minutes }} minutes."
            </p>
          </div>
          <!-- Zero-Tap Terms of Service -->
          <div v-if="authOtpType === 'ZERO_TAP'" class="border border-amber-500/30 bg-amber-500/5 rounded-lg p-3">
            <div class="flex items-start gap-2">
              <input
                id="zero-tap-tos"
                type="checkbox"
                v-model="zeroTapAccepted"
                :disabled="!canWrite || !isEditable"
                class="h-4 w-4 mt-0.5 rounded border-gray-300"
              />
              <Label for="zero-tap-tos" class="text-xs cursor-pointer leading-relaxed">
                By selecting zero-tap, I understand that my business's use of zero-tap authentication is subject to the
                <a href="https://www.whatsapp.com/legal/business-terms/" target="_blank" class="underline text-primary">WhatsApp Business Terms of Service</a>.
                It is my business's responsibility to ensure its customers expect that the code will be automatically filled in on their behalf when they choose to receive the zero-tap code through WhatsApp.
              </Label>
            </div>
          </div>

          <!-- ONE_TAP: autofill text + supported apps -->
          <div v-if="authOtpType === 'ONE_TAP'" class="space-y-3 border rounded-lg p-3">
            <div class="space-y-1">
              <Label class="text-xs">{{ $t('templates.autofillText', 'Autofill Text') }}</Label>
              <Input v-model="form.buttons[0].autofill_text" placeholder="Autofill" class="h-8 text-xs" :disabled="!canWrite || !isEditable" />
            </div>
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <Label class="text-xs">{{ $t('templates.supportedApps', 'Supported Apps') }} *</Label>
                <Button
                  v-if="canWrite && isEditable && (form.buttons[0]?.supported_apps?.length || 0) < 5"
                  type="button" variant="outline" size="xs" class="h-6 text-xs"
                  @click="addSupportedApp"
                >
                  <Plus class="h-3 w-3 mr-1" /> Add App
                </Button>
              </div>
              <div v-for="(app, i) in form.buttons[0]?.supported_apps || []" :key="i" class="flex items-end gap-2">
                <div class="flex-1 space-y-1">
                  <Label class="text-xs">Package Name *</Label>
                  <Input v-model="app.package_name" placeholder="com.example.app" class="h-8 text-xs" :disabled="!canWrite || !isEditable" />
                </div>
                <div class="flex-1 space-y-1">
                  <Label class="text-xs">Signature Hash *</Label>
                  <Input v-model="app.signature_hash" placeholder="K8a/AINcGX7" class="h-8 text-xs" :disabled="!canWrite || !isEditable" />
                </div>
                <Button
                  v-if="canWrite && isEditable && (form.buttons[0]?.supported_apps?.length || 0) > 1"
                  type="button" variant="ghost" size="sm" class="h-8 w-8 p-0 shrink-0"
                  @click="removeSupportedApp(Number(i))"
                >
                  <X class="h-3.5 w-3.5 text-destructive" />
                </Button>
              </div>
            </div>
          </div>

          <!-- ZERO_TAP: supported apps -->
          <div v-if="authOtpType === 'ZERO_TAP'" class="space-y-3 border rounded-lg p-3">
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <Label class="text-xs">{{ $t('templates.supportedApps', 'Supported Apps') }} *</Label>
                <Button
                  v-if="canWrite && isEditable && (form.buttons[0]?.supported_apps?.length || 0) < 5"
                  type="button" variant="outline" size="xs" class="h-6 text-xs"
                  @click="addSupportedApp"
                >
                  <Plus class="h-3 w-3 mr-1" /> Add App
                </Button>
              </div>
              <div v-for="(app, i) in form.buttons[0]?.supported_apps || []" :key="i" class="flex items-end gap-2">
                <div class="flex-1 space-y-1">
                  <Label class="text-xs">Package Name *</Label>
                  <Input v-model="app.package_name" placeholder="com.example.app" class="h-8 text-xs" :disabled="!canWrite || !isEditable" />
                </div>
                <div class="flex-1 space-y-1">
                  <Label class="text-xs">Signature Hash *</Label>
                  <Input v-model="app.signature_hash" placeholder="K8a/AINcGX7" class="h-8 text-xs" :disabled="!canWrite || !isEditable" />
                </div>
                <Button
                  v-if="canWrite && isEditable && (form.buttons[0]?.supported_apps?.length || 0) > 1"
                  type="button" variant="ghost" size="sm" class="h-8 w-8 p-0 shrink-0"
                  @click="removeSupportedApp(Number(i))"
                >
                  <X class="h-3.5 w-3.5 text-destructive" />
                </Button>
              </div>
            </div>
          </div>
        </div>

        <!-- Standard template: editable body -->
        <div v-else class="space-y-1.5">
          <Label class="text-xs">{{ $t('templates.bodyContent', 'Body Content') }} *</Label>
          <Textarea
            v-model="form.body_content"
            :rows="6"
            :disabled="!canWrite || !isEditable"
          />
          <p v-if="hasMixedVariables" class="text-xs text-destructive" v-text="mixedVariablesHint" />
          <p v-else-if="hasDuplicateVariables" class="text-xs text-destructive" v-text="duplicateVariablesHint" />
          <p v-else-if="hasVariableAtEdge" class="text-xs text-destructive" v-text="variablePositionHint" />
          <p v-else class="text-xs text-muted-foreground" v-text="bodyHint" />
        </div>

        <!-- Sample Values for Variables -->
        <div v-if="!isAuthentication && allVariables.length > 0" class="space-y-3">
          <div>
            <Label class="text-xs">{{ $t('templates.sampleValues', 'Sample Values for Variables') }}</Label>
            <p class="text-xs text-muted-foreground mt-0.5">{{ $t('templates.sampleValuesHint', 'Provide example values for your variables. This helps Meta review and approve your template faster.') }}</p>
          </div>
          <div v-for="v in allVariables" :key="`${v.component}-${v.index}`" class="flex items-center gap-3">
            <span class="text-xs text-muted-foreground w-28 shrink-0 font-mono">{{ v.component }}:{{ v.label }}</span>
            <Input
              :model-value="getSampleValueForVar(v.component, v.index)"
              @update:model-value="(val: string) => setSampleValueForVar(v.component, v.index, val)"
              :placeholder="$t('templates.sampleValuePlaceholder', 'e.g. John Doe')"
              class="h-8 text-xs"
              :disabled="!canWrite || !isEditable"
            />
          </div>
        </div>

        <!-- Buttons (hidden for auth templates — OTP managed via selector above) -->
        <div v-if="!isAuthentication" class="space-y-3">
          <div class="flex items-center justify-between">
            <Label class="text-xs">{{ $t('templates.buttons', 'Buttons') }} <span class="text-muted-foreground font-normal">({{ $t('templates.maxButtonsHint', 'up to 3, optional') }})</span></Label>
            <Button
              v-if="canWrite && isEditable"
              type="button"
              variant="outline"
              size="xs"
              class="h-7 text-xs"
              @click="addButton"
              :disabled="form.buttons.length >= 3"
            >
              <Plus class="h-3 w-3 mr-1" />
              {{ $t('templates.addButton', 'Add') }}
            </Button>
          </div>
          <div v-for="(button, index) in form.buttons" :key="index" class="border rounded-lg p-3 space-y-3">
            <div class="flex items-center justify-between">
              <span class="text-xs font-medium">{{ $t('templates.button', 'Button') }} {{ index + 1 }}</span>
              <Button v-if="canWrite && isEditable" type="button" variant="ghost" size="sm" class="h-7 w-7 p-0" @click="removeButton(index)">
                <X class="h-3.5 w-3.5 text-destructive" />
              </Button>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div class="space-y-1">
                <Label class="text-xs">{{ $t('templates.buttonType', 'Type') }}</Label>
                <Select v-model="button.type" :disabled="!canWrite || !isEditable">
                  <SelectTrigger class="h-8 text-xs"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="bt in buttonTypes" :key="bt.value" :value="bt.value">
                      {{ bt.label }}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div class="space-y-1">
                <Label class="text-xs">{{ $t('templates.buttonText', 'Text') }}</Label>
                <Input v-model="button.text" class="h-8 text-xs" :disabled="!canWrite || !isEditable" />
              </div>
            </div>
            <div v-if="button.type === 'URL'" class="space-y-1">
              <Label class="text-xs">{{ $t('templates.buttonUrl', 'URL') }}</Label>
              <Input v-model="button.url" placeholder="https://example.com" class="h-8 text-xs" :disabled="!canWrite || !isEditable" />
              <div v-if="button.url && button.url.includes('{')" class="space-y-1 mt-1">
                <Label class="text-xs">{{ $t('templates.buttonUrlExample', 'URL Example') }}</Label>
                <Input v-model="button.example" placeholder="https://example.com/order/123" class="h-8 text-xs" :disabled="!canWrite || !isEditable" />
              </div>
            </div>
            <div v-if="button.type === 'PHONE_NUMBER'" class="space-y-1">
              <Label class="text-xs">{{ $t('templates.buttonPhoneNumber', 'Phone Number') }}</Label>
              <Input v-model="button.phone_number" placeholder="+1234567890" class="h-8 text-xs" :disabled="!canWrite || !isEditable" />
            </div>
            <div v-if="button.type === 'COPY_CODE'" class="space-y-1">
              <Label class="text-xs">{{ $t('templates.copyCodeExample', 'Example Code') }}</Label>
              <Input v-model="button.example" placeholder="SAVE20" class="h-8 text-xs" :disabled="!canWrite || !isEditable" />
            </div>
            <div v-if="button.type === 'FLOW'" class="space-y-2">
              <div class="space-y-1">
                <Label class="text-xs">{{ $t('templates.flow', 'Flow') }}</Label>
                <Select v-model="button.flow_id" :disabled="!canWrite || !isEditable">
                  <SelectTrigger class="h-8 text-xs">
                    <SelectValue :placeholder="$t('templates.selectFlow', 'Select a Flow')" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="flow in whatsappFlows" :key="flow.id" :value="flow.meta_flow_id || flow.id">
                      {{ flow.name }}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div class="space-y-1">
                <Label class="text-xs">{{ $t('templates.flowAction', 'Flow Action') }}</Label>
                <Select v-model="button.flow_action" :disabled="!canWrite || !isEditable">
                  <SelectTrigger class="h-8 text-xs">
                    <SelectValue placeholder="navigate" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="navigate">Navigate</SelectItem>
                    <SelectItem value="data_exchange">Data Exchange</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div v-if="button.flow_action === 'navigate' && button.flow_id && getFlowScreens(button.flow_id).length > 0" class="space-y-1">
                <Label class="text-xs">{{ $t('templates.navigateScreen', 'Screen') }}</Label>
                <Select v-model="button.navigate_screen" :disabled="!canWrite || !isEditable">
                  <SelectTrigger class="h-8 text-xs">
                    <SelectValue :placeholder="$t('templates.selectScreen', 'Select Screen')" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="screen in getFlowScreens(button.flow_id)" :key="screen" :value="screen">
                      {{ screen }}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div v-else-if="button.flow_action === 'navigate'" class="space-y-1">
                <Label class="text-xs">{{ $t('templates.navigateScreen', 'Screen') }}</Label>
                <Input v-model="button.navigate_screen" placeholder="SCREEN_ID" class="h-8 text-xs" :disabled="!canWrite || !isEditable" />
              </div>
            </div>
            <div v-if="button.type === 'OTP'" class="space-y-2">
              <div class="space-y-1">
                <Label class="text-xs">{{ $t('templates.otpType', 'OTP Type') }}</Label>
                <Select v-model="button.otp_type" :disabled="!canWrite || !isEditable">
                  <SelectTrigger class="h-8 text-xs">
                    <SelectValue placeholder="Copy Code" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="COPY_CODE">Copy Code</SelectItem>
                    <SelectItem value="ONE_TAP">One Tap</SelectItem>
                    <SelectItem value="ZERO_TAP">Zero Tap</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div v-if="button.otp_type === 'ONE_TAP'" class="space-y-1">
                <Label class="text-xs">{{ $t('templates.autofillText', 'Autofill Text') }}</Label>
                <Input v-model="button.autofill_text" placeholder="Autofill" class="h-8 text-xs" :disabled="!canWrite || !isEditable" />
              </div>
              <div v-if="button.otp_type === 'ONE_TAP' || button.otp_type === 'ZERO_TAP'" class="space-y-1">
                <Label class="text-xs">{{ $t('templates.packageName', 'Package Name') }}</Label>
                <Input v-model="button.package_name" placeholder="com.example.app" class="h-8 text-xs" :disabled="!canWrite || !isEditable" />
                <Label class="text-xs">{{ $t('templates.signatureHash', 'Signature Hash') }}</Label>
                <Input v-model="button.signature_hash" placeholder="App signature hash" class="h-8 text-xs" :disabled="!canWrite || !isEditable" />
              </div>
            </div>
          </div>
        </div>

        <div v-if="!isAuthentication" class="space-y-1.5">
          <Label class="text-xs">{{ $t('templates.footerContent', 'Footer Content') }}</Label>
          <Textarea
            v-model="form.footer_content"
            :rows="2"
            :disabled="!canWrite || !isEditable"
          />
        </div>
        </CardContent>
    </Card>

    <!-- Activity Log -->
    <AuditLogPanel
      v-if="template && !isNew"
      :key="auditRefreshKey"
      resource-type="template"
      :resource-id="template.id"
    />

    <!-- Sidebar -->
    <template v-if="!isNew" #sidebar>
      <MetadataPanel
        :created-at="template?.created_at"
        :updated-at="template?.updated_at"
        :created-by-name="template?.created_by_name"
        :updated-by-name="template?.updated_by_name"
      />

      <!-- Editing Guidelines -->
      <Card v-if="template?.meta_template_id">
        <CardHeader class="pb-3">
          <CardTitle class="text-sm font-medium">{{ $t('templates.editingGuidelines', 'Editing Guidelines') }}</CardTitle>
        </CardHeader>
        <CardContent>
          <ul class="list-disc list-inside space-y-2 text-sm text-muted-foreground">
            <li>{{ $t('templates.guideEditLimit', 'Approved templates can be edited up to 10 times in a 30-day window') }}</li>
            <li>{{ $t('templates.guideDailyLimit', 'Within a 24-hour period, you are limited to 1 edit') }}</li>
            <li>{{ $t('templates.guideReview', 'Editing triggers a new review process, which can take up to 24 hours') }}</li>
            <li>{{ $t('templates.guideEditable', 'You can edit: body, header, footer, and buttons') }}</li>
            <li>{{ $t('templates.guideNotEditable', 'You cannot change: name, language, or category of approved templates') }}</li>
            <li>{{ $t('templates.guidePending', 'While under review, the template cannot be used to send messages') }}</li>
            <li>{{ $t('templates.guideRejected', 'Rejected or paused templates have no edit limits') }}</li>
          </ul>
        </CardContent>
      </Card>
    </template>
  </DetailPageLayout>

  <!-- Delete Confirmation -->
  <AlertDialog v-model:open="deleteDialogOpen">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ $t('templates.deleteTemplate', 'Delete Template') }}</AlertDialogTitle>
        <AlertDialogDescription>
          {{ $t('templates.deleteConfirm', 'Are you sure? This action cannot be undone.') }}
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>{{ $t('common.cancel') }}</AlertDialogCancel>
        <AlertDialogAction @click="deleteTemplate">{{ $t('common.delete') }}</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>

  <!-- Preview Dialog -->
  <AlertDialog v-model:open="isPreviewOpen">
    <AlertDialogContent class="max-w-md">
      <AlertDialogHeader>
        <AlertDialogTitle>{{ $t('templates.templatePreview', 'Template Preview') }}</AlertDialogTitle>
        <AlertDialogDescription>{{ template?.display_name || template?.name }}</AlertDialogDescription>
      </AlertDialogHeader>
      <div v-if="template" class="py-2">
        <div class="bg-gray-800 light:bg-[#e5ddd5] rounded-lg p-4">
          <div class="bg-gray-700 light:bg-white rounded-lg shadow max-w-[280px] overflow-hidden">
            <div v-if="template.header_type && template.header_type !== 'NONE'" class="p-3 border-b">
              <div v-if="template.header_type === 'TEXT'" class="font-semibold">{{ previewHeader }}</div>
              <div v-else class="h-32 bg-gray-600 light:bg-gray-200 rounded flex items-center justify-center">
                <span class="text-sm text-gray-400">{{ template.header_type }}</span>
              </div>
            </div>
            <div class="p-3">
              <p class="text-sm whitespace-pre-wrap">{{ previewBody }}</p>
            </div>
            <div v-if="template.footer_content" class="px-3 pb-3">
              <p class="text-xs text-gray-500">{{ template.footer_content }}</p>
            </div>
            <div v-if="template.buttons && template.buttons.length > 0" class="border-t">
              <div v-for="(btn, idx) in template.buttons" :key="idx" class="border-b last:border-b-0">
                <button class="w-full py-2 text-sm text-blue-500 hover:bg-gray-600 light:hover:bg-gray-50">
                  {{ btn.text || btn.title || 'Button' }}
                </button>
              </div>
            </div>
          </div>
        </div>
        <div class="mt-4 space-y-2 text-sm">
          <div class="flex justify-between">
            <span class="text-muted-foreground">{{ $t('templates.status', 'Status') }}:</span>
            <Badge :variant="statusVariant">{{ template.status }}</Badge>
          </div>
          <div class="flex justify-between">
            <span class="text-muted-foreground">{{ $t('templates.category', 'Category') }}:</span>
            <span>{{ template.category }}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-muted-foreground">{{ $t('templates.language', 'Language') }}:</span>
            <span>{{ languages.find(l => l.code === template!.language)?.name || template.language }}</span>
          </div>
          <div v-if="template.meta_template_id" class="flex justify-between">
            <span class="text-muted-foreground">Meta ID:</span>
            <span class="font-mono text-xs">{{ template.meta_template_id }}</span>
          </div>
        </div>
      </div>
      <AlertDialogFooter>
        <AlertDialogCancel>{{ $t('common.close', 'Close') }}</AlertDialogCancel>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>

  <!-- Publish Confirmation -->
  <AlertDialog v-model:open="publishDialogOpen">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>
          {{ template?.meta_template_id ? $t('templates.republishTemplate', 'Republish Template') : $t('templates.publishTemplate', 'Publish Template') }}
        </AlertDialogTitle>
        <AlertDialogDescription>
          {{ template?.meta_template_id
            ? $t('templates.republishConfirm', 'This will resubmit the template to Meta for approval.')
            : $t('templates.publishConfirm', 'This will submit the template to Meta for approval.')
          }}
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>{{ $t('common.cancel') }}</AlertDialogCancel>
        <AlertDialogAction @click="confirmPublish" :disabled="isPublishing">
          {{ template?.meta_template_id ? $t('templates.republish', 'Republish') : $t('templates.publish', 'Publish') }}
        </AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>

  <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
