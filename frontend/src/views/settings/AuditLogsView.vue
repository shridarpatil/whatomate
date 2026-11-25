<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { PageHeader, DataTable, DateRangePicker, type Column } from '@/components/shared'
import { auditLogsService, type AuditLogEntry } from '@/services/api'
import { useUsersStore } from '@/stores/users'
import { useDateRange } from '@/composables/useDateRange'
import { ScrollText } from '@/lib/icons'
import { humanize } from '@/lib/status'
import { formatDate, formatLabel } from '@/lib/utils'

const { t } = useI18n()
const usersStore = useUsersStore()

const logs = ref<AuditLogEntry[]>([])
const isLoading = ref(true)

// Pagination
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 25

// Sort
const sortKey = ref('created_at')
const sortDirection = ref<'asc' | 'desc'>('desc')

// Filters
const filterUser = ref('all')
const filterAction = ref('all')
const filterResourceType = ref('all')

// Date range
const {
  selectedRange, customDateRange, isDatePickerOpen,
  dateRange, formatDateRangeDisplay, applyCustomRange: baseApplyCustomRange,
} = useDateRange()

watch([filterUser, filterAction, filterResourceType], () => applyFilter())

function applyCustomRange() {
  baseApplyCustomRange()
  applyFilter()
}

watch(selectedRange, (val) => {
  if (val !== 'custom') applyFilter()
})

const columns = computed<Column<AuditLogEntry>[]>(() => [
  { key: 'user_name', label: t('auditLogs.user'), sortable: true },
  { key: 'action', label: t('auditLogs.action') },
  { key: 'resource_type', label: t('auditLogs.resource') },
  { key: 'changes', label: t('auditLogs.changes') },
  { key: 'created_at', label: t('auditLogs.date'), sortable: true },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

const resourceTypes = [
  { value: 'account', label: 'Account' },
  { value: 'ai_context', label: 'AI context' },
  { value: 'campaign', label: 'Campaign' },
  { value: 'chatbot_settings', label: 'Chatbot settings' },
  { value: 'chatbot_flow', label: 'Chatbot flow' },
  { value: 'ivr_flow', label: 'IVR flow' },
  { value: 'keyword_rule', label: 'Keyword rule' },
  { value: 'team', label: 'Team' },
  { value: 'template', label: 'Template' },
]

function resourceLabel(type: string): string {
  return resourceTypes.find(r => r.value === type)?.label ?? humanize(type)
}

function handlePageChange(page: number) {
  currentPage.value = page
  fetchLogs()
}

async function fetchLogs() {
  isLoading.value = true
  try {
    const { from, to } = dateRange.value
    const params: Record<string, any> = { page: currentPage.value, limit: pageSize, from, to }
    if (filterUser.value && filterUser.value !== 'all') params.user_id = filterUser.value
    if (filterAction.value && filterAction.value !== 'all') params.action = filterAction.value
    if (filterResourceType.value && filterResourceType.value !== 'all') params.resource_type = filterResourceType.value

    const response = await auditLogsService.list(params)
    const data = (response.data as any).data || response.data
    logs.value = data.audit_logs || []
    totalItems.value = data.total || 0
  } catch {
    logs.value = []
    totalItems.value = 0
  } finally {
    isLoading.value = false
  }
}

function applyFilter() {
  currentPage.value = 1
  fetchLogs()
}

function changeSummary(log: AuditLogEntry): string {
  if (!log.changes || log.changes.length === 0) return '—'
  if (log.action === 'created') return `${log.changes.length} fields set`
  if (log.action === 'deleted') return `${log.changes.length} fields`
  return log.changes.map(c => formatLabel(c.field)).join(', ')
}

onMounted(async () => {
  await usersStore.fetchUsers()
  fetchLogs()
})
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader
      :title="t('auditLogs.title')"
      :description="t('auditLogs.description')"
      :icon="ScrollText"
    />

    <div class="flex-1 scroll-y p-4">
      <article class="card vstack gap-4">
        <div data-toolbar>
          <div>
            <select class="w-auto" v-model="filterUser" :aria-label="t('auditLogs.allUsers')">
              <option value="all">{{ t('auditLogs.allUsers') }}</option>
              <option v-for="user in usersStore.users" :key="user.id" :value="user.id">
                {{ user.full_name || user.email }}
              </option>
            </select>
            <select class="w-auto" v-model="filterAction" :aria-label="t('auditLogs.allActions')">
              <option value="all">{{ t('auditLogs.allActions') }}</option>
              <option value="created">{{ t('auditLogs.created') }}</option>
              <option value="updated">{{ t('auditLogs.updated') }}</option>
              <option value="deleted">{{ t('auditLogs.deleted') }}</option>
            </select>
            <select class="w-auto" v-model="filterResourceType" :aria-label="t('auditLogs.allResources')">
              <option value="all">{{ t('auditLogs.allResources') }}</option>
              <option v-for="rt in resourceTypes" :key="rt.value" :value="rt.value">{{ rt.label }}</option>
            </select>
            <DateRangePicker
              v-model:selected-range="selectedRange"
              v-model:custom-date-range="customDateRange"
              v-model:is-date-picker-open="isDatePickerOpen"
              :format-date-range-display="formatDateRangeDisplay"
              @apply-custom="applyCustomRange"
            />
          </div>
        </div>
        <DataTable
          :items="logs"
          :columns="columns"
          :is-loading="isLoading"
          :empty-icon="ScrollText"
          :empty-title="t('auditLogs.noLogs')"
          :empty-description="t('auditLogs.noLogsDesc')"
          v-model:sort-key="sortKey"
          v-model:sort-direction="sortDirection"
          server-pagination
          :current-page="currentPage"
          :total-items="totalItems"
          :page-size="pageSize"
          item-name="audit logs"
          @page-change="handlePageChange"
        >
          <template #cell-user_name="{ item: log }">
            <RouterLink :to="`/settings/audit-logs/${log.id}`">
              <strong>{{ log.user_name }}</strong>
            </RouterLink>
          </template>

          <template #cell-action="{ item: log }">
            <span class="badge" data-label>{{ t(`auditLogs.${log.action}`, humanize(log.action)) }}</span>
          </template>

          <template #cell-resource_type="{ item: log }">
            {{ resourceLabel(log.resource_type) }}
          </template>

          <template #cell-changes="{ item: log }">
            <span class="text-light">{{ changeSummary(log) }}</span>
          </template>

          <template #cell-created_at="{ item: log }">
            <span class="text-light">{{ formatDate(log.created_at) }}</span>
          </template>

          <template #cell-actions="{ item: log }">
            <div class="hstack items-center justify-end gap-1 flex-nowrap">
              <RouterLink :to="`/settings/audit-logs/${log.id}`" class="button outline small">{{ t('common.view') }}</RouterLink>
            </div>
          </template>
        </DataTable>
      </article>
    </div>
  </div>
</template>
