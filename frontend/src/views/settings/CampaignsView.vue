<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { campaignsService } from '@/services/api'
import { wsService } from '@/services/websocket'
import { toast } from '@/lib/toast'
import { PageHeader, DataTable, DeleteConfirmDialog, SearchInput, RowMenu, ErrorState, DateRangePicker, type Column } from '@/components/shared'
import { campaignTone } from '@/lib/status'
import { getErrorMessage } from '@/lib/api-utils'
import { Plus, Trash2, Megaphone } from '@/lib/icons'
import { formatDate } from '@/lib/utils'
import { useSearchPagination } from '@/composables/useSearchPagination'
import { useDateRange } from '@/composables/useDateRange'

const { t } = useI18n()

interface Campaign {
  id: string
  name: string
  template_name: string
  template_id?: string
  whatsapp_account?: string
  header_media_id?: string
  header_media_filename?: string
  header_media_mime_type?: string
  status: 'draft' | 'scheduled' | 'running' | 'paused' | 'completed' | 'failed' | 'queued' | 'processing' | 'cancelled'
  total_recipients: number
  sent_count: number
  delivered_count: number
  read_count: number
  failed_count: number
  scheduled_at?: string
  started_at?: string
  completed_at?: string
  created_at: string
}

const campaigns = ref<Campaign[]>([])
const isLoading = ref(true)

const columns = computed<Column<Campaign>[]>(() => [
  { key: 'name', label: t('campaigns.campaign'), sortable: true },
  { key: 'status', label: t('campaigns.status'), sortable: true },
  { key: 'stats', label: t('campaigns.delivery') },
  { key: 'created_at', label: t('campaigns.created'), sortable: true },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

const sortKey = ref('created_at')
const sortDirection = ref<'asc' | 'desc'>('desc')
const { searchQuery, currentPage, totalItems, pageSize, handlePageChange, resetAndFetch } = useSearchPagination({
  fetchFn: () => fetchCampaigns(),
})

// Filter state
const filterStatus = ref<string>('all')
const {
  selectedRange,
  customDateRange,
  isDatePickerOpen,
  dateRange,
  formatDateRangeDisplay,
  applyCustomRange: applyCustomRangeBase,
} = useDateRange()

const statusOptions = computed(() => [
  { value: 'all', label: t('campaigns.allStatuses') },
  { value: 'draft', label: t('campaigns.draft') },
  { value: 'queued', label: t('campaigns.queued') },
  { value: 'processing', label: t('campaigns.processing') },
  { value: 'completed', label: t('campaigns.completed') },
  { value: 'failed', label: t('campaigns.failed') },
  { value: 'cancelled', label: t('campaigns.cancelled') },
  { value: 'paused', label: t('campaigns.paused') },
])

// AlertDialog state
const deleteDialogOpen = ref(false)
const campaignToDelete = ref<Campaign | null>(null)
const isDeletingCampaign = ref(false)

// Error state
const error = ref<string | null>(null)

// WebSocket subscription for real-time stats updates
let unsubscribeCampaignStats: (() => void) | null = null

onMounted(async () => {
  await fetchCampaigns()

  // Subscribe to campaign stats updates
  unsubscribeCampaignStats = wsService.onCampaignStatsUpdate((payload) => {
    const campaign = campaigns.value.find(c => c.id === payload.campaign_id)
    if (campaign) {
      campaign.sent_count = payload.sent_count
      campaign.delivered_count = payload.delivered_count
      campaign.read_count = payload.read_count
      campaign.failed_count = payload.failed_count
      if (payload.status) {
        campaign.status = payload.status
      }
    }
  })
})

onUnmounted(() => {
  if (unsubscribeCampaignStats) {
    unsubscribeCampaignStats()
  }
})

async function fetchCampaigns() {
  isLoading.value = true
  error.value = null
  try {
    const { from, to } = dateRange.value
    const params: Record<string, string | number> = {
      from,
      to,
      page: currentPage.value,
      limit: pageSize
    }
    if (filterStatus.value && filterStatus.value !== 'all') {
      params.status = filterStatus.value
    }
    if (searchQuery.value) {
      params.search = searchQuery.value
    }
    const response = await campaignsService.list(params)
    // API returns: { status: "success", data: { campaigns: [...], total: N } }
    const data = response.data.data || response.data
    campaigns.value = data.campaigns || []
    totalItems.value = data.total ?? campaigns.value.length
  } catch (err: any) {
    console.error('Failed to fetch campaigns:', err)
    error.value = getErrorMessage(err, t('campaigns.fetchFailed'))
    campaigns.value = []
    totalItems.value = 0
  } finally {
    isLoading.value = false
  }
}

function applyCustomRange() {
  applyCustomRangeBase()
  fetchCampaigns()
}

// Watch for filter changes
watch([filterStatus, selectedRange], () => {
  if (selectedRange.value !== 'custom') {
    resetAndFetch()
  }
})

function openDeleteDialog(campaign: Campaign) {
  campaignToDelete.value = campaign
  deleteDialogOpen.value = true
}

async function confirmDeleteCampaign() {
  if (!campaignToDelete.value) return

  isDeletingCampaign.value = true
  try {
    await campaignsService.delete(campaignToDelete.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.Campaign') }))
    deleteDialogOpen.value = false
    campaignToDelete.value = null
    await fetchCampaigns()
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('common.failedDelete', { resource: t('resources.campaign') })))
  } finally {
    isDeletingCampaign.value = false
  }
}

const isSending = (c: Campaign) => ['running', 'processing', 'queued'].includes(c.status)
const isEditable = (c: Campaign) => ['draft', 'scheduled'].includes(c.status)

function statusLabel(c: Campaign): string {
  if (isSending(c)) return t('campaigns.sendingPercent', { percent: getProgressPercentage(c) })
  if (c.status === 'completed' && c.failed_count > 0) return t('campaigns.completedWithErrors')
  return t(`campaigns.${c.status}`, c.status)
}

/** Widths (%) of the read / delivered / sent / failed segments, each counted once. */
function deliverySegments(c: Campaign) {
  const total = Math.max(c.total_recipients, 1)
  const read = c.read_count
  const delivered = Math.max(c.delivered_count - c.read_count, 0)
  const sent = Math.max(c.sent_count - c.delivered_count, 0)
  return [
    { k: 'read', pct: (read / total) * 100 },
    { k: 'delivered', pct: (delivered / total) * 100 },
    { k: 'sent', pct: (sent / total) * 100 },
    { k: 'failed', pct: (c.failed_count / total) * 100 },
  ].filter(s => s.pct > 0)
}

const hasStarted = (c: Campaign) => c.sent_count > 0 || c.failed_count > 0

function getProgressPercentage(campaign: Campaign): number {
  if (campaign.total_recipients === 0) return 0
  return Math.round((campaign.sent_count / campaign.total_recipients) * 100)
}

</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader
      :title="$t('campaigns.title')"
      :description="$t('campaigns.subtitle')"
      :icon="Megaphone"
    >
      <template #actions>
        <RouterLink to="/campaigns/new" class="button small">
          <Plus :size="16" />
          {{ $t('campaigns.createCampaign') }}
        </RouterLink>
      </template>
    </PageHeader>

    <!-- Campaigns List -->
    <div class="flex-1 scroll-y p-4">
      <article class="card vstack gap-4">
        <!-- Filters on the left, search on the right. -->
        <div data-toolbar>
          <div>
            <select class="w-auto" v-model="filterStatus" :aria-label="$t('campaigns.status')">
              <option v-for="opt in statusOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
            </select>
            <DateRangePicker
              v-model:selected-range="selectedRange"
              v-model:custom-date-range="customDateRange"
              v-model:is-date-picker-open="isDatePickerOpen"
              :format-date-range-display="formatDateRangeDisplay"
              @apply-custom="applyCustomRange"
            />
          </div>
          <SearchInput v-model="searchQuery" :placeholder="$t('campaigns.searchCampaigns')" />
        </div>
        <ErrorState
          v-if="error && !isLoading"
          :title="$t('campaigns.fetchFailedTitle')"
          :description="error"
          :retry-label="$t('common.retry')"
          @retry="fetchCampaigns"
        />
        <DataTable
          v-else
          :items="campaigns"
          :columns="columns"
          :is-loading="isLoading"
          :empty-icon="Megaphone"
          :empty-title="searchQuery ? $t('campaigns.noMatchingCampaigns') : $t('campaigns.noCampaignsYet')"
          :empty-description="searchQuery ? $t('campaigns.noMatchingCampaignsDesc') : $t('campaigns.noCampaignsYetDesc')"
          v-model:sort-key="sortKey"
          v-model:sort-direction="sortDirection"
          server-pagination
          :current-page="currentPage"
          :total-items="totalItems"
          :page-size="pageSize"
          item-name="campaigns"
          @page-change="handlePageChange"
        >
          <template #cell-name="{ item: campaign }">
            <RouterLink :to="`/campaigns/${campaign.id}`"><strong>{{ campaign.name }}</strong></RouterLink>
            <small class="text-light block">
              {{ campaign.template_name || '—' }} · {{ $t('campaigns.recipientsCount', { count: campaign.total_recipients.toLocaleString() }) }}
            </small>
          </template>
          <template #cell-status="{ item: campaign }">
            <span class="badge" :data-status="campaignTone(campaign.status, campaign.failed_count)">{{ statusLabel(campaign) }}</span>
            <small v-if="campaign.status === 'scheduled' && campaign.scheduled_at" class="text-light block">
              {{ $t('campaigns.startsAt', { date: formatDate(campaign.scheduled_at) }) }}
            </small>
          </template>
          <template #cell-stats="{ item: campaign }">
            <small v-if="!hasStarted(campaign)" class="text-light">{{ $t('campaigns.notStarted') }}</small>
            <div v-else>
              <div
                data-delivery
                role="img"
                :aria-label="$t('campaigns.deliverySummary', { read: campaign.read_count, delivered: campaign.delivered_count, sent: campaign.sent_count, failed: campaign.failed_count, total: campaign.total_recipients })"
              >
                <i v-for="seg in deliverySegments(campaign)" :key="seg.k" :data-k="seg.k" :style="{ width: `${seg.pct}%` }"></i>
              </div>
              <div data-delivery-legend aria-hidden="true">
                <span><i data-key="read"></i>{{ $t('campaigns.read') }} {{ campaign.read_count.toLocaleString() }}</span>
                <span><i data-key="delivered"></i>{{ $t('campaigns.delivered') }} {{ campaign.delivered_count.toLocaleString() }}</span>
                <span><i data-key="sent"></i>{{ $t('campaigns.sent') }} {{ campaign.sent_count.toLocaleString() }}</span>
                <span v-if="campaign.failed_count > 0"><i data-key="failed"></i>{{ $t('campaigns.failed') }} {{ campaign.failed_count.toLocaleString() }}</span>
              </div>
            </div>
          </template>
          <template #cell-created_at="{ item: campaign }">
            <span class="text-light">{{ formatDate(campaign.created_at) }}</span>
          </template>
          <template #cell-actions="{ item: campaign }">
            <div class="hstack items-center justify-end gap-1 flex-nowrap">
              <RouterLink :to="`/campaigns/${campaign.id}`" class="button outline small">
                {{ isEditable(campaign) ? $t('common.edit') : $t('campaigns.viewReport') }}
              </RouterLink>
              <RowMenu>
                <button
                  type="button"
                  role="menuitem"
                  class="ghost"
                  data-variant="danger"
                  :disabled="isSending(campaign)"
                  :title="isSending(campaign) ? $t('campaigns.deleteWhileSending') : undefined"
                  @click="openDeleteDialog(campaign)"
                >
                  <Trash2 :size="14" />
                  {{ isSending(campaign) ? $t('campaigns.deletePauseFirst') : $t('common.delete') }}
                </button>
              </RowMenu>
            </div>
          </template>
          <template #empty-action>
            <RouterLink v-if="!searchQuery" to="/campaigns/new" class="button outline small">
              <Plus :size="16" />
              {{ $t('campaigns.createCampaign') }}
            </RouterLink>
          </template>
        </DataTable>
      </article>
    </div>

    <DeleteConfirmDialog
      v-model:open="deleteDialogOpen"
      :title="$t('campaigns.deleteCampaign')"
      :item-name="campaignToDelete?.name"
      :is-submitting="isDeletingCampaign"
      @confirm="confirmDeleteCampaign"
    />
  </div>
</template>
