<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Component } from 'vue'
import DataTable from '@/components/shared/DataTable.vue'
import type { Column } from '@/components/shared/types'
import Combobox from '@/components/oat/Combobox.vue'
import type { ComboOption } from '@/components/oat/combobox'
import LineChart from '@/components/charts/LineChart.vue'
import BarChart from '@/components/charts/BarChart.vue'
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
import { PageHeader, ErrorState, DateRangePicker } from '@/components/shared'
import {
  BarChart3,
  MessageSquare,
  MessagesSquare,
  Wallet,
  FileText,
  Phone,
  PhoneIncoming,
  PhoneOutgoing,
  RefreshCw,
  TrendingUp,
  Send,
  CheckCircle,
  Eye,
  MousePointerClick
} from '@/lib/icons'
import { toast } from '@/lib/toast'
import { useDateRange } from '@/composables/useDateRange'


const { t } = useI18n()

// State
const isLoading = ref(true)
const isRefreshing = ref(false)
const error = ref<string | null>(null)
const accounts = ref<MetaAnalyticsAccount[]>([])
const selectedAccountId = ref<string>('all')
const activeTab = ref<MetaAnalyticsType>('analytics')
const analyticsData = ref<MetaAnalyticsResponse[]>([])

// Used only when Meta gives us no currency for an account.
const DEFAULT_CURRENCY = 'USD'
const isCached = ref(false)

// Granularity
const selectedGranularity = ref<MetaGranularity>('DAY')

// Time range filter
const {
  selectedRange,
  customDateRange,
  isDatePickerOpen,
  dateRange,
  formatDateRangeDisplay,
  applyCustomRange: applyCustomRangeBase,
} = useDateRange({ defaultPreset: '30days', storageKey: 'meta_insights', utc: true })

// Load other saved preferences (non-date-range)
const savedGranularity = localStorage.getItem('meta_insights_granularity') as MetaGranularity | null
const savedAccountId = localStorage.getItem('meta_insights_account_id')
const savedActiveTab = localStorage.getItem('meta_insights_active_tab') as MetaAnalyticsType | null

if (savedGranularity) {
  selectedGranularity.value = savedGranularity as MetaGranularity
}
if (savedAccountId) {
  selectedAccountId.value = savedAccountId
}
if (savedActiveTab) {
  activeTab.value = savedActiveTab as MetaAnalyticsType
}

const saveOtherPreferences = () => {
  localStorage.setItem('meta_insights_granularity', selectedGranularity.value)
  localStorage.setItem('meta_insights_account_id', selectedAccountId.value)
  localStorage.setItem('meta_insights_active_tab', activeTab.value)
}

const accountOptions = computed<ComboOption[]>(() => [
  { value: 'all', label: t('metaInsights.allAccounts') },
  ...accounts.value.map(a => ({ value: a.id, label: a.name }))
])

const granularityOptions = computed(() => [
  { value: 'HALF_HOUR', label: t('metaInsights.halfHour') },
  { value: 'DAY', label: t('metaInsights.daily') },
  { value: 'MONTH', label: t('metaInsights.monthly') }
])

const tabs = computed(() => [
  { value: 'analytics' as MetaAnalyticsType, icon: MessageSquare, label: t('metaInsights.messaging') },
  { value: 'pricing_analytics' as MetaAnalyticsType, icon: Wallet, label: t('metaInsights.pricing') },
  { value: 'template_analytics' as MetaAnalyticsType, icon: FileText, label: t('metaInsights.templates') },
  { value: 'call_analytics' as MetaAnalyticsType, icon: Phone, label: t('metaInsights.calls') }
])

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
  error.value = null
  try {
    const { from, to } = dateRange.value
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
    const envelope = response.data as any
    // Handle error envelope (status: "error")
    if (envelope?.status === 'error') {
      toast.error(envelope.message || 'Failed to load analytics')
      analyticsData.value = []
      return
    }
    const data = envelope?.data || envelope
    analyticsData.value = data.accounts || []
    isCached.value = data.cached || false
  } catch (err: any) {
    console.error('Failed to load analytics:', err?.response?.data, err)
    const errData = err?.response?.data
    const msg = errData?.data?.message || errData?.message || err?.message || 'Failed to load analytics'
    toast.error(msg)
    error.value = t('metaInsights.errorLoadingInsights')
    analyticsData.value = []
  } finally {
    isLoading.value = false
  }
}

const refreshCache = async () => {
  isRefreshing.value = true
  try {
    await metaAnalyticsService.refresh()
    toast.success(t('metaInsights.cacheRefreshed'))
    await fetchAnalytics()
  } catch (error) {
    console.error('Failed to refresh cache:', error)
    toast.error(t('metaInsights.refreshFailed'))
  } finally {
    isRefreshing.value = false
  }
}

const applyCustomRange = () => {
  applyCustomRangeBase()
  fetchAnalytics()
}

watch(selectedRange, (newValue) => {
  if (newValue !== 'custom') {
    fetchAnalytics()
  }
})

watch(selectedGranularity, () => {
  saveOtherPreferences()
  fetchAnalytics()
})

watch(selectedAccountId, () => {
  saveOtherPreferences()
  fetchAnalytics()
})

watch(activeTab, () => {
  saveOtherPreferences()
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

  // Every cost-bearing type is grouped by currency instead — see
  // pricingGroups / templateGroups / callGroups.

  return null
})

// Meta denominates each WABA's cost in that account's own billing currency,
// so costs from accounts on different currencies must never be summed. The
// cost-bearing tabs render one block per currency instead of one aggregate.
function groupPointsByCurrency<T>(extract: (account: MetaAnalyticsResponse) => T[] | undefined) {
  const byCurrency = new Map<string, T[]>()
  for (const account of analyticsData.value) {
    const points = extract(account)
    if (!points?.length) continue
    const currency = account.currency || DEFAULT_CURRENCY
    const existing = byCurrency.get(currency)
    if (existing) {
      existing.push(...points)
    } else {
      byCurrency.set(currency, [...points])
    }
  }
  return byCurrency
}

const pricingGroups = computed(() => {
  if (activeTab.value !== 'pricing_analytics') return []
  return Array.from(
    groupPointsByCurrency(a => a.data?.pricing_analytics?.data_points),
    ([currency, points]) => ({ currency, agg: aggregatePricingData(points) })
  ).sort((a, b) => b.agg.totals.cost - a.agg.totals.cost)
})

const templateGroups = computed(() => {
  if (activeTab.value !== 'template_analytics') return []
  return Array.from(
    groupPointsByCurrency(a => a.data?.template_analytics?.data_points),
    ([currency, points]) => ({ currency, agg: aggregateTemplateData(points) })
  ).sort((a, b) => b.agg.totals.cost - a.agg.totals.cost)
})

const callGroups = computed(() => {
  if (activeTab.value !== 'call_analytics') return []
  return Array.from(
    groupPointsByCurrency(a => a.data?.call_analytics?.data_points),
    ([currency, points]) => ({ currency, agg: aggregateCallData(points) })
  ).sort((a, b) => b.agg.totals.cost - a.agg.totals.cost)
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

function filteredTemplateRows(data: ReturnType<typeof aggregateTemplateData>) {
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
}

function aggregateCallData(points: MetaCallDataPoint[]) {
  const byTime = new Map<number, { incoming: number; outgoing: number; cost: number; totalDuration: number }>()
  let totalCalls = 0
  let totalIncoming = 0
  let totalOutgoing = 0
  let totalCost = 0
  let totalDuration = 0

  for (const point of points) {
    totalCalls += point.count
    totalCost += point.cost
    totalDuration += point.average_duration * point.count

    if (point.direction === 'USER_INITIATED') {
      totalIncoming += point.count
    } else if (point.direction === 'BUSINESS_INITIATED') {
      totalOutgoing += point.count
    }

    const existing = byTime.get(point.start) || { incoming: 0, outgoing: 0, cost: 0, totalDuration: 0 }
    const isIncoming = point.direction === 'USER_INITIATED'
    byTime.set(point.start, {
      incoming: existing.incoming + (isIncoming ? point.count : 0),
      outgoing: existing.outgoing + (!isIncoming ? point.count : 0),
      cost: existing.cost + point.cost,
      totalDuration: existing.totalDuration + point.average_duration * point.count
    })
  }

  const avgDuration = totalCalls > 0 ? Math.round(totalDuration / totalCalls) : 0
  const sortedTimes = Array.from(byTime.entries()).sort((a, b) => a[0] - b[0])

  return {
    totals: { calls: totalCalls, incoming: totalIncoming, outgoing: totalOutgoing, cost: totalCost, avgDuration },
    timeSeries: sortedTimes.map(([time, data]) => ({
      time,
      incoming: data.incoming,
      outgoing: data.outgoing,
      cost: data.cost
    }))
  }
}

// Chart data
const messagingChart = computed(() => {
  if (!aggregatedData.value || activeTab.value !== 'analytics') {
    return { labels: [] as string[], series: [] }
  }
  const data = aggregatedData.value as ReturnType<typeof aggregateMessagingData>
  return {
    labels: data.timeSeries.map(ts => formatTimestamp(ts.time)),
    series: [
      { label: t('metaInsights.sent'), data: data.timeSeries.map(d => d.sent) },
      { label: t('metaInsights.delivered'), data: data.timeSeries.map(d => d.delivered) }
    ]
  }
})

function pricingChartFor(data: ReturnType<typeof aggregatePricingData>) {
  const categories = Object.entries(data.byCategory)
  return {
    labels: categories.map(([cat]) => formatCategory(cat)),
    series: [
      { label: t('metaInsights.messaging'), data: categories.map(([, val]) => val.volume) },
      { label: t('metaInsights.cost'), data: categories.map(([, val]) => val.cost) }
    ]
  }
}

function callChartFor(data: ReturnType<typeof aggregateCallData>) {
  return {
    labels: data.timeSeries.map(ts => formatTimestamp(ts.time)),
    series: [
      { label: t('metaInsights.incoming'), data: data.timeSeries.map(d => d.incoming) },
      { label: t('metaInsights.outgoing'), data: data.timeSeries.map(d => d.outgoing) }
    ]
  }
}

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

function formatCurrency(value: number, currency: string = DEFAULT_CURRENCY): string {
  try {
    return new Intl.NumberFormat('en-US', { style: 'currency', currency }).format(value)
  } catch {
    // Intl throws RangeError on a code it doesn't recognise. Never blank the
    // whole view over one unexpected currency code.
    return new Intl.NumberFormat('en-US', { style: 'currency', currency: DEFAULT_CURRENCY }).format(value)
  }
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

type StatTile = { key: string; label: string; value: string; icon?: Component; badge?: string }

const messagingTiles = computed<StatTile[]>(() => {
  const totals = (aggregatedData.value as ReturnType<typeof aggregateMessagingData> | null)?.totals
  if (!totals) return []
  return [
    { key: 'sent', icon: Send, label: t('metaInsights.messagesSent'), value: totals.sent.toLocaleString() },
    { key: 'delivered', icon: CheckCircle, label: t('metaInsights.messagesDelivered'), value: totals.delivered.toLocaleString() },
    {
      key: 'rate',
      icon: TrendingUp,
      label: t('metaInsights.deliveryRate'),
      value: `${totals.sent > 0 ? (totals.delivered / totals.sent * 100).toFixed(1) : 0}%`
    }
  ]
})

function pricingTiles(group: { currency: string; agg: ReturnType<typeof aggregatePricingData> }): StatTile[] {
  return [
    { key: 'messages', icon: MessagesSquare, label: t('metaInsights.totalMessages'), value: group.agg.totals.volume.toLocaleString() },
    { key: 'cost', label: t('metaInsights.totalCost'), badge: group.currency, value: formatCurrency(group.agg.totals.cost, group.currency) }
  ]
}

function templateTiles(group: { currency: string; agg: ReturnType<typeof aggregateTemplateData> }): StatTile[] {
  const totals = group.agg.totals
  return [
    { key: 'sent', icon: Send, label: t('metaInsights.sent'), value: totals.sent.toLocaleString() },
    { key: 'delivered', icon: CheckCircle, label: t('metaInsights.delivered'), value: totals.delivered.toLocaleString() },
    { key: 'read', icon: Eye, label: t('metaInsights.read'), value: totals.read.toLocaleString() },
    { key: 'replied', icon: MessagesSquare, label: t('metaInsights.replied'), value: totals.replied.toLocaleString() },
    { key: 'clicked', icon: MousePointerClick, label: t('metaInsights.clicked'), value: totals.clicked.toLocaleString() },
    {
      key: 'cost',
      label: t('metaInsights.totalCost'),
      badge: group.currency,
      value: totals.cost > 0 ? formatCurrency(totals.cost, group.currency) : '-'
    }
  ]
}

function callTiles(group: { currency: string; agg: ReturnType<typeof aggregateCallData> }): StatTile[] {
  const totals = group.agg.totals
  return [
    { key: 'calls', icon: Phone, label: t('metaInsights.totalCalls'), value: totals.calls.toLocaleString() },
    { key: 'incoming', icon: PhoneIncoming, label: t('metaInsights.incoming'), value: totals.incoming.toLocaleString() },
    { key: 'outgoing', icon: PhoneOutgoing, label: t('metaInsights.outgoing'), value: totals.outgoing.toLocaleString() },
    { key: 'duration', icon: TrendingUp, label: t('metaInsights.avgDuration'), value: formatDuration(totals.avgDuration) },
    { key: 'cost', label: t('metaInsights.totalCost'), badge: group.currency, value: formatCurrency(totals.cost, group.currency) }
  ]
}

</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader
      :title="$t('metaInsights.title')"
      :description="$t('metaInsights.subtitle')"
      :icon="BarChart3"
    >
      <template #actions>
        <!-- Account Filter -->
        <Combobox
          v-model="selectedAccountId"
          :options="accountOptions"
          :placeholder="$t('metaInsights.selectAccount')"
          :search-placeholder="$t('metaInsights.searchAccount')"
          :empty-text="$t('metaInsights.noAccountFound')"
          :aria-label="$t('metaInsights.selectAccount')"
        />

        <!-- Granularity Filter -->
        <select v-model="selectedGranularity" :aria-label="$t('metaInsights.granularity')">
          <option v-for="opt in granularityOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
        </select>

        <!-- Time Range Filter -->
        <DateRangePicker
          v-model:selected-range="selectedRange"
          v-model:custom-date-range="customDateRange"
          v-model:is-date-picker-open="isDatePickerOpen"
          :format-date-range-display="formatDateRangeDisplay"
          @apply-custom="applyCustomRange"
        />

        <!-- Refresh Button -->
        <button
          type="button"
          class="outline icon"
          :aria-label="$t('metaInsights.refreshCache')"
          :title="$t('metaInsights.refreshCache')"
          :disabled="isRefreshing"
          :aria-busy="isRefreshing"
          @click="refreshCache"
        >
          <RefreshCw :size="16" />
        </button>

        <!-- Cache indicator -->
        <span v-if="isCached" class="badge" data-variant="secondary">{{ $t('metaInsights.cached') }}</span>
      </template>
    </PageHeader>

    <!-- Content -->
    <div class="flex-1 scroll-y p-4">
      <div class="vstack gap-4">
        <!-- Error State -->
        <ErrorState
          v-if="error && !isLoading"
          :title="$t('common.loadErrorTitle')"
          :description="error"
          :retry-label="$t('common.retry')"
          @retry="fetchAnalytics"
        />

        <!-- Analytics Type Tabs -->
        <div v-if="!error">
          <div role="tablist">
            <button
              v-for="tab in tabs"
              :key="tab.value"
              :id="`meta-tab-${tab.value}`"
              type="button"
              role="tab"
              aria-controls="meta-tabpanel"
              :aria-selected="activeTab === tab.value"
              :title="tab.label"
              @click="activeTab = tab.value"
            >
              <component :is="tab.icon" :size="16" />
              {{ tab.label }}
            </button>
          </div>

          <div id="meta-tabpanel" role="tabpanel" :aria-labelledby="`meta-tab-${activeTab}`" class="vstack gap-4">
            <!-- Messaging Analytics -->
            <template v-if="activeTab === 'analytics'">
              <template v-if="isLoading">
                <div class="row">
              <div v-for="i in 3" :key="i" class="col-4">
                <article class="card" aria-busy="true">
                  <div role="status" class="skeleton line"></div>
                  <div role="status" class="skeleton box"></div>
                </article>
              </div>
            </div>
              </template>
              <template v-else-if="aggregatedData">
                <div class="row">
                <div v-for="tile in messagingTiles" :key="tile.key" class="col-4">
                  <article class="card">
                    <div class="hstack items-center gap-2 text-light">
                      <component :is="tile.icon" v-if="tile.icon" :size="16" />
                      <span v-else class="badge" data-variant="secondary">{{ tile.badge }}</span>
                      <small>{{ tile.label }}</small>
                    </div>
                    <h3>{{ tile.value }}</h3>
                  </article>
                </div>
              </div>

                <article class="card">
                  <h4>{{ $t('metaInsights.messageDeliveryOverTime') }}</h4>
                  <LineChart
                    v-if="messagingChart.labels.length > 0"
                    :labels="messagingChart.labels"
                    :series="messagingChart.series"
                    :height="320"
                    :caption="$t('metaInsights.messageDeliveryOverTime')"
                  />
                  <p v-else class="text-light">{{ $t('metaInsights.noDataForPeriod') }}</p>
                </article>
              </template>
              <p v-else class="text-light align-center p-4">{{ $t('metaInsights.noMessagingData') }}</p>
            </template>

            <!-- Pricing Analytics -->
            <template v-else-if="activeTab === 'pricing_analytics'">
              <article v-if="isLoading" class="card" aria-busy="true">
                <div role="status" class="skeleton box"></div>
              </article>
              <template v-else-if="pricingGroups.length">
                <div v-for="group in pricingGroups" :key="group.currency" class="vstack gap-4">
                  <div v-if="pricingGroups.length > 1" class="hstack items-center gap-2">
                <span class="badge outline">{{ group.currency }}</span>
                <small class="text-light">{{ $t('metaInsights.billedIn') }}</small>
              </div>
                  <div class="row">
                <div v-for="tile in pricingTiles(group)" :key="tile.key" class="col-6">
                  <article class="card">
                    <div class="hstack items-center gap-2 text-light">
                      <component :is="tile.icon" v-if="tile.icon" :size="16" />
                      <span v-else class="badge" data-variant="secondary">{{ tile.badge }}</span>
                      <small>{{ tile.label }}</small>
                    </div>
                    <h3>{{ tile.value }}</h3>
                  </article>
                </div>
              </div>

                  <article class="card">
                    <h4>{{ $t('metaInsights.messagesCostByCategory') }}</h4>
                    <BarChart
                      v-if="Object.keys(group.agg.byCategory).length > 0"
                      :labels="pricingChartFor(group.agg).labels"
                      :series="pricingChartFor(group.agg).series"
                      :height="320"
                      :caption="$t('metaInsights.messagesCostByCategory')"
                    />
                    <p v-else class="text-light">{{ $t('metaInsights.noDataForPeriod') }}</p>
                  </article>

                  <!-- Detailed Breakdown -->
                  <div class="row">
                    <div class="col-6">
                      <article class="card">
                        <h4>{{ $t('metaInsights.freeMessages') }}</h4>
                        <div class="vstack gap-2">
                          <div class="hstack justify-between items-center gap-2">
                            <span class="text-light">{{ $t('metaInsights.freeCustomerService') }}</span>
                            <strong>{{ group.agg.freeMessages.customerService.toLocaleString() }}</strong>
                          </div>
                          <div class="hstack justify-between items-center gap-2">
                            <span class="text-light">{{ $t('metaInsights.freeEntryPoint') }}</span>
                            <strong>{{ group.agg.freeMessages.entryPoint.toLocaleString() }}</strong>
                          </div>
                          <hr />
                          <div class="hstack justify-between items-center gap-2">
                            <strong>{{ $t('metaInsights.totalFree') }}</strong>
                            <span class="badge" data-variant="success">{{ group.agg.freeMessages.total.toLocaleString() }}</span>
                          </div>
                        </div>
                      </article>
                    </div>

                    <div class="col-6">
                      <article class="card">
                        <h4>{{ $t('metaInsights.paidMessages') }}</h4>
                        <div class="vstack gap-2">
                          <div v-for="(count, category) in group.agg.paidMessages.byCategory" :key="category" class="hstack justify-between items-center gap-2">
                            <span class="text-light">{{ formatCategory(category as string) }}</span>
                            <strong>{{ (count as number).toLocaleString() }}</strong>
                          </div>
                          <p v-if="Object.keys(group.agg.paidMessages.byCategory).length === 0" class="text-light">
                            {{ $t('metaInsights.noPaidMessages') }}
                          </p>
                          <template v-else>
                            <hr />
                            <div class="hstack justify-between items-center gap-2">
                              <strong>{{ $t('metaInsights.totalPaid') }}</strong>
                              <span class="badge" data-variant="warning">{{ group.agg.paidMessages.total.toLocaleString() }}</span>
                            </div>
                          </template>
                        </div>
                      </article>
                    </div>

                    <div class="col-6">
                      <article class="card">
                        <h4>{{ $t('metaInsights.costByCategory') }}</h4>
                        <div class="vstack gap-2">
                          <div v-for="(cost, category) in group.agg.costByCategory" :key="category" class="hstack justify-between items-center gap-2">
                            <span class="text-light">{{ formatCategory(category as string) }}</span>
                            <strong>{{ formatCurrency(cost as number, group.currency) }}</strong>
                          </div>
                          <hr />
                          <div class="hstack justify-between items-center gap-2">
                            <strong>{{ $t('metaInsights.totalCost') }}</strong>
                            <span class="badge" data-variant="success">{{ formatCurrency(group.agg.totals.cost, group.currency) }}</span>
                          </div>
                        </div>
                      </article>
                    </div>

                    <div class="col-6">
                      <article class="card">
                        <h4>{{ $t('metaInsights.byCountry') }}</h4>
                        <div class="vstack gap-2">
                          <div v-for="(data, country) in group.agg.byCountry" :key="country" class="hstack justify-between items-center gap-2">
                            <span class="text-light">{{ country }}</span>
                            <span>
                              <strong>{{ (data as {volume: number, cost: number}).volume.toLocaleString() }} {{ $t('metaInsights.msgs') }}</strong>
                              <span class="text-light">{{ formatCurrency((data as {volume: number, cost: number}).cost, group.currency) }}</span>
                            </span>
                          </div>
                        </div>
                      </article>
                    </div>
                  </div>
                </div>
              </template>
              <p v-else class="text-light align-center p-4">{{ $t('metaInsights.noPricingData') }}</p>
            </template>

            <!-- Template Analytics -->
            <template v-else-if="activeTab === 'template_analytics'">
              <template v-if="isLoading">
                <div class="row">
              <div v-for="i in 3" :key="i" class="col-4">
                <article class="card" aria-busy="true">
                  <div role="status" class="skeleton line"></div>
                  <div role="status" class="skeleton box"></div>
                </article>
              </div>
            </div>
              </template>
              <template v-else-if="templateGroups.length">
                <div v-for="group in templateGroups" :key="group.currency" class="vstack gap-4">
                  <div v-if="templateGroups.length > 1" class="hstack items-center gap-2">
                <span class="badge outline">{{ group.currency }}</span>
                <small class="text-light">{{ $t('metaInsights.billedIn') }}</small>
              </div>
                  <div class="row">
                <div v-for="tile in templateTiles(group)" :key="tile.key" class="col-4">
                  <article class="card">
                    <div class="hstack items-center gap-2 text-light">
                      <component :is="tile.icon" v-if="tile.icon" :size="16" />
                      <span v-else class="badge" data-variant="secondary">{{ tile.badge }}</span>
                      <small>{{ tile.label }}</small>
                    </div>
                    <h3>{{ tile.value }}</h3>
                  </article>
                </div>
              </div>

                  <!-- Template Performance Table -->
                  <article class="card">
                    <header class="hstack justify-between items-center flex-wrap gap-4">
                      <h4>{{ $t('metaInsights.templatePerformance') }}</h4>
                      <input
                        v-model="templateSearchQuery"
                        type="search"
                        :placeholder="$t('metaInsights.searchTemplates')"
                        :aria-label="$t('metaInsights.searchTemplates')"
                      />
                    </header>
                    <DataTable
                      :items="filteredTemplateRows(group.agg)"
                      :columns="templateColumns"
                      v-model:sort-key="templateSortKey"
                      v-model:sort-direction="templateSortDirection"
                      :empty-title="templateSearchQuery ? $t('metaInsights.noTemplatesMatching') : $t('metaInsights.noTemplateData')"
                      :empty-description="templateSearchQuery ? '' : $t('metaInsights.noDataForSelectedPeriod')"
                    >
                      <template #cell-name="{ item }">
                        <strong>{{ item.name }}</strong>
                        <div v-if="item.name !== item.templateId"><small class="text-light"><code>{{ item.templateId }}</code></small></div>
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
                        {{ item.cost > 0 ? formatCurrency(item.cost, group.currency) : '-' }}
                      </template>
                    </DataTable>
                  </article>
                </div>
              </template>
              <p v-else class="text-light align-center p-4">{{ $t('metaInsights.noTemplateAnalytics') }}</p>
            </template>

            <!-- Call Analytics -->
            <template v-else-if="activeTab === 'call_analytics'">
              <template v-if="isLoading">
                <div class="row">
              <div v-for="i in 2" :key="i" class="col-6">
                <article class="card" aria-busy="true">
                  <div role="status" class="skeleton line"></div>
                  <div role="status" class="skeleton box"></div>
                </article>
              </div>
            </div>
              </template>
              <template v-else-if="callGroups.length">
                <div v-for="group in callGroups" :key="group.currency" class="vstack gap-4">
                  <div v-if="callGroups.length > 1" class="hstack items-center gap-2">
                <span class="badge outline">{{ group.currency }}</span>
                <small class="text-light">{{ $t('metaInsights.billedIn') }}</small>
              </div>
                  <div class="row">
                <div v-for="tile in callTiles(group)" :key="tile.key" class="col-4">
                  <article class="card">
                    <div class="hstack items-center gap-2 text-light">
                      <component :is="tile.icon" v-if="tile.icon" :size="16" />
                      <span v-else class="badge" data-variant="secondary">{{ tile.badge }}</span>
                      <small>{{ tile.label }}</small>
                    </div>
                    <h3>{{ tile.value }}</h3>
                  </article>
                </div>
              </div>

                  <article class="card">
                    <h4>{{ $t('metaInsights.callsOverTime') }}</h4>
                    <LineChart
                      v-if="group.agg.timeSeries.length > 0"
                      :labels="callChartFor(group.agg).labels"
                      :series="callChartFor(group.agg).series"
                      :height="320"
                      :caption="$t('metaInsights.callsOverTime')"
                    />
                    <p v-else class="text-light">{{ $t('metaInsights.noDataForPeriod') }}</p>
                  </article>
                </div>
              </template>
              <p v-else class="text-light align-center p-4">{{ $t('metaInsights.noCallAnalytics') }}</p>
            </template>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
