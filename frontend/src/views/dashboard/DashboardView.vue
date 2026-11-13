<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Modal from '@/components/oat/Modal.vue'
import PageHeader from '@/components/shared/PageHeader.vue'
import LineChart from '@/components/charts/LineChart.vue'
import BarChart from '@/components/charts/BarChart.vue'
import PieChart from '@/components/charts/PieChart.vue'
import type { ChartSeries } from '@/components/charts/chart-utils'
import { orderWidgets, reorder, resize, GRID_COLS, type Placed } from '@/lib/dashboard-layout'
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
  LayoutDashboard,
  Plus,
  Pencil,
  Trash2,
  BarChart3,
  FileText,
  X,
  GripVertical,
  MoveDiagonal2,
  Megaphone,
  Settings,
  Contact,
  Workflow,
  Key,
  UserX,
  MessageSquareText,
  Webhook,
  ShieldCheck,
  Zap,
  Shield,
  LineChart as LineChartIcon,
  Tags
} from '@/lib/icons'
import { DateRangePicker } from '@/components/shared'
import { useDateRange } from '@/composables/useDateRange'
import { useAppToast } from '@/composables/useAppToast'

const { success, error: showError } = useAppToast()
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
  chat: { label: t('dashboard.startChat'), to: '/chat', icon: MessageSquare },
  campaigns: { label: t('nav.campaigns'), to: '/campaigns', icon: Megaphone },
  templates: { label: t('nav.templates'), to: '/templates', icon: FileText },
  chatbot: { label: t('nav.chatbot'), to: '/chatbot', icon: Bot },
  contacts: { label: t('nav.contacts'), to: '/settings/contacts', icon: Contact },
  flows: { label: t('nav.flows'), to: '/flows', icon: Workflow },
  transfers: { label: t('nav.transfers'), to: '/chatbot/transfers', icon: UserX },
  agentAnalytics: { label: t('nav.agentAnalytics'), to: '/analytics/agents', icon: BarChart3 },
  metaInsights: { label: t('nav.metaInsights'), to: '/analytics/meta-insights', icon: LineChartIcon },
  settings: { label: t('nav.settings'), to: '/settings', icon: Settings },
  accounts: { label: t('nav.accounts'), to: '/settings/accounts', icon: Users },
  cannedResponses: { label: t('nav.cannedResponses'), to: '/settings/canned-responses', icon: MessageSquareText },
  tags: { label: t('nav.tags'), to: '/settings/tags', icon: Tags },
  teams: { label: t('nav.teams'), to: '/settings/teams', icon: Users },
  users: { label: t('nav.users'), to: '/settings/users', icon: Users },
  roles: { label: t('nav.roles'), to: '/settings/roles', icon: Shield },
  apiKeys: { label: t('nav.apiKeys'), to: '/settings/api-keys', icon: Key },
  webhooks: { label: t('nav.webhooks'), to: '/settings/webhooks', icon: Webhook },
  customActions: { label: t('nav.customActions'), to: '/settings/custom-actions', icon: Zap },
  sso: { label: t('nav.sso'), to: '/settings/sso', icon: ShieldCheck },
}))

// Color options (stored values; mapped onto Oat's theme colours)
const colorOptions = computed(() => [
  { value: 'blue', label: t('dashboard.colorBlue') },
  { value: 'green', label: t('dashboard.colorGreen') },
  { value: 'purple', label: t('dashboard.colorPurple') },
  { value: 'orange', label: t('dashboard.colorOrange') },
  { value: 'red', label: t('dashboard.colorRed') },
  { value: 'cyan', label: t('dashboard.colorCyan') }
])

// Chart colours are CSS variables (theme.css): --widget-chart follows the card's
// data-widget-color; --chart-1..8 is the classic categorical palette.
const WIDGET_LINE = 'var(--widget-chart)'
const WIDGET_BAR = 'color-mix(in srgb, var(--widget-chart) 80%, transparent)'
const CHART_PALETTE = Array.from({ length: 8 }, (_, i) => `var(--chart-${i + 1})`)
const paletteColor = (i: number) => CHART_PALETTE[i % CHART_PALETTE.length]

// Chart type options
const chartTypeOptions = computed(() => [
  { value: 'line', label: t('dashboard.chartLine') },
  { value: 'bar', label: t('dashboard.chartBar') },
  { value: 'pie', label: t('dashboard.chartPie') }
])

interface WidgetChart {
  kind: 'line' | 'bar' | 'pie'
  labels: string[]
  series: ChartSeries[]
  values: number[]
}

const getChart = (widget: DashboardWidget): WidgetChart => {
  const data = widgetData.value[widget.id]
  const chartData = data?.chart_data || []
  const dataPoints = data?.data_points || []
  const groupedSeries = data?.grouped_series
  const kind: WidgetChart['kind'] = widget.chart_type === 'bar' || widget.chart_type === 'pie' ? widget.chart_type : 'line'
  const empty = { kind, labels: [] as string[], series: [] as ChartSeries[], values: [] as number[] }
  if (!data) return empty

  // Grouped line chart: one series per group
  if (kind === 'line' && groupedSeries && groupedSeries.datasets.length > 0) {
    return {
      ...empty,
      labels: groupedSeries.labels,
      series: groupedSeries.datasets.map((ds, i) => ({ label: ds.label, data: ds.data, color: paletteColor(i) }))
    }
  }

  // Pie/bar with group_by use data_points (group -> count)
  if (kind === 'pie') {
    const source = dataPoints.length > 0 ? dataPoints : chartData
    return { ...empty, labels: source.map(d => d.label), values: source.map(d => d.value) }
  }

  const grouped = kind === 'bar' && dataPoints.length > 0
  const source = grouped ? dataPoints : chartData
  return {
    ...empty,
    labels: source.map(d => d.label),
    series: [{ label: widget.name, data: source.map(d => d.value), color: kind === 'bar' ? WIDGET_BAR : WIDGET_LINE }],
    // Grouped bars get one palette colour per category (old Chart.js look).
    values: grouped ? source.map(d => d.value) : []
  }
}

const hasChartData = (id: string) =>
  (widgetData.value[id]?.chart_data?.length || 0) > 0 ||
  (widgetData.value[id]?.data_points?.length || 0) > 0 ||
  (widgetData.value[id]?.grouped_series?.datasets?.length || 0) > 0

// Time range filter
const {
  selectedRange,
  customDateRange,
  isDatePickerOpen,
  dateRange,
  formatDateRangeDisplay,
  applyCustomRange: applyCustomRangeBase,
} = useDateRange({ storageKey: 'dashboard' })

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

// Grid layout state: widgets render in a fixed 12-column grid; edit mode reorders them.
// Saved grid_w / grid_h are never changed here, only the flowed x / y.
const GRID_ROW_HEIGHT = 40
// Same pixel height as the old grid-layout-plus item: h rows of 40px plus 16px margins between them.
const GRID_MARGIN = 16

const isReorderMode = ref(false)
const order = ref<Placed[]>([])
let layoutDirty = false

const isChartWidget = (widget: DashboardWidget) => widget.display_type === 'chart'
const isTableWidget = (widget: DashboardWidget) => widget.display_type === 'table'
const isShortcutsWidget = (widget: DashboardWidget) => widget.display_type === 'shortcuts'
const isNumberWidget = (widget: DashboardWidget) => !isChartWidget(widget) && !isTableWidget(widget) && !isShortcutsWidget(widget)

const widgetsById = computed(() => new Map(widgets.value.map(w => [w.id, w])))

const items = computed(() => order.value.flatMap((p, index) => {
  const widget = widgetsById.value.get(p.id)
  return widget ? [{ p, widget, index }] : []
}))

const colSpan = (w: number) => Math.min(12, Math.max(1, Math.round(w)))
const widgetHeight = (h: number) => h * GRID_ROW_HEIGHT + (h - 1) * GRID_MARGIN
// Chart area = card minus padding (2 x 24), header tile (40) and gap (16); the
// "show data" toggle (18) sits below it.
const chartArea = (h: number) => Math.max(100, widgetHeight(h) - 104 - 18)
// Line/bar: SVG height (multi-series lines also show a ~24px legend on top).
const chartHeight = (h: number, multi = false) => chartArea(h) - (multi ? 24 : 0)
// Pie: diameter leaves room for the legend row below it.
const pieSize = (h: number) => `${Math.max(80, chartArea(h) - 28)}px`

// Rebuild order when widgets change
watch(widgets, (val) => {
  order.value = orderWidgets(val)
}, { immediate: true })

// Debounced layout save
let layoutSaveTimer: ReturnType<typeof setTimeout> | null = null

const persistLayout = async () => {
  layoutDirty = false
  const layoutItems: LayoutItem[] = order.value.map(item => ({
    id: item.id,
    grid_x: item.x,
    grid_y: item.y,
    grid_w: item.w,
    grid_h: item.h
  }))
  try {
    await widgetsService.saveLayout(layoutItems)
  } catch (error: any) {
    showError(t('common.error'), error.response?.data?.message || t('dashboard.saveLayoutFailed'))
  }
}

const scheduleLayoutSave = () => {
  layoutDirty = true
  if (layoutSaveTimer) clearTimeout(layoutSaveTimer)
  layoutSaveTimer = setTimeout(persistLayout, 500)
}

const moveWidget = (from: number, to: number) => {
  if (to < 0 || to >= order.value.length) return
  order.value = reorder(order.value, from, to)
  scheduleLayoutSave()
}

const resizeWidget = (index: number, w: number, h: number) => {
  const p = order.value[index]
  if (!p) return
  const next = resize(order.value, index, w, h)
  if (next[index].w === p.w && next[index].h === p.h) return
  order.value = next
  scheduleLayoutSave()
}

// Edit layout: drag a widget onto another to move it there.
const gridEl = ref<HTMLElement | null>(null)
const dragFrom = ref<number | null>(null)
const dragOver = ref<number | null>(null)
let resizing = false

const onDragStart = (index: number, e: DragEvent) => {
  if (resizing) { e.preventDefault(); return }
  dragFrom.value = index
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', String(index))
  }
}
const onDragOver = (index: number, e: DragEvent) => {
  if (dragFrom.value === null) return
  e.preventDefault()
  dragOver.value = index
}
const onDrop = (index: number) => {
  if (dragFrom.value !== null) moveWidget(dragFrom.value, index)
  onDragEnd()
}
const onDragEnd = () => {
  dragFrom.value = null
  dragOver.value = null
}

// Edit layout: drag the corner handle to resize, snapping to grid columns and rows.
const startResize = (index: number, e: PointerEvent) => {
  const start = order.value[index]
  const grid = gridEl.value
  if (!start || !grid) return
  e.preventDefault()
  const handle = e.currentTarget as HTMLElement
  handle.setPointerCapture(e.pointerId)
  resizing = true
  const gap = parseFloat(getComputedStyle(grid).columnGap) || 0
  const colStep = (grid.clientWidth + gap) / GRID_COLS
  const rowStep = GRID_ROW_HEIGHT + GRID_MARGIN
  const { clientX: x0, clientY: y0 } = e
  const { w: w0, h: h0 } = start
  const move = (ev: PointerEvent) => {
    resizeWidget(index, w0 + (ev.clientX - x0) / colStep, h0 + (ev.clientY - y0) / rowStep)
  }
  const stop = () => {
    resizing = false
    handle.removeEventListener('pointermove', move)
    handle.removeEventListener('pointerup', stop)
    handle.removeEventListener('pointercancel', stop)
  }
  handle.addEventListener('pointermove', move)
  handle.addEventListener('pointerup', stop)
  handle.addEventListener('pointercancel', stop)
}

const resizeByKey = (index: number, e: KeyboardEvent) => {
  const p = order.value[index]
  if (!p) return
  const delta: Record<string, [number, number]> = {
    ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, -1], ArrowDown: [0, 1],
  }
  const d = delta[e.key]
  if (!d) return
  e.preventDefault()
  resizeWidget(index, p.w + d[0], p.h + d[1])
}

// Save immediately when leaving edit mode
watch(isReorderMode, (on) => {
  if (on) return
  if (layoutSaveTimer) {
    clearTimeout(layoutSaveTimer)
    layoutSaveTimer = null
  }
  if (layoutDirty) persistLayout()
})

onBeforeUnmount(() => {
  if (layoutSaveTimer) {
    clearTimeout(layoutSaveTimer)
    layoutSaveTimer = null
  }
  if (layoutDirty) persistLayout()
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
    const { from, to } = dateRange.value
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
  applyCustomRangeBase()
  fetchWidgetData()
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
    showError(t('dashboard.validationError'), t('dashboard.nameRequired'))
    return
  }

  if (!isShortcuts && !widgetForm.value.data_source) {
    showError(t('dashboard.validationError'), t('dashboard.dataSourceRequired'))
    return
  }

  // Clean up empty filters
  const cleanFilters = widgetForm.value.filters.filter(f => f.field && f.operator && f.value)

  // Build config
  let config: Record<string, any> = { ...widgetForm.value.config }
  if (isShortcuts) {
    config = { shortcuts: [...selectedShortcuts.value] }
  }

  const payload = {
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
      success(t('common.updatedSuccess', { resource: t('resources.Widget') }))
    } else {
      await widgetsService.create(payload)
      success(t('common.createdSuccess', { resource: t('resources.Widget') }))
    }
    isWidgetDialogOpen.value = false
    await fetchWidgets()
    await fetchWidgetData()
  } catch (error: any) {
    showError(t('common.error'), error.response?.data?.message || t('common.failedSave', { resource: t('resources.widget') }))
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
    success(t('common.deletedSuccess', { resource: t('resources.Widget') }))
    deleteDialogOpen.value = false
    widgetToDelete.value = null
    await fetchWidgets()
    await fetchWidgetData()
  } catch (error: any) {
    showError(t('common.error'), error.response?.data?.message || t('common.failedDelete', { resource: t('resources.widget') }))
  }
}

// Watch for range changes
watch(selectedRange, (newValue) => {
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
  <div class="flex flex-col h-full">
    <PageHeader :title="$t('dashboard.title')" :description="$t('dashboard.subtitle')" :icon="LayoutDashboard">
      <template #actions>
        <button v-if="canCreateWidget" type="button" class="outline small" @click="openAddWidgetDialog">
          <Plus :size="16" />
          {{ $t('dashboard.addWidget') }}
        </button>

        <button
          v-if="canEditWidget && widgets.length > 1"
          type="button"
          class="small"
          :class="{ outline: !isReorderMode }"
          :aria-pressed="isReorderMode"
          @click="isReorderMode = !isReorderMode"
        >
          <GripVertical :size="16" />
          {{ isReorderMode ? $t('common.done') : $t('dashboard.editLayout') }}
        </button>

        <DateRangePicker
          v-model:selected-range="selectedRange"
          v-model:custom-date-range="customDateRange"
          v-model:is-date-picker-open="isDatePickerOpen"
          :format-date-range-display="formatDateRangeDisplay"
          @apply-custom="applyCustomRange"
        />
      </template>
    </PageHeader>

    <!-- Content -->
    <div class="flex-1 scroll-y" data-page-body>
      <!-- Loading Skeleton -->
      <div v-if="isLoading" class="row">
        <div v-for="i in 4" :key="i" class="col-3">
          <article class="card vstack gap-2" data-widget aria-busy="true">
            <header class="hstack items-center justify-between gap-2">
              <div role="status" class="skeleton line" data-size="label"></div>
              <div role="status" class="skeleton box" data-size="tile"></div>
            </header>
            <div role="status" class="skeleton line" data-size="value"></div>
            <div role="status" class="skeleton line" data-size="hint"></div>
          </article>
        </div>
      </div>

      <!-- Widget grid -->
      <div v-else-if="items.length > 0" ref="gridEl" class="row" :data-layout-editing="isReorderMode || undefined">
        <div
          v-for="{ p, widget, index } in items"
          :key="widget.id"
          :class="`col-${colSpan(p.w)}`"
          :style="{ height: `${widgetHeight(p.h)}px` }"
          :draggable="isReorderMode ? 'true' : undefined"
          :data-dragging="dragFrom === index || undefined"
          :data-drag-over="(dragOver === index && dragFrom !== index) || undefined"
          data-testid="dashboard-widget"
          @dragstart="onDragStart(index, $event)"
          @dragover="onDragOver(index, $event)"
          @drop.prevent="onDrop(index)"
          @dragend="onDragEnd"
        >
          <article
            class="card h-full vstack gap-2"
            :data-widget="isNumberWidget(widget) ? 'number' : widget.display_type"
            :data-widget-color="widget.color"
            data-hover-group
          >
            <button
              v-if="isReorderMode"
              type="button"
              class="ghost icon small"
              data-resize-handle
              :aria-label="$t('dashboard.resizeWidget')"
              :title="$t('dashboard.resizeWidget')"
              @pointerdown="startResize(index, $event)"
              @keydown="resizeByKey(index, $event)"
            >
              <MoveDiagonal2 :size="14" />
            </button>
            <header class="hstack items-center gap-2">
              <div class="flex-1 min-w-0">
                <h4 class="truncate">{{ widget.name }}</h4>
                <small v-if="widget.description && !isNumberWidget(widget)" class="text-light">{{ widget.description }}</small>
              </div>

              <div v-if="!isReorderMode && (canEditWidget || canDeleteWidget)" class="hstack gap-1" data-hover-reveal>
                <button
                  v-if="canEditWidget"
                  type="button"
                  class="ghost icon small"
                  data-widget-action="edit"
                  :aria-label="$t('dashboard.editWidgetTooltip')"
                  :title="$t('dashboard.editWidgetTooltip')"
                  @click="openEditWidgetDialog(widget)"
                >
                  <Pencil :size="12" />
                </button>
                <button
                  v-if="canDeleteWidget"
                  type="button"
                  class="ghost icon small"
                  data-widget-action="delete"
                  :aria-label="$t('dashboard.deleteWidgetTooltip')"
                  :title="$t('dashboard.deleteWidgetTooltip')"
                  @click="openDeleteDialog(widget)"
                >
                  <Trash2 :size="12" />
                </button>
              </div>

              <span v-if="isNumberWidget(widget) || isChartWidget(widget)" data-widget-icon aria-hidden="true">
                <component :is="getWidgetIcon(widget.data_source)" :size="20" />
              </span>
            </header>

            <!-- Number -->
            <template v-if="isNumberWidget(widget)">
              <div v-if="isWidgetDataLoading" role="status" class="skeleton line" aria-busy="true"></div>
              <h2 v-else data-widget-value>{{ formatNumber(widgetData[widget.id]?.value || 0) }}</h2>
              <small v-if="widget.show_change && widgetData[widget.id]" class="hstack items-center gap-1" data-widget-change>
                <span
                  class="hstack items-center gap-1"
                  :data-trend="widgetData[widget.id].change > 0 ? 'up' : widgetData[widget.id].change < 0 ? 'down' : 'flat'"
                >
                  <component
                    :is="widgetData[widget.id].change > 0 ? TrendingUp : widgetData[widget.id].change < 0 ? TrendingDown : Minus"
                    :size="12"
                  />
                  {{ Math.abs(widgetData[widget.id].change || 0).toFixed(1) }}%
                </span>
                {{ comparisonPeriodLabel }}
              </small>
            </template>

            <!-- Chart -->
            <template v-else-if="isChartWidget(widget)">
              <div v-if="isWidgetDataLoading" role="status" class="skeleton box" aria-busy="true"></div>
              <div
                v-else-if="hasChartData(widget.id)"
                class="flex-1 min-h-0 scroll-y"
                data-widget-chart
                :style="{ '--pie-size': pieSize(p.h) }"
              >
                <template v-for="chart in [getChart(widget)]" :key="chart.kind">
                  <LineChart
                    v-if="chart.kind === 'line'"
                    :labels="chart.labels"
                    :series="chart.series"
                    :height="chartHeight(p.h, chart.series.length > 1)"
                  />
                  <BarChart
                    v-else-if="chart.kind === 'bar'"
                    :labels="chart.labels"
                    :series="chart.series"
                    :point-colors="chart.values.length ? chart.values.map((_, i) => paletteColor(i)) : undefined"
                    :height="chartHeight(p.h)"
                  />
                  <PieChart
                    v-else
                    :labels="chart.labels"
                    :values="chart.values"
                    :colors="CHART_PALETTE"
                  />
                </template>
              </div>
              <p v-else class="text-light">{{ $t('common.noData') }}</p>
            </template>

            <!-- Table -->
            <template v-else-if="isTableWidget(widget)">
              <div v-if="isWidgetDataLoading" role="status" class="skeleton box" aria-busy="true"></div>
              <!-- Grouped table (group_by set) -->
              <div
                v-else-if="widget.group_by_field && widgetData[widget.id]?.data_points?.length"
                class="table scroll-y flex-1 min-h-0"
              >
                <table>
                  <thead>
                    <tr>
                      <th>{{ widget.group_by_field }}</th>
                      <th>{{ $t('dashboard.count') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="dp in widgetData[widget.id]?.data_points" :key="dp.label">
                      <td>{{ dp.label }}</td>
                      <td>{{ dp.value }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <!-- Row list (no group_by) -->
              <ul
                v-else-if="widgetData[widget.id]?.table_rows?.length"
                class="unstyled vstack gap-2 scroll-y flex-1 min-h-0"
                data-widget-rows
              >
                <li v-for="row in widgetData[widget.id]?.table_rows" :key="row.id" class="hstack gap-2" data-widget-row>
                  <figure data-variant="avatar" :data-direction="row.direction" :aria-label="row.label">
                    <abbr :title="row.label">{{ row.label.split(' ').map((n: string) => n[0]).join('').slice(0, 2).toUpperCase() }}</abbr>
                  </figure>
                  <div class="flex-1 min-w-0">
                    <div class="hstack items-center justify-between gap-2">
                      <strong class="truncate">{{ row.label }}</strong>
                      <small class="text-light hstack items-center gap-1 shrink-0">
                        <Clock :size="12" />
                        {{ formatTime(row.created_at) }}
                      </small>
                    </div>
                    <p class="text-light truncate">{{ row.sub_label }}</p>
                    <div class="hstack gap-2">
                      <span v-if="row.direction" class="badge" :data-variant="row.direction === 'incoming' ? 'success' : 'secondary'" :data-pill="row.direction === 'incoming' ? 'green' : 'blue'">{{ row.direction }}</span>
                      <span
                        v-if="row.status"
                        class="badge"
                        :data-variant="row.status === 'read' ? 'success' : row.status === 'failed' ? 'danger' : 'secondary'"
                        :data-pill="row.status === 'delivered' ? 'blue' : row.status === 'read' ? 'green' : row.status === 'failed' ? 'red' : 'grey'"
                      >{{ row.status }}</span>
                    </div>
                  </div>
                </li>
              </ul>
              <p v-else class="text-light">{{ $t('common.noData') }}</p>
            </template>

            <!-- Shortcuts -->
            <div
              v-else
              class="row scroll-y flex-1 min-h-0"
              data-shortcut-grid
            >
              <template v-for="key in (widget.config?.shortcuts || [])" :key="key">
                <div
                  v-if="SHORTCUT_REGISTRY[key as keyof typeof SHORTCUT_REGISTRY]"
                  :class="p.w >= 8 ? 'col-4' : 'col-6'"
                >
                  <RouterLink
                    :to="SHORTCUT_REGISTRY[key as keyof typeof SHORTCUT_REGISTRY].to"
                    class="vstack items-center justify-center gap-2 w-100"
                    :data-shortcut="key"
                  >
                    <span data-shortcut-icon aria-hidden="true">
                      <component :is="SHORTCUT_REGISTRY[key as keyof typeof SHORTCUT_REGISTRY].icon" :size="24" />
                    </span>
                    {{ SHORTCUT_REGISTRY[key as keyof typeof SHORTCUT_REGISTRY].label }}
                  </RouterLink>
                </div>
              </template>
            </div>
          </article>
        </div>
      </div>
    </div>

    <!-- Widget Dialog -->
    <Modal
      v-model:open="isWidgetDialogOpen"
      data-classic-dialog
      :title="isEditMode ? $t('dashboard.editWidget') : $t('dashboard.createWidget')"
      :description="$t('dashboard.widgetDialogDesc')"
    >
      <div class="vstack gap-4">
        <label data-field>
          {{ $t('dashboard.widgetName') }} *
          <input v-model="widgetForm.name" :placeholder="$t('dashboard.widgetNamePlaceholder')" />
        </label>

        <label data-field>
          {{ $t('dashboard.widgetDescription') }}
          <textarea v-model="widgetForm.description" :placeholder="$t('dashboard.widgetDescriptionPlaceholder')" rows="2"></textarea>
        </label>

        <!-- Data Source (hidden for shortcuts) -->
        <label v-if="widgetForm.display_type !== 'shortcuts'" data-field>
          {{ $t('dashboard.dataSource') }} *
          <select v-model="widgetForm.data_source">
            <option value="" disabled>{{ $t('dashboard.selectDataSource') }}</option>
            <option v-for="source in dataSources" :key="source.name" :value="source.name">{{ source.label }}</option>
          </select>
        </label>

        <!-- Metric (hidden for shortcuts and table) -->
        <label v-if="widgetForm.display_type !== 'shortcuts' && widgetForm.display_type !== 'table'" data-field>
          {{ $t('dashboard.metric') }}
          <select v-model="widgetForm.metric">
            <option value="count">{{ $t('dashboard.metricCount') }}</option>
            <option value="sum">{{ $t('dashboard.metricSum') }}</option>
            <option value="avg">{{ $t('dashboard.metricAverage') }}</option>
          </select>
        </label>

        <!-- Display Type -->
        <label data-field>
          {{ $t('dashboard.displayType') }}
          <select v-model="widgetForm.display_type">
            <option value="number">{{ $t('dashboard.displayNumber') }}</option>
            <option value="chart">{{ $t('dashboard.displayChart') }}</option>
            <option value="table">{{ $t('dashboard.displayTable') }}</option>
            <option value="shortcuts">{{ $t('dashboard.displayShortcuts') }}</option>
          </select>
        </label>

        <!-- Chart Type -->
        <label v-if="widgetForm.display_type === 'chart'" data-field>
          {{ $t('dashboard.chartType') }}
          <select v-model="widgetForm.chart_type">
            <option v-for="ct in chartTypeOptions" :key="ct.value" :value="ct.value">{{ ct.label }}</option>
          </select>
        </label>

        <!-- Group By -->
        <label
          v-if="(widgetForm.display_type === 'chart' || widgetForm.display_type === 'table') && widgetForm.data_source"
          data-field
        >
          {{ $t('dashboard.groupBy') }}
          <select v-model="widgetForm.group_by_field">
            <option value="">{{ $t('dashboard.noneTimeSeries') }}</option>
            <option v-for="field in availableFields" :key="field" :value="field">{{ field }}</option>
          </select>
        </label>

        <!-- Shortcuts selector -->
        <fieldset v-if="widgetForm.display_type === 'shortcuts'" class="vstack gap-2">
          <legend>{{ $t('dashboard.selectShortcuts') }}</legend>
          <label v-for="(shortcut, key) in SHORTCUT_REGISTRY" :key="key" class="hstack items-center gap-2">
            <input v-model="selectedShortcuts" type="checkbox" :value="key" />
            <component :is="shortcut.icon" :size="16" />
            {{ shortcut.label }}
          </label>
        </fieldset>

        <!-- Filters (hidden for shortcuts) -->
        <div v-if="widgetForm.display_type !== 'shortcuts'" class="vstack gap-2">
          <div class="hstack items-center justify-between">
            <strong>{{ $t('dashboard.filters') }} ({{ widgetForm.filters.length }})</strong>
            <button type="button" class="outline small" @click.stop.prevent="addFilter">
              <Plus :size="16" />
              {{ $t('dashboard.addFilter') }}
            </button>
          </div>
          <small v-if="!widgetForm.data_source && widgetForm.filters.length === 0" class="text-light">
            {{ $t('dashboard.selectDataSourceFirst') }}
          </small>
          <div v-for="(filter, index) in widgetForm.filters" :key="index" class="hstack items-center gap-2">
            <select v-model="filter.field" class="flex-1" :aria-label="$t('dashboard.field')">
              <option value="" disabled>{{ $t('dashboard.field') }}</option>
              <option v-for="field in availableFields" :key="field" :value="field">{{ field }}</option>
            </select>
            <select v-model="filter.operator" class="flex-1" :aria-label="$t('dashboard.operator')">
              <option value="" disabled>{{ $t('dashboard.operator') }}</option>
              <option v-for="op in operators" :key="op.value" :value="op.value">{{ op.label }}</option>
            </select>
            <input v-model="filter.value" class="flex-1" :placeholder="$t('dashboard.value')" :aria-label="$t('dashboard.value')" />
            <button
              type="button"
              class="ghost icon shrink-0"
              :aria-label="$t('common.remove', 'Remove')"
              :title="$t('common.remove', 'Remove')"
              @click="removeFilter(index)"
            >
              <X :size="16" />
            </button>
          </div>
        </div>

        <!-- Color (hidden for shortcuts and table) -->
        <label v-if="widgetForm.display_type !== 'shortcuts' && widgetForm.display_type !== 'table'" data-field>
          {{ $t('dashboard.color') }}
          <select v-model="widgetForm.color">
            <option v-for="color in colorOptions" :key="color.value" :value="color.value">{{ color.label }}</option>
          </select>
        </label>

        <!-- Options -->
        <label v-if="widgetForm.display_type === 'number' || widgetForm.display_type === 'percentage'" class="hstack items-center gap-2">
          <input v-model="widgetForm.show_change" type="checkbox" role="switch" />
          {{ $t('dashboard.showPercentChange') }}
        </label>
        <label class="hstack items-center gap-2">
          <input v-model="widgetForm.is_shared" type="checkbox" role="switch" />
          {{ $t('dashboard.shareWithTeam') }}
        </label>
      </div>

      <template #footer>
        <button type="button" class="outline" @click="isWidgetDialogOpen = false">{{ $t('common.cancel') }}</button>
        <button type="button" :disabled="isSavingWidget" @click="saveWidget">
          {{ isSavingWidget ? $t('common.saving') + '...' : (isEditMode ? $t('common.update') : $t('common.create')) }}
        </button>
      </template>
    </Modal>

    <!-- Delete Confirmation Dialog -->
    <Modal
      v-model:open="deleteDialogOpen"
      data-classic-dialog
      alert
      :title="$t('dashboard.deleteWidgetTitle')"
      :description="$t('dashboard.deleteWidgetConfirm', { name: widgetToDelete?.name })"
    >
      <template #footer>
        <button type="button" class="outline" @click="deleteDialogOpen = false">{{ $t('common.cancel') }}</button>
        <button type="button" data-variant="danger" @click="confirmDeleteWidget">{{ $t('common.delete') }}</button>
      </template>
    </Modal>
  </div>
</template>
