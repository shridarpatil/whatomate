<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { PageHeader, DataTable, DateRangePicker, type Column } from '@/components/shared'
import { auditLogsService, type AuditLogEntry } from '@/services/api'
import { useUsersStore } from '@/stores/users'
import { useDateRange } from '@/composables/useDateRange'
import { ScrollText } from 'lucide-vue-next'
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
    case 'created': return 'bg-green-500/10 text-green-500 border-green-500/20'
    case 'updated': return 'bg-blue-500/10 text-blue-500 border-blue-500/20'
    case 'deleted': return 'bg-red-500/10 text-red-500 border-red-500/20'
    default: return ''
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
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader
      :title="t('auditLogs.title')"
      :description="t('auditLogs.description')"
      :icon="ScrollText"
      icon-gradient="bg-gradient-to-br from-amber-500 to-orange-600 shadow-amber-500/20"
    />

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div>
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>{{ t('auditLogs.allActivity') }}</CardTitle>
                  <CardDescription>{{ t('auditLogs.allActivityDesc') }}</CardDescription>
                </div>
                <div class="flex items-center gap-2 flex-wrap">
                  <Select v-model="filterUser" @update:model-value="applyFilter">
                    <SelectTrigger class="w-[180px]">
                      <SelectValue :placeholder="t('auditLogs.allUsers')" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{{ t('auditLogs.allUsers') }}</SelectItem>
                      <SelectItem v-for="user in usersStore.users" :key="user.id" :value="user.id">
                        {{ user.full_name || user.email }}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                  <Select v-model="filterAction" @update:model-value="applyFilter">
                    <SelectTrigger class="w-[140px]">
                      <SelectValue :placeholder="t('auditLogs.allActions')" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{{ t('auditLogs.allActions') }}</SelectItem>
                      <SelectItem value="created">{{ t('auditLogs.created') }}</SelectItem>
                      <SelectItem value="updated">{{ t('auditLogs.updated') }}</SelectItem>
                      <SelectItem value="deleted">{{ t('auditLogs.deleted') }}</SelectItem>
                    </SelectContent>
                  </Select>
                  <Select v-model="filterResourceType" @update:model-value="applyFilter">
                    <SelectTrigger class="w-[180px]">
                      <SelectValue :placeholder="t('auditLogs.allResources')" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{{ t('auditLogs.allResources') }}</SelectItem>
                      <SelectItem value="account">Account</SelectItem>
                      <SelectItem value="ai_context">AI Context</SelectItem>
                      <SelectItem value="campaign">Campaign</SelectItem>
                      <SelectItem value="chatbot_settings">Chatbot Settings</SelectItem>
                      <SelectItem value="chatbot_flow">Chatbot Flow</SelectItem>
                      <SelectItem value="ivr_flow">IVR Flow</SelectItem>
                      <SelectItem value="keyword_rule">Keyword Rule</SelectItem>
                      <SelectItem value="team">Team</SelectItem>
                      <SelectItem value="template">Template</SelectItem>
                    </SelectContent>
                  </Select>
                  <DateRangePicker
                    v-model:selected-range="selectedRange"
                    v-model:custom-date-range="customDateRange"
                    v-model:is-date-picker-open="isDatePickerOpen"
                    :format-date-range-display="formatDateRangeDisplay"
                    @apply-custom="applyCustomRange"
                  />
                </div>
              </div>
            </CardHeader>
            <CardContent>
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
                  <div class="py-1">
                    <RouterLink
                      :to="`/settings/audit-logs/${log.id}`"
                      class="font-medium text-inherit no-underline hover:opacity-80"
                    >
                      {{ log.user_name }}
                    </RouterLink>
                  </div>
                </template>

                <template #cell-action="{ item: log }">
                  <div class="py-1">
                    <Badge variant="outline" :class="[actionVariant(log.action), 'text-xs']">
                      {{ t(`auditLogs.${log.action}`) }}
                    </Badge>
                  </div>
                </template>

                <template #cell-resource_type="{ item: log }">
                  <div class="py-1">
                    <span class="text-sm text-muted-foreground">{{ formatLabel(log.resource_type) }}</span>
                  </div>
                </template>

                <template #cell-changes="{ item: log }">
                  <div class="py-1">
                    <span class="text-sm text-muted-foreground">{{ changeSummary(log) }}</span>
                  </div>
                </template>

                <template #cell-created_at="{ item: log }">
                  <div class="py-1">
                    <span class="text-muted-foreground text-sm">{{ formatDate(log.created_at) }}</span>
                  </div>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>
  </div>
</template>
