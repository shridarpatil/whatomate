<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCallingStore } from '@/stores/calling'
import { accountsService, callLogsService, ivrFlowsService, type CallLog, type IVRFlow } from '@/services/api'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { Phone, PhoneIncoming, PhoneOutgoing, PhoneOff, PhoneMissed, Clock, RefreshCw, Mic } from 'lucide-vue-next'
import DataTable, { type Column } from '@/components/shared/DataTable.vue'
import SearchInput from '@/components/shared/SearchInput.vue'
import IVRPathTree from '@/components/calling/IVRPathTree.vue'

const { t } = useI18n()
const store = useCallingStore()

// Filters
const phoneSearch = ref('')
const statusFilter = ref('all')
const accountFilter = ref('all')
const directionFilter = ref('all')
const ivrFlowFilter = ref('all')
const currentPage = ref(1)
let searchTimeout: number | null = null
const pageSize = 20
const accounts = ref<{ name: string }[]>([])
const ivrFlows = ref<IVRFlow[]>([])

// Detail dialog
const showDetail = ref(false)
const selectedLog = ref<CallLog | null>(null)
const recordingURL = ref<string | null>(null)
const recordingLoading = ref(false)

const statusOptions = [
  { value: 'all', label: t('calling.allStatuses') },
  { value: 'completed', label: t('calling.completed') },
  { value: 'missed', label: t('calling.missed') },
  { value: 'ringing', label: t('calling.ringing') },
  { value: 'answered', label: t('calling.answered') },
  { value: 'rejected', label: t('calling.rejected') },
  { value: 'failed', label: t('calling.failed') }
]

const columns = computed<Column<CallLog>[]>(() => [
  { key: 'caller', label: t('calling.caller') },
  { key: 'direction', label: t('calling.direction') },
  { key: 'status', label: t('calling.status') },
  { key: 'duration', label: t('calling.duration') },
  { key: 'ivr_flow', label: t('calling.ivrFlow') },
  { key: 'whatsapp_account', label: t('calling.account') },
  { key: 'started_at', label: t('calling.time') },
])

function fetchLogs() {
  store.fetchCallLogs({
    status: statusFilter.value !== 'all' ? statusFilter.value : undefined,
    account: accountFilter.value !== 'all' ? accountFilter.value : undefined,
    direction: directionFilter.value !== 'all' ? directionFilter.value : undefined,
    ivr_flow_id: ivrFlowFilter.value !== 'all' ? ivrFlowFilter.value : undefined,
    phone: phoneSearch.value || undefined,
    page: currentPage.value,
    limit: pageSize
  })
}

function handlePageChange(page: number) {
  currentPage.value = page
  fetchLogs()
}

function viewDetail(log: CallLog) {
  selectedLog.value = log
  showDetail.value = true
  recordingURL.value = null

  // Fetch recording URL if recording exists
  if (log.recording_s3_key) {
    recordingLoading.value = true
    callLogsService.getRecordingURL(log.id)
      .then(res => {
        const data = (res.data as any).data ?? res.data
        recordingURL.value = data.url
      })
      .catch(() => {
        recordingURL.value = null
      })
      .finally(() => {
        recordingLoading.value = false
      })
  }
}

function formatDuration(seconds: number): string {
  if (!seconds) return '-'
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return m > 0 ? `${m}m ${s}s` : `${s}s`
}

function formatDate(dateStr?: string): string {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString()
}

function statusVariant(status: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (status) {
    case 'completed': return 'default'
    case 'answered': return 'default'
    case 'ringing': return 'secondary'
    case 'missed': return 'outline'
    case 'rejected': return 'destructive'
    case 'failed': return 'destructive'
    default: return 'secondary'
  }
}

function statusIcon(status: string) {
  switch (status) {
    case 'completed':
    case 'answered':
      return Phone
    case 'missed':
      return PhoneMissed
    case 'ringing':
      return Clock
    default:
      return PhoneOff
  }
}

onMounted(async () => {
  fetchLogs()
  try {
    const res = await accountsService.list()
    const data = res.data as any
    accounts.value = data.data?.accounts ?? data.accounts ?? []
  } catch {
    // Ignore
  }
  try {
    const res = await ivrFlowsService.list()
    const data = res.data as any
    ivrFlows.value = data.data?.ivr_flows ?? data.ivr_flows ?? []
  } catch {
    // Ignore
  }
})

watch([statusFilter, accountFilter, directionFilter, ivrFlowFilter], () => {
  currentPage.value = 1
  fetchLogs()
})

watch(phoneSearch, () => {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = window.setTimeout(() => {
    currentPage.value = 1
    fetchLogs()
  }, 300)
})
</script>

<template>
  <div class="p-6 space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold">{{ t('calling.callLogs') }}</h1>
        <p class="text-muted-foreground">{{ t('calling.callLogsDesc') }}</p>
      </div>
      <Button variant="outline" size="sm" @click="fetchLogs">
        <RefreshCw class="h-4 w-4 mr-2" />
        {{ t('common.refresh') }}
      </Button>
    </div>

    <!-- Filters -->
    <Card>
      <CardContent class="pt-6">
        <div class="flex gap-4 flex-wrap items-center">
          <SearchInput v-model="phoneSearch" :placeholder="t('calling.searchByPhone')" class="w-48" />
          <Select v-model="statusFilter">
            <SelectTrigger class="w-48">
              <SelectValue :placeholder="t('calling.filterByStatus')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="opt in statusOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </SelectItem>
            </SelectContent>
          </Select>

          <Select v-model="directionFilter">
            <SelectTrigger class="w-48">
              <SelectValue :placeholder="t('calling.filterByDirection')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{{ t('calling.allDirections') }}</SelectItem>
              <SelectItem value="incoming">{{ t('calling.incoming') }}</SelectItem>
              <SelectItem value="outgoing">{{ t('calling.outgoing') }}</SelectItem>
            </SelectContent>
          </Select>

          <Select v-model="ivrFlowFilter">
            <SelectTrigger class="w-48">
              <SelectValue :placeholder="t('calling.filterByIVRFlow')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{{ t('calling.allIVRFlows') }}</SelectItem>
              <SelectItem v-for="flow in ivrFlows" :key="flow.id" :value="flow.id">
                {{ flow.name }}
              </SelectItem>
            </SelectContent>
          </Select>

          <Select v-model="accountFilter">
            <SelectTrigger class="w-48">
              <SelectValue :placeholder="t('calling.filterByAccount')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{{ t('calling.allAccounts') }}</SelectItem>
              <SelectItem v-for="acc in accounts" :key="acc.name" :value="acc.name">
                {{ acc.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
      </CardContent>
    </Card>

    <!-- Table -->
    <Card>
      <CardContent class="pt-6">
        <DataTable
          :items="store.callLogs"
          :columns="columns"
          :is-loading="store.callLogsLoading"
          :empty-icon="Phone"
          :empty-title="t('calling.noCallLogs')"
          server-pagination
          :current-page="currentPage"
          :total-items="store.callLogsTotal"
          :page-size="pageSize"
          item-name="call logs"
          max-height="calc(100vh - 320px)"
          @page-change="handlePageChange"
        >
          <template #cell-caller="{ item: log }">
            <div class="cursor-pointer" @click="viewDetail(log)">
              <p class="font-medium">{{ log.contact?.profile_name || log.caller_phone }}</p>
              <p v-if="log.contact?.profile_name" class="text-sm text-muted-foreground">{{ log.caller_phone }}</p>
            </div>
          </template>
          <template #cell-direction="{ item: log }">
            <span class="inline-flex items-center gap-1.5 text-muted-foreground">
              <PhoneIncoming v-if="log.direction === 'incoming'" class="h-3.5 w-3.5" />
              <PhoneOutgoing v-else class="h-3.5 w-3.5" />
              {{ t(`calling.${log.direction}`) }}
            </span>
          </template>
          <template #cell-status="{ item: log }">
            <Badge :variant="statusVariant(log.status)">
              <component :is="statusIcon(log.status)" class="h-3 w-3 mr-1" />
              {{ t(`calling.${log.status}`) }}
            </Badge>
          </template>
          <template #cell-duration="{ item: log }">
            <span class="inline-flex items-center gap-1.5">
              {{ formatDuration(log.duration) }}
              <Mic v-if="log.recording_s3_key" class="h-3.5 w-3.5 text-muted-foreground" :title="t('calling.recording')" />
            </span>
          </template>
          <template #cell-ivr_flow="{ item: log }">
            {{ log.ivr_flow?.name || '-' }}
          </template>
          <template #cell-whatsapp_account="{ item: log }">
            {{ log.whatsapp_account }}
          </template>
          <template #cell-started_at="{ item: log }">
            {{ formatDate(log.started_at || log.created_at) }}
          </template>
        </DataTable>
      </CardContent>
    </Card>

    <!-- Detail Dialog -->
    <Dialog v-model:open="showDetail">
      <DialogContent class="max-w-lg">
        <DialogHeader>
          <DialogTitle>{{ t('calling.callDetail') }}</DialogTitle>
          <DialogDescription>
            {{ selectedLog?.contact?.profile_name || selectedLog?.caller_phone }}
          </DialogDescription>
        </DialogHeader>
        <div v-if="selectedLog" class="space-y-4">
          <div class="grid grid-cols-2 gap-4 text-sm">
            <div>
              <p class="text-muted-foreground">{{ t('calling.caller') }}</p>
              <p class="font-medium">{{ selectedLog.caller_phone }}</p>
            </div>
            <div>
              <p class="text-muted-foreground">{{ t('calling.direction') }}</p>
              <p class="font-medium inline-flex items-center gap-1.5">
                <PhoneIncoming v-if="selectedLog.direction === 'incoming'" class="h-3.5 w-3.5" />
                <PhoneOutgoing v-else class="h-3.5 w-3.5" />
                {{ t(`calling.${selectedLog.direction}`) }}
              </p>
            </div>
            <div>
              <p class="text-muted-foreground">{{ t('calling.status') }}</p>
              <Badge :variant="statusVariant(selectedLog.status)">
                {{ t(`calling.${selectedLog.status}`) }}
              </Badge>
            </div>
            <div>
              <p class="text-muted-foreground">{{ t('calling.duration') }}</p>
              <p class="font-medium">{{ formatDuration(selectedLog.duration) }}</p>
            </div>
            <div>
              <p class="text-muted-foreground">{{ t('calling.account') }}</p>
              <p class="font-medium">{{ selectedLog.whatsapp_account }}</p>
            </div>
            <div>
              <p class="text-muted-foreground">{{ t('calling.startedAt') }}</p>
              <p class="font-medium">{{ formatDate(selectedLog.started_at) }}</p>
            </div>
            <div>
              <p class="text-muted-foreground">{{ t('calling.endedAt') }}</p>
              <p class="font-medium">{{ formatDate(selectedLog.ended_at) }}</p>
            </div>
          </div>

          <div v-if="selectedLog.ivr_flow">
            <p class="text-sm text-muted-foreground mb-1">{{ t('calling.ivrFlow') }}</p>
            <p class="font-medium">{{ selectedLog.ivr_flow.name }}</p>
          </div>

          <div v-if="selectedLog.ivr_path?.steps?.length">
            <p class="text-sm text-muted-foreground mb-3">{{ t('calling.ivrPath') }}</p>
            <IVRPathTree :steps="selectedLog.ivr_path.steps" />
          </div>

          <div v-if="selectedLog.recording_s3_key" class="space-y-2">
            <p class="text-sm text-muted-foreground">{{ t('calling.recording') }}</p>
            <div v-if="recordingLoading" class="flex items-center gap-2 text-sm text-muted-foreground">
              <div class="animate-spin rounded-full h-4 w-4 border-b-2 border-primary" />
              {{ t('common.loading') }}
            </div>
            <audio
              v-else-if="recordingURL"
              :src="recordingURL"
              controls
              preload="none"
              class="w-full"
            />
            <p v-if="selectedLog.recording_duration" class="text-xs text-muted-foreground">
              {{ formatDuration(selectedLog.recording_duration) }}
            </p>
          </div>

          <div v-if="selectedLog.error_message">
            <p class="text-sm text-muted-foreground mb-1">{{ t('calling.error') }}</p>
            <p class="text-sm text-destructive">{{ selectedLog.error_message }}</p>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>
