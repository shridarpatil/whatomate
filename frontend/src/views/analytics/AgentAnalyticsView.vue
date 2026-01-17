<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table'
import { agentAnalyticsService, usersService } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList
} from '@/components/ui/command'
import {
  Users,
  Clock,
  CheckCircle,
  MessageSquare,
  TrendingUp,
  TrendingDown,
  Minus,
  CalendarIcon,
  BarChart3,
  Activity,
  ChevronsUpDown,
  Check,
  Coffee
} from 'lucide-vue-next'
import type { DateRange } from 'reka-ui'
import { CalendarDate } from '@internationalized/date'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line, Bar, Doughnut } from 'vue-chartjs'

// Register Chart.js components
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

interface AgentAnalyticsSummary {
  total_transfers_handled: number
  active_transfers: number
  avg_queue_time_mins: number
  avg_first_response_mins: number
  avg_resolution_mins: number
  transfers_by_source: Record<string, number>
  total_break_time_mins: number
  break_count: number
}

interface AgentPerformanceStats {
  agent_id: string
  agent_name: string
  avg_first_response_mins: number
  avg_resolution_mins: number
  transfers_handled: number
  active_transfers: number
  messages_sent: number
  total_break_time_mins: number
  break_count: number
  is_available: boolean
  current_break_start?: string
}

interface TrendPoint {
  date: string
  transfers_handled: number
  avg_response_mins: number
}

interface AgentAnalyticsResponse {
  summary: AgentAnalyticsSummary
  agent_stats?: AgentPerformanceStats[]
  trend_data: TrendPoint[]
  my_stats?: AgentPerformanceStats
}

const authStore = useAuthStore()
const isAdminOrManager = computed(() => ['admin', 'manager'].includes(authStore.user?.role?.name || ''))

const analytics = ref<AgentAnalyticsResponse | null>(null)
const isLoading = ref(true)

// Agent filter for admins/managers
interface Agent {
  id: string
  full_name: string
  role: string
}
const agents = ref<Agent[]>([])
const selectedAgentId = ref<string>('all')
const agentComboboxOpen = ref(false)

const selectedAgentName = computed(() => {
  if (selectedAgentId.value === 'all') return 'All Agents'
  const agent = agents.value.find(a => a.id === selectedAgentId.value)
  return agent?.full_name || 'Select agent'
})

// Time range filter
type TimeRangePreset = 'today' | '7days' | '30days' | 'this_month' | 'custom'

const loadSavedPreferences = () => {
  const savedRange = localStorage.getItem('agent_analytics_time_range') as TimeRangePreset | null
  const savedCustomRange = localStorage.getItem('agent_analytics_custom_range')

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
const customDateRange = ref<DateRange>(savedPrefs.customRange)
const isDatePickerOpen = ref(false)

const savePreferences = () => {
  localStorage.setItem('agent_analytics_time_range', selectedRange.value)
  if (selectedRange.value === 'custom' && customDateRange.value.start && customDateRange.value.end) {
    localStorage.setItem('agent_analytics_custom_range', JSON.stringify({
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

const formatMinutes = (mins: number): string => {
  if (!mins || mins === 0) return '0m'
  if (mins < 60) return `${Math.round(mins)}m`
  const hours = Math.floor(mins / 60)
  const remainingMins = Math.round(mins % 60)
  return remainingMins > 0 ? `${hours}h ${remainingMins}m` : `${hours}h`
}

const fetchAgents = async () => {
  if (!isAdminOrManager.value) return
  try {
    const response = await usersService.list()
    const data = response.data.data || response.data
    // Filter to only show agents
    agents.value = (data.users || data || []).filter((u: Agent) => u.role === 'agent')
  } catch (error) {
    console.error('Failed to load agents:', error)
  }
}

const fetchAnalytics = async () => {
  isLoading.value = true
  try {
    const { from, to } = getDateRange.value
    const params: { from: string; to: string; agent_id?: string } = { from, to }
    if (isAdminOrManager.value && selectedAgentId.value !== 'all') {
      params.agent_id = selectedAgentId.value
    }
    const response = await agentAnalyticsService.getSummary(params)
    const data = response.data.data || response.data
    analytics.value = data
  } catch (error) {
    console.error('Failed to load agent analytics:', error)
    analytics.value = null
  } finally {
    isLoading.value = false
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

watch(selectedAgentId, () => {
  fetchAnalytics()
})

onMounted(() => {
  fetchAgents()
  fetchAnalytics()
})

// Chart configurations
const trendChartData = computed(() => {
  if (!analytics.value?.trend_data?.length) {
    return {
      labels: [],
      datasets: []
    }
  }

  return {
    labels: analytics.value.trend_data.map(t => {
      const date = new Date(t.date)
      return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
    }),
    datasets: [
      {
        label: 'Transfers Handled',
        data: analytics.value.trend_data.map(t => t.transfers_handled),
        borderColor: 'rgb(59, 130, 246)',
        backgroundColor: 'rgba(59, 130, 246, 0.1)',
        fill: true,
        tension: 0.3
      }
    ]
  }
})

const trendChartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: {
      display: false
    }
  },
  scales: {
    y: {
      beginAtZero: true,
      ticks: {
        stepSize: 1
      }
    }
  }
}

const sourceChartData = computed(() => {
  if (!analytics.value?.summary?.transfers_by_source) {
    return {
      labels: [],
      datasets: []
    }
  }

  const sources = analytics.value.summary.transfers_by_source
  const labels = Object.keys(sources).map(s => s.charAt(0).toUpperCase() + s.slice(1))
  const data = Object.values(sources)

  return {
    labels,
    datasets: [
      {
        data,
        backgroundColor: [
          'rgba(59, 130, 246, 0.8)',
          'rgba(16, 185, 129, 0.8)',
          'rgba(245, 158, 11, 0.8)',
          'rgba(139, 92, 246, 0.8)'
        ],
        borderWidth: 0
      }
    ]
  }
})

const sourceChartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: {
      position: 'bottom' as const
    }
  }
}

const comparisonChartData = computed(() => {
  if (!analytics.value?.agent_stats?.length) {
    return {
      labels: [],
      datasets: []
    }
  }

  return {
    labels: analytics.value.agent_stats.map(a => a.agent_name || 'Unknown'),
    datasets: [
      {
        label: 'Transfers Handled',
        data: analytics.value.agent_stats.map(a => a.transfers_handled),
        backgroundColor: 'rgba(59, 130, 246, 0.8)'
      },
      {
        label: 'Messages Sent',
        data: analytics.value.agent_stats.map(a => a.messages_sent),
        backgroundColor: 'rgba(16, 185, 129, 0.8)'
      }
    ]
  }
})

const comparisonChartOptions = {
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

// Stats to display based on role
const displayStats = computed(() => {
  if (isAdminOrManager.value) {
    return analytics.value?.summary
  }
  return analytics.value?.my_stats
})
</script>

<template>
  <div class="flex flex-col h-full">
    <!-- Header -->
    <header class="border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div class="flex h-16 items-center px-6">
        <BarChart3 class="h-5 w-5 mr-3" />
        <div class="flex-1">
          <h1 class="text-xl font-semibold">Agent Analytics</h1>
          <p class="text-sm text-muted-foreground">
            {{ isAdminOrManager ? 'Performance metrics for all agents' : 'Your performance metrics' }}
          </p>
        </div>

        <!-- Agent Filter (Admin/Manager only) -->
        <div v-if="isAdminOrManager" class="flex items-center gap-2 mr-4">
          <Popover v-model:open="agentComboboxOpen">
            <PopoverTrigger as-child>
              <Button variant="outline" role="combobox" :aria-expanded="agentComboboxOpen" class="w-[200px] justify-between">
                <span class="truncate">{{ selectedAgentName }}</span>
                <ChevronsUpDown class="ml-2 h-4 w-4 shrink-0 opacity-50" />
              </Button>
            </PopoverTrigger>
            <PopoverContent class="w-[200px] p-0">
              <Command>
                <CommandInput placeholder="Search agent..." />
                <CommandList>
                  <CommandEmpty>No agent found.</CommandEmpty>
                  <CommandGroup>
                    <CommandItem
                      value="all"
                      @select="() => { selectedAgentId = 'all'; agentComboboxOpen = false }"
                    >
                      <Check :class="['mr-2 h-4 w-4', selectedAgentId === 'all' ? 'opacity-100' : 'opacity-0']" />
                      All Agents
                    </CommandItem>
                    <CommandItem
                      v-for="agent in agents"
                      :key="agent.id"
                      :value="agent.full_name"
                      @select="() => { selectedAgentId = agent.id; agentComboboxOpen = false }"
                    >
                      <Check :class="['mr-2 h-4 w-4', selectedAgentId === agent.id ? 'opacity-100' : 'opacity-0']" />
                      {{ agent.full_name }}
                    </CommandItem>
                  </CommandGroup>
                </CommandList>
              </Command>
            </PopoverContent>
          </Popover>
        </div>

        <!-- Time Range Filter -->
        <div class="flex items-center gap-2">
          <Select v-model="selectedRange">
            <SelectTrigger class="w-[180px]">
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

          <Popover v-if="selectedRange === 'custom'" v-model:open="isDatePickerOpen">
            <PopoverTrigger as-child>
              <Button variant="outline" class="w-auto">
                <CalendarIcon class="h-4 w-4 mr-2" />
                {{ formatDateRange || 'Select dates' }}
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
      </div>
    </header>

    <!-- Content -->
    <ScrollArea class="flex-1">
      <div class="p-6 space-y-6">
        <!-- Stats Cards -->
        <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-5">
          <template v-if="isLoading">
            <Card v-for="i in 5" :key="i">
              <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
                <Skeleton class="h-4 w-24" />
                <Skeleton class="h-5 w-5 rounded" />
              </CardHeader>
              <CardContent>
                <Skeleton class="h-8 w-20 mb-2" />
                <Skeleton class="h-3 w-32" />
              </CardContent>
            </Card>
          </template>
          <template v-else-if="analytics">
            <!-- Transfers Handled -->
            <Card>
              <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle class="text-sm font-medium">Transfers Handled</CardTitle>
                <CheckCircle class="h-5 w-5 text-green-500" />
              </CardHeader>
              <CardContent>
                <div class="text-2xl font-bold">
                  {{ selectedAgentId === 'all'
                    ? (analytics.summary?.total_transfers_handled ?? 0)
                    : (analytics.my_stats?.transfers_handled ?? 0) }}
                </div>
                <p class="text-xs text-muted-foreground">Completed conversations</p>
              </CardContent>
            </Card>

            <!-- Active Conversations -->
            <Card>
              <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle class="text-sm font-medium">Active Conversations</CardTitle>
                <Activity class="h-5 w-5 text-blue-500" />
              </CardHeader>
              <CardContent>
                <div class="text-2xl font-bold">
                  {{ selectedAgentId === 'all'
                    ? (analytics.summary?.active_transfers ?? 0)
                    : (analytics.my_stats?.active_transfers ?? 0) }}
                </div>
                <p class="text-xs text-muted-foreground">Currently in progress</p>
              </CardContent>
            </Card>

            <!-- Avg Resolution Time -->
            <Card>
              <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle class="text-sm font-medium">Avg Resolution Time</CardTitle>
                <Clock class="h-5 w-5 text-orange-500" />
              </CardHeader>
              <CardContent>
                <div class="text-2xl font-bold">
                  {{ formatMinutes(selectedAgentId === 'all'
                    ? (analytics.summary?.avg_resolution_mins ?? 0)
                    : (analytics.my_stats?.avg_resolution_mins ?? 0)) }}
                </div>
                <p class="text-xs text-muted-foreground">Time to resolve</p>
              </CardContent>
            </Card>

            <!-- Messages Sent (for specific agent) or Queue Time (for all agents) -->
            <Card v-if="isAdminOrManager && selectedAgentId === 'all'">
              <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle class="text-sm font-medium">Avg Queue Time</CardTitle>
                <Clock class="h-5 w-5 text-purple-500" />
              </CardHeader>
              <CardContent>
                <div class="text-2xl font-bold">
                  {{ formatMinutes(analytics.summary?.avg_queue_time_mins || 0) }}
                </div>
                <p class="text-xs text-muted-foreground">Wait before assignment</p>
              </CardContent>
            </Card>
            <Card v-else>
              <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle class="text-sm font-medium">Messages Sent</CardTitle>
                <MessageSquare class="h-5 w-5 text-purple-500" />
              </CardHeader>
              <CardContent>
                <div class="text-2xl font-bold">
                  {{ analytics.my_stats?.messages_sent || 0 }}
                </div>
                <p class="text-xs text-muted-foreground">Outgoing messages</p>
              </CardContent>
            </Card>

            <!-- Break Time -->
            <Card>
              <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle class="text-sm font-medium">Break Time</CardTitle>
                <Coffee class="h-5 w-5 text-amber-500" />
              </CardHeader>
              <CardContent>
                <div class="text-2xl font-bold">
                  {{ formatMinutes(analytics.my_stats?.total_break_time_mins ?? analytics.summary?.total_break_time_mins ?? 0) }}
                </div>
                <p class="text-xs text-muted-foreground">
                  {{ analytics.my_stats?.break_count ?? analytics.summary?.break_count ?? 0 }} breaks taken
                </p>
              </CardContent>
            </Card>
          </template>
        </div>

        <!-- Charts Row -->
        <div class="grid gap-4 md:grid-cols-2">
          <!-- Trend Chart -->
          <Card>
            <CardHeader>
              <CardTitle>Transfer Trends</CardTitle>
              <CardDescription>Transfers handled over time</CardDescription>
            </CardHeader>
            <CardContent>
              <div class="h-64">
                <template v-if="isLoading">
                  <Skeleton class="h-full w-full" />
                </template>
                <template v-else-if="trendChartData.labels.length > 0">
                  <Line :data="trendChartData" :options="trendChartOptions" />
                </template>
                <template v-else>
                  <div class="h-full flex items-center justify-center text-muted-foreground">
                    No data available
                  </div>
                </template>
              </div>
            </CardContent>
          </Card>

          <!-- Source Distribution -->
          <Card>
            <CardHeader>
              <CardTitle>Conversation Sources</CardTitle>
              <CardDescription>How conversations are initiated</CardDescription>
            </CardHeader>
            <CardContent>
              <div class="h-64">
                <template v-if="isLoading">
                  <Skeleton class="h-full w-full" />
                </template>
                <template v-else-if="sourceChartData.labels.length > 0">
                  <Doughnut :data="sourceChartData" :options="sourceChartOptions" />
                </template>
                <template v-else>
                  <div class="h-full flex items-center justify-center text-muted-foreground">
                    No data available
                  </div>
                </template>
              </div>
            </CardContent>
          </Card>
        </div>

        <!-- Agent Comparison (Admin/Manager only, when viewing all agents) -->
        <template v-if="isAdminOrManager && selectedAgentId === 'all'">
          <Card>
            <CardHeader>
              <CardTitle>Agent Comparison</CardTitle>
              <CardDescription>Performance comparison across agents</CardDescription>
            </CardHeader>
            <CardContent>
              <div class="h-64">
                <template v-if="isLoading">
                  <Skeleton class="h-full w-full" />
                </template>
                <template v-else-if="comparisonChartData.labels.length > 0">
                  <Bar :data="comparisonChartData" :options="comparisonChartOptions" />
                </template>
                <template v-else>
                  <div class="h-full flex items-center justify-center text-muted-foreground">
                    No agents found
                  </div>
                </template>
              </div>
            </CardContent>
          </Card>
        </template>
      </div>
    </ScrollArea>
  </div>
</template>
