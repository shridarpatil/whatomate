<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { chatbotService, usersService, teamsService, type Team } from '@/services/api'
import { useTransfersStore, type AgentTransfer, getSLAStatus } from '@/stores/transfers'
import { useAuthStore } from '@/stores/auth'
import { toast } from 'vue-sonner'
import { useRouter } from 'vue-router'
import {
  UserX,
  Play,
  MessageSquare,
  User,
  Clock,
  Loader2,
  Users,
  UserPlus,
  AlertTriangle,
  CheckCircle2,
  XCircle
} from 'lucide-vue-next'

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
    toast.error('Failed to load agents list')
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
  if (!teamId) return 'General Queue'
  const team = teams.value.find(t => t.id === teamId)
  return team?.name || 'Unknown Team'
}

async function pickNextTransfer() {
  isPicking.value = true
  try {
    const response = await chatbotService.pickNextTransfer()
    const data = response.data.data || response.data

    if (data.transfer) {
      toast.success('Transfer picked', {
        description: `You are now assigned to ${data.transfer.contact_name || data.transfer.phone_number}`
      })
      await fetchTransfers()

      // Navigate to chat
      router.push(`/chat/${data.transfer.contact_id}`)
    } else {
      toast.info('No transfers in queue')
    }
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to pick transfer')
  } finally {
    isPicking.value = false
  }
}

async function resumeTransfer(transfer: AgentTransfer) {
  isResuming.value = true
  try {
    await chatbotService.resumeTransfer(transfer.id)
    toast.success('Transfer resumed', {
      description: 'Chatbot is now active for this contact'
    })
    await fetchTransfers()
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to resume transfer')
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
    toast.success('Transfer updated')
    assignDialogOpen.value = false
    await fetchTransfers()
  } catch (error: any) {
    toast.error(error.response?.data?.message || 'Failed to assign transfer')
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
      return { label: 'Flow', variant: 'secondary' as const }
    case 'keyword':
      return { label: 'Keyword', variant: 'outline' as const }
    default:
      return { label: 'Manual', variant: 'default' as const }
  }
}

function getSLABadge(transfer: AgentTransfer) {
  const status = getSLAStatus(transfer)
  switch (status) {
    case 'breached':
      return { label: 'SLA Breached', variant: 'destructive' as const, icon: 'xcircle' }
    case 'warning':
      return { label: 'At Risk', variant: 'warning' as const, icon: 'alert' }
    case 'expired':
      return { label: 'Expired', variant: 'secondary' as const, icon: 'xcircle' }
    default:
      return { label: 'On Track', variant: 'outline' as const, icon: 'check' }
  }
}

function formatTimeRemaining(deadline: string | undefined): string {
  if (!deadline) return '-'
  const now = new Date()
  const deadlineDate = new Date(deadline)
  const diff = deadlineDate.getTime() - now.getTime()

  if (diff <= 0) return 'Overdue'

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
    <!-- Header -->
    <header class="border-b border-white/[0.08] light:border-gray-200 bg-[#0a0a0b]/95 light:bg-white/95 backdrop-blur">
      <div class="flex h-16 items-center px-6">
        <div class="h-8 w-8 rounded-lg bg-gradient-to-br from-red-500 to-orange-600 flex items-center justify-center mr-3 shadow-lg shadow-red-500/20">
          <UserX class="h-4 w-4 text-white" />
        </div>
        <div class="flex-1">
          <h1 class="text-xl font-semibold text-white light:text-gray-900">Transfers</h1>
          <p class="text-sm text-white/50 light:text-gray-500">Manage agent transfers and queue</p>
        </div>

        <!-- Queue pickup for agents -->
        <div v-if="!isAdminOrManager" class="flex items-center gap-4">
          <div class="text-sm text-white/50 light:text-gray-500">
            <Users class="h-4 w-4 inline mr-1" />
            {{ transfersStore.queueCount }} waiting in queue
          </div>
          <Button variant="outline" size="sm" @click="pickNextTransfer" :disabled="isPicking || transfersStore.queueCount === 0">
            <Loader2 v-if="isPicking" class="mr-2 h-4 w-4 animate-spin" />
            <Play v-else class="mr-2 h-4 w-4" />
            Pick Next
          </Button>
        </div>
      </div>
    </header>

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
              <h3 class="text-lg font-semibold text-white light:text-gray-900">My Transfers</h3>
              <p class="text-sm text-white/50 light:text-gray-500">Contacts transferred to you for human support</p>
            </div>
            <div class="px-6 pb-6">
              <div v-if="myTransfers.length === 0" class="text-center py-8 text-white/50 light:text-gray-500">
                <div class="h-16 w-16 rounded-xl bg-red-500/20 flex items-center justify-center mx-auto mb-4">
                  <UserX class="h-8 w-8 text-red-400" />
                </div>
                <p>No active transfers assigned to you</p>
                <p class="text-sm mt-2">Click "Pick Next" to get a transfer from the queue</p>
              </div>

              <Table v-else>
                <TableHeader>
                  <TableRow>
                    <TableHead>Contact</TableHead>
                    <TableHead>Phone</TableHead>
                    <TableHead>Transferred At</TableHead>
                    <TableHead>Source</TableHead>
                    <TableHead class="text-right">Actions</TableHead>
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
                        <TooltipContent>View Chat</TooltipContent>
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
                        <TooltipContent>Resume Chatbot</TooltipContent>
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
                My Transfers
                <Badge v-if="myTransfers.length > 0" class="ml-2" variant="secondary">
                  {{ myTransfers.length }}
                </Badge>
              </TabsTrigger>
              <TabsTrigger value="queue">
                Queue
                <Badge v-if="queueTransfers.length > 0" class="ml-2" variant="destructive">
                  {{ queueTransfers.length }}
                </Badge>
              </TabsTrigger>
              <TabsTrigger value="all">All Active</TabsTrigger>
              <TabsTrigger value="history">History</TabsTrigger>
            </TabsList>

            <!-- My Transfers Tab -->
            <TabsContent value="my-transfers">
              <Card>
                <CardHeader>
                  <CardTitle>My Transfers</CardTitle>
                  <CardDescription>Transfers assigned to you</CardDescription>
                </CardHeader>
                <CardContent>
                  <div v-if="myTransfers.length === 0" class="text-center py-8 text-muted-foreground">
                    <UserX class="h-12 w-12 mx-auto mb-4 opacity-50" />
                    <p>No active transfers assigned to you</p>
                  </div>

                  <Table v-else>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Contact</TableHead>
                        <TableHead>Phone</TableHead>
                        <TableHead>Transferred At</TableHead>
                        <TableHead>Source</TableHead>
                        <TableHead>Notes</TableHead>
                        <TableHead class="text-right">Actions</TableHead>
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
                            Chat
                          </Button>
                          <Button
                            size="sm"
                            variant="outline"
                            @click="resumeTransfer(transfer)"
                            :disabled="isResuming"
                          >
                            <Play class="h-4 w-4 mr-1" />
                            Resume
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
                      <CardTitle>Transfer Queue</CardTitle>
                      <CardDescription>Unassigned transfers waiting for pickup (FIFO)</CardDescription>
                    </div>
                    <div class="flex items-center gap-3">
                      <div class="flex items-center gap-2 text-sm text-muted-foreground">
                        <Badge variant="outline">General: {{ teamQueueCounts.general || 0 }}</Badge>
                        <Badge v-for="team in teams" :key="team.id" variant="outline">
                          {{ team.name }}: {{ teamQueueCounts[team.id] || 0 }}
                        </Badge>
                      </div>
                      <Select v-model="selectedTeamFilter">
                        <SelectTrigger class="w-[180px]">
                          <SelectValue placeholder="Filter by team" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="all">All Queues</SelectItem>
                          <SelectItem value="general">General Queue</SelectItem>
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
                    <p>No transfers in queue</p>
                  </div>

                  <Table v-else>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Contact</TableHead>
                        <TableHead>Phone</TableHead>
                        <TableHead>Team</TableHead>
                        <TableHead>SLA</TableHead>
                        <TableHead>Waiting</TableHead>
                        <TableHead>Source</TableHead>
                        <TableHead class="text-right">Actions</TableHead>
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
                                <p v-if="transfer.sla_response_deadline">Response deadline: {{ formatDate(transfer.sla_response_deadline) }}</p>
                                <p v-if="transfer.escalation_level > 0">Escalation level: {{ transfer.escalation_level }}</p>
                                <p v-if="transfer.sla_breached">Breached at: {{ formatDate(transfer.sla_breached_at!) }}</p>
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
                            Assign
                          </Button>
                          <Button size="sm" variant="outline" @click="viewChat(transfer)">
                            <MessageSquare class="h-4 w-4 mr-1" />
                            Chat
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
                  <CardTitle>All Active Transfers</CardTitle>
                  <CardDescription>All currently active transfers</CardDescription>
                </CardHeader>
                <CardContent>
                  <div v-if="allActiveTransfers.length === 0" class="text-center py-8 text-muted-foreground">
                    <UserX class="h-12 w-12 mx-auto mb-4 opacity-50" />
                    <p>No active transfers</p>
                  </div>

                  <Table v-else>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Contact</TableHead>
                        <TableHead>Phone</TableHead>
                        <TableHead>Assigned To</TableHead>
                        <TableHead>Team</TableHead>
                        <TableHead>SLA</TableHead>
                        <TableHead>Source</TableHead>
                        <TableHead class="text-right">Actions</TableHead>
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
                          <Badge v-else variant="destructive">Unassigned</Badge>
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
                                <p v-if="transfer.picked_up_at">Picked up: {{ formatDate(transfer.picked_up_at) }}</p>
                                <p v-else-if="transfer.sla_response_deadline">Response deadline: {{ formatDate(transfer.sla_response_deadline) }}</p>
                                <p v-if="transfer.escalation_level > 0">Escalation level: {{ transfer.escalation_level }}</p>
                                <p v-if="transfer.sla_breached">Breached at: {{ formatDate(transfer.sla_breached_at!) }}</p>
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
                    <span>Transfer History</span>
                    <span v-if="historyTotalCount > 0" class="text-sm font-normal text-muted-foreground">
                      {{ historyTransfers.length }} of {{ historyTotalCount }}
                    </span>
                  </CardTitle>
                  <CardDescription>Resumed transfers</CardDescription>
                </CardHeader>
                <CardContent>
                  <!-- Loading state -->
                  <div v-if="isLoadingHistory && historyTransfers.length === 0" class="text-center py-8">
                    <Loader2 class="h-8 w-8 mx-auto mb-4 animate-spin text-muted-foreground" />
                    <p class="text-muted-foreground">Loading history...</p>
                  </div>

                  <div v-else-if="historyTransfers.length === 0" class="text-center py-8 text-muted-foreground">
                    <Clock class="h-12 w-12 mx-auto mb-4 opacity-50" />
                    <p>No transfer history</p>
                  </div>

                  <template v-else>
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Contact</TableHead>
                          <TableHead>Phone</TableHead>
                          <TableHead>Handled By</TableHead>
                          <TableHead>Transferred At</TableHead>
                          <TableHead>Resumed At</TableHead>
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
                        Load More
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
          <DialogTitle>Reassign Transfer</DialogTitle>
          <DialogDescription>
            Change agent or team assignment
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-4">
          <div v-if="transferToAssign" class="text-sm border rounded-lg p-3 bg-muted/50">
            <p><strong>Contact:</strong> {{ transferToAssign.contact_name }}</p>
            <p><strong>Phone:</strong> {{ transferToAssign.phone_number }}</p>
          </div>

          <div class="space-y-2">
            <label class="text-sm font-medium">Team Queue</label>
            <Select v-model="selectedTeamId">
              <SelectTrigger>
                <SelectValue placeholder="Select a team" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="general">General Queue</SelectItem>
                <SelectItem v-for="team in teams" :key="team.id" :value="team.id">
                  {{ team.name }}
                </SelectItem>
              </SelectContent>
            </Select>
            <p class="text-xs text-muted-foreground">Move transfer to a different team's queue</p>
          </div>

          <div class="space-y-2">
            <label class="text-sm font-medium">Assign to Agent</label>
            <Select v-model="selectedAgentId">
              <SelectTrigger>
                <SelectValue placeholder="Select an agent" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="unassigned">Unassigned (in queue)</SelectItem>
                <SelectItem v-for="agent in agents" :key="agent.id" :value="agent.id">
                  {{ agent.full_name }}
                </SelectItem>
              </SelectContent>
            </Select>
            <p class="text-xs text-muted-foreground">Directly assign to an agent or leave in queue</p>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" @click="assignDialogOpen = false">Cancel</Button>
          <Button size="sm" @click="assignTransfer" :disabled="isAssigning">
            <Loader2 v-if="isAssigning" class="mr-2 h-4 w-4 animate-spin" />
            Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
