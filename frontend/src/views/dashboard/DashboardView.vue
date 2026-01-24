<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { RangeCalendar } from '@/components/ui/range-calendar'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from '@/components/ui/alert-dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { analyticsService, dashboardWidgetsService, type DashboardWidget, type WidgetData } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import {
  MessageSquare,
  Users,
  Bot,
  Send,
  TrendingUp,
  TrendingDown,
  Minus,
  Clock,
  CheckCheck,
  CalendarIcon,
  LayoutDashboard,
  Plus,
  Pencil,
  Trash2,
  BarChart3,
  X
} from 'lucide-vue-next'
import type { DateRange } from 'reka-ui'
import { CalendarDate } from '@internationalized/date'
import { useToast } from '@/components/ui/toast'

const { toast } = useToast()
const authStore = useAuthStore()

// Permission checks
const canCreateWidget = computed(() => authStore.hasPermission('analytics', 'write'))
const canEditWidget = computed(() => authStore.hasPermission('analytics', 'write'))
const canDeleteWidget = computed(() => authStore.hasPermission('analytics', 'delete'))

interface RecentMessage {
  id: string
  contact_name: string
  content: string
  direction: string
  created_at: string
  status: string
}

// Widgets state
const widgets = ref<DashboardWidget[]>([])
const widgetData = ref<Record<string, WidgetData>>({})
const recentMessages = ref<RecentMessage[]>([])
const isLoading = ref(true)
const isWidgetDataLoading = ref(false)

// Widget builder state
const isWidgetDialogOpen = ref(false)
const isEditMode = ref(false)
const editingWidgetId = ref<string | null>(null)
const isSavingWidget = ref(false)

// Delete dialog state
const deleteDialogOpen = ref(false)
const widgetToDelete = ref<DashboardWidget | null>(null)

const dataSources = ref<Array<{ name: string; label: string; fields: string[] }>>([])
const metrics = ref<string[]>([])
const displayTypes = ref<string[]>([])
const operators = ref<Array<{ value: string; label: string }>>([])

const widgetForm = ref({
  name: '',
  description: '',
  data_source: '',
  metric: 'count',
  field: '',
  filters: [] as Array<{ field: string; operator: string; value: string }>,
  display_type: 'number',
  chart_type: '',
  show_change: true,
  color: 'blue',
  size: 'small',
  is_shared: false
})

// Color options
const colorOptions = [
  { value: 'blue', label: 'Blue', bg: 'bg-blue-500/20', text: 'text-blue-400' },
  { value: 'green', label: 'Green', bg: 'bg-emerald-500/20', text: 'text-emerald-400' },
  { value: 'purple', label: 'Purple', bg: 'bg-purple-500/20', text: 'text-purple-400' },
  { value: 'orange', label: 'Orange', bg: 'bg-orange-500/20', text: 'text-orange-400' },
  { value: 'red', label: 'Red', bg: 'bg-red-500/20', text: 'text-red-400' },
  { value: 'cyan', label: 'Cyan', bg: 'bg-cyan-500/20', text: 'text-cyan-400' }
]

// Time range filter
type TimeRangePreset = 'today' | '7days' | '30days' | 'this_month' | 'custom'

const loadSavedPreferences = () => {
  const savedRange = localStorage.getItem('dashboard_time_range') as TimeRangePreset | null
  const savedCustomRange = localStorage.getItem('dashboard_custom_range')

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
    } catch (e) {
      console.error('Failed to parse saved custom range:', e)
    }
  }

  return {
    range: savedRange || 'this_month',
    customRange
  }
}

const savedPrefs = loadSavedPreferences()
const selectedRange = ref<TimeRangePreset>(savedPrefs.range as TimeRangePreset)
const customDateRange = ref<any>(savedPrefs.customRange)
const isDatePickerOpen = ref(false)

const savePreferences = () => {
  localStorage.setItem('dashboard_time_range', selectedRange.value)
  if (selectedRange.value === 'custom' && customDateRange.value.start && customDateRange.value.end) {
    localStorage.setItem('dashboard_custom_range', JSON.stringify({
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

const comparisonPeriodLabel = computed(() => {
  switch (selectedRange.value) {
    case 'today':
      return 'from yesterday'
    case '7days':
      return 'from previous 7 days'
    case '30days':
      return 'from previous 30 days'
    case 'this_month':
      return 'from last month'
    case 'custom':
      return 'from previous period'
    default:
      return 'from previous period'
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

const formatNumber = (num: number): string => {
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return Math.round(num).toString()
}

const formatTime = (dateStr: string): string => {
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMins = Math.floor(diffMs / 60000)
  const diffHours = Math.floor(diffMs / 3600000)
  const diffDays = Math.floor(diffMs / 86400000)

  if (diffMins < 1) return 'Just now'
  if (diffMins < 60) return `${diffMins}m ago`
  if (diffHours < 24) return `${diffHours}h ago`
  return `${diffDays}d ago`
}

const getWidgetColor = (color: string) => {
  const colorConfig = colorOptions.find(c => c.value === color) || colorOptions[0]
  return colorConfig
}

const getWidgetIcon = (dataSource: string) => {
  switch (dataSource) {
    case 'messages':
      return MessageSquare
    case 'contacts':
      return Users
    case 'sessions':
      return Bot
    case 'campaigns':
      return Send
    case 'transfers':
      return Users
    default:
      return BarChart3
  }
}

const availableFields = computed(() => {
  if (!widgetForm.value.data_source) return []
  const source = dataSources.value.find(s => s.name === widgetForm.value.data_source)
  return source?.fields || []
})

// Fetch data
const fetchWidgets = async () => {
  try {
    const response = await dashboardWidgetsService.list()
    widgets.value = (response.data as any).data?.widgets || []
  } catch (error) {
    console.error('Failed to load widgets:', error)
    widgets.value = []
  }
}

const fetchWidgetData = async () => {
  if (widgets.value.length === 0) return

  isWidgetDataLoading.value = true
  try {
    const { from, to } = getDateRange.value
    const response = await dashboardWidgetsService.getAllData({ from, to })
    widgetData.value = (response.data as any).data?.data || {}
  } catch (error) {
    console.error('Failed to load widget data:', error)
    widgetData.value = {}
  } finally {
    isWidgetDataLoading.value = false
  }
}

const fetchRecentMessages = async () => {
  try {
    const { from, to } = getDateRange.value
    const response = await analyticsService.dashboard({ from, to })
    const data = response.data.data || response.data
    recentMessages.value = data.recent_messages || []
  } catch (error) {
    console.error('Failed to load recent messages:', error)
    recentMessages.value = []
  }
}

const fetchDataSources = async () => {
  try {
    const response = await dashboardWidgetsService.getDataSources()
    const data = (response.data as any).data || response.data
    dataSources.value = data.data_sources || []
    metrics.value = data.metrics || []
    displayTypes.value = data.display_types || []
    operators.value = data.operators || []
  } catch (error) {
    console.error('Failed to load data sources:', error)
  }
}

const fetchDashboardData = async () => {
  isLoading.value = true
  try {
    await Promise.all([
      fetchWidgets(),
      fetchRecentMessages(),
      fetchDataSources()
    ])
    await fetchWidgetData()
  } finally {
    isLoading.value = false
  }
}

const applyCustomRange = () => {
  if (customDateRange.value.start && customDateRange.value.end) {
    isDatePickerOpen.value = false
    savePreferences()
    fetchWidgetData()
    fetchRecentMessages()
  }
}

// Widget CRUD
const openAddWidgetDialog = () => {
  isEditMode.value = false
  editingWidgetId.value = null
  widgetForm.value = {
    name: '',
    description: '',
    data_source: '',
    metric: 'count',
    field: '',
    filters: [],
    display_type: 'number',
    chart_type: '',
    show_change: true,
    color: 'blue',
    size: 'small',
    is_shared: false
  }
  isWidgetDialogOpen.value = true
}

const openEditWidgetDialog = (widget: DashboardWidget) => {
  isEditMode.value = true
  editingWidgetId.value = widget.id
  widgetForm.value = {
    name: widget.name,
    description: widget.description,
    data_source: widget.data_source,
    metric: widget.metric,
    field: widget.field,
    filters: [...widget.filters],
    display_type: widget.display_type,
    chart_type: widget.chart_type,
    show_change: widget.show_change,
    color: widget.color || 'blue',
    size: widget.size,
    is_shared: widget.is_shared
  }
  isWidgetDialogOpen.value = true
}

const addFilter = () => {
  widgetForm.value.filters.push({ field: '', operator: 'equals', value: '' })
}

const removeFilter = (index: number) => {
  widgetForm.value.filters.splice(index, 1)
}

const saveWidget = async () => {
  if (!widgetForm.value.name || !widgetForm.value.data_source) {
    toast({
      title: 'Validation Error',
      description: 'Name and data source are required',
      variant: 'destructive'
    })
    return
  }

  // Clean up empty filters
  const cleanFilters = widgetForm.value.filters.filter(f => f.field && f.operator && f.value)

  const payload = {
    name: widgetForm.value.name,
    description: widgetForm.value.description,
    data_source: widgetForm.value.data_source,
    metric: widgetForm.value.metric,
    field: widgetForm.value.field,
    filters: cleanFilters,
    display_type: widgetForm.value.display_type,
    chart_type: widgetForm.value.chart_type,
    show_change: widgetForm.value.show_change,
    color: widgetForm.value.color,
    size: widgetForm.value.size,
    is_shared: widgetForm.value.is_shared
  }

  isSavingWidget.value = true
  try {
    if (isEditMode.value && editingWidgetId.value) {
      await dashboardWidgetsService.update(editingWidgetId.value, payload)
      toast({ title: 'Widget updated successfully' })
    } else {
      await dashboardWidgetsService.create(payload)
      toast({ title: 'Widget created successfully' })
    }
    isWidgetDialogOpen.value = false
    await fetchWidgets()
    await fetchWidgetData()
  } catch (error: any) {
    toast({
      title: 'Error',
      description: error.response?.data?.message || 'Failed to save widget',
      variant: 'destructive'
    })
  } finally {
    isSavingWidget.value = false
  }
}

const openDeleteDialog = (widget: DashboardWidget) => {
  widgetToDelete.value = widget
  deleteDialogOpen.value = true
}

const confirmDeleteWidget = async () => {
  if (!widgetToDelete.value) return

  try {
    await dashboardWidgetsService.delete(widgetToDelete.value.id)
    toast({ title: 'Widget deleted successfully' })
    deleteDialogOpen.value = false
    widgetToDelete.value = null
    await fetchWidgets()
    await fetchWidgetData()
  } catch (error: any) {
    toast({
      title: 'Error',
      description: error.response?.data?.message || 'Failed to delete widget',
      variant: 'destructive'
    })
  }
}

// Watch for range changes
watch(selectedRange, (newValue) => {
  savePreferences()
  if (newValue !== 'custom') {
    fetchWidgetData()
    fetchRecentMessages()
  }
})

onMounted(() => {
  fetchDashboardData()
})
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <!-- Header -->
    <header class="border-b border-white/[0.08] light:border-gray-200 bg-[#0a0a0b]/95 light:bg-white/95 backdrop-blur">
      <div class="flex h-16 items-center px-6">
        <div class="h-8 w-8 rounded-lg bg-gradient-to-br from-emerald-500 to-green-600 flex items-center justify-center mr-3 shadow-lg shadow-emerald-500/20">
          <LayoutDashboard class="h-4 w-4 text-white" />
        </div>
        <div class="flex-1">
          <h1 class="text-xl font-semibold text-white light:text-gray-900">Dashboard</h1>
          <p class="text-sm text-white/50 light:text-gray-500">Customizable analytics overview</p>
        </div>

        <!-- Time Range Filter -->
        <div class="flex items-center gap-2">
          <Button v-if="canCreateWidget" variant="outline" size="sm" @click="openAddWidgetDialog" class="bg-white/[0.04] border-white/[0.1] text-white/70 hover:bg-white/[0.08] hover:text-white light:bg-white light:border-gray-200 light:text-gray-700">
            <Plus class="h-4 w-4 mr-2" />
            Add Widget
          </Button>

          <Select v-model="selectedRange">
            <SelectTrigger class="w-[180px] bg-white/[0.04] border-white/[0.1] text-white/70 hover:bg-white/[0.08] light:bg-white light:border-gray-200 light:text-gray-700">
              <SelectValue placeholder="Select range" />
            </SelectTrigger>
            <SelectContent class="bg-[#141414] border-white/[0.08] light:bg-white light:border-gray-200">
              <SelectItem value="today" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">Today</SelectItem>
              <SelectItem value="7days" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">Last 7 days</SelectItem>
              <SelectItem value="30days" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">Last 30 days</SelectItem>
              <SelectItem value="this_month" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">This month</SelectItem>
              <SelectItem value="custom" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">Custom range</SelectItem>
            </SelectContent>
          </Select>

          <Popover v-if="selectedRange === 'custom'" v-model:open="isDatePickerOpen">
            <PopoverTrigger as-child>
              <Button variant="outline" class="w-auto bg-white/[0.04] border-white/[0.1] text-white/70 hover:bg-white/[0.08] hover:text-white light:bg-white light:border-gray-200 light:text-gray-700 light:hover:bg-gray-50">
                <CalendarIcon class="h-4 w-4 mr-2" />
                {{ formatDateRange || 'Select dates' }}
              </Button>
            </PopoverTrigger>
            <PopoverContent class="w-auto p-4 bg-[#141414] border-white/[0.08] light:bg-white light:border-gray-200" align="end">
              <div class="space-y-4">
                <RangeCalendar v-model="customDateRange" :number-of-months="2" />
                <Button class="w-full" @click="applyCustomRange" :disabled="!customDateRange.start || !customDateRange.end">
                  Apply Range
                </Button>
              </div>
            </PopoverContent>
          </Popover>
        </div>
      </div>
    </header>

    <!-- Content -->
    <ScrollArea class="flex-1">
      <div class="p-6 space-y-6">
        <!-- Widgets Grid -->
        <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          <!-- Loading State -->
          <template v-if="isLoading">
            <div v-for="i in 4" :key="i" class="rounded-xl border border-white/[0.08] bg-white/[0.02] p-6 light:bg-white light:border-gray-200">
              <div class="flex flex-row items-center justify-between space-y-0 pb-2">
                <Skeleton class="h-4 w-24 bg-white/[0.08] light:bg-gray-200" />
                <Skeleton class="h-10 w-10 rounded-lg bg-white/[0.08] light:bg-gray-200" />
              </div>
              <div class="pt-2">
                <Skeleton class="h-8 w-20 mb-2 bg-white/[0.08] light:bg-gray-200" />
                <Skeleton class="h-3 w-32 bg-white/[0.08] light:bg-gray-200" />
              </div>
            </div>
          </template>

          <!-- Widget Cards -->
          <template v-else>
            <div
              v-for="widget in widgets"
              :key="widget.id"
              class="group relative card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] p-6 light:bg-white light:border-gray-200 hover:bg-white/[0.06] light:hover:bg-gray-50 transition-colors"
            >
              <div class="flex flex-row items-start justify-between space-y-0 pb-2">
                <div class="flex-1">
                  <span class="text-sm font-medium text-white/50 light:text-gray-500">
                    {{ widget.name }}
                  </span>
                </div>
                <div class="flex items-center gap-2">
                  <!-- Actions - hidden by default, shown on card hover -->
                  <div v-if="canEditWidget || canDeleteWidget" class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <Button
                      v-if="canEditWidget"
                      variant="ghost"
                      size="icon"
                      class="h-6 w-6 text-white/20 hover:text-white hover:bg-white/[0.1] light:text-gray-300 light:hover:text-gray-700 light:hover:bg-gray-100"
                      @click.stop="openEditWidgetDialog(widget)"
                      title="Edit widget"
                    >
                      <Pencil class="h-3 w-3" />
                    </Button>
                    <Button
                      v-if="canDeleteWidget"
                      variant="ghost"
                      size="icon"
                      class="h-6 w-6 text-white/20 hover:text-red-400 hover:bg-red-500/10 light:text-gray-300 light:hover:text-red-600 light:hover:bg-red-50"
                      @click.stop="openDeleteDialog(widget)"
                      title="Delete widget"
                    >
                      <Trash2 class="h-3 w-3" />
                    </Button>
                  </div>
                  <!-- Icon -->
                  <div :class="['h-10 w-10 rounded-lg flex items-center justify-center', getWidgetColor(widget.color).bg]">
                    <component :is="getWidgetIcon(widget.data_source)" :class="['h-5 w-5', getWidgetColor(widget.color).text]" />
                  </div>
                </div>
              </div>

              <div class="pt-2">
                <div class="text-3xl font-bold text-white light:text-gray-900">
                  <template v-if="isWidgetDataLoading">
                    <Skeleton class="h-8 w-20 bg-white/[0.08] light:bg-gray-200" />
                  </template>
                  <template v-else>
                    {{ formatNumber(widgetData[widget.id]?.value || 0) }}
                  </template>
                </div>
                <div v-if="widget.show_change && widgetData[widget.id]" class="flex items-center text-xs text-white/40 light:text-gray-500 mt-1">
                  <component
                    :is="widgetData[widget.id]?.change > 0 ? TrendingUp : widgetData[widget.id]?.change < 0 ? TrendingDown : Minus"
                    :class="[
                      'h-3 w-3 mr-1',
                      widgetData[widget.id]?.change > 0 ? 'text-emerald-400' : widgetData[widget.id]?.change < 0 ? 'text-red-400' : 'text-white/30'
                    ]"
                  />
                  <span :class="widgetData[widget.id]?.change > 0 ? 'text-emerald-400' : widgetData[widget.id]?.change < 0 ? 'text-red-400' : 'text-white/30 light:text-gray-400'">
                    {{ Math.abs(widgetData[widget.id]?.change || 0).toFixed(1) }}%
                  </span>
                  <span class="ml-1">{{ comparisonPeriodLabel }}</span>
                </div>
              </div>
            </div>
          </template>
        </div>

        <!-- Recent Activity -->
        <div class="grid gap-4 md:grid-cols-2">
          <!-- Recent Messages -->
          <div class="rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-white light:border-gray-200">
            <div class="p-6 pb-3">
              <h3 class="text-lg font-semibold text-white light:text-gray-900">Recent Messages</h3>
              <p class="text-sm text-white/40 light:text-gray-500">Latest conversations from your contacts</p>
            </div>
            <div class="p-6 pt-3">
              <div class="space-y-4">
                <div
                  v-for="message in recentMessages"
                  :key="message.id"
                  class="flex items-start gap-3 p-3 rounded-lg hover:bg-white/[0.04] light:hover:bg-gray-50 transition-colors"
                >
                  <div
                    :class="[
                      'h-10 w-10 rounded-lg flex items-center justify-center text-sm font-medium',
                      message.direction === 'incoming' ? 'bg-gradient-to-br from-emerald-500 to-green-600 text-white' : 'bg-gradient-to-br from-blue-500 to-cyan-600 text-white'
                    ]"
                  >
                    {{ message.contact_name.split(' ').map(n => n[0]).join('').slice(0, 2) }}
                  </div>
                  <div class="flex-1 min-w-0">
                    <div class="flex items-center justify-between">
                      <p class="text-sm font-medium truncate text-white light:text-gray-900">{{ message.contact_name }}</p>
                      <span class="text-xs text-white/40 light:text-gray-500 flex items-center gap-1">
                        <Clock class="h-3 w-3" />
                        {{ formatTime(message.created_at) }}
                      </span>
                    </div>
                    <p class="text-sm text-white/50 light:text-gray-600 truncate">{{ message.content }}</p>
                    <div class="flex items-center gap-2 mt-1">
                      <span
                        :class="[
                          'text-[10px] px-1.5 py-0.5 rounded-full font-medium',
                          message.direction === 'incoming' ? 'bg-emerald-500/20 text-emerald-400 light:bg-emerald-100 light:text-emerald-700' : 'bg-blue-500/20 text-blue-400 light:bg-blue-100 light:text-blue-700'
                        ]"
                      >
                        {{ message.direction }}
                      </span>
                      <span v-if="message.status === 'delivered'" class="text-xs text-white/40 light:text-gray-500 flex items-center">
                        <CheckCheck class="h-3 w-3 mr-1 text-blue-400" />
                        Delivered
                      </span>
                    </div>
                  </div>
                </div>
                <div v-if="recentMessages.length === 0" class="text-center py-8 text-white/40 light:text-gray-500">
                  No recent messages
                </div>
              </div>
            </div>
          </div>

          <!-- Quick Actions -->
          <div class="rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-white light:border-gray-200">
            <div class="p-6 pb-3">
              <h3 class="text-lg font-semibold text-white light:text-gray-900">Quick Actions</h3>
              <p class="text-sm text-white/40 light:text-gray-500">Common tasks and shortcuts</p>
            </div>
            <div class="p-6 pt-3">
              <div class="grid grid-cols-2 gap-3">
                <RouterLink
                  to="/chat"
                  class="card-interactive flex flex-col items-center justify-center p-4 rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-gray-50 light:border-gray-200"
                >
                  <div class="h-12 w-12 rounded-lg bg-gradient-to-br from-emerald-500 to-green-600 flex items-center justify-center mb-2 shadow-lg shadow-emerald-500/20">
                    <MessageSquare class="h-6 w-6 text-white" />
                  </div>
                  <span class="text-sm font-medium text-white light:text-gray-900">Start Chat</span>
                </RouterLink>
                <RouterLink
                  to="/campaigns"
                  class="card-interactive flex flex-col items-center justify-center p-4 rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-gray-50 light:border-gray-200"
                >
                  <div class="h-12 w-12 rounded-lg bg-gradient-to-br from-orange-500 to-amber-600 flex items-center justify-center mb-2 shadow-lg shadow-orange-500/20">
                    <Send class="h-6 w-6 text-white" />
                  </div>
                  <span class="text-sm font-medium text-white light:text-gray-900">New Campaign</span>
                </RouterLink>
                <RouterLink
                  to="/templates"
                  class="card-interactive flex flex-col items-center justify-center p-4 rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-gray-50 light:border-gray-200"
                >
                  <div class="h-12 w-12 rounded-lg bg-gradient-to-br from-blue-500 to-cyan-600 flex items-center justify-center mb-2 shadow-lg shadow-blue-500/20">
                    <span class="text-white text-xl font-bold">T</span>
                  </div>
                  <span class="text-sm font-medium text-white light:text-gray-900">Templates</span>
                </RouterLink>
                <RouterLink
                  to="/chatbot"
                  class="card-interactive flex flex-col items-center justify-center p-4 rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-gray-50 light:border-gray-200"
                >
                  <div class="h-12 w-12 rounded-lg bg-gradient-to-br from-purple-500 to-pink-600 flex items-center justify-center mb-2 shadow-lg shadow-purple-500/20">
                    <Bot class="h-6 w-6 text-white" />
                  </div>
                  <span class="text-sm font-medium text-white light:text-gray-900">Chatbot</span>
                </RouterLink>
              </div>
            </div>
          </div>
        </div>
      </div>
    </ScrollArea>

    <!-- Widget Dialog -->
    <Dialog v-model:open="isWidgetDialogOpen">
      <DialogContent class="sm:max-w-[500px] bg-[#141414] border-white/[0.08] text-white light:bg-white light:border-gray-200 light:text-gray-900">
        <DialogHeader>
          <DialogTitle>{{ isEditMode ? 'Edit Widget' : 'Create Widget' }}</DialogTitle>
          <DialogDescription class="text-white/50 light:text-gray-500">
            Define a custom analytics widget for your dashboard
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-4">
          <!-- Name -->
          <div class="space-y-2">
            <Label class="text-white/70 light:text-gray-700">Name *</Label>
            <Input
              v-model="widgetForm.name"
              placeholder="e.g., Failed Messages"
              class="bg-white/[0.04] border-white/[0.1] text-white placeholder:text-white/30 light:bg-white light:border-gray-300 light:text-gray-900"
            />
          </div>

          <!-- Description -->
          <div class="space-y-2">
            <Label class="text-white/70 light:text-gray-700">Description</Label>
            <Textarea
              v-model="widgetForm.description"
              placeholder="Optional description"
              class="bg-white/[0.04] border-white/[0.1] text-white placeholder:text-white/30 light:bg-white light:border-gray-300 light:text-gray-900"
              :rows="2"
            />
          </div>

          <!-- Data Source -->
          <div class="space-y-2">
            <Label class="text-white/70 light:text-gray-700">Data Source *</Label>
            <Select :model-value="widgetForm.data_source" @update:model-value="(val) => widgetForm.data_source = String(val)">
              <SelectTrigger class="bg-white/[0.04] border-white/[0.1] text-white light:bg-white light:border-gray-300 light:text-gray-900">
                <SelectValue placeholder="Select data source" />
              </SelectTrigger>
              <SelectContent class="bg-[#1a1a1a] border-white/[0.08] light:bg-white light:border-gray-200">
                <SelectItem
                  v-for="source in dataSources"
                  :key="source.name"
                  :value="source.name"
                  class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100"
                >
                  {{ source.label }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <!-- Metric -->
          <div class="space-y-2">
            <Label class="text-white/70 light:text-gray-700">Metric</Label>
            <Select :model-value="widgetForm.metric" @update:model-value="(val) => widgetForm.metric = String(val)">
              <SelectTrigger class="bg-white/[0.04] border-white/[0.1] text-white light:bg-white light:border-gray-300 light:text-gray-900">
                <SelectValue placeholder="Select metric" />
              </SelectTrigger>
              <SelectContent class="bg-[#1a1a1a] border-white/[0.08] light:bg-white light:border-gray-200">
                <SelectItem value="count" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">Count</SelectItem>
                <SelectItem value="sum" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">Sum</SelectItem>
                <SelectItem value="avg" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">Average</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <!-- Filters -->
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <Label class="text-white/70 light:text-gray-700">Filters ({{ widgetForm.filters.length }})</Label>
              <Button type="button" variant="outline" size="sm" @click.stop.prevent="addFilter" class="border-white/20 text-white hover:bg-white/10 light:border-gray-300 light:text-gray-700">
                <Plus class="h-4 w-4 mr-1" />
                Add Filter
              </Button>
            </div>
            <p v-if="!widgetForm.data_source && widgetForm.filters.length === 0" class="text-xs text-white/40 light:text-gray-500">
              Select a data source first to add filters
            </p>
            <div v-for="(filter, index) in widgetForm.filters" :key="index" class="flex items-center gap-2">
              <div class="flex-1">
                <Select :model-value="filter.field" @update:model-value="(val) => filter.field = String(val)">
                  <SelectTrigger class="w-full bg-white/[0.04] border-white/[0.1] text-white text-sm light:bg-white light:border-gray-300 light:text-gray-900">
                    <SelectValue placeholder="Field" />
                  </SelectTrigger>
                  <SelectContent class="bg-[#1a1a1a] border-white/[0.08] light:bg-white light:border-gray-200">
                    <SelectItem
                      v-for="field in availableFields"
                      :key="field"
                      :value="field"
                      class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100"
                    >
                      {{ field }}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div class="w-36">
                <Select :model-value="filter.operator" @update:model-value="(val) => filter.operator = String(val)">
                  <SelectTrigger class="w-full bg-white/[0.04] border-white/[0.1] text-white text-sm light:bg-white light:border-gray-300 light:text-gray-900">
                    <SelectValue placeholder="Operator" />
                  </SelectTrigger>
                  <SelectContent class="bg-[#1a1a1a] border-white/[0.08] light:bg-white light:border-gray-200">
                    <SelectItem
                      v-for="op in operators"
                      :key="op.value"
                      :value="op.value"
                      class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100"
                    >
                      {{ op.label }}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <Input
                v-model="filter.value"
                placeholder="Value"
                class="flex-1 bg-white/[0.04] border-white/[0.1] text-white text-sm placeholder:text-white/30 light:bg-white light:border-gray-300 light:text-gray-900"
              />
              <Button variant="ghost" size="icon" @click="removeFilter(index)" class="text-white/50 hover:text-red-400 shrink-0">
                <X class="h-4 w-4" />
              </Button>
            </div>
          </div>

          <!-- Color -->
          <div class="space-y-2">
            <Label class="text-white/70 light:text-gray-700">Color</Label>
            <div class="flex gap-2">
              <button
                v-for="color in colorOptions"
                :key="color.value"
                :class="[
                  'w-8 h-8 rounded-lg flex items-center justify-center transition-all',
                  color.bg,
                  widgetForm.color === color.value ? 'ring-2 ring-white/50' : ''
                ]"
                @click="widgetForm.color = color.value"
              >
                <div :class="['w-4 h-4 rounded-full', color.text.replace('text-', 'bg-')]"></div>
              </button>
            </div>
          </div>

          <!-- Options -->
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <Switch v-model:checked="widgetForm.show_change" />
              <Label class="text-white/70 light:text-gray-700">Show % change</Label>
            </div>
            <div class="flex items-center gap-2">
              <Switch v-model:checked="widgetForm.is_shared" />
              <Label class="text-white/70 light:text-gray-700">Share with team</Label>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="isWidgetDialogOpen = false" class="border-white/[0.1] text-white/70 hover:bg-white/[0.08] light:border-gray-300 light:text-gray-700">
            Cancel
          </Button>
          <Button @click="saveWidget" :disabled="isSavingWidget">
            {{ isSavingWidget ? 'Saving...' : (isEditMode ? 'Update' : 'Create') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Delete Confirmation Dialog -->
    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent class="bg-[#141414] border-white/[0.08] light:bg-white light:border-gray-200">
        <AlertDialogHeader>
          <AlertDialogTitle class="text-white light:text-gray-900">Delete Widget</AlertDialogTitle>
          <AlertDialogDescription class="text-white/60 light:text-gray-500">
            Are you sure you want to delete "{{ widgetToDelete?.name }}"? This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel class="bg-transparent border-white/[0.1] text-white/70 hover:bg-white/[0.08] light:border-gray-300 light:text-gray-700 light:hover:bg-gray-100">
            Cancel
          </AlertDialogCancel>
          <AlertDialogAction @click="confirmDeleteWidget" class="bg-red-600 text-white hover:bg-red-700">
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
