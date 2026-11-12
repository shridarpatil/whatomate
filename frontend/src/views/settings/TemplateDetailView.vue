<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { api, templatesService, flowsService } from '@/services/api'
import { toast } from '@/lib/toast'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import Modal from '@/components/oat/Modal.vue'
import { ConfirmDialog } from '@/components/shared'
import {
  FileText,
  Trash2,
  Save,
  Upload,
  Check,
  Eye,
  Send,
  Plus,
  X,
  Info,
} from '@/lib/icons'
import { getErrorMessage } from '@/lib/api-utils'
import { getQualityBadgeVariant, getQualityRatingLabel } from '@/lib/utils'

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
  quality_rating?: string
}

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const authStore = useAuthStore()

const bodyHint = 'Use {{1}}, {{2}} for positional or {{name}}, {{email}} for named parameters.'
const mixedVariablesHint = 'Cannot mix positional ({{1}}, {{2}}) and named ({{name}}) variables. Use one type only.'
const duplicateVariablesHint = 'Duplicate variables found. Each variable should appear only once in the template.'
const variablePositionHint = 'Variables cannot be at the very start or end of the template body.'
const headerTooManyVariablesHint = 'Meta allows at most one variable in a TEXT header.'

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

function setAuthOtpType(type: string) {
  if (!type) return
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

// Meta allows at most one variable in a TEXT header. Count unique names so
// {{name}} … {{name}} (same variable referenced twice) still passes.
const hasTooManyHeaderVariables = computed(() => {
  if (form.value.header_type !== 'TEXT') return false
  return new Set(headerVariables.value).size > 1
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
    case 'APPROVED': return 'success' as const
    case 'REJECTED': return 'danger' as const
    case 'PENDING': return 'warning' as const
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
  if (!isAuthentication.value
    && ['IMAGE', 'VIDEO', 'DOCUMENT'].includes(form.value.header_type)
    && !form.value.header_content) {
    const fallback = `Upload a sample ${form.value.header_type.toLowerCase()} before saving — Meta requires it for template approval.`
    toast.error(t('templates.headerMediaRequired', fallback))
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
    if (hasTooManyHeaderVariables.value) {
      toast.error(t('templates.headerTooManyVariables', headerTooManyVariablesHint))
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
      header_content: isAuthentication.value ? '' : form.value.header_content,
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
    back-link="/templates"
    :breadcrumbs="breadcrumbs"
    :is-loading="isLoading"
    :is-not-found="isNotFound"
    :not-found-title="$t('templates.notFound', 'Template not found')"
  >
    <template #actions>
      <div class="hstack gap-2">
        <button v-if="!isNew" type="button" class="outline small" @click="isPreviewOpen = true">
          <Eye :size="16" /> {{ $t('templates.preview', 'Preview') }}
        </button>
        <button
          v-if="canPublish"
          type="button"
          class="outline small"
          :disabled="isPublishing"
          :aria-busy="isPublishing ? 'true' : undefined"
          @click="publishDialogOpen = true"
        >
          <Send v-if="!isPublishing" :size="16" />
          {{ template?.meta_template_id ? $t('templates.republish', 'Republish') : $t('templates.publish', 'Publish') }}
        </button>
        <button
          v-if="canWrite && (hasChanges || isNew)"
          type="button"
          class="small"
          :disabled="isSaving"
          :aria-busy="isSaving ? 'true' : undefined"
          @click="save"
        >
          <Save v-if="!isSaving" :size="16" /> {{ isSaving ? $t('common.saving', 'Saving...') : isNew ? $t('common.create') : $t('common.save') }}
        </button>
        <button v-if="canDelete && !isNew" type="button" class="small" data-variant="danger" @click="deleteDialogOpen = true">
          <Trash2 :size="16" /> {{ $t('common.delete') }}
        </button>
      </div>
    </template>

    <!-- Details Card -->
    <article class="card">
      <details :open="isDetailsOpen" @toggle="isDetailsOpen = ($event.target as HTMLDetailsElement).open">
        <summary>
          <span class="hstack gap-2">
            {{ $t('templates.details', 'Details') }}
            <span v-if="!isNew && template?.status" class="badge" :data-variant="statusVariant">{{ template.status }}</span>
            <span v-if="!isNew && template?.quality_rating" class="badge" :data-variant="getQualityBadgeVariant(template.quality_rating)">
              {{ getQualityRatingLabel(template.quality_rating, t) }}
            </span>
          </span>
        </summary>
        <div class="vstack gap-4 mt-4">
          <!-- Edit limits info for approved templates -->
          <div v-if="template?.status?.toUpperCase() === 'APPROVED'" role="alert" class="hstack gap-2">
            <Info :size="14" />
            <span>{{ $t('templates.editLimitsInfo', 'Approved templates can be edited up to 10 times in 30 days (1 edit per 24 hours). Editing triggers a new review which may take up to 24 hours. Name, language, and category cannot be changed.') }}</span>
          </div>
          <label data-field>
            {{ $t('templates.whatsappAccount', 'WhatsApp Account') }}
            <select v-model="form.whatsapp_account" :disabled="!canWrite || !!template?.meta_template_id">
              <option value="" disabled>{{ $t('templates.selectAccount', 'Select account') }}</option>
              <option v-for="account in accounts" :key="account.id" :value="account.name">{{ account.name }}</option>
            </select>
          </label>
          <label data-field>
            {{ $t('templates.name', 'Name') }} *
            <input v-model="form.name" :disabled="!canWrite || !!template?.meta_template_id" />
          </label>
          <label data-field>
            {{ $t('templates.displayName', 'Display Name') }}
            <input v-model="form.display_name" :disabled="!canWrite || !isEditable" />
          </label>
          <label data-field>
            {{ $t('templates.language', 'Language') }}
            <select v-model="form.language" :disabled="!canWrite || !!template?.meta_template_id">
              <option v-for="lang in languages" :key="lang.code" :value="lang.code">{{ lang.name }}</option>
            </select>
          </label>
          <label data-field>
            {{ $t('templates.category', 'Category') }}
            <select v-model="form.category" :disabled="!canWrite || !isEditable || (!!template?.meta_template_id && template?.status?.toUpperCase() === 'APPROVED')">
              <option v-for="cat in categories" :key="cat.value" :value="cat.value">{{ cat.label }}</option>
            </select>
          </label>
        </div>
      </details>
    </article>

    <!-- Content Card -->
    <article class="card">
      <header>
        <h3>{{ $t('templates.content', 'Content') }}</h3>
      </header>
      <div class="vstack gap-4">
        <label v-if="!isAuthentication" data-field>
          {{ $t('templates.headerType', 'Header Type') }}
          <select v-model="form.header_type" :disabled="!canWrite || !isEditable">
            <option v-for="ht in headerTypes" :key="ht.value" :value="ht.value">{{ ht.label }}</option>
          </select>
        </label>
        <label v-if="form.header_type === 'TEXT'" data-field>
          {{ $t('templates.headerContent', 'Header Content') }}
          <input id="header-content" v-model="form.header_content" :disabled="!canWrite || !isEditable" :aria-invalid="hasTooManyHeaderVariables ? 'true' : undefined" />
          <div v-if="hasTooManyHeaderVariables" class="error" role="status" v-text="headerTooManyVariablesHint" />
        </label>

        <!-- Header Media Upload for IMAGE/VIDEO/DOCUMENT -->
        <div v-else-if="['IMAGE', 'VIDEO', 'DOCUMENT'].includes(form.header_type)" class="vstack gap-2">
          <strong>{{ $t('templates.headerSample', 'Header') }} {{ form.header_type.toLowerCase() }}</strong>
          <div class="hstack gap-2">
            <input
              type="file"
              class="flex-1"
              :accept="getAcceptedFileTypes()"
              :disabled="!canWrite || !isEditable"
              :aria-label="$t('templates.headerSample', 'Header') + ' ' + form.header_type.toLowerCase()"
              @change="onHeaderMediaFileChange"
            />
            <button
              type="button"
              class="small"
              :disabled="!headerMediaFile || headerMediaUploading"
              :aria-busy="headerMediaUploading ? 'true' : undefined"
              @click="uploadHeaderMedia"
            >
              <Upload v-if="!headerMediaUploading" :size="14" />
              {{ $t('templates.uploadMedia', 'Upload') }}
            </button>
          </div>
          <small v-if="headerMediaFilename && !headerMediaHandle" class="text-light">
            {{ headerMediaFilename }}
          </small>
          <div v-if="headerMediaHandle" role="alert" data-variant="success" class="vstack gap-1">
            <div class="hstack gap-2">
              <Check :size="14" />
              <span>{{ $t('templates.mediaUploadedSuccess', 'Media uploaded') }}</span>
            </div>
            <small class="truncate"><code>Handle: {{ headerMediaHandle.substring(0, 40) }}...</code></small>
          </div>
          <small class="text-light">
            <span v-if="form.header_type === 'IMAGE'">JPEG or PNG, max 5MB</span>
            <span v-else-if="form.header_type === 'VIDEO'">MP4, max 16MB</span>
            <span v-else-if="form.header_type === 'DOCUMENT'">PDF, max 100MB</span>
          </small>
        </div>

        <!-- Authentication template: fixed body & options -->
        <div v-if="isAuthentication" class="vstack gap-4">
          <!-- OTP Code Delivery Method -->
          <label data-field>
            {{ $t('templates.codeDelivery', 'Code Delivery Method') }}
            <select :value="authOtpType" :disabled="!canWrite || !isEditable" @change="setAuthOtpType(($event.target as HTMLSelectElement).value)">
              <option value="COPY_CODE">Copy Code</option>
              <option value="ONE_TAP">One-Tap Autofill (Android only)</option>
              <option value="ZERO_TAP">Zero-Tap (Android only)</option>
            </select>
            <small data-hint>
              <span v-if="authOtpType === 'COPY_CODE'">User taps a button to copy the code to clipboard.</span>
              <span v-else-if="authOtpType === 'ONE_TAP'">User taps a button to autofill the code in your app. Requires app configuration.</span>
              <span v-else-if="authOtpType === 'ZERO_TAP'">Code is delivered to your app automatically. Requires app configuration.</span>
            </small>
          </label>

          <div class="vstack gap-1">
            <strong>{{ $t('templates.bodyContent', 'Body Content') }}</strong>
            <div class="card">
              <code>{'{{1}}'}</code> is your verification code.
              <div v-if="form.add_security_recommendation">For your security, do not share this code.</div>
            </div>
            <small class="text-light">Authentication templates use fixed preset text defined by Meta.</small>
          </div>
          <label>
            <input
              id="security-rec"
              type="checkbox"
              v-model="form.add_security_recommendation"
              :disabled="!canWrite || !isEditable"
            />
            {{ $t('templates.addSecurityRecommendation', 'Add security recommendation') }}
          </label>
          <div class="vstack gap-2">
            <label>
              <input
                id="code-expiration"
                type="checkbox"
                :checked="form.code_expiration_minutes > 0"
                :disabled="!canWrite || !isEditable"
                @change="form.code_expiration_minutes = ($event.target as HTMLInputElement).checked ? 10 : 0"
              />
              {{ $t('templates.addCodeExpiration', 'Add expiration time for the code') }}
            </label>
            <div v-if="form.code_expiration_minutes > 0" class="hstack gap-2">
              <input
                type="number"
                :value="form.code_expiration_minutes"
                min="1"
                max="90"
                :aria-label="$t('templates.addCodeExpiration', 'Add expiration time for the code')"
                :disabled="!canWrite || !isEditable"
                @input="form.code_expiration_minutes = ($event.target as HTMLInputElement).value ? parseInt(($event.target as HTMLInputElement).value) : 0"
              />
              <small class="text-light">minutes (1-90)</small>
            </div>
            <small v-if="form.code_expiration_minutes > 0" class="text-light">
              Footer: "This code expires in {{ form.code_expiration_minutes }} minutes."
            </small>
          </div>
          <!-- Zero-Tap Terms of Service -->
          <div v-if="authOtpType === 'ZERO_TAP'" role="alert" data-variant="warning">
            <label>
              <input
                id="zero-tap-tos"
                type="checkbox"
                v-model="zeroTapAccepted"
                :disabled="!canWrite || !isEditable"
              />
              <span>
                By selecting zero-tap, I understand that my business's use of zero-tap authentication is subject to the
                <a href="https://www.whatsapp.com/legal/business-terms/" target="_blank" rel="noopener noreferrer">WhatsApp Business Terms of Service</a>.
                It is my business's responsibility to ensure its customers expect that the code will be automatically filled in on their behalf when they choose to receive the zero-tap code through WhatsApp.
              </span>
            </label>
          </div>

          <!-- ONE_TAP / ZERO_TAP: autofill text + supported apps -->
          <fieldset v-if="authOtpType === 'ONE_TAP' || authOtpType === 'ZERO_TAP'" class="vstack gap-4">
            <label v-if="authOtpType === 'ONE_TAP'" data-field>
              {{ $t('templates.autofillText', 'Autofill Text') }}
              <input v-model="form.buttons[0].autofill_text" placeholder="Autofill" :disabled="!canWrite || !isEditable" />
            </label>
            <div class="vstack gap-2">
              <div class="hstack justify-between">
                <strong>{{ $t('templates.supportedApps', 'Supported Apps') }} *</strong>
                <button
                  v-if="canWrite && isEditable && (form.buttons[0]?.supported_apps?.length || 0) < 5"
                  type="button"
                  class="outline small"
                  @click="addSupportedApp"
                >
                  <Plus :size="14" /> Add App
                </button>
              </div>
              <div v-for="(app, i) in form.buttons[0]?.supported_apps || []" :key="i" class="hstack gap-2">
                <label data-field class="flex-1">
                  Package Name *
                  <input v-model="app.package_name" placeholder="com.example.app" :disabled="!canWrite || !isEditable" />
                </label>
                <label data-field class="flex-1">
                  Signature Hash *
                  <input v-model="app.signature_hash" placeholder="K8a/AINcGX7" :disabled="!canWrite || !isEditable" />
                </label>
                <button
                  v-if="canWrite && isEditable && (form.buttons[0]?.supported_apps?.length || 0) > 1"
                  type="button"
                  class="ghost small icon"
                  :title="$t('common.remove')"
                  :aria-label="$t('common.remove')"
                  @click="removeSupportedApp(Number(i))"
                >
                  <X :size="14" />
                </button>
              </div>
            </div>
          </fieldset>
        </div>

        <!-- Standard template: editable body -->
        <label v-else data-field>
          {{ $t('templates.bodyContent', 'Body Content') }} *
          <textarea
            v-model="form.body_content"
            rows="6"
            :disabled="!canWrite || !isEditable"
            :aria-invalid="hasMixedVariables || hasDuplicateVariables || hasVariableAtEdge ? 'true' : undefined"
          />
          <div v-if="hasMixedVariables" class="error" role="status" v-text="mixedVariablesHint" />
          <div v-else-if="hasDuplicateVariables" class="error" role="status" v-text="duplicateVariablesHint" />
          <div v-else-if="hasVariableAtEdge" class="error" role="status" v-text="variablePositionHint" />
          <small v-else data-hint v-text="bodyHint" />
        </label>

        <!-- Sample Values for Variables -->
        <div v-if="!isAuthentication && allVariables.length > 0" class="vstack gap-2">
          <div>
            <strong>{{ $t('templates.sampleValues', 'Sample Values for Variables') }}</strong>
            <p class="text-light">{{ $t('templates.sampleValuesHint', 'Provide example values for your variables. This helps Meta review and approve your template faster.') }}</p>
          </div>
          <div v-for="v in allVariables" :key="`${v.component}-${v.index}`" class="hstack gap-2">
            <code>{{ v.component }}:{{ v.label }}</code>
            <input
              class="flex-1"
              :value="getSampleValueForVar(v.component, v.index)"
              :placeholder="$t('templates.sampleValuePlaceholder', 'e.g. John Doe')"
              :aria-label="`${v.component}:${v.label}`"
              :disabled="!canWrite || !isEditable"
              @input="setSampleValueForVar(v.component, v.index, ($event.target as HTMLInputElement).value)"
            />
          </div>
        </div>

        <!-- Buttons (hidden for auth templates — OTP managed via selector above) -->
        <div v-if="!isAuthentication" class="vstack gap-2">
          <div class="hstack justify-between">
            <strong>{{ $t('templates.buttons', 'Buttons') }} <small class="text-light">({{ $t('templates.maxButtonsHint', 'up to 3, optional') }})</small></strong>
            <button
              v-if="canWrite && isEditable"
              type="button"
              class="outline small"
              :disabled="form.buttons.length >= 3"
              @click="addButton"
            >
              <Plus :size="14" />
              {{ $t('templates.addButton', 'Add') }}
            </button>
          </div>
          <fieldset v-for="(button, index) in form.buttons" :key="index" class="vstack gap-2">
            <div class="hstack justify-between">
              <strong>{{ $t('templates.button', 'Button') }} {{ index + 1 }}</strong>
              <button
                v-if="canWrite && isEditable"
                type="button"
                class="ghost small icon"
                :title="$t('common.remove')"
                :aria-label="$t('common.remove')"
                @click="removeButton(index)"
              >
                <X :size="14" />
              </button>
            </div>
            <div class="row">
              <label data-field class="col-6">
                {{ $t('templates.buttonType', 'Type') }}
                <select v-model="button.type" :disabled="!canWrite || !isEditable">
                  <option v-for="bt in buttonTypes" :key="bt.value" :value="bt.value">{{ bt.label }}</option>
                </select>
              </label>
              <label data-field class="col-6">
                {{ $t('templates.buttonText', 'Text') }}
                <input v-model="button.text" :disabled="!canWrite || !isEditable" />
              </label>
            </div>
            <div v-if="button.type === 'URL'" class="vstack gap-2">
              <label data-field>
                {{ $t('templates.buttonUrl', 'URL') }}
                <input v-model="button.url" placeholder="https://example.com" :disabled="!canWrite || !isEditable" />
              </label>
              <label v-if="button.url && button.url.includes('{')" data-field>
                {{ $t('templates.buttonUrlExample', 'URL Example') }}
                <input v-model="button.example" placeholder="https://example.com/order/123" :disabled="!canWrite || !isEditable" />
              </label>
            </div>
            <label v-if="button.type === 'PHONE_NUMBER'" data-field>
              {{ $t('templates.buttonPhoneNumber', 'Phone Number') }}
              <input v-model="button.phone_number" placeholder="+1234567890" :disabled="!canWrite || !isEditable" />
            </label>
            <label v-if="button.type === 'COPY_CODE'" data-field>
              {{ $t('templates.copyCodeExample', 'Example Code') }}
              <input v-model="button.example" placeholder="SAVE20" :disabled="!canWrite || !isEditable" />
            </label>
            <div v-if="button.type === 'FLOW'" class="vstack gap-2">
              <label data-field>
                {{ $t('templates.flow', 'Flow') }}
                <select v-model="button.flow_id" :disabled="!canWrite || !isEditable">
                  <option value="" disabled>{{ $t('templates.selectFlow', 'Select a Flow') }}</option>
                  <option v-for="flow in whatsappFlows" :key="flow.id" :value="flow.meta_flow_id || flow.id">{{ flow.name }}</option>
                </select>
              </label>
              <label data-field>
                {{ $t('templates.flowAction', 'Flow Action') }}
                <select v-model="button.flow_action" :disabled="!canWrite || !isEditable">
                  <option value="" disabled>navigate</option>
                  <option value="navigate">Navigate</option>
                  <option value="data_exchange">Data Exchange</option>
                </select>
              </label>
              <label v-if="button.flow_action === 'navigate' && button.flow_id && getFlowScreens(button.flow_id).length > 0" data-field>
                {{ $t('templates.navigateScreen', 'Screen') }}
                <select v-model="button.navigate_screen" :disabled="!canWrite || !isEditable">
                  <option value="" disabled>{{ $t('templates.selectScreen', 'Select Screen') }}</option>
                  <option v-for="screen in getFlowScreens(button.flow_id)" :key="screen" :value="screen">{{ screen }}</option>
                </select>
              </label>
              <label v-else-if="button.flow_action === 'navigate'" data-field>
                {{ $t('templates.navigateScreen', 'Screen') }}
                <input v-model="button.navigate_screen" placeholder="SCREEN_ID" :disabled="!canWrite || !isEditable" />
              </label>
            </div>
            <div v-if="button.type === 'OTP'" class="vstack gap-2">
              <label data-field>
                {{ $t('templates.otpType', 'OTP Type') }}
                <select v-model="button.otp_type" :disabled="!canWrite || !isEditable">
                  <option value="" disabled>Copy Code</option>
                  <option value="COPY_CODE">Copy Code</option>
                  <option value="ONE_TAP">One Tap</option>
                  <option value="ZERO_TAP">Zero Tap</option>
                </select>
              </label>
              <label v-if="button.otp_type === 'ONE_TAP'" data-field>
                {{ $t('templates.autofillText', 'Autofill Text') }}
                <input v-model="button.autofill_text" placeholder="Autofill" :disabled="!canWrite || !isEditable" />
              </label>
              <div v-if="button.otp_type === 'ONE_TAP' || button.otp_type === 'ZERO_TAP'" class="vstack gap-2">
                <label data-field>
                  {{ $t('templates.packageName', 'Package Name') }}
                  <input v-model="button.package_name" placeholder="com.example.app" :disabled="!canWrite || !isEditable" />
                </label>
                <label data-field>
                  {{ $t('templates.signatureHash', 'Signature Hash') }}
                  <input v-model="button.signature_hash" placeholder="App signature hash" :disabled="!canWrite || !isEditable" />
                </label>
              </div>
            </div>
          </fieldset>
        </div>

        <label v-if="!isAuthentication" data-field>
          {{ $t('templates.footerContent', 'Footer Content') }}
          <textarea
            v-model="form.footer_content"
            rows="2"
            :disabled="!canWrite || !isEditable"
          />
        </label>
      </div>
    </article>

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
      <article v-if="template?.meta_template_id" class="card">
        <header>
          <h3>{{ $t('templates.editingGuidelines', 'Editing Guidelines') }}</h3>
        </header>
        <ul class="text-light">
          <li>{{ $t('templates.guideEditLimit', 'Approved templates can be edited up to 10 times in a 30-day window') }}</li>
          <li>{{ $t('templates.guideDailyLimit', 'Within a 24-hour period, you are limited to 1 edit') }}</li>
          <li>{{ $t('templates.guideReview', 'Editing triggers a new review process, which can take up to 24 hours') }}</li>
          <li>{{ $t('templates.guideEditable', 'You can edit: body, header, footer, and buttons') }}</li>
          <li>{{ $t('templates.guideNotEditable', 'You cannot change: name, language, or category of approved templates') }}</li>
          <li>{{ $t('templates.guidePending', 'While under review, the template cannot be used to send messages') }}</li>
          <li>{{ $t('templates.guideRejected', 'Rejected or paused templates have no edit limits') }}</li>
        </ul>
      </article>
    </template>
  </DetailPageLayout>

  <!-- Delete Confirmation -->
  <ConfirmDialog
    v-model:open="deleteDialogOpen"
    :title="$t('templates.deleteTemplate', 'Delete Template')"
    :description="$t('templates.deleteConfirm', 'Are you sure? This action cannot be undone.')"
    :confirm-label="$t('common.delete')"
    @confirm="deleteTemplate"
  />

  <!-- Preview Dialog -->
  <Modal
    v-model:open="isPreviewOpen"
    :title="$t('templates.templatePreview', 'Template Preview')"
    :description="template?.display_name || template?.name"
  >
    <div v-if="template" class="vstack gap-4">
      <div class="card vstack gap-2">
        <div v-if="template.header_type && template.header_type !== 'NONE'">
          <strong v-if="template.header_type === 'TEXT'">{{ previewHeader }}</strong>
          <div v-else class="card align-center">
            <small class="text-light">{{ template.header_type }}</small>
          </div>
        </div>
        <p class="pre-wrap">{{ previewBody }}</p>
        <small v-if="template.footer_content" class="text-light">{{ template.footer_content }}</small>
        <div v-if="template.buttons && template.buttons.length > 0" class="vstack gap-1">
          <button v-for="(btn, idx) in template.buttons" :key="idx" type="button" class="outline small">
            {{ btn.text || btn.title || 'Button' }}
          </button>
        </div>
      </div>
      <div class="vstack gap-2">
        <div class="hstack justify-between">
          <span class="text-light">{{ $t('templates.status', 'Status') }}:</span>
          <span class="badge" :data-variant="statusVariant">{{ template.status }}</span>
        </div>
        <div v-if="template.quality_rating" class="hstack justify-between">
          <span class="text-light">{{ $t('templates.qualityRating', 'Quality Rating') }}:</span>
          <span class="badge" :data-variant="getQualityBadgeVariant(template.quality_rating)">
            {{ getQualityRatingLabel(template.quality_rating, t) }}
          </span>
        </div>
        <div class="hstack justify-between">
          <span class="text-light">{{ $t('templates.category', 'Category') }}:</span>
          <span>{{ template.category }}</span>
        </div>
        <div class="hstack justify-between">
          <span class="text-light">{{ $t('templates.language', 'Language') }}:</span>
          <span>{{ languages.find(l => l.code === template!.language)?.name || template.language }}</span>
        </div>
        <div v-if="template.meta_template_id" class="hstack justify-between">
          <span class="text-light">Meta ID:</span>
          <code>{{ template.meta_template_id }}</code>
        </div>
      </div>
    </div>
    <template #footer>
      <button type="button" class="outline" @click="isPreviewOpen = false">{{ $t('common.close', 'Close') }}</button>
    </template>
  </Modal>

  <!-- Publish Confirmation -->
  <ConfirmDialog
    v-model:open="publishDialogOpen"
    :title="template?.meta_template_id ? $t('templates.republishTemplate', 'Republish Template') : $t('templates.publishTemplate', 'Publish Template')"
    :description="template?.meta_template_id
      ? $t('templates.republishConfirm', 'This will resubmit the template to Meta for approval.')
      : $t('templates.publishConfirm', 'This will submit the template to Meta for approval.')"
    :confirm-label="template?.meta_template_id ? $t('templates.republish', 'Republish') : $t('templates.publish', 'Publish')"
    :is-submitting="isPublishing"
    @confirm="confirmPublish"
  />

  <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
