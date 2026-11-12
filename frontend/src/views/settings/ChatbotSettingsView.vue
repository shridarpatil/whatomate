<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { PageHeader, AuditLogPanel } from '@/components/shared'
import Combobox from '@/components/oat/Combobox.vue'
import MessageButtonsEditor from '@/components/shared/MessageButtonsEditor.vue'
import type { ButtonConfig } from '@/types/flow-preview'
import { validateWhatsAppButtons } from '@/lib/whatsappButtons'
import { toast } from '@/lib/toast'
import { Bot, Brain, X, Clock, AlertTriangle, MessageSquare, Users } from '@/lib/icons'
import { chatbotService } from '@/services/api'
import { useUsersStore } from '@/stores/users'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const usersStore = useUsersStore()
const authStore = useAuthStore()

// The active org may be overridden by the X-Organization-ID header
// (localStorage.selected_organization_id) when a super admin switches orgs.
// That override is what the backend uses for scoping, so we must read it here
// too — otherwise the activity log panel would query the user's default org.
const orgID = computed(
  () => localStorage.getItem('selected_organization_id') || authStore.organizationId,
)

// Bump these keys to force the AuditLogPanel to remount and refetch after
// a save. The backend writes audit entries asynchronously in a goroutine,
// so we delay the remount slightly to give the write time to land.
const messagesLogKey = ref(0)
const agentsLogKey = ref(0)
const hoursLogKey = ref(0)
const slaLogKey = ref(0)
const aiLogKey = ref(0)

function refreshActivityLog(key: typeof messagesLogKey) {
  setTimeout(() => { key.value++ }, 500)
}

const isSubmitting = ref(false)
const isLoading = ref(true)

// Chatbot Settings
interface BusinessHour {
  day: number
  enabled: boolean
  start_time: string
  end_time: string
}

const daysOfWeek = computed(() => [
  t('chatbotSettings.sunday'),
  t('chatbotSettings.monday'),
  t('chatbotSettings.tuesday'),
  t('chatbotSettings.wednesday'),
  t('chatbotSettings.thursday'),
  t('chatbotSettings.friday'),
  t('chatbotSettings.saturday')
])

const defaultBusinessHours: BusinessHour[] = [
  { day: 0, enabled: false, start_time: '09:00', end_time: '17:00' },
  { day: 1, enabled: true, start_time: '09:00', end_time: '17:00' },
  { day: 2, enabled: true, start_time: '09:00', end_time: '17:00' },
  { day: 3, enabled: true, start_time: '09:00', end_time: '17:00' },
  { day: 4, enabled: true, start_time: '09:00', end_time: '17:00' },
  { day: 5, enabled: true, start_time: '09:00', end_time: '17:00' },
  { day: 6, enabled: false, start_time: '09:00', end_time: '17:00' },
]

const chatbotSettings = ref({
  greeting_message: '',
  greeting_buttons: [] as ButtonConfig[],
  fallback_message: '',
  fallback_buttons: [] as ButtonConfig[],
  session_timeout_minutes: 30,
  business_hours_enabled: false,
  business_hours: [...defaultBusinessHours] as BusinessHour[],
  out_of_hours_message: '',
  allow_automated_outside_hours: true,
  allow_agent_queue_pickup: true,
  assign_to_same_agent: true,
  agent_current_conversation_only: false
})

// AI Settings
const aiSettings = ref({
  ai_enabled: false,
  ai_provider: '',
  ai_api_key: '',
  ai_model: '',
  ai_max_tokens: 500,
  ai_system_prompt: ''
})

const isAIEnabled = ref(false)

const aiProviders = [
  { value: 'openai', label: 'OpenAI', models: ['gpt-4o', 'gpt-4o-mini', 'gpt-4-turbo', 'gpt-3.5-turbo'] },
  { value: 'anthropic', label: 'Anthropic', models: ['claude-3-5-sonnet-latest', 'claude-3-5-haiku-latest', 'claude-3-opus-latest'] },
  { value: 'google', label: 'Google AI', models: ['gemini-2.0-flash', 'gemini-2.0-flash-lite', 'gemini-1.5-flash', 'gemini-1.5-flash-8b'] }
]

const availableModels = computed(() => {
  const provider = aiProviders.find(p => p.value === aiSettings.value.ai_provider)
  return provider?.models || []
})

watch(isAIEnabled, (newValue) => {
  aiSettings.value.ai_enabled = newValue
})

// SLA Settings
const slaSettings = ref({
  sla_enabled: false,
  sla_response_minutes: 15,
  sla_resolution_minutes: 60,
  sla_escalation_minutes: 30,
  sla_auto_close_hours: 24,
  sla_auto_close_message: '',
  sla_warning_message: '',
  sla_escalation_notify_ids: [] as string[],
  // Client inactivity settings
  client_reminder_enabled: false,
  client_reminder_minutes: 30,
  client_reminder_message: '',
  client_auto_close_minutes: 60,
  client_auto_close_message: ''
})

const isClientReminderEnabled = ref(false)

watch(isClientReminderEnabled, (newValue) => {
  slaSettings.value.client_reminder_enabled = newValue
})

const isSLAEnabled = ref(false)
const availableUsers = ref<{ id: string; full_name: string }[]>([])

const selectedEscalationUsers = computed(() => {
  return availableUsers.value.filter(u =>
    slaSettings.value.sla_escalation_notify_ids.includes(u.id)
  )
})

const unselectedUsers = computed(() => {
  return availableUsers.value.filter(u =>
    !slaSettings.value.sla_escalation_notify_ids.includes(u.id)
  )
})

const unselectedUserOptions = computed(() =>
  unselectedUsers.value.map(u => ({ value: u.id, label: u.full_name }))
)

watch(isSLAEnabled, (newValue) => {
  slaSettings.value.sla_enabled = newValue
})

onMounted(async () => {
  try {
    const [chatbotResponse] = await Promise.all([
      chatbotService.getSettings(),
      usersStore.fetchUsers()
    ])

    // Users for escalation notify
    availableUsers.value = usersStore.users
      .filter((u) => u.is_active !== false)
      .map((u) => ({ id: u.id, full_name: u.full_name }))

    // Chatbot settings
    const chatbotData = chatbotResponse.data.data || chatbotResponse.data
    if (chatbotData.settings) {
      const loadedHours = chatbotData.settings.business_hours || []
      const mergedHours = defaultBusinessHours.map(defaultDay => {
        const loaded = loadedHours.find((h: BusinessHour) => h.day === defaultDay.day)
        return loaded || defaultDay
      })

      chatbotSettings.value = {
        greeting_message: chatbotData.settings.greeting_message || '',
        greeting_buttons: chatbotData.settings.greeting_buttons || [],
        fallback_message: chatbotData.settings.fallback_message || '',
        fallback_buttons: chatbotData.settings.fallback_buttons || [],
        session_timeout_minutes: chatbotData.settings.session_timeout_minutes || 30,
        business_hours_enabled: chatbotData.settings.business_hours_enabled || false,
        business_hours: mergedHours,
        out_of_hours_message: chatbotData.settings.out_of_hours_message || '',
        allow_automated_outside_hours: chatbotData.settings.allow_automated_outside_hours !== false,
        allow_agent_queue_pickup: chatbotData.settings.allow_agent_queue_pickup !== false,
        assign_to_same_agent: chatbotData.settings.assign_to_same_agent !== false,
        agent_current_conversation_only: chatbotData.settings.agent_current_conversation_only === true
      }

      const aiEnabledValue = chatbotData.settings.ai_enabled === true
      isAIEnabled.value = aiEnabledValue
      aiSettings.value = {
        ai_enabled: aiEnabledValue,
        ai_provider: chatbotData.settings.ai_provider || '',
        ai_api_key: '',
        ai_model: chatbotData.settings.ai_model || '',
        ai_max_tokens: chatbotData.settings.ai_max_tokens || 500,
        ai_system_prompt: chatbotData.settings.ai_system_prompt || ''
      }

      const slaEnabledValue = chatbotData.settings.sla_enabled === true
      isSLAEnabled.value = slaEnabledValue
      const clientReminderEnabledValue = chatbotData.settings.client_reminder_enabled === true
      isClientReminderEnabled.value = clientReminderEnabledValue
      slaSettings.value = {
        sla_enabled: slaEnabledValue,
        sla_response_minutes: chatbotData.settings.sla_response_minutes || 15,
        sla_resolution_minutes: chatbotData.settings.sla_resolution_minutes || 60,
        sla_escalation_minutes: chatbotData.settings.sla_escalation_minutes || 30,
        sla_auto_close_hours: chatbotData.settings.sla_auto_close_hours || 24,
        sla_auto_close_message: chatbotData.settings.sla_auto_close_message || '',
        sla_warning_message: chatbotData.settings.sla_warning_message || '',
        sla_escalation_notify_ids: chatbotData.settings.sla_escalation_notify_ids || [],
        client_reminder_enabled: clientReminderEnabledValue,
        client_reminder_minutes: chatbotData.settings.client_reminder_minutes || 30,
        client_reminder_message: chatbotData.settings.client_reminder_message || '',
        client_auto_close_minutes: chatbotData.settings.client_auto_close_minutes || 60,
        client_auto_close_message: chatbotData.settings.client_auto_close_message || ''
      }
    }
  } catch (error) {
    console.error('Failed to load settings:', error)
  } finally {
    isLoading.value = false
  }
})

async function saveMessagesSettings() {
  // An unsendable combination produces no WhatsApp message at all — the send
  // fails server-side with only a log line — so block the save and say why.
  // Previously incomplete buttons were filtered out of the payload instead,
  // which hid the mistake rather than reporting it.
  const buttonError =
    validateWhatsAppButtons(chatbotSettings.value.greeting_buttons, t) ??
    validateWhatsAppButtons(chatbotSettings.value.fallback_buttons, t)
  if (buttonError) {
    toast.error(buttonError)
    return
  }

  isSubmitting.value = true
  try {
    await chatbotService.updateSettings({
      greeting_message: chatbotSettings.value.greeting_message,
      greeting_buttons: chatbotSettings.value.greeting_buttons,
      fallback_message: chatbotSettings.value.fallback_message,
      fallback_buttons: chatbotSettings.value.fallback_buttons,
      session_timeout_minutes: chatbotSettings.value.session_timeout_minutes
    })
    toast.success(t('chatbotSettings.messagesSaved'))
    refreshActivityLog(messagesLogKey)
  } catch (error) {
    toast.error(t('common.failedSave', { resource: t('resources.chatbotSettings') }))
  } finally {
    isSubmitting.value = false
  }
}

async function saveAgentSettings() {
  isSubmitting.value = true
  try {
    await chatbotService.updateSettings({
      allow_agent_queue_pickup: chatbotSettings.value.allow_agent_queue_pickup,
      assign_to_same_agent: chatbotSettings.value.assign_to_same_agent,
      agent_current_conversation_only: chatbotSettings.value.agent_current_conversation_only
    })
    toast.success(t('chatbotSettings.agentSettingsSaved'))
    refreshActivityLog(agentsLogKey)
  } catch (error) {
    toast.error(t('common.failedSave', { resource: t('resources.chatbotSettings') }))
  } finally {
    isSubmitting.value = false
  }
}

async function saveBusinessHoursSettings() {
  isSubmitting.value = true
  try {
    await chatbotService.updateSettings({
      business_hours_enabled: chatbotSettings.value.business_hours_enabled,
      business_hours: chatbotSettings.value.business_hours,
      out_of_hours_message: chatbotSettings.value.out_of_hours_message,
      allow_automated_outside_hours: chatbotSettings.value.allow_automated_outside_hours
    })
    toast.success(t('chatbotSettings.businessHoursSaved'))
    refreshActivityLog(hoursLogKey)
  } catch (error) {
    toast.error(t('common.failedSave', { resource: t('resources.chatbotSettings') }))
  } finally {
    isSubmitting.value = false
  }
}

async function saveAISettings() {
  isSubmitting.value = true
  try {
    const payload: any = {
      ai_enabled: aiSettings.value.ai_enabled,
      ai_provider: aiSettings.value.ai_provider,
      ai_model: aiSettings.value.ai_model,
      ai_max_tokens: aiSettings.value.ai_max_tokens,
      ai_system_prompt: aiSettings.value.ai_system_prompt
    }
    if (aiSettings.value.ai_api_key) {
      payload.ai_api_key = aiSettings.value.ai_api_key
    }
    await chatbotService.updateSettings(payload)
    toast.success(t('chatbotSettings.aiSettingsSaved'))
    aiSettings.value.ai_api_key = ''
    refreshActivityLog(aiLogKey)
  } catch (error) {
    toast.error(t('chatbotSettings.aiSaveFailed'))
  } finally {
    isSubmitting.value = false
  }
}

async function saveSLASettings() {
  isSubmitting.value = true
  try {
    await chatbotService.updateSettings({
      sla_enabled: slaSettings.value.sla_enabled,
      sla_response_minutes: slaSettings.value.sla_response_minutes,
      sla_resolution_minutes: slaSettings.value.sla_resolution_minutes,
      sla_escalation_minutes: slaSettings.value.sla_escalation_minutes,
      sla_auto_close_hours: slaSettings.value.sla_auto_close_hours,
      sla_auto_close_message: slaSettings.value.sla_auto_close_message,
      sla_warning_message: slaSettings.value.sla_warning_message,
      sla_escalation_notify_ids: slaSettings.value.sla_escalation_notify_ids,
      client_reminder_enabled: slaSettings.value.client_reminder_enabled,
      client_reminder_minutes: slaSettings.value.client_reminder_minutes,
      client_reminder_message: slaSettings.value.client_reminder_message,
      client_auto_close_minutes: slaSettings.value.client_auto_close_minutes,
      client_auto_close_message: slaSettings.value.client_auto_close_message
    })
    toast.success(t('chatbotSettings.slaSettingsSaved'))
    refreshActivityLog(slaLogKey)
  } catch (error) {
    toast.error(t('chatbotSettings.slaSaveFailed'))
  } finally {
    isSubmitting.value = false
  }
}

function addEscalationUser(userId: string | null) {
  if (!userId) return
  if (!slaSettings.value.sla_escalation_notify_ids.includes(userId)) {
    slaSettings.value.sla_escalation_notify_ids.push(userId)
  }
}

function removeEscalationUser(userId: string) {
  const index = slaSettings.value.sla_escalation_notify_ids.indexOf(userId)
  if (index !== -1) {
    slaSettings.value.sla_escalation_notify_ids.splice(index, 1)
  }
}
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader :title="$t('chatbotSettings.title')" :description="$t('chatbotSettings.subtitle')" :icon="Bot" />
    <div class="flex-1 scroll-y p-4">
      <div class="container">
        <ot-tabs>
          <div role="tablist">
            <button type="button" role="tab">
              <MessageSquare :size="16" />
              {{ $t('chatbotSettings.messages') }}
            </button>
            <button type="button" role="tab">
              <Users :size="16" />
              {{ $t('chatbotSettings.agents') }}
            </button>
            <button type="button" role="tab">
              <Clock :size="16" />
              {{ $t('chatbotSettings.hours') }}
            </button>
            <button type="button" role="tab">
              <AlertTriangle :size="16" />
              {{ $t('chatbotSettings.sla') }}
            </button>
            <button type="button" role="tab">
              <Brain :size="16" />
              {{ $t('chatbotSettings.ai') }}
            </button>
          </div>

          <!-- Messages Tab -->
          <div role="tabpanel">
            <article class="card">
              <header>
                <h3>{{ $t('chatbotSettings.messagesTitle') }}</h3>
              </header>
              <div data-field>
                <label for="greeting">{{ $t('chatbotSettings.greetingMessage') }}</label>
                <textarea
                  id="greeting"
                  v-model="chatbotSettings.greeting_message"
                  :placeholder="$t('chatbotSettings.greetingPlaceholder') + '...'"
                  rows="2"
                />
                <MessageButtonsEditor
                  :buttons="chatbotSettings.greeting_buttons"
                  :allowed-types="['reply', 'url']"
                  @update:buttons="chatbotSettings.greeting_buttons = $event"
                />
              </div>
              <hr>
              <div data-field>
                <label for="fallback">{{ $t('chatbotSettings.fallbackMessage') }}</label>
                <textarea
                  id="fallback"
                  v-model="chatbotSettings.fallback_message"
                  :placeholder="$t('chatbotSettings.fallbackPlaceholder') + '...'"
                  rows="2"
                />
                <MessageButtonsEditor
                  :buttons="chatbotSettings.fallback_buttons"
                  :allowed-types="['reply', 'url']"
                  @update:buttons="chatbotSettings.fallback_buttons = $event"
                />
              </div>
              <hr>
              <label data-field>
                {{ $t('chatbotSettings.sessionTimeout') }}
                <input id="timeout" v-model.number="chatbotSettings.session_timeout_minutes" type="number" min="5" max="120" />
                <small data-hint>{{ $t('chatbotSettings.sessionTimeoutHint') }}</small>
              </label>
              <div class="hstack justify-end">
                <button type="button" :aria-busy="isSubmitting" :disabled="isSubmitting" @click="saveMessagesSettings">
                  {{ $t('chatbotSettings.saveChanges') }}
                </button>
              </div>
            </article>
            <div v-if="orgID" class="mt-4">
              <AuditLogPanel :key="messagesLogKey" resource-type="settings.chatbot.messages" :resource-id="orgID" />
            </div>
          </div>

          <!-- Agents Tab -->
          <div role="tabpanel">
            <article class="card">
              <header>
                <h3>{{ $t('chatbotSettings.agentSettings') }}</h3>
              </header>
              <label data-field>
                <span class="hstack justify-between">
                  <span>{{ $t('chatbotSettings.allowQueuePickup') }}</span>
                  <input type="checkbox" role="switch" v-model="chatbotSettings.allow_agent_queue_pickup" />
                </span>
                <small data-hint>{{ $t('chatbotSettings.allowQueuePickupDesc') }}</small>
              </label>
              <hr>
              <label data-field>
                <span class="hstack justify-between">
                  <span>{{ $t('chatbotSettings.assignSameAgent') }}</span>
                  <input type="checkbox" role="switch" v-model="chatbotSettings.assign_to_same_agent" />
                </span>
                <small data-hint>{{ $t('chatbotSettings.assignSameAgentDesc') }}</small>
              </label>
              <hr>
              <label data-field>
                <span class="hstack justify-between">
                  <span>{{ $t('chatbotSettings.currentConversationOnly') }}</span>
                  <input type="checkbox" role="switch" v-model="chatbotSettings.agent_current_conversation_only" />
                </span>
                <small data-hint>{{ $t('chatbotSettings.currentConversationOnlyDesc') }}</small>
              </label>
              <div class="hstack justify-end">
                <button type="button" :aria-busy="isSubmitting" :disabled="isSubmitting" @click="saveAgentSettings">
                  {{ $t('chatbotSettings.saveChanges') }}
                </button>
              </div>
            </article>
            <div v-if="orgID" class="mt-4">
              <AuditLogPanel :key="agentsLogKey" resource-type="settings.chatbot.agents" :resource-id="orgID" />
            </div>
          </div>

          <!-- Business Hours Tab -->
          <div role="tabpanel">
            <article class="card">
              <header>
                <h3>{{ $t('chatbotSettings.businessHours') }}</h3>
              </header>
              <label data-field>
                <span class="hstack justify-between">
                  <span>{{ $t('chatbotSettings.enableBusinessHours') }}</span>
                  <input type="checkbox" role="switch" v-model="chatbotSettings.business_hours_enabled" />
                </span>
                <small data-hint>{{ $t('chatbotSettings.enableBusinessHoursDesc') }}</small>
              </label>
              <div v-if="chatbotSettings.business_hours_enabled" class="vstack gap-2">
                <hr>
                <div class="vstack gap-2">
                  <div v-for="hour in chatbotSettings.business_hours" :key="hour.day" class="hstack gap-4">
                    <input type="checkbox" role="switch" v-model="hour.enabled" :aria-label="daysOfWeek[hour.day]" />
                    <strong class="flex-1" :class="{ 'text-light': !hour.enabled }">{{ daysOfWeek[hour.day] }}</strong>
                    <div class="hstack gap-2">
                      <input v-model="hour.start_time" type="time" :aria-label="$t('chatbotSettings.startTime', { day: daysOfWeek[hour.day] })" :disabled="!hour.enabled" />
                      <span class="text-light">{{ $t('chatbotSettings.to') }}</span>
                      <input v-model="hour.end_time" type="time" :aria-label="$t('chatbotSettings.endTime', { day: daysOfWeek[hour.day] })" :disabled="!hour.enabled" />
                    </div>
                  </div>
                </div>
                <hr>
                <label data-field>
                  {{ $t('chatbotSettings.outOfHoursMessage') }}
                  <textarea v-model="chatbotSettings.out_of_hours_message" :placeholder="$t('chatbotSettings.outOfHoursPlaceholder') + '...'" rows="2" />
                </label>
                <label data-field>
                  <span class="hstack justify-between">
                    <span>{{ $t('chatbotSettings.allowAutomatedOutsideHours') }}</span>
                    <input type="checkbox" role="switch" v-model="chatbotSettings.allow_automated_outside_hours" />
                  </span>
                  <small data-hint>{{ $t('chatbotSettings.allowAutomatedOutsideHoursDesc') }}</small>
                </label>
              </div>
              <div class="hstack justify-end">
                <button type="button" :aria-busy="isSubmitting" :disabled="isSubmitting" @click="saveBusinessHoursSettings">
                  {{ $t('chatbotSettings.saveChanges') }}
                </button>
              </div>
            </article>
            <div v-if="orgID" class="mt-4">
              <AuditLogPanel :key="hoursLogKey" resource-type="settings.chatbot.hours" :resource-id="orgID" />
            </div>
          </div>

          <!-- SLA Tab -->
          <div role="tabpanel">
            <article class="card">
              <header>
                <h3>{{ $t('chatbotSettings.slaSettings') }}</h3>
              </header>
              <label data-field>
                <span class="hstack justify-between">
                  <span>{{ $t('chatbotSettings.enableSlaTracking') }}</span>
                  <input type="checkbox" role="switch" v-model="isSLAEnabled" />
                </span>
                <small data-hint>{{ $t('chatbotSettings.enableSlaTrackingDesc') }}</small>
              </label>
              <div v-if="isSLAEnabled" class="vstack gap-2">
                <hr>
                <div class="row">
                  <div class="col-6">
                    <label data-field>
                      {{ $t('chatbotSettings.responseTime') }}
                      <input v-model.number="slaSettings.sla_response_minutes" type="number" min="1" max="1440" />
                      <small data-hint>{{ $t('chatbotSettings.responseTimeHint') }}</small>
                    </label>
                  </div>
                  <div class="col-6">
                    <label data-field>
                      {{ $t('chatbotSettings.escalationTime') }}
                      <input v-model.number="slaSettings.sla_escalation_minutes" type="number" min="1" max="1440" />
                      <small data-hint>{{ $t('chatbotSettings.escalationTimeHint') }}</small>
                    </label>
                  </div>
                </div>
                <div class="row">
                  <div class="col-6">
                    <label data-field>
                      {{ $t('chatbotSettings.resolutionTime') }}
                      <input v-model.number="slaSettings.sla_resolution_minutes" type="number" min="1" max="10080" />
                      <small data-hint>{{ $t('chatbotSettings.resolutionTimeHint') }}</small>
                    </label>
                  </div>
                  <div class="col-6">
                    <label data-field>
                      {{ $t('chatbotSettings.autoCloseHours') }}
                      <input v-model.number="slaSettings.sla_auto_close_hours" type="number" min="1" max="168" />
                      <small data-hint>{{ $t('chatbotSettings.autoCloseHoursHint') }}</small>
                    </label>
                  </div>
                </div>
                <label data-field>
                  {{ $t('chatbotSettings.autoCloseMessage') }}
                  <textarea v-model="slaSettings.sla_auto_close_message" :placeholder="$t('chatbotSettings.autoClosePlaceholder') + '...'" rows="2" />
                </label>
                <label data-field>
                  {{ $t('chatbotSettings.customerWarningMessage') }}
                  <textarea v-model="slaSettings.sla_warning_message" :placeholder="$t('chatbotSettings.customerWarningPlaceholder') + '...'" rows="2" />
                </label>
                <hr>
                <div class="vstack gap-2">
                  <div class="hstack justify-between">
                    <div>
                      <strong>{{ $t('chatbotSettings.escalationNotifyContacts') }}</strong>
                      <p><small class="text-light">{{ $t('chatbotSettings.escalationNotifyHint') }}</small></p>
                    </div>
                    <Combobox
                      :model-value="null"
                      :options="unselectedUserOptions"
                      :placeholder="$t('chatbotSettings.addUser')"
                      :search-placeholder="$t('chatbotSettings.searchUsers') + '...'"
                      :empty-text="$t('chatbotSettings.noUsersFound')"
                      :disabled="unselectedUsers.length === 0"
                      @update:model-value="addEscalationUser"
                    />
                  </div>
                  <div v-if="selectedEscalationUsers.length > 0" class="hstack gap-2 flex-wrap">
                    <span v-for="user in selectedEscalationUsers" :key="user.id" class="badge outline">
                      {{ user.full_name }}
                      <button
                        type="button"
                        class="ghost icon small"
                        :title="$t('common.remove')"
                        :aria-label="`${$t('common.remove')} ${user.full_name}`"
                        @click="removeEscalationUser(user.id)"
                      >
                        <X :size="12" />
                      </button>
                    </span>
                  </div>
                  <p v-else class="text-light">{{ $t('chatbotSettings.noUsersSelected') }}</p>
                </div>
              </div>
              <hr>
              <!-- Client Inactivity Settings (Chatbot Only) -->
              <label data-field>
                <span class="hstack justify-between">
                  <span>{{ $t('chatbotSettings.clientInactivityReminders') }}</span>
                  <input type="checkbox" role="switch" v-model="isClientReminderEnabled" />
                </span>
                <small data-hint>{{ $t('chatbotSettings.clientInactivityRemindersDesc') }}</small>
              </label>
              <div v-if="isClientReminderEnabled" class="vstack gap-2">
                <div class="row">
                  <div class="col-6">
                    <label data-field>
                      {{ $t('chatbotSettings.reminderAfter') }}
                      <input v-model.number="slaSettings.client_reminder_minutes" type="number" min="1" max="1440" />
                      <small data-hint>{{ $t('chatbotSettings.reminderAfterHint') }}</small>
                    </label>
                  </div>
                  <div class="col-6">
                    <label data-field>
                      {{ $t('chatbotSettings.autoCloseAfter') }}
                      <input v-model.number="slaSettings.client_auto_close_minutes" type="number" min="1" max="1440" />
                      <small data-hint>{{ $t('chatbotSettings.autoCloseAfterHint') }}</small>
                    </label>
                  </div>
                </div>
                <label data-field>
                  {{ $t('chatbotSettings.reminderMessage') }}
                  <textarea v-model="slaSettings.client_reminder_message" :placeholder="$t('chatbotSettings.reminderPlaceholder') + '...'" rows="2" />
                </label>
                <label data-field>
                  {{ $t('chatbotSettings.clientAutoCloseMessage') }}
                  <textarea v-model="slaSettings.client_auto_close_message" :placeholder="$t('chatbotSettings.clientAutoClosePlaceholder') + '...'" rows="2" />
                </label>
              </div>
              <div class="hstack justify-end">
                <button type="button" :aria-busy="isSubmitting" :disabled="isSubmitting" @click="saveSLASettings">
                  {{ $t('chatbotSettings.saveChanges') }}
                </button>
              </div>
            </article>
            <div v-if="orgID" class="mt-4">
              <AuditLogPanel :key="slaLogKey" resource-type="settings.chatbot.sla" :resource-id="orgID" />
            </div>
          </div>

          <!-- AI Tab -->
          <div role="tabpanel">
            <article class="card">
              <header>
                <h3>{{ $t('chatbotSettings.aiSettings') }}</h3>
              </header>
              <label data-field>
                <span class="hstack justify-between">
                  <span>{{ $t('chatbotSettings.enableAiResponses') }}</span>
                  <input type="checkbox" role="switch" v-model="isAIEnabled" />
                </span>
                <small data-hint>{{ $t('chatbotSettings.enableAiResponsesDesc') }}</small>
              </label>
              <div v-if="isAIEnabled" class="vstack gap-2">
                <hr>
                <div class="row">
                  <div class="col-6">
                    <label data-field>
                      {{ $t('chatbotSettings.aiProvider') }}
                      <select v-model="aiSettings.ai_provider">
                        <option value="">{{ $t('chatbotSettings.selectProvider') }}...</option>
                        <option v-for="provider in aiProviders" :key="provider.value" :value="provider.value">{{ provider.label }}</option>
                      </select>
                    </label>
                  </div>
                  <div class="col-6">
                    <label data-field>
                      {{ $t('chatbotSettings.model') }}
                      <select v-model="aiSettings.ai_model" :disabled="!aiSettings.ai_provider">
                        <option value="">{{ $t('chatbotSettings.selectModel') }}...</option>
                        <option v-for="model in availableModels" :key="model" :value="model">{{ model }}</option>
                      </select>
                    </label>
                  </div>
                </div>
                <label data-field>
                  {{ $t('chatbotSettings.apiKey') }}
                  <input v-model="aiSettings.ai_api_key" type="password" :placeholder="$t('chatbotSettings.apiKeyPlaceholder') + '...'" />
                  <small data-hint>{{ $t('chatbotSettings.apiKeyHint') }}</small>
                </label>
                <label data-field>
                  {{ $t('chatbotSettings.maxTokens') }}
                  <input v-model.number="aiSettings.ai_max_tokens" type="number" min="100" max="4000" />
                </label>
                <label data-field>
                  {{ $t('chatbotSettings.systemPrompt') }}
                  <textarea v-model="aiSettings.ai_system_prompt" :placeholder="$t('chatbotSettings.systemPromptPlaceholder') + '...'" rows="3" />
                </label>
              </div>
              <div class="hstack justify-end">
                <button type="button" :aria-busy="isSubmitting" :disabled="isSubmitting" @click="saveAISettings">
                  {{ $t('chatbotSettings.saveChanges') }}
                </button>
              </div>
            </article>
            <div v-if="orgID" class="mt-4">
              <AuditLogPanel :key="aiLogKey" resource-type="settings.chatbot.ai" :resource-id="orgID" />
            </div>
          </div>
        </ot-tabs>
      </div>
    </div>
  </div>
</template>
