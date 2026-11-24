<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Modal from '@/components/oat/Modal.vue'
import { PageHeader, IconButton, ErrorState, ConfirmDialog } from '@/components/shared'
import { chatbotService, type Team } from '@/services/api'
import { useTransfersStore, type AgentTransfer, getSLAStatus } from '@/stores/transfers'
import { useAuthStore } from '@/stores/auth'
import { useUsersStore } from '@/stores/users'
import { useTeamsStore } from '@/stores/teams'
import { toast } from '@/lib/toast'
import { useRouter } from 'vue-router'
import { UserX, Play, MessageSquare, User, Clock, Users, UserPlus, AlertTriangle, CheckCircle2, XCircle } from '@/lib/icons'
import { getErrorMessage } from '@/lib/api-utils'

const { t } = useI18n()

const router = useRouter()
const transfersStore = useTransfersStore()
const authStore = useAuthStore()
const usersStore = useUsersStore()
const teamsStore = useTeamsStore()

const isLoading = ref(true)
const error = ref<string | null>(null)
const isPicking = ref(false)
// Org-level kill switch surfaced from chatbot settings. The backend rejects
// pickup with 403 when this is false (PickNextTransfer in agent_transfers.go),
// so the UI must hide / disable the action to match. Default true mirrors the
// server default so we don't briefly disable the button while settings load.
const allowQueuePickup = ref(true)
const isAssigning = ref(false)
const isResuming = ref(false)
const activeTab = ref('my-transfers')
const assignDialogOpen = ref(false)
const resumeDialogOpen = ref(false)
const transferToResume = ref<AgentTransfer | null>(null)
const transferToAssign = ref<AgentTransfer | null>(null)
const selectedAgentId = ref<string>('')
const selectedTeamId = ref<string>('')
const agents = ref<{ id: string; full_name: string }[]>([])
const teams = ref<Team[]>([])
const selectedTeamFilter = ref<string>('all')

// Backend grants full transfer visibility on transfers:write (agent_transfers.go:110).
// Honor the same permission here instead of hardcoding role names — otherwise
// custom roles with the right permissions still see the agent-only view.
const isAdminOrManager = computed(() => authStore.hasPermission('transfers', 'write'))
const currentUserId = computed(() => authStore.user?.id)

const myTransfers = computed(() =>
  transfersStore.transfers.filter(t =>
    t.status === 'active' && t.agent_id === currentUserId.value
  )
)

const queueTransfers = computed(() => {
  let transfers = transfersStore.transfers.filter(t =>
    t.status === 'active' && !t.agent_id
  )
  // Apply team filter
  if (selectedTeamFilter.value !== 'all') {
    if (selectedTeamFilter.value === 'general') {
      transfers = transfers.filter(t => !t.team_id)
    } else {
      transfers = transfers.filter(t => t.team_id === selectedTeamFilter.value)
    }
  }
  return transfers
})

// Team queue counts for display
const teamQueueCounts = computed(() => {
  const counts: Record<string, number> = { general: 0 }
  transfersStore.transfers.filter(t => t.status === 'active' && !t.agent_id).forEach(t => {
    if (!t.team_id) {
      counts.general++
    } else {
      counts[t.team_id] = (counts[t.team_id] || 0) + 1
    }
  })
  return counts
})

const allActiveTransfers = computed(() =>
  transfersStore.transfers.filter(t => t.status === 'active')
)

// Use store's history transfers with pagination
const historyTransfers = computed(() => transfersStore.historyTransfers)
const hasMoreHistory = computed(() => transfersStore.hasMoreHistory)
const isLoadingHistory = computed(() => transfersStore.isLoadingHistory)
const historyTotalCount = computed(() => transfersStore.historyTotalCount)

// Fetch history when switching to history tab
watch(activeTab, async (newTab) => {
  if (newTab === 'history' && historyTransfers.value.length === 0) {
    await transfersStore.fetchHistory()
  }
})

onMounted(async () => {
  await Promise.all([fetchTransfers(), fetchTeams(), fetchAllowQueuePickup()])
  // Always try to fetch agents for admin/manager - the API will reject if unauthorized
  if (isAdminOrManager.value) {
    await fetchAgents()
  }
  // No polling - WebSocket handles real-time updates
  // Reconnection refresh handles sync after disconnect
})

async function fetchAllowQueuePickup() {
  // Agents (no transfers:write) are gated by allow_agent_queue_pickup; admins
  // bypass the toggle, so we only need the setting for the agent-only view.
  if (isAdminOrManager.value) return
  try {
    const resp = await chatbotService.getSettings()
    // API returns { data: { settings: {...}, stats: {...} } }.
    const settings = resp.data?.data?.settings ?? resp.data?.settings
    if (typeof settings?.allow_agent_queue_pickup === 'boolean') {
      allowQueuePickup.value = settings.allow_agent_queue_pickup
    }
  } catch {
    // Settings endpoint may be unavailable for some users; fall back to the
    // server default (true) — backend will still 403 if pickup is disabled.
  }
}

async function fetchTransfers() {
  isLoading.value = true
  error.value = null
  try {
    await transfersStore.fetchTransfers({ status: 'active' })
  } catch (err) {
    console.error('Failed to load transfers:', err)
    error.value = t('agentTransfers.fetchError')
  } finally {
    isLoading.value = false
  }
}

async function fetchAgents() {
  try {
    await usersStore.fetchUsers()
    agents.value = usersStore.users
      .filter((u) => u.is_active !== false)
      .map((u) => ({ id: u.id, full_name: u.full_name }))
  } catch {
    toast.error(t('agentTransfers.failedLoadAgents'))
  }
}

async function fetchTeams() {
  try {
    await teamsStore.fetchTeams()
    teams.value = teamsStore.teams.filter((t: Team) => t.is_active)
  } catch {
    teams.value = []
  }
}

function getTeamName(teamId: string | undefined): string {
  if (!teamId) return t('agentTransfers.generalQueue')
  const team = teams.value.find(t => t.id === teamId)
  return team?.name || t('agentTransfers.generalQueue')
}

async function pickNextTransfer() {
  isPicking.value = true
  try {
    const response = await chatbotService.pickNextTransfer()
    const data = response.data.data || response.data

    if (data.transfer) {
      toast.success(t('agentTransfers.transferPicked'), {
        description: t('agentTransfers.assignedToContact', { contact: data.transfer.contact_name || data.transfer.phone_number })
      })
      await fetchTransfers()

      // Navigate to chat
      router.push(`/chat/${data.transfer.contact_id}`)
    } else {
      toast.info(t('agentTransfers.noTransfersInQueueInfo'))
    }
  } catch (error) {
    toast.error(getErrorMessage(error, t('agentTransfers.failedPickTransfer')))
  } finally {
    isPicking.value = false
  }
}

function openResumeDialog(transfer: AgentTransfer) {
  transferToResume.value = transfer
  resumeDialogOpen.value = true
}

async function confirmResumeTransfer() {
  if (!transferToResume.value) return

  isResuming.value = true
  try {
    await chatbotService.resumeTransfer(transferToResume.value.id)
    toast.success(t('agentTransfers.transferResumed'), {
      description: t('agentTransfers.chatbotNowActive')
    })
    resumeDialogOpen.value = false
    transferToResume.value = null
    await fetchTransfers()
  } catch (err) {
    toast.error(getErrorMessage(err, t('agentTransfers.failedResumeTransfer')))
  } finally {
    isResuming.value = false
  }
}

async function openAssignDialog(transfer: AgentTransfer) {
  transferToAssign.value = transfer
  selectedAgentId.value = transfer.agent_id || 'unassigned'
  selectedTeamId.value = transfer.team_id || 'general'
  assignDialogOpen.value = true

  // Fetch agents if not already loaded
  if (agents.value.length === 0) {
    await fetchAgents()
  }
}

async function assignTransfer() {
  if (!transferToAssign.value) return

  isAssigning.value = true
  try {
    // Map "unassigned" to null for the API
    const agentId = selectedAgentId.value === 'unassigned' ? null : selectedAgentId.value
    // Map "general" to empty string (general queue), otherwise pass team_id
    // Only pass team_id if it changed from the original
    const originalTeamId = transferToAssign.value.team_id || 'general'
    let teamId: string | null | undefined = undefined
    if (selectedTeamId.value !== originalTeamId) {
      teamId = selectedTeamId.value === 'general' ? '' : selectedTeamId.value
    }

    await chatbotService.assignTransfer(
      transferToAssign.value.id,
      agentId,
      teamId
    )
    toast.success(t('common.updatedSuccess', { resource: t('resources.Transfer') }))
    assignDialogOpen.value = false
    await fetchTransfers()
  } catch (error) {
    toast.error(getErrorMessage(error, t('agentTransfers.failedAssignTransfer')))
  } finally {
    isAssigning.value = false
  }
}

function viewChat(transfer: AgentTransfer) {
  router.push(`/chat/${transfer.contact_id}`)
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleString()
}

function getSourceBadge(source: string) {
  switch (source) {
    case 'flow':
      return { label: t('agentTransfers.flow'), variant: 'secondary' as const }
    case 'keyword':
      return { label: t('agentTransfers.keyword'), variant: 'outline' as const }
    default:
      return { label: t('agentTransfers.manual'), variant: 'default' as const }
  }
}

// Map the old shadcn badge variants onto Oat's data-variant / outline class.
function badgeVariant(variant: string): string | undefined {
  switch (variant) {
    case 'destructive': return 'danger'
    case 'warning': return 'warning'
    case 'secondary': return 'secondary'
    default: return undefined
  }
}

function slaTitle(transfer: AgentTransfer, showPickedUp = false): string {
  const lines: string[] = []
  if (showPickedUp && transfer.picked_up_at) {
    lines.push(`${t('agentTransfers.pickedUpAt')}: ${formatDate(transfer.picked_up_at)}`)
  } else if (transfer.sla_response_deadline) {
    lines.push(`${t('agentTransfers.responseDeadline')}: ${formatDate(transfer.sla_response_deadline)}`)
  }
  if (transfer.escalation_level > 0) {
    lines.push(`${t('agentTransfers.escalationLevel')}: ${transfer.escalation_level}`)
  }
  if (transfer.sla_breached && transfer.sla_breached_at) {
    lines.push(`${t('agentTransfers.breachedAt')}: ${formatDate(transfer.sla_breached_at)}`)
  }
  return lines.join('\n')
}

const tabs = computed(() => [
  { value: 'my-transfers', label: t('agentTransfers.myTransfers'), count: myTransfers.value.length, variant: 'secondary' },
  { value: 'queue', label: t('agentTransfers.queue'), count: queueTransfers.value.length, variant: 'danger' },
  { value: 'all', label: t('agentTransfers.allActive') },
  { value: 'history', label: t('agentTransfers.history') },
] as { value: string; label: string; count?: number; variant?: string }[])

function getSLABadge(transfer: AgentTransfer) {
  const status = getSLAStatus(transfer)
  switch (status) {
    case 'breached':
      return { label: t('agentTransfers.slaBreached'), variant: 'destructive' as const, icon: 'xcircle' }
    case 'warning':
      return { label: t('agentTransfers.atRisk'), variant: 'warning' as const, icon: 'alert' }
    case 'expired':
      return { label: t('agentTransfers.expired'), variant: 'secondary' as const, icon: 'xcircle' }
    default:
      return { label: t('agentTransfers.onTrack'), variant: 'outline' as const, icon: 'check' }
  }
}

function formatTimeRemaining(deadline: string | undefined): string {
  if (!deadline) return '-'
  const now = new Date()
  const deadlineDate = new Date(deadline)
  const diff = deadlineDate.getTime() - now.getTime()

  if (diff <= 0) return t('agentTransfers.overdue')

  const minutes = Math.floor(diff / 60000)
  const hours = Math.floor(minutes / 60)

  if (hours > 0) {
    return `${hours}h ${minutes % 60}m`
  }
  return `${minutes}m`
}
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader :title="$t('agentTransfers.title')" :description="$t('agentTransfers.subtitle')" :icon="UserX">
      <template v-if="!isAdminOrManager" #actions>
        <div class="hstack gap-4">
          <small class="text-light hstack gap-1">
            <Users :size="16" />
            {{ $t('agentTransfers.waitingInQueue', { count: transfersStore.queueCount }) }}
          </small>
          <span :title="!allowQueuePickup ? $t('agentTransfers.queuePickupDisabled') : undefined">
            <button
              type="button"
              class="outline small"
              :disabled="!allowQueuePickup || isPicking || transfersStore.queueCount === 0"
              :aria-busy="isPicking ? 'true' : undefined"
              data-spinner="small"
              @click="pickNextTransfer"
            >
              <Play v-if="!isPicking" :size="16" />
              {{ $t('agentTransfers.pickNext') }}
            </button>
          </span>
        </div>
      </template>
    </PageHeader>

    <!-- Content -->
    <div class="flex-1 scroll-y p-4">
      <div class="vstack gap-6">
        <!-- Loading skeleton -->
        <div v-if="isLoading" class="vstack gap-4" role="status" aria-busy="true">
          <div class="skeleton line"></div>
          <div class="skeleton box"></div>
        </div>

        <!-- Error state -->
        <ErrorState
          v-else-if="error"
          :title="$t('common.loadErrorTitle')"
          :description="error"
          :retry-label="$t('common.retry')"
          @retry="fetchTransfers"
        />

        <!-- Agent View (no tabs, just their transfers) -->
        <article v-else-if="!isAdminOrManager" class="card">
          <header>
            <h3>{{ $t('agentTransfers.myTransfers') }}</h3>
          </header>
          <div v-if="myTransfers.length === 0" class="align-center vstack gap-2 items-center">
            <UserX :size="32" />
            <p>{{ $t('agentTransfers.noActiveTransfers') }}</p>
            <small class="text-light">{{ $t('agentTransfers.clickPickNext') }}</small>
          </div>

          <div v-else class="table">
            <table>
              <thead>
                <tr>
                  <th>{{ $t('agentTransfers.contact') }}</th>
                  <th>{{ $t('agentTransfers.phone') }}</th>
                  <th>{{ $t('agentTransfers.transferredAt') }}</th>
                  <th>{{ $t('agentTransfers.source') }}</th>
                  <th class="align-right">{{ $t('agentTransfers.actions') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="transfer in myTransfers" :key="transfer.id">
                  <td><strong>{{ transfer.contact_name }}</strong></td>
                  <td>{{ transfer.phone_number }}</td>
                  <td>{{ formatDate(transfer.transferred_at) }}</td>
                  <td>
                    <span class="badge" :class="{ outline: getSourceBadge(transfer.source).variant === 'outline' }" :data-variant="badgeVariant(getSourceBadge(transfer.source).variant)">
                      {{ getSourceBadge(transfer.source).label }}
                    </span>
                  </td>
                  <td class="align-right">
                    <div class="hstack gap-1 justify-end">
                      <IconButton :icon="MessageSquare" :label="$t('agentTransfers.viewChat')" variant="outline" size="sm" @click="viewChat(transfer)" />
                      <IconButton :icon="Play" :label="$t('agentTransfers.resumeChatbot')" variant="outline" size="sm" :disabled="isResuming" @click="openResumeDialog(transfer)" />
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </article>

        <!-- Admin/Manager View (with tabs) -->
        <div v-else class="vstack gap-4">
          <div role="tablist">
            <button
              v-for="tab in tabs"
              :id="`transfers-tab-${tab.value}`"
              :key="tab.value"
              type="button"
              role="tab"
              aria-controls="transfers-tabpanel"
              :aria-selected="activeTab === tab.value"
              @click="activeTab = tab.value"
            >
              {{ tab.label }}
              <span v-if="tab.count" class="badge" :data-variant="tab.variant">{{ tab.count }}</span>
            </button>
          </div>

          <div id="transfers-tabpanel" role="tabpanel" :aria-labelledby="`transfers-tab-${activeTab}`">
            <!-- My Transfers Tab -->
            <article v-if="activeTab === 'my-transfers'" class="card">
              <header>
                <h3>{{ $t('agentTransfers.myTransfers') }}</h3>
              </header>
              <div v-if="myTransfers.length === 0" class="align-center vstack gap-2 items-center">
                <UserX :size="32" />
                <p>{{ $t('agentTransfers.noActiveTransfers') }}</p>
              </div>

              <div v-else class="table">
                <table>
                  <thead>
                    <tr>
                      <th>{{ $t('agentTransfers.contact') }}</th>
                      <th>{{ $t('agentTransfers.phone') }}</th>
                      <th>{{ $t('agentTransfers.transferredAt') }}</th>
                      <th>{{ $t('agentTransfers.source') }}</th>
                      <th>{{ $t('agentTransfers.notes') }}</th>
                      <th class="align-right">{{ $t('agentTransfers.actions') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="transfer in myTransfers" :key="transfer.id">
                      <td><strong>{{ transfer.contact_name }}</strong></td>
                      <td>{{ transfer.phone_number }}</td>
                      <td>{{ formatDate(transfer.transferred_at) }}</td>
                      <td>
                        <span class="badge" :class="{ outline: getSourceBadge(transfer.source).variant === 'outline' }" :data-variant="badgeVariant(getSourceBadge(transfer.source).variant)">
                          {{ getSourceBadge(transfer.source).label }}
                        </span>
                      </td>
                      <td><span class="truncate">{{ transfer.notes || '-' }}</span></td>
                      <td class="align-right">
                        <div class="hstack gap-1 justify-end">
                          <button type="button" class="outline small" @click="viewChat(transfer)">
                            <MessageSquare :size="16" />
                            {{ $t('agentTransfers.chat') }}
                          </button>
                          <button type="button" class="outline small" :disabled="isResuming" @click="openResumeDialog(transfer)">
                            <Play :size="16" />
                            {{ $t('agentTransfers.resume') }}
                          </button>
                        </div>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </article>

            <!-- Queue Tab -->
            <article v-else-if="activeTab === 'queue'" class="card">
              <header class="hstack justify-between flex-wrap gap-4">
                <h3>{{ $t('agentTransfers.transferQueue') }}</h3>
                <div class="hstack gap-4 flex-wrap">
                  <div class="hstack gap-2 flex-wrap">
                    <span class="badge outline">{{ $t('agentTransfers.general') }}: {{ teamQueueCounts.general || 0 }}</span>
                    <span v-for="team in teams" :key="team.id" class="badge outline">
                      {{ team.name }}: {{ teamQueueCounts[team.id] || 0 }}
                    </span>
                  </div>
                  <select v-model="selectedTeamFilter" :aria-label="$t('agentTransfers.filterByTeam')">
                    <option value="all">{{ $t('agentTransfers.allQueues') }}</option>
                    <option value="general">{{ $t('agentTransfers.generalQueue') }}</option>
                    <option v-for="team in teams" :key="team.id" :value="team.id">{{ team.name }}</option>
                  </select>
                </div>
              </header>
              <div v-if="queueTransfers.length === 0" class="align-center vstack gap-2 items-center">
                <Clock :size="32" />
                <p>{{ $t('agentTransfers.noTransfersInQueue') }}</p>
              </div>

              <div v-else class="table">
                <table>
                  <thead>
                    <tr>
                      <th>{{ $t('agentTransfers.contact') }}</th>
                      <th>{{ $t('agentTransfers.phone') }}</th>
                      <th>{{ $t('agentTransfers.team') }}</th>
                      <th>{{ $t('agentTransfers.sla') }}</th>
                      <th>{{ $t('agentTransfers.waiting') }}</th>
                      <th>{{ $t('agentTransfers.source') }}</th>
                      <th class="align-right">{{ $t('agentTransfers.actions') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="transfer in queueTransfers" :key="transfer.id">
                      <td><strong>{{ transfer.contact_name }}</strong></td>
                      <td>{{ transfer.phone_number }}</td>
                      <td>
                        <span class="badge outline">
                          <Users :size="12" />
                          {{ getTeamName(transfer.team_id) }}
                        </span>
                      </td>
                      <td>
                        <span class="badge" :class="{ outline: getSLABadge(transfer).variant === 'outline' }" :data-variant="badgeVariant(getSLABadge(transfer).variant)" :title="slaTitle(transfer)">
                          <XCircle v-if="getSLABadge(transfer).icon === 'xcircle'" :size="12" />
                          <AlertTriangle v-else-if="getSLABadge(transfer).icon === 'alert'" :size="12" />
                          <CheckCircle2 v-else :size="12" />
                          {{ getSLABadge(transfer).label }}
                        </span>
                      </td>
                      <td>
                        <strong v-if="getSLAStatus(transfer) === 'breached'">{{ formatTimeRemaining(transfer.sla_response_deadline) }}</strong>
                        <span v-else>{{ formatTimeRemaining(transfer.sla_response_deadline) }}</span>
                      </td>
                      <td>
                        <span class="badge" :class="{ outline: getSourceBadge(transfer.source).variant === 'outline' }" :data-variant="badgeVariant(getSourceBadge(transfer.source).variant)">
                          {{ getSourceBadge(transfer.source).label }}
                        </span>
                      </td>
                      <td class="align-right">
                        <div class="hstack gap-1 justify-end">
                          <button type="button" class="outline small" @click="openAssignDialog(transfer)">
                            <UserPlus :size="16" />
                            {{ $t('agentTransfers.assign') }}
                          </button>
                          <button type="button" class="outline small" @click="viewChat(transfer)">
                            <MessageSquare :size="16" />
                            {{ $t('agentTransfers.chat') }}
                          </button>
                        </div>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </article>

            <!-- All Active Tab -->
            <article v-else-if="activeTab === 'all'" class="card">
              <header>
                <h3>{{ $t('agentTransfers.allActiveTransfers') }}</h3>
              </header>
              <div v-if="allActiveTransfers.length === 0" class="align-center vstack gap-2 items-center">
                <UserX :size="32" />
                <p>{{ $t('agentTransfers.noActiveTransfersGlobal') }}</p>
              </div>

              <div v-else class="table">
                <table>
                  <thead>
                    <tr>
                      <th>{{ $t('agentTransfers.contact') }}</th>
                      <th>{{ $t('agentTransfers.phone') }}</th>
                      <th>{{ $t('agentTransfers.assignedTo') }}</th>
                      <th>{{ $t('agentTransfers.team') }}</th>
                      <th>{{ $t('agentTransfers.sla') }}</th>
                      <th>{{ $t('agentTransfers.source') }}</th>
                      <th class="align-right">{{ $t('agentTransfers.actions') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="transfer in allActiveTransfers" :key="transfer.id">
                      <td><strong>{{ transfer.contact_name }}</strong></td>
                      <td>{{ transfer.phone_number }}</td>
                      <td>
                        <span v-if="transfer.agent_name" class="badge outline">
                          <User :size="12" />
                          {{ transfer.agent_name }}
                        </span>
                        <span v-else class="badge" data-variant="danger">{{ $t('agentTransfers.unassigned') }}</span>
                      </td>
                      <td>
                        <span class="badge outline">
                          <Users :size="12" />
                          {{ getTeamName(transfer.team_id) }}
                        </span>
                      </td>
                      <td>
                        <span class="badge" :class="{ outline: getSLABadge(transfer).variant === 'outline' }" :data-variant="badgeVariant(getSLABadge(transfer).variant)" :title="slaTitle(transfer, true)">
                          <XCircle v-if="getSLABadge(transfer).icon === 'xcircle'" :size="12" />
                          <AlertTriangle v-else-if="getSLABadge(transfer).icon === 'alert'" :size="12" />
                          <CheckCircle2 v-else :size="12" />
                          {{ getSLABadge(transfer).label }}
                        </span>
                      </td>
                      <td>
                        <span class="badge" :class="{ outline: getSourceBadge(transfer.source).variant === 'outline' }" :data-variant="badgeVariant(getSourceBadge(transfer.source).variant)">
                          {{ getSourceBadge(transfer.source).label }}
                        </span>
                      </td>
                      <td class="align-right">
                        <div class="hstack gap-1 justify-end">
                          <IconButton :icon="UserPlus" :label="$t('agentTransfers.assign')" variant="outline" size="sm" @click="openAssignDialog(transfer)" />
                          <IconButton :icon="MessageSquare" :label="$t('agentTransfers.chat')" variant="outline" size="sm" @click="viewChat(transfer)" />
                          <IconButton :icon="Play" :label="$t('agentTransfers.resumeChatbot')" variant="outline" size="sm" :disabled="isResuming" @click="openResumeDialog(transfer)" />
                        </div>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </article>

            <!-- History Tab -->
            <article v-else-if="activeTab === 'history'" class="card">
              <header>
                <div class="hstack justify-between">
                  <h3>{{ $t('agentTransfers.transferHistory') }}</h3>
                  <small v-if="historyTotalCount > 0" class="text-light">
                    {{ historyTransfers.length }} of {{ historyTotalCount }}
                  </small>
                </div>
              </header>
              <!-- Loading state -->
              <div v-if="isLoadingHistory && historyTransfers.length === 0" class="align-center" role="status" aria-busy="true" data-spinner="large">
                <p class="text-light">{{ $t('agentTransfers.loadingHistory') }}...</p>
              </div>

              <div v-else-if="historyTransfers.length === 0" class="align-center vstack gap-2 items-center">
                <Clock :size="32" />
                <p>{{ $t('agentTransfers.noTransferHistory') }}</p>
              </div>

              <template v-else>
                <div class="table">
                  <table>
                    <thead>
                      <tr>
                        <th>{{ $t('agentTransfers.contact') }}</th>
                        <th>{{ $t('agentTransfers.phone') }}</th>
                        <th>{{ $t('agentTransfers.handledBy') }}</th>
                        <th>{{ $t('agentTransfers.transferredAt') }}</th>
                        <th>{{ $t('agentTransfers.resumedAt') }}</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="transfer in historyTransfers" :key="transfer.id">
                        <td><strong>{{ transfer.contact_name }}</strong></td>
                        <td>{{ transfer.phone_number }}</td>
                        <td>{{ transfer.agent_name || '-' }}</td>
                        <td>{{ formatDate(transfer.transferred_at) }}</td>
                        <td>{{ transfer.resumed_at ? formatDate(transfer.resumed_at) : '-' }}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>

                <!-- Load More button -->
                <div v-if="hasMoreHistory" class="hstack justify-center">
                  <button
                    type="button"
                    class="outline"
                    :disabled="isLoadingHistory"
                    :aria-busy="isLoadingHistory ? 'true' : undefined"
                    data-spinner="small"
                    @click="transfersStore.loadMoreHistory()"
                  >
                    {{ $t('agentTransfers.loadMore') }}
                  </button>
                </div>
              </template>
            </article>
          </div>
        </div>
      </div>
    </div>

    <!-- Assign Dialog -->
    <Modal
      v-model:open="assignDialogOpen"
      :title="$t('agentTransfers.reassignTransfer')"
      :description="$t('agentTransfers.changeAssignment')"
    >
      <div class="vstack gap-4">
        <div v-if="transferToAssign" class="vstack gap-1">
          <p><strong>{{ $t('agentTransfers.contact') }}:</strong> {{ transferToAssign.contact_name }}</p>
          <p><strong>{{ $t('agentTransfers.phone') }}:</strong> {{ transferToAssign.phone_number }}</p>
        </div>

        <label data-field>
          {{ $t('agentTransfers.teamQueue') }}
          <select v-model="selectedTeamId">
            <option value="general">{{ $t('agentTransfers.generalQueue') }}</option>
            <option v-for="team in teams" :key="team.id" :value="team.id">{{ team.name }}</option>
          </select>
          <small data-hint>{{ $t('agentTransfers.moveToTeamQueue') }}</small>
        </label>

        <label data-field>
          {{ $t('agentTransfers.assignToAgent') }}
          <select v-model="selectedAgentId">
            <option value="unassigned">{{ $t('agentTransfers.unassignedInQueue') }}</option>
            <option v-for="agent in agents" :key="agent.id" :value="agent.id">{{ agent.full_name }}</option>
          </select>
          <small data-hint>{{ $t('agentTransfers.directlyAssign') }}</small>
        </label>
      </div>
      <template #footer>
        <button type="button" class="outline" @click="assignDialogOpen = false">{{ $t('common.cancel') }}</button>
        <button type="button" :disabled="isAssigning" :aria-busy="isAssigning ? 'true' : undefined" data-spinner="small" @click="assignTransfer">
          {{ $t('agentTransfers.save') }}
        </button>
      </template>
    </Modal>

    <!-- Resume Confirmation Dialog -->
    <ConfirmDialog
      v-model:open="resumeDialogOpen"
      :title="$t('agentTransfers.confirmResumeTitle')"
      :description="$t('agentTransfers.confirmResumeDescription')"
      :confirm-label="$t('agentTransfers.confirmResumeLabel')"
      :cancel-label="$t('common.cancel')"
      :is-submitting="isResuming"
      @confirm="confirmResumeTransfer"
    />
  </div>
</template>
