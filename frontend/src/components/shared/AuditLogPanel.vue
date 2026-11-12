<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { auditLogsService, type AuditLogEntry } from '@/services/api'
import { formatDateTime, formatLabel } from '@/lib/utils'
import { History, ChevronDown } from '@/lib/icons'

const props = defineProps<{
  resourceType: string
  resourceId: string
}>()

const logs = ref<AuditLogEntry[]>([])
const isLoading = ref(false)
const total = ref(0)
const page = ref(1)
const limit = 10

const actionConfig: Record<string, { variant: string; label: string }> = {
  created: { variant: 'success', label: 'Created' },
  updated: { variant: 'secondary', label: 'Updated' },
  deleted: { variant: 'danger', label: 'Deleted' },
}

function formatValue(val: any): string {
  if (val === null || val === undefined || val === '') return '—'
  if (typeof val === 'boolean') return val ? 'Yes' : 'No'
  if (Array.isArray(val)) {
    if (val.length === 0) return '—'
    // Format array of objects (e.g. buttons) as readable text
    if (typeof val[0] === 'object' && val[0] !== null) {
      return val.map(item => item.text || item.name || item.title || JSON.stringify(item)).join(', ')
    }
    return val.join(', ') || '—'
  }
  if (typeof val === 'object') {
    // For simple objects with a "body" key (like response_content), show the body
    if (val.body) return String(val.body)
    const s = JSON.stringify(val)
    return s.length > 120 ? s.slice(0, 120) + '…' : s
  }
  return String(val)
}

async function loadLogs(append = false) {
  isLoading.value = true
  try {
    const response = await auditLogsService.list({
      resource_type: props.resourceType,
      resource_id: props.resourceId,
      page: page.value,
      limit,
    })
    const data = (response.data as any).data || response.data
    const entries = data.audit_logs || []
    if (append) {
      logs.value.push(...entries)
    } else {
      logs.value = entries
    }
    total.value = data.total || 0
  } catch {
    // silently fail — audit logs are not critical
  } finally {
    isLoading.value = false
  }
}

function loadMore() {
  page.value++
  loadLogs(true)
}

onMounted(() => loadLogs())
</script>

<template>
  <article class="card">
    <header>
      <div class="hstack gap-2">
        <History :size="16" />
        <h3>{{ $t('common.activityLog', 'Activity Log') }}</h3>
      </div>
    </header>

    <!-- Loading state -->
    <div v-if="isLoading && logs.length === 0" class="flex items-center justify-center p-4" role="status" aria-busy="true" />

    <!-- Empty state -->
    <p v-else-if="logs.length === 0" class="text-light align-center p-4">
      {{ $t('common.noActivity', 'No activity recorded yet') }}
    </p>

    <!-- Timeline -->
    <ul v-else class="unstyled vstack gap-4">
      <li v-for="log in logs" :key="log.id" class="vstack gap-2">
        <div class="hstack gap-2 flex-wrap">
          <strong>{{ log.user_name }}</strong>
          <span class="badge" :data-variant="actionConfig[log.action]?.variant || 'secondary'">
            {{ actionConfig[log.action]?.label || log.action }}
          </span>
          <small class="text-light">{{ formatDateTime(log.created_at) }}</small>
        </div>

        <!-- Changes -->
        <div v-if="log.action === 'updated' && log.changes?.length > 0" class="vstack gap-1">
          <div
            v-for="(change, idx) in log.changes"
            :key="idx"
            class="min-w-0"
          >
            <small><strong>{{ formatLabel(change.field) }}:</strong></small>
            <div>
              <small class="text-light">{{ formatValue(change.old_value) }}</small>
              <small> → </small>
              <small>{{ formatValue(change.new_value) }}</small>
            </div>
          </div>
        </div>

        <!-- Created fields summary -->
        <small v-else-if="log.action === 'created' && log.changes?.length > 0" class="text-light">
          {{ log.changes.length }} field{{ log.changes.length !== 1 ? 's' : '' }} set
        </small>
      </li>
    </ul>

    <!-- Load more -->
    <footer v-if="logs.length < total" class="flex justify-center">
      <button
        type="button"
        class="ghost small"
        :disabled="isLoading"
        :aria-busy="isLoading ? 'true' : undefined"
        data-spinner="small"
        @click="loadMore"
      >
        <ChevronDown :size="14" />
        {{ isLoading ? $t('common.loading') + '...' : $t('common.loadMore', 'Load more') }}
      </button>
    </footer>
  </article>
</template>
