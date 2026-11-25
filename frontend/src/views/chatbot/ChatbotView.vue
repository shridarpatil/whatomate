<script setup lang="ts">
import ChatbotTabs from '@/components/layout/ChatbotTabs.vue'
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { chatbotService } from '@/services/api'
import { toast } from '@/lib/toast'
import { PageHeader, ConfirmDialog, ErrorState } from '@/components/shared'
import { getErrorMessage } from '@/lib/api-utils'
import { useAuthStore } from '@/stores/auth'
import {
  Bot,
  Sparkles,
  Power,
  Settings,
  TrendingUp,
  Users,
  MessageSquare
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

const authStore = useAuthStore()
// Turning the chatbot on/off and editing its settings need settings.chatbot write.
const canManageChatbot = authStore.hasPermission('settings.chatbot', 'write')

// Checklist steps link to other chatbot pages; only show the ones this user can open.
const allSteps = computed(() => [
  {
    key: 'greeting', done: !!settings.value.greeting_message, to: '/settings/chatbot',
    title: t('chatbot.stepGreeting'), detail: t('chatbot.stepGreetingDetail'), link: t('chatbot.addGreeting'),
  },
  {
    key: 'keywords', done: stats.value.keywords_count > 0, to: '/chatbot/keywords',
    title: t('chatbot.keywordRules'), detail: t('chatbot.rulesConfigured', stats.value.keywords_count), link: t('chatbot.openKeywordRules'),
  },
  {
    key: 'flows', done: stats.value.flows_count > 0, to: '/chatbot/flows',
    title: t('chatbot.conversationFlows'), detail: t('chatbot.flowsCreated', stats.value.flows_count), link: t('chatbot.openBotFlows'),
  },
  {
    key: 'ai', done: stats.value.ai_contexts_count > 0, to: '/chatbot/ai',
    title: t('chatbot.aiContexts'), detail: t('chatbot.contextsActive', stats.value.ai_contexts_count), link: t('chatbot.openAiContexts'),
  },
])
const STEP_PERMISSION: Record<string, string> = {
  greeting: 'settings.chatbot', keywords: 'chatbot.keywords', flows: 'flows.chatbot', ai: 'chatbot.ai',
}
const steps = computed(() => allSteps.value.filter(s => authStore.hasPermission(STEP_PERMISSION[s.key], 'read')))
const stepsDone = computed(() => steps.value.filter(s => s.done).length)

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
          <span v-if="settings.enabled" class="badge" data-status="success">{{ $t('chatbot.active') }}</span>
          <!-- While off, the banner below carries the "turn on" action. -->
          <button
            v-if="settings.enabled && canManageChatbot"
            type="button"
            class="outline small"
            data-variant="danger"
            :disabled="isToggling"
            @click="showToggleConfirm = true"
          >
            <Power :size="16" />
            {{ $t('chatbot.disable') }}
          </button>
        </div>
      </template>
    </PageHeader>
    <ChatbotTabs />

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
        <!-- The chatbot being off is the most important fact on this page. -->
        <div v-if="!isLoading && !settings.enabled" role="alert" data-variant="warning" class="hstack justify-between gap-4">
          <div><strong>{{ $t('chatbot.offTitle') }}</strong> {{ $t('chatbot.offDescription') }}</div>
          <button v-if="canManageChatbot" type="button" class="small" :disabled="isToggling" @click="showToggleConfirm = true">
            <Power :size="16" />
            {{ $t('chatbot.turnOn') }}
          </button>
        </div>
        <!-- Stats -->
        <div class="stat-grid">
          <template v-if="isLoading">
            <article v-for="i in 4" :key="i" class="card vstack" data-widget="number" aria-busy="true">
              <div role="status" class="skeleton line" data-size="label"></div>
              <div role="status" class="skeleton line" data-size="value"></div>
            </article>
          </template>
          <template v-else>
            <article v-for="card in statCards" :key="card.key" class="card vstack" data-widget="number">
              <header><h4>{{ card.title }}</h4></header>
              <p data-widget-value>{{ stats[card.key as keyof Stats].toLocaleString() }}</p>
            </article>
          </template>
        </div>

        <div class="row">
          <!-- What the chatbot does today; unset options say what happens and link to the fix. -->
          <article class="card vstack gap-4 col-7">
            <header class="hstack justify-between flex-wrap gap-2">
              <h3>{{ $t('chatbot.howItReplies') }}</h3>
              <RouterLink v-if="canManageChatbot" to="/settings/chatbot" class="button outline small">
                <Settings :size="16" />
                {{ $t('chatbot.editSettings') }}
              </RouterLink>
            </header>
            <dl data-facts>
              <dt>{{ $t('chatbot.greetingMessage') }}</dt>
              <dd>
                <span v-if="settings.greeting_message" class="pre-wrap">{{ settings.greeting_message }}</span>
                <template v-else><span class="text-light">{{ $t('chatbot.greetingUnset') }}</span> <RouterLink v-if="canManageChatbot" to="/settings/chatbot">{{ $t('chatbot.addGreeting') }}</RouterLink></template>
              </dd>
              <dt>{{ $t('chatbot.fallbackMessage') }}</dt>
              <dd>
                <span v-if="settings.fallback_message" class="pre-wrap">{{ settings.fallback_message }}</span>
                <template v-else><span class="text-light">{{ $t('chatbot.fallbackUnset') }}</span> <RouterLink v-if="canManageChatbot" to="/settings/chatbot">{{ $t('chatbot.addFallback') }}</RouterLink></template>
              </dd>
              <dt>{{ $t('chatbot.sessionTimeout') }}</dt>
              <dd>{{ $t('chatbot.sessionTimeoutValue', { count: settings.session_timeout_minutes }) }}</dd>
              <dt>{{ $t('chatbot.aiProvider') }}</dt>
              <dd>
                <span v-if="settings.ai_enabled" class="badge" data-status="success">{{ settings.ai_provider || $t('chatbot.notConfigured') }}</span>
                <template v-else><span class="badge" data-status="neutral">{{ $t('chatbot.disabled') }}</span> <RouterLink v-if="canManageChatbot" to="/settings/chatbot">{{ $t('chatbot.connectAi') }}</RouterLink></template>
              </dd>
            </dl>
          </article>

          <!-- Set-up checklist built from what already exists. -->
          <article class="card vstack gap-4 col-5">
            <header class="hstack justify-between">
              <h3>{{ $t('chatbot.getStarted') }}</h3>
              <small class="text-light">{{ $t('chatbot.stepsDone', { done: stepsDone, total: steps.length }) }}</small>
            </header>
            <ol data-steps>
              <li v-for="step in steps" :key="step.key" :data-done="step.done || undefined">
                <strong>{{ step.title }}</strong>
                <small class="text-light">{{ step.detail }}</small>
                <RouterLink v-if="!step.done || step.key !== 'greeting'" :to="step.to">{{ step.link }}</RouterLink>
              </li>
            </ol>
          </article>
        </div>
      </div>
    </div>
  </div>
</template>
