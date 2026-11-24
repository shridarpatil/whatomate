<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { PageHeader, DataTable, DateRangePicker, type Column } from '@/components/shared'
import { auditLogsService, type AuditLogEntry } from '@/services/api'
import { useUsersStore } from '@/stores/users'
import { useDateRange } from '@/composables/useDateRange'
import { ScrollText } from '@/lib/icons'
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
])

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

function actionVariant(action: string): string {
  switch (action) {
    case 'created': return 'success'
    case 'deleted': return 'danger'
    default: return 'secondary'
  }
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
      <article class="card">
        <header class="hstack justify-between flex-wrap gap-4">
          <div>
            <h3>{{ t('auditLogs.allActivity') }}</h3>
            <p class="text-light">{{ t('auditLogs.allActivityDesc') }}</p>
          </div>
          <div class="hstack gap-2 flex-wrap">
            <select v-model="filterUser" :aria-label="t('auditLogs.allUsers')">
              <option value="all">{{ t('auditLogs.allUsers') }}</option>
              <option v-for="user in usersStore.users" :key="user.id" :value="user.id">
                {{ user.full_name || user.email }}
              </option>
            </select>
            <select v-model="filterAction" :aria-label="t('auditLogs.allActions')">
              <option value="all">{{ t('auditLogs.allActions') }}</option>
              <option value="created">{{ t('auditLogs.created') }}</option>
              <option value="updated">{{ t('auditLogs.updated') }}</option>
              <option value="deleted">{{ t('auditLogs.deleted') }}</option>
            </select>
            <select v-model="filterResourceType" :aria-label="t('auditLogs.allResources')">
              <option value="all">{{ t('auditLogs.allResources') }}</option>
              <option value="account">Account</option>
              <option value="ai_context">AI Context</option>
              <option value="campaign">Campaign</option>
              <option value="chatbot_settings">Chatbot Settings</option>
              <option value="chatbot_flow">Chatbot Flow</option>
              <option value="ivr_flow">IVR Flow</option>
              <option value="keyword_rule">Keyword Rule</option>
              <option value="team">Team</option>
              <option value="template">Template</option>
            </select>
            <DateRangePicker
              v-model:selected-range="selectedRange"
              v-model:custom-date-range="customDateRange"
              v-model:is-date-picker-open="isDatePickerOpen"
              :format-date-range-display="formatDateRangeDisplay"
              @apply-custom="applyCustomRange"
            />
          </div>
        </header>
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
            <span class="badge" :data-variant="actionVariant(log.action)">
              {{ t(`auditLogs.${log.action}`) }}
            </span>
          </template>

          <template #cell-resource_type="{ item: log }">
            <span class="text-light">{{ formatLabel(log.resource_type) }}</span>
          </template>

          <template #cell-changes="{ item: log }">
            <span class="text-light">{{ changeSummary(log) }}</span>
          </template>

          <template #cell-created_at="{ item: log }">
            <span class="text-light">{{ formatDate(log.created_at) }}</span>
          </template>
        </DataTable>
      </article>
    </div>
  </div>
</template>
