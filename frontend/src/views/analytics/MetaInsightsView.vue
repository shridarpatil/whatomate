<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import DataTable from '@/components/shared/DataTable.vue'
import type { Column } from '@/components/shared/DataTable.vue'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { RangeCalendar } from '@/components/ui/range-calendar'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import {
  metaAnalyticsService,
  type MetaAnalyticsType,
  type MetaGranularity,
  type MetaAnalyticsAccount,
  type MetaAnalyticsResponse,
  type MetaMessagingDataPoint,
  type MetaPricingDataPoint,
  type MetaTemplateDataPoint,
  type MetaCallDataPoint
} from '@/services/api'
import { PageHeader } from '@/components/shared'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList
} from '@/components/ui/command'
import {
  BarChart3,
  CalendarIcon,
  ChevronsUpDown,
  Check,
  MessageSquare,
  MessagesSquare,
  DollarSign,
  FileText,
  Phone,
  RefreshCw,
  TrendingUp,
  Send,
  CheckCircle,
  Eye,
  MousePointerClick,
  Search
} from 'lucide-vue-next'
import type { DateRange } from 'reka-ui'
import { CalendarDate } from '@internationalized/date'
import { Line, Bar } from '@/lib/charts'
import { useToast } from '@/components/ui/toast'

const { toast } = useToast()
const { t } = useI18n()

// State
const isLoading = ref(true)
const isRefreshing = ref(false)
const accounts = ref<MetaAnalyticsAccount[]>([])
const selectedAccountId = ref<string>('all')
const accountComboboxOpen = ref(false)
const activeTab = ref<MetaAnalyticsType>('analytics')
const analyticsData = ref<MetaAnalyticsResponse[]>([])
const isCached = ref(false)

// Granularity
const selectedGranularity = ref<MetaGranularity>('DAY')

// Time range filter
type TimeRangePreset = 'today' | '7days' | '30days' | 'this_month' | 'custom'

const loadSavedPreferences = () => {
  const savedRange = localStorage.getItem('meta_insights_time_range') as TimeRangePreset | null
  const savedCustomRange = localStorage.getItem('meta_insights_custom_range')
  const savedGranularity = localStorage.getItem('meta_insights_granularity') as MetaGranularity | null
  const savedAccountId = localStorage.getItem('meta_insights_account_id')
  const savedActiveTab = localStorage.getItem('meta_insights_active_tab') as MetaAnalyticsType | null

  let customRange: DateRange = { start: undefined, end: undefined }
  if (savedCustomRange) {
    try {
      const parsed = JSON.parse(savedCustomRange)
      if (parsed.start && parsed.end) {
        customRange = {
          start: new CalendarDate(parsed.start.year, parsed.start.month, parsed.start.day),
          end: new CalendarDate(parsed.end.year, parsed.end.month, parsed.end.day)
        }
      }
    } catch {
      // Ignore parse errors
    }
  }

  return {
    range: savedRange || '30days',
    customRange,
    granularity: savedGranularity || 'DAY',
    accountId: savedAccountId || 'all',
    activeTab: savedActiveTab || 'analytics'
  }
}

const savedPrefs = loadSavedPreferences()
const selectedRange = ref<TimeRangePreset>(savedPrefs.range as TimeRangePreset)
const customDateRange = ref<any>(savedPrefs.customRange)
const isDatePickerOpen = ref(false)

// Apply saved preferences
if (savedPrefs.granularity) {
  selectedGranularity.value = savedPrefs.granularity as MetaGranularity
}
if (savedPrefs.accountId) {
  selectedAccountId.value = savedPrefs.accountId
}
if (savedPrefs.activeTab) {
  activeTab.value = savedPrefs.activeTab as MetaAnalyticsType
}

const savePreferences = () => {
  localStorage.setItem('meta_insights_time_range', selectedRange.value)
  localStorage.setItem('meta_insights_granularity', selectedGranularity.value)
  localStorage.setItem('meta_insights_account_id', selectedAccountId.value)
  localStorage.setItem('meta_insights_active_tab', activeTab.value)
  if (selectedRange.value === 'custom' && customDateRange.value.start && customDateRange.value.end) {
    localStorage.setItem('meta_insights_custom_range', JSON.stringify({
      start: {
        year: customDateRange.value.start.year,
        month: customDateRange.value.start.month,
        day: customDateRange.value.start.day
      },
      end: {
        year: customDateRange.value.end.year,
        month: customDateRange.value.end.month,
        day: customDateRange.value.end.day
      }
    }))
  }
}

const selectedAccountName = computed(() => {
  if (selectedAccountId.value === 'all') return t('metaInsights.allAccounts')
  const account = accounts.value.find(a => a.id === selectedAccountId.value)
  return account?.name || t('metaInsights.selectAccount')
})

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

const formatDateRange = computed(() => {
  if (selectedRange.value === 'custom' && customDateRange.value.start && customDateRange.value.end) {
    const start = customDateRange.value.start
    const end = customDateRange.value.end
    const startStr = `${start.month}/${start.day}/${start.year}`
    const endStr = `${end.month}/${end.day}/${end.year}`
    return `${startStr} - ${endStr}`
  }
  return ''
})

const fetchAccounts = async () => {
  try {
    const response = await metaAnalyticsService.getAccounts()
    const data = (response.data as any).data || response.data
    accounts.value = data.accounts || []
  } catch (error) {
    console.error('Failed to load accounts:', error)
  }
}

const fetchAnalytics = async () => {
  isLoading.value = true
  try {
    const { from, to } = getDateRange.value
    const params: {
      analytics_type: MetaAnalyticsType
      start: string
      end: string
      granularity: MetaGranularity
      account_id?: string
    } = {
      analytics_type: activeTab.value,
      start: from,
      end: to,
      granularity: selectedGranularity.value
    }

    if (selectedAccountId.value !== 'all') {
      params.account_id = selectedAccountId.value
    }

    const response = await metaAnalyticsService.get(params)
    const data = (response.data as any).data || response.data
    analyticsData.value = data.accounts || []
    isCached.value = data.cached || false
  } catch (error) {
    console.error('Failed to load analytics:', error)
    analyticsData.value = []
  } finally {
    isLoading.value = false
  }
}

const refreshCache = async () => {
  isRefreshing.value = true
  try {
    await metaAnalyticsService.refresh()
    toast({
      title: t('metaInsights.cacheCleared'),
      description: t('metaInsights.cacheRefreshed')
    })
    await fetchAnalytics()
  } catch (error) {
    console.error('Failed to refresh cache:', error)
    toast({
      title: t('common.error'),
      description: t('metaInsights.refreshFailed'),
      variant: 'destructive'
    })
  } finally {
    isRefreshing.value = false
  }
}

const applyCustomRange = () => {
  if (customDateRange.value.start && customDateRange.value.end) {
    isDatePickerOpen.value = false
    savePreferences()
    fetchAnalytics()
  }
}

watch(selectedRange, (newValue) => {
  savePreferences()
  if (newValue !== 'custom') {
    fetchAnalytics()
  }
})

watch(selectedGranularity, () => {
  savePreferences()
  fetchAnalytics()
})

watch(selectedAccountId, () => {
  savePreferences()
  fetchAnalytics()
})

watch(activeTab, () => {
  savePreferences()
  fetchAnalytics()
})

onMounted(() => {
  fetchAccounts()
  fetchAnalytics()
})

// Aggregate data across accounts
const aggregatedData = computed(() => {
  if (!analyticsData.value.length) return null

  // For messaging analytics
  if (activeTab.value === 'analytics') {
    const allPoints: MetaMessagingDataPoint[] = []
    for (const account of analyticsData.value) {
      if (account.data?.analytics?.data_points) {
        allPoints.push(...account.data.analytics.data_points)
      }
    }
    return aggregateMessagingData(allPoints)
  }

  // For pricing analytics
  if (activeTab.value === 'pricing_analytics') {
    const allPoints: MetaPricingDataPoint[] = []
    for (const account of analyticsData.value) {
      if (account.data?.pricing_analytics?.data_points) {
        allPoints.push(...account.data.pricing_analytics.data_points)
      }
    }
    return aggregatePricingData(allPoints)
  }

  // For template analytics
  if (activeTab.value === 'template_analytics') {
    const allPoints: MetaTemplateDataPoint[] = []
    for (const account of analyticsData.value) {
      if (account.data?.template_analytics?.data_points) {
        allPoints.push(...account.data.template_analytics.data_points)
      }
    }
    return aggregateTemplateData(allPoints)
  }

  // For call analytics
  if (activeTab.value === 'call_analytics') {
    const allPoints: MetaCallDataPoint[] = []
    for (const account of analyticsData.value) {
      if (account.data?.call_analytics?.data_points) {
        allPoints.push(...account.data.call_analytics.data_points)
      }
    }
    return aggregateCallData(allPoints)
  }

  return null
})

// Aggregation functions
function aggregateMessagingData(points: MetaMessagingDataPoint[]) {
  const byTime = new Map<number, { sent: number; delivered: number }>()
  let totalSent = 0
  let totalDelivered = 0

  for (const point of points) {
    totalSent += point.sent
    totalDelivered += point.delivered

    const existing = byTime.get(point.start) || { sent: 0, delivered: 0 }
    byTime.set(point.start, {
      sent: existing.sent + point.sent,
      delivered: existing.delivered + point.delivered
    })
  }

  const sortedTimes = Array.from(byTime.entries()).sort((a, b) => a[0] - b[0])

  return {
    totals: { sent: totalSent, delivered: totalDelivered },
    timeSeries: sortedTimes.map(([time, data]) => ({
      time,
      ...data
    }))
  }
}

function aggregatePricingData(points: MetaPricingDataPoint[]) {
  // Aggregate by category, type, and country
  const byCategory = new Map<string, { volume: number; cost: number }>()
  const byType = new Map<string, { volume: number; cost: number }>()
  const byCountry = new Map<string, { volume: number; cost: number }>()

  // Track free vs paid
  const freeMessages = { total: 0, customerService: 0, entryPoint: 0 }
  const paidMessages = { total: 0, byCategory: new Map<string, number>() }
  const costByCategory = new Map<string, number>()

  let totalVolume = 0
  let totalCost = 0

  for (const point of points) {
    const volume = point.volume || 0
    const cost = point.cost || 0
    totalVolume += volume
    totalCost += cost

    // By pricing category (MARKETING, UTILITY, AUTHENTICATION, SERVICE)
    const category = point.pricing_category || 'OTHER'
    const catData = byCategory.get(category) || { volume: 0, cost: 0 }
    byCategory.set(category, { volume: catData.volume + volume, cost: catData.cost + cost })
    costByCategory.set(category, (costByCategory.get(category) || 0) + cost)

    // By pricing type (FREE_CUSTOMER_SERVICE, FREE_ENTRY_POINT, REGULAR)
    const pricingType = point.pricing_type || 'OTHER'
    const typeData = byType.get(pricingType) || { volume: 0, cost: 0 }
    byType.set(pricingType, { volume: typeData.volume + volume, cost: typeData.cost + cost })

    // Free vs Paid breakdown
    if (pricingType === 'FREE_CUSTOMER_SERVICE') {
      freeMessages.total += volume
      freeMessages.customerService += volume
    } else if (pricingType === 'FREE_ENTRY_POINT') {
      freeMessages.total += volume
      freeMessages.entryPoint += volume
    } else {
      paidMessages.total += volume
      paidMessages.byCategory.set(category, (paidMessages.byCategory.get(category) || 0) + volume)
    }

    // By country
    const country = point.country || 'UNKNOWN'
    const countryData = byCountry.get(country) || { volume: 0, cost: 0 }
    byCountry.set(country, { volume: countryData.volume + volume, cost: countryData.cost + cost })
  }

  return {
    totals: { volume: totalVolume, cost: totalCost },
    byCategory: Object.fromEntries(byCategory),
    byType: Object.fromEntries(byType),
    byCountry: Object.fromEntries(byCountry),
    freeMessages,
    paidMessages: {
      total: paidMessages.total,
      byCategory: Object.fromEntries(paidMessages.byCategory)
    },
    costByCategory: Object.fromEntries(costByCategory)
  }
}

function aggregateTemplateData(points: MetaTemplateDataPoint[]) {
  const byTemplate = new Map<string, { sent: number; delivered: number; read: number; replied: number; clicked: number; cost: number }>()
  let totalSent = 0
  let totalDelivered = 0
  let totalRead = 0
  let totalReplied = 0
  let totalClicked = 0
  let totalCost = 0

  for (const point of points) {
    totalSent += point.sent
    totalDelivered += point.delivered
    totalRead += point.read
    totalReplied += point.replied || 0

    // Sum all clicked counts from the clicked array
    const clickedCount = point.clicked?.reduce((sum, c) => sum + c.count, 0) || 0
    totalClicked += clickedCount

    // Extract amount_spent from cost array (using 'value' field per Meta API)
    const amountSpent = point.cost?.find(c => c.type === 'amount_spent')?.value || 0
    totalCost += amountSpent

    const templateId = point.template_id
    const existing = byTemplate.get(templateId) || { sent: 0, delivered: 0, read: 0, replied: 0, clicked: 0, cost: 0 }
    byTemplate.set(templateId, {
      sent: existing.sent + point.sent,
      delivered: existing.delivered + point.delivered,
      read: existing.read + point.read,
      replied: existing.replied + (point.replied || 0),
      clicked: existing.clicked + clickedCount,
      cost: existing.cost + amountSpent
    })
  }

  return {
    totals: { sent: totalSent, delivered: totalDelivered, read: totalRead, replied: totalReplied, clicked: totalClicked, cost: totalCost },
    byTemplate: Object.fromEntries(byTemplate)
  }
}

// Computed property for template name lookup
const templateNamesMap = computed(() => {
  const names: Record<string, string> = {}
  for (const account of analyticsData.value) {
    if (account.template_names) {
      Object.assign(names, account.template_names)
    }
  }
  return names
})

function getTemplateName(templateId: string): string {
  return templateNamesMap.value[templateId] || templateId
}

// Template table search and sorting
const templateSearchQuery = ref('')
const templateSortKey = ref('sent')
const templateSortDirection = ref<'asc' | 'desc'>('desc')

// Template table columns definition
const templateColumns = computed<Column<any>[]>(() => [
  { key: 'name', label: t('metaInsights.template'), sortable: true },
  { key: 'sent', label: t('metaInsights.sent'), align: 'right', sortable: true },
  { key: 'delivered', label: t('metaInsights.delivered'), align: 'right', sortable: true },
  { key: 'read', label: t('metaInsights.read'), align: 'right', sortable: true },
  { key: 'replied', label: t('metaInsights.replied'), align: 'right', sortable: true },
  { key: 'clicked', label: t('metaInsights.clicked'), align: 'right', sortable: true },
  { key: 'deliveryRate', label: t('metaInsights.deliveryPercent'), align: 'right', sortable: true },
  { key: 'readRate', label: t('metaInsights.readPercent'), align: 'right', sortable: true },
  { key: 'cost', label: t('metaInsights.cost'), align: 'right', sortable: true },
])

const filteredTemplateData = computed(() => {
  if (!aggregatedData.value || activeTab.value !== 'template_analytics') {
    return []
  }

  const data = aggregatedData.value as ReturnType<typeof aggregateTemplateData>
  let entries = Object.entries(data.byTemplate).map(([templateId, stats]) => ({
    id: templateId,
    templateId,
    name: getTemplateName(templateId),
    ...stats,
    deliveryRate: stats.sent > 0 ? (stats.delivered / stats.sent * 100) : 0,
    readRate: stats.delivered > 0 ? (stats.read / stats.delivered * 100) : 0
  }))

  // Filter by search query
  if (templateSearchQuery.value) {
    const query = templateSearchQuery.value.toLowerCase()
    entries = entries.filter(e =>
      e.name.toLowerCase().includes(query) ||
      e.templateId.toLowerCase().includes(query)
    )
  }

  return entries
})

function aggregateCallData(points: MetaCallDataPoint[]) {
  const byType = new Map<string, number>()
  const byDirection = new Map<string, number>()
  let totalCalls = 0
  let totalDuration = 0

  for (const point of points) {
    totalCalls += point.total_calls
    totalDuration += point.call_duration

    const callType = point.call_type || 'UNKNOWN'
    byType.set(callType, (byType.get(callType) || 0) + point.total_calls)

    const direction = point.call_direction || 'UNKNOWN'
    byDirection.set(direction, (byDirection.get(direction) || 0) + point.total_calls)
  }

  return {
    totals: { calls: totalCalls, duration: totalDuration },
    byType: Object.fromEntries(byType),
    byDirection: Object.fromEntries(byDirection)
  }
}

// Chart data
const messagingChartData = computed(() => {
  if (!aggregatedData.value || activeTab.value !== 'analytics') {
    return { labels: [], datasets: [] }
  }

  const data = aggregatedData.value as ReturnType<typeof aggregateMessagingData>

  return {
    labels: data.timeSeries.map(t => formatTimestamp(t.time)),
    datasets: [
      {
        label: t('metaInsights.sent'),
        data: data.timeSeries.map(d => d.sent),
        borderColor: 'rgb(59, 130, 246)',
        backgroundColor: 'rgba(59, 130, 246, 0.1)',
        fill: true,
        tension: 0.3
      },
      {
        label: t('metaInsights.delivered'),
        data: data.timeSeries.map(d => d.delivered),
        borderColor: 'rgb(16, 185, 129)',
        backgroundColor: 'rgba(16, 185, 129, 0.1)',
        fill: true,
        tension: 0.3
      }
    ]
  }
})

const pricingChartData = computed(() => {
  if (!aggregatedData.value || activeTab.value !== 'pricing_analytics') {
    return { labels: [], datasets: [] }
  }

  const data = aggregatedData.value as ReturnType<typeof aggregatePricingData>
  const categories = Object.entries(data.byCategory)

  return {
    labels: categories.map(([cat]) => formatCategory(cat)),
    datasets: [
      {
        label: t('metaInsights.messaging'),
        data: categories.map(([, val]) => val.volume),
        backgroundColor: 'rgba(59, 130, 246, 0.8)'
      },
      {
        label: t('metaInsights.cost'),
        data: categories.map(([, val]) => val.cost),
        backgroundColor: 'rgba(16, 185, 129, 0.8)'
      }
    ]
  }
})

// Helper functions
function formatTimestamp(ts: number): string {
  const date = new Date(ts * 1000)
  if (selectedGranularity.value === 'HALF_HOUR') {
    return date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })
  } else if (selectedGranularity.value === 'MONTH') {
    return date.toLocaleDateString('en-US', { month: 'short', year: 'numeric' })
  }
  return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}

function formatCategory(category: string): string {
  return category.charAt(0) + category.slice(1).toLowerCase().replace(/_/g, ' ')
}

function formatCurrency(value: number): string {
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(value)
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  if (mins < 60) return secs > 0 ? `${mins}m ${secs}s` : `${mins}m`
  const hours = Math.floor(mins / 60)
  const remainingMins = mins % 60
  return remainingMins > 0 ? `${hours}h ${remainingMins}m` : `${hours}h`
}

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: {
      position: 'bottom' as const
    }
  },
  scales: {
    y: {
      beginAtZero: true
    }
  }
}


</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader
      :title="$t('metaInsights.title')"
      :description="$t('metaInsights.subtitle')"
      :icon="BarChart3"
      icon-gradient="bg-gradient-to-br from-green-500 to-emerald-600 shadow-green-500/20"
    >
      <template #actions>
        <!-- Account Filter -->
        <div class="flex items-center gap-2">
          <Popover v-model:open="accountComboboxOpen">
            <PopoverTrigger as-child>
              <Button variant="outline" role="combobox" :aria-expanded="accountComboboxOpen" class="w-[180px] justify-between">
                <span class="truncate">{{ selectedAccountName }}</span>
                <ChevronsUpDown class="ml-2 h-4 w-4 shrink-0 opacity-50" />
              </Button>
            </PopoverTrigger>
            <PopoverContent class="w-[180px] p-0">
              <Command>
                <CommandInput :placeholder="$t('metaInsights.searchAccount')" />
                <CommandList>
                  <CommandEmpty>{{ $t('metaInsights.noAccountFound') }}</CommandEmpty>
                  <CommandGroup>
                    <CommandItem
                      value="all"
                      @select="() => { selectedAccountId = 'all'; accountComboboxOpen = false }"
                    >
                      <Check :class="['mr-2 h-4 w-4', selectedAccountId === 'all' ? 'opacity-100' : 'opacity-0']" />
                      {{ $t('metaInsights.allAccounts') }}
                    </CommandItem>
                    <CommandItem
                      v-for="account in accounts"
                      :key="account.id"
                      :value="account.name"
                      @select="() => { selectedAccountId = account.id; accountComboboxOpen = false }"
                    >
                      <Check :class="['mr-2 h-4 w-4', selectedAccountId === account.id ? 'opacity-100' : 'opacity-0']" />
                      {{ account.name }}
                    </CommandItem>
                  </CommandGroup>
                </CommandList>
              </Command>
            </PopoverContent>
          </Popover>
        </div>

        <!-- Granularity Filter -->
        <Select v-model="selectedGranularity">
          <SelectTrigger class="w-[130px]">
            <SelectValue :placeholder="$t('metaInsights.granularity')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="HALF_HOUR">{{ $t('metaInsights.halfHour') }}</SelectItem>
            <SelectItem value="DAY">{{ $t('metaInsights.daily') }}</SelectItem>
            <SelectItem value="MONTH">{{ $t('metaInsights.monthly') }}</SelectItem>
          </SelectContent>
        </Select>

        <!-- Time Range Filter -->
        <Select v-model="selectedRange">
          <SelectTrigger class="w-[150px]">
            <SelectValue :placeholder="$t('metaInsights.selectRange')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="today">{{ $t('metaInsights.today') }}</SelectItem>
            <SelectItem value="7days">{{ $t('metaInsights.last7Days') }}</SelectItem>
            <SelectItem value="30days">{{ $t('metaInsights.last30Days') }}</SelectItem>
            <SelectItem value="this_month">{{ $t('metaInsights.thisMonth') }}</SelectItem>
            <SelectItem value="custom">{{ $t('metaInsights.customRange') }}</SelectItem>
          </SelectContent>
        </Select>

        <Popover v-if="selectedRange === 'custom'" v-model:open="isDatePickerOpen">
          <PopoverTrigger as-child>
            <Button variant="outline" class="w-auto">
              <CalendarIcon class="h-4 w-4 mr-2" />
              {{ formatDateRange || $t('metaInsights.selectDates') }}
            </Button>
          </PopoverTrigger>
          <PopoverContent class="w-auto p-4" align="end">
            <div class="space-y-4">
              <RangeCalendar v-model="customDateRange" :number-of-months="2" />
              <Button class="w-full" @click="applyCustomRange" :disabled="!customDateRange.start || !customDateRange.end">
                {{ $t('metaInsights.applyRange') }}
              </Button>
            </div>
          </PopoverContent>
        </Popover>

        <!-- Refresh Button -->
        <Button variant="outline" size="icon" @click="refreshCache" :disabled="isRefreshing">
          <RefreshCw :class="['h-4 w-4', isRefreshing && 'animate-spin']" />
        </Button>

        <!-- Cache indicator -->
        <Badge v-if="isCached" variant="secondary" class="ml-2">
          {{ $t('metaInsights.cached') }}
        </Badge>
      </template>
    </PageHeader>

    <!-- Content -->
    <ScrollArea class="flex-1">
      <div class="p-6 space-y-6">
        <!-- Analytics Type Tabs -->
        <Tabs v-model="activeTab" class="w-full">
          <TabsList class="grid w-full grid-cols-4 lg:w-auto lg:inline-flex">
            <TabsTrigger value="analytics">
              <MessageSquare class="h-4 w-4 lg:mr-2" />
              <span class="hidden lg:inline">{{ $t('metaInsights.messaging') }}</span>
            </TabsTrigger>
            <TabsTrigger value="pricing_analytics">
              <DollarSign class="h-4 w-4 lg:mr-2" />
              <span class="hidden lg:inline">{{ $t('metaInsights.pricing') }}</span>
            </TabsTrigger>
            <TabsTrigger value="template_analytics">
              <FileText class="h-4 w-4 lg:mr-2" />
              <span class="hidden lg:inline">{{ $t('metaInsights.templates') }}</span>
            </TabsTrigger>
            <TabsTrigger value="call_analytics">
              <Phone class="h-4 w-4 lg:mr-2" />
              <span class="hidden lg:inline">{{ $t('metaInsights.calls') }}</span>
            </TabsTrigger>
          </TabsList>

          <!-- Messaging Analytics -->
          <TabsContent value="analytics" class="space-y-6">
            <template v-if="isLoading">
              <div class="grid gap-4 md:grid-cols-3">
                <div v-for="i in 3" :key="i" class="rounded-xl border border-white/[0.08] bg-white/[0.02] p-6 light:bg-white light:border-gray-200">
                  <Skeleton class="h-4 w-24 mb-2 bg-white/[0.08] light:bg-gray-200" />
                  <Skeleton class="h-8 w-16 bg-white/[0.08] light:bg-gray-200" />
                </div>
              </div>
            </template>
            <template v-else-if="aggregatedData && activeTab === 'analytics'">
              <!-- Stats Cards -->
              <div class="grid gap-4 md:grid-cols-3">
                <div class="card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] p-6 light:bg-white light:border-gray-200">
                  <div class="flex flex-row items-center justify-between space-y-0 pb-2">
                    <span class="text-sm font-medium text-white/50 light:text-gray-500">{{ $t('metaInsights.messagesSent') }}</span>
                    <div class="h-10 w-10 rounded-lg bg-blue-500/20 flex items-center justify-center">
                      <Send class="h-5 w-5 text-blue-400" />
                    </div>
                  </div>
                  <div class="pt-2">
                    <div class="text-3xl font-bold text-white light:text-gray-900">
                      {{ (aggregatedData as ReturnType<typeof aggregateMessagingData>).totals.sent.toLocaleString() }}
                    </div>
                  </div>
                </div>

                <div class="card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] p-6 light:bg-white light:border-gray-200">
                  <div class="flex flex-row items-center justify-between space-y-0 pb-2">
                    <span class="text-sm font-medium text-white/50 light:text-gray-500">{{ $t('metaInsights.messagesDelivered') }}</span>
                    <div class="h-10 w-10 rounded-lg bg-emerald-500/20 flex items-center justify-center">
                      <CheckCircle class="h-5 w-5 text-emerald-400" />
                    </div>
                  </div>
                  <div class="pt-2">
                    <div class="text-3xl font-bold text-white light:text-gray-900">
                      {{ (aggregatedData as ReturnType<typeof aggregateMessagingData>).totals.delivered.toLocaleString() }}
                    </div>
                  </div>
                </div>

                <div class="card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] p-6 light:bg-white light:border-gray-200">
                  <div class="flex flex-row items-center justify-between space-y-0 pb-2">
                    <span class="text-sm font-medium text-white/50 light:text-gray-500">{{ $t('metaInsights.deliveryRate') }}</span>
                    <div class="h-10 w-10 rounded-lg bg-purple-500/20 flex items-center justify-center">
                      <TrendingUp class="h-5 w-5 text-purple-400" />
                    </div>
                  </div>
                  <div class="pt-2">
                    <div class="text-3xl font-bold text-white light:text-gray-900">
                      {{ ((aggregatedData as ReturnType<typeof aggregateMessagingData>).totals.sent > 0
                        ? ((aggregatedData as ReturnType<typeof aggregateMessagingData>).totals.delivered / (aggregatedData as ReturnType<typeof aggregateMessagingData>).totals.sent * 100).toFixed(1)
                        : 0) }}%
                    </div>
                  </div>
                </div>
              </div>

              <!-- Chart -->
              <Card>
                <CardHeader>
                  <CardTitle>{{ $t('metaInsights.messageDeliveryOverTime') }}</CardTitle>
                  <CardDescription>{{ $t('metaInsights.sentVsDelivered') }}</CardDescription>
                </CardHeader>
                <CardContent>
                  <div class="h-80">
                    <Line v-if="messagingChartData.labels.length > 0" :data="messagingChartData" :options="chartOptions" />
                    <div v-else class="h-full flex items-center justify-center text-muted-foreground">
                      {{ $t('metaInsights.noDataForPeriod') }}
                    </div>
                  </div>
                </CardContent>
              </Card>
            </template>
            <template v-else>
              <div class="text-center py-12 text-muted-foreground">
                {{ $t('metaInsights.noMessagingData') }}
              </div>
            </template>
          </TabsContent>
          <!-- Pricing Analytics -->
          <TabsContent value="pricing_analytics" class="space-y-6">
            <template v-if="isLoading">
              <Skeleton class="h-64 bg-white/[0.08] light:bg-gray-200" />
            </template>
            <template v-else-if="aggregatedData && activeTab === 'pricing_analytics'">
              <!-- Stats Cards -->
              <div class="grid gap-4 md:grid-cols-2">
                <div class="card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] p-6 light:bg-white light:border-gray-200">
                  <div class="flex flex-row items-center justify-between space-y-0 pb-2">
                    <span class="text-sm font-medium text-white/50 light:text-gray-500">{{ $t('metaInsights.totalMessages') }}</span>
                    <div class="h-10 w-10 rounded-lg bg-blue-500/20 flex items-center justify-center">
                      <MessagesSquare class="h-5 w-5 text-blue-400" />
                    </div>
                  </div>
                  <div class="pt-2">
                    <div class="text-3xl font-bold text-white light:text-gray-900">
                      {{ (aggregatedData as ReturnType<typeof aggregatePricingData>).totals.volume.toLocaleString() }}
                    </div>
                  </div>
                </div>

                <div class="card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] p-6 light:bg-white light:border-gray-200">
                  <div class="flex flex-row items-center justify-between space-y-0 pb-2">
                    <span class="text-sm font-medium text-white/50 light:text-gray-500">{{ $t('metaInsights.totalCost') }}</span>
                    <div class="h-10 w-10 rounded-lg bg-emerald-500/20 flex items-center justify-center">
                      <DollarSign class="h-5 w-5 text-emerald-400" />
                    </div>
                  </div>
                  <div class="pt-2">
                    <div class="text-3xl font-bold text-white light:text-gray-900">
                      {{ formatCurrency((aggregatedData as ReturnType<typeof aggregatePricingData>).totals.cost) }}
                    </div>
                  </div>
                </div>
              </div>

              <!-- Chart -->
              <Card>
                <CardHeader>
                  <CardTitle>{{ $t('metaInsights.messagesCostByCategory') }}</CardTitle>
                  <CardDescription>{{ $t('metaInsights.breakdownByCategory') }}</CardDescription>
                </CardHeader>
                <CardContent>
                  <div class="h-80">
                    <Bar v-if="pricingChartData.labels.length > 0" :data="pricingChartData" :options="chartOptions" />
                    <div v-else class="h-full flex items-center justify-center text-muted-foreground">
                      {{ $t('metaInsights.noDataForPeriod') }}
                    </div>
                  </div>
                </CardContent>
              </Card>

              <!-- Detailed Breakdown -->
              <div class="grid gap-6 lg:grid-cols-2">
                <!-- Free Messages Breakdown -->
                <Card>
                  <CardHeader>
                    <CardTitle>{{ $t('metaInsights.freeMessages') }}</CardTitle>
                    <CardDescription>{{ $t('metaInsights.freeTierBreakdown') }}</CardDescription>
                  </CardHeader>
                  <CardContent>
                    <div class="space-y-3">
                      <div class="flex items-center justify-between py-2 border-b border-white/[0.08] light:border-gray-100">
                        <span class="text-sm text-white/70 light:text-gray-600">{{ $t('metaInsights.freeCustomerService') }}</span>
                        <span class="font-semibold text-white light:text-gray-900">{{ (aggregatedData as ReturnType<typeof aggregatePricingData>).freeMessages.customerService.toLocaleString() }}</span>
                      </div>
                      <div class="flex items-center justify-between py-2 border-b border-white/[0.08] light:border-gray-100">
                        <span class="text-sm text-white/70 light:text-gray-600">{{ $t('metaInsights.freeEntryPoint') }}</span>
                        <span class="font-semibold text-white light:text-gray-900">{{ (aggregatedData as ReturnType<typeof aggregatePricingData>).freeMessages.entryPoint.toLocaleString() }}</span>
                      </div>
                      <div class="flex items-center justify-between py-2 bg-green-500/10 rounded px-2 -mx-2">
                        <span class="text-sm font-medium text-green-400 light:text-green-600">{{ $t('metaInsights.totalFree') }}</span>
                        <span class="font-bold text-green-400 light:text-green-600">{{ (aggregatedData as ReturnType<typeof aggregatePricingData>).freeMessages.total.toLocaleString() }}</span>
                      </div>
                    </div>
                  </CardContent>
                </Card>

                <!-- Paid Messages by Category -->
                <Card>
                  <CardHeader>
                    <CardTitle>{{ $t('metaInsights.paidMessages') }}</CardTitle>
                    <CardDescription>{{ $t('metaInsights.paidBreakdown') }}</CardDescription>
                  </CardHeader>
                  <CardContent>
                    <div class="space-y-3">
                      <div v-for="(count, category) in (aggregatedData as ReturnType<typeof aggregatePricingData>).paidMessages.byCategory" :key="category" class="flex items-center justify-between py-2 border-b border-white/[0.08] light:border-gray-100 last:border-0">
                        <span class="text-sm text-white/70 light:text-gray-600">{{ formatCategory(category as string) }}</span>
                        <span class="font-semibold text-white light:text-gray-900">{{ (count as number).toLocaleString() }}</span>
                      </div>
                      <div v-if="Object.keys((aggregatedData as ReturnType<typeof aggregatePricingData>).paidMessages.byCategory).length === 0" class="text-center text-white/40 light:text-gray-400 py-4">
                        {{ $t('metaInsights.noPaidMessages') }}
                      </div>
                      <div v-else class="flex items-center justify-between py-2 bg-amber-500/10 rounded px-2 -mx-2">
                        <span class="text-sm font-medium text-amber-400 light:text-amber-600">{{ $t('metaInsights.totalPaid') }}</span>
                        <span class="font-bold text-amber-400 light:text-amber-600">{{ (aggregatedData as ReturnType<typeof aggregatePricingData>).paidMessages.total.toLocaleString() }}</span>
                      </div>
                    </div>
                  </CardContent>
                </Card>

                <!-- Cost by Category -->
                <Card>
                  <CardHeader>
                    <CardTitle>{{ $t('metaInsights.costByCategory') }}</CardTitle>
                    <CardDescription>{{ $t('metaInsights.approximateCharges') }}</CardDescription>
                  </CardHeader>
                  <CardContent>
                    <div class="space-y-3">
                      <div v-for="(cost, category) in (aggregatedData as ReturnType<typeof aggregatePricingData>).costByCategory" :key="category" class="flex items-center justify-between py-2 border-b border-white/[0.08] light:border-gray-100 last:border-0">
                        <span class="text-sm text-white/70 light:text-gray-600">{{ formatCategory(category as string) }}</span>
                        <span class="font-semibold text-white light:text-gray-900">{{ formatCurrency(cost as number) }}</span>
                      </div>
                      <div class="flex items-center justify-between py-2 bg-emerald-500/10 rounded px-2 -mx-2">
                        <span class="text-sm font-medium text-emerald-400 light:text-emerald-600">{{ $t('metaInsights.totalCost') }}</span>
                        <span class="font-bold text-emerald-400 light:text-emerald-600">{{ formatCurrency((aggregatedData as ReturnType<typeof aggregatePricingData>).totals.cost) }}</span>
                      </div>
                    </div>
                  </CardContent>
                </Card>

                <!-- By Country -->
                <Card>
                  <CardHeader>
                    <CardTitle>{{ $t('metaInsights.byCountry') }}</CardTitle>
                    <CardDescription>{{ $t('metaInsights.messagesCostByCountry') }}</CardDescription>
                  </CardHeader>
                  <CardContent>
                    <div class="space-y-3">
                      <div v-for="(data, country) in (aggregatedData as ReturnType<typeof aggregatePricingData>).byCountry" :key="country" class="flex items-center justify-between py-2 border-b border-white/[0.08] light:border-gray-100 last:border-0">
                        <span class="text-sm text-white/70 light:text-gray-600">{{ country }}</span>
                        <div class="text-right">
                          <span class="font-semibold text-white light:text-gray-900">{{ (data as {volume: number, cost: number}).volume.toLocaleString() }} {{ $t('metaInsights.msgs') }}</span>
                          <span class="text-white/50 light:text-gray-500 ml-2">{{ formatCurrency((data as {volume: number, cost: number}).cost) }}</span>
                        </div>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              </div>
            </template>
            <template v-else>
              <div class="text-center py-12 text-muted-foreground">
                {{ $t('metaInsights.noPricingData') }}
              </div>
            </template>
          </TabsContent>

          <!-- Template Analytics -->
          <TabsContent value="template_analytics" class="space-y-6">
            <template v-if="isLoading">
              <div class="grid gap-4 md:grid-cols-3">
                <div v-for="i in 3" :key="i" class="rounded-xl border border-white/[0.08] bg-white/[0.02] p-6 light:bg-white light:border-gray-200">
                  <Skeleton class="h-4 w-24 mb-2 bg-white/[0.08] light:bg-gray-200" />
                  <Skeleton class="h-8 w-16 bg-white/[0.08] light:bg-gray-200" />
                </div>
              </div>
            </template>
            <template v-else-if="aggregatedData && activeTab === 'template_analytics'">
              <!-- Stats Cards -->
              <div class="grid gap-4 md:grid-cols-6">
                <div class="card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] p-6 light:bg-white light:border-gray-200">
                  <div class="flex flex-row items-center justify-between space-y-0 pb-2">
                    <span class="text-sm font-medium text-white/50 light:text-gray-500">{{ $t('metaInsights.sent') }}</span>
                    <div class="h-10 w-10 rounded-lg bg-blue-500/20 flex items-center justify-center">
                      <Send class="h-5 w-5 text-blue-400" />
                    </div>
                  </div>
                  <div class="pt-2">
                    <div class="text-3xl font-bold text-white light:text-gray-900">
                      {{ (aggregatedData as ReturnType<typeof aggregateTemplateData>).totals.sent.toLocaleString() }}
                    </div>
                  </div>
                </div>

                <div class="card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] p-6 light:bg-white light:border-gray-200">
                  <div class="flex flex-row items-center justify-between space-y-0 pb-2">
                    <span class="text-sm font-medium text-white/50 light:text-gray-500">{{ $t('metaInsights.delivered') }}</span>
                    <div class="h-10 w-10 rounded-lg bg-emerald-500/20 flex items-center justify-center">
                      <CheckCircle class="h-5 w-5 text-emerald-400" />
                    </div>
                  </div>
                  <div class="pt-2">
                    <div class="text-3xl font-bold text-white light:text-gray-900">
                      {{ (aggregatedData as ReturnType<typeof aggregateTemplateData>).totals.delivered.toLocaleString() }}
                    </div>
                  </div>
                </div>

                <div class="card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] p-6 light:bg-white light:border-gray-200">
                  <div class="flex flex-row items-center justify-between space-y-0 pb-2">
                    <span class="text-sm font-medium text-white/50 light:text-gray-500">{{ $t('metaInsights.read') }}</span>
                    <div class="h-10 w-10 rounded-lg bg-purple-500/20 flex items-center justify-center">
                      <Eye class="h-5 w-5 text-purple-400" />
                    </div>
                  </div>
                  <div class="pt-2">
                    <div class="text-3xl font-bold text-white light:text-gray-900">
                      {{ (aggregatedData as ReturnType<typeof aggregateTemplateData>).totals.read.toLocaleString() }}
                    </div>
                  </div>
                </div>

                <div class="card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] p-6 light:bg-white light:border-gray-200">
                  <div class="flex flex-row items-center justify-between space-y-0 pb-2">
                    <span class="text-sm font-medium text-white/50 light:text-gray-500">{{ $t('metaInsights.replied') }}</span>
                    <div class="h-10 w-10 rounded-lg bg-amber-500/20 flex items-center justify-center">
                      <MessagesSquare class="h-5 w-5 text-amber-400" />
                    </div>
                  </div>
                  <div class="pt-2">
                    <div class="text-3xl font-bold text-white light:text-gray-900">
                      {{ (aggregatedData as ReturnType<typeof aggregateTemplateData>).totals.replied.toLocaleString() }}
                    </div>
                  </div>
                </div>

                <div class="card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] p-6 light:bg-white light:border-gray-200">
                  <div class="flex flex-row items-center justify-between space-y-0 pb-2">
                    <span class="text-sm font-medium text-white/50 light:text-gray-500">{{ $t('metaInsights.clicked') }}</span>
                    <div class="h-10 w-10 rounded-lg bg-cyan-500/20 flex items-center justify-center">
                      <MousePointerClick class="h-5 w-5 text-cyan-400" />
                    </div>
                  </div>
                  <div class="pt-2">
                    <div class="text-3xl font-bold text-white light:text-gray-900">
                      {{ (aggregatedData as ReturnType<typeof aggregateTemplateData>).totals.clicked.toLocaleString() }}
                    </div>
                  </div>
                </div>

                <div class="card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] p-6 light:bg-white light:border-gray-200">
                  <div class="flex flex-row items-center justify-between space-y-0 pb-2">
                    <span class="text-sm font-medium text-white/50 light:text-gray-500">{{ $t('metaInsights.totalCost') }}</span>
                    <div class="h-10 w-10 rounded-lg bg-rose-500/20 flex items-center justify-center">
                      <DollarSign class="h-5 w-5 text-rose-400" />
                    </div>
                  </div>
                  <div class="pt-2">
                    <div class="text-3xl font-bold text-white light:text-gray-900">
                      {{ (aggregatedData as ReturnType<typeof aggregateTemplateData>).totals.cost > 0
                        ? formatCurrency((aggregatedData as ReturnType<typeof aggregateTemplateData>).totals.cost)
                        : '-' }}
                    </div>
                  </div>
                </div>
              </div>

              <!-- Template Performance Table -->
              <Card>
                <CardHeader>
                  <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                    <div>
                      <CardTitle>{{ $t('metaInsights.templatePerformance') }}</CardTitle>
                      <CardDescription>{{ $t('metaInsights.performanceByTemplate') }}</CardDescription>
                    </div>
                    <div class="relative w-full sm:w-64">
                      <Search class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-white/40 light:text-gray-400" />
                      <Input
                        v-model="templateSearchQuery"
                        :placeholder="$t('metaInsights.searchTemplates')"
                        class="pl-9"
                      />
                    </div>
                  </div>
                </CardHeader>
                <CardContent>
                  <DataTable
                    :items="filteredTemplateData"
                    :columns="templateColumns"
                    v-model:sort-key="templateSortKey"
                    v-model:sort-direction="templateSortDirection"
                    :empty-title="templateSearchQuery ? $t('metaInsights.noTemplatesMatching') : $t('metaInsights.noTemplateData')"
                    :empty-description="templateSearchQuery ? '' : $t('metaInsights.noDataForSelectedPeriod')"
                  >
                    <template #cell-name="{ item }">
                      <div class="font-medium">{{ item.name }}</div>
                      <div v-if="item.name !== item.templateId" class="text-xs text-muted-foreground font-mono">{{ item.templateId }}</div>
                    </template>
                    <template #cell-sent="{ item }">
                      {{ item.sent.toLocaleString() }}
                    </template>
                    <template #cell-delivered="{ item }">
                      {{ item.delivered.toLocaleString() }}
                    </template>
                    <template #cell-read="{ item }">
                      {{ item.read.toLocaleString() }}
                    </template>
                    <template #cell-replied="{ item }">
                      {{ item.replied.toLocaleString() }}
                    </template>
                    <template #cell-clicked="{ item }">
                      {{ item.clicked.toLocaleString() }}
                    </template>
                    <template #cell-deliveryRate="{ item }">
                      {{ item.deliveryRate.toFixed(1) }}%
                    </template>
                    <template #cell-readRate="{ item }">
                      {{ item.readRate.toFixed(1) }}%
                    </template>
                    <template #cell-cost="{ item }">
                      {{ item.cost > 0 ? formatCurrency(item.cost) : '-' }}
                    </template>
                  </DataTable>
                </CardContent>
              </Card>
            </template>
            <template v-else>
              <div class="text-center py-12 text-muted-foreground">
                {{ $t('metaInsights.noTemplateAnalytics') }}
              </div>
            </template>
          </TabsContent>

          <!-- Call Analytics -->
          <TabsContent value="call_analytics" class="space-y-6">
            <template v-if="isLoading">
              <div class="grid gap-4 md:grid-cols-2">
                <div v-for="i in 2" :key="i" class="rounded-xl border border-white/[0.08] bg-white/[0.02] p-6 light:bg-white light:border-gray-200">
                  <Skeleton class="h-4 w-24 mb-2 bg-white/[0.08] light:bg-gray-200" />
                  <Skeleton class="h-8 w-16 bg-white/[0.08] light:bg-gray-200" />
                </div>
              </div>
            </template>
            <template v-else-if="aggregatedData && activeTab === 'call_analytics'">
              <!-- Stats Cards -->
              <div class="grid gap-4 md:grid-cols-2">
                <div class="card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] p-6 light:bg-white light:border-gray-200">
                  <div class="flex flex-row items-center justify-between space-y-0 pb-2">
                    <span class="text-sm font-medium text-white/50 light:text-gray-500">{{ $t('metaInsights.totalCalls') }}</span>
                    <div class="h-10 w-10 rounded-lg bg-blue-500/20 flex items-center justify-center">
                      <Phone class="h-5 w-5 text-blue-400" />
                    </div>
                  </div>
                  <div class="pt-2">
                    <div class="text-3xl font-bold text-white light:text-gray-900">
                      {{ (aggregatedData as ReturnType<typeof aggregateCallData>).totals.calls.toLocaleString() }}
                    </div>
                  </div>
                </div>

                <div class="card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] p-6 light:bg-white light:border-gray-200">
                  <div class="flex flex-row items-center justify-between space-y-0 pb-2">
                    <span class="text-sm font-medium text-white/50 light:text-gray-500">{{ $t('metaInsights.totalDuration') }}</span>
                    <div class="h-10 w-10 rounded-lg bg-emerald-500/20 flex items-center justify-center">
                      <TrendingUp class="h-5 w-5 text-emerald-400" />
                    </div>
                  </div>
                  <div class="pt-2">
                    <div class="text-3xl font-bold text-white light:text-gray-900">
                      {{ formatDuration((aggregatedData as ReturnType<typeof aggregateCallData>).totals.duration) }}
                    </div>
                  </div>
                </div>
              </div>

              <!-- Call Breakdown -->
              <div class="grid gap-4 md:grid-cols-2">
                <Card>
                  <CardHeader>
                    <CardTitle>{{ $t('metaInsights.callsByType') }}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div class="space-y-3">
                      <div
                        v-for="[type, count] in Object.entries((aggregatedData as ReturnType<typeof aggregateCallData>).byType)"
                        :key="type"
                        class="flex items-center justify-between"
                      >
                        <span class="text-white/70 light:text-gray-600">{{ formatCategory(type) }}</span>
                        <span class="font-medium text-white light:text-gray-900">{{ count.toLocaleString() }}</span>
                      </div>
                      <div v-if="Object.keys((aggregatedData as ReturnType<typeof aggregateCallData>).byType).length === 0" class="text-muted-foreground text-center py-4">
                        {{ $t('metaInsights.noCallData') }}
                      </div>
                    </div>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader>
                    <CardTitle>{{ $t('metaInsights.callsByDirection') }}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div class="space-y-3">
                      <div
                        v-for="[direction, count] in Object.entries((aggregatedData as ReturnType<typeof aggregateCallData>).byDirection)"
                        :key="direction"
                        class="flex items-center justify-between"
                      >
                        <span class="text-white/70 light:text-gray-600">{{ formatCategory(direction) }}</span>
                        <span class="font-medium text-white light:text-gray-900">{{ count.toLocaleString() }}</span>
                      </div>
                      <div v-if="Object.keys((aggregatedData as ReturnType<typeof aggregateCallData>).byDirection).length === 0" class="text-muted-foreground text-center py-4">
                        {{ $t('metaInsights.noCallData') }}
                      </div>
                    </div>
                  </CardContent>
                </Card>
              </div>
            </template>
            <template v-else>
              <div class="text-center py-12 text-muted-foreground">
                {{ $t('metaInsights.noCallAnalytics') }}
              </div>
            </template>
          </TabsContent>
        </Tabs>
      </div>
    </ScrollArea>
  </div>
</template>
