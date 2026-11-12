<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import { auditLogsService, type AuditLogEntry } from '@/services/api'
import { formatDateTime, formatLabel } from '@/lib/utils'
import { ScrollText, Info, ExternalLink, ArrowRight } from '@/lib/icons'

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
    case 'created': return 'success'
    case 'deleted': return 'danger'
    default: return 'secondary'
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
    <article v-if="log" class="card">
      <header>
        <h3>{{ t('auditLogs.changes') }}</h3>
      </header>
      <div v-if="log.changes && log.changes.length > 0" class="vstack gap-4">
        <div
          v-for="(change, idx) in log.changes"
          :key="idx"
          class="vstack gap-2"
        >
          <strong>{{ formatLabel(change.field) }}</strong>
          <div v-if="log.action === 'updated'" class="hstack gap-2">
            <del>{{ formatValue(change.old_value) }}</del>
            <ArrowRight :size="16" />
            <ins>{{ formatValue(change.new_value) }}</ins>
          </div>
          <span v-else-if="log.action === 'created'">{{ formatValue(change.new_value) }}</span>
          <del v-else>{{ formatValue(change.old_value) }}</del>
          <hr v-if="idx < log.changes.length - 1">
        </div>
      </div>
      <p v-else class="text-light">{{ t('auditLogs.noChanges') }}</p>
    </article>

    <template #sidebar>
      <article v-if="log" class="card">
        <header class="hstack gap-2">
          <Info :size="16" />
          <h3>{{ t('common.details', 'Details') }}</h3>
        </header>
        <div class="vstack gap-4">
          <div>
            <small class="text-light">{{ t('auditLogs.user') }}</small>
            <p><strong>{{ log.user_name }}</strong></p>
          </div>
          <div>
            <small class="text-light">{{ t('auditLogs.action') }}</small>
            <p>
              <span class="badge" :data-variant="actionVariant(log.action)">
                {{ t(`auditLogs.${log.action}`) }}
              </span>
            </p>
          </div>
          <div>
            <small class="text-light">{{ t('auditLogs.resource') }}</small>
            <p><strong>{{ formatLabel(log.resource_type) }}</strong></p>
          </div>
          <div>
            <small class="text-light">{{ t('auditLogs.date') }}</small>
            <p>{{ formatDateTime(log.created_at) }}</p>
          </div>
          <RouterLink v-if="resourceLink" :to="resourceLink" class="button outline small w-100">
            <ExternalLink :size="14" />
            {{ t('auditLogs.viewResource') }}
          </RouterLink>
        </div>
      </article>
    </template>
  </DetailPageLayout>
</template>
