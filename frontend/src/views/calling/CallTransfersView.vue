<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCallingStore } from '@/stores/calling'
import { callTransfersService, type CallTransfer } from '@/services/api'
import PageHeader from '@/components/shared/PageHeader.vue'
import { Phone, PhoneOff, PhoneForwarded, RefreshCw, Clock } from '@/lib/icons'
import { toast } from '@/lib/toast'
import DataTable from '@/components/shared/DataTable.vue'
import type { Column } from '@/components/shared/types'
import { ErrorState } from '@/components/shared'

const { t } = useI18n()
const store = useCallingStore()

const activeTab = ref('waiting')
const historyTransfers = ref<CallTransfer[]>([])
const historyTotal = ref(0)
const historyPage = ref(1)
const historyLoading = ref(false)
const historyError = ref<string | null>(null)
const pageSize = 20

const waitingColumns = computed<Column<CallTransfer>[]>(() => [
  { key: 'caller_phone', label: t('callTransfers.callerPhone') },
  { key: 'status', label: t('common.status') },
  { key: 'transferred_at', label: t('callTransfers.transferredAt') },
  { key: 'actions', label: t('common.actions') },
])

const historyColumns = computed<Column<CallTransfer>[]>(() => [
  { key: 'caller_phone', label: t('callTransfers.callerPhone') },
  { key: 'status', label: t('common.status') },
  { key: 'hold_duration', label: t('callTransfers.holdDuration') },
  { key: 'talk_duration', label: t('callTransfers.talkDuration') },
  { key: 'transferred_at', label: t('callTransfers.transferredAt') },
])

async function fetchHistory() {
  historyLoading.value = true
  historyError.value = null
  try {
    const response = await callTransfersService.list({ page: historyPage.value, limit: pageSize })
    const data = response.data as any
    historyTransfers.value = (data.data?.call_transfers ?? data.call_transfers ?? [])
      .filter((ct: CallTransfer) => ct.status !== 'waiting')
    historyTotal.value = data.data?.total ?? data.total ?? 0
  } catch {
    historyError.value = t('callTransfers.errorLoadingTransfers')
  } finally {
    historyLoading.value = false
  }
}

function handleHistoryPageChange(page: number) {
  historyPage.value = page
  fetchHistory()
}

async function handleAccept(id: string) {
  try {
    await store.acceptTransfer(id)
    toast.success(t('callTransfers.callConnected'))
  } catch (err: any) {
    toast.error(t('callTransfers.acceptFailed'), {
      description: err.message || ''
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

function statusVariant(status: string): 'success' | 'warning' | 'danger' | 'secondary' {
  switch (status) {
    case 'waiting': return 'warning'
    case 'connected': return 'success'
    case 'abandoned': return 'danger'
    default: return 'secondary'
  }
}

onMounted(() => {
  store.fetchWaitingTransfers()
  fetchHistory()
})
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader :title="t('callTransfers.title')">
      <template #actions>
        <button type="button" class="outline small" @click="store.fetchWaitingTransfers(); fetchHistory()">
          <RefreshCw :size="16" />
          {{ t('common.refresh') }}
        </button>
      </template>
    </PageHeader>

    <div class="flex-1 scroll-y p-4 vstack gap-4">
      <div role="tablist">
        <button
          id="transfers-tab-waiting"
          type="button"
          role="tab"
          aria-controls="transfers-tabpanel"
          :aria-selected="activeTab === 'waiting'"
          @click="activeTab = 'waiting'"
        >
          {{ t('callTransfers.waiting') }}
          <span v-if="store.waitingTransfers.length > 0" class="badge" data-variant="danger">
            {{ store.waitingTransfers.length }}
          </span>
        </button>
        <button
          id="transfers-tab-history"
          type="button"
          role="tab"
          aria-controls="transfers-tabpanel"
          :aria-selected="activeTab === 'history'"
          @click="activeTab = 'history'"
        >
          {{ t('callTransfers.history') }}
        </button>
      </div>

      <div id="transfers-tabpanel" role="tabpanel" :aria-labelledby="`transfers-tab-${activeTab}`">
        <article v-if="activeTab === 'waiting'" class="card">
          <DataTable
            :items="store.waitingTransfers"
            :columns="waitingColumns"
            :is-loading="false"
            :empty-icon="PhoneForwarded"
            :empty-title="t('callTransfers.noWaiting')"
          >
            <template #cell-caller_phone="{ item: transfer }">
              <div class="hstack gap-2">
                <Phone :size="16" />
                <span>{{ transfer.contact?.profile_name || transfer.caller_phone }}</span>
              </div>
            </template>
            <template #cell-status>
              <span class="badge" data-variant="warning">{{ t('callTransfers.waiting') }}</span>
            </template>
            <template #cell-transferred_at="{ item: transfer }">
              {{ formatDate(transfer.transferred_at) }}
            </template>
            <template #cell-actions="{ item: transfer }">
              <button type="button" class="small" @click="handleAccept(transfer.id)">
                <Phone :size="14" />
                {{ t('callTransfers.accept') }}
              </button>
            </template>
          </DataTable>
        </article>

        <template v-else>
          <ErrorState
            v-if="historyError && !historyLoading"
            :title="$t('common.loadErrorTitle')"
            :description="historyError"
            :retry-label="$t('common.retry')"
            @retry="fetchHistory"
          />
          <article v-else class="card">
            <DataTable
              :items="historyTransfers"
              :columns="historyColumns"
              :is-loading="historyLoading"
              :empty-icon="Clock"
              :empty-title="t('common.noResults')"
              server-pagination
              :current-page="historyPage"
              :total-items="historyTotal"
              :page-size="pageSize"
              item-name="transfers"
              max-height="calc(100vh - 320px)"
              @page-change="handleHistoryPageChange"
            >
              <template #cell-caller_phone="{ item: transfer }">
                <div class="hstack gap-2">
                  <component :is="transfer.status === 'completed' ? Phone : PhoneOff" :size="16" />
                  <span>{{ transfer.contact?.profile_name || transfer.caller_phone }}</span>
                </div>
              </template>
              <template #cell-status="{ item: transfer }">
                <span class="badge" :data-variant="statusVariant(transfer.status)">
                  {{ transfer.status }}
                </span>
              </template>
              <template #cell-hold_duration="{ item: transfer }">
                {{ formatDuration(transfer.hold_duration) }}
              </template>
              <template #cell-talk_duration="{ item: transfer }">
                {{ formatDuration(transfer.talk_duration) }}
              </template>
              <template #cell-transferred_at="{ item: transfer }">
                {{ formatDate(transfer.transferred_at) }}
              </template>
            </DataTable>
          </article>
        </template>
      </div>
    </div>
  </div>
</template>
