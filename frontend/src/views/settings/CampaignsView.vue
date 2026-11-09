<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
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
import { campaignsService, templatesService, accountsService, contactsService, tagsService, type Tag } from '@/services/api'
import { wsService } from '@/services/websocket'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { toast } from 'vue-sonner'
import { PageHeader, DataTable, DeleteConfirmDialog, SearchInput, type Column } from '@/components/shared'
import HeaderMediaUpload from '@/components/shared/HeaderMediaUpload.vue'
import { useCampaignReportExport } from '@/composables/useCampaignReportExport'
import { useHeaderMedia } from '@/composables/useHeaderMedia'
import { useViewRefresh } from '@/composables/useViewRefresh'
import { getErrorMessage } from '@/lib/api-utils'
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
  MessageSquare,
  ImageIcon,
  FileText,
  Copy
} from 'lucide-vue-next'
import { formatDate, formatDayMonthYear } from '@/lib/utils'
import { useDebounceFn } from '@vueuse/core'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const { exportingCampaignId, exportCampaignReport } = useCampaignReportExport(t)

interface Campaign {
  id: string
  name: string
  template_name: string
  template_id?: string
  source_type?: string
  source_id?: string
  whatsapp_account?: string
  header_media_url?: string
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
  pause_reason?: string
}

interface Template {
  id: string
  name: string
  display_name?: string
  status: string
  body_content?: string
  header_type?: string  // TEXT, IMAGE, DOCUMENT, VIDEO
  header_content?: string
  buttons?: any[]
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

interface CampaignContact {
  id: string
  phone_number: string
  profile_name?: string
  name?: string
  whatsapp_account?: string
  tags?: string[]
}

interface Recipient {
  id: string
  phone_number: string
  recipient_name: string
  status: string
  sent_at?: string
  delivered_at?: string
  read_at?: string
  error_message?: string
  template_params?: Record<string, any>
  created_at?: string
}

const campaigns = ref<Campaign[]>([])
const templates = ref<Template[]>([])
const accounts = ref<Account[]>([])
const isLoading = ref(true)
const isCreating = ref(false)
const showCreateDialog = ref(false)
const editingCampaignId = ref<string | null>(null) // null = create mode, string = edit mode
const isUploadingMedia = ref(false)

const columns = computed<Column<Campaign>[]>(() => [
  { key: 'name', label: t('campaigns.campaign'), sortable: true },
  { key: 'status', label: t('campaigns.status'), sortable: true },
  { key: 'stats', label: t('campaigns.progress') },
  { key: 'created_at', label: t('campaigns.created'), sortable: true },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

const sortKey = ref('created_at')
const sortDirection = ref<'asc' | 'desc'>('desc')
const searchQuery = ref('')

// Pagination state
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 20

function canCampaignAcceptRecipients(campaign?: Campaign | null): boolean {
  if (!campaign) return false
  return campaign.status !== 'cancelled' && campaign.source_type !== 'rsvp_reminder'
}

function handlePageChange(page: number) {
  currentPage.value = page
  fetchCampaigns()
}

// Filter state
const filterStatus = ref<string>('all')
type TimeRangePreset = 'today' | '7days' | '30days' | 'this_month' | 'custom'
const selectedRange = ref<TimeRangePreset>('this_month')
const customDateRange = ref<any>({ start: undefined, end: undefined })
const isDatePickerOpen = ref(false)

const statusOptions = computed(() => [
  { value: 'all', label: t('campaigns.allStatuses') },
  { value: 'draft', label: t('campaigns.draft') },
  { value: 'queued', label: t('campaigns.queued') },
  { value: 'processing', label: t('campaigns.processing') },
  { value: 'completed', label: t('campaigns.completed') },
  { value: 'failed', label: t('campaigns.failed') },
  { value: 'cancelled', label: t('campaigns.cancelled') },
  { value: 'paused', label: t('campaigns.paused') },
])

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
    return `${formatDayMonthYear(start.year, start.month, start.day)} - ${formatDayMonthYear(end.year, end.month, end.day)}`
  }
  return ''
})

// Recipients state
const showRecipientsDialog = ref(false)
const showCampaignReportDialog = ref(false)
const showAddRecipientsDialog = ref(false)
const selectedCampaign = ref<Campaign | null>(null)
const recipients = ref<Recipient[]>([])
const isLoadingRecipients = ref(false)
const isAddingRecipients = ref(false)
const recipientsInput = ref('')
const recipientSearchQuery = ref('')
const recipientFilterStatus = ref('all')
const lastLoadedRecipientsCampaignId = ref<string | null>(null)
const campaignTimingTick = ref(Date.now())
let campaignTimingInterval: number | null = null

const activeCampaignStatuses = new Set(['scheduled', 'queued', 'processing', 'running', 'paused'])
const recipientStatusOrder = ['pending', 'queued', 'processing', 'sent', 'delivered', 'read', 'failed', 'cancelled']

const campaignOverview = computed(() => {
  return campaigns.value.reduce(
    (summary, campaign) => {
      const processedCount = getCampaignProcessedCount(campaign)
      summary.totalCampaigns += 1
      summary.activeCampaigns += activeCampaignStatuses.has(campaign.status) ? 1 : 0
      summary.totalRecipients += campaign.total_recipients
      summary.processedRecipients += processedCount
      summary.pendingRecipients += Math.max(campaign.total_recipients - processedCount, 0)
      summary.sentCount += campaign.sent_count
      summary.deliveredCount += campaign.delivered_count
      summary.readCount += campaign.read_count
      summary.failedCount += campaign.failed_count
      return summary
    },
    {
      totalCampaigns: 0,
      activeCampaigns: 0,
      totalRecipients: 0,
      processedRecipients: 0,
      pendingRecipients: 0,
      sentCount: 0,
      deliveredCount: 0,
      readCount: 0,
      failedCount: 0,
    }
  )
})

const selectedCampaignMetrics = computed(() => {
  if (!selectedCampaign.value) return null

  const campaign = selectedCampaign.value
  const processed = getCampaignProcessedCount(campaign)
  const pending = Math.max(campaign.total_recipients - processed, 0)
  const elapsedMs = getCampaignElapsedMs(campaign)
  const totalDurationMs = getCampaignTotalDurationMs(campaign)
  const estimatedRemainingMs = getCampaignEstimatedRemainingMs(campaign)

  return {
    processed,
    pending,
    progressRate: getProgressPercentage(campaign),
    deliveryRate: getPercentage(campaign.delivered_count, campaign.total_recipients),
    readRate: getPercentage(campaign.read_count, campaign.total_recipients),
    failureRate: getPercentage(campaign.failed_count, campaign.total_recipients),
    elapsedLabel: formatDuration(elapsedMs),
    totalDurationLabel: totalDurationMs === null ? '-' : formatDuration(totalDurationMs),
    speedLabel: formatCampaignSpeed(campaign),
    estimatedRemainingLabel: estimatedRemainingMs === null ? '-' : formatDuration(estimatedRemainingMs),
  }
})

const recipientStatusOptions = computed(() => {
  const availableStatuses = new Set(recipients.value.map(recipient => recipient.status).filter(Boolean))
  const dynamicStatuses = [...availableStatuses].filter(status => !recipientStatusOrder.includes(status)).sort()
  const orderedStatuses = [
    ...recipientStatusOrder.filter(status => availableStatuses.has(status)),
    ...dynamicStatuses,
  ]

  return [
    { value: 'all', label: 'All recipients' },
    ...orderedStatuses.map(status => ({
      value: status,
      label: formatStatusLabel(status),
    })),
  ]
})

const recipientDashboard = computed(() => {
  const counts = {
    total: recipients.value.length,
    pending: 0,
    sent: 0,
    delivered: 0,
    read: 0,
    failed: 0,
  }

  for (const recipient of recipients.value) {
    switch (recipient.status) {
      case 'pending':
      case 'queued':
      case 'processing':
        counts.pending += 1
        break
      case 'sent':
        counts.sent += 1
        break
      case 'delivered':
        counts.delivered += 1
        break
      case 'read':
        counts.read += 1
        break
      case 'failed':
        counts.failed += 1
        break
      default:
        break
    }
  }

  return counts
})

const filteredRecipients = computed(() => {
  const search = recipientSearchQuery.value.trim().toLowerCase()

  return recipients.value.filter((recipient) => {
    const matchesStatus = recipientFilterStatus.value === 'all' || recipient.status === recipientFilterStatus.value
    if (!matchesStatus) return false

    if (!search) return true

    return [
      recipient.phone_number,
      recipient.recipient_name,
      recipient.error_message,
      JSON.stringify(recipient.template_params || {}),
    ]
      .filter(Boolean)
      .some(value => String(value).toLowerCase().includes(search))
  })
})

const recipientFailureSummary = computed(() => {
  const grouped = new Map<string, number>()

  for (const recipient of recipients.value) {
    if (recipient.status !== 'failed') continue
    const message = (recipient.error_message || 'Unknown failure').trim() || 'Unknown failure'
    grouped.set(message, (grouped.get(message) || 0) + 1)
  }

  return [...grouped.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)
    .map(([message, count]) => ({ message, count }))
})

// CSV upload state
const csvFile = ref<File | null>(null)
const csvValidation = ref<CSVValidation | null>(null)
const isValidatingCSV = ref(false)
const selectedTemplate = ref<Template | null>(null)
const addRecipientsTab = ref('manual')
const campaignContacts = ref<CampaignContact[]>([])
const availableContactGroups = ref<Tag[]>([])
const isLoadingCampaignContacts = ref(false)
const isLoadingContactGroups = ref(false)
const contactSearchQuery = ref('')
const selectedContactIds = ref<string[]>([])
const selectedContactGroupNames = ref<string[]>([])

// Media upload state
// Computed: template parameter format hints
const templateParamNames = computed(() => {
  if (!selectedTemplate.value) return []
  return getTemplateParamNames(selectedTemplate.value)
})

const canImportFromContacts = computed(() => Boolean(selectedTemplate.value) && templateParamNames.value.length === 0)

const selectedContactCount = computed(() => selectedContactIds.value.length)

const selectedContactGroupCount = computed(() => selectedContactGroupNames.value.length)

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
  const line2 = `+0987654321, ${exampleValues.map((v) => {
    if (v === 'John Doe') return 'Jane Smith'
    if (v === 'ORD-123') return 'ORD-456'
    return v
  }).join(', ')}`
  return `${line1}\n${line2}`
})

// Manual input validation
interface ManualInputValidation {
  isValid: boolean
  totalLines: number
  validLines: number
  invalidLines: { lineNumber: number; reason: string }[]
}

const recipientImportBatchSize = 1000

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
      invalidLines.push({ lineNumber: i + 1, reason: t('campaigns.invalidPhoneNumber') })
      continue
    }

    // Validate params count
    const providedParams = parts.slice(1).filter(p => p.length > 0).length
    if (params.length > 0 && providedParams < params.length) {
      invalidLines.push({
        lineNumber: i + 1,
        reason: t('campaigns.missingParameters', { needed: params.length, has: providedParams })
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
  template_id: '',
  header_media_url: '',
  scheduled_at: ''
})

// AlertDialog state
const deleteDialogOpen = ref(false)
const cancelDialogOpen = ref(false)
const campaignToDelete = ref<Campaign | null>(null)
const campaignToCancel = ref<Campaign | null>(null)

// WebSocket subscription for real-time stats updates
let unsubscribeCampaignStats: (() => void) | null = null

function isLiveCampaignStatus(status?: Campaign['status'] | string | null) {
  return ['scheduled', 'queued', 'processing', 'running', 'paused'].includes(status || '')
}

const hasRealtimeCampaignActivity = computed(() => {
  if (showCampaignReportDialog.value || showRecipientsDialog.value) {
    return true
  }

  if (selectedCampaign.value && isLiveCampaignStatus(selectedCampaign.value.status)) {
    return true
  }

  return campaigns.value.some(campaign => isLiveCampaignStatus(campaign.status))
})

async function refreshCampaignRealtime(options: { refreshRecipients?: boolean } = {}) {
  if (!hasRealtimeCampaignActivity.value) {
    return
  }

  await fetchCampaigns()

  if (!selectedCampaign.value) {
    return
  }

  const refreshedCampaign = campaigns.value.find(campaign => campaign.id === selectedCampaign.value?.id)
  if (!refreshedCampaign) {
    return
  }

  selectedCampaign.value = refreshedCampaign

  if (options.refreshRecipients || showCampaignReportDialog.value || showRecipientsDialog.value) {
    await loadCampaignRecipients(refreshedCampaign, true)
  }
}

const queueRealtimeRefresh = useDebounceFn((refreshRecipients = false) => {
  void refreshCampaignRealtime({ refreshRecipients })
}, 1200, { maxWait: 4000 })

const { refreshNow: refreshCampaignRealtimeNow } = useViewRefresh(
  () => refreshCampaignRealtime({ refreshRecipients: showCampaignReportDialog.value || showRecipientsDialog.value }),
  {
    intervalMs: 5000,
    minGapMs: 3000,
    refreshOnFocus: true,
    refreshOnVisible: true
  }
)

onMounted(async () => {
  campaignTimingInterval = window.setInterval(() => {
    campaignTimingTick.value = Date.now()
  }, 1000)

  await Promise.all([
    fetchCampaigns(),
    fetchAccounts()
  ])

  await handleCreateFromTemplateRoute()

  // Subscribe to campaign stats updates
  unsubscribeCampaignStats = wsService.onCampaignStatsUpdate((payload) => {
    const campaign = campaigns.value.find(c => c.id === payload.campaign_id)
    if (campaign) {
      if (typeof payload.total_recipients === 'number') {
        campaign.total_recipients = payload.total_recipients
      }
      campaign.sent_count = payload.sent_count
      campaign.delivered_count = payload.delivered_count
      campaign.read_count = payload.read_count
      campaign.failed_count = payload.failed_count
      campaign.started_at = payload.started_at ?? campaign.started_at
      campaign.completed_at = payload.completed_at ?? undefined
      if (payload.status) {
        campaign.status = payload.status
      }
    }
    const activeCampaign = selectedCampaign.value
    if (!activeCampaign || activeCampaign.id !== payload.campaign_id) {
      return
    }

    if (typeof payload.total_recipients === 'number') {
      activeCampaign.total_recipients = payload.total_recipients
    }
    activeCampaign.sent_count = payload.sent_count
    activeCampaign.delivered_count = payload.delivered_count
    activeCampaign.read_count = payload.read_count
    activeCampaign.failed_count = payload.failed_count
    activeCampaign.started_at = payload.started_at ?? activeCampaign.started_at
    activeCampaign.completed_at = payload.completed_at ?? undefined
    if (payload.status) {
      activeCampaign.status = payload.status
    }

    queueRealtimeRefresh(Boolean(activeCampaign && activeCampaign.id === payload.campaign_id))
  })
})

onUnmounted(() => {
  if (unsubscribeCampaignStats) {
    unsubscribeCampaignStats()
  }
  if (campaignTimingInterval) {
    window.clearInterval(campaignTimingInterval)
    campaignTimingInterval = null
  }
})

async function fetchCampaigns() {
  isLoading.value = true
  try {
    const { from, to } = getDateRange.value
    const params: Record<string, string | number> = {
      from,
      to,
      page: currentPage.value,
      limit: pageSize
    }
    if (filterStatus.value && filterStatus.value !== 'all') {
      params.status = filterStatus.value
    }
    if (searchQuery.value) {
      params.search = searchQuery.value
    }
    const response = await campaignsService.list(params)
    // API returns: { status: "success", data: { campaigns: [...], total: N } }
    const data = response.data.data || response.data
    campaigns.value = data.campaigns || []
    if (selectedCampaign.value) {
      const updatedCampaign = campaigns.value.find(campaign => campaign.id === selectedCampaign.value?.id)
      if (updatedCampaign) {
        selectedCampaign.value = updatedCampaign
      }
    }
    totalItems.value = data.total ?? campaigns.value.length
  } catch (error) {
    console.error('Failed to fetch campaigns:', error)
    campaigns.value = []
    totalItems.value = 0
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

// Debounced search
const debouncedSearch = useDebounceFn(() => {
  currentPage.value = 1
  fetchCampaigns()
}, 300)

watch(searchQuery, () => debouncedSearch())

const debouncedContactSearch = useDebounceFn(() => {
  fetchCampaignContacts()
}, 300)

watch(contactSearchQuery, () => {
  if (addRecipientsTab.value === 'contacts') {
    debouncedContactSearch()
  }
})

watch(addRecipientsTab, (tab) => {
  if (tab !== 'contacts') return
  fetchContactGroups()
  fetchCampaignContacts()
})

watch(
  () => [route.query.createFromTemplate, route.query.templateId, route.query.account, route.query.campaignName],
  () => {
    handleCreateFromTemplateRoute()
  }
)

// Watch for filter changes
watch([filterStatus, selectedRange], () => {
  currentPage.value = 1
  if (selectedRange.value !== 'custom') {
    fetchCampaigns()
  }
})

async function fetchTemplates(account?: string) {
  try {
    const response = await templatesService.list(account ? { account } : undefined)
    const data = (response.data as any).data || response.data
    templates.value = data.templates || []
  } catch (error) {
    console.error('Failed to fetch templates:', error)
    templates.value = []
  }
}

const selectedTemplateObj = computed(() =>
  templates.value.find(t => t.id === newCampaign.value.template_id)
)

const campaignHeaderType = computed(() => selectedTemplateObj.value?.header_type)
const {
  file: campaignMediaFile,
  previewUrl: campaignMediaPreview,
  needsMedia: selectedTemplateNeedsMedia,
  acceptTypes: campaignMediaAccept,
  mediaLabel: campaignMediaLabel,
  handleFileChange: handleCampaignMediaFile,
  clear: clearCampaignMedia,
} = useHeaderMedia(campaignHeaderType)

// Re-fetch templates when account changes
watch(() => newCampaign.value.whatsapp_account, (account) => {
  newCampaign.value.template_id = ''
  if (account) {
    fetchTemplates(account)
  } else {
    templates.value = []
  }
})

async function fetchAccounts() {
  try {
    const response = await accountsService.list()
    accounts.value = response.data.data?.accounts || []
  } catch (error) {
    console.error('Failed to fetch accounts:', error)
    accounts.value = []
  }
}

async function handleCreateFromTemplateRoute() {
  if (route.name !== 'campaigns') return
  if (route.query.createFromTemplate !== '1') return

  const templateId = typeof route.query.templateId === 'string' ? route.query.templateId : ''
  const account = typeof route.query.account === 'string' ? route.query.account : ''
  const campaignName = typeof route.query.campaignName === 'string' ? route.query.campaignName : ''

  if (!templateId || !account) {
    await router.replace({ name: 'campaigns' })
    return
  }

  editingCampaignId.value = null
  resetForm()

  newCampaign.value.whatsapp_account = account
  await fetchTemplates(account)
  newCampaign.value.template_id = templateId
  newCampaign.value.name = campaignName || 'New Campaign'
  showCreateDialog.value = true

  await router.replace({ name: 'campaigns' })
}

async function fetchCampaignContacts() {
  if (!showAddRecipientsDialog.value || addRecipientsTab.value !== 'contacts') return
  if (!canImportFromContacts.value) {
    campaignContacts.value = []
    return
  }

  isLoadingCampaignContacts.value = true
  try {
    const response = await contactsService.list({
      search: contactSearchQuery.value.trim() || undefined,
      page: 1,
      limit: 50,
    })
    const data = response.data as any
    const responseData = data.data || data
    campaignContacts.value = responseData.contacts || []
  } catch (error) {
    console.error('Failed to fetch campaign contacts:', error)
    campaignContacts.value = []
  } finally {
    isLoadingCampaignContacts.value = false
  }
}

async function fetchContactGroups() {
  if (!showAddRecipientsDialog.value || addRecipientsTab.value !== 'contacts') return

  isLoadingContactGroups.value = true
  try {
    const response = await tagsService.list({ page: 1, limit: 100 })
    const data = response.data as any
    const responseData = data.data || data
    availableContactGroups.value = responseData.tags || []
  } catch (error) {
    console.error('Failed to fetch contact groups:', error)
    availableContactGroups.value = []
  } finally {
    isLoadingContactGroups.value = false
  }
}

async function createCampaign() {
  if (!newCampaign.value.name) {
    toast.error(t('campaigns.enterCampaignName'))
    return
  }
  if (!newCampaign.value.whatsapp_account) {
    toast.error(t('campaigns.selectWhatsappAccount'))
    return
  }
  if (!newCampaign.value.template_id) {
    toast.error(t('campaigns.selectTemplateRequired'))
    return
  }

  isCreating.value = true
  try {
    const response = await campaignsService.create({
      name: newCampaign.value.name,
      whatsapp_account: newCampaign.value.whatsapp_account,
      template_id: newCampaign.value.template_id,
      header_media_url: campaignMediaFile.value ? '' : newCampaign.value.header_media_url.trim(),
      scheduled_at: toCampaignScheduledAt(newCampaign.value.scheduled_at)
    })
    const created = response.data.data || response.data
    editingCampaignId.value = created?.id || null

    if (campaignMediaFile.value && created?.id) {
      try {
        await campaignsService.uploadMedia(created.id, campaignMediaFile.value)
      } catch (error: any) {
        toast.error(getErrorMessage(error, t('campaigns.mediaUploadFailed')))
        await fetchCampaigns()
        return
      }
    }

    toast.success(t('common.createdSuccess', { resource: t('resources.Campaign') }))
    showCreateDialog.value = false
    editingCampaignId.value = null
    resetForm()
    await fetchCampaigns()
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('common.failedCreate', { resource: t('resources.campaign') })))
  } finally {
    isCreating.value = false
  }
}

function resetForm() {
  newCampaign.value = {
    name: '',
    whatsapp_account: '',
    template_id: '',
    header_media_url: '',
    scheduled_at: ''
  }
  clearCampaignMedia()
}

function openEditDialog(campaign: Campaign) {
  editingCampaignId.value = campaign.id
  newCampaign.value = {
    name: campaign.name,
    whatsapp_account: campaign.whatsapp_account || '',
    template_id: campaign.template_id || '',
    header_media_url: campaign.header_media_url || '',
    scheduled_at: formatDateTimeLocal(campaign.scheduled_at)
  }
  showCreateDialog.value = true
}

function openCreateDialog() {
  editingCampaignId.value = null
  resetForm()
  showCreateDialog.value = true
}

async function saveCampaign() {
  if (!newCampaign.value.name) {
    toast.error(t('campaigns.enterCampaignName'))
    return
  }

  if (editingCampaignId.value) {
    // Update existing campaign
    isCreating.value = true
    try {
      await campaignsService.update(editingCampaignId.value, {
        name: newCampaign.value.name,
        whatsapp_account: newCampaign.value.whatsapp_account,
        template_id: newCampaign.value.template_id,
        header_media_url: campaignMediaFile.value ? '' : newCampaign.value.header_media_url.trim(),
        scheduled_at: toCampaignScheduledAt(newCampaign.value.scheduled_at)
      })

      if (campaignMediaFile.value) {
        try {
          await campaignsService.uploadMedia(editingCampaignId.value, campaignMediaFile.value)
        } catch (error: any) {
          toast.error(getErrorMessage(error, t('campaigns.mediaUploadFailed')))
          await fetchCampaigns()
          return
        }
      }

      toast.success(t('common.updatedSuccess', { resource: t('resources.Campaign') }))
      showCreateDialog.value = false
      editingCampaignId.value = null
      resetForm()
      await fetchCampaigns()
    } catch (error: any) {
      toast.error(getErrorMessage(error, t('common.failedUpdate', { resource: t('resources.campaign') })))
    } finally {
      isCreating.value = false
    }
  } else {
    // Create new campaign
    await createCampaign()
  }
}

async function startCampaign(campaign: Campaign) {
  try {
    await campaignsService.start(campaign.id)
    toast.success(t('campaigns.campaignStarted'))
    await fetchCampaigns()
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('campaigns.startFailed')))
  }
}

async function pauseCampaign(campaign: Campaign) {
  try {
    await campaignsService.pause(campaign.id)
    toast.success(t('campaigns.campaignPaused'))
    await fetchCampaigns()
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('campaigns.pauseFailed')))
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
    toast.success(t('campaigns.campaignCancelled'))
    cancelDialogOpen.value = false
    campaignToCancel.value = null
    await fetchCampaigns()
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('campaigns.cancelFailed')))
  }
}

async function retryFailed(campaign: Campaign) {
  try {
    const response = await campaignsService.retryFailed(campaign.id)
    const result = response.data.data
    const skipped = result?.skipped_inactive_count || 0
    const description = skipped > 0 ? `${skipped} inactive contact(s) skipped` : undefined
    toast.success(t('campaigns.retryingFailed', { count: result?.retry_count || 0 }), { description })
    await fetchCampaigns()
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('campaigns.retryFailedError')))
  }
}

function canResendCampaign(campaign: Campaign): boolean {
  return campaign.source_type !== 'rsvp_reminder' && ['completed', 'failed', 'cancelled', 'paused'].includes(campaign.status)
}

async function resendCampaign(campaign: Campaign) {
  try {
    const response = await campaignsService.resend(campaign.id)
    const result = response.data.data
    const queued = result?.queued_count || 0
    const skipped = result?.skipped_inactive_count || 0
    const description = skipped > 0 ? `${skipped} inactive contact(s) skipped` : undefined
    toast.success(`Resent ${queued} recipient(s) as a new campaign`, { description })
    await fetchCampaigns()
  } catch (error: any) {
    toast.error(getErrorMessage(error, 'Failed to resend campaign'))
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
    toast.success(t('common.deletedSuccess', { resource: t('resources.Campaign') }))
    deleteDialogOpen.value = false
    campaignToDelete.value = null
    await fetchCampaigns()
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('common.failedDelete', { resource: t('resources.campaign') })))
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

function formatStatusLabel(status: string): string {
  return status
    .replace(/_/g, ' ')
    .replace(/\b\w/g, char => char.toUpperCase())
}

function getCampaignProcessedCount(campaign: Campaign): number {
  return Math.min(campaign.total_recipients, campaign.sent_count + campaign.failed_count)
}

function getPercentage(value: number, total: number): number {
  if (!total) return 0
  return Math.round((value / total) * 1000) / 10
}

function formatPercentage(value: number): string {
  return `${value.toFixed(1)}%`
}

function getProgressPercentage(campaign: Campaign): number {
  if (campaign.total_recipients === 0) return 0
  return Math.round((getCampaignProcessedCount(campaign) / campaign.total_recipients) * 100)
}

function parseCampaignTimestamp(value?: string): number | null {
  if (!value) return null
  const timestamp = new Date(value).getTime()
  return Number.isNaN(timestamp) ? null : timestamp
}

function getCampaignElapsedMs(campaign: Campaign): number {
  void campaignTimingTick.value
  const startedAt = parseCampaignTimestamp(campaign.started_at)
  if (startedAt === null) return 0

  const completedAt = parseCampaignTimestamp(campaign.completed_at)
  const endAt = completedAt ?? campaignTimingTick.value
  return Math.max(endAt - startedAt, 0)
}

function getCampaignTotalDurationMs(campaign: Campaign): number | null {
  const startedAt = parseCampaignTimestamp(campaign.started_at)
  const completedAt = parseCampaignTimestamp(campaign.completed_at)
  if (startedAt === null || completedAt === null) return null
  return Math.max(completedAt - startedAt, 0)
}

function getCampaignSpeedPerMinute(campaign: Campaign): number {
  const elapsedMs = getCampaignElapsedMs(campaign)
  const processed = getCampaignProcessedCount(campaign)
  if (elapsedMs < 1000 || processed === 0) return 0
  return processed / (elapsedMs / 60000)
}

function formatCampaignSpeed(campaign: Campaign): string {
  const speed = getCampaignSpeedPerMinute(campaign)
  if (speed <= 0) return '-'
  const formatted = speed < 10 ? speed.toFixed(1) : Math.round(speed).toString()
  return `${formatted}/min`
}

function getCampaignEstimatedRemainingMs(campaign: Campaign): number | null {
  if (!activeCampaignStatuses.has(campaign.status)) return null
  const pending = Math.max(campaign.total_recipients - getCampaignProcessedCount(campaign), 0)
  const elapsedMs = getCampaignElapsedMs(campaign)
  const processed = getCampaignProcessedCount(campaign)
  if (pending === 0 || elapsedMs < 1000 || processed === 0) return null
  return Math.round((pending / processed) * elapsedMs)
}

function formatDuration(ms: number): string {
  if (ms <= 0) return '-'

  const totalSeconds = Math.max(1, Math.round(ms / 1000))
  const days = Math.floor(totalSeconds / 86400)
  const hours = Math.floor((totalSeconds % 86400) / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60

  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${minutes}m`
  if (minutes > 0) return `${minutes}m ${seconds}s`
  return `${seconds}s`
}

// Standalone media upload from table action
const mediaUploadTarget = ref<Campaign | null>(null)

function triggerMediaUpload(campaign: Campaign) {
  mediaUploadTarget.value = campaign
  nextTick(() => {
    const input = document.getElementById('campaign-media-upload') as HTMLInputElement
    if (input) {
      input.value = ''
      input.click()
    }
  })
}

async function handleStandaloneMediaUpload(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file || !mediaUploadTarget.value) return

  isUploadingMedia.value = true
  try {
    await campaignsService.uploadMedia(mediaUploadTarget.value.id, file)
    toast.success(t('campaigns.mediaUploaded'))
    // Clear cached preview so it reloads
    delete mediaBlobUrls.value[mediaUploadTarget.value.id]
    delete mediaLoadingState.value[mediaUploadTarget.value.id]
    await fetchCampaigns()
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('campaigns.mediaUploadFailed')))
  } finally {
    isUploadingMedia.value = false
    mediaUploadTarget.value = null
  }
}

function campaignNeedsMedia(campaign: Campaign): boolean {
  const tpl = templates.value.find(t => t.id === campaign.template_id)
  if (tpl) {
    const ht = tpl.header_type
    return ht === 'IMAGE' || ht === 'VIDEO' || ht === 'DOCUMENT'
  }
  // If template not in local list, check if campaign already has media fields
  return !!campaign.header_media_id || !!campaign.header_media_url
}

function campaignHasMedia(campaign: Campaign): boolean {
  return !!campaign.header_media_id || !!campaign.header_media_url
}

// Cache for media blob URLs and loading states
const mediaBlobUrls = ref<Record<string, string>>({})
const mediaLoadingState = ref<Record<string, 'loading' | 'loaded' | 'error'>>({})

async function loadMediaPreview(campaignId: string) {
  if (mediaLoadingState.value[campaignId]) return // Already loading or loaded

  mediaLoadingState.value[campaignId] = 'loading'
  try {
    const response = await campaignsService.getMedia(campaignId)
    const blob = new Blob([response.data], { type: String(response.headers['content-type'] ?? '') })
    mediaBlobUrls.value[campaignId] = URL.createObjectURL(blob)
    mediaLoadingState.value[campaignId] = 'loaded'
  } catch (error) {
    console.error('Failed to load media preview:', error)
    mediaLoadingState.value[campaignId] = 'error'
  }
}

function getMediaPreviewUrl(campaignId: string): string {
  if (!mediaLoadingState.value[campaignId]) {
    loadMediaPreview(campaignId)
  }
  return mediaBlobUrls.value[campaignId] || ''
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

function resetRecipientFilters() {
  recipientSearchQuery.value = ''
  recipientFilterStatus.value = 'all'
}

async function loadCampaignRecipients(campaign: Campaign, force = false) {
  if (!force && lastLoadedRecipientsCampaignId.value === campaign.id) {
    return
  }

  selectedCampaign.value = campaign
  isLoadingRecipients.value = true
  try {
    const response = await campaignsService.getRecipients(campaign.id)
    recipients.value = response.data.data?.recipients || []
    lastLoadedRecipientsCampaignId.value = campaign.id
  } catch (error) {
    console.error('Failed to fetch recipients:', error)
    toast.error(t('common.failedLoad', { resource: t('resources.recipients') }))
    recipients.value = []
    lastLoadedRecipientsCampaignId.value = null
  } finally {
    isLoadingRecipients.value = false
  }
}

async function openCampaignReport(campaign: Campaign) {
  selectedCampaign.value = campaign
  resetRecipientFilters()
  showCampaignReportDialog.value = true
  await loadCampaignRecipients(campaign)
  await refreshCampaignRealtimeNow(true)
}

async function viewRecipients(campaign: Campaign) {
  selectedCampaign.value = campaign
  resetRecipientFilters()
  showRecipientsDialog.value = true
  await loadCampaignRecipients(campaign)
  await refreshCampaignRealtimeNow(true)
}

async function refreshSelectedCampaignData() {
  if (!selectedCampaign.value) return

  const activeCampaignId = selectedCampaign.value.id
  await fetchCampaigns()
  const refreshedCampaign = campaigns.value.find(campaign => campaign.id === activeCampaignId)
  if (refreshedCampaign) {
    selectedCampaign.value = refreshedCampaign
    await loadCampaignRecipients(refreshedCampaign, true)
  }
}

function openRecipientsFromReport() {
  if (!selectedCampaign.value) return
  showCampaignReportDialog.value = false
  showRecipientsDialog.value = true
}

async function deleteRecipient(recipientId: string) {
  if (!selectedCampaign.value) return

  deletingRecipientId.value = recipientId
  try {
    await campaignsService.deleteRecipient(selectedCampaign.value.id, recipientId)
    recipients.value = recipients.value.filter(r => r.id !== recipientId)
    lastLoadedRecipientsCampaignId.value = selectedCampaign.value.id
    // Update recipient count in selectedCampaign
    selectedCampaign.value.total_recipients = recipients.value.length
    toast.success(t('common.deletedSuccess', { resource: t('resources.Recipient') }))
    await fetchCampaigns() // Refresh campaigns list
    // Update selectedCampaign with fresh data
    const updated = campaigns.value.find(c => c.id === selectedCampaign.value?.id)
    if (updated) {
      selectedCampaign.value = updated
    }
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('common.failedDelete', { resource: t('resources.recipient') })))
  } finally {
    deletingRecipientId.value = null
  }
}

async function addRecipients() {
  if (!selectedCampaign.value) return

  const lines = recipientsInput.value.trim().split('\n').filter(line => line.trim())
  if (lines.length === 0) {
    toast.error(t('campaigns.enterPhoneNumber'))
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
    const result = await addRecipientListToCampaign(selectedCampaign.value.id, recipientsList)
    toast.success(t('campaigns.addedRecipients', { count: result?.added_count || recipientsList.length }))
    showAddRecipientsDialog.value = false
    recipientsInput.value = ''
    lastLoadedRecipientsCampaignId.value = null
    await fetchCampaigns()
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('campaigns.addRecipientsFailed')))
  } finally {
    isAddingRecipients.value = false
  }
}

function getRecipientStatusClass(status: string): string {
  switch (status) {
    case 'sent':
    case 'delivered':
    case 'read':
      return 'border-green-600 text-green-600'
    case 'pending':
    case 'queued':
    case 'processing':
      return 'border-amber-500 text-amber-600'
    case 'failed':
      return 'border-destructive text-destructive'
    default:
      return ''
  }
}

function isCampaignContactSelected(contactId: string) {
  return selectedContactIds.value.includes(contactId)
}

function isCampaignContactGroupSelected(tagName: string) {
  return selectedContactGroupNames.value.includes(tagName)
}

function toggleCampaignContact(contactId: string) {
  if (isCampaignContactSelected(contactId)) {
    selectedContactIds.value = selectedContactIds.value.filter(id => id !== contactId)
    return
  }
  selectedContactIds.value = [...selectedContactIds.value, contactId]
}

function toggleCampaignContactGroup(tagName: string) {
  if (isCampaignContactGroupSelected(tagName)) {
    selectedContactGroupNames.value = selectedContactGroupNames.value.filter(name => name !== tagName)
    return
  }
  selectedContactGroupNames.value = [...selectedContactGroupNames.value, tagName]
}

async function addRecipientsFromContacts() {
  if (!selectedCampaign.value) return

  if (!canImportFromContacts.value) {
    toast.error('Contact and group import is available only for templates without variables.')
    return
  }

  if (selectedContactIds.value.length === 0 && selectedContactGroupNames.value.length === 0) {
    toast.error('Select at least one contact or contact group.')
    return
  }

  isAddingRecipients.value = true
  try {
    const response = await campaignsService.addRecipients(selectedCampaign.value.id, {
      contact_ids: selectedContactIds.value,
      tag_names: selectedContactGroupNames.value,
    })
    const result = response.data.data
    toast.success(t('campaigns.addedRecipients', { count: result?.added_count || 0 }))
    showAddRecipientsDialog.value = false
    selectedContactIds.value = []
    selectedContactGroupNames.value = []
    contactSearchQuery.value = ''
    lastLoadedRecipientsCampaignId.value = null
    await fetchCampaigns()
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('campaigns.addRecipientsFailed')))
  } finally {
    isAddingRecipients.value = false
  }
}

function getRecipientLastActivity(recipient: Recipient): string {
  return recipient.read_at || recipient.delivered_at || recipient.sent_at || recipient.created_at || ''
}

function getRecipientActivityLabel(recipient: Recipient): string {
  if (recipient.read_at) return 'Read'
  if (recipient.delivered_at) return 'Delivered'
  if (recipient.sent_at) return 'Sent'
  return 'Updated'
}

// CSV functions
function getTemplateParamNames(template: Template): string[] {
  const contents = [template.body_content || '']
  for (const button of template.buttons || []) {
    if (String(button?.type || '').toUpperCase() === 'URL' && button?.url) {
      contents.push(String(button.url))
    }
  }

  const seen = new Set<string>()
  const names: string[] = []

  for (const content of contents) {
    const matches = content.match(/\{\{([^}]+)\}\}/g) || []
    for (const m of matches) {
      const name = m.replace(/[{}]/g, '').trim()
      if (name && !seen.has(name)) {
        seen.add(name)
        names.push(name)
      }
    }
  }

  return names
}

function formatDateTimeLocal(value?: string): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''

  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  return `${year}-${month}-${day}T${hours}:${minutes}`
}

function toCampaignScheduledAt(value: string): string | null {
  if (!value) return null
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return null
  return date.toISOString()
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
  selectedTemplate.value = null
  addRecipientsTab.value = 'manual'
  contactSearchQuery.value = ''
  selectedContactIds.value = []
  selectedContactGroupNames.value = []
  campaignContacts.value = []

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
        errors: [t('campaigns.csvEmpty')],
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
      globalErrors.push(t('campaigns.missingPhoneColumn'))
    }

    // Warn about mixed param types
    if (hasMixedParamTypes(templateParamNames)) {
      globalWarnings.push(t('campaigns.mixedParamTypes'))
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
        globalErrors.push(t('campaigns.missingParamColumns', { params: unmappedParams.join(', ') }))
      }

      // Warn if named params are being mapped positionally (not by column name)
      const namedParams = templateParamNames.filter(n => !/^\d+$/.test(n))
      if (namedParams.length > 0) {
        const positionallyMapped = namedParams.filter(n => !mappedParamNames.has(n))
        if (positionallyMapped.length > 0) {
          globalWarnings.push(t('campaigns.paramsMappedPositionally', { params: positionallyMapped.join(', ') }))
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
        rowErrors.push(t('campaigns.missingPhoneNumber'))
      } else if (!phone.match(/^\+?\d{10,15}$/)) {
        rowErrors.push(t('campaigns.invalidPhoneFormat'))
      } else {
        // Check for duplicates
        if (seenPhones.has(cleanPhone)) {
          rowErrors.push(t('campaigns.duplicatePhone', { row: seenPhones.get(cleanPhone)! + 1 }))
        } else {
          seenPhones.set(cleanPhone, rows.length)
        }
      }

      // Validate params count if template requires params
      const providedParamCount = Object.keys(params).length
      if (templateParamNames.length > 0 && providedParamCount < templateParamNames.length) {
        rowErrors.push(t('campaigns.templateRequiresParamsError', { required: templateParamNames.length, found: providedParamCount }))
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
      errors: [t('campaigns.parseCsvFailed')],
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
    toast.error(t('campaigns.noValidRowsToImport'))
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
    const result = await addRecipientListToCampaign(selectedCampaign.value.id, recipientsList)
    toast.success(t('campaigns.addedRecipients', { count: result?.added_count || recipientsList.length }))
    showAddRecipientsDialog.value = false
    csvFile.value = null
    csvValidation.value = null
    lastLoadedRecipientsCampaignId.value = null
    await fetchCampaigns()
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('campaigns.addRecipientsFailed')))
  } finally {
    isAddingRecipients.value = false
  }
}

async function addRecipientListToCampaign(
  campaignId: string,
  recipients: Array<{ phone_number: string; recipient_name?: string; template_params?: Record<string, any> }>
) {
  if (recipients.length <= recipientImportBatchSize) {
    const response = await campaignsService.addRecipients(campaignId, { recipients })
    return response.data.data
  }

  let addedCount = 0
  let queuedCount = 0
  let totalRecipients = 0
  let sendStarted = false

  for (let i = 0; i < recipients.length; i += recipientImportBatchSize) {
    const batch = recipients.slice(i, i + recipientImportBatchSize)
    const response = await campaignsService.addRecipients(campaignId, { recipients: batch })
    const result = response.data.data || {}
    addedCount += result.added_count || 0
    queuedCount += result.queued_count || 0
    totalRecipients = result.total_recipients || totalRecipients
    sendStarted = sendStarted || !!result.send_started
  }

  return {
    added_count: addedCount,
    queued_count: queuedCount,
    total_recipients: totalRecipients,
    send_started: sendStarted
  }
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader
      :title="$t('campaigns.title')"
      :subtitle="$t('campaigns.subtitle')"
      :icon="Megaphone"
      icon-gradient="bg-gradient-to-br from-rose-500 to-pink-600 shadow-rose-500/20"
    >
      <template #actions>
        <Button variant="outline" size="sm" @click="openCreateDialog">
          <Plus class="h-4 w-4 mr-2" />
          {{ $t('campaigns.createCampaign') }}
        </Button>
      </template>
    </PageHeader>

    <Dialog v-model:open="showCreateDialog">
          <DialogContent class="sm:max-w-[500px]">
            <DialogHeader>
              <DialogTitle>{{ editingCampaignId ? $t('campaigns.editCampaign') : $t('campaigns.createNewCampaign') }}</DialogTitle>
              <DialogDescription>
                {{ editingCampaignId ? $t('campaigns.editDescription') : $t('campaigns.createDescription') }}
              </DialogDescription>
            </DialogHeader>
            <div class="grid gap-4 py-4">
              <div class="grid gap-2">
                <Label for="name">{{ $t('campaigns.campaignName') }}</Label>
                <Input
                  id="name"
                  v-model="newCampaign.name"
                  :placeholder="$t('campaigns.campaignNamePlaceholder')"
                  :disabled="isCreating"
                />
              </div>
              <div class="grid gap-2">
                <Label for="account">{{ $t('campaigns.whatsappAccount') }}</Label>
                <Select v-model="newCampaign.whatsapp_account" :disabled="isCreating">
                  <SelectTrigger>
                    <SelectValue :placeholder="$t('campaigns.selectAccount')" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="account in accounts" :key="account.id" :value="account.name">
                      {{ account.name }}
                    </SelectItem>
                  </SelectContent>
                </Select>
                <p v-if="accounts.length === 0" class="text-xs text-muted-foreground">
                  {{ $t('campaigns.noAccountsFound') }}
                </p>
              </div>
              <div class="grid gap-2">
                <Label for="template">{{ $t('campaigns.messageTemplate') }}</Label>
                <Select v-model="newCampaign.template_id" :disabled="isCreating">
                  <SelectTrigger>
                    <SelectValue :placeholder="$t('campaigns.selectTemplate')" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="template in templates" :key="template.id" :value="template.id">
                      {{ template.display_name || template.name }}
                    </SelectItem>
                  </SelectContent>
                </Select>
                <p v-if="templates.length === 0" class="text-xs text-muted-foreground">
                  {{ $t('campaigns.noTemplatesFound') }}
                </p>
              </div>
              <div class="grid gap-2">
                <Label for="scheduled_at">Schedule Send</Label>
                <Input
                  id="scheduled_at"
                  v-model="newCampaign.scheduled_at"
                  type="datetime-local"
                  :disabled="isCreating"
                />
                <p class="text-xs text-muted-foreground">
                  Leave empty to start immediately when the campaign is started.
                </p>
              </div>
              <!-- Header media upload (shown when template needs IMAGE/VIDEO/DOCUMENT) -->
              <div v-if="selectedTemplateNeedsMedia" class="grid gap-2">
                <Label for="header_media_url">Header Media URL</Label>
                <Input
                  id="header_media_url"
                  v-model="newCampaign.header_media_url"
                  placeholder="https://example.com/media.jpg"
                  :disabled="isCreating || !!campaignMediaFile"
                />
                <p class="text-xs text-muted-foreground">
                  Optional. If you upload a file, the file is used instead of this URL.
                </p>
              </div>
              <HeaderMediaUpload
                v-if="selectedTemplateNeedsMedia"
                :file="campaignMediaFile"
                :preview-url="campaignMediaPreview"
                :accept-types="campaignMediaAccept"
                :media-label="campaignMediaLabel"
                :label="$t('campaigns.headerMedia')"
                @change="handleCampaignMediaFile"
                @clear="clearCampaignMedia"
              />
            </div>
            <DialogFooter>
              <Button variant="outline" size="sm" @click="showCreateDialog = false; editingCampaignId = null" :disabled="isCreating">
                {{ $t('common.cancel') }}
              </Button>
              <Button size="sm" @click="saveCampaign" :disabled="isCreating">
                <Loader2 v-if="isCreating" class="h-4 w-4 mr-2 animate-spin" />
                {{ editingCampaignId ? $t('campaigns.saveChanges') : $t('campaigns.createCampaign') }}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

    <!-- Campaigns List -->
    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto">
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>{{ $t('campaigns.yourCampaigns') }}</CardTitle>
                  <CardDescription>{{ $t('campaigns.yourCampaignsDesc') }}</CardDescription>
                </div>
                <div class="flex items-center gap-2 flex-wrap">
                  <Select v-model="filterStatus">
                    <SelectTrigger class="w-[140px]">
                      <SelectValue :placeholder="$t('campaigns.allStatuses')" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem v-for="opt in statusOptions" :key="opt.value" :value="opt.value">
                        {{ opt.label }}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                  <Select v-model="selectedRange">
                    <SelectTrigger class="w-[140px]">
                      <SelectValue :placeholder="$t('campaigns.selectRange')" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="today">{{ $t('campaigns.today') }}</SelectItem>
                      <SelectItem value="7days">{{ $t('campaigns.last7Days') }}</SelectItem>
                      <SelectItem value="30days">{{ $t('campaigns.last30Days') }}</SelectItem>
                      <SelectItem value="this_month">{{ $t('campaigns.thisMonth') }}</SelectItem>
                      <SelectItem value="custom">{{ $t('campaigns.customRange') }}</SelectItem>
                    </SelectContent>
                  </Select>
                  <SearchInput v-model="searchQuery" :placeholder="$t('campaigns.searchCampaigns') + '...'" class="w-48" />
                  <Popover v-if="selectedRange === 'custom'" v-model:open="isDatePickerOpen">
                    <PopoverTrigger as-child>
                      <Button variant="outline" size="sm">
                        <CalendarIcon class="h-4 w-4 mr-1" />
                        {{ formatDateRangeDisplay || $t('common.select') }}
                      </Button>
                    </PopoverTrigger>
                    <PopoverContent class="w-auto p-4" align="end">
                      <div class="space-y-4">
                        <RangeCalendar v-model="customDateRange" :number-of-months="2" />
                        <Button class="w-full" size="sm" @click="applyCustomRange" :disabled="!customDateRange.start || !customDateRange.end">
                          {{ $t('campaigns.applyRange') }}
                        </Button>
                      </div>
                    </PopoverContent>
                  </Popover>
                  <Button variant="outline" size="sm" @click="fetchCampaigns" :disabled="isLoading">
                    <RefreshCw :class="['h-4 w-4 mr-2', isLoading ? 'animate-spin' : '']" />
                    {{ $t('common.refresh') }}
                  </Button>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <div class="mb-6 grid gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-6">
                <div class="rounded-xl border bg-muted/30 p-4">
                  <p class="text-xs uppercase tracking-wide text-muted-foreground">Campaigns</p>
                  <p class="mt-2 text-2xl font-semibold">{{ campaignOverview.totalCampaigns }}</p>
                  <p class="mt-1 text-xs text-muted-foreground">Visible in the current range</p>
                </div>
                <div class="rounded-xl border bg-muted/30 p-4">
                  <p class="text-xs uppercase tracking-wide text-muted-foreground">Active</p>
                  <p class="mt-2 text-2xl font-semibold">{{ campaignOverview.activeCampaigns }}</p>
                  <p class="mt-1 text-xs text-muted-foreground">Scheduled, queued, processing, running, or paused</p>
                </div>
                <div class="rounded-xl border bg-muted/30 p-4">
                  <p class="text-xs uppercase tracking-wide text-muted-foreground">Recipients</p>
                  <p class="mt-2 text-2xl font-semibold">{{ campaignOverview.totalRecipients }}</p>
                  <p class="mt-1 text-xs text-muted-foreground">Across the campaigns on this page</p>
                </div>
                <div class="rounded-xl border bg-muted/30 p-4">
                  <p class="text-xs uppercase tracking-wide text-muted-foreground">Processed</p>
                  <p class="mt-2 text-2xl font-semibold">{{ campaignOverview.processedRecipients }}</p>
                  <p class="mt-1 text-xs text-muted-foreground">
                    {{ formatPercentage(getPercentage(campaignOverview.processedRecipients, campaignOverview.totalRecipients)) }} completion
                  </p>
                </div>
                <div class="rounded-xl border bg-muted/30 p-4">
                  <p class="text-xs uppercase tracking-wide text-muted-foreground">Delivered</p>
                  <p class="mt-2 text-2xl font-semibold text-green-600">{{ campaignOverview.deliveredCount }}</p>
                  <p class="mt-1 text-xs text-muted-foreground">
                    {{ formatPercentage(getPercentage(campaignOverview.deliveredCount, campaignOverview.totalRecipients)) }} delivery rate
                  </p>
                </div>
                <div class="rounded-xl border bg-muted/30 p-4">
                  <p class="text-xs uppercase tracking-wide text-muted-foreground">Failed</p>
                  <p class="mt-2 text-2xl font-semibold text-destructive">{{ campaignOverview.failedCount }}</p>
                  <p class="mt-1 text-xs text-muted-foreground">
                    {{ campaignOverview.pendingRecipients }} still pending
                  </p>
                </div>
              </div>
              <DataTable
                :items="campaigns"
                :columns="columns"
                :is-loading="isLoading"
                :empty-icon="Megaphone"
                :empty-title="searchQuery ? $t('campaigns.noMatchingCampaigns') : $t('campaigns.noCampaignsYet')"
                :empty-description="searchQuery ? $t('campaigns.noMatchingCampaignsDesc') : $t('campaigns.noCampaignsYetDesc')"
                v-model:sort-key="sortKey"
                v-model:sort-direction="sortDirection"
                server-pagination
                :current-page="currentPage"
                :total-items="totalItems"
                :page-size="pageSize"
                item-name="campaigns"
                @page-change="handlePageChange"
              >
                <template #cell-name="{ item: campaign }">
                  <div>
                    <div class="flex items-center gap-1.5">
                      <RouterLink :to="`/campaigns/${campaign.id}`" class="font-medium text-inherit no-underline hover:opacity-80">{{ campaign.name }}</RouterLink>
                      <ImageIcon v-if="campaignHasMedia(campaign)" class="h-3.5 w-3.5 text-muted-foreground cursor-pointer hover:text-foreground" :title="campaign.header_media_filename" @click.stop="openMediaPreview(campaign)" />
                    </div>
                    <p class="text-xs text-muted-foreground">{{ campaign.template_name || $t('campaigns.noTemplate') }}</p>
                    <Badge v-if="campaign.source_type === 'rsvp_reminder'" variant="secondary" class="mt-1 text-[10px]">{{ $t('campaigns.rsvpReminder') }}</Badge>
                  </div>
                </template>
                <template #cell-status="{ item: campaign }">
                  <Badge
                    variant="outline"
                    :class="[getStatusClass(campaign.status), 'text-xs']"
                    :title="campaign.status === 'paused' ? campaign.pause_reason : undefined"
                  >
                    <component :is="getStatusIcon(campaign.status)" class="h-3 w-3 mr-1" />
                    {{ formatStatusLabel(campaign.status) }}
                  </Badge>
                  <p
                    v-if="campaign.status === 'paused' && campaign.pause_reason"
                    class="mt-1 max-w-[16rem] text-[11px] leading-snug text-amber-600"
                  >
                    {{ $t('campaigns.autoPausedShort', 'Auto-paused: WhatsApp delivery reports not reaching the app') }}
                  </p>
                </template>
                <template #cell-stats="{ item: campaign }">
                  <div class="space-y-1">
                    <div v-if="campaign.status === 'running' || campaign.status === 'processing'" class="w-32">
                      <Progress :model-value="getProgressPercentage(campaign)" class="h-1.5" />
                      <span class="text-xs text-muted-foreground">{{ getProgressPercentage(campaign) }}%</span>
                    </div>
                    <div class="flex items-center gap-3 text-xs">
                      <span title="Recipients"><Users class="h-3 w-3 inline mr-0.5" />{{ campaign.total_recipients }}</span>
                      <span class="text-green-600" title="Delivered">{{ campaign.delivered_count }}</span>
                      <span class="text-blue-600" title="Read">{{ campaign.read_count }}</span>
                      <span v-if="campaign.failed_count > 0" class="text-destructive" title="Failed">{{ campaign.failed_count }}</span>
                    </div>
                    <div v-if="campaign.started_at" class="flex items-center gap-3 text-xs text-muted-foreground">
                      <span title="Elapsed time"><Clock class="h-3 w-3 inline mr-0.5" />{{ formatDuration(getCampaignElapsedMs(campaign)) }}</span>
                      <span title="Processing speed">{{ formatCampaignSpeed(campaign) }}</span>
                    </div>
                  </div>
                </template>
                <template #cell-created_at="{ item: campaign }">
                  <span class="text-muted-foreground text-sm">{{ formatDate(campaign.created_at) }}</span>
                </template>
                <template #cell-actions="{ item: campaign }">
                  <div class="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click="openCampaignReport(campaign)" title="View Report">
                      <Eye class="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      class="h-8 w-8"
                      :disabled="exportingCampaignId === campaign.id"
                      @click="exportCampaignReport(campaign)"
                      :title="$t('campaigns.exportReport')"
                    >
                      <Loader2 v-if="exportingCampaignId === campaign.id" class="h-4 w-4 animate-spin" />
                      <FileSpreadsheet v-else class="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="icon" class="h-8 w-8" @click="viewRecipients(campaign)" title="View Recipients">
                      <Users class="h-4 w-4" />
                    </Button>
                    <Button
                      v-if="canCampaignAcceptRecipients(campaign)"
                      variant="ghost"
                      size="icon"
                      class="h-8 w-8"
                      @click="openAddRecipientsDialog(campaign as any)"
                      :title="$t('campaigns.addRecipients')"
                    >
                      <UserPlus class="h-4 w-4" />
                    </Button>
                    <Button v-if="campaign.status === 'draft' && campaign.source_type !== 'rsvp_reminder'" variant="ghost" size="icon" class="h-8 w-8" @click="openEditDialog(campaign)" title="Edit">
                      <Pencil class="h-4 w-4" />
                    </Button>
                    <Tooltip v-if="campaign.status === 'draft' && campaignNeedsMedia(campaign) && !campaignHasMedia(campaign)">
                      <TooltipTrigger as-child>
                        <Button variant="ghost" size="icon" class="h-8 w-8 text-amber-500" @click="triggerMediaUpload(campaign)">
                          <ImageIcon class="h-4 w-4" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>{{ $t('campaigns.uploadMedia') }}</TooltipContent>
                    </Tooltip>
                    <Tooltip v-if="campaignHasMedia(campaign)">
                      <TooltipTrigger as-child>
                        <Button variant="ghost" size="icon" class="h-8 w-8 text-green-600" @click="openMediaPreview(campaign)">
                          <ImageIcon class="h-4 w-4" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>{{ $t('campaigns.viewMedia') }}</TooltipContent>
                    </Tooltip>
                    <Button
                      v-if="campaign.status === 'draft' || campaign.status === 'scheduled'"
                      variant="ghost"
                      size="icon"
                      class="h-8 w-8 text-green-600"
                      @click="startCampaign(campaign)"
                      title="Start"
                    >
                      <Play class="h-4 w-4" />
                    </Button>
                    <Button
                      v-if="campaign.status === 'running' || campaign.status === 'processing'"
                      variant="ghost"
                      size="icon"
                      class="h-8 w-8"
                      @click="pauseCampaign(campaign)"
                      title="Pause"
                    >
                      <Pause class="h-4 w-4" />
                    </Button>
                    <Button
                      v-if="campaign.status === 'paused'"
                      variant="ghost"
                      size="icon"
                      class="h-8 w-8 text-green-600"
                      @click="startCampaign(campaign)"
                      title="Resume"
                    >
                      <Play class="h-4 w-4" />
                    </Button>
                    <Button
                      v-if="campaign.failed_count > 0 && (campaign.status === 'completed' || campaign.status === 'paused' || campaign.status === 'failed')"
                      variant="ghost"
                      size="icon"
                      class="h-8 w-8"
                      @click="retryFailed(campaign)"
                      title="Retry Failed"
                    >
                      <RefreshCw class="h-4 w-4" />
                    </Button>
                    <Button
                      v-if="canResendCampaign(campaign)"
                      variant="ghost"
                      size="icon"
                      class="h-8 w-8"
                      @click="resendCampaign(campaign)"
                      title="Resend as New Campaign"
                    >
                      <Copy class="h-4 w-4" />
                    </Button>
                    <Button
                      v-if="campaign.status === 'running' || campaign.status === 'paused' || campaign.status === 'processing' || campaign.status === 'queued'"
                      variant="ghost"
                      size="icon"
                      class="h-8 w-8 text-destructive"
                      @click="openCancelDialog(campaign)"
                      title="Cancel"
                    >
                      <XCircle class="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      class="h-8 w-8 text-destructive"
                      @click="openDeleteDialog(campaign)"
                      :disabled="campaign.status === 'running' || campaign.status === 'processing' || campaign.source_type === 'rsvp_reminder'"
                      title="Delete"
                    >
                      <Trash2 class="h-4 w-4" />
                    </Button>
                  </div>
                </template>
                <template #empty-action>
                  <Button v-if="!searchQuery" variant="outline" size="sm" @click="showCreateDialog = true">
                    <Plus class="h-4 w-4 mr-2" />
                    {{ $t('campaigns.createCampaign') }}
                  </Button>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <!-- Campaign Report Dialog -->
    <Dialog v-model:open="showCampaignReportDialog">
      <DialogContent class="sm:max-w-[960px] max-h-[88vh]">
        <DialogHeader>
          <DialogTitle>{{ selectedCampaign?.name || 'Campaign Report' }}</DialogTitle>
          <DialogDescription>
            {{ selectedCampaign?.template_name || $t('campaigns.noTemplate') }}
            <span v-if="selectedCampaign?.whatsapp_account"> | {{ selectedCampaign.whatsapp_account }}</span>
          </DialogDescription>
        </DialogHeader>

        <ScrollArea class="max-h-[70vh] pr-4">
          <div v-if="selectedCampaign" class="space-y-6 py-2">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div class="flex items-center gap-2">
                <Badge variant="outline" :class="[getStatusClass(selectedCampaign.status), 'text-xs']">
                  <component :is="getStatusIcon(selectedCampaign.status)" class="mr-1 h-3 w-3" />
                  {{ formatStatusLabel(selectedCampaign.status) }}
                </Badge>
                <span class="text-sm text-muted-foreground">
                  Created {{ formatDate(selectedCampaign.created_at) }}
                </span>
              </div>
              <div
                v-if="selectedCampaign.status === 'paused' && selectedCampaign.pause_reason"
                class="order-last w-full rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-sm text-amber-700 light:text-amber-800"
              >
                {{ selectedCampaign.pause_reason }}
              </div>
              <div class="flex flex-wrap gap-2">
                <Button variant="outline" size="sm" @click="refreshSelectedCampaignData" :disabled="isLoading || isLoadingRecipients">
                  <RefreshCw :class="['mr-2 h-4 w-4', isLoading || isLoadingRecipients ? 'animate-spin' : '']" />
                  {{ $t('common.refresh') }}
                </Button>
                <Button
                  v-if="selectedCampaign"
                  variant="outline"
                  size="sm"
                  :disabled="exportingCampaignId === selectedCampaign.id"
                  @click="exportCampaignReport(selectedCampaign)"
                >
                  <Loader2 v-if="exportingCampaignId === selectedCampaign.id" class="mr-2 h-4 w-4 animate-spin" />
                  <FileSpreadsheet v-else class="mr-2 h-4 w-4" />
                  {{ $t('campaigns.exportReport') }}
                </Button>
                <Button
                  v-if="canCampaignAcceptRecipients(selectedCampaign)"
                  variant="outline"
                  size="sm"
                  @click="showCampaignReportDialog = false; openAddRecipientsDialog(selectedCampaign as any)"
                >
                  <UserPlus class="mr-2 h-4 w-4" />
                  {{ $t('campaigns.addRecipients') }}
                </Button>
                <Button variant="outline" size="sm" @click="openRecipientsFromReport">
                  <Users class="mr-2 h-4 w-4" />
                  {{ $t('campaigns.viewRecipients') }}
                </Button>
              </div>
            </div>

            <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              <div class="rounded-xl border bg-muted/30 p-4">
                <p class="text-xs uppercase tracking-wide text-muted-foreground">Recipients</p>
                <p class="mt-2 text-2xl font-semibold">{{ selectedCampaign.total_recipients }}</p>
              </div>
              <div class="rounded-xl border bg-muted/30 p-4">
                <p class="text-xs uppercase tracking-wide text-muted-foreground">Processed</p>
                <p class="mt-2 text-2xl font-semibold">{{ selectedCampaignMetrics?.processed || 0 }}</p>
                <p class="mt-1 text-xs text-muted-foreground">{{ formatPercentage(selectedCampaignMetrics?.progressRate || 0) }}</p>
              </div>
              <div class="rounded-xl border bg-muted/30 p-4">
                <p class="text-xs uppercase tracking-wide text-muted-foreground">Elapsed</p>
                <p class="mt-2 text-2xl font-semibold">{{ selectedCampaignMetrics?.elapsedLabel || '-' }}</p>
                <p class="mt-1 text-xs text-muted-foreground">
                  {{ selectedCampaignMetrics && selectedCampaignMetrics.estimatedRemainingLabel !== '-' ? `${selectedCampaignMetrics.estimatedRemainingLabel} remaining` : 'No estimate' }}
                </p>
              </div>
              <div class="rounded-xl border bg-muted/30 p-4">
                <p class="text-xs uppercase tracking-wide text-muted-foreground">Speed</p>
                <p class="mt-2 text-2xl font-semibold">{{ selectedCampaignMetrics?.speedLabel || '-' }}</p>
                <p class="mt-1 text-xs text-muted-foreground">processed recipients</p>
              </div>
              <div class="rounded-xl border bg-muted/30 p-4">
                <p class="text-xs uppercase tracking-wide text-muted-foreground">Sent</p>
                <p class="mt-2 text-2xl font-semibold">{{ selectedCampaign.sent_count }}</p>
              </div>
              <div class="rounded-xl border bg-muted/30 p-4">
                <p class="text-xs uppercase tracking-wide text-muted-foreground">Delivered</p>
                <p class="mt-2 text-2xl font-semibold text-green-600">{{ selectedCampaign.delivered_count }}</p>
                <p class="mt-1 text-xs text-muted-foreground">{{ formatPercentage(selectedCampaignMetrics?.deliveryRate || 0) }}</p>
              </div>
              <div class="rounded-xl border bg-muted/30 p-4">
                <p class="text-xs uppercase tracking-wide text-muted-foreground">Read</p>
                <p class="mt-2 text-2xl font-semibold text-blue-600">{{ selectedCampaign.read_count }}</p>
                <p class="mt-1 text-xs text-muted-foreground">{{ formatPercentage(selectedCampaignMetrics?.readRate || 0) }}</p>
              </div>
              <div class="rounded-xl border bg-muted/30 p-4">
                <p class="text-xs uppercase tracking-wide text-muted-foreground">Failed</p>
                <p class="mt-2 text-2xl font-semibold text-destructive">{{ selectedCampaign.failed_count }}</p>
                <p class="mt-1 text-xs text-muted-foreground">{{ selectedCampaignMetrics?.pending || 0 }} pending</p>
              </div>
            </div>

            <div class="grid gap-4 xl:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
              <div class="rounded-xl border p-4">
                <div class="flex items-center justify-between gap-2">
                  <h3 class="text-sm font-semibold">{{ $t('common.overview') }}</h3>
                  <span class="text-xs text-muted-foreground">{{ recipients.length }} recipients loaded</span>
                </div>
                <div class="mt-4 space-y-4">
                  <div>
                    <div class="mb-1 flex items-center justify-between text-sm">
                      <span class="text-muted-foreground">Processing progress</span>
                      <span class="font-medium">{{ formatPercentage(selectedCampaignMetrics?.progressRate || 0) }}</span>
                    </div>
                    <Progress :model-value="selectedCampaignMetrics?.progressRate || 0" class="h-2" />
                  </div>
                  <div class="grid gap-3 sm:grid-cols-2">
                    <div class="rounded-lg bg-muted/40 p-3">
                      <p class="text-xs uppercase tracking-wide text-muted-foreground">Delivery rate</p>
                      <p class="mt-1 text-lg font-semibold">{{ formatPercentage(selectedCampaignMetrics?.deliveryRate || 0) }}</p>
                    </div>
                    <div class="rounded-lg bg-muted/40 p-3">
                      <p class="text-xs uppercase tracking-wide text-muted-foreground">Read rate</p>
                      <p class="mt-1 text-lg font-semibold">{{ formatPercentage(selectedCampaignMetrics?.readRate || 0) }}</p>
                    </div>
                    <div class="rounded-lg bg-muted/40 p-3">
                      <p class="text-xs uppercase tracking-wide text-muted-foreground">Failure rate</p>
                      <p class="mt-1 text-lg font-semibold">{{ formatPercentage(selectedCampaignMetrics?.failureRate || 0) }}</p>
                    </div>
                    <div class="rounded-lg bg-muted/40 p-3">
                      <p class="text-xs uppercase tracking-wide text-muted-foreground">Pending recipients</p>
                      <p class="mt-1 text-lg font-semibold">{{ selectedCampaignMetrics?.pending || 0 }}</p>
                    </div>
                    <div class="rounded-lg bg-muted/40 p-3">
                      <p class="text-xs uppercase tracking-wide text-muted-foreground">Average speed</p>
                      <p class="mt-1 text-lg font-semibold">{{ selectedCampaignMetrics?.speedLabel || '-' }}</p>
                    </div>
                    <div class="rounded-lg bg-muted/40 p-3">
                      <p class="text-xs uppercase tracking-wide text-muted-foreground">Estimated remaining</p>
                      <p class="mt-1 text-lg font-semibold">{{ selectedCampaignMetrics?.estimatedRemainingLabel || '-' }}</p>
                    </div>
                  </div>
                </div>
              </div>

              <div class="rounded-xl border p-4">
                <h3 class="text-sm font-semibold">Campaign details</h3>
                <dl class="mt-4 space-y-3 text-sm">
                  <div class="flex items-start justify-between gap-4">
                    <dt class="text-muted-foreground">WhatsApp account</dt>
                    <dd class="text-right">{{ selectedCampaign.whatsapp_account || '-' }}</dd>
                  </div>
                  <div class="flex items-start justify-between gap-4">
                    <dt class="text-muted-foreground">Template</dt>
                    <dd class="text-right">{{ selectedCampaign.template_name || $t('campaigns.noTemplate') }}</dd>
                  </div>
                  <div class="flex items-start justify-between gap-4">
                    <dt class="text-muted-foreground">Scheduled</dt>
                    <dd class="text-right">{{ selectedCampaign.scheduled_at ? formatDate(selectedCampaign.scheduled_at) : 'Not scheduled' }}</dd>
                  </div>
                  <div class="flex items-start justify-between gap-4">
                    <dt class="text-muted-foreground">Started</dt>
                    <dd class="text-right">{{ selectedCampaign.started_at ? formatDate(selectedCampaign.started_at) : '-' }}</dd>
                  </div>
                  <div class="flex items-start justify-between gap-4">
                    <dt class="text-muted-foreground">Completed</dt>
                    <dd class="text-right">{{ selectedCampaign.completed_at ? formatDate(selectedCampaign.completed_at) : '-' }}</dd>
                  </div>
                  <div class="flex items-start justify-between gap-4">
                    <dt class="text-muted-foreground">Elapsed</dt>
                    <dd class="text-right">{{ selectedCampaignMetrics?.elapsedLabel || '-' }}</dd>
                  </div>
                  <div class="flex items-start justify-between gap-4">
                    <dt class="text-muted-foreground">Total time</dt>
                    <dd class="text-right">{{ selectedCampaignMetrics?.totalDurationLabel || '-' }}</dd>
                  </div>
                  <div class="flex items-start justify-between gap-4">
                    <dt class="text-muted-foreground">Average speed</dt>
                    <dd class="text-right">{{ selectedCampaignMetrics?.speedLabel || '-' }}</dd>
                  </div>
                  <div class="flex items-start justify-between gap-4">
                    <dt class="text-muted-foreground">Media</dt>
                    <dd class="text-right">{{ selectedCampaign.header_media_filename || 'None' }}</dd>
                  </div>
                </dl>
              </div>
            </div>

            <div class="rounded-xl border p-4">
              <div class="flex items-center justify-between gap-2">
                <h3 class="text-sm font-semibold">Top failure reasons</h3>
                <span v-if="isLoadingRecipients" class="text-xs text-muted-foreground">Refreshing recipient details...</span>
              </div>
              <div v-if="recipientFailureSummary.length" class="mt-4 space-y-3">
                <div
                  v-for="failure in recipientFailureSummary"
                  :key="failure.message"
                  class="flex items-start justify-between gap-4 rounded-lg bg-muted/40 p-3"
                >
                  <p class="text-sm leading-6">{{ failure.message }}</p>
                  <Badge variant="outline" class="shrink-0">{{ failure.count }}</Badge>
                </div>
              </div>
              <div v-else class="mt-4 rounded-lg bg-muted/30 p-4 text-sm text-muted-foreground">
                No failed recipients with error details yet.
              </div>
            </div>
          </div>
        </ScrollArea>

        <DialogFooter>
          <Button variant="outline" size="sm" @click="showCampaignReportDialog = false">{{ $t('common.close') }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- View Recipients Dialog -->
    <Dialog v-model:open="showRecipientsDialog">
      <DialogContent class="sm:max-w-[960px] max-h-[88vh]">
        <DialogHeader>
          <DialogTitle>{{ $t('campaigns.campaignRecipients') }}</DialogTitle>
          <DialogDescription>
            {{ selectedCampaign?.name }} | Showing {{ filteredRecipients.length }} of {{ recipients.length }} recipients
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-4 py-4">
          <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <div class="grid gap-3 sm:grid-cols-2 lg:flex lg:flex-1">
              <Input
                v-model="recipientSearchQuery"
                placeholder="Search phone, name, error, or params"
                class="lg:max-w-sm"
              />
              <Select v-model="recipientFilterStatus">
                <SelectTrigger class="w-full lg:w-[220px]">
                  <SelectValue placeholder="All recipients" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem
                    v-for="option in recipientStatusOptions"
                    :key="option.value"
                    :value="option.value"
                  >
                    {{ option.label }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div class="flex flex-wrap gap-2">
              <Button variant="outline" size="sm" @click="refreshSelectedCampaignData" :disabled="isLoadingRecipients || isLoading">
                <RefreshCw :class="['mr-2 h-4 w-4', isLoadingRecipients || isLoading ? 'animate-spin' : '']" />
                {{ $t('common.refresh') }}
              </Button>
              <Button
                v-if="canCampaignAcceptRecipients(selectedCampaign)"
                variant="outline"
                size="sm"
                @click="showRecipientsDialog = false; openAddRecipientsDialog(selectedCampaign as any)"
              >
                <UserPlus class="h-4 w-4 mr-2" />
                {{ $t('campaigns.addMore') }}
              </Button>
            </div>
          </div>

          <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-6">
            <div class="rounded-xl border bg-muted/30 p-4">
              <p class="text-xs uppercase tracking-wide text-muted-foreground">Total</p>
              <p class="mt-2 text-2xl font-semibold">{{ recipientDashboard.total }}</p>
            </div>
            <div class="rounded-xl border bg-muted/30 p-4">
              <p class="text-xs uppercase tracking-wide text-muted-foreground">Pending</p>
              <p class="mt-2 text-2xl font-semibold text-amber-600">{{ recipientDashboard.pending }}</p>
            </div>
            <div class="rounded-xl border bg-muted/30 p-4">
              <p class="text-xs uppercase tracking-wide text-muted-foreground">Sent</p>
              <p class="mt-2 text-2xl font-semibold">{{ recipientDashboard.sent }}</p>
            </div>
            <div class="rounded-xl border bg-muted/30 p-4">
              <p class="text-xs uppercase tracking-wide text-muted-foreground">Delivered</p>
              <p class="mt-2 text-2xl font-semibold text-green-600">{{ recipientDashboard.delivered }}</p>
            </div>
            <div class="rounded-xl border bg-muted/30 p-4">
              <p class="text-xs uppercase tracking-wide text-muted-foreground">Read</p>
              <p class="mt-2 text-2xl font-semibold text-blue-600">{{ recipientDashboard.read }}</p>
            </div>
            <div class="rounded-xl border bg-muted/30 p-4">
              <p class="text-xs uppercase tracking-wide text-muted-foreground">Failed</p>
              <p class="mt-2 text-2xl font-semibold text-destructive">{{ recipientDashboard.failed }}</p>
            </div>
          </div>

          <div v-if="isLoadingRecipients" class="flex items-center justify-center py-8">
            <Loader2 class="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
          <div v-else-if="recipients.length === 0" class="text-center py-8 text-muted-foreground">
            <Users class="h-12 w-12 mx-auto mb-2 opacity-50" />
            <p>{{ $t('campaigns.noRecipientsYet') }}</p>
            <Button
              v-if="canCampaignAcceptRecipients(selectedCampaign)"
              variant="outline"
              size="sm"
              class="mt-4"
              @click="showRecipientsDialog = false; openAddRecipientsDialog(selectedCampaign as any)"
            >
              <UserPlus class="h-4 w-4 mr-2" />
              {{ $t('campaigns.addRecipients') }}
            </Button>
          </div>
          <div v-else-if="filteredRecipients.length === 0" class="rounded-xl border border-dashed p-8 text-center text-muted-foreground">
            No recipients match the current filters.
          </div>
          <ScrollArea v-else class="h-[420px]">
            <div class="space-y-3 md:hidden">
              <div
                v-for="recipient in filteredRecipients"
                :key="recipient.id"
                class="rounded-xl border p-4"
              >
                <div class="flex items-start justify-between gap-3">
                  <div>
                    <p class="font-mono text-sm">{{ recipient.phone_number }}</p>
                    <p class="mt-1 text-sm text-muted-foreground">{{ recipient.recipient_name || 'No name' }}</p>
                  </div>
                  <Badge variant="outline" :class="getRecipientStatusClass(recipient.status)">
                    {{ formatStatusLabel(recipient.status) }}
                  </Badge>
                </div>
                <div class="mt-3 grid gap-2 text-sm">
                  <div class="flex items-start justify-between gap-4">
                    <span class="text-muted-foreground">{{ getRecipientActivityLabel(recipient) }}</span>
                    <span class="text-right">{{ getRecipientLastActivity(recipient) ? formatDate(getRecipientLastActivity(recipient)) : '-' }}</span>
                  </div>
                  <div v-if="recipient.error_message" class="rounded-lg bg-destructive/5 p-3 text-destructive">
                    {{ recipient.error_message }}
                  </div>
                </div>
                <div v-if="selectedCampaign?.status === 'draft'" class="mt-3 flex justify-end">
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
                </div>
              </div>
            </div>

            <table class="hidden w-full text-sm md:table">
              <thead class="sticky top-0 bg-background border-b">
                <tr>
                  <th class="text-left py-2 px-2">{{ $t('campaigns.phoneNumber') }}</th>
                  <th class="text-left py-2 px-2">{{ $t('campaigns.name') }}</th>
                  <th class="text-left py-2 px-2">{{ $t('campaigns.status') }}</th>
                  <th class="text-left py-2 px-2">Last activity</th>
                  <th class="text-left py-2 px-2">Error</th>
                  <th v-if="selectedCampaign?.status === 'draft'" class="text-center py-2 px-2 w-16"></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="recipient in filteredRecipients" :key="recipient.id" class="border-b align-top">
                  <td class="py-3 px-2 font-mono">{{ recipient.phone_number }}</td>
                  <td class="py-3 px-2">{{ recipient.recipient_name || '-' }}</td>
                  <td class="py-3 px-2">
                    <Badge variant="outline" :class="getRecipientStatusClass(recipient.status)">
                      {{ formatStatusLabel(recipient.status) }}
                    </Badge>
                  </td>
                  <td class="py-3 px-2 text-muted-foreground">
                    <div>{{ getRecipientActivityLabel(recipient) }}</div>
                    <div>{{ getRecipientLastActivity(recipient) ? formatDate(getRecipientLastActivity(recipient)) : '-' }}</div>
                  </td>
                  <td class="py-3 px-2">
                    <span v-if="recipient.error_message" class="block max-w-[280px] text-destructive">
                      {{ recipient.error_message }}
                    </span>
                    <span v-else class="text-muted-foreground">-</span>
                  </td>
                  <td v-if="selectedCampaign?.status === 'draft'" class="py-3 px-2 text-center">
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
          <Button variant="outline" size="sm" @click="showRecipientsDialog = false">{{ $t('common.close') }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Add Recipients Dialog -->
    <Dialog v-model:open="showAddRecipientsDialog">
      <DialogContent class="w-[calc(100vw-2rem)] sm:max-w-[760px] max-h-[calc(100dvh-2rem)] overflow-hidden flex flex-col gap-0 p-0">
        <DialogHeader class="shrink-0 px-6 py-5 pr-12">
          <DialogTitle>{{ $t('campaigns.addRecipients') }}</DialogTitle>
          <DialogDescription>
            {{ $t('campaigns.addRecipientsTo', { name: selectedCampaign?.name }) }}
            <span v-if="templateParamNames.length > 0" class="block mt-1">
              {{ $t('campaigns.templateRequiresParams', { count: templateParamNames.length }) }}
            </span>
          </DialogDescription>
        </DialogHeader>

        <div class="min-h-0 flex-1 overflow-y-auto px-6 pb-4">
          <!-- Template Preview -->
          <div v-if="selectedTemplate?.body_content" class="mb-4 rounded-lg border bg-muted/50">
            <div class="flex items-center gap-2 border-b px-3 py-2">
              <MessageSquare class="h-4 w-4 text-muted-foreground" />
              <span class="text-sm font-medium">{{ $t('campaigns.templatePreview') }}</span>
            </div>
            <div class="max-h-[30dvh] overflow-y-auto px-3 py-3">
              <p class="text-sm whitespace-pre-wrap" v-html="highlightTemplateParams(selectedTemplate.body_content)"></p>
            </div>
          </div>

          <Tabs v-model="addRecipientsTab" class="w-full">
            <TabsList class="grid w-full grid-cols-3">
              <TabsTrigger value="manual" class="min-w-0">
                <UserPlus class="h-4 w-4 mr-2 shrink-0" />
                <span class="truncate">{{ $t('campaigns.manualEntry') }}</span>
              </TabsTrigger>
              <TabsTrigger value="csv" class="min-w-0">
                <FileSpreadsheet class="h-4 w-4 mr-2 shrink-0" />
                <span class="truncate">{{ $t('campaigns.uploadCsv') }}</span>
              </TabsTrigger>
              <TabsTrigger value="contacts" class="min-w-0">
                <Users class="h-4 w-4 mr-2 shrink-0" />
                <span class="truncate">Contacts & Groups</span>
              </TabsTrigger>
            </TabsList>

            <!-- Manual Entry Tab -->
            <TabsContent value="manual" class="mt-4">
            <div class="space-y-4">
              <div class="bg-muted p-3 rounded-lg text-sm">
                <p class="font-medium mb-2">{{ $t('campaigns.formatOneLine') }}</p>
                <code class="bg-background px-2 py-1 rounded block">{{ manualEntryFormat }}</code>
                <p v-if="templateParamNames.length > 0" class="text-muted-foreground mt-2 text-xs">
                  {{ $t('campaigns.templateParameters') }} <span v-for="(param, idx) in templateParamNames" :key="param"><code class="bg-background px-1 rounded">{{ formatParamName(param) }}</code><span v-if="idx < templateParamNames.length - 1">, </span></span>
                </p>
              </div>
              <div class="space-y-2">
                <Label for="recipients">{{ $t('campaigns.recipientsLabel') }}</Label>
                <Textarea
                  id="recipients"
                  v-model="recipientsInput"
                  :placeholder="recipientPlaceholder"
                  :rows="6"
                  class="min-h-[140px] font-mono text-sm"
                  :disabled="isAddingRecipients"
                />
                <!-- Validation status -->
                <div v-if="recipientsInput.trim()" class="space-y-2">
                  <p v-if="manualInputValidation.isValid" class="text-xs text-green-600">
                    {{ $t('campaigns.recipientsValid', { count: manualInputValidation.validLines }) }}
                  </p>
                  <div v-else-if="manualInputValidation.invalidLines.length > 0" class="text-xs">
                    <p class="text-destructive font-medium mb-1">
                      {{ $t('campaigns.linesHaveErrors', { invalid: manualInputValidation.invalidLines.length, total: manualInputValidation.totalLines }) }}
                    </p>
                    <ul class="text-destructive space-y-0.5 max-h-20 overflow-y-auto">
                      <li v-for="err in manualInputValidation.invalidLines.slice(0, 5)" :key="err.lineNumber">
                        {{ $t('campaigns.lineError', { line: err.lineNumber, reason: err.reason }) }}
                      </li>
                      <li v-if="manualInputValidation.invalidLines.length > 5" class="text-muted-foreground">
                        {{ $t('campaigns.andMoreErrors', { count: manualInputValidation.invalidLines.length - 5 }) }}
                      </li>
                    </ul>
                  </div>
                  <p v-else class="text-xs text-muted-foreground">
                    {{ $t('campaigns.recipientsEntered', { count: manualInputValidation.totalLines }) }}
                  </p>
                </div>
              </div>
              <div class="flex justify-end">
                <Button @click="addRecipients" :disabled="isAddingRecipients || !manualInputValidation.isValid">
                  <Loader2 v-if="isAddingRecipients" class="h-4 w-4 mr-2 animate-spin" />
                  <Upload v-else class="h-4 w-4 mr-2" />
                  {{ $t('campaigns.addRecipients') }}
                </Button>
              </div>
            </div>
            </TabsContent>

            <!-- CSV Upload Tab -->
            <TabsContent value="csv" class="mt-4">
            <div class="space-y-4">
              <!-- CSV Format Info -->
              <div class="bg-muted p-3 rounded-lg text-sm">
                <p class="font-medium mb-2">{{ $t('campaigns.requiredCsvColumns') }}</p>
                <div class="flex flex-wrap gap-2">
                  <code v-for="col in csvColumnsHint" :key="col" class="bg-background px-2 py-1 rounded text-xs">{{ col }}</code>
                </div>
                <p v-if="templateParamNames.length > 0" class="text-muted-foreground mt-2 text-xs">
                  {{ $t('campaigns.templateParameters') }} <span v-for="(param, idx) in templateParamNames" :key="param"><code class="bg-background px-1 rounded">{{ formatParamName(param) }}</code><span v-if="idx < templateParamNames.length - 1">, </span></span>
                </p>
              </div>

              <!-- File Upload -->
              <div class="space-y-2">
                <Label for="csv-file">{{ $t('campaigns.selectCsvFile') }}</Label>
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
                <span class="ml-2 text-muted-foreground">{{ $t('campaigns.validatingCsv') }}</span>
              </div>

              <div v-else-if="csvValidation" class="space-y-4">
                <!-- Global Errors -->
                <div v-if="csvValidation.errors.length > 0" class="bg-destructive/10 border border-destructive/20 rounded-lg p-3">
                  <div class="flex items-center gap-2 text-destructive font-medium mb-2">
                    <AlertTriangle class="h-4 w-4" />
                    {{ $t('campaigns.validationErrors') }}
                  </div>
                  <ul class="list-disc list-inside text-sm text-destructive">
                    <li v-for="error in csvValidation.errors" :key="error">{{ error }}</li>
                  </ul>
                </div>

                <!-- Warnings -->
                <div v-if="csvValidation.warnings && csvValidation.warnings.length > 0" class="bg-orange-500/10 border border-orange-500/20 rounded-lg p-3">
                  <div class="flex items-center gap-2 text-orange-600 font-medium mb-2">
                    <AlertTriangle class="h-4 w-4" />
                    {{ $t('campaigns.warnings') }}
                  </div>
                  <ul class="list-disc list-inside text-sm text-orange-600">
                    <li v-for="warning in csvValidation.warnings" :key="warning">{{ warning }}</li>
                  </ul>
                </div>

                <!-- Column Mapping Info -->
                <div v-if="csvValidation.columnMapping && csvValidation.columnMapping.length > 0" class="bg-muted/50 border rounded-lg p-3">
                  <div class="text-sm font-medium mb-2">{{ $t('campaigns.columnMapping') }}</div>
                  <div class="flex flex-wrap gap-2">
                    <div
                      v-for="mapping in csvValidation.columnMapping"
                      :key="mapping.paramName"
                      class="text-xs bg-background border rounded px-2 py-1"
                    >
                      <span class="text-muted-foreground">{{ mapping.csvColumn }}</span>
                      <span class="mx-1">-></span>
                      <span class="font-mono text-primary">{{ formatParamName(mapping.paramName) }}</span>
                    </div>
                  </div>
                </div>

                <!-- Summary -->
                <div class="flex flex-wrap items-center gap-4 text-sm">
                  <div class="flex items-center gap-1">
                    <Check class="h-4 w-4 text-green-600" />
                    <span>{{ csvValidation.rows.filter(r => r.isValid).length }} {{ $t('campaigns.valid') }}</span>
                  </div>
                  <div v-if="csvValidation.rows.filter(r => !r.isValid).length > 0" class="flex items-center gap-1">
                    <AlertTriangle class="h-4 w-4 text-destructive" />
                    <span>{{ csvValidation.rows.filter(r => !r.isValid).length }} {{ $t('campaigns.invalid') }}</span>
                  </div>
                  <div v-if="csvValidation.rows.filter(r => r.errors.some(e => e.includes('Duplicate'))).length > 0" class="flex items-center gap-1 text-orange-600">
                    <Users class="h-4 w-4" />
                    <span>{{ csvValidation.rows.filter(r => r.errors.some(e => e.includes('Duplicate'))).length }} {{ $t('campaigns.duplicates') }}</span>
                  </div>
                  <div class="text-muted-foreground">
                    {{ $t('campaigns.columns') }} {{ csvValidation.csvColumns.join(', ') }}
                  </div>
                </div>

                <!-- Preview Table -->
                <div v-if="csvValidation.rows.length > 0" class="border rounded-lg overflow-hidden">
                  <ScrollArea class="h-[200px]">
                    <table class="w-full text-sm">
                      <thead class="sticky top-0 bg-muted border-b">
                        <tr>
                          <th class="text-left py-2 px-3 w-8"></th>
                          <th class="text-left py-2 px-3">{{ $t('campaigns.phone') }}</th>
                          <th class="text-left py-2 px-3">{{ $t('campaigns.name') }}</th>
                          <th class="text-left py-2 px-3">{{ $t('campaigns.parameters') }}</th>
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
                    {{ $t('campaigns.showingFirst', { count: 50, total: csvValidation.rows.length }) }}
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
                    {{ $t('campaigns.importRecipients', { count: csvValidation.rows.filter(r => r.isValid).length }) }}
                  </Button>
                </div>
              </div>

              <!-- Empty state -->
              <div v-else class="text-center py-8 text-muted-foreground">
                <FileSpreadsheet class="h-12 w-12 mx-auto mb-2 opacity-50" />
                <p>{{ $t('campaigns.selectCsvToPreview') }}</p>
              </div>
            </div>
            </TabsContent>

            <TabsContent value="contacts" class="mt-4">
            <div class="space-y-4">
              <div class="rounded-lg border bg-muted/50 p-3 text-sm">
                <p class="font-medium">Import from contacts or contact groups</p>
                <p class="mt-1 text-muted-foreground">
                  Contact groups use contact tags. Matching contacts are deduplicated automatically before recipients are created.
                </p>
                <p v-if="!canImportFromContacts" class="mt-2 text-destructive">
                  This template has variables, so contacts and groups cannot be imported directly yet. Use manual entry or CSV with parameter values.
                </p>
              </div>

              <div class="grid gap-3 sm:grid-cols-3">
                <div class="rounded-lg border p-3">
                  <p class="text-xs uppercase tracking-wide text-muted-foreground">Selected Contacts</p>
                  <p class="mt-1 text-2xl font-semibold">{{ selectedContactCount }}</p>
                </div>
                <div class="rounded-lg border p-3">
                  <p class="text-xs uppercase tracking-wide text-muted-foreground">Selected Groups</p>
                  <p class="mt-1 text-2xl font-semibold">{{ selectedContactGroupCount }}</p>
                </div>
                <div class="rounded-lg border p-3">
                  <p class="text-xs uppercase tracking-wide text-muted-foreground">Loaded Contacts</p>
                  <p class="mt-1 text-2xl font-semibold">{{ campaignContacts.length }}</p>
                </div>
              </div>

              <div class="space-y-2">
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <Label>Contact Groups</Label>
                  <span class="text-xs text-muted-foreground">Tag-based groups</span>
                </div>
                <div v-if="isLoadingContactGroups" class="flex items-center gap-2 rounded-lg border p-3 text-sm text-muted-foreground">
                  <Loader2 class="h-4 w-4 animate-spin" />
                  Loading contact groups...
                </div>
                <div v-else-if="availableContactGroups.length === 0" class="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
                  No contact groups found.
                </div>
                <div v-else class="flex flex-wrap gap-2">
                  <Button
                    v-for="group in availableContactGroups"
                    :key="group.name"
                    type="button"
                    size="sm"
                    :variant="isCampaignContactGroupSelected(group.name) ? 'default' : 'outline'"
                    @click="toggleCampaignContactGroup(group.name)"
                  >
                    <span class="mr-2 inline-block h-2.5 w-2.5 rounded-full" :style="{ backgroundColor: group.color || '#94a3b8' }"></span>
                    {{ group.name }}
                  </Button>
                </div>
              </div>

              <div class="space-y-2">
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <Label for="campaign-contact-search">Contacts</Label>
                  <span class="text-xs text-muted-foreground">Showing up to 50 matches</span>
                </div>
                <Input
                  id="campaign-contact-search"
                  v-model="contactSearchQuery"
                  placeholder="Search by contact name or phone number"
                  :disabled="!canImportFromContacts || isLoadingCampaignContacts || isAddingRecipients"
                />
                <div v-if="isLoadingCampaignContacts" class="flex items-center gap-2 rounded-lg border p-3 text-sm text-muted-foreground">
                  <Loader2 class="h-4 w-4 animate-spin" />
                  Loading contacts...
                </div>
                <div v-else-if="campaignContacts.length === 0" class="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
                  No contacts found for the current search.
                </div>
                <div v-else class="max-h-[280px] space-y-2 overflow-y-auto pr-1">
                  <button
                    v-for="contact in campaignContacts"
                    :key="contact.id"
                    type="button"
                    class="flex w-full items-start justify-between gap-3 rounded-lg border p-3 text-left transition-colors"
                    :class="isCampaignContactSelected(contact.id) ? 'border-primary bg-primary/5' : 'hover:bg-muted/50'"
                    @click="toggleCampaignContact(contact.id)"
                  >
                    <div class="min-w-0 space-y-1">
                      <div class="font-medium">
                        {{ contact.profile_name || contact.name || contact.phone_number }}
                      </div>
                      <div class="font-mono text-xs text-muted-foreground">
                        {{ contact.phone_number }}
                      </div>
                      <div v-if="contact.tags && contact.tags.length" class="flex flex-wrap gap-1">
                        <Badge v-for="tag in contact.tags" :key="`${contact.id}-${tag}`" variant="secondary" class="text-[10px]">
                          {{ tag }}
                        </Badge>
                      </div>
                    </div>
                    <CheckCircle v-if="isCampaignContactSelected(contact.id)" class="mt-0.5 h-4 w-4 shrink-0 text-primary" />
                    <Users v-else class="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                  </button>
                </div>
              </div>

              <div class="flex flex-col gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between">
                <p class="text-sm text-muted-foreground">
                  Selected {{ selectedContactCount }} contacts and {{ selectedContactGroupCount }} groups.
                </p>
                <Button
                  @click="addRecipientsFromContacts"
                  :disabled="isAddingRecipients || !canImportFromContacts || (selectedContactCount === 0 && selectedContactGroupCount === 0)"
                >
                  <Loader2 v-if="isAddingRecipients" class="h-4 w-4 mr-2 animate-spin" />
                  <Upload v-else class="h-4 w-4 mr-2" />
                  Add from Contacts
                </Button>
              </div>
            </div>
            </TabsContent>
          </Tabs>
        </div>

        <DialogFooter class="shrink-0 border-t px-6 py-4">
          <Button variant="outline" size="sm" @click="showAddRecipientsDialog = false" :disabled="isAddingRecipients">
            {{ $t('common.cancel') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <DeleteConfirmDialog
      v-model:open="deleteDialogOpen"
      :title="$t('campaigns.deleteCampaign')"
      :item-name="campaignToDelete?.name"
      @confirm="confirmDeleteCampaign"
    />

    <!-- Cancel Confirmation Dialog -->
    <AlertDialog v-model:open="cancelDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('campaigns.cancelConfirmTitle') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('campaigns.cancelConfirmDesc', { name: campaignToCancel?.name }) }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('campaigns.keepRunning') }}</AlertDialogCancel>
          <AlertDialogAction @click="confirmCancelCampaign">{{ $t('campaigns.cancelCampaign') }}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <!-- Media Preview Dialog -->
    <Dialog v-model:open="showMediaPreviewDialog">
      <DialogContent class="sm:max-w-[600px]">
        <DialogHeader>
          <DialogTitle>{{ $t('campaigns.mediaPreview') }}</DialogTitle>
          <DialogDescription>
            {{ previewingCampaign?.header_media_filename }}
            <span v-if="previewingCampaign?.header_media_mime_type" class="text-xs"> ({{ previewingCampaign.header_media_mime_type }})</span>
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
          <div v-else class="flex flex-col items-center gap-3 py-6 text-muted-foreground">
            <FileText class="h-16 w-16" />
            <span class="text-sm font-medium">{{ previewingCampaign?.header_media_filename }}</span>
          </div>
        </div>
        <DialogFooter>
          <Button
            v-if="previewingCampaign?.status === 'draft'"
            variant="outline"
            @click="showMediaPreviewDialog = false; triggerMediaUpload(previewingCampaign!)"
          >
            {{ $t('campaigns.replaceMedia') }}
          </Button>
          <Button variant="outline" @click="showMediaPreviewDialog = false">{{ $t('common.close') }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
    <!-- Hidden file input for standalone media upload from table -->
    <input id="campaign-media-upload" type="file" accept="image/jpeg,image/png,image/webp,video/mp4,video/3gpp,.pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx" class="hidden" @change="handleStandaloneMediaUpload" />
  </div>
</template>
