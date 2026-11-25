<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { toast } from '@/lib/toast'
import { useCallingStore } from '@/stores/calling'
import { accountsService, callLogsService, ivrFlowsService, type CallLog, type CallTransfer, type IVRFlow } from '@/services/api'
import Modal from '@/components/oat/Modal.vue'
import PageHeader from '@/components/shared/PageHeader.vue'
import { Phone, PhoneIncoming, PhoneOutgoing, RefreshCw, Mic, Headphones, ArrowRightLeft, Link } from '@/lib/icons'
import { humanize, type StatusTone } from '@/lib/status'
import DataTable from '@/components/shared/DataTable.vue'
import type { Column } from '@/components/shared/types'
import SearchInput from '@/components/shared/SearchInput.vue'
import { ErrorState } from '@/components/shared'
import IVRPathTree from '@/components/calling/IVRPathTree.vue'

const { t } = useI18n()
const store = useCallingStore()
const route = useRoute()
const router = useRouter()

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
const error = ref<string | null>(null)

// Detail dialog
const showDetail = ref(false)
const selectedLog = ref<CallLog | null>(null)
const selectedTransfers = ref<CallTransfer[]>([])
const recordingURL = ref<string | null>(null)
const recordingLoading = ref(false)

const statusOptions = [
  { value: 'all', label: t('calling.allStatuses') },
  { value: 'completed', label: t('calling.completed') },
  { value: 'missed', label: t('calling.missed') },
  { value: 'ringing', label: t('calling.ringing') },
  { value: 'answered', label: t('calling.answered') },
  { value: 'rejected', label: t('calling.rejected') },
  { value: 'failed', label: t('calling.failed') },
  { value: 'transferring', label: t('calling.transferring') },
  { value: 'initiating', label: t('calling.initiating') }
]

const columns = computed<Column<CallLog>[]>(() => [
  { key: 'caller', label: t('calling.caller') },
  { key: 'direction', label: t('calling.direction') },
  { key: 'status', label: t('calling.status') },
  { key: 'duration', label: t('calling.duration') },
  { key: 'agent', label: t('calling.agent') },
  { key: 'disconnected_by', label: t('calling.disconnectedBy') },
  { key: 'ivr_flow', label: t('calling.ivrFlow') },
  { key: 'whatsapp_account', label: t('calling.account') },
  { key: 'started_at', label: t('calling.time') },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

async function fetchLogs() {
  error.value = null
  try {
    await store.fetchCallLogs({
      status: statusFilter.value !== 'all' ? statusFilter.value : undefined,
      account: accountFilter.value !== 'all' ? accountFilter.value : undefined,
      direction: directionFilter.value !== 'all' ? directionFilter.value : undefined,
      ivr_flow_id: ivrFlowFilter.value !== 'all' ? ivrFlowFilter.value : undefined,
      phone: phoneSearch.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
  } catch {
    error.value = t('calling.errorLoadingCallLogs')
  }
}

function handlePageChange(page: number) {
  currentPage.value = page
  fetchLogs()
}

// The open call log is kept in the URL (?id=) so the detail can be shared as a link.
async function openDetail(id: string) {
  selectedLog.value = store.callLogs.find(l => l.id === id) ?? null
  selectedTransfers.value = []
  showDetail.value = true
  recordingURL.value = null

  // Fetch full call log detail (includes transfers)
  try {
    const res = await callLogsService.get(id)
    const data = (res.data as any).data ?? res.data
    if (data.call_log) {
      selectedLog.value = data.call_log
    }
    if (data.transfers) {
      selectedTransfers.value = data.transfers
    }
  } catch {
    if (!selectedLog.value) {
      showDetail.value = false
      toast.error(t('calling.callLogNotFound'))
      return
    }
  }

  // Fetch recording URL if recording exists
  if (selectedLog.value?.recording_s3_key) {
    recordingLoading.value = true
    callLogsService.getRecordingURL(id)
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

async function copyDetailLink() {
  try {
    await navigator.clipboard.writeText(window.location.href)
    toast.success(t('common.copiedToClipboard'))
  } catch {
    toast.error(t('common.clipboardFailed'))
  }
}

watch(() => route.query.id, (id) => {
  if (typeof id === 'string' && id) {
    openDetail(id)
  } else {
    showDetail.value = false
  }
}, { immediate: true })

watch(showDetail, (open) => {
  if (!open && route.query.id) {
    router.replace({ query: { ...route.query, id: undefined } })
  }
})

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

const CALL_TONE: Record<string, StatusTone> = {
  completed: 'success',
  answered: 'success',
  accepted: 'success',
  ringing: 'info',
  initiating: 'info',
  transferring: 'info',
  missed: 'warning',
  rejected: 'danger',
  failed: 'danger',
}
const TRANSFER_TONE: Record<string, StatusTone> = {
  waiting: 'warning',
  connected: 'info',
  completed: 'success',
  no_answer: 'warning',
  abandoned: 'danger',
}

const statusTone = (status: string): StatusTone => CALL_TONE[status] ?? 'neutral'
const statusLabel = (status: string) => t(`calling.${status}`, humanize(status))
const transferTone = (status: string): StatusTone => TRANSFER_TONE[status] ?? 'neutral'
const directionLabel = (direction: string) => t(`calling.${direction}`, humanize(direction))
const disconnectedByLabel = (value: string) => t(`calling.disconnectedBy${value.charAt(0).toUpperCase() + value.slice(1)}`, humanize(value))

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
  <div class="flex flex-col h-full">
    <PageHeader :title="t('calling.callLogs')" :description="t('calling.callLogsDesc')">
      <template #actions>
        <button type="button" class="outline small" @click="fetchLogs">
          <RefreshCw :size="16" />
          {{ t('common.refresh') }}
        </button>
      </template>
    </PageHeader>

    <div class="flex-1 scroll-y p-4 vstack gap-4">
      <!-- Error State -->
      <ErrorState
        v-if="error && !store.callLogsLoading"
        :title="$t('common.loadErrorTitle')"
        :description="error"
        :retry-label="$t('common.retry')"
        @retry="fetchLogs"
      />

      <article v-if="!error" class="card vstack gap-4">
        <div data-toolbar>
          <div>
            <select class="w-auto" v-model="statusFilter" :aria-label="t('calling.filterByStatus')">
              <option v-for="opt in statusOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
            </select>
            <select class="w-auto" v-model="directionFilter" :aria-label="t('calling.filterByDirection')">
              <option value="all">{{ t('calling.allDirections') }}</option>
              <option value="incoming">{{ t('calling.incoming') }}</option>
              <option value="outgoing">{{ t('calling.outgoing') }}</option>
            </select>
            <select class="w-auto" v-model="ivrFlowFilter" :aria-label="t('calling.filterByIVRFlow')">
              <option value="all">{{ t('calling.allIVRFlows') }}</option>
              <option v-for="flow in ivrFlows" :key="flow.id" :value="flow.id">{{ flow.name }}</option>
            </select>
            <select class="w-auto" v-model="accountFilter" :aria-label="t('calling.filterByAccount')">
              <option value="all">{{ t('calling.allAccounts') }}</option>
              <option v-for="acc in accounts" :key="acc.name" :value="acc.name">{{ acc.name }}</option>
            </select>
          </div>
          <SearchInput v-model="phoneSearch" :placeholder="t('calling.searchByPhone')" />
        </div>
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
            <RouterLink :to="{ query: { ...route.query, id: log.id } }"><strong>{{ log.contact?.profile_name || log.caller_phone }}</strong></RouterLink>
            <small v-if="log.contact?.profile_name" class="text-light block">{{ log.caller_phone }}</small>
          </template>
          <template #cell-direction="{ item: log }">
            <span class="badge" data-label>{{ directionLabel(log.direction) }}</span>
          </template>
          <template #cell-status="{ item: log }">
            <span class="badge" :data-status="statusTone(log.status)">{{ statusLabel(log.status) }}</span>
          </template>
          <template #cell-duration="{ item: log }">
            <span class="hstack gap-2 flex-nowrap">
              {{ formatDuration(log.duration) }}
              <Mic v-if="log.recording_s3_key" :size="14" :title="t('calling.recording')" />
            </span>
          </template>
          <template #cell-agent="{ item: log }">
            <span v-if="log.agent">{{ log.agent.full_name }}</span>
            <span v-else class="text-light">-</span>
          </template>
          <template #cell-disconnected_by="{ item: log }">
            <span v-if="log.disconnected_by">{{ disconnectedByLabel(log.disconnected_by) }}</span>
            <span v-else class="text-light">-</span>
          </template>
          <template #cell-ivr_flow="{ item: log }">
            {{ log.ivr_flow?.name || '-' }}
          </template>
          <template #cell-whatsapp_account="{ item: log }">
            {{ log.whatsapp_account }}
          </template>
          <template #cell-started_at="{ item: log }">
            <span class="text-light">{{ formatDate(log.started_at || log.created_at) }}</span>
          </template>
          <template #cell-actions="{ item: log }">
            <div class="hstack items-center justify-end gap-1 flex-nowrap">
              <RouterLink :to="{ query: { ...route.query, id: log.id } }" class="button outline small">{{ t('common.view') }}</RouterLink>
            </div>
          </template>
        </DataTable>
      </article>
    </div>

    <!-- Detail Dialog -->
    <Modal v-model:open="showDetail" :title="t('calling.callDetail')" size="large">
      <div v-if="selectedLog" class="vstack gap-4">
        <div class="hstack items-center justify-between gap-2">
          <span class="text-light">{{ selectedLog.contact?.profile_name || selectedLog.caller_phone }}</span>
          <button type="button" class="outline small" @click="copyDetailLink">
            <Link :size="14" />
            {{ t('calling.copyLink') }}
          </button>
        </div>

        <div class="row">
          <div class="col-6">
            <small class="text-light">{{ t('calling.caller') }}</small>
            <p><strong>{{ selectedLog.caller_phone }}</strong></p>
          </div>
          <div class="col-6">
            <small class="text-light">{{ t('calling.direction') }}</small>
            <p class="hstack gap-2">
              <PhoneIncoming v-if="selectedLog.direction === 'incoming'" :size="14" />
              <PhoneOutgoing v-else :size="14" />
              <strong>{{ directionLabel(selectedLog.direction) }}</strong>
            </p>
          </div>
          <div class="col-6">
            <small class="text-light">{{ t('calling.status') }}</small>
            <p>
              <span class="badge" :data-status="statusTone(selectedLog.status)">{{ statusLabel(selectedLog.status) }}</span>
            </p>
          </div>
          <div class="col-6">
            <small class="text-light">{{ t('calling.duration') }}</small>
            <p><strong>{{ formatDuration(selectedLog.duration) }}</strong></p>
          </div>
          <div class="col-6">
            <small class="text-light">{{ t('calling.account') }}</small>
            <p><strong>{{ selectedLog.whatsapp_account }}</strong></p>
          </div>
          <div class="col-6">
            <small class="text-light">{{ t('calling.startedAt') }}</small>
            <p><strong>{{ formatDate(selectedLog.started_at) }}</strong></p>
          </div>
          <div class="col-6">
            <small class="text-light">{{ t('calling.endedAt') }}</small>
            <p><strong>{{ formatDate(selectedLog.ended_at) }}</strong></p>
          </div>
          <div v-if="selectedLog.disconnected_by" class="col-6">
            <small class="text-light">{{ t('calling.disconnectedBy') }}</small>
            <p>
              <strong>{{ disconnectedByLabel(selectedLog.disconnected_by) }}</strong>
            </p>
          </div>
          <div v-if="selectedLog.agent" class="col-6">
            <small class="text-light">{{ t('calling.pickedBy') }}</small>
            <p class="hstack gap-2">
              <Headphones :size="14" />
              <strong>{{ selectedLog.agent.full_name }}</strong>
            </p>
          </div>
        </div>

        <div v-if="selectedTransfers.length > 0" class="vstack gap-2">
          <small class="text-light hstack gap-2">
            <ArrowRightLeft :size="14" />
            {{ t('calling.transferHistory') }}
          </small>
          <article v-for="transfer in selectedTransfers" :key="transfer.id" class="card">
            <div class="hstack items-center justify-between">
              <span class="badge" :data-status="transferTone(transfer.status)">{{ t(`callTransfers.${transfer.status}`, humanize(transfer.status)) }}</span>
              <small class="text-light">{{ formatDate(transfer.transferred_at) }}</small>
            </div>
            <p v-if="transfer.team" class="text-light">
              {{ t('calling.team') }}: <strong>{{ transfer.team.name }}</strong>
            </p>
            <p v-if="transfer.initiating_agent" class="text-light">
              {{ t('calling.transferredBy') }}: <strong>{{ transfer.initiating_agent.full_name }}</strong>
            </p>
            <p v-if="transfer.agent" class="text-light">
              {{ t('calling.pickedBy') }}: <strong>{{ transfer.agent.full_name }}</strong>
            </p>
          </article>
        </div>

        <div v-if="selectedLog.ivr_flow">
          <small class="text-light">{{ t('calling.ivrFlow') }}</small>
          <p><strong>{{ selectedLog.ivr_flow.name }}</strong></p>
        </div>

        <div v-if="selectedLog.ivr_path?.steps?.length">
          <small class="text-light">{{ t('calling.ivrPath') }}</small>
          <IVRPathTree :steps="selectedLog.ivr_path.steps" />
        </div>

        <div v-if="selectedLog.recording_s3_key" class="vstack gap-2">
          <small class="text-light">{{ t('calling.recording') }}</small>
          <div v-if="recordingLoading" class="hstack gap-2 text-light" role="status" aria-busy="true" data-spinner="small">
            {{ t('common.loading') }}
          </div>
          <!-- Only mounted while the dialog is open so closing it stops playback. -->
          <audio
            v-else-if="showDetail && recordingURL"
            :src="recordingURL"
            controls
            preload="none"
            class="w-100"
          />
          <small v-if="selectedLog.recording_duration" class="text-light">
            {{ formatDuration(selectedLog.recording_duration) }}
          </small>
        </div>

        <div v-if="selectedLog.error_message">
          <small class="text-light">{{ t('calling.error') }}</small>
          <div role="alert" data-variant="danger">{{ selectedLog.error_message }}</div>
        </div>
      </div>
    </Modal>
  </div>
</template>
