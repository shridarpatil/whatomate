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
import TemplateEditor from './TemplateEditor.vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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
import { FileText, Trash2, Save, Loader2, Send, Info } from 'lucide-vue-next'
import { getErrorMessage } from '@/lib/api-utils'
import { getQualityBadgeClass, getQualityRatingLabel } from '@/lib/utils'

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

const templateId = computed(() => route.params.id as string)
const isNew = computed(() => templateId.value === 'new')
const isAuthentication = computed(() => form.value.category === 'AUTHENTICATION')

// OTP type for auth templates — derived from the buttons array
const authOtpType = computed(() => {
  if (!isAuthentication.value) return 'COPY_CODE'
  const otpBtn = form.value.buttons.find((b: any) => b.type === 'OTP')
  return otpBtn?.otp_type || 'COPY_CODE'
})

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
const isDetailsOpen = ref(true)

// Picked in the editor, uploaded to Meta only when the template is saved.
const pendingMediaFile = ref<File | null>(null)

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
  // UI-only: gates saving a ZERO_TAP template. Not sent to the API.
  zero_tap_accepted: false,
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

// index must match TemplateEditor.sampleIndexFor(): positional vars carry their
// index in the name, named vars take their slot. Diverging here would make the
// prune below delete samples the editor has just written.
const varIndex = (name: string, slot: number) =>
  /^\d+$/.test(name) ? parseInt(name, 10) : slot + 1

const allVariables = computed(() => {
  const vars: { component: string; name: string; index: number }[] = []
  headerVariables.value.forEach((v, i) => vars.push({ component: 'header', name: v, index: varIndex(v, i) }))
  bodyVariables.value.forEach((v, i) => vars.push({ component: 'body', name: v, index: varIndex(v, i) }))
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

// Meta requires positional variables to run 1, 2, 3… with no gaps.
const firstSequenceGap = computed(() => {
  const nums = bodyVariables.value
    .filter(v => /^\d+$/.test(v))
    .map(Number)
    .sort((a, b) => a - b)
  const unique = [...new Set(nums)]
  for (let i = 0; i < unique.length; i++) {
    if (unique[i] !== i + 1) return { expected: i + 1, found: unique[i] }
  }
  return null
})

// Meta rejects a template whose variables have no example values.
const missingSamples = computed(() =>
  allVariables.value
    .filter(v => !form.value.sample_values.some(
      (s: any) => s.component === v.component && s.index === v.index && String(s.value || '').trim()
    ))
    .map(v => `{{${v.name}}}`)
)

const firstButtonError = computed(() => {
  for (const btn of form.value.buttons as any[]) {
    if (btn.type === 'OTP') continue
    if (!String(btn.text || '').trim()) return 'Every button needs a label.'
    if (btn.type === 'URL') {
      const url = String(btn.url || '').trim()
      if (!url || url === '{{1}}') return 'Website URL buttons need a URL.'
      if (btn.urlType === 'DYNAMIC' && !url.endsWith('{{1}}')) return 'A dynamic URL must end with {{1}}.'
    }
    if (btn.type === 'PHONE_NUMBER' && !String(btn.phone_number || '').trim()) return 'Phone buttons need a number.'
    if (btn.type === 'FLOW' && !String(btn.flow_id || '').trim()) return 'Flow buttons need a Flow selected.'
  }
  return ''
})

// Build sample_values array from form inputs
// Sync sample_values when variables change — remove stale entries
watch(allVariables, (vars) => {
  form.value.sample_values = form.value.sample_values.filter((sv: any) =>
    vars.some(v => v.component === sv.component && v.index === sv.index)
  )
})

const statusVariant = computed(() => {
  switch (template.value?.status?.toUpperCase()) {
    case 'APPROVED': return 'default' as const
    case 'REJECTED': return 'destructive' as const
    case 'PAUSED': return 'outline' as const
    default: return 'secondary' as const
  }
})

const breadcrumbs = computed(() => [
  { label: t('nav.templates', 'Templates'), href: '/templates' },
  { label: isNew.value ? t('templates.newTemplate', 'New Template') : (template.value?.display_name || template.value?.name || '') },
])

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
    // An already-saved ZERO_TAP template was accepted when it was created.
    zero_tap_accepted: (template.value.buttons || []).some(
      (b: any) => b.type === 'OTP' && b.otp_type === 'ZERO_TAP'
    ),
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
    && !form.value.header_content && !pendingMediaFile.value) {
    const fallback = `Choose a sample ${form.value.header_type.toLowerCase()} — Meta requires one for approval.`
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
      toast.error(t('templates.headerTooManyVariables', 'Meta allows at most one variable in a TEXT header.'))
      return
    }
    const gap = firstSequenceGap.value
    if (gap) {
      toast.error(t('templates.variablesNotSequential', `Variables must be sequential from {{1}}. Expected {{${gap.expected}}}, found {{${gap.found}}}.`))
      return
    }
    if (missingSamples.value.length) {
      toast.error(t('templates.samplesRequired', `Add sample values for: ${missingSamples.value.join(', ')}`))
      return
    }
    const badButton = firstButtonError.value
    if (badButton) {
      toast.error(badButton)
      return
    }
  }
  if (isAuthentication.value && form.value.code_expiration_minutes && (form.value.code_expiration_minutes < 1 || form.value.code_expiration_minutes > 90)) {
    toast.error(t('templates.invalidExpiration', 'Code expiration must be between 1 and 90 minutes'))
    return
  }
  if (isAuthentication.value && authOtpType.value === 'ZERO_TAP' && !form.value.zero_tap_accepted) {
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
    if (pendingMediaFile.value) {
      const upload = await templatesService.uploadMedia(form.value.whatsapp_account, pendingMediaFile.value)
      form.value.header_content = (upload.data as any).data.handle
      pendingMediaFile.value = null
    }

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

async function loadFlows() {
  try {
    const response = await flowsService.list({ limit: 100 })
    const data = (response.data as any).data || response.data
    whatsappFlows.value = (data.flows || []).filter((f: any) => f.status === 'PUBLISHED')
  } catch {
    // non-critical
  }
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

    <!-- Account, name, category, header, body, variables, buttons and the live
         WhatsApp preview all live in TemplateEditor. -->
    <!-- Approved templates can still be edited; Meta only freezes their identity
         fields, which is what is-published locks. -->
    <div v-if="template?.status?.toUpperCase() === 'APPROVED'" class="flex items-start gap-2 rounded-md bg-blue-500/10 border border-blue-500/20 px-3 py-2 mb-4 text-xs text-blue-400 light:text-blue-600">
      <Info class="h-3.5 w-3.5 shrink-0 mt-0.5" />
      <span>{{ $t('templates.editLimitsInfo', 'Approved templates can be edited up to 10 times in 30 days (1 edit per 24 hours). Editing triggers a new review which may take up to 24 hours. Name, language, and category cannot be changed.') }}</span>
    </div>

    <TemplateEditor
      v-model="form"
      v-model:media-file="pendingMediaFile"
      :is-edit="!isNew"
      :is-published="!!template?.meta_template_id"
      :accounts="accounts"
      :flows="whatsappFlows"
      :disabled="!canWrite || !isEditable"
    />

    <!-- Activity Log -->
    <AuditLogPanel
      v-if="template && !isNew"
      :key="auditRefreshKey"
      resource-type="template"
      :resource-id="template.id"
    />

    <!-- Sidebar -->
    <template v-if="!isNew" #sidebar>
      <Card v-if="template">
        <CardHeader class="pb-3">
          <CardTitle class="text-sm font-medium">{{ $t('templates.status', 'Status') }}</CardTitle>
        </CardHeader>
        <CardContent class="space-y-2 text-sm">
          <div class="flex items-center justify-between">
            <span class="text-muted-foreground">{{ $t('templates.status', 'Status') }}</span>
            <Badge :variant="statusVariant">{{ template.status }}</Badge>
          </div>
          <div v-if="template.quality_rating" class="flex items-center justify-between">
            <span class="text-muted-foreground">{{ $t('templates.qualityRating', 'Quality Rating') }}</span>
            <Badge :class="getQualityBadgeClass(template.quality_rating)">
              {{ getQualityRatingLabel(template.quality_rating, t) }}
            </Badge>
          </div>
          <div v-if="template.meta_template_id" class="flex items-center justify-between">
            <span class="text-muted-foreground">Meta ID</span>
            <span class="font-mono text-xs">{{ template.meta_template_id }}</span>
          </div>
        </CardContent>
      </Card>

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
