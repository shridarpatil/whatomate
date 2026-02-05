<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { GridLayout, GridItem } from 'grid-layout-plus'
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
import { widgetsService, type DashboardWidget, type WidgetData, type LayoutItem } from '@/services/api'
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
  CalendarIcon,
  LayoutDashboard,
  Plus,
  Pencil,
  Trash2,
  BarChart3,
  FileText,
  X,
  GripVertical
} from 'lucide-vue-next'
// Centralized Chart.js setup (registered once)
import { Line, Bar, Pie } from '@/lib/charts'
import type { DateRange } from 'reka-ui'
import { CalendarDate } from '@internationalized/date'
import { useToast } from '@/components/ui/toast'

const { toast } = useToast()
const { t } = useI18n()
const authStore = useAuthStore()

// Permission checks
const canCreateWidget = computed(() => authStore.hasPermission('analytics', 'write'))
const canEditWidget = computed(() => authStore.hasPermission('analytics', 'write'))
const canDeleteWidget = computed(() => authStore.hasPermission('analytics', 'delete'))

// Widgets state
const widgets = ref<DashboardWidget[]>([])
const widgetData = ref<Record<string, WidgetData>>({})

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
  group_by_field: '',
  show_change: true,
  color: 'blue',
  size: 'small',
  config: {} as Record<string, any>,
  is_shared: false
})

// Selected shortcuts for shortcuts widget creation
const selectedShortcuts = ref<string[]>([])

// Shortcut registry
const SHORTCUT_REGISTRY = computed(() => ({
  chat: { label: t('dashboard.startChat'), to: '/chat', icon: MessageSquare, gradient: 'from-emerald-500 to-green-600' },
  campaigns: { label: t('dashboard.campaigns'), to: '/campaigns', icon: Send, gradient: 'from-orange-500 to-amber-600' },
  templates: { label: t('dashboard.templates'), to: '/templates', icon: FileText, gradient: 'from-blue-500 to-cyan-600' },
  chatbot: { label: t('dashboard.chatbot'), to: '/chatbot', icon: Bot, gradient: 'from-purple-500 to-pink-600' },
  contacts: { label: t('dashboard.contacts'), to: '/settings/contacts', icon: Users, gradient: 'from-cyan-500 to-blue-600' },
  analytics: { label: t('dashboard.analytics'), to: '/analytics', icon: BarChart3, gradient: 'from-green-500 to-emerald-600' },
}))

// Color options
const colorOptions = computed(() => [
  { value: 'blue', label: t('dashboard.colorBlue'), bg: 'bg-blue-500/20', text: 'text-blue-400' },
  { value: 'green', label: t('dashboard.colorGreen'), bg: 'bg-emerald-500/20', text: 'text-emerald-400' },
  { value: 'purple', label: t('dashboard.colorPurple'), bg: 'bg-purple-500/20', text: 'text-purple-400' },
  { value: 'orange', label: t('dashboard.colorOrange'), bg: 'bg-orange-500/20', text: 'text-orange-400' },
  { value: 'red', label: t('dashboard.colorRed'), bg: 'bg-red-500/20', text: 'text-red-400' },
  { value: 'cyan', label: t('dashboard.colorCyan'), bg: 'bg-cyan-500/20', text: 'text-cyan-400' }
])

// Chart type options
const chartTypeOptions = computed(() => [
  { value: 'line', label: t('dashboard.chartLine') },
  { value: 'bar', label: t('dashboard.chartBar') },
  { value: 'pie', label: t('dashboard.chartPie') }
])

// Chart color palette for pie charts
const chartColors = [
  'rgba(59, 130, 246, 0.8)',
  'rgba(16, 185, 129, 0.8)',
  'rgba(245, 158, 11, 0.8)',
  'rgba(139, 92, 246, 0.8)',
  'rgba(239, 68, 68, 0.8)',
  'rgba(6, 182, 212, 0.8)',
  'rgba(236, 72, 153, 0.8)',
  'rgba(234, 179, 8, 0.8)'
]

const getChartComponentData = (widget: DashboardWidget) => {
  const data = widgetData.value[widget.id]
  if (!data) return { labels: [], datasets: [] }

  const chartData = data.chart_data || []
  const dataPoints = data.data_points || []
  const groupedSeries = data.grouped_series

  // Grouped line chart: multiple datasets from grouped_series
  if (widget.chart_type === 'line' && groupedSeries && groupedSeries.datasets.length > 0) {
    return {
      labels: groupedSeries.labels,
      datasets: groupedSeries.datasets.map((ds, i) => ({
        label: ds.label,
        data: ds.data,
        borderColor: chartColors[i % chartColors.length],
        backgroundColor: chartColors[i % chartColors.length].replace('0.8)', '0.1)'),
        fill: false,
        tension: 0.3
      }))
    }
  }

  // Bar/Pie with group_by uses data_points (group → count)
  if (widget.chart_type === 'pie') {
    const source = dataPoints.length > 0 ? dataPoints : chartData
    return {
      labels: source.map((d: { label: string }) => d.label),
      datasets: [{
        data: source.map((d: { value: number }) => d.value),
        backgroundColor: chartColors.slice(0, source.length),
        borderWidth: 0
      }]
    }
  }

  if (widget.chart_type === 'bar' && dataPoints.length > 0) {
    return {
      labels: dataPoints.map((d: { label: string }) => d.label),
      datasets: [{
        label: widget.name,
        data: dataPoints.map((d: { value: number }) => d.value),
        backgroundColor: dataPoints.map((_: any, i: number) => chartColors[i % chartColors.length]),
        borderWidth: 0
      }]
    }
  }

  // Default: line and bar charts use time-series chart_data
  const colorMap: Record<string, string> = {
    blue: 'rgb(59, 130, 246)',
    green: 'rgb(16, 185, 129)',
    purple: 'rgb(139, 92, 246)',
    orange: 'rgb(245, 158, 11)',
    red: 'rgb(239, 68, 68)',
    cyan: 'rgb(6, 182, 212)'
  }
  const borderColor = colorMap[widget.color] || colorMap.blue

  return {
    labels: chartData.map((d: { label: string }) => d.label),
    datasets: [{
      label: widget.name,
      data: chartData.map((d: { value: number }) => d.value),
      borderColor,
      backgroundColor: widget.chart_type === 'bar'
        ? borderColor.replace('rgb', 'rgba').replace(')', ', 0.8)')
        : borderColor.replace('rgb', 'rgba').replace(')', ', 0.1)'),
      fill: widget.chart_type === 'line',
      tension: 0.3
    }]
  }
}

const lineBarChartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: true, position: 'top' as const }
  },
  scales: {
    y: { beginAtZero: true }
  }
}

const pieChartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { position: 'bottom' as const }
  }
}

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
      return t('dashboard.fromYesterday')
    case '7days':
      return t('dashboard.fromPrevious7Days')
    case '30days':
      return t('dashboard.fromPrevious30Days')
    case 'this_month':
      return t('dashboard.fromLastMonth')
    case 'custom':
      return t('dashboard.fromPreviousPeriod')
    default:
      return t('dashboard.fromPreviousPeriod')
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

  if (diffMins < 1) return t('dashboard.justNow')
  if (diffMins < 60) return t('dashboard.minutesAgo', { count: diffMins })
  if (diffHours < 24) return t('dashboard.hoursAgo', { count: diffHours })
  return t('dashboard.daysAgo', { count: diffDays })
}

const getWidgetColor = (color: string) => {
  const colorConfig = colorOptions.value.find(c => c.value === color) || colorOptions.value[0]
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

// Grid layout state
const GRID_COLS = 12
const GRID_ROW_HEIGHT = 40
const GRID_MARGIN: [number, number] = [16, 16]

const isDragMode = ref(false)
const gridLayout = ref<Array<{ i: string; x: number; y: number; w: number; h: number }>>([])

const isChartWidget = (widget: DashboardWidget) => widget.display_type === 'chart'
const isTableWidget = (widget: DashboardWidget) => widget.display_type === 'table'
const isShortcutsWidget = (widget: DashboardWidget) => widget.display_type === 'shortcuts'
const isNumberWidget = (widget: DashboardWidget) => !isChartWidget(widget) && !isTableWidget(widget) && !isShortcutsWidget(widget)

const getWidgetById = (id: string): DashboardWidget | undefined => {
  return widgets.value.find(w => w.id === id)
}

const computeGridLayout = (widgetList: DashboardWidget[]) => {
  const layout: Array<{ i: string; x: number; y: number; w: number; h: number }> = []

  // Separate positioned (grid_w > 0) from legacy (grid_w === 0) widgets
  const positioned = widgetList.filter(w => w.grid_w > 0)
  const legacy = widgetList.filter(w => w.grid_w === 0)

  // Add positioned widgets as-is
  for (const w of positioned) {
    layout.push({ i: w.id, x: w.grid_x, y: w.grid_y, w: w.grid_w, h: w.grid_h })
  }

  // Auto-position legacy widgets
  if (legacy.length > 0) {
    // Find the max y used by positioned widgets to place legacy below
    let nextY = 0
    if (positioned.length > 0) {
      nextY = Math.max(...positioned.map(w => w.grid_y + w.grid_h))
    }

    let curX = 0
    let curY = nextY

    // Number widgets first, then chart/table/shortcuts widgets
    const legacyNumber = legacy.filter(w => !['chart', 'table', 'shortcuts'].includes(w.display_type))
    const legacyLarge = legacy.filter(w => ['chart', 'table', 'shortcuts'].includes(w.display_type))

    for (const w of legacyNumber) {
      const itemW = 3
      const itemH = 3
      if (curX + itemW > GRID_COLS) {
        curX = 0
        curY += itemH
      }
      layout.push({ i: w.id, x: curX, y: curY, w: itemW, h: itemH })
      curX += itemW
    }

    // Move to next row for large widgets
    if (legacyNumber.length > 0 && legacyLarge.length > 0) {
      curX = 0
      curY += 3
    }

    for (const w of legacyLarge) {
      let itemW = 6
      let itemH = 5
      if (w.display_type === 'table' || w.display_type === 'shortcuts') {
        itemW = 6
        itemH = 8
      }
      if (curX + itemW > GRID_COLS) {
        curX = 0
        curY += itemH
      }
      layout.push({ i: w.id, x: curX, y: curY, w: itemW, h: itemH })
      curX += itemW
    }
  }

  return layout
}

// Rebuild grid layout when widgets change
watch(widgets, (val) => {
  gridLayout.value = computeGridLayout(val)
}, { immediate: true })

// Debounced layout save
let layoutSaveTimer: ReturnType<typeof setTimeout> | null = null

const persistLayout = async () => {
  const layoutItems: LayoutItem[] = gridLayout.value.map(item => ({
    id: item.i,
    grid_x: item.x,
    grid_y: item.y,
    grid_w: item.w,
    grid_h: item.h
  }))
  try {
    await widgetsService.saveLayout(layoutItems)
  } catch (error: any) {
    toast({
      title: t('common.error'),
      description: error.response?.data?.message || t('dashboard.saveLayoutFailed'),
      variant: 'destructive'
    })
  }
}

const onLayoutUpdate = (newLayout: Array<{ i: string; x: number; y: number; w: number; h: number }>) => {
  gridLayout.value = newLayout
  if (!isDragMode.value) return
  if (layoutSaveTimer) clearTimeout(layoutSaveTimer)
  layoutSaveTimer = setTimeout(persistLayout, 500)
}

// Save immediately when exiting drag mode
watch(isDragMode, (newVal, oldVal) => {
  if (oldVal && !newVal) {
    // Toggled off — save now
    if (layoutSaveTimer) {
      clearTimeout(layoutSaveTimer)
      layoutSaveTimer = null
    }
    persistLayout()
  }
})

const availableFields = computed(() => {
  if (!widgetForm.value.data_source) return []
  const source = dataSources.value.find(s => s.name === widgetForm.value.data_source)
  return source?.fields || []
})

// Fetch data
const fetchWidgets = async () => {
  try {
    const response = await widgetsService.list()
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
    const response = await widgetsService.getAllData({ from, to })
    widgetData.value = (response.data as any).data?.data || {}
  } catch (error) {
    console.error('Failed to load widget data:', error)
    widgetData.value = {}
  } finally {
    isWidgetDataLoading.value = false
  }
}

const fetchDataSources = async () => {
  try {
    const response = await widgetsService.getDataSources()
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
    group_by_field: '',
    show_change: true,
    color: 'blue',
    size: 'small',
    config: {},
    is_shared: false
  }
  selectedShortcuts.value = []
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
    group_by_field: widget.group_by_field || '',
    show_change: widget.show_change,
    color: widget.color || 'blue',
    size: widget.size,
    config: widget.config || {},
    is_shared: widget.is_shared
  }
  // Populate selectedShortcuts from config
  if (widget.display_type === 'shortcuts' && widget.config?.shortcuts) {
    selectedShortcuts.value = [...widget.config.shortcuts as string[]]
  } else {
    selectedShortcuts.value = []
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
  const isShortcuts = widgetForm.value.display_type === 'shortcuts'

  if (!widgetForm.value.name) {
    toast({
      title: t('dashboard.validationError'),
      description: t('dashboard.nameRequired'),
      variant: 'destructive'
    })
    return
  }

  if (!isShortcuts && !widgetForm.value.data_source) {
    toast({
      title: t('dashboard.validationError'),
      description: t('dashboard.dataSourceRequired'),
      variant: 'destructive'
    })
    return
  }

  // Clean up empty filters
  const cleanFilters = widgetForm.value.filters.filter(f => f.field && f.operator && f.value)

  // Build config
  let config: Record<string, any> = { ...widgetForm.value.config }
  if (isShortcuts) {
    config = { shortcuts: [...selectedShortcuts.value] }
  }

  const payload: Record<string, any> = {
    name: widgetForm.value.name,
    description: widgetForm.value.description,
    data_source: widgetForm.value.data_source,
    metric: widgetForm.value.metric,
    field: widgetForm.value.field,
    filters: cleanFilters,
    display_type: widgetForm.value.display_type,
    chart_type: widgetForm.value.chart_type,
    group_by_field: widgetForm.value.group_by_field,
    show_change: widgetForm.value.show_change,
    color: widgetForm.value.color,
    size: widgetForm.value.size,
    config,
    is_shared: widgetForm.value.is_shared
  }

  isSavingWidget.value = true
  try {
    if (isEditMode.value && editingWidgetId.value) {
      await widgetsService.update(editingWidgetId.value, payload)
      toast({ title: t('common.updatedSuccess', { resource: t('resources.Widget') }) })
    } else {
      await widgetsService.create(payload)
      toast({ title: t('common.createdSuccess', { resource: t('resources.Widget') }) })
    }
    isWidgetDialogOpen.value = false
    await fetchWidgets()
    await fetchWidgetData()
  } catch (error: any) {
    toast({
      title: t('common.error'),
      description: error.response?.data?.message || t('common.failedSave', { resource: t('resources.widget') }),
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
    await widgetsService.delete(widgetToDelete.value.id)
    toast({ title: t('common.deletedSuccess', { resource: t('resources.Widget') }) })
    deleteDialogOpen.value = false
    widgetToDelete.value = null
    await fetchWidgets()
    await fetchWidgetData()
  } catch (error: any) {
    toast({
      title: t('common.error'),
      description: error.response?.data?.message || t('common.failedDelete', { resource: t('resources.widget') }),
      variant: 'destructive'
    })
  }
}

// Watch for range changes
watch(selectedRange, (newValue) => {
  savePreferences()
  if (newValue !== 'custom') {
    fetchWidgetData()
  }
})

// Set default chart_type when display_type changes to chart
watch(() => widgetForm.value.display_type, (newVal) => {
  if (newVal === 'chart' && !widgetForm.value.chart_type) {
    widgetForm.value.chart_type = 'line'
  }
  if (newVal !== 'chart') {
    widgetForm.value.chart_type = ''
  }
  if (newVal !== 'chart' && newVal !== 'table') {
    widgetForm.value.group_by_field = ''
  }
  if (newVal === 'shortcuts') {
    widgetForm.value.data_source = ''
    widgetForm.value.metric = 'count'
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
          <h1 class="text-xl font-semibold text-white light:text-gray-900">{{ $t('dashboard.title') }}</h1>
          <p class="text-sm text-white/50 light:text-gray-500">{{ $t('dashboard.subtitle') }}</p>
        </div>

        <!-- Time Range Filter -->
        <div class="flex items-center gap-2">
          <Button v-if="canCreateWidget" variant="outline" size="sm" @click="openAddWidgetDialog" class="bg-white/[0.04] border-white/[0.1] text-white/70 hover:bg-white/[0.08] hover:text-white light:bg-white light:border-gray-200 light:text-gray-700">
            <Plus class="h-4 w-4 mr-2" />
            {{ $t('dashboard.addWidget') }}
          </Button>

          <Button
            v-if="canEditWidget && widgets.length > 1"
            variant="outline"
            size="sm"
            @click="isDragMode = !isDragMode"
            :class="[
              isDragMode
                ? 'bg-emerald-500/20 border-emerald-500/40 text-emerald-400 hover:bg-emerald-500/30 hover:text-emerald-300'
                : 'bg-white/[0.04] border-white/[0.1] text-white/70 hover:bg-white/[0.08] hover:text-white light:bg-white light:border-gray-200 light:text-gray-700'
            ]"
          >
            <GripVertical class="h-4 w-4 mr-2" />
            {{ isDragMode ? $t('common.done') : $t('dashboard.editLayout') }}
          </Button>

          <Select v-model="selectedRange">
            <SelectTrigger class="w-[180px] bg-white/[0.04] border-white/[0.1] text-white/70 hover:bg-white/[0.08] light:bg-white light:border-gray-200 light:text-gray-700">
              <SelectValue :placeholder="$t('dashboard.selectRange')" />
            </SelectTrigger>
            <SelectContent class="bg-[#141414] border-white/[0.08] light:bg-white light:border-gray-200">
              <SelectItem value="today" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">{{ $t('dashboard.today') }}</SelectItem>
              <SelectItem value="7days" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">{{ $t('dashboard.last7Days') }}</SelectItem>
              <SelectItem value="30days" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">{{ $t('dashboard.last30Days') }}</SelectItem>
              <SelectItem value="this_month" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">{{ $t('dashboard.thisMonth') }}</SelectItem>
              <SelectItem value="custom" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">{{ $t('dashboard.customRange') }}</SelectItem>
            </SelectContent>
          </Select>

          <Popover v-if="selectedRange === 'custom'" v-model:open="isDatePickerOpen">
            <PopoverTrigger as-child>
              <Button variant="outline" class="w-auto bg-white/[0.04] border-white/[0.1] text-white/70 hover:bg-white/[0.08] hover:text-white light:bg-white light:border-gray-200 light:text-gray-700 light:hover:bg-gray-50">
                <CalendarIcon class="h-4 w-4 mr-2" />
                {{ formatDateRange || $t('dashboard.selectDates') }}
              </Button>
            </PopoverTrigger>
            <PopoverContent class="w-auto p-4 bg-[#141414] border-white/[0.08] light:bg-white light:border-gray-200" align="end">
              <div class="space-y-4">
                <RangeCalendar v-model="customDateRange" :number-of-months="2" />
                <Button class="w-full" @click="applyCustomRange" :disabled="!customDateRange.start || !customDateRange.end">
                  {{ $t('dashboard.applyRange') }}
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
        <!-- Loading Skeleton -->
        <div v-if="isLoading" class="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
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
        </div>

        <!-- Widget Grid Layout -->
        <GridLayout
          v-if="!isLoading && gridLayout.length > 0"
          :layout="gridLayout"
          :col-num="GRID_COLS"
          :row-height="GRID_ROW_HEIGHT"
          :margin="GRID_MARGIN"
          :is-draggable="isDragMode"
          :is-resizable="isDragMode"
          :vertical-compact="true"
          :use-css-transforms="true"
          @layout-updated="onLayoutUpdate"
        >
          <GridItem
            v-for="item in gridLayout"
            :key="item.i"
            :i="item.i"
            :x="item.x"
            :y="item.y"
            :w="item.w"
            :h="item.h"
            :min-w="2"
            :min-h="2"
            drag-allow-from=".widget-drag-handle"
          >
            <!-- Number widget card -->
            <div
              v-if="getWidgetById(item.i) && isNumberWidget(getWidgetById(item.i)!)"
              class="group relative h-full card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] p-6 light:bg-white light:border-gray-200 hover:bg-white/[0.06] light:hover:bg-gray-50 transition-colors overflow-hidden"
            >
              <!-- Drag handle indicator -->
              <div v-if="isDragMode" class="widget-drag-handle absolute top-2 left-2 text-white/20 light:text-gray-300 cursor-grab active:cursor-grabbing z-10">
                <GripVertical class="h-4 w-4" />
              </div>

              <div class="flex flex-row items-start justify-between space-y-0 pb-2">
                <div class="flex-1">
                  <span class="text-sm font-medium text-white/50 light:text-gray-500">
                    {{ getWidgetById(item.i)!.name }}
                  </span>
                </div>
                <div class="flex items-center gap-2">
                  <!-- Actions - hidden in drag mode -->
                  <div v-if="!isDragMode && (canEditWidget || canDeleteWidget)" class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <Button
                      v-if="canEditWidget"
                      variant="ghost"
                      size="icon"
                      class="h-6 w-6 text-white/20 hover:text-white hover:bg-white/[0.1] light:text-gray-300 light:hover:text-gray-700 light:hover:bg-gray-100"
                      @click.stop="openEditWidgetDialog(getWidgetById(item.i)!)"
                      :title="$t('dashboard.editWidgetTooltip')"
                    >
                      <Pencil class="h-3 w-3" />
                    </Button>
                    <Button
                      v-if="canDeleteWidget"
                      variant="ghost"
                      size="icon"
                      class="h-6 w-6 text-white/20 hover:text-red-400 hover:bg-red-500/10 light:text-gray-300 light:hover:text-red-600 light:hover:bg-red-50"
                      @click.stop="openDeleteDialog(getWidgetById(item.i)!)"
                      :title="$t('dashboard.deleteWidgetTooltip')"
                    >
                      <Trash2 class="h-3 w-3" />
                    </Button>
                  </div>
                  <!-- Icon -->
                  <div :class="['h-10 w-10 rounded-lg flex items-center justify-center', getWidgetColor(getWidgetById(item.i)!.color).bg]">
                    <component :is="getWidgetIcon(getWidgetById(item.i)!.data_source)" :class="['h-5 w-5', getWidgetColor(getWidgetById(item.i)!.color).text]" />
                  </div>
                </div>
              </div>

              <div class="pt-2">
                <div class="text-3xl font-bold text-white light:text-gray-900">
                  <template v-if="isWidgetDataLoading">
                    <Skeleton class="h-8 w-20 bg-white/[0.08] light:bg-gray-200" />
                  </template>
                  <template v-else>
                    {{ formatNumber(widgetData[item.i]?.value || 0) }}
                  </template>
                </div>
                <div v-if="getWidgetById(item.i)!.show_change && widgetData[item.i]" class="flex items-center text-xs text-white/40 light:text-gray-500 mt-1">
                  <component
                    :is="widgetData[item.i]?.change > 0 ? TrendingUp : widgetData[item.i]?.change < 0 ? TrendingDown : Minus"
                    :class="[
                      'h-3 w-3 mr-1',
                      widgetData[item.i]?.change > 0 ? 'text-emerald-400' : widgetData[item.i]?.change < 0 ? 'text-red-400' : 'text-white/30'
                    ]"
                  />
                  <span :class="widgetData[item.i]?.change > 0 ? 'text-emerald-400' : widgetData[item.i]?.change < 0 ? 'text-red-400' : 'text-white/30 light:text-gray-400'">
                    {{ Math.abs(widgetData[item.i]?.change || 0).toFixed(1) }}%
                  </span>
                  <span class="ml-1">{{ comparisonPeriodLabel }}</span>
                </div>
              </div>
            </div>

            <!-- Chart widget card -->
            <div
              v-else-if="getWidgetById(item.i) && isChartWidget(getWidgetById(item.i)!)"
              class="group relative h-full flex flex-col card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] p-6 light:bg-white light:border-gray-200 hover:bg-white/[0.06] light:hover:bg-gray-50 transition-colors overflow-hidden"
            >
              <!-- Drag handle indicator -->
              <div v-if="isDragMode" class="widget-drag-handle absolute top-2 left-2 text-white/20 light:text-gray-300 cursor-grab active:cursor-grabbing z-10">
                <GripVertical class="h-4 w-4" />
              </div>

              <div class="flex flex-row items-center justify-between pb-2">
                <div>
                  <span class="text-sm font-medium text-white/50 light:text-gray-500">{{ getWidgetById(item.i)!.name }}</span>
                  <p v-if="getWidgetById(item.i)!.description" class="text-xs text-white/30 light:text-gray-400 mt-0.5">{{ getWidgetById(item.i)!.description }}</p>
                </div>
                <div class="flex items-center gap-2">
                  <div v-if="!isDragMode && (canEditWidget || canDeleteWidget)" class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <Button
                      v-if="canEditWidget"
                      variant="ghost"
                      size="icon"
                      class="h-6 w-6 text-white/20 hover:text-white hover:bg-white/[0.1] light:text-gray-300 light:hover:text-gray-700 light:hover:bg-gray-100"
                      @click.stop="openEditWidgetDialog(getWidgetById(item.i)!)"
                      :title="$t('dashboard.editWidgetTooltip')"
                    >
                      <Pencil class="h-3 w-3" />
                    </Button>
                    <Button
                      v-if="canDeleteWidget"
                      variant="ghost"
                      size="icon"
                      class="h-6 w-6 text-white/20 hover:text-red-400 hover:bg-red-500/10 light:text-gray-300 light:hover:text-red-600 light:hover:bg-red-50"
                      @click.stop="openDeleteDialog(getWidgetById(item.i)!)"
                      :title="$t('dashboard.deleteWidgetTooltip')"
                    >
                      <Trash2 class="h-3 w-3" />
                    </Button>
                  </div>
                  <div :class="['h-10 w-10 rounded-lg flex items-center justify-center', getWidgetColor(getWidgetById(item.i)!.color).bg]">
                    <component :is="getWidgetIcon(getWidgetById(item.i)!.data_source)" :class="['h-5 w-5', getWidgetColor(getWidgetById(item.i)!.color).text]" />
                  </div>
                </div>
              </div>
              <div class="flex-1 min-h-0 pt-2">
                <template v-if="isWidgetDataLoading">
                  <Skeleton class="h-full w-full bg-white/[0.08] light:bg-gray-200" />
                </template>
                <template v-else-if="(widgetData[item.i]?.chart_data?.length || 0) > 0 || (widgetData[item.i]?.data_points?.length || 0) > 0 || (widgetData[item.i]?.grouped_series?.datasets?.length || 0) > 0">
                  <Line v-if="getWidgetById(item.i)!.chart_type === 'line'" :data="getChartComponentData(getWidgetById(item.i)!)" :options="lineBarChartOptions" />
                  <Bar v-else-if="getWidgetById(item.i)!.chart_type === 'bar'" :data="getChartComponentData(getWidgetById(item.i)!)" :options="lineBarChartOptions" />
                  <Pie v-else-if="getWidgetById(item.i)!.chart_type === 'pie'" :data="getChartComponentData(getWidgetById(item.i)!)" :options="pieChartOptions" />
                </template>
                <template v-else>
                  <div class="h-full flex items-center justify-center text-white/40 light:text-gray-400">
                    {{ $t('common.noData') }}
                  </div>
                </template>
              </div>
            </div>

            <!-- Table widget card -->
            <div
              v-else-if="getWidgetById(item.i) && isTableWidget(getWidgetById(item.i)!)"
              class="group relative h-full flex flex-col card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] light:bg-white light:border-gray-200 hover:bg-white/[0.06] light:hover:bg-gray-50 transition-colors overflow-hidden"
            >
              <!-- Drag handle -->
              <div v-if="isDragMode" class="widget-drag-handle absolute top-2 left-2 text-white/20 light:text-gray-300 cursor-grab active:cursor-grabbing z-10">
                <GripVertical class="h-4 w-4" />
              </div>

              <div class="p-6 pb-3 flex flex-row items-center justify-between">
                <div>
                  <span class="text-sm font-medium text-white/50 light:text-gray-500">{{ getWidgetById(item.i)!.name }}</span>
                  <p v-if="getWidgetById(item.i)!.description" class="text-xs text-white/30 light:text-gray-400 mt-0.5">{{ getWidgetById(item.i)!.description }}</p>
                </div>
                <div class="flex items-center gap-2">
                  <div v-if="!isDragMode && (canEditWidget || canDeleteWidget)" class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <Button v-if="canEditWidget" variant="ghost" size="icon" class="h-6 w-6 text-white/20 hover:text-white hover:bg-white/[0.1] light:text-gray-300 light:hover:text-gray-700 light:hover:bg-gray-100" @click.stop="openEditWidgetDialog(getWidgetById(item.i)!)" :title="$t('dashboard.editWidgetTooltip')">
                      <Pencil class="h-3 w-3" />
                    </Button>
                    <Button v-if="canDeleteWidget" variant="ghost" size="icon" class="h-6 w-6 text-white/20 hover:text-red-400 hover:bg-red-500/10 light:text-gray-300 light:hover:text-red-600 light:hover:bg-red-50" @click.stop="openDeleteDialog(getWidgetById(item.i)!)" :title="$t('dashboard.deleteWidgetTooltip')">
                      <Trash2 class="h-3 w-3" />
                    </Button>
                  </div>
                </div>
              </div>

              <div class="flex-1 min-h-0 overflow-auto px-6 pb-6">
                <template v-if="isWidgetDataLoading">
                  <Skeleton class="h-full w-full bg-white/[0.08] light:bg-gray-200" />
                </template>
                <!-- Grouped table (group_by set) -->
                <template v-else-if="getWidgetById(item.i)!.group_by_field && widgetData[item.i]?.data_points?.length">
                  <table class="w-full">
                    <thead>
                      <tr class="border-b border-white/[0.08] light:border-gray-200">
                        <th class="text-left py-2 text-xs font-medium text-white/40 light:text-gray-500 uppercase">{{ getWidgetById(item.i)!.group_by_field }}</th>
                        <th class="text-right py-2 text-xs font-medium text-white/40 light:text-gray-500 uppercase">{{ $t('dashboard.count') }}</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="dp in widgetData[item.i]?.data_points" :key="dp.label" class="border-b border-white/[0.04] light:border-gray-100">
                        <td class="py-2 text-sm text-white/70 light:text-gray-700">{{ dp.label }}</td>
                        <td class="py-2 text-sm text-right text-white light:text-gray-900 font-medium">{{ dp.value }}</td>
                      </tr>
                    </tbody>
                  </table>
                </template>
                <!-- Row list (no group_by) -->
                <template v-else-if="widgetData[item.i]?.table_rows?.length">
                  <div class="space-y-3">
                    <div
                      v-for="row in widgetData[item.i]?.table_rows"
                      :key="row.id"
                      class="flex items-start gap-3 p-3 rounded-lg hover:bg-white/[0.04] light:hover:bg-gray-50 transition-colors"
                    >
                      <div
                        :class="[
                          'h-10 w-10 rounded-lg flex items-center justify-center text-sm font-medium shrink-0',
                          row.direction === 'incoming' ? 'bg-gradient-to-br from-emerald-500 to-green-600 text-white' : 'bg-gradient-to-br from-blue-500 to-cyan-600 text-white'
                        ]"
                      >
                        {{ row.label.split(' ').map((n: string) => n[0]).join('').slice(0, 2).toUpperCase() }}
                      </div>
                      <div class="flex-1 min-w-0">
                        <div class="flex items-center justify-between">
                          <p class="text-sm font-medium truncate text-white light:text-gray-900">{{ row.label }}</p>
                          <span class="text-xs text-white/40 light:text-gray-500 flex items-center gap-1 shrink-0">
                            <Clock class="h-3 w-3" />
                            {{ formatTime(row.created_at) }}
                          </span>
                        </div>
                        <p class="text-sm text-white/50 light:text-gray-600 truncate">{{ row.sub_label }}</p>
                        <div class="flex items-center gap-2 mt-1">
                          <span
                            v-if="row.direction"
                            :class="[
                              'text-[10px] px-1.5 py-0.5 rounded-full font-medium',
                              row.direction === 'incoming' ? 'bg-emerald-500/20 text-emerald-400 light:bg-emerald-100 light:text-emerald-700' : 'bg-blue-500/20 text-blue-400 light:bg-blue-100 light:text-blue-700'
                            ]"
                          >
                            {{ row.direction }}
                          </span>
                          <span
                            v-if="row.status"
                            :class="[
                              'text-[10px] px-1.5 py-0.5 rounded-full font-medium',
                              row.status === 'delivered' ? 'bg-blue-500/20 text-blue-400 light:bg-blue-100 light:text-blue-700' :
                              row.status === 'read' ? 'bg-emerald-500/20 text-emerald-400 light:bg-emerald-100 light:text-emerald-700' :
                              row.status === 'failed' ? 'bg-red-500/20 text-red-400 light:bg-red-100 light:text-red-700' :
                              'bg-white/10 text-white/50 light:bg-gray-100 light:text-gray-600'
                            ]"
                          >
                            {{ row.status }}
                          </span>
                        </div>
                      </div>
                    </div>
                  </div>
                </template>
                <template v-else>
                  <div class="h-full flex items-center justify-center text-white/40 light:text-gray-400">
                    {{ $t('common.noData') }}
                  </div>
                </template>
              </div>
            </div>

            <!-- Shortcuts widget card -->
            <div
              v-else-if="getWidgetById(item.i) && isShortcutsWidget(getWidgetById(item.i)!)"
              class="group relative h-full flex flex-col card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] light:bg-white light:border-gray-200 hover:bg-white/[0.06] light:hover:bg-gray-50 transition-colors overflow-hidden"
            >
              <!-- Drag handle -->
              <div v-if="isDragMode" class="widget-drag-handle absolute top-2 left-2 text-white/20 light:text-gray-300 cursor-grab active:cursor-grabbing z-10">
                <GripVertical class="h-4 w-4" />
              </div>

              <div class="p-6 pb-3 flex flex-row items-center justify-between">
                <div>
                  <span class="text-sm font-medium text-white/50 light:text-gray-500">{{ getWidgetById(item.i)!.name }}</span>
                  <p v-if="getWidgetById(item.i)!.description" class="text-xs text-white/30 light:text-gray-400 mt-0.5">{{ getWidgetById(item.i)!.description }}</p>
                </div>
                <div class="flex items-center gap-2">
                  <div v-if="!isDragMode && (canEditWidget || canDeleteWidget)" class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <Button v-if="canEditWidget" variant="ghost" size="icon" class="h-6 w-6 text-white/20 hover:text-white hover:bg-white/[0.1] light:text-gray-300 light:hover:text-gray-700 light:hover:bg-gray-100" @click.stop="openEditWidgetDialog(getWidgetById(item.i)!)" :title="$t('dashboard.editWidgetTooltip')">
                      <Pencil class="h-3 w-3" />
                    </Button>
                    <Button v-if="canDeleteWidget" variant="ghost" size="icon" class="h-6 w-6 text-white/20 hover:text-red-400 hover:bg-red-500/10 light:text-gray-300 light:hover:text-red-600 light:hover:bg-red-50" @click.stop="openDeleteDialog(getWidgetById(item.i)!)" :title="$t('dashboard.deleteWidgetTooltip')">
                      <Trash2 class="h-3 w-3" />
                    </Button>
                  </div>
                </div>
              </div>

              <div class="flex-1 min-h-0 px-6 pb-6">
                <div class="grid grid-cols-2 gap-3">
                  <template v-for="key in (getWidgetById(item.i)!.config?.shortcuts || [])" :key="key">
                    <RouterLink
                      v-if="SHORTCUT_REGISTRY[key as keyof typeof SHORTCUT_REGISTRY]"
                      :to="SHORTCUT_REGISTRY[key as keyof typeof SHORTCUT_REGISTRY].to"
                      class="card-interactive flex flex-col items-center justify-center p-4 rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-gray-50 light:border-gray-200"
                    >
                      <div :class="['h-12 w-12 rounded-lg bg-gradient-to-br flex items-center justify-center mb-2 shadow-lg', SHORTCUT_REGISTRY[key as keyof typeof SHORTCUT_REGISTRY].gradient, 'shadow-' + (key as string) + '-500/20']">
                        <component :is="SHORTCUT_REGISTRY[key as keyof typeof SHORTCUT_REGISTRY].icon" class="h-6 w-6 text-white" />
                      </div>
                      <span class="text-sm font-medium text-white light:text-gray-900">{{ SHORTCUT_REGISTRY[key as keyof typeof SHORTCUT_REGISTRY].label }}</span>
                    </RouterLink>
                  </template>
                </div>
              </div>
            </div>
          </GridItem>
        </GridLayout>

      </div>
    </ScrollArea>

    <!-- Widget Dialog -->
    <Dialog v-model:open="isWidgetDialogOpen">
      <DialogContent class="sm:max-w-[500px] bg-[#141414] border-white/[0.08] text-white light:bg-white light:border-gray-200 light:text-gray-900">
        <DialogHeader>
          <DialogTitle>{{ isEditMode ? $t('dashboard.editWidget') : $t('dashboard.createWidget') }}</DialogTitle>
          <DialogDescription class="text-white/50 light:text-gray-500">
            {{ $t('dashboard.widgetDialogDesc') }}
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-4">
          <!-- Name -->
          <div class="space-y-2">
            <Label class="text-white/70 light:text-gray-700">{{ $t('dashboard.widgetName') }} *</Label>
            <Input
              v-model="widgetForm.name"
              :placeholder="$t('dashboard.widgetNamePlaceholder')"
              class="bg-white/[0.04] border-white/[0.1] text-white placeholder:text-white/30 light:bg-white light:border-gray-300 light:text-gray-900"
            />
          </div>

          <!-- Description -->
          <div class="space-y-2">
            <Label class="text-white/70 light:text-gray-700">{{ $t('dashboard.widgetDescription') }}</Label>
            <Textarea
              v-model="widgetForm.description"
              :placeholder="$t('dashboard.widgetDescriptionPlaceholder')"
              class="bg-white/[0.04] border-white/[0.1] text-white placeholder:text-white/30 light:bg-white light:border-gray-300 light:text-gray-900"
              :rows="2"
            />
          </div>

          <!-- Data Source (hidden for shortcuts) -->
          <div v-if="widgetForm.display_type !== 'shortcuts'" class="space-y-2">
            <Label class="text-white/70 light:text-gray-700">{{ $t('dashboard.dataSource') }} *</Label>
            <Select :model-value="widgetForm.data_source" @update:model-value="(val) => widgetForm.data_source = String(val)">
              <SelectTrigger class="bg-white/[0.04] border-white/[0.1] text-white light:bg-white light:border-gray-300 light:text-gray-900">
                <SelectValue :placeholder="$t('dashboard.selectDataSource')" />
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

          <!-- Metric (hidden for shortcuts and table) -->
          <div v-if="widgetForm.display_type !== 'shortcuts' && widgetForm.display_type !== 'table'" class="space-y-2">
            <Label class="text-white/70 light:text-gray-700">{{ $t('dashboard.metric') }}</Label>
            <Select :model-value="widgetForm.metric" @update:model-value="(val) => widgetForm.metric = String(val)">
              <SelectTrigger class="bg-white/[0.04] border-white/[0.1] text-white light:bg-white light:border-gray-300 light:text-gray-900">
                <SelectValue :placeholder="$t('dashboard.selectMetric')" />
              </SelectTrigger>
              <SelectContent class="bg-[#1a1a1a] border-white/[0.08] light:bg-white light:border-gray-200">
                <SelectItem value="count" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">{{ $t('dashboard.metricCount') }}</SelectItem>
                <SelectItem value="sum" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">{{ $t('dashboard.metricSum') }}</SelectItem>
                <SelectItem value="avg" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">{{ $t('dashboard.metricAverage') }}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <!-- Display Type -->
          <div class="space-y-2">
            <Label class="text-white/70 light:text-gray-700">{{ $t('dashboard.displayType') }}</Label>
            <Select :model-value="widgetForm.display_type" @update:model-value="(val) => widgetForm.display_type = String(val)">
              <SelectTrigger class="bg-white/[0.04] border-white/[0.1] text-white light:bg-white light:border-gray-300 light:text-gray-900">
                <SelectValue :placeholder="$t('dashboard.selectDisplayType')" />
              </SelectTrigger>
              <SelectContent class="bg-[#1a1a1a] border-white/[0.08] light:bg-white light:border-gray-200">
                <SelectItem value="number" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">{{ $t('dashboard.displayNumber') }}</SelectItem>
                <SelectItem value="chart" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">{{ $t('dashboard.displayChart') }}</SelectItem>
                <SelectItem value="table" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">{{ $t('dashboard.displayTable') }}</SelectItem>
                <SelectItem value="shortcuts" class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100">{{ $t('dashboard.displayShortcuts') }}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <!-- Chart Type (visible when display type is chart) -->
          <div v-if="widgetForm.display_type === 'chart'" class="space-y-2">
            <Label class="text-white/70 light:text-gray-700">{{ $t('dashboard.chartType') }}</Label>
            <Select :model-value="widgetForm.chart_type" @update:model-value="(val) => widgetForm.chart_type = String(val)">
              <SelectTrigger class="bg-white/[0.04] border-white/[0.1] text-white light:bg-white light:border-gray-300 light:text-gray-900">
                <SelectValue :placeholder="$t('dashboard.selectChartType')" />
              </SelectTrigger>
              <SelectContent class="bg-[#1a1a1a] border-white/[0.08] light:bg-white light:border-gray-200">
                <SelectItem
                  v-for="ct in chartTypeOptions"
                  :key="ct.value"
                  :value="ct.value"
                  class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100"
                >
                  {{ ct.label }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <!-- Group By (visible when display type is chart or table, and data source is selected) -->
          <div v-if="(widgetForm.display_type === 'chart' || widgetForm.display_type === 'table') && widgetForm.data_source" class="space-y-2">
            <Label class="text-white/70 light:text-gray-700">{{ $t('dashboard.groupBy') }}</Label>
            <Select :model-value="widgetForm.group_by_field || 'none'" @update:model-value="(val) => widgetForm.group_by_field = val === 'none' ? '' : String(val)">
              <SelectTrigger class="bg-white/[0.04] border-white/[0.1] text-white light:bg-white light:border-gray-300 light:text-gray-900">
                <SelectValue :placeholder="$t('dashboard.noneTimeSeries')" />
              </SelectTrigger>
              <SelectContent class="bg-[#1a1a1a] border-white/[0.08] light:bg-white light:border-gray-200">
                <SelectItem
                  value="none"
                  class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100"
                >
                  {{ $t('dashboard.noneTimeSeries') }}
                </SelectItem>
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

          <!-- Shortcuts selector (only for shortcuts display type) -->
          <div v-if="widgetForm.display_type === 'shortcuts'" class="space-y-2">
            <Label class="text-white/70 light:text-gray-700">{{ $t('dashboard.selectShortcuts') }}</Label>
            <div class="space-y-2">
              <label
                v-for="(shortcut, key) in SHORTCUT_REGISTRY"
                :key="key"
                class="flex items-center gap-3 p-2 rounded-lg hover:bg-white/[0.04] light:hover:bg-gray-50 cursor-pointer"
              >
                <input
                  type="checkbox"
                  :value="key"
                  v-model="selectedShortcuts"
                  class="rounded border-white/20 bg-white/[0.04] text-emerald-500 focus:ring-emerald-500 light:border-gray-300 light:bg-white"
                />
                <div class="flex items-center gap-2">
                  <div :class="['h-8 w-8 rounded-lg bg-gradient-to-br flex items-center justify-center', shortcut.gradient]">
                    <component :is="shortcut.icon" class="h-4 w-4 text-white" />
                  </div>
                  <span class="text-sm text-white/70 light:text-gray-700">{{ shortcut.label }}</span>
                </div>
              </label>
            </div>
          </div>

          <!-- Filters (hidden for shortcuts) -->
          <div v-if="widgetForm.display_type !== 'shortcuts'" class="space-y-2">
            <div class="flex items-center justify-between">
              <Label class="text-white/70 light:text-gray-700">{{ $t('dashboard.filters') }} ({{ widgetForm.filters.length }})</Label>
              <Button type="button" variant="outline" size="sm" @click.stop.prevent="addFilter" class="border-white/20 text-white hover:bg-white/10 light:border-gray-300 light:text-gray-700">
                <Plus class="h-4 w-4 mr-1" />
                {{ $t('dashboard.addFilter') }}
              </Button>
            </div>
            <p v-if="!widgetForm.data_source && widgetForm.filters.length === 0" class="text-xs text-white/40 light:text-gray-500">
              {{ $t('dashboard.selectDataSourceFirst') }}
            </p>
            <div v-for="(filter, index) in widgetForm.filters" :key="index" class="flex items-center gap-2">
              <div class="flex-1">
                <Select :model-value="filter.field" @update:model-value="(val) => filter.field = String(val)">
                  <SelectTrigger class="w-full bg-white/[0.04] border-white/[0.1] text-white text-sm light:bg-white light:border-gray-300 light:text-gray-900">
                    <SelectValue :placeholder="$t('dashboard.field')" />
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
                    <SelectValue :placeholder="$t('dashboard.operator')" />
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
                :placeholder="$t('dashboard.value')"
                class="flex-1 bg-white/[0.04] border-white/[0.1] text-white text-sm placeholder:text-white/30 light:bg-white light:border-gray-300 light:text-gray-900"
              />
              <Button variant="ghost" size="icon" @click="removeFilter(index)" class="text-white/50 hover:text-red-400 shrink-0">
                <X class="h-4 w-4" />
              </Button>
            </div>
          </div>

          <!-- Color (hidden for shortcuts and table) -->
          <div v-if="widgetForm.display_type !== 'shortcuts' && widgetForm.display_type !== 'table'" class="space-y-2">
            <Label class="text-white/70 light:text-gray-700">{{ $t('dashboard.color') }}</Label>
            <Select :model-value="widgetForm.color" @update:model-value="(val) => widgetForm.color = String(val)">
              <SelectTrigger class="bg-white/[0.04] border-white/[0.1] text-white light:bg-white light:border-gray-300 light:text-gray-900">
                <SelectValue :placeholder="$t('dashboard.selectColor')" />
              </SelectTrigger>
              <SelectContent class="bg-[#1a1a1a] border-white/[0.08] light:bg-white light:border-gray-200">
                <SelectItem
                  v-for="color in colorOptions"
                  :key="color.value"
                  :value="color.value"
                  class="text-white/70 focus:bg-white/[0.08] focus:text-white light:text-gray-700 light:focus:bg-gray-100"
                >
                  <span class="flex items-center gap-2">
                    <span :class="['inline-block w-3 h-3 rounded-full', color.bg]"></span>
                    {{ color.label }}
                  </span>
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <!-- Options -->
          <div class="flex items-center justify-between">
            <div v-if="widgetForm.display_type === 'number' || widgetForm.display_type === 'percentage'" class="flex items-center gap-2">
              <Switch v-model:checked="widgetForm.show_change" />
              <Label class="text-white/70 light:text-gray-700">{{ $t('dashboard.showPercentChange') }}</Label>
            </div>
            <div class="flex items-center gap-2">
              <Switch v-model:checked="widgetForm.is_shared" />
              <Label class="text-white/70 light:text-gray-700">{{ $t('dashboard.shareWithTeam') }}</Label>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="isWidgetDialogOpen = false" class="border-white/[0.1] text-white/70 hover:bg-white/[0.08] light:border-gray-300 light:text-gray-700">
            {{ $t('common.cancel') }}
          </Button>
          <Button @click="saveWidget" :disabled="isSavingWidget">
            {{ isSavingWidget ? $t('common.saving') + '...' : (isEditMode ? $t('common.update') : $t('common.create')) }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Delete Confirmation Dialog -->
    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent class="bg-[#141414] border-white/[0.08] light:bg-white light:border-gray-200">
        <AlertDialogHeader>
          <AlertDialogTitle class="text-white light:text-gray-900">{{ $t('dashboard.deleteWidgetTitle') }}</AlertDialogTitle>
          <AlertDialogDescription class="text-white/60 light:text-gray-500">
            {{ $t('dashboard.deleteWidgetConfirm', { name: widgetToDelete?.name }) }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel class="bg-transparent border-white/[0.1] text-white/70 hover:bg-white/[0.08] light:border-gray-300 light:text-gray-700 light:hover:bg-gray-100">
            {{ $t('common.cancel') }}
          </AlertDialogCancel>
          <AlertDialogAction @click="confirmDeleteWidget" class="bg-red-600 text-white hover:bg-red-700">
            {{ $t('common.delete') }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>

<style>
/* Grid layout placeholder styling */
.vue-grid-item.vue-grid-placeholder {
  background: transparent !important;
  border: 2px dashed rgba(16, 185, 129, 0.4) !important;
  border-radius: 0.75rem;
}

/* Grid resize handle styling */
.vue-grid-item > .vue-resizable-handle {
  width: 20px;
  height: 20px;
  bottom: 4px;
  right: 4px;
  background: none;
  cursor: se-resize;
}

.vue-grid-item > .vue-resizable-handle::after {
  content: '';
  position: absolute;
  right: 4px;
  bottom: 4px;
  width: 8px;
  height: 8px;
  border-right: 2px solid rgba(255, 255, 255, 0.2);
  border-bottom: 2px solid rgba(255, 255, 255, 0.2);
  border-radius: 0 0 2px 0;
}

.light .vue-grid-item > .vue-resizable-handle::after {
  border-right-color: rgba(0, 0, 0, 0.2);
  border-bottom-color: rgba(0, 0, 0, 0.2);
}

/* Ensure grid items don't overflow */
.vue-grid-item {
  transition: all 200ms ease;
}
</style>
