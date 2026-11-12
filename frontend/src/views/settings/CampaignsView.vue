<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { campaignsService } from '@/services/api'
import { wsService } from '@/services/websocket'
import { toast } from '@/lib/toast'
import { PageHeader, DataTable, DeleteConfirmDialog, SearchInput, IconButton, ErrorState, DateRangePicker, type Column } from '@/components/shared'
import { getErrorMessage } from '@/lib/api-utils'
import {
  Plus,
  Pencil,
  Trash2,
  Megaphone,
  Play,
  Pause,
  Users,
  CheckCircle,
  Clock,
  AlertCircle,
} from '@/lib/icons'
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
  { key: 'template', label: t('campaigns.template', 'Template') },
  { key: 'status', label: t('campaigns.status'), sortable: true },
  { key: 'stats', label: t('campaigns.progress') },
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

function getStatusIcon(status: string) {
  switch (status) {
    case 'completed':
      return CheckCircle
    case 'running':
    case 'processing':
    case 'queued':
      return Play
    case 'paused':
      return Pause
    case 'scheduled':
      return Clock
    case 'failed':
    case 'cancelled':
      return AlertCircle
    default:
      return Megaphone
  }
}

function getStatusVariant(status: string): string | undefined {
  switch (status) {
    case 'completed':
      return 'success'
    case 'running':
    case 'processing':
    case 'queued':
      return 'secondary'
    case 'failed':
    case 'cancelled':
      return 'danger'
    default:
      return undefined
  }
}

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
        <RouterLink to="/campaigns/new" class="button outline small">
          <Plus :size="16" />
          {{ $t('campaigns.createCampaign') }}
        </RouterLink>
      </template>
    </PageHeader>

    <!-- Campaigns List -->
    <div class="flex-1 scroll-y p-4">
      <article class="card">
        <header class="hstack justify-between items-center flex-wrap gap-4">
          <div>
            <h3>{{ $t('campaigns.yourCampaigns') }}</h3>
            <p class="text-light">{{ $t('campaigns.yourCampaignsDesc') }}</p>
          </div>
          <div class="hstack items-center gap-2 flex-wrap">
            <select v-model="filterStatus" :aria-label="$t('campaigns.status')">
              <option v-for="opt in statusOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
            </select>
            <DateRangePicker
              v-model:selected-range="selectedRange"
              v-model:custom-date-range="customDateRange"
              v-model:is-date-picker-open="isDatePickerOpen"
              :format-date-range-display="formatDateRangeDisplay"
              @apply-custom="applyCustomRange"
            />
            <SearchInput v-model="searchQuery" :placeholder="$t('campaigns.searchCampaigns') + '...'" />
          </div>
        </header>
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
          </template>
          <template #cell-template="{ item: campaign }">
            <span class="text-light">{{ campaign.template_name || '—' }}</span>
          </template>
          <template #cell-status="{ item: campaign }">
            <span class="badge" :data-variant="getStatusVariant(campaign.status)">
              <component :is="getStatusIcon(campaign.status)" :size="12" />
              {{ campaign.status }}
            </span>
          </template>
          <template #cell-stats="{ item: campaign }">
            <div class="vstack gap-1">
              <div v-if="campaign.status === 'running' || campaign.status === 'processing'" class="vstack">
                <progress :value="getProgressPercentage(campaign)" max="100" :aria-label="$t('campaigns.progress')"></progress>
                <small class="text-light">{{ getProgressPercentage(campaign) }}%</small>
              </div>
              <div class="hstack items-center gap-2">
                <small :title="$t('campaigns.recipients')"><Users :size="12" /> {{ campaign.total_recipients }}</small>
                <small :title="$t('campaigns.delivered')">{{ campaign.delivered_count }}</small>
                <small :title="$t('campaigns.read')">{{ campaign.read_count }}</small>
                <small v-if="campaign.failed_count > 0" :title="$t('campaigns.failed')">{{ campaign.failed_count }}</small>
              </div>
            </div>
          </template>
          <template #cell-created_at="{ item: campaign }">
            <span class="text-light">{{ formatDate(campaign.created_at) }}</span>
          </template>
          <template #cell-actions="{ item: campaign }">
            <div class="hstack items-center justify-end gap-1">
              <RouterLink :to="`/campaigns/${campaign.id}`" class="button ghost icon" :title="$t('campaigns.editCampaign')" :aria-label="$t('campaigns.editCampaign')"><Pencil :size="16" /></RouterLink>
              <IconButton
                :icon="Trash2"
                :label="$t('campaigns.deleteCampaign')"
                :disabled="campaign.status === 'running' || campaign.status === 'processing'"
                @click="openDeleteDialog(campaign)"
              />
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
