<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { chatbotService } from '@/services/api'
import { toast } from '@/lib/toast'
import { PageHeader, ConfirmDialog, ErrorState } from '@/components/shared'
import { getErrorMessage } from '@/lib/api-utils'
import {
  Bot,
  Key,
  Workflow,
  Sparkles,
  Power,
  Settings,
  TrendingUp,
  Users,
  MessageSquare,
  Clock
} from '@/lib/icons'

const { t } = useI18n()

interface ChatbotSettings {
  enabled: boolean
  greeting_message: string
  fallback_message: string
  session_timeout_minutes: number
  ai_enabled: boolean
  ai_provider: string
}

interface Stats {
  total_sessions: number
  active_sessions: number
  messages_handled: number
  ai_responses: number
  agent_transfers: number
  keywords_count: number
  flows_count: number
  ai_contexts_count: number
}

const settings = ref<ChatbotSettings>({
  enabled: false,
  greeting_message: '',
  fallback_message: '',
  session_timeout_minutes: 30,
  ai_enabled: false,
  ai_provider: ''
})

const stats = ref<Stats>({
  total_sessions: 0,
  active_sessions: 0,
  messages_handled: 0,
  ai_responses: 0,
  agent_transfers: 0,
  keywords_count: 0,
  flows_count: 0,
  ai_contexts_count: 0
})

const isLoading = ref(true)
const isToggling = ref(false)
const error = ref(false)
const showToggleConfirm = ref(false)

onMounted(async () => {
  try {
    const response = await chatbotService.getSettings()
    // API response is wrapped in { status: "success", data: { settings: {...}, stats: {...} } }
    const data = response.data.data || response.data
    settings.value = data.settings || settings.value
    stats.value = data.stats || stats.value
  } catch (err) {
    console.error('Failed to load chatbot settings:', err)
    error.value = true
  } finally {
    isLoading.value = false
  }
})

async function toggleChatbot() {
  isToggling.value = true
  try {
    const newState = !settings.value.enabled
    await chatbotService.updateSettings({ enabled: newState })
    settings.value.enabled = newState
    toast.success(newState ? t('common.enabledSuccess', { resource: t('resources.Chatbot') }) : t('common.disabledSuccess', { resource: t('resources.Chatbot') }))
  } catch (err: any) {
    toast.error(getErrorMessage(err, t('common.failedToggle', { resource: t('resources.chatbot') })))
  } finally {
    isToggling.value = false
    showToggleConfirm.value = false
  }
}

async function retryFetch() {
  isLoading.value = true
  error.value = false
  try {
    const response = await chatbotService.getSettings()
    const data = response.data.data || response.data
    settings.value = data.settings || settings.value
    stats.value = data.stats || stats.value
  } catch (err) {
    console.error('Failed to load chatbot settings:', err)
    error.value = true
  } finally {
    isLoading.value = false
  }
}

const statCards = computed(() => [
  { title: t('chatbot.totalSessions'), key: 'total_sessions', icon: Users },
  { title: t('chatbot.activeSessions'), key: 'active_sessions', icon: MessageSquare },
  { title: t('chatbot.messagesHandled'), key: 'messages_handled', icon: TrendingUp },
  { title: t('chatbot.aiResponses'), key: 'ai_responses', icon: Sparkles }
])
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader
      :title="$t('chatbot.title')"
      :description="$t('chatbot.subtitle')"
      :icon="Bot"
    >
      <template #actions>
        <div class="hstack gap-2">
          <span class="badge" :data-variant="settings.enabled ? 'success' : 'secondary'">
            {{ settings.enabled ? $t('chatbot.active') : $t('chatbot.inactive') }}
          </span>
          <button
            type="button"
            class="outline small"
            :data-variant="settings.enabled ? 'danger' : undefined"
            :disabled="isToggling"
            @click="showToggleConfirm = true"
          >
            <Power :size="16" />
            {{ settings.enabled ? $t('chatbot.disable') : $t('chatbot.enable') }}
          </button>
        </div>
      </template>
    </PageHeader>

    <!-- Confirm Toggle Dialog -->
    <ConfirmDialog
      v-model:open="showToggleConfirm"
      :title="settings.enabled ? $t('chatbot.confirmDisableTitle') : $t('chatbot.confirmEnableTitle')"
      :description="settings.enabled ? $t('chatbot.confirmDisableDescription') : $t('chatbot.confirmEnableDescription')"
      :confirm-label="settings.enabled ? $t('chatbot.disable') : $t('chatbot.enable')"
      :variant="settings.enabled ? 'destructive' : 'default'"
      :is-submitting="isToggling"
      @confirm="toggleChatbot"
    />

    <!-- Error State -->
    <ErrorState
      v-if="error && !isLoading"
      :title="$t('chatbot.fetchErrorTitle')"
      :description="$t('chatbot.fetchErrorDescription')"
      :retry-label="$t('common.retry')"
      class="flex-1"
      @retry="retryFetch"
    />

    <!-- Content -->
    <div v-else class="flex-1 scroll-y p-4">
      <div class="vstack gap-6">
        <!-- Stats -->
        <div class="row">
          <template v-if="isLoading">
            <div v-for="i in 4" :key="i" class="col-3">
              <article class="card" aria-busy="true">
                <div role="status" class="skeleton line"></div>
                <div role="status" class="skeleton box"></div>
              </article>
            </div>
          </template>
          <template v-else>
            <div v-for="card in statCards" :key="card.key" class="col-3">
              <article class="card">
                <header class="hstack justify-between">
                  <small class="text-light">{{ card.title }}</small>
                  <component :is="card.icon" :size="20" />
                </header>
                <h2>{{ stats[card.key as keyof Stats].toLocaleString() }}</h2>
              </article>
            </div>
          </template>
        </div>

        <!-- Quick Actions -->
        <div class="row">
          <div class="col-4">
            <RouterLink to="/chatbot/keywords" class="unstyled">
              <article class="card">
                <header class="hstack gap-2">
                  <Key :size="20" />
                  <div>
                    <h3>{{ $t('chatbot.keywordRules') }}</h3>
                    <small class="text-light">{{ $t('chatbot.rulesConfigured', { count: stats.keywords_count }) }}</small>
                  </div>
                </header>
              </article>
            </RouterLink>
          </div>

          <div class="col-4">
            <RouterLink to="/chatbot/flows" class="unstyled">
              <article class="card">
                <header class="hstack gap-2">
                  <Workflow :size="20" />
                  <div>
                    <h3>{{ $t('chatbot.conversationFlows') }}</h3>
                    <small class="text-light">{{ $t('chatbot.flowsCreated', { count: stats.flows_count }) }}</small>
                  </div>
                </header>
              </article>
            </RouterLink>
          </div>

          <div class="col-4">
            <RouterLink to="/chatbot/ai" class="unstyled">
              <article class="card">
                <header class="hstack gap-2">
                  <Sparkles :size="20" />
                  <div>
                    <h3>{{ $t('chatbot.aiContexts') }}</h3>
                    <small class="text-light">{{ $t('chatbot.contextsActive', { count: stats.ai_contexts_count }) }}</small>
                  </div>
                </header>
              </article>
            </RouterLink>
          </div>
        </div>

        <!-- Current Settings -->
        <article class="card">
          <header class="hstack justify-between flex-wrap gap-2">
            <h3>{{ $t('chatbot.currentConfiguration') }}</h3>
            <RouterLink to="/settings/chatbot" class="button outline small">
              <Settings :size="16" />
              {{ $t('chatbot.editSettings') }}
            </RouterLink>
          </header>
          <div class="row">
            <div class="col-6 card p-4 vstack gap-1">
              <small class="text-light">{{ $t('chatbot.greetingMessage') }}</small>
              <p v-if="settings.greeting_message" class="pre-wrap">{{ settings.greeting_message }}</p>
              <em v-else class="text-lighter">{{ $t('chatbot.notConfigured') }}</em>
            </div>
            <div class="col-6 card p-4 vstack gap-1">
              <small class="text-light">{{ $t('chatbot.fallbackMessage') }}</small>
              <p v-if="settings.fallback_message" class="pre-wrap">{{ settings.fallback_message }}</p>
              <em v-else class="text-lighter">{{ $t('chatbot.notConfigured') }}</em>
            </div>
            <div class="col-6 card p-4 vstack gap-1">
              <small class="text-light">{{ $t('chatbot.sessionTimeout') }}</small>
              <strong class="hstack gap-2">
                <Clock :size="16" />
                {{ $t('chatbot.minutes', { count: settings.session_timeout_minutes }) }}
              </strong>
            </div>
            <div class="col-6 card p-4 vstack gap-1">
              <small class="text-light">{{ $t('chatbot.aiProvider') }}</small>
              <div class="hstack gap-2">
                <span v-if="settings.ai_enabled" class="badge" data-variant="success">
                  {{ settings.ai_provider || $t('chatbot.notConfigured') }}
                </span>
                <span v-else class="badge" data-variant="secondary">{{ $t('chatbot.disabled') }}</span>
              </div>
            </div>
          </div>
        </article>
      </div>
    </div>
  </div>
</template>
