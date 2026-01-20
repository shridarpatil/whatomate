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
import { analyticsService } from '@/services/api'
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
  LayoutDashboard
} from 'lucide-vue-next'
import type { DateRange } from 'reka-ui'
import { type DateValue, CalendarDate } from '@internationalized/date'

interface DashboardStats {
  total_messages: number
  messages_change: number
  total_contacts: number
  contacts_change: number
  chatbot_sessions: number
  chatbot_change: number
  campaigns_sent: number
  campaigns_change: number
}

interface RecentMessage {
  id: string
  contact_name: string
  content: string
  direction: string
  created_at: string
  status: string
}

const stats = ref<DashboardStats>({
  total_messages: 0,
  messages_change: 0,
  total_contacts: 0,
  contacts_change: 0,
  chatbot_sessions: 0,
  chatbot_change: 0,
  campaigns_sent: 0,
  campaigns_change: 0
})

const recentMessages = ref<RecentMessage[]>([])
const isLoading = ref(true)

// Time range filter
type TimeRangePreset = 'today' | '7days' | '30days' | 'this_month' | 'custom'

// Load saved preferences from localStorage
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
const customDateRange = ref<DateRange>(savedPrefs.customRange)
const isDatePickerOpen = ref(false)

// Save preferences to localStorage
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

const timeRangeOptions = [
  { value: 'today', label: 'Today' },
  { value: '7days', label: 'Last 7 days' },
  { value: '30days', label: 'Last 30 days' },
  { value: 'this_month', label: 'This month' },
  { value: 'custom', label: 'Custom range' }
]

// Format date as YYYY-MM-DD in local timezone (avoids UTC conversion issues)
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

const statCards = [
  {
    title: 'Total Messages',
    key: 'total_messages',
    changeKey: 'messages_change',
    icon: MessageSquare,
    color: 'text-blue-500'
  },
  {
    title: 'Contacts',
    key: 'total_contacts',
    changeKey: 'contacts_change',
    icon: Users,
    color: 'text-green-500'
  },
  {
    title: 'Chatbot Sessions',
    key: 'chatbot_sessions',
    changeKey: 'chatbot_change',
    icon: Bot,
    color: 'text-purple-500'
  },
  {
    title: 'Campaigns Sent',
    key: 'campaigns_sent',
    changeKey: 'campaigns_change',
    icon: Send,
    color: 'text-orange-500'
  }
]

const formatNumber = (num: number): string => {
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return num.toString()
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

const fetchDashboardData = async () => {
  isLoading.value = true
  try {
    const { from, to } = getDateRange.value
    const response = await analyticsService.dashboard({ from, to })
    // API response is wrapped in { status: "success", data: { stats: {...}, recent_messages: [...] } }
    const data = response.data.data || response.data
    stats.value = data.stats || {
      total_messages: 0,
      messages_change: 0,
      total_contacts: 0,
      contacts_change: 0,
      chatbot_sessions: 0,
      chatbot_change: 0,
      campaigns_sent: 0,
      campaigns_change: 0
    }
    recentMessages.value = data.recent_messages || []
  } catch (error) {
    console.error('Failed to load dashboard data:', error)
    // Use empty data on error
    stats.value = {
      total_messages: 0,
      messages_change: 0,
      total_contacts: 0,
      contacts_change: 0,
      chatbot_sessions: 0,
      chatbot_change: 0,
      campaigns_sent: 0,
      campaigns_change: 0
    }
    recentMessages.value = []
  } finally {
    isLoading.value = false
  }
}

const applyCustomRange = () => {
  if (customDateRange.value.start && customDateRange.value.end) {
    isDatePickerOpen.value = false
    savePreferences()
    fetchDashboardData()
  }
}

// Watch for range changes (fetch data for preset ranges, custom range uses Apply button)
watch(selectedRange, (newValue) => {
  savePreferences()
  if (newValue !== 'custom') {
    fetchDashboardData()
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
          <p class="text-sm text-white/50 light:text-gray-500">Overview of your messaging platform</p>
        </div>

        <!-- Time Range Filter -->
        <div class="flex items-center gap-2">
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

          <!-- Custom Range Popover -->
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
        <!-- Stats Cards -->
        <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          <!-- Skeleton Loading State -->
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
          <!-- Actual Stats -->
          <template v-else>
            <div v-for="card in statCards" :key="card.key" class="card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] p-6 light:bg-white light:border-gray-200">
              <div class="flex flex-row items-center justify-between space-y-0 pb-2">
                <span class="text-sm font-medium text-white/50 light:text-gray-500">
                  {{ card.title }}
                </span>
                <div :class="[
                  'h-10 w-10 rounded-lg flex items-center justify-center',
                  card.key === 'total_messages' ? 'bg-blue-500/20' : '',
                  card.key === 'total_contacts' ? 'bg-emerald-500/20' : '',
                  card.key === 'chatbot_sessions' ? 'bg-purple-500/20' : '',
                  card.key === 'campaigns_sent' ? 'bg-orange-500/20' : ''
                ]">
                  <component :is="card.icon" :class="[
                    'h-5 w-5',
                    card.key === 'total_messages' ? 'text-blue-400' : '',
                    card.key === 'total_contacts' ? 'text-emerald-400' : '',
                    card.key === 'chatbot_sessions' ? 'text-purple-400' : '',
                    card.key === 'campaigns_sent' ? 'text-orange-400' : ''
                  ]" />
                </div>
              </div>
              <div class="pt-2">
                <div class="text-3xl font-bold text-white light:text-gray-900">
                  {{ formatNumber(stats[card.key as keyof DashboardStats] as number) }}
                </div>
                <div class="flex items-center text-xs text-white/40 light:text-gray-500 mt-1">
                  <component
                    :is="(stats[card.changeKey as keyof DashboardStats] as number) > 0 ? TrendingUp : (stats[card.changeKey as keyof DashboardStats] as number) < 0 ? TrendingDown : Minus"
                    :class="[
                      'h-3 w-3 mr-1',
                      (stats[card.changeKey as keyof DashboardStats] as number) > 0 ? 'text-emerald-400' : (stats[card.changeKey as keyof DashboardStats] as number) < 0 ? 'text-red-400' : 'text-white/30'
                    ]"
                  />
                  <span :class="(stats[card.changeKey as keyof DashboardStats] as number) > 0 ? 'text-emerald-400' : (stats[card.changeKey as keyof DashboardStats] as number) < 0 ? 'text-red-400' : 'text-white/30 light:text-gray-400'">
                    {{ Math.abs(stats[card.changeKey as keyof DashboardStats] as number).toFixed(1) }}%
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
  </div>
</template>
