<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { auditLogsService, type AuditLogEntry } from '@/services/api'
import { formatDateTime, formatLabel } from '@/lib/utils'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { History, Plus, Pencil, Trash2, Loader2, ChevronDown } from 'lucide-vue-next'

const props = defineProps<{
  resourceType: string
  resourceId: string
}>()

const logs = ref<AuditLogEntry[]>([])
const isLoading = ref(false)
const total = ref(0)
const page = ref(1)
const limit = 10

const actionConfig: Record<string, { icon: any; color: string; label: string }> = {
  created: { icon: Plus, color: 'bg-green-500', label: 'Created' },
  updated: { icon: Pencil, color: 'bg-blue-500', label: 'Updated' },
  deleted: { icon: Trash2, color: 'bg-red-500', label: 'Deleted' },
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
    return JSON.stringify(val)
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
  <Card class="overflow-hidden">
    <CardHeader class="pb-3">
      <div class="flex items-center gap-2">
        <History class="h-4 w-4 text-muted-foreground" />
        <CardTitle class="text-sm font-medium">{{ $t('common.activityLog', 'Activity Log') }}</CardTitle>
      </div>
    </CardHeader>
    <CardContent>
      <!-- Loading state -->
      <div v-if="isLoading && logs.length === 0" class="flex items-center justify-center py-8">
        <Loader2 class="h-5 w-5 animate-spin text-muted-foreground" />
      </div>

      <!-- Empty state -->
      <p v-else-if="logs.length === 0" class="text-sm text-muted-foreground text-center py-6">
        {{ $t('common.noActivity', 'No activity recorded yet') }}
      </p>

      <!-- Timeline -->
      <div v-else class="relative">
        <!-- Vertical line -->
        <div class="absolute left-3 top-3 bottom-3 w-px bg-border" />

        <div v-for="log in logs" :key="log.id" class="relative pl-9 pb-6 last:pb-0">
          <!-- Dot -->
          <div
            :class="[
              'absolute left-1.5 top-1 w-3 h-3 rounded-full border-2 border-background',
              actionConfig[log.action]?.color || 'bg-muted-foreground'
            ]"
          />

          <!-- Content -->
          <div>
            <div class="flex items-center gap-2 flex-wrap">
              <span class="text-sm font-medium">{{ log.user_name }}</span>
              <Badge variant="outline" class="text-[10px] px-1.5 py-0">
                {{ actionConfig[log.action]?.label || log.action }}
              </Badge>
              <span class="text-xs text-muted-foreground">{{ formatDateTime(log.created_at) }}</span>
            </div>

            <!-- Changes -->
            <div v-if="log.action === 'updated' && log.changes?.length > 0" class="mt-2 space-y-1">
              <div
                v-for="(change, idx) in log.changes"
                :key="idx"
                class="text-xs rounded-md bg-muted/50 px-2.5 py-1.5 overflow-hidden"
              >
                <span class="font-medium text-foreground">{{ formatLabel(change.field) }}:</span>
                <div class="mt-0.5 text-muted-foreground break-words">
                  <span>{{ formatValue(change.old_value) }}</span>
                  <span class="mx-1">→</span>
                  <span class="text-foreground">{{ formatValue(change.new_value) }}</span>
                </div>
              </div>
            </div>

            <!-- Created fields summary -->
            <div v-else-if="log.action === 'created' && log.changes?.length > 0" class="mt-1">
              <span class="text-xs text-muted-foreground">
                {{ log.changes.length }} field{{ log.changes.length !== 1 ? 's' : '' }} set
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- Load more -->
      <div v-if="logs.length < total" class="mt-4 flex justify-center">
        <Button variant="ghost" size="sm" class="text-xs" :disabled="isLoading" @click="loadMore">
          <ChevronDown class="h-3.5 w-3.5 mr-1" />
          {{ isLoading ? $t('common.loading') + '...' : $t('common.loadMore', 'Load more') }}
        </Button>
      </div>
    </CardContent>
  </Card>
</template>
