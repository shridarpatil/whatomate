<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { PageHeader } from '@/components/shared'
import { chatbotService, usersService, teamsService, type Team } from '@/services/api'
import { useTransfersStore, type AgentTransfer, getSLAStatus } from '@/stores/transfers'
import { useAuthStore } from '@/stores/auth'
import { toast } from 'vue-sonner'
import { useRouter } from 'vue-router'
import { UserX, Play, MessageSquare, User, Clock, Loader2, Users, UserPlus, AlertTriangle, CheckCircle2, XCircle } from 'lucide-vue-next'
import { getErrorMessage } from '@/lib/api-utils'

const { t } = useI18n()

const router = useRouter()
const transfersStore = useTransfersStore()
const authStore = useAuthStore()

const isLoading = ref(true)
const isPicking = ref(false)
const isAssigning = ref(false)
const isResuming = ref(false)
const activeTab = ref('my-transfers')
const assignDialogOpen = ref(false)
const transferToAssign = ref<AgentTransfer | null>(null)
const selectedAgentId = ref<string>('')
const selectedTeamId = ref<string>('')
const agents = ref<{ id: string; full_name: string }[]>([])
const teams = ref<Team[]>([])
const selectedTeamFilter = ref<string>('all')

const userRole = computed(() => authStore.user?.role?.name)
const isAdminOrManager = computed(() => userRole.value === 'admin' || userRole.value === 'manager')
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
  await Promise.all([fetchTransfers(), fetchTeams()])
  // Always try to fetch agents for admin/manager - the API will reject if unauthorized
  if (isAdminOrManager.value) {
    await fetchAgents()
  }
  // No polling - WebSocket handles real-time updates
  // Reconnection refresh handles sync after disconnect
})

async function fetchTransfers() {
  isLoading.value = true
  try {
    await transfersStore.fetchTransfers()
  } finally {
    isLoading.value = false
  }
}

async function fetchAgents() {
  try {
    const response = await usersService.list()
    const data = response.data.data || response.data
    const usersList = data.users || data || []
    agents.value = usersList.filter((u: any) => u.is_active !== false).map((u: any) => ({
      id: u.id,
      full_name: u.full_name
    }))
  } catch {
    toast.error(t('agentTransfers.failedLoadAgents'))
  }
}

async function fetchTeams() {
  try {
    const response = await teamsService.list()
    const data = (response.data as any).data || response.data
    teams.value = (data.teams || []).filter((t: Team) => t.is_active)
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

async function resumeTransfer(transfer: AgentTransfer) {
  isResuming.value = true
  try {
    await chatbotService.resumeTransfer(transfer.id)
    toast.success(t('agentTransfers.transferResumed'), {
      description: t('agentTransfers.chatbotNowActive')
    })
    await fetchTransfers()
  } catch (error) {
    toast.error(getErrorMessage(error, t('agentTransfers.failedResumeTransfer')))
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
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('agentTransfers.title')" :subtitle="$t('agentTransfers.subtitle')" :icon="UserX" icon-gradient="bg-gradient-to-br from-red-500 to-orange-600 shadow-red-500/20">
      <template v-if="!isAdminOrManager" #actions>
        <div class="flex items-center gap-4">
          <div class="text-sm text-white/50 light:text-gray-500">
            <Users class="h-4 w-4 inline mr-1" />
            {{ $t('agentTransfers.waitingInQueue', { count: transfersStore.queueCount }) }}
          </div>
          <Button variant="outline" size="sm" @click="pickNextTransfer" :disabled="isPicking || transfersStore.queueCount === 0">
            <Loader2 v-if="isPicking" class="mr-2 h-4 w-4 animate-spin" />
            <Play v-else class="mr-2 h-4 w-4" />
            {{ $t('agentTransfers.pickNext') }}
          </Button>
        </div>
      </template>
    </PageHeader>

    <!-- Content -->
    <ScrollArea class="flex-1">
      <div class="p-6 space-y-6">
        <!-- Loading skeleton -->
        <div v-if="isLoading" class="space-y-4">
          <Skeleton class="h-12 w-full bg-white/[0.08] light:bg-gray-200 rounded-xl" />
          <Skeleton class="h-64 w-full bg-white/[0.08] light:bg-gray-200 rounded-xl" />
        </div>

        <!-- Agent View (no tabs, just their transfers) -->
        <div v-else-if="!isAdminOrManager">
          <div class="rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-white light:border-gray-200">
            <div class="p-6">
              <h3 class="text-lg font-semibold text-white light:text-gray-900">{{ $t('agentTransfers.myTransfers') }}</h3>
              <p class="text-sm text-white/50 light:text-gray-500">{{ $t('agentTransfers.contactsTransferred') }}</p>
            </div>
            <div class="px-6 pb-6">
              <div v-if="myTransfers.length === 0" class="text-center py-8 text-white/50 light:text-gray-500">
                <div class="h-16 w-16 rounded-xl bg-red-500/20 flex items-center justify-center mx-auto mb-4">
                  <UserX class="h-8 w-8 text-red-400" />
                </div>
                <p>{{ $t('agentTransfers.noActiveTransfers') }}</p>
                <p class="text-sm mt-2">{{ $t('agentTransfers.clickPickNext') }}</p>
              </div>

              <Table v-else>
                <TableHeader>
                  <TableRow>
                    <TableHead>{{ $t('agentTransfers.contact') }}</TableHead>
                    <TableHead>{{ $t('agentTransfers.phone') }}</TableHead>
                    <TableHead>{{ $t('agentTransfers.transferredAt') }}</TableHead>
                    <TableHead>{{ $t('agentTransfers.source') }}</TableHead>
                    <TableHead class="text-right">{{ $t('agentTransfers.actions') }}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow v-for="transfer in myTransfers" :key="transfer.id">
                    <TableCell class="font-medium">{{ transfer.contact_name }}</TableCell>
                    <TableCell>{{ transfer.phone_number }}</TableCell>
                    <TableCell>{{ formatDate(transfer.transferred_at) }}</TableCell>
                    <TableCell>
                      <Badge :variant="getSourceBadge(transfer.source).variant">
                        {{ getSourceBadge(transfer.source).label }}
                      </Badge>
                    </TableCell>
                    <TableCell class="text-right space-x-2">
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button size="sm" variant="outline" @click="viewChat(transfer)">
                            <MessageSquare class="h-4 w-4" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>{{ $t('agentTransfers.viewChat') }}</TooltipContent>
                      </Tooltip>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            size="sm"
                            variant="outline"
                            @click="resumeTransfer(transfer)"
                            :disabled="isResuming"
                          >
                            <Play class="h-4 w-4" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>{{ $t('agentTransfers.resumeChatbot') }}</TooltipContent>
                      </Tooltip>
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </div>
          </div>
        </div>

        <!-- Admin/Manager View (with tabs) -->
        <div v-else>
          <Tabs v-model="activeTab" class="w-full">
            <TabsList class="mb-6">
              <TabsTrigger value="my-transfers">
                {{ $t('agentTransfers.myTransfers') }}
                <Badge v-if="myTransfers.length > 0" class="ml-2" variant="secondary">
                  {{ myTransfers.length }}
                </Badge>
              </TabsTrigger>
              <TabsTrigger value="queue">
                {{ $t('agentTransfers.queue') }}
                <Badge v-if="queueTransfers.length > 0" class="ml-2" variant="destructive">
                  {{ queueTransfers.length }}
                </Badge>
              </TabsTrigger>
              <TabsTrigger value="all">{{ $t('agentTransfers.allActive') }}</TabsTrigger>
              <TabsTrigger value="history">{{ $t('agentTransfers.history') }}</TabsTrigger>
            </TabsList>

            <!-- My Transfers Tab -->
            <TabsContent value="my-transfers">
              <Card>
                <CardHeader>
                  <CardTitle>{{ $t('agentTransfers.myTransfers') }}</CardTitle>
                  <CardDescription>{{ $t('agentTransfers.transfersAssignedToYou') }}</CardDescription>
                </CardHeader>
                <CardContent>
                  <div v-if="myTransfers.length === 0" class="text-center py-8 text-muted-foreground">
                    <UserX class="h-12 w-12 mx-auto mb-4 opacity-50" />
                    <p>{{ $t('agentTransfers.noActiveTransfers') }}</p>
                  </div>

                  <Table v-else>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{{ $t('agentTransfers.contact') }}</TableHead>
                        <TableHead>{{ $t('agentTransfers.phone') }}</TableHead>
                        <TableHead>{{ $t('agentTransfers.transferredAt') }}</TableHead>
                        <TableHead>{{ $t('agentTransfers.source') }}</TableHead>
                        <TableHead>{{ $t('agentTransfers.notes') }}</TableHead>
                        <TableHead class="text-right">{{ $t('agentTransfers.actions') }}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      <TableRow v-for="transfer in myTransfers" :key="transfer.id">
                        <TableCell class="font-medium">{{ transfer.contact_name }}</TableCell>
                        <TableCell>{{ transfer.phone_number }}</TableCell>
                        <TableCell>{{ formatDate(transfer.transferred_at) }}</TableCell>
                        <TableCell>
                          <Badge :variant="getSourceBadge(transfer.source).variant">
                            {{ getSourceBadge(transfer.source).label }}
                          </Badge>
                        </TableCell>
                        <TableCell class="max-w-[200px] truncate">{{ transfer.notes || '-' }}</TableCell>
                        <TableCell class="text-right space-x-2">
                          <Button size="sm" variant="outline" @click="viewChat(transfer)">
                            <MessageSquare class="h-4 w-4 mr-1" />
                            {{ $t('agentTransfers.chat') }}
                          </Button>
                          <Button
                            size="sm"
                            variant="outline"
                            @click="resumeTransfer(transfer)"
                            :disabled="isResuming"
                          >
                            <Play class="h-4 w-4 mr-1" />
                            {{ $t('agentTransfers.resume') }}
                          </Button>
                        </TableCell>
                      </TableRow>
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>
            </TabsContent>

            <!-- Queue Tab -->
            <TabsContent value="queue">
              <Card>
                <CardHeader>
                  <div class="flex items-center justify-between">
                    <div>
                      <CardTitle>{{ $t('agentTransfers.transferQueue') }}</CardTitle>
                      <CardDescription>{{ $t('agentTransfers.unassignedTransfers') }}</CardDescription>
                    </div>
                    <div class="flex items-center gap-3">
                      <div class="flex items-center gap-2 text-sm text-muted-foreground">
                        <Badge variant="outline">{{ $t('agentTransfers.general') }}: {{ teamQueueCounts.general || 0 }}</Badge>
                        <Badge v-for="team in teams" :key="team.id" variant="outline">
                          {{ team.name }}: {{ teamQueueCounts[team.id] || 0 }}
                        </Badge>
                      </div>
                      <Select v-model="selectedTeamFilter">
                        <SelectTrigger class="w-[180px]">
                          <SelectValue :placeholder="$t('agentTransfers.filterByTeam')" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="all">{{ $t('agentTransfers.allQueues') }}</SelectItem>
                          <SelectItem value="general">{{ $t('agentTransfers.generalQueue') }}</SelectItem>
                          <SelectItem v-for="team in teams" :key="team.id" :value="team.id">
                            {{ team.name }}
                          </SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                </CardHeader>
                <CardContent>
                  <div v-if="queueTransfers.length === 0" class="text-center py-8 text-muted-foreground">
                    <Clock class="h-12 w-12 mx-auto mb-4 opacity-50" />
                    <p>{{ $t('agentTransfers.noTransfersInQueue') }}</p>
                  </div>

                  <Table v-else>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{{ $t('agentTransfers.contact') }}</TableHead>
                        <TableHead>{{ $t('agentTransfers.phone') }}</TableHead>
                        <TableHead>{{ $t('agentTransfers.team') }}</TableHead>
                        <TableHead>{{ $t('agentTransfers.sla') }}</TableHead>
                        <TableHead>{{ $t('agentTransfers.waiting') }}</TableHead>
                        <TableHead>{{ $t('agentTransfers.source') }}</TableHead>
                        <TableHead class="text-right">{{ $t('agentTransfers.actions') }}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      <TableRow v-for="transfer in queueTransfers" :key="transfer.id">
                        <TableCell class="font-medium">{{ transfer.contact_name }}</TableCell>
                        <TableCell>{{ transfer.phone_number }}</TableCell>
                        <TableCell>
                          <Badge variant="outline">
                            <Users class="h-3 w-3 mr-1" />
                            {{ getTeamName(transfer.team_id) }}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Badge :variant="getSLABadge(transfer).variant" class="cursor-help">
                                <XCircle v-if="getSLABadge(transfer).icon === 'xcircle'" class="h-3 w-3 mr-1" />
                                <AlertTriangle v-else-if="getSLABadge(transfer).icon === 'alert'" class="h-3 w-3 mr-1" />
                                <CheckCircle2 v-else class="h-3 w-3 mr-1" />
                                {{ getSLABadge(transfer).label }}
                              </Badge>
                            </TooltipTrigger>
                            <TooltipContent>
                              <div class="text-xs space-y-1">
                                <p v-if="transfer.sla_response_deadline">{{ $t('agentTransfers.responseDeadline') }}: {{ formatDate(transfer.sla_response_deadline) }}</p>
                                <p v-if="transfer.escalation_level > 0">{{ $t('agentTransfers.escalationLevel') }}: {{ transfer.escalation_level }}</p>
                                <p v-if="transfer.sla_breached">{{ $t('agentTransfers.breachedAt') }}: {{ formatDate(transfer.sla_breached_at!) }}</p>
                              </div>
                            </TooltipContent>
                          </Tooltip>
                        </TableCell>
                        <TableCell>
                          <span :class="{ 'text-destructive font-medium': getSLAStatus(transfer) === 'breached' }">
                            {{ formatTimeRemaining(transfer.sla_response_deadline) }}
                          </span>
                        </TableCell>
                        <TableCell>
                          <Badge :variant="getSourceBadge(transfer.source).variant">
                            {{ getSourceBadge(transfer.source).label }}
                          </Badge>
                        </TableCell>
                        <TableCell class="text-right space-x-2">
                          <Button size="sm" variant="outline" @click="openAssignDialog(transfer)">
                            <UserPlus class="h-4 w-4 mr-1" />
                            {{ $t('agentTransfers.assign') }}
                          </Button>
                          <Button size="sm" variant="outline" @click="viewChat(transfer)">
                            <MessageSquare class="h-4 w-4 mr-1" />
                            {{ $t('agentTransfers.chat') }}
                          </Button>
                        </TableCell>
                      </TableRow>
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>
            </TabsContent>

            <!-- All Active Tab -->
            <TabsContent value="all">
              <Card>
                <CardHeader>
                  <CardTitle>{{ $t('agentTransfers.allActiveTransfers') }}</CardTitle>
                  <CardDescription>{{ $t('agentTransfers.allCurrentlyActive') }}</CardDescription>
                </CardHeader>
                <CardContent>
                  <div v-if="allActiveTransfers.length === 0" class="text-center py-8 text-muted-foreground">
                    <UserX class="h-12 w-12 mx-auto mb-4 opacity-50" />
                    <p>{{ $t('agentTransfers.noActiveTransfersGlobal') }}</p>
                  </div>

                  <Table v-else>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{{ $t('agentTransfers.contact') }}</TableHead>
                        <TableHead>{{ $t('agentTransfers.phone') }}</TableHead>
                        <TableHead>{{ $t('agentTransfers.assignedTo') }}</TableHead>
                        <TableHead>{{ $t('agentTransfers.team') }}</TableHead>
                        <TableHead>{{ $t('agentTransfers.sla') }}</TableHead>
                        <TableHead>{{ $t('agentTransfers.source') }}</TableHead>
                        <TableHead class="text-right">{{ $t('agentTransfers.actions') }}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      <TableRow v-for="transfer in allActiveTransfers" :key="transfer.id">
                        <TableCell class="font-medium">{{ transfer.contact_name }}</TableCell>
                        <TableCell>{{ transfer.phone_number }}</TableCell>
                        <TableCell>
                          <Badge v-if="transfer.agent_name" variant="outline">
                            <User class="h-3 w-3 mr-1" />
                            {{ transfer.agent_name }}
                          </Badge>
                          <Badge v-else variant="destructive">{{ $t('agentTransfers.unassigned') }}</Badge>
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline">
                            <Users class="h-3 w-3 mr-1" />
                            {{ getTeamName(transfer.team_id) }}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Badge :variant="getSLABadge(transfer).variant" class="cursor-help">
                                <XCircle v-if="getSLABadge(transfer).icon === 'xcircle'" class="h-3 w-3 mr-1" />
                                <AlertTriangle v-else-if="getSLABadge(transfer).icon === 'alert'" class="h-3 w-3 mr-1" />
                                <CheckCircle2 v-else class="h-3 w-3 mr-1" />
                                {{ getSLABadge(transfer).label }}
                              </Badge>
                            </TooltipTrigger>
                            <TooltipContent>
                              <div class="text-xs space-y-1">
                                <p v-if="transfer.picked_up_at">{{ $t('agentTransfers.pickedUpAt') }}: {{ formatDate(transfer.picked_up_at) }}</p>
                                <p v-else-if="transfer.sla_response_deadline">{{ $t('agentTransfers.responseDeadline') }}: {{ formatDate(transfer.sla_response_deadline) }}</p>
                                <p v-if="transfer.escalation_level > 0">{{ $t('agentTransfers.escalationLevel') }}: {{ transfer.escalation_level }}</p>
                                <p v-if="transfer.sla_breached">{{ $t('agentTransfers.breachedAt') }}: {{ formatDate(transfer.sla_breached_at!) }}</p>
                              </div>
                            </TooltipContent>
                          </Tooltip>
                        </TableCell>
                        <TableCell>
                          <Badge :variant="getSourceBadge(transfer.source).variant">
                            {{ getSourceBadge(transfer.source).label }}
                          </Badge>
                        </TableCell>
                        <TableCell class="text-right space-x-2">
                          <Button size="sm" variant="outline" @click="openAssignDialog(transfer)">
                            <UserPlus class="h-4 w-4" />
                          </Button>
                          <Button size="sm" variant="outline" @click="viewChat(transfer)">
                            <MessageSquare class="h-4 w-4" />
                          </Button>
                          <Button
                            size="sm"
                            variant="outline"
                            @click="resumeTransfer(transfer)"
                            :disabled="isResuming"
                          >
                            <Play class="h-4 w-4" />
                          </Button>
                        </TableCell>
                      </TableRow>
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>
            </TabsContent>

            <!-- History Tab -->
            <TabsContent value="history">
              <Card>
                <CardHeader>
                  <CardTitle class="flex items-center justify-between">
                    <span>{{ $t('agentTransfers.transferHistory') }}</span>
                    <span v-if="historyTotalCount > 0" class="text-sm font-normal text-muted-foreground">
                      {{ historyTransfers.length }} of {{ historyTotalCount }}
                    </span>
                  </CardTitle>
                  <CardDescription>{{ $t('agentTransfers.resumedTransfers') }}</CardDescription>
                </CardHeader>
                <CardContent>
                  <!-- Loading state -->
                  <div v-if="isLoadingHistory && historyTransfers.length === 0" class="text-center py-8">
                    <Loader2 class="h-8 w-8 mx-auto mb-4 animate-spin text-muted-foreground" />
                    <p class="text-muted-foreground">{{ $t('agentTransfers.loadingHistory') }}...</p>
                  </div>

                  <div v-else-if="historyTransfers.length === 0" class="text-center py-8 text-muted-foreground">
                    <Clock class="h-12 w-12 mx-auto mb-4 opacity-50" />
                    <p>{{ $t('agentTransfers.noTransferHistory') }}</p>
                  </div>

                  <template v-else>
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{{ $t('agentTransfers.contact') }}</TableHead>
                          <TableHead>{{ $t('agentTransfers.phone') }}</TableHead>
                          <TableHead>{{ $t('agentTransfers.handledBy') }}</TableHead>
                          <TableHead>{{ $t('agentTransfers.transferredAt') }}</TableHead>
                          <TableHead>{{ $t('agentTransfers.resumedAt') }}</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        <TableRow v-for="transfer in historyTransfers" :key="transfer.id">
                          <TableCell class="font-medium">{{ transfer.contact_name }}</TableCell>
                          <TableCell>{{ transfer.phone_number }}</TableCell>
                          <TableCell>{{ transfer.agent_name || '-' }}</TableCell>
                          <TableCell>{{ formatDate(transfer.transferred_at) }}</TableCell>
                          <TableCell>{{ transfer.resumed_at ? formatDate(transfer.resumed_at) : '-' }}</TableCell>
                        </TableRow>
                      </TableBody>
                    </Table>

                    <!-- Load More button -->
                    <div v-if="hasMoreHistory" class="flex justify-center mt-4">
                      <Button
                        variant="outline"
                        @click="transfersStore.loadMoreHistory()"
                        :disabled="isLoadingHistory"
                      >
                        <Loader2 v-if="isLoadingHistory" class="h-4 w-4 mr-2 animate-spin" />
                        {{ $t('agentTransfers.loadMore') }}
                      </Button>
                    </div>
                  </template>
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        </div>
      </div>
    </ScrollArea>

    <!-- Assign Dialog -->
    <Dialog v-model:open="assignDialogOpen">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{{ $t('agentTransfers.reassignTransfer') }}</DialogTitle>
          <DialogDescription>
            {{ $t('agentTransfers.changeAssignment') }}
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-4">
          <div v-if="transferToAssign" class="text-sm border rounded-lg p-3 bg-muted/50">
            <p><strong>{{ $t('agentTransfers.contact') }}:</strong> {{ transferToAssign.contact_name }}</p>
            <p><strong>{{ $t('agentTransfers.phone') }}:</strong> {{ transferToAssign.phone_number }}</p>
          </div>

          <div class="space-y-2">
            <label class="text-sm font-medium">{{ $t('agentTransfers.teamQueue') }}</label>
            <Select v-model="selectedTeamId">
              <SelectTrigger>
                <SelectValue :placeholder="$t('agentTransfers.selectTeam')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="general">{{ $t('agentTransfers.generalQueue') }}</SelectItem>
                <SelectItem v-for="team in teams" :key="team.id" :value="team.id">
                  {{ team.name }}
                </SelectItem>
              </SelectContent>
            </Select>
            <p class="text-xs text-muted-foreground">{{ $t('agentTransfers.moveToTeamQueue') }}</p>
          </div>

          <div class="space-y-2">
            <label class="text-sm font-medium">{{ $t('agentTransfers.assignToAgent') }}</label>
            <Select v-model="selectedAgentId">
              <SelectTrigger>
                <SelectValue :placeholder="$t('agentTransfers.selectAgent')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="unassigned">{{ $t('agentTransfers.unassignedInQueue') }}</SelectItem>
                <SelectItem v-for="agent in agents" :key="agent.id" :value="agent.id">
                  {{ agent.full_name }}
                </SelectItem>
              </SelectContent>
            </Select>
            <p class="text-xs text-muted-foreground">{{ $t('agentTransfers.directlyAssign') }}</p>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" @click="assignDialogOpen = false">{{ $t('common.cancel') }}</Button>
          <Button size="sm" @click="assignTransfer" :disabled="isAssigning">
            <Loader2 v-if="isAssigning" class="mr-2 h-4 w-4 animate-spin" />
            {{ $t('agentTransfers.save') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
