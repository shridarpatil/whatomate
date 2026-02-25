<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCallingStore } from '@/stores/calling'
import { callTransfersService, type CallTransfer } from '@/services/api'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { Phone, PhoneOff, PhoneForwarded, RefreshCw, Clock } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import DataTable, { type Column } from '@/components/shared/DataTable.vue'

const { t } = useI18n()
const store = useCallingStore()

const activeTab = ref('waiting')
const historyTransfers = ref<CallTransfer[]>([])
const historyTotal = ref(0)
const historyPage = ref(1)
const historyLoading = ref(false)
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
  try {
    const response = await callTransfersService.list({ page: historyPage.value, limit: pageSize })
    const data = response.data as any
    historyTransfers.value = (data.data?.call_transfers ?? data.call_transfers ?? [])
      .filter((t: CallTransfer) => t.status !== 'waiting')
    historyTotal.value = data.data?.total ?? data.total ?? 0
  } catch {
    // Silently handle
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

function statusVariant(status: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (status) {
    case 'waiting': return 'default'
    case 'connected': return 'default'
    case 'completed': return 'secondary'
    case 'abandoned': return 'destructive'
    case 'no_answer': return 'outline'
    default: return 'secondary'
  }
}

onMounted(() => {
  store.fetchWaitingTransfers()
  fetchHistory()
})
</script>

<template>
  <div class="p-6 space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold">{{ t('callTransfers.title') }}</h1>
      </div>
      <Button variant="outline" size="sm" @click="store.fetchWaitingTransfers(); fetchHistory()">
        <RefreshCw class="h-4 w-4 mr-2" />
        {{ t('common.refresh') }}
      </Button>
    </div>

    <Tabs v-model="activeTab">
      <TabsList>
        <TabsTrigger value="waiting">
          {{ t('callTransfers.waiting') }}
          <Badge v-if="store.waitingTransfers.length > 0" variant="destructive" class="ml-2 h-5 min-w-[20px]">
            {{ store.waitingTransfers.length }}
          </Badge>
        </TabsTrigger>
        <TabsTrigger value="history">{{ t('callTransfers.history') }}</TabsTrigger>
      </TabsList>

      <TabsContent value="waiting" class="mt-4">
        <Card>
          <CardContent class="pt-6">
            <DataTable
              :items="store.waitingTransfers"
              :columns="waitingColumns"
              :is-loading="false"
              :empty-icon="PhoneForwarded"
              :empty-title="t('callTransfers.noWaiting')"
            >
              <template #cell-caller_phone="{ item: transfer }">
                <div class="flex items-center gap-2">
                  <Phone class="h-4 w-4 text-green-400" />
                  <span>{{ transfer.contact?.profile_name || transfer.caller_phone }}</span>
                </div>
              </template>
              <template #cell-status>
                <Badge variant="default" class="bg-yellow-600/20 text-yellow-400 border-yellow-600/30">
                  {{ t('callTransfers.waiting') }}
                </Badge>
              </template>
              <template #cell-transferred_at="{ item: transfer }">
                {{ formatDate(transfer.transferred_at) }}
              </template>
              <template #cell-actions="{ item: transfer }">
                <Button
                  size="sm"
                  class="bg-green-600 hover:bg-green-500 text-white"
                  @click="handleAccept(transfer.id)"
                >
                  <Phone class="h-3.5 w-3.5 mr-1" />
                  {{ t('callTransfers.accept') }}
                </Button>
              </template>
            </DataTable>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="history" class="mt-4">
        <Card>
          <CardContent class="pt-6">
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
                <div class="flex items-center gap-2">
                  <component :is="transfer.status === 'completed' ? Phone : PhoneOff"
                    class="h-4 w-4"
                    :class="transfer.status === 'completed' ? 'text-green-400' : 'text-red-400'"
                  />
                  <span>{{ transfer.contact?.profile_name || transfer.caller_phone }}</span>
                </div>
              </template>
              <template #cell-status="{ item: transfer }">
                <Badge :variant="statusVariant(transfer.status)">
                  {{ transfer.status }}
                </Badge>
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
          </CardContent>
        </Card>
      </TabsContent>
    </Tabs>
  </div>
</template>
