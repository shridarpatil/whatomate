<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { chatbotService } from '@/services/api'
import { toast } from '@/lib/toast'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import { Sparkles, Save } from '@/lib/icons'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const contextId = computed(() => route.params.id as string)
const isNew = computed(() => contextId.value === 'new')
const contextData = ref<any>(null)
const isLoading = ref(true)
const isNotFound = ref(false)
const isSaving = ref(false)
const hasChanges = ref(false)

const { showLeaveDialog, confirmLeave, cancelLeave } = useUnsavedChangesGuard(hasChanges)

const form = ref({
  name: '',
  context_type: 'static',
  trigger_keywords: '',
  static_content: '',
  api_url: '',
  api_method: 'GET',
  api_headers: '{}',
  api_response_path: '',
  priority: 10,
  enabled: true,
})

const breadcrumbs = computed(() => [
  { label: t('nav.chatbot', 'Chatbot'), href: '/chatbot' },
  { label: t('nav.aiContexts', 'AI Contexts'), href: '/chatbot/ai' },
  { label: isNew.value ? t('aiContexts.newContext', 'New Context') : (contextData.value?.name || '') },
])

// Helper to display variable placeholders without Vue parsing issues
const variableExample = (name: string) => `{{${name}}}`

async function loadContext() {
  isLoading.value = true
  isNotFound.value = false
  try {
    const response = await chatbotService.getAIContext(contextId.value)
    const data = (response.data as any).data?.context || (response.data as any).data || response.data
    contextData.value = data
    syncForm(data)
    nextTick(() => { hasChanges.value = false })
  } catch {
    isNotFound.value = true
  } finally {
    isLoading.value = false
  }
}

function syncForm(data: any) {
  if (!data) return
  form.value = {
    name: data.name || '',
    context_type: data.context_type || 'static',
    trigger_keywords: (data.trigger_keywords || []).join(', '),
    static_content: data.static_content || '',
    api_url: data.api_config?.url || '',
    api_method: data.api_config?.method || 'GET',
    api_headers: JSON.stringify(data.api_config?.headers || {}, null, 2),
    api_response_path: data.api_config?.response_path || '',
    priority: data.priority ?? 10,
    enabled: data.enabled ?? true,
  }
}

// Track form changes
watch(form, () => {
  hasChanges.value = true
}, { deep: true })

function parseJSON(str: string): Record<string, any> {
  if (!str.trim()) return {}
  try {
    return JSON.parse(str)
  } catch {
    throw new Error('Invalid JSON')
  }
}

function buildPayload() {
  let headers = {}
  if (form.value.api_headers.trim()) {
    headers = parseJSON(form.value.api_headers)
  }

  return {
    name: form.value.name,
    context_type: form.value.context_type,
    trigger_keywords: form.value.trigger_keywords.split(',').map(k => k.trim()).filter(Boolean),
    static_content: form.value.static_content,
    api_config: form.value.context_type === 'api' ? {
      url: form.value.api_url,
      method: form.value.api_method,
      headers,
      response_path: form.value.api_response_path,
    } : {},
    priority: form.value.priority,
    enabled: form.value.enabled,
  }
}

async function save() {
  if (!form.value.name.trim()) {
    toast.error(t('aiContexts.enterName', 'Name is required'))
    return
  }

  if (form.value.context_type === 'api' && !form.value.api_url.trim()) {
    toast.error(t('aiContexts.enterApiUrl', 'API URL is required'))
    return
  }

  isSaving.value = true
  try {
    const payload = buildPayload()

    if (isNew.value) {
      const response = await chatbotService.createAIContext(payload)
      const created = (response.data as any).data?.context || (response.data as any).data || response.data
      hasChanges.value = false
      toast.success(t('common.createdSuccess', { resource: t('resources.AIContext', 'AI Context') }))
      router.replace(`/chatbot/ai/${created.id}`)
    } else {
      await chatbotService.updateAIContext(contextId.value, payload)
      await loadContext()
      hasChanges.value = false
      toast.success(t('common.updatedSuccess', { resource: t('resources.AIContext', 'AI Context') }))
    }
  } catch (err: any) {
    if (err?.message === 'Invalid JSON') {
      toast.error(t('aiContexts.invalidHeaders', 'Invalid JSON in headers'))
    } else {
      toast.error(
        isNew.value
          ? t('common.failedSave', { resource: t('resources.AIContext', 'AI Context') })
          : t('common.failedSave', { resource: t('resources.AIContext', 'AI Context') })
      )
    }
  } finally {
    isSaving.value = false
  }
}

onMounted(async () => {
  if (isNew.value) {
    isLoading.value = false
    hasChanges.value = false
  } else {
    await loadContext()
  }
})
</script>

<template>
  <div class="h-full">
  <DetailPageLayout
    :title="isNew ? $t('aiContexts.newContext', 'New AI Context') : (contextData?.name || '')"
    :icon="Sparkles"
    back-link="/chatbot/ai"
    :breadcrumbs="breadcrumbs"
    :is-loading="isLoading"
    :is-not-found="isNotFound"
    :not-found-title="$t('aiContexts.notFound', 'AI Context not found')"
  >
    <template #actions>
      <div class="hstack gap-2">
        <button v-if="hasChanges || isNew" type="button" class="small" :disabled="isSaving" :aria-busy="isSaving ? 'true' : undefined" data-spinner="small" @click="save">
          <Save v-if="!isSaving" :size="16" /> {{ isSaving ? $t('common.saving', 'Saving...') : isNew ? $t('common.create') : $t('common.save') }}
        </button>
      </div>
    </template>

    <!-- Details Card -->
    <article class="card">
      <header class="hstack justify-between">
        <h3>{{ $t('aiContexts.details', 'Details') }}</h3>
        <span class="badge" :data-variant="form.enabled ? 'success' : 'secondary'">
          {{ form.enabled ? $t('aiContexts.active', 'Active') : $t('aiContexts.inactive', 'Inactive') }}
        </span>
      </header>
      <div class="vstack gap-4">
        <label data-field>
          {{ $t('aiContexts.name', 'Name') }} *
          <input v-model="form.name" :placeholder="$t('aiContexts.namePlaceholder', 'e.g. Product Knowledge Base')">
        </label>

        <label data-field>
          {{ $t('aiContexts.contextType', 'Context Type') }}
          <select v-model="form.context_type">
            <option value="static">{{ $t('aiContexts.staticContent', 'Static Content') }}</option>
            <option value="api">{{ $t('aiContexts.apiFetch', 'API Fetch') }}</option>
          </select>
        </label>

        <label data-field>
          {{ $t('aiContexts.triggerKeywords', 'Trigger Keywords') }}
          <input
            v-model="form.trigger_keywords"
            :placeholder="$t('aiContexts.triggerKeywordsPlaceholder', 'pricing, plans, cost')"
          >
          <small data-hint>{{ $t('aiContexts.triggerKeywordsHint', 'Comma-separated. Leave empty to always trigger.') }}</small>
        </label>

        <label data-field>
          {{ $t('aiContexts.contentPrompt', 'Static Content') }}
          <textarea
            v-model="form.static_content"
            :placeholder="$t('aiContexts.contentPlaceholder', 'Enter prompt text') + '...'"
            rows="6"
          />
          <small data-hint>
            {{ $t('aiContexts.contentHint', 'Supports variable placeholders') }}:
            <code>{{ variableExample('variable') }}</code>
          </small>
        </label>

        <div class="row">
          <label data-field class="col-6">
            {{ $t('aiContexts.priorityLabel', 'Priority') }}
            <input v-model.number="form.priority" type="number" min="1" max="100">
            <small data-hint>{{ $t('aiContexts.priorityHint', 'Higher priority contexts are used first (1-100)') }}</small>
          </label>
          <label class="hstack gap-2 col-6">
            <input v-model="form.enabled" type="checkbox" role="switch">
            {{ $t('aiContexts.enabled', 'Enabled') }}
          </label>
        </div>
      </div>
    </article>

    <!-- API Configuration Card (only for api type) -->
    <article v-if="form.context_type === 'api'" class="card">
      <header>
        <h3>{{ $t('aiContexts.apiConfiguration', 'API Configuration') }}</h3>
      </header>
      <div class="vstack gap-4">

        <div class="row">
          <label data-field class="col-3">
            {{ $t('aiContexts.method', 'Method') }}
            <select v-model="form.api_method">
              <option value="GET">GET</option>
              <option value="POST">POST</option>
            </select>
          </label>
          <label data-field class="col-9">
            {{ $t('aiContexts.apiUrl', 'API URL') }}
            <input
              v-model="form.api_url"
              :placeholder="$t('aiContexts.apiUrlPlaceholder', 'https://api.example.com/data')"
            >
          </label>
        </div>
        <small class="text-light">
          {{ $t('aiContexts.variables', 'Variables') }}: <code>{{ variableExample('phone_number') }}</code>, <code>{{ variableExample('user_message') }}</code>
        </small>

        <label data-field>
          {{ $t('aiContexts.headersOptional', 'Headers (optional)') }}
          <textarea
            v-model="form.api_headers"
            placeholder='{"Authorization": "Bearer ..."}'
            rows="3"
          />
          <small data-hint>{{ $t('aiContexts.headersHint', 'JSON format') }}</small>
        </label>

        <label data-field>
          {{ $t('aiContexts.responsePath', 'Response Path') }}
          <input
            v-model="form.api_response_path"
            :placeholder="$t('aiContexts.responsePathPlaceholder', '$.data.result')"
          >
          <small data-hint>{{ $t('aiContexts.responsePathHint', 'JSONPath to extract from the API response') }}</small>
        </label>
      </div>
    </article>

    <!-- Activity Log -->
    <AuditLogPanel
      v-if="contextData && !isNew"
      resource-type="ai_context"
      :resource-id="contextData.id"
    />

    <!-- Sidebar -->
    <template v-if="!isNew" #sidebar>
      <MetadataPanel
        :created-at="contextData?.created_at"
        :updated-at="contextData?.updated_at"
        :created-by-name="contextData?.created_by_name"
        :updated-by-name="contextData?.updated_by_name"
      />
    </template>
  </DetailPageLayout>

  <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
