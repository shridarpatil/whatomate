<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { campaignsService, templatesService, api } from '@/services/api'
import { wsService } from '@/services/websocket'
import { toast } from '@/lib/toast'
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
import Modal from '@/components/oat/Modal.vue'
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
  Download,
} from '@/lib/icons'

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
const csvInput = ref<HTMLInputElement | null>(null)

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

// Template parameter helpers. Header and body are tracked separately so that
// a positional header {{1}} and body {{1}} (which Meta treats as distinct,
// component-scoped variables) don't collapse to a single column.
function extractParamNames(text?: string): string[] {
  if (!text) return []
  const seen = new Set<string>()
  const out: string[] = []
  const matches = text.match(/\{\{([^}]+)\}\}/g) || []
  for (const m of matches) {
    const name = m.replace(/[{}]/g, '').trim()
    if (name && !seen.has(name)) {
      seen.add(name)
      out.push(name)
    }
  }
  return out
}

const templateHeaderParamName = computed<string | null>(() => {
  const tpl = selectedTemplate.value
  if (!tpl || tpl.header_type !== 'TEXT') return null
  const names = extractParamNames(tpl.header_content)
  return names[0] || null
})

const templateBodyParamNames = computed<string[]>(() => {
  if (!selectedTemplate.value) return []
  return extractParamNames(selectedTemplate.value.body_content)
})

// Concatenated list (header first if present, then body). Used purely to
// render the input slot count / example placeholders — the actual recipient
// payload splits header and body back apart.
const templateParamNames = computed<string[]>(() => {
  const list: string[] = []
  if (templateHeaderParamName.value) list.push('header')
  list.push(...templateBodyParamNames.value)
  return list
})

function exampleValue(name: string, index: number): string {
  if (name === 'header') return 'Summer'
  if (/^\d+$/.test(name)) return `value${index + 1}`
  if (name.toLowerCase().includes('name')) return 'John Doe'
  if (name.toLowerCase().includes('order')) return 'ORD-123'
  if (name.toLowerCase().includes('date')) return '2024-01-15'
  if (name.toLowerCase().includes('amount') || name.toLowerCase().includes('price')) return '99.99'
  return `${name}_value`
}

const recipientPlaceholder = computed(() => {
  const params = templateParamNames.value
  if (params.length === 0) {
    return `+1234567890, John Doe\n+0987654321, Jane Smith\n+1122334455`
  }
  const exampleValues = params.map(exampleValue)
  return `+1234567890, John Doe, ${exampleValues.join(', ')}\n+0987654321, Jane Smith, ${exampleValues.join(', ')}`
})

const manualEntryFormat = computed(() => {
  const labels: string[] = []
  if (templateHeaderParamName.value) {
    labels.push(`header (${templateHeaderParamName.value})`)
  }
  for (const p of templateBodyParamNames.value) {
    labels.push(/^\d+$/.test(p) ? `param${p}` : p)
  }
  if (labels.length === 0) return 'phone_number, name (optional)'
  return `phone_number, name, ${labels.join(', ')}`
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

function getStatusVariant(status: string): string | undefined {
  switch (status) {
    case 'completed':
      return 'success'
    case 'running':
    case 'processing':
    case 'queued':
      return 'secondary'
    case 'failed':
    case 'cancelled':
      return 'danger'
    default:
      return undefined
  }
}

function getRecipientStatusVariant(status: string): string | undefined {
  switch (status) {
    case 'sent':
    case 'delivered':
      return 'success'
    case 'failed':
      return 'danger'
    default:
      return undefined
  }
}

const progressPercent = computed(() => {
  const c = campaign.value
  if (!c || c.total_recipients <= 0) return 0
  return Math.round(((c.sent_count + c.failed_count) / c.total_recipients) * 100)
})

// Recipients section is open on first load when the list is short.
const recipientsOpen = ref(false)

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
    const blob = new Blob([response.data], { type: response.headers.get('content-type') ?? undefined })
    existingMediaUrl.value = URL.createObjectURL(blob)
  } catch {
    existingMediaUrl.value = null
  }
}

let recipientsLoadedOnce = false

async function loadRecipients() {
  if (isNew.value || !campaign.value) return
  isLoadingRecipients.value = true
  try {
    const response = await campaignsService.getRecipients(campaign.value.id)
    recipients.value = (response.data as any).data?.recipients || []
    if (!recipientsLoadedOnce) {
      recipientsOpen.value = recipients.value.length > 0 && recipients.value.length <= 20
    }
  } catch {
    recipients.value = []
  } finally {
    recipientsLoadedOnce = true
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
  const expectedCount = templateParamNames.value.length
  const lines = recipientsInput.value.trim().split('\n').filter((line: string) => line.trim())

  if (lines.length === 0) {
    return { isValid: false, totalLines: 0, validLines: 0, invalidLines: [] as { lineNumber: number; reason: string }[] }
  }

  const invalidLines: { lineNumber: number; reason: string }[] = []
  const seenPhones = new Set<string>()

  for (let i = 0; i < lines.length; i++) {
    const parts = lines[i].split(',').map((p: string) => p.trim())
    const phone = parts[0]?.replace(/[^\d+]/g, '')

    if (!phone || !phone.match(/^\+?\d{10,15}$/)) {
      invalidLines.push({ lineNumber: i + 1, reason: 'Invalid phone number' })
      continue
    }

    if (seenPhones.has(phone)) {
      invalidLines.push({ lineNumber: i + 1, reason: `Duplicate phone number (${phone})` })
      continue
    }
    seenPhones.add(phone)

    // Layout: phone, name, <header_value if any>, <body_val_1>, ..., <body_val_n>
    // Name (parts[1]) is optional — accept blank — but the column count must
    // match the template so values don't shift into the wrong slot.
    const provided = parts.slice(2).filter((p: string) => p.length > 0).length
    if (expectedCount > 0 && provided < expectedCount) {
      invalidLines.push({
        lineNumber: i + 1,
        reason: `Need ${expectedCount} parameter value${expectedCount === 1 ? '' : 's'} after the name column, got ${provided}. Format: ${manualEntryFormat.value}`,
      })
      continue
    }
    if (expectedCount > 0 && parts.length - 2 > expectedCount) {
      // Extra trailing columns usually mean an unquoted comma in a value —
      // call it out instead of silently truncating.
      invalidLines.push({
        lineNumber: i + 1,
        reason: `Too many columns: expected ${expectedCount + 2}, got ${parts.length}. If a value contains a comma, remove it.`,
      })
    }
  }

  return {
    isValid: invalidLines.length === 0 && lines.length > 0,
    totalLines: lines.length,
    validLines: lines.length - invalidLines.length,
    invalidLines,
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

  // Layout: phone, name, <header value if any>, <body_val_1>, ..., <body_val_n>
  // Header and body values are split into separate payload keys so a
  // positional header {{1}} doesn't collide with body {{1}}.
  const headerName = templateHeaderParamName.value
  const bodyNames = templateBodyParamNames.value
  const recipientsList = lines.map(line => {
    const parts = line.split(',').map(p => p.trim())
    const recipient: {
      phone_number: string
      recipient_name?: string
      template_params?: Record<string, any>
      header_params?: Record<string, any>
    } = {
      phone_number: parts[0].replace(/[^\d+]/g, ''),
    }
    if (parts[1]?.trim()) {
      recipient.recipient_name = parts[1].trim()
    }

    let cursor = 2
    if (headerName) {
      const val = parts[cursor]
      if (val && val.length > 0) {
        recipient.header_params = { [headerName]: val }
      }
      cursor++
    }

    const params: Record<string, any> = {}
    for (let i = 0; i < bodyNames.length; i++) {
      const val = parts[cursor + i]
      if (val && val.length > 0) {
        params[bodyNames[i]] = val
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

// Header row of the sample CSV — mirrors what the parser accepts.
const csvColumns = computed<string[]>(() => {
  const cols = ['phone_number', 'name']
  if (templateHeaderParamName.value) cols.push('header')
  for (const p of templateBodyParamNames.value) cols.push(p)
  return cols
})

const csvHeaderRow = computed(() => csvColumns.value.join(','))

function downloadSampleCSV() {
  const cols = csvColumns.value
  const exampleRow = cols.map((c, idx) => {
    if (idx === 0) return '+1234567890'
    if (c === 'name') return 'John Doe'
    if (c === 'header') return templateHeaderParamName.value
      ? exampleValue(templateHeaderParamName.value, 0)
      : 'Summer'
    return exampleValue(c, idx)
  })
  const secondRow = cols.map((c, idx) => {
    if (idx === 0) return '+0987654321'
    if (c === 'name') return 'Jane Smith'
    if (c === 'header') return templateHeaderParamName.value
      ? exampleValue(templateHeaderParamName.value, 1)
      : 'Winter'
    return exampleValue(c, idx)
  })
  const csv = `${csvHeaderRow.value}\n${exampleRow.join(',')}\n${secondRow.join(',')}\n`

  const filename = `recipients-${campaign.value?.name?.toLowerCase().replace(/\s+/g, '-') || 'sample'}.csv`
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
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

    // Header parameter column — accept the reserved name 'header', or the
    // variable's name itself when unambiguous (named templates only).
    const headerName = templateHeaderParamName.value
    let headerColIdx = -1
    if (headerName) {
      headerColIdx = headers.findIndex(h => h === 'header')
      if (headerColIdx === -1 && !/^\d+$/.test(headerName)) {
        headerColIdx = headers.findIndex(h => h === headerName.toLowerCase())
      }
    }

    const bodyNames = templateBodyParamNames.value
    const recipientsList: {
      phone_number: string
      recipient_name?: string
      template_params?: Record<string, any>
      header_params?: Record<string, any>
    }[] = []

    for (let i = 1; i < lines.length; i++) {
      const values = lines[i].split(',').map(v => v.trim())
      const phone = values[phoneIndex]?.replace(/[^\d+]/g, '')
      if (!phone) continue

      const recipient: {
        phone_number: string
        recipient_name?: string
        template_params?: Record<string, any>
        header_params?: Record<string, any>
      } = { phone_number: phone }

      if (nameIndex !== -1 && values[nameIndex]?.trim()) {
        recipient.recipient_name = values[nameIndex].trim()
      }

      const usedIndices = new Set<number>([phoneIndex])
      if (nameIndex !== -1) usedIndices.add(nameIndex)

      if (headerName && headerColIdx !== -1) {
        const v = values[headerColIdx]?.trim()
        if (v) recipient.header_params = { [headerName]: v }
        usedIndices.add(headerColIdx)
      }

      if (bodyNames.length > 0) {
        const params: Record<string, any> = {}

        // Try name matching first
        for (const paramName of bodyNames) {
          const colIdx = headers.findIndex((h, idx) =>
            !usedIndices.has(idx) && h === paramName.toLowerCase()
          )
          if (colIdx !== -1) {
            params[paramName] = values[colIdx]?.trim() || ''
            usedIndices.add(colIdx)
          }
        }

        // Positional fallback for unmapped params
        const unmapped = bodyNames.filter(p => !(p in params))
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
    back-link="/campaigns"
    :breadcrumbs="breadcrumbs"
    :is-loading="isLoading"
    :is-not-found="isNotFound"
    :not-found-title="$t('campaigns.notFound', 'Campaign not found')"
  >
    <template #actions>
      <div class="hstack gap-2">
        <!-- Status badge for existing campaigns -->
        <span v-if="!isNew && campaign" class="badge" :data-variant="getStatusVariant(campaign.status)">
          <component :is="getStatusIcon(campaign.status)" :size="12" />
          {{ campaign.status }}
        </span>

        <!-- Start/Resume -->
        <button v-if="!isNew && campaign && canStart" type="button" class="outline small" @click="startDialogOpen = true">
          <Play :size="16" />
          {{ campaign.status === 'paused' ? $t('campaigns.resume', 'Resume') : $t('campaigns.start', 'Start') }}
        </button>

        <!-- Pause -->
        <button v-if="!isNew && campaign && canPause" type="button" class="outline small" @click="pauseDialogOpen = true">
          <Pause :size="16" />
          {{ $t('campaigns.pause', 'Pause') }}
        </button>

        <!-- Cancel -->
        <button v-if="!isNew && campaign && canCancel" type="button" class="outline small" @click="cancelDialogOpen = true">
          <XCircle :size="16" />
          {{ $t('campaigns.cancel', 'Cancel') }}
        </button>

        <!-- Retry Failed -->
        <button v-if="!isNew && campaign && canRetryFailed" type="button" class="outline small" @click="retryFailed">
          <RefreshCw :size="16" />
          {{ $t('campaigns.retryFailed', 'Retry Failed') }}
        </button>

        <button
          v-if="isDraft && (hasChanges || isNew)"
          type="button"
          class="small"
          :disabled="isSaving"
          :aria-busy="isSaving ? 'true' : undefined"
          @click="save"
        >
          <Save v-if="!isSaving" :size="16" />
          {{ isSaving ? $t('common.saving', 'Saving...') : isNew ? $t('common.create') : $t('common.save') }}
        </button>
        <button v-if="isDraft && !isNew" type="button" class="small" data-variant="danger" @click="deleteDialogOpen = true">
          <Trash2 :size="16" /> {{ $t('common.delete') }}
        </button>
      </div>
    </template>

    <!-- Details Card -->
    <article class="card">
      <header>
        <h3>{{ $t('campaigns.details', 'Details') }}</h3>
      </header>
      <div class="vstack gap-4">
        <label data-field>
          {{ $t('campaigns.name', 'Name') }} *
          <input id="name" v-model="form.name" :disabled="!isDraft" :placeholder="$t('campaigns.namePlaceholder', 'Enter campaign name')" />
        </label>
        <label data-field>
          {{ $t('campaigns.whatsappAccount', 'WhatsApp Account') }}
          <select v-model="form.whatsapp_account" :disabled="!isDraft">
            <option value="" disabled>{{ $t('campaigns.selectAccount', 'Select account') }}</option>
            <option v-for="account in accounts" :key="account.name" :value="account.name">{{ account.name }}</option>
          </select>
        </label>
        <label data-field>
          {{ $t('campaigns.template', 'Template') }}
          <select v-model="form.template_id" :disabled="!isDraft || !form.whatsapp_account">
            <option value="" disabled>{{ form.whatsapp_account ? $t('campaigns.selectTemplate', 'Select template') : $t('campaigns.selectAccountFirst', 'Select an account first') }}</option>
            <option v-for="tmpl in templates.filter(t => t.id)" :key="tmpl.id" :value="tmpl.id">{{ tmpl.display_name || tmpl.name }}</option>
          </select>
        </label>

        <!-- Media Upload Section -->
        <div v-if="templateNeedsMedia" class="vstack gap-2">
          <strong>{{ $t('campaigns.headerMedia', 'Header Media') }}</strong>
          <!-- Show existing media with preview -->
          <div v-if="campaign?.header_media_filename && !mediaFile" class="vstack gap-2">
            <!-- Image preview -->
            <img
              v-if="campaign.header_media_mime_type?.startsWith('image/') && existingMediaUrl"
              :src="existingMediaUrl"
              :alt="campaign.header_media_filename"
              class="media-preview"
            />
            <!-- Video preview -->
            <video
              v-else-if="campaign.header_media_mime_type?.startsWith('video/') && existingMediaUrl"
              :src="existingMediaUrl"
              controls
              class="media-preview"
            />
            <!-- File info bar -->
            <div class="hstack gap-2">
              <Upload :size="16" />
              <span class="truncate flex-1">{{ campaign.header_media_filename }}</span>
              <button v-if="isDraft" type="button" class="ghost small" @click="showMediaUpload = true">
                {{ $t('campaigns.replace', 'Replace') }}
              </button>
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

        <div v-if="!isNew && campaign" class="vstack gap-1">
          <small class="text-light">{{ $t('campaigns.status', 'Status') }}</small>
          <div>
            <span class="badge" :data-variant="getStatusVariant(campaign.status)">
              <component :is="getStatusIcon(campaign.status)" :size="12" />
              {{ campaign.status }}
            </span>
          </div>
        </div>
      </div>
    </article>

    <!-- Stats Card (existing campaigns only) -->
    <article v-if="!isNew && campaign" class="card">
      <header>
        <h3>{{ $t('campaigns.statistics', 'Statistics') }}</h3>
      </header>
      <div class="hstack gap-4 flex-wrap">
        <div class="vstack items-center gap-1 flex-1">
          <Users :size="16" />
          <strong>{{ campaign.total_recipients }}</strong>
          <small class="text-light">{{ $t('campaigns.totalRecipients', 'Recipients') }}</small>
        </div>
        <div class="vstack items-center gap-1 flex-1">
          <Send :size="16" />
          <strong>{{ campaign.sent_count }}</strong>
          <small class="text-light">{{ $t('campaigns.sent', 'Sent') }}</small>
        </div>
        <div class="vstack items-center gap-1 flex-1">
          <CheckCircle :size="16" />
          <strong>{{ campaign.delivered_count }}</strong>
          <small class="text-light">{{ $t('campaigns.delivered', 'Delivered') }}</small>
        </div>
        <div class="vstack items-center gap-1 flex-1">
          <Eye :size="16" />
          <strong>{{ campaign.read_count }}</strong>
          <small class="text-light">{{ $t('campaigns.read', 'Read') }}</small>
        </div>
        <div class="vstack items-center gap-1 flex-1">
          <XCircle :size="16" />
          <strong>{{ campaign.failed_count }}</strong>
          <small class="text-light">{{ $t('campaigns.failed', 'Failed') }}</small>
        </div>
      </div>

      <!-- Progress Bar -->
      <div v-if="campaign.total_recipients > 0" class="vstack gap-1 mt-4">
        <div class="hstack justify-between">
          <small class="text-light">{{ $t('campaigns.progress', 'Progress') }}</small>
          <small class="text-light">{{ progressPercent }}%</small>
        </div>
        <progress :value="progressPercent" max="100" :aria-label="$t('campaigns.progress', 'Progress')"></progress>
      </div>
    </article>

    <!-- Recipients Card (collapsible) -->
    <article v-if="!isNew && campaign" class="card">
      <div v-if="isDraft" class="hstack justify-end mb-2">
        <button type="button" class="outline small" @click="openAddRecipientsDialog">
          <UserPlus :size="16" />
          {{ $t('campaigns.addRecipients', 'Add') }}
        </button>
      </div>
      <details :open="recipientsOpen" @toggle="recipientsOpen = ($event.target as HTMLDetailsElement).open">
        <summary>
          {{ $t('campaigns.recipients', 'Recipients') }} ({{ recipients.length }})
        </summary>
        <div class="vstack gap-4 mt-4">
          <div v-if="isLoadingRecipients" class="align-center p-4" role="status" aria-busy="true" data-spinner="large" :aria-label="$t('common.loading', 'Loading...')" />
          <div v-else-if="recipients.length === 0" class="vstack items-center gap-2 p-4">
            <Users :size="32" />
            <p class="text-light">{{ $t('campaigns.noRecipients', 'No recipients yet') }}</p>
            <button v-if="isDraft" type="button" class="outline small" @click="openAddRecipientsDialog">
              <UserPlus :size="16" />
              {{ $t('campaigns.addRecipients', 'Add Recipients') }}
            </button>
          </div>
          <div v-else class="table scroll-y">
            <table>
              <thead>
                <tr>
                  <th>{{ $t('campaigns.phoneNumber', 'Phone Number') }}</th>
                  <th>{{ $t('campaigns.recipientName', 'Name') }}</th>
                  <th>{{ $t('campaigns.status', 'Status') }}</th>
                  <th>{{ $t('campaigns.sentAt', 'Sent At') }}</th>
                  <th v-if="isDraft" :aria-label="$t('common.actions')"></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="recipient in recipients" :key="recipient.id">
                  <td><code>{{ recipient.phone_number }}</code></td>
                  <td>{{ recipient.recipient_name || '-' }}</td>
                  <td>
                    <div class="vstack gap-1">
                      <span class="badge" :data-variant="getRecipientStatusVariant(recipient.status)">{{ recipient.status }}</span>
                      <small
                        v-if="recipient.status === 'failed' && recipient.error_message"
                        class="truncate"
                        :title="recipient.error_message"
                      >
                        {{ recipient.error_message }}
                      </small>
                    </div>
                  </td>
                  <td class="text-light">{{ recipient.sent_at ? formatDateTime(recipient.sent_at) : '-' }}</td>
                  <td v-if="isDraft">
                    <button
                      type="button"
                      class="ghost small icon"
                      :title="$t('common.delete')"
                      :aria-label="$t('common.delete')"
                      :disabled="deletingRecipientId === recipient.id"
                      @click="deleteRecipient(recipient.id)"
                    >
                      <Trash2 :size="14" />
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </details>
    </article>

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
  <ConfirmDialog
    v-model:open="deleteDialogOpen"
    :title="$t('campaigns.deleteCampaign', 'Delete Campaign')"
    :description="$t('campaigns.deleteConfirm', 'Are you sure? This action cannot be undone.')"
    :confirm-label="$t('common.delete')"
    @confirm="deleteCampaign"
  />

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
  <Modal
    v-model:open="showAddRecipientsDialog"
    size="large"
    :title="$t('campaigns.addRecipients', 'Add Recipients')"
    :description="$t('campaigns.addRecipientsDescription', 'Add phone numbers manually or upload a CSV file.')"
  >
    <div role="tablist" class="w-100">
      <button
        type="button"
        role="tab"
        class="flex-1"
        :aria-selected="addRecipientsTab === 'manual'"
        @click="addRecipientsTab = 'manual'"
      >
        <UserPlus :size="16" />
        {{ $t('campaigns.manualEntry', 'Manual Entry') }}
      </button>
      <button
        type="button"
        role="tab"
        class="flex-1"
        :aria-selected="addRecipientsTab === 'csv'"
        @click="addRecipientsTab = 'csv'"
      >
        <FileSpreadsheet :size="16" />
        {{ $t('campaigns.csvUpload', 'CSV Upload') }}
      </button>
    </div>

    <!-- Manual Entry Tab -->
    <div v-if="addRecipientsTab === 'manual'" role="tabpanel">
      <div class="vstack gap-2">
        <label data-field>
          <span>{{ $t('campaigns.formatHint', 'Format') }}: <code>{{ manualEntryFormat }}</code></span>
          <small data-hint>{{ $t('campaigns.recipientFormatNote', 'One recipient per line. Name is optional. Template parameters must match the selected template.') }}</small>
          <textarea
            id="recipients"
            v-model="recipientsInput"
            :placeholder="recipientPlaceholder"
            rows="8"
          />
        </label>
        <!-- Validation Feedback -->
        <div v-if="recipientsInput.trim()">
          <small v-if="manualInputValidation.isValid" class="text-light">
            {{ manualInputValidation.validLines }} valid recipient{{ manualInputValidation.validLines !== 1 ? 's' : '' }}
          </small>
          <div v-else-if="manualInputValidation.invalidLines.length > 0" class="error" role="status">
            <p>
              {{ manualInputValidation.invalidLines.length }} of {{ manualInputValidation.totalLines }} lines have errors:
            </p>
            <ul>
              <li v-for="err in manualInputValidation.invalidLines.slice(0, 5)" :key="err.lineNumber">
                Line {{ err.lineNumber }}: {{ err.reason }}
              </li>
              <li v-if="manualInputValidation.invalidLines.length > 5">
                ...and {{ manualInputValidation.invalidLines.length - 5 }} more
              </li>
            </ul>
          </div>
        </div>
        <div class="hstack justify-end">
          <button
            type="button"
            :disabled="isAddingRecipients || !manualInputValidation.isValid"
            :aria-busy="isAddingRecipients ? 'true' : undefined"
            @click="addRecipients"
          >
            <UserPlus v-if="!isAddingRecipients" :size="16" />
            {{ isAddingRecipients ? $t('common.adding', 'Adding...') : $t('campaigns.addRecipients', 'Add Recipients') }}
          </button>
        </div>
      </div>
    </div>

    <!-- CSV Upload Tab -->
    <div v-if="addRecipientsTab === 'csv'" role="tabpanel">
      <div class="vstack gap-2">
        <small class="text-light">
          {{ $t('campaigns.csvFormatHint', 'CSV must include a phone_number (or phone, mobile, number) column. Optionally include a name column. For templates with a TEXT header variable, add a "header" column. Then one column per body parameter.') }}
        </small>
        <div class="hstack justify-between">
          <code>{{ csvHeaderRow }}</code>
          <button type="button" class="ghost small" @click="downloadSampleCSV">
            <Download :size="14" />
            {{ $t('campaigns.downloadSampleCsv', 'Download sample CSV') }}
          </button>
        </div>
        <button type="button" class="outline" @click="csvInput?.click()">
          <Upload :size="16" />
          {{ csvFile ? csvFile.name : $t('campaigns.clickToUploadCSV', 'Click to select CSV file') }}
        </button>
        <input ref="csvInput" type="file" accept=".csv" hidden @change="handleCSVFileSelect" />
        <div class="hstack justify-end">
          <button
            type="button"
            :disabled="isAddingRecipients || !csvFile"
            :aria-busy="isAddingRecipients ? 'true' : undefined"
            @click="addRecipientsFromCSV"
          >
            <FileSpreadsheet v-if="!isAddingRecipients" :size="16" />
            {{ isAddingRecipients ? $t('common.importing', 'Importing...') : $t('campaigns.importCSV', 'Import CSV') }}
          </button>
        </div>
      </div>
    </div>
  </Modal>

  <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
