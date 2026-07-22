<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { chatbotService } from '@/services/api'
import { toast } from 'vue-sonner'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Sparkles, Save } from 'lucide-vue-next'

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
    icon-gradient="bg-gradient-to-br from-violet-500 to-purple-600 shadow-violet-500/20"
    back-link="/chatbot/ai"
    :breadcrumbs="breadcrumbs"
    :is-loading="isLoading"
    :is-not-found="isNotFound"
    :not-found-title="$t('aiContexts.notFound', 'AI Context not found')"
  >
    <template #actions>
      <div class="flex items-center gap-2">
        <Button v-if="hasChanges || isNew" size="sm" @click="save" :disabled="isSaving">
          <Save class="h-4 w-4 mr-1" /> {{ isSaving ? $t('common.saving', 'Saving...') : isNew ? $t('common.create') : $t('common.save') }}
        </Button>
      </div>
    </template>

    <!-- Details Card -->
    <Card>
      <CardHeader class="pb-3">
        <div class="flex items-center justify-between">
          <CardTitle class="text-sm font-medium">{{ $t('aiContexts.details', 'Details') }}</CardTitle>
          <Badge :variant="form.enabled ? 'default' : 'secondary'">
            {{ form.enabled ? $t('aiContexts.active', 'Active') : $t('aiContexts.inactive', 'Inactive') }}
          </Badge>
        </div>
      </CardHeader>
      <CardContent class="space-y-4">
        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('aiContexts.name', 'Name') }} *</Label>
          <Input v-model="form.name" :placeholder="$t('aiContexts.namePlaceholder', 'e.g. Product Knowledge Base')" />
        </div>

        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('aiContexts.contextType', 'Context Type') }}</Label>
          <Select v-model="form.context_type">
            <SelectTrigger><SelectValue :placeholder="$t('aiContexts.selectType', 'Select type')" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="static">{{ $t('aiContexts.staticContent', 'Static Content') }}</SelectItem>
              <SelectItem value="api">{{ $t('aiContexts.apiFetch', 'API Fetch') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('aiContexts.triggerKeywords', 'Trigger Keywords') }}</Label>
          <Input
            v-model="form.trigger_keywords"
            :placeholder="$t('aiContexts.triggerKeywordsPlaceholder', 'pricing, plans, cost')"
          />
          <p class="text-xs text-muted-foreground">
            {{ $t('aiContexts.triggerKeywordsHint', 'Comma-separated. Leave empty to always trigger.') }}
          </p>
        </div>

        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('aiContexts.contentPrompt', 'Static Content') }}</Label>
          <Textarea
            v-model="form.static_content"
            :placeholder="$t('aiContexts.contentPlaceholder', 'Enter prompt text') + '...'"
            :rows="6"
          />
          <p class="text-xs text-muted-foreground">
            {{ $t('aiContexts.contentHint', 'Supports variable placeholders') }}:
            <code class="bg-muted px-1 rounded">{{ variableExample('variable') }}</code>
          </p>
        </div>

        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('aiContexts.priorityLabel', 'Priority') }}</Label>
            <Input v-model.number="form.priority" type="number" min="1" max="100" />
            <p class="text-xs text-muted-foreground">{{ $t('aiContexts.priorityHint', 'Higher priority contexts are used first (1-100)') }}</p>
          </div>
          <div class="flex items-center gap-2 pt-6">
            <Switch :checked="form.enabled" @update:checked="form.enabled = $event" />
            <Label class="text-xs">{{ $t('aiContexts.enabled', 'Enabled') }}</Label>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- API Configuration Card (only for api type) -->
    <Card v-if="form.context_type === 'api'">
      <CardHeader class="pb-3">
        <CardTitle class="text-sm font-medium">{{ $t('aiContexts.apiConfiguration', 'API Configuration') }}</CardTitle>
      </CardHeader>
      <CardContent class="space-y-4">
        <p class="text-xs text-muted-foreground">{{ $t('aiContexts.apiConfigHint', 'Configure the external API to fetch dynamic context.') }}</p>

        <div class="grid grid-cols-4 gap-4">
          <div class="col-span-1 space-y-1.5">
            <Label class="text-xs">{{ $t('aiContexts.method', 'Method') }}</Label>
            <Select v-model="form.api_method">
              <SelectTrigger><SelectValue :placeholder="$t('aiContexts.method', 'Method')" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="GET">GET</SelectItem>
                <SelectItem value="POST">POST</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="col-span-3 space-y-1.5">
            <Label class="text-xs">{{ $t('aiContexts.apiUrl', 'API URL') }}</Label>
            <Input
              v-model="form.api_url"
              :placeholder="$t('aiContexts.apiUrlPlaceholder', 'https://api.example.com/data')"
            />
          </div>
        </div>
        <p class="text-xs text-muted-foreground">
          {{ $t('aiContexts.variables', 'Variables') }}: <code class="bg-muted px-1 rounded">{{ variableExample('phone_number') }}</code>, <code class="bg-muted px-1 rounded">{{ variableExample('user_message') }}</code>
        </p>

        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('aiContexts.headersOptional', 'Headers (optional)') }}</Label>
          <Textarea
            v-model="form.api_headers"
            placeholder='{"Authorization": "Bearer ..."}'
            :rows="3"
          />
          <p class="text-xs text-muted-foreground">{{ $t('aiContexts.headersHint', 'JSON format') }}</p>
        </div>

        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('aiContexts.responsePath', 'Response Path') }}</Label>
          <Input
            v-model="form.api_response_path"
            :placeholder="$t('aiContexts.responsePathPlaceholder', '$.data.result')"
          />
          <p class="text-xs text-muted-foreground">{{ $t('aiContexts.responsePathHint', 'JSONPath to extract from the API response') }}</p>
        </div>
      </CardContent>
    </Card>

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
