<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Skeleton } from '@/components/ui/skeleton'
import { Progress } from '@/components/ui/progress'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { RangeCalendar } from '@/components/ui/range-calendar'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger
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
import { campaignsService, templatesService, accountsService } from '@/services/api'
import { wsService } from '@/services/websocket'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { toast } from 'vue-sonner'
import {
  Plus,
  Pencil,
  Trash2,
  Megaphone,
  Play,
  Pause,
  XCircle,
  Users,
  CheckCircle,
  Clock,
  AlertCircle,
  Loader2,
  Upload,
  UserPlus,
  Eye,
  FileSpreadsheet,
  AlertTriangle,
  Check,
  RefreshCw,
  CalendarIcon,
  Image,
  FileText,
  Video,
  X,
  MessageSquare
} from 'lucide-vue-next'
import { formatDate } from '@/lib/utils'
import type { DateRange } from 'reka-ui'
import { CalendarDate } from '@internationalized/date'

interface Campaign {
  id: string
  name: string
  template_name: string
  template_id?: string
  whatsapp_account?: string
  header_media_id?: string
  header_media_filename?: string
  header_media_mime_type?: string
  status: 'draft' | 'scheduled' | 'running' | 'paused' | 'completed' | 'failed' | 'queued' | 'processing' | 'cancelled'
  total_recipients: number
  sent_count: number
  delivered_count: number
  read_count: number
  failed_count: number
  scheduled_at?: string
  started_at?: string
  completed_at?: string
  created_at: string
}

interface Template {
  id: string
  name: string
  display_name?: string
  status: string
  body_content?: string
  header_type?: string  // TEXT, IMAGE, DOCUMENT, VIDEO
  header_content?: string
}

interface CSVRow {
  phone_number: string
  name: string
  params: Record<string, string>  // keyed by param name (e.g., {"name": "John"} or {"1": "John"})
  isValid: boolean
  errors: string[]
}

interface CSVValidation {
  isValid: boolean
  rows: CSVRow[]
  templateParamNames: string[]  // e.g., ["name", "order_id"] or ["1", "2"]
  csvColumns: string[]
  columnMapping: { csvColumn: string; paramName: string }[]  // Shows how CSV columns map to params
  errors: string[]
  warnings: string[]  // Non-blocking warnings (e.g., mixed param types)
}

interface Account {
  id: string
  name: string
  phone_id: string
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

const campaigns = ref<Campaign[]>([])
const templates = ref<Template[]>([])
const accounts = ref<Account[]>([])
const isLoading = ref(true)
const isCreating = ref(false)
const showCreateDialog = ref(false)

// Filter state
const filterStatus = ref<string>('all')
type TimeRangePreset = 'today' | '7days' | '30days' | 'this_month' | 'custom'
const selectedRange = ref<TimeRangePreset>('this_month')
const customDateRange = ref<DateRange>({ start: undefined, end: undefined })
const isDatePickerOpen = ref(false)

const statusOptions = [
  { value: 'all', label: 'All Statuses' },
  { value: 'draft', label: 'Draft' },
  { value: 'queued', label: 'Queued' },
  { value: 'processing', label: 'Processing' },
  { value: 'completed', label: 'Completed' },
  { value: 'failed', label: 'Failed' },
  { value: 'cancelled', label: 'Cancelled' },
  { value: 'paused', label: 'Paused' },
]

// Format date as YYYY-MM-DD in local timezone
const formatDateLocal = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const getDateRange = computed(() => {
  const now = new Date()
  let from: Date
  let to: Date = now

  switch (selectedRange.value) {
    case 'today':
      from = new Date(now.getFullYear(), now.getMonth(), now.getDate())
      to = new Date(now.getFullYear(), now.getMonth(), now.getDate())
      break
    case '7days':
      from = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 7)
      to = new Date(now.getFullYear(), now.getMonth(), now.getDate())
      break
    case '30days':
      from = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 30)
      to = new Date(now.getFullYear(), now.getMonth(), now.getDate())
      break
    case 'this_month':
      from = new Date(now.getFullYear(), now.getMonth(), 1)
      to = new Date(now.getFullYear(), now.getMonth(), now.getDate())
      break
    case 'custom':
      if (customDateRange.value.start && customDateRange.value.end) {
        from = new Date(customDateRange.value.start.year, customDateRange.value.start.month - 1, customDateRange.value.start.day)
        to = new Date(customDateRange.value.end.year, customDateRange.value.end.month - 1, customDateRange.value.end.day)
      } else {
        from = new Date(now.getFullYear(), now.getMonth(), 1)
        to = new Date(now.getFullYear(), now.getMonth(), now.getDate())
      }
      break
    default:
      from = new Date(now.getFullYear(), now.getMonth(), 1)
      to = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  }

  return {
    from: formatDateLocal(from),
    to: formatDateLocal(to)
  }
})

const formatDateRangeDisplay = computed(() => {
  if (selectedRange.value === 'custom' && customDateRange.value.start && customDateRange.value.end) {
    const start = customDateRange.value.start
    const end = customDateRange.value.end
    return `${start.month}/${start.day}/${start.year} - ${end.month}/${end.day}/${end.year}`
  }
  return ''
})

// Recipients state
const showRecipientsDialog = ref(false)
const showAddRecipientsDialog = ref(false)
const selectedCampaign = ref<Campaign | null>(null)
const recipients = ref<Recipient[]>([])
const isLoadingRecipients = ref(false)
const isAddingRecipients = ref(false)
const recipientsInput = ref('')

// CSV upload state
const csvFile = ref<File | null>(null)
const csvValidation = ref<CSVValidation | null>(null)
const isValidatingCSV = ref(false)
const selectedTemplate = ref<Template | null>(null)
const addRecipientsTab = ref('manual')

// Media upload state
const mediaFile = ref<File | null>(null)
const isUploadingMedia = ref(false)
const mediaPreviewUrl = ref<string | null>(null)

// Computed: template parameter format hints
const templateParamNames = computed(() => {
  if (!selectedTemplate.value) return []
  return getTemplateParamNames(selectedTemplate.value)
})

const manualEntryFormat = computed(() => {
  const params = templateParamNames.value
  if (params.length === 0) {
    return 'phone_number'
  }
  return `phone_number, ${params.join(', ')}`
})

const csvColumnsHint = computed(() => {
  const params = templateParamNames.value
  if (params.length === 0) {
    return ['phone_number (or phone, mobile, number)']
  }
  return [
    'phone_number (or phone, mobile, number)',
    ...params.map(p => p)
  ]
})

function formatParamName(param: string): string {
  return `{{${param}}}`
}

// Dynamic placeholder for recipient input based on template parameters
const recipientPlaceholder = computed(() => {
  const params = templateParamNames.value
  if (params.length === 0) {
    return `+1234567890
+0987654321
+1122334455`
  }
  // Generate example values for each parameter
  const exampleValues = params.map((p, i) => {
    if (/^\d+$/.test(p)) {
      return `value${i + 1}`
    }
    // Use parameter name as hint for example value
    if (p.toLowerCase().includes('name')) return 'John Doe'
    if (p.toLowerCase().includes('order')) return 'ORD-123'
    if (p.toLowerCase().includes('date')) return '2024-01-15'
    if (p.toLowerCase().includes('amount') || p.toLowerCase().includes('price')) return '99.99'
    return `${p}_value`
  })
  const line1 = `+1234567890, ${exampleValues.join(', ')}`
  const line2 = `+0987654321, ${exampleValues.map((v, i) => {
    if (v === 'John Doe') return 'Jane Smith'
    if (v === 'ORD-123') return 'ORD-456'
    return v
  }).join(', ')}`
  return `${line1}\n${line2}`
})

// Check if selected campaign's template has media header
const campaignTemplateHasMedia = computed(() => {
  if (!selectedCampaign.value?.template_id) return false
  const template = templates.value.find(t => t.id === selectedCampaign.value?.template_id)
  return template?.header_type && template.header_type !== 'TEXT'
})

const campaignTemplateMediaType = computed(() => {
  if (!selectedCampaign.value?.template_id) return null
  const template = templates.value.find(t => t.id === selectedCampaign.value?.template_id)
  return template?.header_type || null
})

// Get accepted file types based on template header type
const acceptedMediaTypes = computed(() => {
  const type = campaignTemplateMediaType.value
  switch (type) {
    case 'IMAGE': return 'image/jpeg,image/png'
    case 'VIDEO': return 'video/mp4,video/3gpp'
    case 'DOCUMENT': return 'application/pdf,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document'
    default: return '*/*'
  }
})

function handleMediaFileSelect(event: Event) {
  const input = event.target as HTMLInputElement
  if (input.files && input.files[0]) {
    mediaFile.value = input.files[0]
    // Create preview URL for images
    if (mediaFile.value.type.startsWith('image/')) {
      mediaPreviewUrl.value = URL.createObjectURL(mediaFile.value)
    } else {
      mediaPreviewUrl.value = null
    }
  }
}

function clearMediaFile() {
  mediaFile.value = null
  if (mediaPreviewUrl.value) {
    URL.revokeObjectURL(mediaPreviewUrl.value)
    mediaPreviewUrl.value = null
  }
}

async function uploadCampaignMedia() {
  if (!selectedCampaign.value || !mediaFile.value) return

  isUploadingMedia.value = true
  try {
    const response = await campaignsService.uploadMedia(selectedCampaign.value.id, mediaFile.value)
    const result = response.data.data
    toast.success('Media uploaded successfully')
    // Update campaign with media ID
    selectedCampaign.value.header_media_id = result.media_id
    await fetchCampaigns()
    // Update selectedCampaign with fresh data
    const updated = campaigns.value.find(c => c.id === selectedCampaign.value?.id)
    if (updated) {
      selectedCampaign.value = updated
    }
    clearMediaFile()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to upload media'
    toast.error(message)
  } finally {
    isUploadingMedia.value = false
  }
}

// Manual input validation
interface ManualInputValidation {
  isValid: boolean
  totalLines: number
  validLines: number
  invalidLines: { lineNumber: number; reason: string }[]
}

const manualInputValidation = computed((): ManualInputValidation => {
  const params = templateParamNames.value
  const lines = recipientsInput.value.trim().split('\n').filter(line => line.trim())

  if (lines.length === 0) {
    return { isValid: false, totalLines: 0, validLines: 0, invalidLines: [] }
  }

  const invalidLines: { lineNumber: number; reason: string }[] = []

  for (let i = 0; i < lines.length; i++) {
    const parts = lines[i].split(',').map(p => p.trim())
    const phone = parts[0]?.replace(/[^\d+]/g, '')

    // Validate phone number
    if (!phone || !phone.match(/^\+?\d{10,15}$/)) {
      invalidLines.push({ lineNumber: i + 1, reason: 'Invalid phone number' })
      continue
    }

    // Validate params count
    const providedParams = parts.slice(1).filter(p => p.length > 0).length
    if (params.length > 0 && providedParams < params.length) {
      invalidLines.push({
        lineNumber: i + 1,
        reason: `Missing parameters: needs ${params.length}, has ${providedParams}`
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

// Form state
const newCampaign = ref({
  name: '',
  whatsapp_account: '',
  template_id: ''
})

// AlertDialog state
const deleteDialogOpen = ref(false)
const cancelDialogOpen = ref(false)
const campaignToDelete = ref<Campaign | null>(null)
const campaignToCancel = ref<Campaign | null>(null)

// WebSocket subscription for real-time stats updates
let unsubscribeCampaignStats: (() => void) | null = null

onMounted(async () => {
  await Promise.all([
    fetchCampaigns(),
    fetchTemplates(),
    fetchAccounts()
  ])

  // Subscribe to campaign stats updates
  unsubscribeCampaignStats = wsService.onCampaignStatsUpdate((payload) => {
    const campaign = campaigns.value.find(c => c.id === payload.campaign_id)
    if (campaign) {
      campaign.sent_count = payload.sent_count
      campaign.delivered_count = payload.delivered_count
      campaign.read_count = payload.read_count
      campaign.failed_count = payload.failed_count
      if (payload.status) {
        campaign.status = payload.status
      }
    }
  })
})

onUnmounted(() => {
  if (unsubscribeCampaignStats) {
    unsubscribeCampaignStats()
  }
})

async function fetchCampaigns() {
  isLoading.value = true
  try {
    const { from, to } = getDateRange.value
    const params: Record<string, string> = { from, to }
    if (filterStatus.value && filterStatus.value !== 'all') {
      params.status = filterStatus.value
    }
    const response = await campaignsService.list(params)
    // API returns: { status: "success", data: { campaigns: [...] } }
    campaigns.value = response.data.data?.campaigns || []
  } catch (error) {
    console.error('Failed to fetch campaigns:', error)
    campaigns.value = []
  } finally {
    isLoading.value = false
  }
}

function applyCustomRange() {
  if (customDateRange.value.start && customDateRange.value.end) {
    isDatePickerOpen.value = false
    fetchCampaigns()
  }
}

// Watch for filter changes
watch([filterStatus, selectedRange], () => {
  if (selectedRange.value !== 'custom') {
    fetchCampaigns()
  }
})

async function fetchTemplates() {
  try {
    const response = await templatesService.list()
    templates.value = response.data.data?.templates || []
  } catch (error) {
    console.error('Failed to fetch templates:', error)
    templates.value = []
  }
}

async function fetchAccounts() {
  try {
    const response = await accountsService.list()
    accounts.value = response.data.data?.accounts || []
  } catch (error) {
    console.error('Failed to fetch accounts:', error)
    accounts.value = []
  }
}

async function createCampaign() {
  if (!newCampaign.value.name) {
    toast.error('Please enter a campaign name')
    return
  }
  if (!newCampaign.value.whatsapp_account) {
    toast.error('Please select a WhatsApp account')
    return
  }
  if (!newCampaign.value.template_id) {
    toast.error('Please select a template')
    return
  }

  isCreating.value = true
  try {
    await campaignsService.create({
      name: newCampaign.value.name,
      whatsapp_account: newCampaign.value.whatsapp_account,
      template_id: newCampaign.value.template_id
    })
    toast.success('Campaign created successfully')
    showCreateDialog.value = false
    resetForm()
    await fetchCampaigns()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to create campaign'
    toast.error(message)
  } finally {
    isCreating.value = false
  }
}

function resetForm() {
  newCampaign.value = {
    name: '',
    whatsapp_account: '',
    template_id: ''
  }
}

async function startCampaign(campaign: Campaign) {
  try {
    await campaignsService.start(campaign.id)
    toast.success('Campaign started')
    await fetchCampaigns()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to start campaign'
    toast.error(message)
  }
}

async function pauseCampaign(campaign: Campaign) {
  try {
    await campaignsService.pause(campaign.id)
    toast.success('Campaign paused')
    await fetchCampaigns()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to pause campaign'
    toast.error(message)
  }
}

function openCancelDialog(campaign: Campaign) {
  campaignToCancel.value = campaign
  cancelDialogOpen.value = true
}

async function confirmCancelCampaign() {
  if (!campaignToCancel.value) return

  try {
    await campaignsService.cancel(campaignToCancel.value.id)
    toast.success('Campaign cancelled')
    cancelDialogOpen.value = false
    campaignToCancel.value = null
    await fetchCampaigns()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to cancel campaign'
    toast.error(message)
  }
}

async function retryFailed(campaign: Campaign) {
  try {
    const response = await campaignsService.retryFailed(campaign.id)
    const result = response.data.data
    toast.success(`Retrying ${result?.retry_count || 0} failed message(s)`)
    await fetchCampaigns()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to retry failed messages'
    toast.error(message)
  }
}

function openDeleteDialog(campaign: Campaign) {
  campaignToDelete.value = campaign
  deleteDialogOpen.value = true
}

async function confirmDeleteCampaign() {
  if (!campaignToDelete.value) return

  try {
    await campaignsService.delete(campaignToDelete.value.id)
    toast.success('Campaign deleted')
    deleteDialogOpen.value = false
    campaignToDelete.value = null
    await fetchCampaigns()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to delete campaign'
    toast.error(message)
  }
}

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

function getProgressPercentage(campaign: Campaign): number {
  if (campaign.total_recipients === 0) return 0
  return Math.round((campaign.sent_count / campaign.total_recipients) * 100)
}

// Helper functions for media upload
function getTemplateHeaderType(templateId: string | undefined): string | null {
  if (!templateId) return null
  const template = templates.value.find(t => t.id === templateId)
  return template?.header_type || null
}

function getMediaIcon(headerType: string | null) {
  switch (headerType) {
    case 'IMAGE': return Image
    case 'VIDEO': return Video
    case 'DOCUMENT': return FileText
    default: return FileText
  }
}

function getAcceptedMediaTypes(headerType: string | null): string {
  switch (headerType) {
    case 'IMAGE': return 'image/jpeg,image/png'
    case 'VIDEO': return 'video/mp4,video/3gpp'
    case 'DOCUMENT': return 'application/pdf,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document'
    default: return '*/*'
  }
}

function triggerMediaFileInput(campaignId: string) {
  const input = window.document.querySelector(`input[data-campaign-id="${campaignId}"]`) as HTMLInputElement
  input?.click()
}

// Cache for media blob URLs and loading states
const mediaBlobUrls = ref<Record<string, string>>({})
const mediaLoadingState = ref<Record<string, 'loading' | 'loaded' | 'error'>>({})

async function loadMediaPreview(campaignId: string) {
  if (mediaLoadingState.value[campaignId]) return // Already loading or loaded

  mediaLoadingState.value[campaignId] = 'loading'
  try {
    const response = await campaignsService.getMedia(campaignId)
    const blob = new Blob([response.data], { type: response.headers['content-type'] })
    mediaBlobUrls.value[campaignId] = URL.createObjectURL(blob)
    mediaLoadingState.value[campaignId] = 'loaded'
  } catch (error) {
    console.error('Failed to load media preview:', error)
    mediaLoadingState.value[campaignId] = 'error'
  }
}

function getMediaPreviewUrl(campaignId: string): string {
  return mediaBlobUrls.value[campaignId] || ''
}

function isMediaPreviewAvailable(campaignId: string): boolean {
  // Trigger loading if not started
  if (!mediaLoadingState.value[campaignId]) {
    loadMediaPreview(campaignId)
  }
  return mediaLoadingState.value[campaignId] === 'loaded'
}

function isMediaPreviewLoading(campaignId: string): boolean {
  return mediaLoadingState.value[campaignId] === 'loading'
}

// Media preview dialog
const showMediaPreviewDialog = ref(false)
const previewingCampaign = ref<Campaign | null>(null)

function openMediaPreview(campaign: Campaign) {
  previewingCampaign.value = campaign
  showMediaPreviewDialog.value = true
}

// Recipients functions
const deletingRecipientId = ref<string | null>(null)

async function viewRecipients(campaign: Campaign) {
  selectedCampaign.value = campaign
  showRecipientsDialog.value = true
  isLoadingRecipients.value = true
  try {
    const response = await campaignsService.getRecipients(campaign.id)
    recipients.value = response.data.data?.recipients || []
  } catch (error) {
    console.error('Failed to fetch recipients:', error)
    toast.error('Failed to load recipients')
    recipients.value = []
  } finally {
    isLoadingRecipients.value = false
  }
}

async function deleteRecipient(recipientId: string) {
  if (!selectedCampaign.value) return

  deletingRecipientId.value = recipientId
  try {
    await campaignsService.deleteRecipient(selectedCampaign.value.id, recipientId)
    recipients.value = recipients.value.filter(r => r.id !== recipientId)
    // Update recipient count in selectedCampaign
    selectedCampaign.value.total_recipients = recipients.value.length
    toast.success('Recipient deleted')
    await fetchCampaigns() // Refresh campaigns list
    // Update selectedCampaign with fresh data
    const updated = campaigns.value.find(c => c.id === selectedCampaign.value?.id)
    if (updated) {
      selectedCampaign.value = updated
    }
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to delete recipient'
    toast.error(message)
  } finally {
    deletingRecipientId.value = null
  }
}

async function addRecipients() {
  if (!selectedCampaign.value) return

  const lines = recipientsInput.value.trim().split('\n').filter(line => line.trim())
  if (lines.length === 0) {
    toast.error('Please enter at least one phone number')
    return
  }

  // Get template parameter names for mapping
  const paramNames = templateParamNames.value

  // Parse CSV/text input - format: phone_number, param1, param2, ...
  // Parameters are mapped to template parameter names in order
  const recipientsList = lines.map(line => {
    const parts = line.split(',').map(p => p.trim())
    const recipient: { phone_number: string; recipient_name?: string; template_params?: Record<string, any> } = {
      phone_number: parts[0].replace(/[^\d+]/g, '') // Clean phone number
    }

    // Map values to template parameter names
    const params: Record<string, any> = {}
    for (let i = 1; i < parts.length && i <= paramNames.length; i++) {
      if (parts[i] && parts[i].length > 0) {
        params[paramNames[i - 1]] = parts[i]
      }
    }

    if (Object.keys(params).length > 0) {
      recipient.template_params = params
    }
    return recipient
  })

  isAddingRecipients.value = true
  try {
    const response = await campaignsService.addRecipients(selectedCampaign.value.id, recipientsList)
    const result = response.data.data
    toast.success(`Added ${result?.added_count || recipientsList.length} recipients`)
    showAddRecipientsDialog.value = false
    recipientsInput.value = ''
    await fetchCampaigns()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to add recipients'
    toast.error(message)
  } finally {
    isAddingRecipients.value = false
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

// CSV functions
function getTemplateParamNames(template: Template): string[] {
  // Extract parameter names from body_content on-the-fly
  // Supports both positional ({{1}}, {{2}}) and named ({{name}}, {{order_id}}) parameters
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

function highlightTemplateParams(content: string): string {
  // Escape HTML first to prevent XSS
  const escaped = content
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
  // Highlight parameters with a styled span
  return escaped.replace(
    /\{\{([^}]+)\}\}/g,
    '<span class="bg-primary/20 text-primary px-1 rounded font-medium">{{$1}}</span>'
  )
}

function hasMixedParamTypes(paramNames: string[]): boolean {
  // Check if template has both positional (numeric) and named parameters
  if (paramNames.length === 0) return false
  const hasPositional = paramNames.some(n => /^\d+$/.test(n))
  const hasNamed = paramNames.some(n => !/^\d+$/.test(n))
  return hasPositional && hasNamed
}

async function openAddRecipientsDialog(campaign: Campaign) {
  selectedCampaign.value = campaign
  recipientsInput.value = ''
  csvFile.value = null
  csvValidation.value = null
  addRecipientsTab.value = 'manual'

  // Fetch template details to get body_content
  if (campaign.template_id) {
    try {
      const response = await templatesService.get(campaign.template_id)
      selectedTemplate.value = response.data.data || response.data
    } catch (error) {
      console.error('Failed to fetch template:', error)
      selectedTemplate.value = null
    }
  }

  showAddRecipientsDialog.value = true
}

function handleCSVFileSelect(event: Event) {
  const input = event.target as HTMLInputElement
  if (input.files && input.files[0]) {
    csvFile.value = input.files[0]
    validateCSV()
  }
}

async function validateCSV() {
  if (!csvFile.value || !selectedTemplate.value) return

  isValidatingCSV.value = true
  csvValidation.value = null

  try {
    const text = await csvFile.value.text()
    const lines = text.split('\n').filter(line => line.trim())

    if (lines.length === 0) {
      csvValidation.value = {
        isValid: false,
        rows: [],
        templateParamNames: [],
        csvColumns: [],
        columnMapping: [],
        errors: ['CSV file is empty'],
        warnings: []
      }
      return
    }

    // Parse header row
    const headerLine = lines[0]
    const headers = parseCSVLine(headerLine).map(h => h.toLowerCase().trim())

    // Find required columns
    const phoneIndex = headers.findIndex(h =>
      h === 'phone' || h === 'phone_number' || h === 'phonenumber' || h === 'mobile' || h === 'number'
    )
    const nameIndex = headers.findIndex(h =>
      h === 'name' || h === 'recipient_name' || h === 'recipientname' || h === 'customer_name'
    )

    // Get template parameter names (e.g., ["name", "order_id"] or ["1", "2"])
    const templateParamNames = getTemplateParamNames(selectedTemplate.value)

    const globalErrors: string[] = []
    const globalWarnings: string[] = []

    if (phoneIndex === -1) {
      globalErrors.push('Missing required column: phone_number (or phone, mobile, number)')
    }

    // Warn about mixed param types
    if (hasMixedParamTypes(templateParamNames)) {
      globalWarnings.push('Template has mixed parameter types (e.g., {{1}} and {{name}}). This may cause unexpected behavior. Use CSV columns that exactly match the parameter names.')
    }

    // Map CSV columns to template parameter names
    // Strategy:
    // 1. Try to match CSV headers to template param names directly
    // 2. Fall back to positional mapping for remaining params
    const paramColumnMapping: { csvIndex: number; paramName: string }[] = []
    const usedCsvIndices = new Set<number>([phoneIndex, nameIndex].filter(i => i >= 0))
    const mappedParamNames = new Set<string>()

    // First pass: exact matches between CSV headers and template param names
    for (const paramName of templateParamNames) {
      const csvIndex = headers.findIndex((h, idx) =>
        !usedCsvIndices.has(idx) && (h === paramName.toLowerCase() || h === `param${paramName}` || h === `{{${paramName}}}`)
      )
      if (csvIndex !== -1) {
        paramColumnMapping.push({ csvIndex, paramName })
        usedCsvIndices.add(csvIndex)
        mappedParamNames.add(paramName)
      }
    }

    // Second pass: positional mapping for unmapped params
    const remainingParamNames = templateParamNames.filter(n => !mappedParamNames.has(n))
    const remainingCsvIndices = headers
      .map((_, idx) => idx)
      .filter(idx => !usedCsvIndices.has(idx))
      .sort((a, b) => a - b)

    for (let i = 0; i < remainingParamNames.length && i < remainingCsvIndices.length; i++) {
      paramColumnMapping.push({ csvIndex: remainingCsvIndices[i], paramName: remainingParamNames[i] })
    }

    // Validate CSV columns match template params
    if (templateParamNames.length > 0) {
      // Check for missing columns (params that couldn't be mapped)
      const mappedCount = paramColumnMapping.length
      if (mappedCount < templateParamNames.length) {
        const unmappedParams = templateParamNames.slice(mappedCount)
        globalErrors.push(`Missing columns for template parameters: ${unmappedParams.join(', ')}`)
      }

      // Warn if named params are being mapped positionally (not by column name)
      const namedParams = templateParamNames.filter(n => !/^\d+$/.test(n))
      if (namedParams.length > 0) {
        const positionallyMapped = namedParams.filter(n => !mappedParamNames.has(n))
        if (positionallyMapped.length > 0) {
          globalWarnings.push(`Parameters mapped by position (not column name): ${positionallyMapped.join(', ')}. For best results, use column names that match the template parameters.`)
        }
      }
    }

    // Parse data rows
    const rows: CSVRow[] = []
    const seenPhones = new Map<string, number>() // phone -> first occurrence row index

    for (let i = 1; i < lines.length; i++) {
      const values = parseCSVLine(lines[i])
      if (values.length === 0 || (values.length === 1 && !values[0].trim())) continue

      const rowErrors: string[] = []
      const phone = phoneIndex >= 0 ? values[phoneIndex]?.trim() || '' : ''
      const cleanPhone = phone.replace(/[^\d+]/g, '') // Normalize for duplicate check
      const name = nameIndex >= 0 ? values[nameIndex]?.trim() || '' : ''

      // Build params object with proper keys
      const params: Record<string, string> = {}
      for (const mapping of paramColumnMapping) {
        const value = values[mapping.csvIndex]?.trim() || ''
        if (value) {
          params[mapping.paramName] = value
        }
      }

      // Validate phone number
      if (!phone) {
        rowErrors.push('Missing phone number')
      } else if (!phone.match(/^\+?\d{10,15}$/)) {
        rowErrors.push('Invalid phone number format')
      } else {
        // Check for duplicates
        if (seenPhones.has(cleanPhone)) {
          rowErrors.push(`Duplicate phone number (first seen in row ${seenPhones.get(cleanPhone)! + 1})`)
        } else {
          seenPhones.set(cleanPhone, rows.length)
        }
      }

      // Validate params count if template requires params
      const providedParamCount = Object.keys(params).length
      if (templateParamNames.length > 0 && providedParamCount < templateParamNames.length) {
        rowErrors.push(`Template requires ${templateParamNames.length} parameter(s), found ${providedParamCount}`)
      }

      rows.push({
        phone_number: phone,
        name,
        params,
        isValid: rowErrors.length === 0,
        errors: rowErrors
      })
    }

    const validRows = rows.filter(r => r.isValid)

    // Build column mapping for display
    const columnMapping = paramColumnMapping.map(m => ({
      csvColumn: headers[m.csvIndex],
      paramName: m.paramName
    }))

    csvValidation.value = {
      isValid: globalErrors.length === 0 && validRows.length > 0,
      rows,
      templateParamNames,
      csvColumns: headers,
      columnMapping,
      errors: globalErrors,
      warnings: globalWarnings
    }
  } catch (error) {
    console.error('Failed to parse CSV:', error)
    csvValidation.value = {
      isValid: false,
      rows: [],
      templateParamNames: [],
      csvColumns: [],
      columnMapping: [],
      errors: ['Failed to parse CSV file'],
      warnings: []
    }
  } finally {
    isValidatingCSV.value = false
  }
}

function parseCSVLine(line: string): string[] {
  const result: string[] = []
  let current = ''
  let inQuotes = false

  for (let i = 0; i < line.length; i++) {
    const char = line[i]

    if (char === '"') {
      if (inQuotes && line[i + 1] === '"') {
        current += '"'
        i++
      } else {
        inQuotes = !inQuotes
      }
    } else if (char === ',' && !inQuotes) {
      result.push(current)
      current = ''
    } else {
      current += char
    }
  }
  result.push(current)

  return result
}

async function addRecipientsFromCSV() {
  if (!selectedCampaign.value || !csvValidation.value) return

  const validRows = csvValidation.value.rows.filter(r => r.isValid)
  if (validRows.length === 0) {
    toast.error('No valid rows to import')
    return
  }

  const recipientsList = validRows.map(row => {
    const recipient: { phone_number: string; recipient_name?: string; template_params?: Record<string, any> } = {
      phone_number: row.phone_number.replace(/[^\d+]/g, '')
    }
    if (row.name) {
      recipient.recipient_name = row.name
    }
    // Use params directly - already keyed by param name (e.g., {"name": "John"} or {"1": "John"})
    if (Object.keys(row.params).length > 0) {
      recipient.template_params = row.params
    }
    return recipient
  })

  isAddingRecipients.value = true
  try {
    const response = await campaignsService.addRecipients(selectedCampaign.value.id, recipientsList)
    const result = response.data.data
    toast.success(`Added ${result?.added_count || recipientsList.length} recipients from CSV`)
    showAddRecipientsDialog.value = false
    csvFile.value = null
    csvValidation.value = null
    await fetchCampaigns()
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to add recipients'
    toast.error(message)
  } finally {
    isAddingRecipients.value = false
  }
}
</script>

<template>
  <div class="flex flex-col h-full">
    <!-- Header -->
    <header class="border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div class="flex h-16 items-center px-6">
        <Megaphone class="h-5 w-5 mr-3" />
        <div class="flex-1">
          <h1 class="text-xl font-semibold">Campaigns</h1>
          <p class="text-sm text-muted-foreground">Manage bulk messaging campaigns</p>
        </div>

        <!-- Filters -->
        <div class="flex items-center gap-2 mr-4">
          <!-- Status Filter -->
          <Select v-model="filterStatus">
            <SelectTrigger class="w-[140px]">
              <SelectValue placeholder="All Statuses" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="opt in statusOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </SelectItem>
            </SelectContent>
          </Select>

          <!-- Time Range Filter -->
          <Select v-model="selectedRange">
            <SelectTrigger class="w-[150px]">
              <SelectValue placeholder="Select range" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="today">Today</SelectItem>
              <SelectItem value="7days">Last 7 days</SelectItem>
              <SelectItem value="30days">Last 30 days</SelectItem>
              <SelectItem value="this_month">This month</SelectItem>
              <SelectItem value="custom">Custom range</SelectItem>
            </SelectContent>
          </Select>

          <!-- Custom Range Popover -->
          <Popover v-if="selectedRange === 'custom'" v-model:open="isDatePickerOpen">
            <PopoverTrigger as-child>
              <Button variant="outline" class="w-auto">
                <CalendarIcon class="h-4 w-4 mr-2" />
                {{ formatDateRangeDisplay || 'Select dates' }}
              </Button>
            </PopoverTrigger>
            <PopoverContent class="w-auto p-4" align="end">
              <div class="space-y-4">
                <RangeCalendar v-model="customDateRange" :number-of-months="2" />
                <Button class="w-full" @click="applyCustomRange" :disabled="!customDateRange.start || !customDateRange.end">
                  Apply Range
                </Button>
              </div>
            </PopoverContent>
          </Popover>
        </div>

        <Dialog v-model:open="showCreateDialog">
          <DialogTrigger as-child>
            <Button variant="outline" size="sm">
              <Plus class="h-4 w-4 mr-2" />
              Create Campaign
            </Button>
          </DialogTrigger>
          <DialogContent class="sm:max-w-[500px]">
            <DialogHeader>
              <DialogTitle>Create New Campaign</DialogTitle>
              <DialogDescription>
                Create a new bulk messaging campaign. You can add recipients after creation.
              </DialogDescription>
            </DialogHeader>
            <div class="grid gap-4 py-4">
              <div class="grid gap-2">
                <Label for="name">Campaign Name</Label>
                <Input
                  id="name"
                  v-model="newCampaign.name"
                  placeholder="e.g., Holiday Promotion"
                  :disabled="isCreating"
                />
              </div>
              <div class="grid gap-2">
                <Label for="account">WhatsApp Account</Label>
                <Select v-model="newCampaign.whatsapp_account" :disabled="isCreating">
                  <SelectTrigger>
                    <SelectValue placeholder="Select an account" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="account in accounts" :key="account.id" :value="account.name">
                      {{ account.name }}
                    </SelectItem>
                  </SelectContent>
                </Select>
                <p v-if="accounts.length === 0" class="text-xs text-muted-foreground">
                  No accounts found. Please add a WhatsApp account first.
                </p>
              </div>
              <div class="grid gap-2">
                <Label for="template">Message Template</Label>
                <Select v-model="newCampaign.template_id" :disabled="isCreating">
                  <SelectTrigger>
                    <SelectValue placeholder="Select a template" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="template in templates" :key="template.id" :value="template.id">
                      {{ template.display_name || template.name }}
                    </SelectItem>
                  </SelectContent>
                </Select>
                <p v-if="templates.length === 0" class="text-xs text-muted-foreground">
                  No templates found. Please create a template first.
                </p>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" size="sm" @click="showCreateDialog = false" :disabled="isCreating">
                Cancel
              </Button>
              <Button size="sm" @click="createCampaign" :disabled="isCreating">
                <Loader2 v-if="isCreating" class="h-4 w-4 mr-2 animate-spin" />
                Create Campaign
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </header>

    <!-- Campaigns List -->
    <ScrollArea class="flex-1">
      <div class="p-6 space-y-4">
        <!-- Loading State -->
        <div v-if="isLoading" class="flex items-center justify-center py-12">
          <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
        </div>

        <!-- Campaign Cards -->
        <Card v-for="campaign in campaigns" :key="campaign.id">
          <CardContent class="p-6">
            <div class="flex items-start justify-between mb-4">
              <div class="flex items-center gap-4">
                <div class="h-12 w-12 rounded-lg bg-orange-100 dark:bg-orange-900 flex items-center justify-center">
                  <Megaphone class="h-6 w-6 text-orange-600 dark:text-orange-400" />
                </div>
                <div>
                  <h3 class="font-semibold text-lg">{{ campaign.name }}</h3>
                  <p class="text-sm text-muted-foreground">
                    Template: {{ campaign.template_name || 'N/A' }}
                  </p>
                </div>
              </div>
              <Badge variant="outline" :class="getStatusClass(campaign.status)">
                <component :is="getStatusIcon(campaign.status)" class="h-3 w-3 mr-1" />
                {{ campaign.status }}
              </Badge>
            </div>

            <!-- Progress Bar -->
            <div v-if="campaign.status === 'running' || campaign.status === 'processing'" class="mb-4">
              <div class="flex items-center justify-between text-sm mb-1">
                <span>Progress</span>
                <span>{{ getProgressPercentage(campaign) }}%</span>
              </div>
              <Progress :model-value="getProgressPercentage(campaign)" class="h-2" />
            </div>

            <!-- Stats -->
            <div class="grid grid-cols-5 gap-4 mb-4">
              <div class="text-center">
                <p class="text-2xl font-bold">{{ campaign.total_recipients.toLocaleString() }}</p>
                <p class="text-xs text-muted-foreground">Recipients</p>
              </div>
              <div class="text-center">
                <p class="text-2xl font-bold">{{ campaign.sent_count.toLocaleString() }}</p>
                <p class="text-xs text-muted-foreground">Sent</p>
              </div>
              <div class="text-center">
                <p class="text-2xl font-bold text-green-600">{{ campaign.delivered_count.toLocaleString() }}</p>
                <p class="text-xs text-muted-foreground">Delivered</p>
              </div>
              <div class="text-center">
                <p class="text-2xl font-bold text-blue-600">{{ campaign.read_count.toLocaleString() }}</p>
                <p class="text-xs text-muted-foreground">Read</p>
              </div>
              <div class="text-center">
                <p class="text-2xl font-bold text-destructive">{{ campaign.failed_count.toLocaleString() }}</p>
                <p class="text-xs text-muted-foreground">Failed</p>
              </div>
            </div>

            <!-- Timing Info -->
            <div class="text-xs text-muted-foreground mb-4">
              <span v-if="campaign.scheduled_at">
                Scheduled: {{ formatDate(campaign.scheduled_at) }}
              </span>
              <span v-else-if="campaign.started_at">
                Started: {{ formatDate(campaign.started_at) }}
              </span>
              <span v-else-if="campaign.completed_at">
                Completed: {{ formatDate(campaign.completed_at) }}
              </span>
              <span v-else>
                Created: {{ formatDate(campaign.created_at) }}
              </span>
            </div>

            <!-- Media Upload Section (for templates with media header) -->
            <div
              v-if="getTemplateHeaderType(campaign.template_id) && getTemplateHeaderType(campaign.template_id) !== 'TEXT'"
              class="mb-4 p-3 rounded-lg border bg-muted/30"
            >
              <div class="flex items-center gap-2 mb-2">
                <component :is="getMediaIcon(getTemplateHeaderType(campaign.template_id))" class="h-4 w-4 text-muted-foreground" />
                <span class="text-sm font-medium">Header Media ({{ getTemplateHeaderType(campaign.template_id) }})</span>
              </div>

              <div v-if="campaign.header_media_id" class="flex items-center gap-3 p-2 bg-green-50 dark:bg-green-950/30 rounded border border-green-200 dark:border-green-800">
                <!-- Thumbnail -->
                <div class="relative flex-shrink-0">
                  <!-- Loading -->
                  <div v-if="isMediaPreviewLoading(campaign.id)" class="w-12 h-12 flex items-center justify-center bg-muted rounded">
                    <Loader2 class="h-4 w-4 animate-spin text-muted-foreground" />
                  </div>
                  <!-- Image Thumbnail -->
                  <img
                    v-else-if="campaign.header_media_mime_type?.startsWith('image/') && isMediaPreviewAvailable(campaign.id)"
                    :src="getMediaPreviewUrl(campaign.id)"
                    :alt="campaign.header_media_filename"
                    class="w-12 h-12 object-cover rounded"
                  />
                  <!-- Video Thumbnail -->
                  <div v-else-if="campaign.header_media_mime_type?.startsWith('video/')" class="w-12 h-12 flex items-center justify-center bg-muted rounded">
                    <Video class="h-5 w-5 text-muted-foreground" />
                  </div>
                  <!-- Document Icon -->
                  <div v-else class="w-12 h-12 flex items-center justify-center bg-muted rounded">
                    <component :is="getMediaIcon(getTemplateHeaderType(campaign.template_id))" class="h-5 w-5 text-muted-foreground" />
                  </div>
                </div>
                <!-- File Info -->
                <div class="flex-1 min-w-0">
                  <p class="text-sm font-medium text-green-700 dark:text-green-400 truncate">
                    {{ campaign.header_media_filename || 'Media file' }}
                  </p>
                  <p class="text-xs text-muted-foreground">
                    {{ campaign.header_media_mime_type || 'Unknown type' }}
                  </p>
                </div>
                <!-- Preview Button -->
                <Button
                  v-if="isMediaPreviewAvailable(campaign.id) && (campaign.header_media_mime_type?.startsWith('image/') || campaign.header_media_mime_type?.startsWith('video/'))"
                  variant="ghost"
                  size="sm"
                  @click="openMediaPreview(campaign)"
                >
                  <Eye class="h-4 w-4" />
                </Button>
                <CheckCircle class="h-4 w-4 text-green-600 flex-shrink-0" />
              </div>

              <div v-else-if="campaign.status === 'draft'" class="space-y-2">
                <p class="text-xs text-muted-foreground">Upload media for template header</p>
                <div v-if="selectedCampaign?.id === campaign.id && mediaFile">
                  <div class="flex items-center gap-2 p-2 bg-background rounded border">
                    <component :is="getMediaIcon(getTemplateHeaderType(campaign.template_id))" class="h-4 w-4" />
                    <span class="text-sm flex-1 truncate">{{ mediaFile.name }}</span>
                    <Button variant="ghost" size="icon" class="h-6 w-6" @click="clearMediaFile">
                      <X class="h-3 w-3" />
                    </Button>
                  </div>
                  <Button
                    size="sm"
                    class="mt-2"
                    @click="uploadCampaignMedia"
                    :disabled="isUploadingMedia"
                  >
                    <Loader2 v-if="isUploadingMedia" class="h-4 w-4 mr-1 animate-spin" />
                    <Upload v-else class="h-4 w-4 mr-1" />
                    Upload
                  </Button>
                </div>
                <div v-else>
                  <input
                    type="file"
                    :data-campaign-id="campaign.id"
                    class="hidden"
                    :accept="getAcceptedMediaTypes(getTemplateHeaderType(campaign.template_id))"
                    @change="(e) => { selectedCampaign = campaign; handleMediaFileSelect(e) }"
                  />
                  <Button
                    variant="outline"
                    size="sm"
                    @click="triggerMediaFileInput(campaign.id)"
                  >
                    <Upload class="h-4 w-4 mr-1" />
                    Select File
                  </Button>
                </div>
              </div>

              <div v-else class="flex items-center gap-2">
                <AlertCircle class="h-4 w-4 text-amber-500" />
                <span class="text-sm text-amber-600">No media uploaded</span>
              </div>
            </div>

            <!-- Actions -->
            <div class="flex items-center justify-between border-t pt-4">
              <div class="flex gap-2">
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon" @click="viewRecipients(campaign)">
                      <Eye class="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>View Recipients</TooltipContent>
                </Tooltip>
                <Tooltip v-if="campaign.status === 'draft'">
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon" @click="openAddRecipientsDialog(campaign as any)">
                      <UserPlus class="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Add Recipients</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon">
                      <Pencil class="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Edit Campaign</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button
                      variant="ghost"
                      size="icon"
                      @click="openDeleteDialog(campaign)"
                      :disabled="campaign.status === 'running' || campaign.status === 'processing'"
                    >
                      <Trash2 class="h-4 w-4 text-destructive" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Delete Campaign</TooltipContent>
                </Tooltip>
              </div>
              <div class="flex gap-2">
                <Button
                  v-if="campaign.status === 'draft' || campaign.status === 'scheduled'"
                  size="sm"
                  @click="startCampaign(campaign)"
                >
                  <Play class="h-4 w-4 mr-1" />
                  Start
                </Button>
                <Button
                  v-if="campaign.status === 'running' || campaign.status === 'processing'"
                  variant="outline"
                  size="sm"
                  @click="pauseCampaign(campaign)"
                >
                  <Pause class="h-4 w-4 mr-1" />
                  Pause
                </Button>
                <Button
                  v-if="campaign.status === 'paused'"
                  size="sm"
                  @click="startCampaign(campaign)"
                >
                  <Play class="h-4 w-4 mr-1" />
                  Resume
                </Button>
                <Button
                  v-if="campaign.failed_count > 0 && (campaign.status === 'completed' || campaign.status === 'paused' || campaign.status === 'failed')"
                  variant="outline"
                  size="sm"
                  @click="retryFailed(campaign)"
                >
                  <RefreshCw class="h-4 w-4 mr-1" />
                  Retry Failed
                </Button>
                <Button
                  v-if="campaign.status === 'running' || campaign.status === 'paused' || campaign.status === 'processing' || campaign.status === 'queued'"
                  variant="destructive"
                  size="sm"
                  @click="openCancelDialog(campaign)"
                >
                  <XCircle class="h-4 w-4 mr-1" />
                  Cancel
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- Empty State -->
        <Card v-if="campaigns.length === 0 && !isLoading">
          <CardContent class="py-12 text-center text-muted-foreground">
            <Megaphone class="h-12 w-12 mx-auto mb-4 opacity-50" />
            <p class="text-lg font-medium">No campaigns yet</p>
            <p class="text-sm mb-4">Create your first bulk messaging campaign.</p>
            <Button variant="outline" size="sm" @click="showCreateDialog = true">
              <Plus class="h-4 w-4 mr-2" />
              Create Campaign
            </Button>
          </CardContent>
        </Card>
      </div>
    </ScrollArea>

    <!-- View Recipients Dialog -->
    <Dialog v-model:open="showRecipientsDialog">
      <DialogContent class="sm:max-w-[700px] max-h-[80vh]">
        <DialogHeader>
          <DialogTitle>Campaign Recipients</DialogTitle>
          <DialogDescription>
            {{ selectedCampaign?.name }} - {{ recipients.length }} recipient(s)
          </DialogDescription>
        </DialogHeader>
        <div class="py-4">
          <div v-if="isLoadingRecipients" class="flex items-center justify-center py-8">
            <Loader2 class="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
          <div v-else-if="recipients.length === 0" class="text-center py-8 text-muted-foreground">
            <Users class="h-12 w-12 mx-auto mb-2 opacity-50" />
            <p>No recipients added yet</p>
            <Button
              v-if="selectedCampaign?.status === 'draft'"
              variant="outline"
              size="sm"
              class="mt-4"
              @click="showRecipientsDialog = false; openAddRecipientsDialog(selectedCampaign as any)"
            >
              <UserPlus class="h-4 w-4 mr-2" />
              Add Recipients
            </Button>
          </div>
          <ScrollArea v-else class="h-[400px]">
            <table class="w-full text-sm">
              <thead class="sticky top-0 bg-background border-b">
                <tr>
                  <th class="text-left py-2 px-2">Phone Number</th>
                  <th class="text-left py-2 px-2">Name</th>
                  <th class="text-left py-2 px-2">Status</th>
                  <th class="text-left py-2 px-2">Sent At</th>
                  <th v-if="selectedCampaign?.status === 'draft'" class="text-center py-2 px-2 w-16"></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="recipient in recipients" :key="recipient.id" class="border-b">
                  <td class="py-2 px-2 font-mono">{{ recipient.phone_number }}</td>
                  <td class="py-2 px-2">{{ recipient.recipient_name || '-' }}</td>
                  <td class="py-2 px-2">
                    <div class="flex flex-col gap-1">
                      <Badge variant="outline" :class="getRecipientStatusClass(recipient.status)">
                        {{ recipient.status }}
                      </Badge>
                      <span v-if="recipient.status === 'failed' && recipient.error_message" class="text-xs text-destructive max-w-[200px] truncate" :title="recipient.error_message">
                        {{ recipient.error_message }}
                      </span>
                    </div>
                  </td>
                  <td class="py-2 px-2 text-muted-foreground">
                    {{ recipient.sent_at ? formatDate(recipient.sent_at) : '-' }}
                  </td>
                  <td v-if="selectedCampaign?.status === 'draft'" class="py-2 px-2 text-center">
                    <Button
                      variant="ghost"
                      size="icon"
                      class="h-7 w-7"
                      @click="deleteRecipient(recipient.id)"
                      :disabled="deletingRecipientId === recipient.id"
                    >
                      <Loader2 v-if="deletingRecipientId === recipient.id" class="h-4 w-4 animate-spin" />
                      <Trash2 v-else class="h-4 w-4 text-muted-foreground hover:text-destructive" />
                    </Button>
                  </td>
                </tr>
              </tbody>
            </table>
          </ScrollArea>
        </div>
        <DialogFooter>
          <Button
            v-if="selectedCampaign?.status === 'draft'"
            variant="outline"
            size="sm"
            @click="showRecipientsDialog = false; openAddRecipientsDialog(selectedCampaign as any)"
          >
            <UserPlus class="h-4 w-4 mr-2" />
            Add More
          </Button>
          <Button variant="outline" size="sm" @click="showRecipientsDialog = false">Close</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Add Recipients Dialog -->
    <Dialog v-model:open="showAddRecipientsDialog">
      <DialogContent class="sm:max-w-[700px] max-h-[85vh]">
        <DialogHeader>
          <DialogTitle>Add Recipients</DialogTitle>
          <DialogDescription>
            Add recipients to "{{ selectedCampaign?.name }}"
            <span v-if="templateParamNames.length > 0" class="block mt-1">
              Template requires {{ templateParamNames.length }} parameter(s)
            </span>
          </DialogDescription>
        </DialogHeader>

        <!-- Template Preview -->
        <div v-if="selectedTemplate?.body_content" class="mb-4 p-3 bg-muted/50 rounded-lg border">
          <div class="flex items-center gap-2 mb-2">
            <MessageSquare class="h-4 w-4 text-muted-foreground" />
            <span class="text-sm font-medium">Template Preview</span>
          </div>
          <p class="text-sm whitespace-pre-wrap" v-html="highlightTemplateParams(selectedTemplate.body_content)"></p>
        </div>

        <Tabs v-model="addRecipientsTab" class="w-full">
          <TabsList class="grid w-full grid-cols-2">
            <TabsTrigger value="manual">
              <UserPlus class="h-4 w-4 mr-2" />
              Manual Entry
            </TabsTrigger>
            <TabsTrigger value="csv">
              <FileSpreadsheet class="h-4 w-4 mr-2" />
              Upload CSV
            </TabsTrigger>
          </TabsList>

          <!-- Manual Entry Tab -->
          <TabsContent value="manual" class="mt-4">
            <div class="space-y-4">
              <div class="bg-muted p-3 rounded-lg text-sm">
                <p class="font-medium mb-2">Format (one per line):</p>
                <code class="bg-background px-2 py-1 rounded block">{{ manualEntryFormat }}</code>
                <p v-if="templateParamNames.length > 0" class="text-muted-foreground mt-2 text-xs">
                  Template parameters: <span v-for="(param, idx) in templateParamNames" :key="param"><code class="bg-background px-1 rounded">{{ formatParamName(param) }}</code><span v-if="idx < templateParamNames.length - 1">, </span></span>
                </p>
              </div>
              <div class="space-y-2">
                <Label for="recipients">Recipients</Label>
                <Textarea
                  id="recipients"
                  v-model="recipientsInput"
                  :placeholder="recipientPlaceholder"
                  rows="8"
                  class="font-mono text-sm"
                  :disabled="isAddingRecipients"
                />
                <!-- Validation status -->
                <div v-if="recipientsInput.trim()" class="space-y-2">
                  <p v-if="manualInputValidation.isValid" class="text-xs text-green-600">
                    {{ manualInputValidation.validLines }} recipient(s) valid
                  </p>
                  <div v-else-if="manualInputValidation.invalidLines.length > 0" class="text-xs">
                    <p class="text-destructive font-medium mb-1">
                      {{ manualInputValidation.invalidLines.length }} of {{ manualInputValidation.totalLines }} line(s) have errors:
                    </p>
                    <ul class="text-destructive space-y-0.5 max-h-20 overflow-y-auto">
                      <li v-for="err in manualInputValidation.invalidLines.slice(0, 5)" :key="err.lineNumber">
                        Line {{ err.lineNumber }}: {{ err.reason }}
                      </li>
                      <li v-if="manualInputValidation.invalidLines.length > 5" class="text-muted-foreground">
                        ... and {{ manualInputValidation.invalidLines.length - 5 }} more errors
                      </li>
                    </ul>
                  </div>
                  <p v-else class="text-xs text-muted-foreground">
                    {{ manualInputValidation.totalLines }} recipient(s) entered
                  </p>
                </div>
              </div>
              <div class="flex justify-end">
                <Button @click="addRecipients" :disabled="isAddingRecipients || !manualInputValidation.isValid">
                  <Loader2 v-if="isAddingRecipients" class="h-4 w-4 mr-2 animate-spin" />
                  <Upload v-else class="h-4 w-4 mr-2" />
                  Add Recipients
                </Button>
              </div>
            </div>
          </TabsContent>

          <!-- CSV Upload Tab -->
          <TabsContent value="csv" class="mt-4">
            <div class="space-y-4">
              <!-- CSV Format Info -->
              <div class="bg-muted p-3 rounded-lg text-sm">
                <p class="font-medium mb-2">Required CSV Columns:</p>
                <div class="flex flex-wrap gap-2">
                  <code v-for="col in csvColumnsHint" :key="col" class="bg-background px-2 py-1 rounded text-xs">{{ col }}</code>
                </div>
                <p v-if="templateParamNames.length > 0" class="text-muted-foreground mt-2 text-xs">
                  Template parameters: <span v-for="(param, idx) in templateParamNames" :key="param"><code class="bg-background px-1 rounded">{{ formatParamName(param) }}</code><span v-if="idx < templateParamNames.length - 1">, </span></span>
                </p>
              </div>

              <!-- File Upload -->
              <div class="space-y-2">
                <Label for="csv-file">Select CSV File</Label>
                <div class="flex items-center gap-2">
                  <Input
                    id="csv-file"
                    type="file"
                    accept=".csv"
                    @change="handleCSVFileSelect"
                    :disabled="isValidatingCSV || isAddingRecipients"
                    class="flex-1"
                  />
                  <Button
                    v-if="csvFile"
                    variant="outline"
                    size="icon"
                    @click="csvFile = null; csvValidation = null"
                    :disabled="isValidatingCSV || isAddingRecipients"
                  >
                    <XCircle class="h-4 w-4" />
                  </Button>
                </div>
              </div>

              <!-- Validation Results -->
              <div v-if="isValidatingCSV" class="flex items-center justify-center py-8">
                <Loader2 class="h-6 w-6 animate-spin text-muted-foreground" />
                <span class="ml-2 text-muted-foreground">Validating CSV...</span>
              </div>

              <div v-else-if="csvValidation" class="space-y-4">
                <!-- Global Errors -->
                <div v-if="csvValidation.errors.length > 0" class="bg-destructive/10 border border-destructive/20 rounded-lg p-3">
                  <div class="flex items-center gap-2 text-destructive font-medium mb-2">
                    <AlertTriangle class="h-4 w-4" />
                    Validation Errors
                  </div>
                  <ul class="list-disc list-inside text-sm text-destructive">
                    <li v-for="error in csvValidation.errors" :key="error">{{ error }}</li>
                  </ul>
                </div>

                <!-- Warnings -->
                <div v-if="csvValidation.warnings && csvValidation.warnings.length > 0" class="bg-orange-500/10 border border-orange-500/20 rounded-lg p-3">
                  <div class="flex items-center gap-2 text-orange-600 font-medium mb-2">
                    <AlertTriangle class="h-4 w-4" />
                    Warnings
                  </div>
                  <ul class="list-disc list-inside text-sm text-orange-600">
                    <li v-for="warning in csvValidation.warnings" :key="warning">{{ warning }}</li>
                  </ul>
                </div>

                <!-- Column Mapping Info -->
                <div v-if="csvValidation.columnMapping && csvValidation.columnMapping.length > 0" class="bg-muted/50 border rounded-lg p-3">
                  <div class="text-sm font-medium mb-2">Column Mapping</div>
                  <div class="flex flex-wrap gap-2">
                    <div
                      v-for="mapping in csvValidation.columnMapping"
                      :key="mapping.paramName"
                      class="text-xs bg-background border rounded px-2 py-1"
                    >
                      <span class="text-muted-foreground">{{ mapping.csvColumn }}</span>
                      <span class="mx-1">→</span>
                      <span class="font-mono text-primary">{{ formatParamName(mapping.paramName) }}</span>
                    </div>
                  </div>
                </div>

                <!-- Summary -->
                <div class="flex flex-wrap items-center gap-4 text-sm">
                  <div class="flex items-center gap-1">
                    <Check class="h-4 w-4 text-green-600" />
                    <span>{{ csvValidation.rows.filter(r => r.isValid).length }} valid</span>
                  </div>
                  <div v-if="csvValidation.rows.filter(r => !r.isValid).length > 0" class="flex items-center gap-1">
                    <AlertTriangle class="h-4 w-4 text-destructive" />
                    <span>{{ csvValidation.rows.filter(r => !r.isValid).length }} invalid</span>
                  </div>
                  <div v-if="csvValidation.rows.filter(r => r.errors.some(e => e.includes('Duplicate'))).length > 0" class="flex items-center gap-1 text-orange-600">
                    <Users class="h-4 w-4" />
                    <span>{{ csvValidation.rows.filter(r => r.errors.some(e => e.includes('Duplicate'))).length }} duplicates</span>
                  </div>
                  <div class="text-muted-foreground">
                    Columns: {{ csvValidation.csvColumns.join(', ') }}
                  </div>
                </div>

                <!-- Preview Table -->
                <div v-if="csvValidation.rows.length > 0" class="border rounded-lg overflow-hidden">
                  <ScrollArea class="h-[200px]">
                    <table class="w-full text-sm">
                      <thead class="sticky top-0 bg-muted border-b">
                        <tr>
                          <th class="text-left py-2 px-3 w-8"></th>
                          <th class="text-left py-2 px-3">Phone</th>
                          <th class="text-left py-2 px-3">Name</th>
                          <th class="text-left py-2 px-3">Parameters</th>
                        </tr>
                      </thead>
                      <tbody>
                        <tr
                          v-for="(row, index) in csvValidation.rows.slice(0, 50)"
                          :key="index"
                          :class="row.isValid ? '' : 'bg-destructive/5'"
                          class="border-b last:border-0"
                        >
                          <td class="py-2 px-3">
                            <Check v-if="row.isValid" class="h-4 w-4 text-green-600" />
                            <Tooltip v-else>
                              <TooltipTrigger>
                                <AlertTriangle class="h-4 w-4 text-destructive" />
                              </TooltipTrigger>
                              <TooltipContent>
                                <ul class="text-xs">
                                  <li v-for="err in row.errors" :key="err">{{ err }}</li>
                                </ul>
                              </TooltipContent>
                            </Tooltip>
                          </td>
                          <td class="py-2 px-3 font-mono">{{ row.phone_number || '-' }}</td>
                          <td class="py-2 px-3">{{ row.name || '-' }}</td>
                          <td class="py-2 px-3 text-muted-foreground">
                            {{ Object.values(row.params).filter(p => p).join(', ') || '-' }}
                          </td>
                        </tr>
                      </tbody>
                    </table>
                  </ScrollArea>
                  <div v-if="csvValidation.rows.length > 50" class="text-xs text-muted-foreground text-center py-2 border-t">
                    Showing first 50 of {{ csvValidation.rows.length }} rows
                  </div>
                </div>

                <!-- Import Button -->
                <div class="flex justify-end">
                  <Button
                    @click="addRecipientsFromCSV"
                    :disabled="isAddingRecipients || !csvValidation.isValid || csvValidation.rows.filter(r => r.isValid).length === 0"
                  >
                    <Loader2 v-if="isAddingRecipients" class="h-4 w-4 mr-2 animate-spin" />
                    <Upload v-else class="h-4 w-4 mr-2" />
                    Import {{ csvValidation.rows.filter(r => r.isValid).length }} Recipients
                  </Button>
                </div>
              </div>

              <!-- Empty state -->
              <div v-else class="text-center py-8 text-muted-foreground">
                <FileSpreadsheet class="h-12 w-12 mx-auto mb-2 opacity-50" />
                <p>Select a CSV file to preview and validate</p>
              </div>
            </div>
          </TabsContent>
        </Tabs>

        <DialogFooter class="border-t pt-4 mt-4">
          <Button variant="outline" size="sm" @click="showAddRecipientsDialog = false" :disabled="isAddingRecipients">
            Cancel
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Delete Confirmation Dialog -->
    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete Campaign</AlertDialogTitle>
          <AlertDialogDescription>
            Are you sure you want to delete "{{ campaignToDelete?.name }}"? This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction @click="confirmDeleteCampaign">Delete</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <!-- Cancel Confirmation Dialog -->
    <AlertDialog v-model:open="cancelDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Cancel Campaign</AlertDialogTitle>
          <AlertDialogDescription>
            Are you sure you want to cancel "{{ campaignToCancel?.name }}"? This will stop all pending messages.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Keep Running</AlertDialogCancel>
          <AlertDialogAction @click="confirmCancelCampaign">Cancel Campaign</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <!-- Media Preview Dialog -->
    <Dialog v-model:open="showMediaPreviewDialog">
      <DialogContent class="sm:max-w-[600px]">
        <DialogHeader>
          <DialogTitle>Media Preview</DialogTitle>
          <DialogDescription>
            {{ previewingCampaign?.header_media_filename }}
          </DialogDescription>
        </DialogHeader>
        <div class="flex items-center justify-center py-4">
          <img
            v-if="previewingCampaign?.header_media_mime_type?.startsWith('image/') && previewingCampaign?.id"
            :src="getMediaPreviewUrl(previewingCampaign.id)"
            :alt="previewingCampaign?.header_media_filename"
            class="max-w-full max-h-[60vh] object-contain rounded"
          />
          <video
            v-else-if="previewingCampaign?.header_media_mime_type?.startsWith('video/') && previewingCampaign?.id"
            :src="getMediaPreviewUrl(previewingCampaign.id)"
            controls
            class="max-w-full max-h-[60vh] rounded"
          />
        </div>
        <DialogFooter>
          <Button variant="outline" @click="showMediaPreviewDialog = false">Close</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
