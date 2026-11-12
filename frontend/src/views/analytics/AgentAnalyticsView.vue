<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { agentAnalyticsService } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { useUsersStore } from '@/stores/users'
import { PageHeader, ErrorState, DateRangePicker } from '@/components/shared'
import Combobox from '@/components/oat/Combobox.vue'
import type { ComboOption } from '@/components/oat/combobox'
import LineChart from '@/components/charts/LineChart.vue'
import BarChart from '@/components/charts/BarChart.vue'
import PieChart from '@/components/charts/PieChart.vue'
import {
  Clock,
  CheckCircle,
  MessageSquare,
  BarChart3,
  Activity,
  Coffee
} from '@/lib/icons'
import { useDateRange } from '@/composables/useDateRange'

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

const { t } = useI18n()
const authStore = useAuthStore()
const usersStore = useUsersStore()
// Permission-driven (analytics.agents:read), not role-name based — custom
// roles with the right permission should get the same view.
const isAdminOrManager = computed(() => authStore.hasPermission('analytics.agents', 'read'))

const analytics = ref<AgentAnalyticsResponse | null>(null)
const isLoading = ref(true)
const error = ref<string | null>(null)

// Agent filter for admins/managers
interface Agent {
  id: string
  full_name: string
  role: string
}
const agents = ref<Agent[]>([])
const selectedAgentId = ref<string>('all')

// Time range filter
const {
  selectedRange,
  customDateRange,
  isDatePickerOpen,
  dateRange,
  formatDateRangeDisplay,
  applyCustomRange: applyCustomRangeBase,
} = useDateRange({ storageKey: 'agent_analytics' })

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
    await usersStore.fetchUsers()
    agents.value = usersStore.users
      .map((u) => ({ id: u.id, full_name: u.full_name, role: u.role?.name || '' }))
  } catch (error) {
    console.error('Failed to load agents:', error)
  }
}

const fetchAnalytics = async () => {
  isLoading.value = true
  error.value = null
  try {
    const { from, to } = dateRange.value
    const params: { from: string; to: string; agent_id?: string } = { from, to }
    if (isAdminOrManager.value && selectedAgentId.value !== 'all') {
      params.agent_id = selectedAgentId.value
    }
    const response = await agentAnalyticsService.getSummary(params)
    const data = response.data.data || response.data
    analytics.value = data
  } catch (err) {
    console.error('Failed to load agent analytics:', err)
    error.value = t('agentAnalytics.errorLoadingAnalytics')
    analytics.value = null
  } finally {
    isLoading.value = false
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

watch(selectedAgentId, () => {
  fetchAnalytics()
})

onMounted(() => {
  fetchAgents()
  fetchAnalytics()
})

// Chart data
const trendLabels = computed(() =>
  (analytics.value?.trend_data ?? []).map(p =>
    new Date(p.date).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
  )
)
const trendSeries = computed(() => [{
  label: t('agentAnalytics.transfersHandled'),
  data: (analytics.value?.trend_data ?? []).map(d => d.transfers_handled)
}])

const sourceEntries = computed(() => Object.entries(analytics.value?.summary?.transfers_by_source ?? {}))
const sourceLabels = computed(() => sourceEntries.value.map(([s]) => s.charAt(0).toUpperCase() + s.slice(1)))
const sourceValues = computed(() => sourceEntries.value.map(([, v]) => v))

const comparisonLabels = computed(() =>
  (analytics.value?.agent_stats ?? []).map(a => a.agent_name || 'Unknown')
)
const comparisonSeries = computed(() => [
  {
    label: t('agentAnalytics.transfersHandled'),
    data: (analytics.value?.agent_stats ?? []).map(a => a.transfers_handled)
  },
  {
    label: t('agentAnalytics.messagesSent'),
    data: (analytics.value?.agent_stats ?? []).map(a => a.messages_sent)
  }
])

const agentOptions = computed<ComboOption[]>(() => [
  { value: 'all', label: t('agentAnalytics.allAgents') },
  ...agents.value.map(a => ({ value: a.id, label: a.full_name }))
])

const isAllAgents = computed(() => isAdminOrManager.value && selectedAgentId.value === 'all')

const statTiles = computed(() => {
  const a = analytics.value
  if (!a) return []
  const all = selectedAgentId.value === 'all'
  const breaks = a.my_stats?.break_count ?? a.summary?.break_count ?? 0
  return [
    {
      key: 'handled',
      icon: CheckCircle,
      label: t('agentAnalytics.transfersHandled'),
      value: String(all ? (a.summary?.total_transfers_handled ?? 0) : (a.my_stats?.transfers_handled ?? 0)),
      hint: t('agentAnalytics.completedConversations')
    },
    {
      key: 'active',
      icon: Activity,
      label: t('agentAnalytics.activeConversations'),
      value: String(all ? (a.summary?.active_transfers ?? 0) : (a.my_stats?.active_transfers ?? 0)),
      hint: t('agentAnalytics.currentlyInProgress')
    },
    {
      key: 'resolution',
      icon: Clock,
      label: t('agentAnalytics.avgResolutionTime'),
      value: formatMinutes(all ? (a.summary?.avg_resolution_mins ?? 0) : (a.my_stats?.avg_resolution_mins ?? 0)),
      hint: t('agentAnalytics.timeToResolve')
    },
    isAllAgents.value
      ? {
          key: 'queue',
          icon: Clock,
          label: t('agentAnalytics.avgQueueTime'),
          value: formatMinutes(a.summary?.avg_queue_time_mins || 0),
          hint: t('agentAnalytics.waitBeforeAssignment')
        }
      : {
          key: 'messages',
          icon: MessageSquare,
          label: t('agentAnalytics.messagesSent'),
          value: String(a.my_stats?.messages_sent || 0),
          hint: t('agentAnalytics.outgoingMessages')
        },
    {
      key: 'break',
      icon: Coffee,
      label: t('agentAnalytics.breakTime'),
      value: formatMinutes(a.my_stats?.total_break_time_mins ?? a.summary?.total_break_time_mins ?? 0),
      hint: t('agentAnalytics.breaksTaken', { count: breaks })
    }
  ]
})

// Stats to display based on role (reserved for future use)
const _displayStats = computed(() => {
  if (isAdminOrManager.value) {
    return analytics.value?.summary
  }
  return analytics.value?.my_stats
})
void _displayStats.value // Suppress unused warning
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader
      :title="$t('agentAnalytics.title')"
      :description="isAdminOrManager ? $t('agentAnalytics.subtitle') : $t('agentAnalytics.myMetrics')"
      :icon="BarChart3"
    >
      <template #actions>
        <div class="hstack gap-2 items-center flex-wrap justify-end">
          <!-- Agent Filter (Admin/Manager only) -->
          <Combobox
            v-if="isAdminOrManager"
            v-model="selectedAgentId"
            :options="agentOptions"
            :placeholder="$t('agentAnalytics.selectAgent')"
            :search-placeholder="$t('agentAnalytics.searchAgent')"
            :empty-text="$t('agentAnalytics.noAgentFound')"
            :aria-label="$t('agentAnalytics.selectAgent')"
          />

          <!-- Time Range Filter -->
          <DateRangePicker
            v-model:selected-range="selectedRange"
            v-model:custom-date-range="customDateRange"
            v-model:is-date-picker-open="isDatePickerOpen"
            :format-date-range-display="formatDateRangeDisplay"
            @apply-custom="applyCustomRange"
          />
        </div>
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

        <!-- Stats Cards -->
        <div v-if="!error" class="row" data-testid="agent-stats">
          <template v-if="isLoading">
            <div v-for="i in 5" :key="i" class="col-4">
              <article class="card" aria-busy="true">
                <div role="status" class="skeleton line"></div>
                <div role="status" class="skeleton box"></div>
              </article>
            </div>
          </template>
          <template v-else>
            <div v-for="tile in statTiles" :key="tile.key" class="col-4">
              <article class="card">
                <div class="hstack items-center gap-2 text-light">
                  <component :is="tile.icon" :size="16" />
                  <small>{{ tile.label }}</small>
                </div>
                <h3>{{ tile.value }}</h3>
                <small class="text-light">{{ tile.hint }}</small>
              </article>
            </div>
          </template>
        </div>

        <!-- Charts Row -->
        <div v-if="!error" class="row">
          <div class="col-6">
            <article class="card">
              <h4>{{ $t('agentAnalytics.transferTrends') }}</h4>
              <div v-if="isLoading" role="status" class="skeleton box" aria-busy="true"></div>
              <LineChart
                v-else-if="trendLabels.length > 0"
                :labels="trendLabels"
                :series="trendSeries"
                :caption="$t('agentAnalytics.transferTrends')"
              />
              <p v-else class="text-light">{{ $t('agentAnalytics.noDataAvailable') }}</p>
            </article>
          </div>

          <div class="col-6">
            <article class="card">
              <h4>{{ $t('agentAnalytics.conversationSources') }}</h4>
              <div v-if="isLoading" role="status" class="skeleton box" aria-busy="true"></div>
              <PieChart
                v-else-if="sourceLabels.length > 0"
                donut
                :labels="sourceLabels"
                :values="sourceValues"
                :caption="$t('agentAnalytics.conversationSources')"
              />
              <p v-else class="text-light">{{ $t('agentAnalytics.noDataAvailable') }}</p>
            </article>
          </div>
        </div>

        <!-- Agent Comparison (Admin/Manager only, when viewing all agents) -->
        <article v-if="!error && isAllAgents" class="card">
          <h4>{{ $t('agentAnalytics.agentComparison') }}</h4>
          <div v-if="isLoading" role="status" class="skeleton box" aria-busy="true"></div>
          <BarChart
            v-else-if="comparisonLabels.length > 0"
            :labels="comparisonLabels"
            :series="comparisonSeries"
            :caption="$t('agentAnalytics.agentComparison')"
          />
          <p v-else class="text-light">{{ $t('agentAnalytics.noAgentsFound') }}</p>
        </article>
      </div>
    </div>
  </div>
</template>
