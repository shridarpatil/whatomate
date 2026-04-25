<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import { auditLogsService, type AuditLogEntry } from '@/services/api'
import { formatDateTime, formatLabel } from '@/lib/utils'
import { ScrollText, Info, ExternalLink, ArrowRight } from 'lucide-vue-next'

const route = useRoute()
const { t } = useI18n()

const logId = computed(() => route.params.id as string)
const log = ref<AuditLogEntry | null>(null)
const isLoading = ref(true)

const resourceRouteMap: Record<string, (id: string) => string> = {
  template: (id) => `/templates/${id}`,
  chatbot_settings: () => `/settings/chatbot`,
  chatbot_flow: (id) => `/chatbot/flows/${id}`,
  keyword_rule: (id) => `/chatbot/keywords/${id}`,
  account: (id) => `/settings/accounts/${id}`,
  organization: () => `/settings`,
  user: (id) => `/settings/users/${id}`,
  role: (id) => `/settings/roles/${id}`,
  team: (id) => `/settings/teams/${id}`,
  webhook: () => `/settings/webhooks`,
  ivr_flow: (id) => `/calling/ivr-flows/${id}`,
  campaign: (id) => `/campaigns/${id}`,
  custom_action: () => `/settings/custom-actions`,
  canned_response: () => `/settings/canned-responses`,
  api_key: () => `/settings/api-keys`,
  ai_context: () => `/chatbot/ai`,
  contact: (id) => `/chat?contact=${id}`,
  tag: () => `/settings/tags`,
}

const resourceLink = computed(() => {
  if (!log.value) return null
  const fn = resourceRouteMap[log.value.resource_type]
  return fn ? fn(log.value.resource_id) : null
})

const title = computed(() => {
  if (!log.value) return t('auditLogs.title')
  return `${formatLabel(log.value.resource_type)} ${log.value.action}`
})

const breadcrumbs = computed(() => [
  { label: t('nav.settings'), href: '/settings' },
  { label: t('auditLogs.title'), href: '/settings/audit-logs' },
  { label: title.value },
])

function formatValue(val: any): string {
  if (val === null || val === undefined || val === '') return '—'
  if (typeof val === 'boolean') return val ? 'Yes' : 'No'
  if (Array.isArray(val)) {
    if (val.length === 0) return '—'
    if (typeof val[0] === 'object' && val[0] !== null) {
      return val.map(item => item.text || item.name || item.title || JSON.stringify(item)).join(', ')
    }
    return val.join(', ') || '—'
  }
  if (typeof val === 'object') {
    if (val.body) return String(val.body)
    return JSON.stringify(val)
  }
  return String(val)
}

function actionVariant(action: string): string {
  switch (action) {
    case 'created': return 'bg-green-500/10 text-green-500 border-green-500/20'
    case 'updated': return 'bg-blue-500/10 text-blue-500 border-blue-500/20'
    case 'deleted': return 'bg-red-500/10 text-red-500 border-red-500/20'
    default: return ''
  }
}

onMounted(async () => {
  try {
    const response = await auditLogsService.get(logId.value)
    log.value = (response.data as any).data || response.data
  } catch {
    // handled by isNotFound
  } finally {
    isLoading.value = false
  }
})
</script>

<template>
  <DetailPageLayout
    :title="title"
    :icon="ScrollText"
    back-link="/settings/audit-logs"
    :breadcrumbs="breadcrumbs"
    :is-loading="isLoading"
    :is-not-found="!isLoading && !log"
    :not-found-title="t('auditLogs.noLogs')"
  >
    <Card v-if="log">
      <CardHeader class="pb-3">
        <CardTitle class="text-sm font-medium">{{ t('auditLogs.changes') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <div v-if="log.changes && log.changes.length > 0" class="space-y-3">
          <div
            v-for="(change, idx) in log.changes"
            :key="idx"
            class="rounded-md bg-muted/50 px-3 py-2.5"
          >
            <span class="text-sm font-medium">{{ formatLabel(change.field) }}</span>
            <div class="mt-1 text-sm">
              <template v-if="log.action === 'updated'">
                <div class="flex items-start gap-2 text-muted-foreground">
                  <span class="text-red-400 line-through break-words">{{ formatValue(change.old_value) }}</span>
                  <ArrowRight class="h-4 w-4 shrink-0 mt-0.5" />
                  <span class="text-green-400 break-words">{{ formatValue(change.new_value) }}</span>
                </div>
              </template>
              <template v-else-if="log.action === 'created'">
                <span class="text-muted-foreground break-words">{{ formatValue(change.new_value) }}</span>
              </template>
              <template v-else>
                <span class="text-red-400 break-words">{{ formatValue(change.old_value) }}</span>
              </template>
            </div>
          </div>
        </div>
        <p v-else class="text-sm text-muted-foreground">{{ t('auditLogs.noChanges') }}</p>
      </CardContent>
    </Card>

    <template #sidebar>
      <Card v-if="log" class="overflow-hidden">
        <CardHeader class="pb-3">
          <div class="flex items-center gap-2">
            <Info class="h-4 w-4 text-muted-foreground" />
            <CardTitle class="text-sm font-medium">{{ t('common.details', 'Details') }}</CardTitle>
          </div>
        </CardHeader>
        <CardContent class="space-y-3 text-sm">
          <div>
            <span class="text-muted-foreground text-xs">{{ t('auditLogs.user') }}</span>
            <p class="font-medium">{{ log.user_name }}</p>
          </div>
          <div>
            <span class="text-muted-foreground text-xs">{{ t('auditLogs.action') }}</span>
            <div class="mt-0.5">
              <Badge variant="outline" :class="[actionVariant(log.action), 'text-xs']">
                {{ t(`auditLogs.${log.action}`) }}
              </Badge>
            </div>
          </div>
          <div>
            <span class="text-muted-foreground text-xs">{{ t('auditLogs.resource') }}</span>
            <p class="font-medium">{{ formatLabel(log.resource_type) }}</p>
          </div>
          <div>
            <span class="text-muted-foreground text-xs">{{ t('auditLogs.date') }}</span>
            <p>{{ formatDateTime(log.created_at) }}</p>
          </div>
          <RouterLink v-if="resourceLink" :to="resourceLink">
            <Button variant="outline" size="sm" class="w-full mt-2">
              <ExternalLink class="h-3.5 w-3.5 mr-1.5" />
              {{ t('auditLogs.viewResource') }}
            </Button>
          </RouterLink>
        </CardContent>
      </Card>
    </template>
  </DetailPageLayout>
</template>
