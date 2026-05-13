<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { campaignsService, templatesService, api } from '@/services/api'
import { wsService } from '@/services/websocket'
import { toast } from 'vue-sonner'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import { useHeaderMedia } from '@/composables/useHeaderMedia'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDateTime } from '@/lib/utils'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import { ConfirmDialog } from '@/components/shared'
import HeaderMediaUpload from '@/components/shared/HeaderMediaUpload.vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Megaphone,
  Trash2,
  Save,
  CheckCircle,
  Play,
  Pause,
  Clock,
  AlertCircle,
  Users,
  Send,
  Eye,
  XCircle,
  RefreshCw,
  UserPlus,
  Upload,
  FileSpreadsheet,
  ChevronDown,
} from 'lucide-vue-next'

interface Campaign {
  id: string
  name: string
  whatsapp_account?: string
  template_id?: string
  template_name?: string
  header_media_id?: string
  header_media_filename?: string
  header_media_mime_type?: string
  status: string
  total_recipients: number
  sent_count: number
  delivered_count: number
  read_count: number
  failed_count: number
  scheduled_at?: string
  started_at?: string
  completed_at?: string
  created_by_name?: string
  updated_by_name?: string
  created_at: string
  updated_at: string
}

interface Account {
  id: string
  name: string
}

interface Template {
  id: string
  name: string
  display_name?: string
  status: string
  body_content?: string
  header_type?: string
  header_content?: string
}

interface Recipient {
  id: string
  phone_number: string
  recipient_name: string
  status: string
  sent_at?: string
  delivered_at?: string
  error_message?: string
}

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const campaignId = computed(() => route.params.id as string)
const isNew = computed(() => campaignId.value === 'new')
const campaign = ref<Campaign | null>(null)
const isLoading = ref(true)
const isNotFound = ref(false)
const isSaving = ref(false)
const hasChanges = ref(false)
const deleteDialogOpen = ref(false)

const accounts = ref<Account[]>([])
const templates = ref<Template[]>([])

const { showLeaveDialog, confirmLeave, cancelLeave } = useUnsavedChangesGuard(hasChanges)

const isDraft = computed(() => isNew.value || campaign.value?.status === 'draft')

const form = ref({
  name: '',
  whatsapp_account: '',
  template_id: '',
  scheduled_at: '',
})

const breadcrumbs = computed(() => [
  { label: t('nav.campaigns', 'Campaigns'), href: '/campaigns' },
  { label: isNew.value ? t('campaigns.newCampaign', 'New Campaign') : (campaign.value?.name || '') },
])

// --- Action dialog state ---
const startDialogOpen = ref(false)
const pauseDialogOpen = ref(false)
const cancelDialogOpen = ref(false)
const isStarting = ref(false)
const isPausing = ref(false)

const canStart = computed(() => {
  const s = campaign.value?.status
  return (s === 'draft' || s === 'scheduled' || s === 'paused') && (campaign.value?.total_recipients || 0) > 0
})
const canPause = computed(() => {
  const s = campaign.value?.status
  return s === 'running' || s === 'processing'
})
const canCancel = computed(() => {
  const s = campaign.value?.status
  return s === 'running' || s === 'paused' || s === 'processing' || s === 'queued'
})
const canRetryFailed = computed(() => {
  if (!campaign.value || campaign.value.failed_count <= 0) return false
  const s = campaign.value.status
  return s === 'completed' || s === 'paused' || s === 'failed'
})

// --- Recipients state ---
const recipients = ref<Recipient[]>([])
const isLoadingRecipients = ref(false)
const deletingRecipientId = ref<string | null>(null)
const showAddRecipientsDialog = ref(false)
const isAddingRecipients = ref(false)
const auditRefreshKey = ref(0)
const existingMediaUrl = ref<string | null>(null)
const showMediaUpload = ref(false)
const recipientsInput = ref('')
const addRecipientsTab = ref('manual')
const csvFile = ref<File | null>(null)

// --- Selected template for param extraction ---
const selectedTemplate = ref<Template | null>(null)

// --- Header media state ---
const selectedTemplateHeaderType = computed(() => selectedTemplate.value?.header_type)
const {
  file: mediaFile,
  previewUrl: mediaPreview,
  needsMedia: templateNeedsMedia,
  acceptTypes: mediaAcceptTypes,
  mediaLabel,
  handleFileChange: handleMediaFileChange,
  clear: clearMedia,
} = useHeaderMedia(selectedTemplateHeaderType)
const isUploadingMedia = ref(false)

// Template parameter helpers
function getTemplateParamNames(template: Template): string[] {
  if (!template.body_content) return []
  const matches = template.body_content.match(/\{\{([^}]+)\}\}/g) || []
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

const templateParamNames = computed(() => {
  if (!selectedTemplate.value) return []
  return getTemplateParamNames(selectedTemplate.value)
})

const recipientPlaceholder = computed(() => {
  const params = templateParamNames.value
  if (params.length === 0) {
    return `+1234567890, John Doe\n+0987654321, Jane Smith\n+1122334455`
  }
  const exampleValues = params.map((p, i) => {
    if (/^\d+$/.test(p)) return `value${i + 1}`
    if (p.toLowerCase().includes('name')) return 'John Doe'
    if (p.toLowerCase().includes('order')) return 'ORD-123'
    if (p.toLowerCase().includes('date')) return '2024-01-15'
    if (p.toLowerCase().includes('amount') || p.toLowerCase().includes('price')) return '99.99'
    return `${p}_value`
  })
  return `+1234567890, John Doe, ${exampleValues.join(', ')}\n+0987654321, Jane Smith, ${exampleValues.join(', ')}`
})

const manualEntryFormat = computed(() => {
  const params = templateParamNames.value
  if (params.length === 0) return 'phone_number, name (optional)'
  const paramLabels = params.map(p => /^\d+$/.test(p) ? `param${p}` : p)
  return `phone_number, name, ${paramLabels.join(', ')}`
})

// Status helpers
function getStatusIcon(status: string) {
  switch (status) {
    case 'completed':
      return CheckCircle
    case 'running':
    case 'processing':
    case 'queued':
      return Play
    case 'paused':
      return Pause
    case 'scheduled':
      return Clock
    case 'failed':
    case 'cancelled':
      return AlertCircle
    default:
      return Megaphone
  }
}

function getStatusClass(status: string): string {
  switch (status) {
    case 'completed':
      return 'border-green-600 text-green-600'
    case 'running':
    case 'processing':
    case 'queued':
      return 'border-blue-600 text-blue-600'
    case 'failed':
    case 'cancelled':
      return 'border-destructive text-destructive'
    default:
      return ''
  }
}

function getRecipientStatusClass(status: string): string {
  switch (status) {
    case 'sent':
    case 'delivered':
      return 'border-green-600 text-green-600'
    case 'failed':
      return 'border-destructive text-destructive'
    default:
      return ''
  }
}

async function loadAccounts() {
  try {
    const response = await api.get('/accounts')
    accounts.value = (response.data as any).data?.accounts || []
  } catch {
    accounts.value = []
  }
}

async function loadTemplates() {
  if (!form.value.whatsapp_account) {
    templates.value = []
    return
  }
  try {
    const response = await api.get('/templates', {
      params: { whatsapp_account: form.value.whatsapp_account },
    })
    templates.value = (response.data as any).data?.templates || []
  } catch {
    templates.value = []
  }
}

async function loadCampaign() {
  isLoading.value = true
  isNotFound.value = false
  try {
    const response = await campaignsService.get(campaignId.value)
    const data = (response.data as any).data || response.data
    campaign.value = data
    syncForm()
    nextTick(() => { hasChanges.value = false })
    auditRefreshKey.value++
  } catch {
    isNotFound.value = true
  } finally {
    isLoading.value = false
  }
}

function syncForm() {
  if (!campaign.value) return
  form.value = {
    name: campaign.value.name || '',
    whatsapp_account: campaign.value.whatsapp_account || '',
    template_id: campaign.value.template_id || '',
    scheduled_at: campaign.value.scheduled_at ? campaign.value.scheduled_at.slice(0, 16) : '',
  }
}

// Track form changes
watch(form, () => {
  hasChanges.value = true
}, { deep: true })

// Reload templates when account changes
watch(() => form.value.whatsapp_account, (newVal, oldVal) => {
  if (newVal !== oldVal) {
    // Clear template selection if account changed
    if (oldVal) {
      form.value.template_id = ''
    }
    loadTemplates()
  }
})

// Fetch full template details when template_id changes (for param names & header type)
watch(() => form.value.template_id, async (newId) => {
  if (newId) {
    try {
      const response = await templatesService.get(newId)
      selectedTemplate.value = (response.data as any).data || response.data
    } catch {
      selectedTemplate.value = null
    }
  } else {
    selectedTemplate.value = null
  }
})

async function save() {
  if (!form.value.name.trim()) {
    toast.error(t('campaigns.nameRequired', 'Campaign name is required'))
    return
  }
  isSaving.value = true
  try {
    const payload: Record<string, any> = {
      name: form.value.name,
      whatsapp_account: form.value.whatsapp_account || undefined,
      template_id: form.value.template_id || undefined,
      scheduled_at: form.value.scheduled_at || undefined,
    }
    if (isNew.value) {
      const response = await campaignsService.create(payload)
      const created = (response.data as any).data || response.data
      // Upload media if selected
      if (mediaFile.value && created?.id) {
        try {
          await campaignsService.uploadMedia(created.id, mediaFile.value)
        } catch {
          toast.error(t('campaigns.mediaUploadFailed', 'Failed to upload media'))
        }
      }
      hasChanges.value = false
      toast.success(t('campaigns.created', 'Campaign created'))
      router.replace(`/campaigns/${created.id}`)
    } else {
      await campaignsService.update(campaign.value!.id, payload)
      // Upload media if selected
      if (mediaFile.value) {
        try {
          isUploadingMedia.value = true
          await campaignsService.uploadMedia(campaign.value!.id, mediaFile.value)
          clearMedia()
        } catch {
          toast.error(t('campaigns.mediaUploadFailed', 'Failed to upload media'))
        } finally {
          isUploadingMedia.value = false
        }
      }
      await loadCampaign()
      hasChanges.value = false
      toast.success(t('campaigns.updated', 'Campaign updated'))
    }
  } catch {
    toast.error(
      isNew.value
        ? t('campaigns.createFailed', 'Failed to create campaign')
        : t('campaigns.updateFailed', 'Failed to update campaign'),
    )
  } finally {
    isSaving.value = false
  }
}

async function deleteCampaign() {
  if (!campaign.value) return
  try {
    await campaignsService.delete(campaign.value.id)
    toast.success(t('campaigns.deleted', 'Campaign deleted'))
    router.push('/campaigns')
  } catch {
    toast.error(t('campaigns.deleteFailed', 'Failed to delete campaign'))
  }
  deleteDialogOpen.value = false
}

// --- Campaign action handlers ---
async function confirmStartCampaign() {
  if (!campaign.value) return
  isStarting.value = true
  try {
    await campaignsService.start(campaign.value.id)
    toast.success(t('campaigns.campaignStarted', 'Campaign started'))
    startDialogOpen.value = false
    await loadCampaign()
  } catch (err: unknown) {
    toast.error(getErrorMessage(err, t('campaigns.startFailed', 'Failed to start campaign')))
  } finally {
    isStarting.value = false
  }
}

async function confirmPauseCampaign() {
  if (!campaign.value) return
  isPausing.value = true
  try {
    await campaignsService.pause(campaign.value.id)
    toast.success(t('campaigns.campaignPaused', 'Campaign paused'))
    pauseDialogOpen.value = false
    await loadCampaign()
  } catch (err: unknown) {
    toast.error(getErrorMessage(err, t('campaigns.pauseFailed', 'Failed to pause campaign')))
  } finally {
    isPausing.value = false
  }
}

async function confirmCancelCampaign() {
  if (!campaign.value) return
  try {
    await campaignsService.cancel(campaign.value.id)
    toast.success(t('campaigns.campaignCancelled', 'Campaign cancelled'))
    cancelDialogOpen.value = false
    await loadCampaign()
  } catch (err: unknown) {
    toast.error(getErrorMessage(err, t('campaigns.cancelFailed', 'Failed to cancel campaign')))
  }
}

async function retryFailed() {
  if (!campaign.value) return
  try {
    const response = await campaignsService.retryFailed(campaign.value.id)
    const result = (response.data as any).data
    toast.success(t('campaigns.retryingFailed', { count: result?.retry_count || 0 }, `Retrying ${result?.retry_count || 0} failed messages`))
    await loadCampaign()
    await loadRecipients()
  } catch (err: unknown) {
    toast.error(getErrorMessage(err, t('campaigns.retryFailedError', 'Failed to retry')))
  }
}

// --- Recipients ---
async function loadExistingMedia() {
  if (!campaign.value?.id) return
  try {
    const response = await campaignsService.getMedia(campaign.value.id)
    const blob = new Blob([response.data], { type: (response.headers as any)['content-type'] })
    existingMediaUrl.value = URL.createObjectURL(blob)
  } catch {
    existingMediaUrl.value = null
  }
}

async function loadRecipients() {
  if (isNew.value || !campaign.value) return
  isLoadingRecipients.value = true
  try {
    const response = await campaignsService.getRecipients(campaign.value.id)
    recipients.value = (response.data as any).data?.recipients || []
  } catch {
    recipients.value = []
  } finally {
    isLoadingRecipients.value = false
  }
}

async function deleteRecipient(recipientId: string) {
  if (!campaign.value) return
  deletingRecipientId.value = recipientId
  try {
    await campaignsService.deleteRecipient(campaign.value.id, recipientId)
    recipients.value = recipients.value.filter(r => r.id !== recipientId)
    toast.success(t('common.deletedSuccess', { resource: 'Recipient' }, 'Recipient deleted'))
    await loadCampaign()
  } catch (err: unknown) {
    toast.error(getErrorMessage(err, t('common.failedDelete', { resource: 'recipient' }, 'Failed to delete recipient')))
  } finally {
    deletingRecipientId.value = null
  }
}

async function openAddRecipientsDialog() {
  recipientsInput.value = ''
  csvFile.value = null
  addRecipientsTab.value = 'manual'

  // Fetch template details if needed
  if (campaign.value?.template_id && !selectedTemplate.value) {
    try {
      const response = await templatesService.get(campaign.value.template_id)
      selectedTemplate.value = (response.data as any).data || response.data
    } catch {
      selectedTemplate.value = null
    }
  }

  showAddRecipientsDialog.value = true
}

const manualInputValidation = computed(() => {
  const params = templateParamNames.value
  const lines = recipientsInput.value.trim().split('\n').filter((line: string) => line.trim())

  if (lines.length === 0) {
    return { isValid: false, totalLines: 0, validLines: 0, invalidLines: [] as { lineNumber: number; reason: string }[] }
  }

  const invalidLines: { lineNumber: number; reason: string }[] = []

  for (let i = 0; i < lines.length; i++) {
    const parts = lines[i].split(',').map((p: string) => p.trim())
    const phone = parts[0]?.replace(/[^\d+]/g, '')

    if (!phone || !phone.match(/^\+?\d{10,15}$/)) {
      invalidLines.push({ lineNumber: i + 1, reason: 'Invalid phone number' })
      continue
    }

    // Params start from 3rd field (after phone + name)
    const providedParams = parts.slice(2).filter((p: string) => p.length > 0).length
    if (params.length > 0 && providedParams < params.length) {
      invalidLines.push({
        lineNumber: i + 1,
        reason: `Missing template parameters (need ${params.length}, got ${providedParams})`
      })
    }
  }

  return {
    isValid: invalidLines.length === 0 && lines.length > 0,
    totalLines: lines.length,
    validLines: lines.length - invalidLines.length,
    invalidLines
  }
})

async function addRecipients() {
  if (!campaign.value) return

  if (!manualInputValidation.value.isValid) {
    toast.error('Please fix validation errors before adding')
    return
  }

  const lines = recipientsInput.value.trim().split('\n').filter((line: string) => line.trim())
  if (lines.length === 0) {
    toast.error(t('campaigns.enterPhoneNumber', 'Please enter at least one phone number'))
    return
  }

  // Format: phone_number, name, param1, param2, ...
  const paramNames = templateParamNames.value
  const recipientsList = lines.map(line => {
    const parts = line.split(',').map(p => p.trim())
    const recipient: { phone_number: string; recipient_name?: string; template_params?: Record<string, any> } = {
      phone_number: parts[0].replace(/[^\d+]/g, ''),
    }
    // Second field is always recipient name
    if (parts[1]?.trim()) {
      recipient.recipient_name = parts[1].trim()
    }
    // Template params start from third field
    const params: Record<string, any> = {}
    for (let i = 0; i < paramNames.length; i++) {
      const val = parts[i + 2] // offset by 2 (phone + name)
      if (val && val.length > 0) {
        params[paramNames[i]] = val
      }
    }
    if (Object.keys(params).length > 0) {
      recipient.template_params = params
    }
    return recipient
  })

  isAddingRecipients.value = true
  try {
    const response = await campaignsService.addRecipients(campaign.value.id, recipientsList)
    const result = (response.data as any).data
    toast.success(t('campaigns.addedRecipients', { count: result?.added_count || recipientsList.length }, `Added ${result?.added_count || recipientsList.length} recipients`))
    showAddRecipientsDialog.value = false
    recipientsInput.value = ''
    await loadCampaign()
    await loadRecipients()
  } catch (err: unknown) {
    toast.error(getErrorMessage(err, t('campaigns.addRecipientsFailed', 'Failed to add recipients')))
  } finally {
    isAddingRecipients.value = false
  }
}

function handleCSVFileSelect(event: Event) {
  const input = event.target as HTMLInputElement
  if (input.files && input.files[0]) {
    csvFile.value = input.files[0]
  }
}

async function addRecipientsFromCSV() {
  if (!campaign.value || !csvFile.value) return

  isAddingRecipients.value = true
  try {
    const text = await csvFile.value.text()
    const lines = text.split('\n').filter(line => line.trim())
    if (lines.length < 2) {
      toast.error(t('campaigns.csvEmpty', 'CSV file is empty or has no data rows'))
      isAddingRecipients.value = false
      return
    }

    const headers = lines[0].split(',').map(h => h.toLowerCase().trim())
    const phoneIndex = headers.findIndex(h =>
      h === 'phone' || h === 'phone_number' || h === 'phonenumber' || h === 'mobile' || h === 'number'
    )

    if (phoneIndex === -1) {
      toast.error(t('campaigns.missingPhoneColumn', 'CSV must have a phone_number column'))
      isAddingRecipients.value = false
      return
    }

    // Detect name column
    const nameIndex = headers.findIndex(h =>
      h === 'name' || h === 'recipient_name' || h === 'recipientname' || h === 'customer_name'
    )

    const paramNames = templateParamNames.value
    const recipientsList: { phone_number: string; recipient_name?: string; template_params?: Record<string, any> }[] = []

    for (let i = 1; i < lines.length; i++) {
      const values = lines[i].split(',').map(v => v.trim())
      const phone = values[phoneIndex]?.replace(/[^\d+]/g, '')
      if (!phone) continue

      const recipient: { phone_number: string; recipient_name?: string; template_params?: Record<string, any> } = {
        phone_number: phone,
      }

      // Extract name if column exists
      if (nameIndex !== -1 && values[nameIndex]?.trim()) {
        recipient.recipient_name = values[nameIndex].trim()
      }

      // Map remaining columns to template params by name match or position
      if (paramNames.length > 0) {
        const params: Record<string, any> = {}
        const usedIndices = new Set<number>([phoneIndex])
        if (nameIndex !== -1) usedIndices.add(nameIndex)

        // Try name matching first
        for (const paramName of paramNames) {
          const colIdx = headers.findIndex((h, idx) =>
            !usedIndices.has(idx) && h === paramName.toLowerCase()
          )
          if (colIdx !== -1) {
            params[paramName] = values[colIdx]?.trim() || ''
            usedIndices.add(colIdx)
          }
        }

        // Positional fallback for unmapped params
        const unmapped = paramNames.filter(p => !(p in params))
        const remainingCols = headers.map((_, idx) => idx).filter(idx => !usedIndices.has(idx))
        for (let j = 0; j < unmapped.length && j < remainingCols.length; j++) {
          params[unmapped[j]] = values[remainingCols[j]]?.trim() || ''
        }

        if (Object.keys(params).length > 0) {
          recipient.template_params = params
        }
      }

      recipientsList.push(recipient)
    }

    if (recipientsList.length === 0) {
      toast.error(t('campaigns.noValidRowsToImport', 'No valid rows found in CSV'))
      isAddingRecipients.value = false
      return
    }

    const response = await campaignsService.addRecipients(campaign.value.id, recipientsList)
    const result = (response.data as any).data
    toast.success(t('campaigns.addedFromCsv', { count: result?.added_count || recipientsList.length }, `Imported ${result?.added_count || recipientsList.length} recipients from CSV`))
    showAddRecipientsDialog.value = false
    csvFile.value = null
    await loadCampaign()
    await loadRecipients()
  } catch (err: unknown) {
    toast.error(getErrorMessage(err, t('campaigns.addRecipientsFailed', 'Failed to add recipients')))
  } finally {
    isAddingRecipients.value = false
  }
}

onMounted(async () => {
  await loadAccounts()
  if (isNew.value) {
    isLoading.value = false
    hasChanges.value = false
  } else {
    await loadCampaign()
    // Load templates for the selected account after campaign loads
    if (form.value.whatsapp_account) {
      await loadTemplates()
    }
    // Load template details for header media detection
    if (form.value.template_id) {
      try {
        const response = await templatesService.get(form.value.template_id)
        selectedTemplate.value = (response.data as any).data || response.data
      } catch {
        selectedTemplate.value = null
      }
    }
    await loadRecipients()
    // Load media preview if exists
    if (campaign.value?.header_media_id) {
      loadExistingMedia()
    }
  }

  // Subscribe to real-time campaign stats updates
  if (!isNew.value) {
    unsubscribeStats = wsService.onCampaignStatsUpdate((payload: any) => {
      if (campaign.value && payload.campaign_id === campaign.value.id) {
        campaign.value.sent_count = payload.sent_count
        campaign.value.delivered_count = payload.delivered_count
        campaign.value.read_count = payload.read_count
        campaign.value.failed_count = payload.failed_count
        if (payload.status && payload.status !== campaign.value.status) {
          campaign.value.status = payload.status
          auditRefreshKey.value++
        }
      }
    })
  }
})

let unsubscribeStats: (() => void) | null = null

onUnmounted(() => {
  if (unsubscribeStats) {
    unsubscribeStats()
  }
})
</script>

<template>
  <div class="h-full">
  <DetailPageLayout
    :title="isNew ? $t('campaigns.newCampaign', 'New Campaign') : (campaign?.name || '')"
    :icon="Megaphone"
    icon-gradient="bg-gradient-to-br from-pink-500 to-rose-600 shadow-pink-500/20"
    back-link="/campaigns"
    :breadcrumbs="breadcrumbs"
    :is-loading="isLoading"
    :is-not-found="isNotFound"
    :not-found-title="$t('campaigns.notFound', 'Campaign not found')"
  >
    <template #actions>
      <div class="flex items-center gap-2">
        <!-- Status badge for existing campaigns -->
        <Badge
          v-if="!isNew && campaign"
          variant="outline"
          :class="[getStatusClass(campaign.status), 'text-xs']"
        >
          <component :is="getStatusIcon(campaign.status)" class="h-3 w-3 mr-1" />
          {{ campaign.status }}
        </Badge>

        <!-- Start/Resume -->
        <Button
          v-if="!isNew && campaign && canStart"
          variant="outline"
          size="sm"
          @click="startDialogOpen = true"
        >
          <Play class="h-4 w-4 mr-1" />
          {{ campaign.status === 'paused' ? $t('campaigns.resume', 'Resume') : $t('campaigns.start', 'Start') }}
        </Button>

        <!-- Pause -->
        <Button
          v-if="!isNew && campaign && canPause"
          variant="outline"
          size="sm"
          @click="pauseDialogOpen = true"
        >
          <Pause class="h-4 w-4 mr-1" />
          {{ $t('campaigns.pause', 'Pause') }}
        </Button>

        <!-- Cancel -->
        <Button
          v-if="!isNew && campaign && canCancel"
          variant="outline"
          size="sm"
          @click="cancelDialogOpen = true"
        >
          <XCircle class="h-4 w-4 mr-1" />
          {{ $t('campaigns.cancel', 'Cancel') }}
        </Button>

        <!-- Retry Failed -->
        <Button
          v-if="!isNew && campaign && canRetryFailed"
          variant="outline"
          size="sm"
          @click="retryFailed"
        >
          <RefreshCw class="h-4 w-4 mr-1" />
          {{ $t('campaigns.retryFailed', 'Retry Failed') }}
        </Button>

        <Button
          v-if="isDraft && (hasChanges || isNew)"
          size="sm"
          @click="save"
          :disabled="isSaving"
        >
          <Save class="h-4 w-4 mr-1" />
          {{ isSaving ? $t('common.saving', 'Saving...') : isNew ? $t('common.create') : $t('common.save') }}
        </Button>
        <Button
          v-if="isDraft && !isNew"
          variant="destructive"
          size="sm"
          @click="deleteDialogOpen = true"
        >
          <Trash2 class="h-4 w-4 mr-1" /> {{ $t('common.delete') }}
        </Button>
      </div>
    </template>

    <!-- Details Card -->
    <Card>
      <CardHeader class="pb-3">
        <CardTitle class="text-sm font-medium">{{ $t('campaigns.details', 'Details') }}</CardTitle>
      </CardHeader>
      <CardContent class="space-y-4">
        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('campaigns.name', 'Name') }} *</Label>
          <Input v-model="form.name" :disabled="!isDraft" :placeholder="$t('campaigns.namePlaceholder', 'Enter campaign name')" />
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('campaigns.whatsappAccount', 'WhatsApp Account') }}</Label>
          <Select v-model="form.whatsapp_account" :disabled="!isDraft">
            <SelectTrigger>
              <SelectValue :placeholder="$t('campaigns.selectAccount', 'Select account')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="account in accounts" :key="account.name" :value="account.name">
                {{ account.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('campaigns.template', 'Template') }}</Label>
          <Select v-model="form.template_id" :disabled="!isDraft || !form.whatsapp_account">
            <SelectTrigger>
              <SelectValue :placeholder="form.whatsapp_account ? $t('campaigns.selectTemplate', 'Select template') : $t('campaigns.selectAccountFirst', 'Select an account first')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="tmpl in templates.filter(t => t.id)" :key="tmpl.id" :value="tmpl.id">
                {{ tmpl.display_name || tmpl.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>

        <!-- Media Upload Section -->
        <div v-if="templateNeedsMedia" class="space-y-1.5">
          <Label class="text-xs">{{ $t('campaigns.headerMedia', 'Header Media') }}</Label>
          <!-- Show existing media with preview -->
          <div v-if="campaign?.header_media_filename && !mediaFile" class="rounded-lg border overflow-hidden">
            <!-- Image preview -->
            <img
              v-if="campaign.header_media_mime_type?.startsWith('image/') && existingMediaUrl"
              :src="existingMediaUrl"
              :alt="campaign.header_media_filename"
              class="w-full max-h-48 object-contain bg-muted"
            />
            <!-- Video preview -->
            <video
              v-else-if="campaign.header_media_mime_type?.startsWith('video/') && existingMediaUrl"
              :src="existingMediaUrl"
              controls
              class="w-full max-h-48"
            />
            <!-- File info bar -->
            <div class="flex items-center gap-2 px-3 py-2 bg-muted/50 text-sm">
              <Upload class="h-4 w-4 text-muted-foreground shrink-0" />
              <span class="truncate flex-1">{{ campaign.header_media_filename }}</span>
              <Button v-if="isDraft" variant="ghost" size="sm" class="h-6 text-xs" @click="showMediaUpload = true">
                {{ $t('campaigns.replace', 'Replace') }}
              </Button>
            </div>
          </div>
          <!-- Upload component for drafts without existing media or when replacing -->
          <HeaderMediaUpload
            v-if="isDraft && (!campaign?.header_media_filename || mediaFile || showMediaUpload)"
            :file="mediaFile"
            :preview-url="mediaPreview"
            :accept-types="mediaAcceptTypes"
            :media-label="mediaLabel"
            @change="handleMediaFileChange"
            @clear="clearMedia"
          />
        </div>

        <!-- Schedule field hidden until scheduling functionality is implemented -->
        <!-- <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('campaigns.scheduledAt', 'Schedule') }}</Label>
          <Input v-model="form.scheduled_at" type="datetime-local" :disabled="!isDraft" />
        </div> -->
        <div v-if="!isNew && campaign" class="space-y-1.5">
          <Label class="text-xs">{{ $t('campaigns.status', 'Status') }}</Label>
          <div>
            <Badge variant="outline" :class="[getStatusClass(campaign.status), 'text-xs']">
              <component :is="getStatusIcon(campaign.status)" class="h-3 w-3 mr-1" />
              {{ campaign.status }}
            </Badge>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Stats Card (existing campaigns only) -->
    <Card v-if="!isNew && campaign">
      <CardHeader class="pb-3">
        <CardTitle class="text-sm font-medium">{{ $t('campaigns.statistics', 'Statistics') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
          <div class="flex flex-col items-center gap-1 rounded-lg border p-3">
            <Users class="h-4 w-4 text-muted-foreground" />
            <span class="text-lg font-semibold">{{ campaign.total_recipients }}</span>
            <span class="text-[10px] text-muted-foreground uppercase tracking-wide">{{ $t('campaigns.totalRecipients', 'Recipients') }}</span>
          </div>
          <div class="flex flex-col items-center gap-1 rounded-lg border p-3">
            <Send class="h-4 w-4 text-blue-500" />
            <span class="text-lg font-semibold">{{ campaign.sent_count }}</span>
            <span class="text-[10px] text-muted-foreground uppercase tracking-wide">{{ $t('campaigns.sent', 'Sent') }}</span>
          </div>
          <div class="flex flex-col items-center gap-1 rounded-lg border p-3">
            <CheckCircle class="h-4 w-4 text-green-500" />
            <span class="text-lg font-semibold">{{ campaign.delivered_count }}</span>
            <span class="text-[10px] text-muted-foreground uppercase tracking-wide">{{ $t('campaigns.delivered', 'Delivered') }}</span>
          </div>
          <div class="flex flex-col items-center gap-1 rounded-lg border p-3">
            <Eye class="h-4 w-4 text-purple-500" />
            <span class="text-lg font-semibold">{{ campaign.read_count }}</span>
            <span class="text-[10px] text-muted-foreground uppercase tracking-wide">{{ $t('campaigns.read', 'Read') }}</span>
          </div>
          <div class="flex flex-col items-center gap-1 rounded-lg border p-3">
            <XCircle class="h-4 w-4 text-destructive" />
            <span class="text-lg font-semibold">{{ campaign.failed_count }}</span>
            <span class="text-[10px] text-muted-foreground uppercase tracking-wide">{{ $t('campaigns.failed', 'Failed') }}</span>
          </div>
        </div>

        <!-- Progress Bar -->
        <div v-if="campaign.total_recipients > 0" class="mt-4 space-y-2">
          <div class="flex items-center justify-between text-xs text-muted-foreground">
            <span>{{ $t('campaigns.progress', 'Progress') }}</span>
            <span>{{ Math.round(((campaign.sent_count + campaign.failed_count) / campaign.total_recipients) * 100) }}%</span>
          </div>
          <div class="h-2.5 w-full bg-muted rounded-full overflow-hidden flex">
            <div
              class="bg-green-500 h-full transition-all duration-500"
              :style="{ width: `${(campaign.delivered_count / campaign.total_recipients) * 100}%` }"
            />
            <div
              class="bg-blue-500 h-full transition-all duration-500"
              :style="{ width: `${((campaign.sent_count - campaign.delivered_count) / campaign.total_recipients) * 100}%` }"
            />
            <div
              class="bg-destructive h-full transition-all duration-500"
              :style="{ width: `${(campaign.failed_count / campaign.total_recipients) * 100}%` }"
            />
          </div>
          <div class="flex items-center gap-4 text-[10px] text-muted-foreground">
            <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full bg-green-500" /> {{ $t('campaigns.delivered', 'Delivered') }}</span>
            <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full bg-blue-500" /> {{ $t('campaigns.sent', 'Sent') }}</span>
            <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full bg-destructive" /> {{ $t('campaigns.failed', 'Failed') }}</span>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Recipients Card (collapsible) -->
    <Card v-if="!isNew && campaign">
      <Collapsible :default-open="recipients.length > 0 && recipients.length <= 20">
        <CardHeader class="pb-3 flex flex-row items-center justify-between">
          <CollapsibleTrigger class="flex items-center gap-2 cursor-pointer hover:opacity-80">
            <ChevronDown class="h-4 w-4 text-muted-foreground transition-transform [[data-state=closed]_&]:rotate-[-90deg]" />
            <CardTitle class="text-sm font-medium">
              {{ $t('campaigns.recipients', 'Recipients') }} ({{ recipients.length }})
            </CardTitle>
          </CollapsibleTrigger>
          <Button v-if="isDraft" variant="outline" size="sm" @click="openAddRecipientsDialog">
            <UserPlus class="h-4 w-4 mr-1" />
            {{ $t('campaigns.addRecipients', 'Add') }}
          </Button>
        </CardHeader>
        <CollapsibleContent>
        <CardContent>
        <div v-if="isLoadingRecipients" class="text-center py-8 text-sm text-muted-foreground">
          {{ $t('common.loading', 'Loading...') }}
        </div>
        <div v-else-if="recipients.length === 0" class="text-center py-8">
          <Users class="h-8 w-8 mx-auto text-muted-foreground mb-2" />
          <p class="text-sm text-muted-foreground">{{ $t('campaigns.noRecipients', 'No recipients yet') }}</p>
          <Button v-if="isDraft" variant="outline" size="sm" class="mt-3" @click="openAddRecipientsDialog">
            <UserPlus class="h-4 w-4 mr-1" />
            {{ $t('campaigns.addRecipients', 'Add Recipients') }}
          </Button>
        </div>
        <div v-else class="overflow-auto max-h-96">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{{ $t('campaigns.phoneNumber', 'Phone Number') }}</TableHead>
                <TableHead>{{ $t('campaigns.recipientName', 'Name') }}</TableHead>
                <TableHead>{{ $t('campaigns.status', 'Status') }}</TableHead>
                <TableHead>{{ $t('campaigns.sentAt', 'Sent At') }}</TableHead>
                <TableHead v-if="isDraft" class="w-10" />
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="recipient in recipients" :key="recipient.id">
                <TableCell class="font-mono text-xs">{{ recipient.phone_number }}</TableCell>
                <TableCell class="text-sm">{{ recipient.recipient_name || '-' }}</TableCell>
                <TableCell>
                  <Badge variant="outline" :class="[getRecipientStatusClass(recipient.status), 'text-xs']">
                    {{ recipient.status }}
                  </Badge>
                </TableCell>
                <TableCell class="text-xs text-muted-foreground">
                  {{ recipient.sent_at ? formatDateTime(recipient.sent_at) : '-' }}
                </TableCell>
                <TableCell v-if="isDraft">
                  <Button
                    variant="ghost"
                    size="icon"
                    class="h-7 w-7"
                    :disabled="deletingRecipientId === recipient.id"
                    @click="deleteRecipient(recipient.id)"
                  >
                    <Trash2 class="h-3.5 w-3.5 text-destructive" />
                  </Button>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </CardContent>
        </CollapsibleContent>
      </Collapsible>
    </Card>

    <!-- Audit Log -->
    <AuditLogPanel
      v-if="campaign && !isNew"
      :key="auditRefreshKey"
      resource-type="campaign"
      :resource-id="campaign.id"
    />

    <!-- Sidebar -->
    <template v-if="!isNew" #sidebar>
      <MetadataPanel
        :created-at="campaign?.created_at"
        :updated-at="campaign?.updated_at"
        :created-by-name="campaign?.created_by_name"
        :updated-by-name="campaign?.updated_by_name"
      />
    </template>
  </DetailPageLayout>

  <!-- Delete Confirmation -->
  <AlertDialog v-model:open="deleteDialogOpen">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ $t('campaigns.deleteCampaign', 'Delete Campaign') }}</AlertDialogTitle>
        <AlertDialogDescription>
          {{ $t('campaigns.deleteConfirm', 'Are you sure? This action cannot be undone.') }}
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>{{ $t('common.cancel') }}</AlertDialogCancel>
        <AlertDialogAction @click="deleteCampaign">{{ $t('common.delete') }}</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>

  <!-- Start/Resume Confirmation -->
  <ConfirmDialog
    v-model:open="startDialogOpen"
    :title="$t('campaigns.startCampaign', 'Start Campaign')"
    :description="$t('campaigns.startConfirm', 'This will begin sending messages to all recipients. Continue?')"
    :confirm-label="$t('campaigns.start', 'Start')"
    :is-submitting="isStarting"
    @confirm="confirmStartCampaign"
  />

  <!-- Pause Confirmation -->
  <ConfirmDialog
    v-model:open="pauseDialogOpen"
    :title="$t('campaigns.pauseCampaign', 'Pause Campaign')"
    :description="$t('campaigns.pauseConfirm', 'This will pause sending messages. You can resume later.')"
    :confirm-label="$t('campaigns.pause', 'Pause')"
    :is-submitting="isPausing"
    @confirm="confirmPauseCampaign"
  />

  <!-- Cancel Confirmation -->
  <ConfirmDialog
    v-model:open="cancelDialogOpen"
    :title="$t('campaigns.cancelCampaign', 'Cancel Campaign')"
    :description="$t('campaigns.cancelConfirm', 'This will permanently stop the campaign. Messages already sent cannot be recalled.')"
    :confirm-label="$t('campaigns.cancel', 'Cancel Campaign')"
    variant="destructive"
    @confirm="confirmCancelCampaign"
  />

  <!-- Add Recipients Dialog -->
  <Dialog v-model:open="showAddRecipientsDialog">
    <DialogContent class="sm:max-w-[550px]">
      <DialogHeader>
        <DialogTitle>{{ $t('campaigns.addRecipients', 'Add Recipients') }}</DialogTitle>
        <DialogDescription>
          {{ $t('campaigns.addRecipientsDescription', 'Add phone numbers manually or upload a CSV file.') }}
        </DialogDescription>
      </DialogHeader>

      <Tabs v-model="addRecipientsTab">
        <TabsList class="w-full">
          <TabsTrigger value="manual" class="flex-1">
            <UserPlus class="h-4 w-4 mr-1" />
            {{ $t('campaigns.manualEntry', 'Manual Entry') }}
          </TabsTrigger>
          <TabsTrigger value="csv" class="flex-1">
            <FileSpreadsheet class="h-4 w-4 mr-1" />
            {{ $t('campaigns.csvUpload', 'CSV Upload') }}
          </TabsTrigger>
        </TabsList>

        <!-- Manual Entry Tab -->
        <TabsContent value="manual" class="space-y-3 mt-3">
          <div class="space-y-1.5">
            <Label class="text-xs text-muted-foreground">
              {{ $t('campaigns.formatHint', 'Format') }}: <code class="text-[10px] bg-muted px-1 rounded">{{ manualEntryFormat }}</code>
            </Label>
            <p class="text-[10px] text-muted-foreground mb-1">
              {{ $t('campaigns.recipientFormatNote', 'One recipient per line. Name is optional. Template parameters must match the selected template.') }}
            </p>
            <Textarea
              v-model="recipientsInput"
              :placeholder="recipientPlaceholder"
              :rows="8"
              class="font-mono text-sm"
            />
          </div>
          <!-- Validation Feedback -->
          <div v-if="recipientsInput.trim()">
            <p v-if="manualInputValidation.isValid" class="text-xs text-green-600">
              {{ manualInputValidation.validLines }} valid recipient{{ manualInputValidation.validLines !== 1 ? 's' : '' }}
            </p>
            <div v-else-if="manualInputValidation.invalidLines.length > 0" class="text-xs space-y-1">
              <p class="text-destructive font-medium">
                {{ manualInputValidation.invalidLines.length }} of {{ manualInputValidation.totalLines }} lines have errors:
              </p>
              <ul class="list-disc list-inside text-destructive space-y-0.5">
                <li v-for="err in manualInputValidation.invalidLines.slice(0, 5)" :key="err.lineNumber">
                  Line {{ err.lineNumber }}: {{ err.reason }}
                </li>
                <li v-if="manualInputValidation.invalidLines.length > 5" class="text-muted-foreground">
                  ...and {{ manualInputValidation.invalidLines.length - 5 }} more
                </li>
              </ul>
            </div>
          </div>
          <DialogFooter>
            <Button
              @click="addRecipients"
              :disabled="isAddingRecipients || !manualInputValidation.isValid"
            >
              <UserPlus class="h-4 w-4 mr-1" />
              {{ isAddingRecipients ? $t('common.adding', 'Adding...') : $t('campaigns.addRecipients', 'Add Recipients') }}
            </Button>
          </DialogFooter>
        </TabsContent>

        <!-- CSV Upload Tab -->
        <TabsContent value="csv" class="space-y-3 mt-3">
          <div class="space-y-1.5">
            <Label class="text-xs text-muted-foreground">
              {{ $t('campaigns.csvFormatHint', 'CSV must include a phone_number (or phone, mobile, number) column. Optionally include a name (or recipient_name) column, followed by template parameter columns.') }}
            </Label>
            <div
              class="border-2 border-dashed rounded-lg p-6 text-center cursor-pointer hover:border-primary/50 transition-colors"
              @click="($refs.csvInput as HTMLInputElement)?.click()"
            >
              <Upload class="h-6 w-6 mx-auto text-muted-foreground mb-1" />
              <p class="text-sm text-muted-foreground">
                {{ csvFile ? csvFile.name : $t('campaigns.clickToUploadCSV', 'Click to select CSV file') }}
              </p>
            </div>
            <input ref="csvInput" type="file" accept=".csv" class="hidden" @change="handleCSVFileSelect" />
          </div>
          <DialogFooter>
            <Button
              @click="addRecipientsFromCSV"
              :disabled="isAddingRecipients || !csvFile"
            >
              <FileSpreadsheet class="h-4 w-4 mr-1" />
              {{ isAddingRecipients ? $t('common.importing', 'Importing...') : $t('campaigns.importCSV', 'Import CSV') }}
            </Button>
          </DialogFooter>
        </TabsContent>
      </Tabs>
    </DialogContent>
  </Dialog>

  <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
