<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { PageHeader } from '@/components/shared'
import { toast } from 'vue-sonner'
import { Bot, Loader2, Brain, Plus, X, Clock, AlertTriangle, UserPlus, MessageSquare, Users } from 'lucide-vue-next'
import { usersService, chatbotService } from '@/services/api'

const { t } = useI18n()

const isSubmitting = ref(false)
const isLoading = ref(true)

// Chatbot Settings
interface MessageButton {
  id: string
  title: string
}

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
  greeting_buttons: [] as MessageButton[],
  fallback_message: '',
  fallback_buttons: [] as MessageButton[],
  session_timeout_minutes: 30,
  business_hours_enabled: false,
  business_hours: [...defaultBusinessHours] as BusinessHour[],
  out_of_hours_message: '',
  allow_automated_outside_hours: true,
  allow_agent_queue_pickup: true,
  assign_to_same_agent: true,
  agent_current_conversation_only: false
})

// Button management functions
const addGreetingButton = () => {
  if (chatbotSettings.value.greeting_buttons.length >= 10) {
    toast.error(t('chatbotSettings.maxButtonsError'))
    return
  }
  const id = `btn_${Date.now()}`
  chatbotSettings.value.greeting_buttons.push({ id, title: '' })
}

const removeGreetingButton = (index: number) => {
  chatbotSettings.value.greeting_buttons.splice(index, 1)
}

const addFallbackButton = () => {
  if (chatbotSettings.value.fallback_buttons.length >= 10) {
    toast.error(t('chatbotSettings.maxButtonsError'))
    return
  }
  const id = `btn_${Date.now()}`
  chatbotSettings.value.fallback_buttons.push({ id, title: '' })
}

const removeFallbackButton = (index: number) => {
  chatbotSettings.value.fallback_buttons.splice(index, 1)
}

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
const escalationComboboxOpen = ref(false)

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

watch(isSLAEnabled, (newValue) => {
  slaSettings.value.sla_enabled = newValue
})

onMounted(async () => {
  try {
    const [chatbotResponse, usersResponse] = await Promise.all([
      chatbotService.getSettings(),
      usersService.list()
    ])

    // Users for escalation notify
    const usersData = usersResponse.data.data || usersResponse.data
    const usersList = usersData.users || usersData || []
    availableUsers.value = usersList.filter((u: any) => u.is_active !== false).map((u: any) => ({
      id: u.id,
      full_name: u.full_name
    }))

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
  const invalidGreetingBtn = chatbotSettings.value.greeting_buttons.find(btn => !btn.title.trim())
  if (invalidGreetingBtn) {
    toast.error(t('chatbotSettings.greetingButtonsRequired'))
    return
  }
  const invalidFallbackBtn = chatbotSettings.value.fallback_buttons.find(btn => !btn.title.trim())
  if (invalidFallbackBtn) {
    toast.error(t('chatbotSettings.fallbackButtonsRequired'))
    return
  }

  isSubmitting.value = true
  try {
    await chatbotService.updateSettings({
      greeting_message: chatbotSettings.value.greeting_message,
      greeting_buttons: chatbotSettings.value.greeting_buttons.filter(btn => btn.title.trim()),
      fallback_message: chatbotSettings.value.fallback_message,
      fallback_buttons: chatbotSettings.value.fallback_buttons.filter(btn => btn.title.trim()),
      session_timeout_minutes: chatbotSettings.value.session_timeout_minutes
    })
    toast.success(t('chatbotSettings.messagesSaved'))
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
  } catch (error) {
    toast.error(t('chatbotSettings.slaSaveFailed'))
  } finally {
    isSubmitting.value = false
  }
}

function addEscalationUser(userId: string) {
  if (!slaSettings.value.sla_escalation_notify_ids.includes(userId)) {
    slaSettings.value.sla_escalation_notify_ids.push(userId)
  }
  escalationComboboxOpen.value = false
}

function removeEscalationUser(userId: string) {
  const index = slaSettings.value.sla_escalation_notify_ids.indexOf(userId)
  if (index !== -1) {
    slaSettings.value.sla_escalation_notify_ids.splice(index, 1)
  }
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('chatbotSettings.title')" :subtitle="$t('chatbotSettings.subtitle')" :icon="Bot" icon-gradient="bg-gradient-to-br from-purple-500 to-pink-600 shadow-purple-500/20" />
    <ScrollArea class="flex-1">
      <div class="p-6 space-y-4 max-w-4xl mx-auto">
        <Tabs default-value="messages" class="w-full">
          <TabsList class="grid w-full grid-cols-5 mb-6">
            <TabsTrigger value="messages">
              <MessageSquare class="h-4 w-4 mr-2" />
              {{ $t('chatbotSettings.messages') }}
            </TabsTrigger>
            <TabsTrigger value="agents">
              <Users class="h-4 w-4 mr-2" />
              {{ $t('chatbotSettings.agents') }}
            </TabsTrigger>
            <TabsTrigger value="hours">
              <Clock class="h-4 w-4 mr-2" />
              {{ $t('chatbotSettings.hours') }}
            </TabsTrigger>
            <TabsTrigger value="sla">
              <AlertTriangle class="h-4 w-4 mr-2" />
              {{ $t('chatbotSettings.sla') }}
            </TabsTrigger>
            <TabsTrigger value="ai">
              <Brain class="h-4 w-4 mr-2" />
              {{ $t('chatbotSettings.ai') }}
            </TabsTrigger>
          </TabsList>

          <!-- Messages Tab -->
          <TabsContent value="messages">
            <Card>
              <CardHeader>
                <CardTitle>{{ $t('chatbotSettings.messagesTitle') }}</CardTitle>
                <CardDescription>{{ $t('chatbotSettings.messagesDesc') }}</CardDescription>
              </CardHeader>
              <CardContent class="space-y-4">
                <div class="space-y-2">
                  <Label for="greeting">{{ $t('chatbotSettings.greetingMessage') }}</Label>
                  <Textarea
                    id="greeting"
                    v-model="chatbotSettings.greeting_message"
                    :placeholder="$t('chatbotSettings.greetingPlaceholder') + '...'"
                    :rows="2"
                  />
                  <div class="mt-2">
                    <div class="flex items-center justify-between mb-2">
                      <Label class="text-sm text-muted-foreground">{{ $t('chatbotSettings.quickReplyButtons') }}</Label>
                      <Button
                        variant="outline"
                        size="sm"
                        @click="addGreetingButton"
                        :disabled="chatbotSettings.greeting_buttons.length >= 10"
                      >
                        <Plus class="h-4 w-4 mr-1" />
                        {{ $t('chatbotSettings.addButton') }}
                      </Button>
                    </div>
                    <div v-if="chatbotSettings.greeting_buttons.length > 0" class="space-y-2">
                      <div
                        v-for="(button, index) in chatbotSettings.greeting_buttons"
                        :key="button.id"
                        class="flex items-center gap-2"
                      >
                        <Input
                          v-model="button.title"
                          :placeholder="$t('chatbotSettings.buttonPlaceholder') + '...'"
                          maxlength="20"
                          class="flex-1"
                        />
                        <Button variant="ghost" size="icon" @click="removeGreetingButton(index)">
                          <X class="h-4 w-4" />
                        </Button>
                      </div>
                      <p class="text-xs text-muted-foreground">{{ $t('chatbotSettings.buttonHint') }}</p>
                    </div>
                  </div>
                </div>

                <Separator />

                <div class="space-y-2">
                  <Label for="fallback">{{ $t('chatbotSettings.fallbackMessage') }}</Label>
                  <Textarea
                    id="fallback"
                    v-model="chatbotSettings.fallback_message"
                    :placeholder="$t('chatbotSettings.fallbackPlaceholder') + '...'"
                    :rows="2"
                  />
                  <div class="mt-2">
                    <div class="flex items-center justify-between mb-2">
                      <Label class="text-sm text-muted-foreground">{{ $t('chatbotSettings.quickReplyButtons') }}</Label>
                      <Button
                        variant="outline"
                        size="sm"
                        @click="addFallbackButton"
                        :disabled="chatbotSettings.fallback_buttons.length >= 10"
                      >
                        <Plus class="h-4 w-4 mr-1" />
                        {{ $t('chatbotSettings.addButton') }}
                      </Button>
                    </div>
                    <div v-if="chatbotSettings.fallback_buttons.length > 0" class="space-y-2">
                      <div
                        v-for="(button, index) in chatbotSettings.fallback_buttons"
                        :key="button.id"
                        class="flex items-center gap-2"
                      >
                        <Input
                          v-model="button.title"
                          :placeholder="$t('chatbotSettings.buttonPlaceholder') + '...'"
                          maxlength="20"
                          class="flex-1"
                        />
                        <Button variant="ghost" size="icon" @click="removeFallbackButton(index)">
                          <X class="h-4 w-4" />
                        </Button>
                      </div>
                      <p class="text-xs text-muted-foreground">{{ $t('chatbotSettings.buttonHint') }}</p>
                    </div>
                  </div>
                </div>

                <Separator />

                <div class="space-y-2">
                  <Label for="timeout">{{ $t('chatbotSettings.sessionTimeout') }}</Label>
                  <Input
                    id="timeout"
                    v-model.number="chatbotSettings.session_timeout_minutes"
                    type="number"
                    min="5"
                    max="120"
                    class="w-32"
                  />
                  <p class="text-xs text-muted-foreground">{{ $t('chatbotSettings.sessionTimeoutHint') }}</p>
                </div>

                <div class="flex justify-end pt-2">
                  <Button @click="saveMessagesSettings" :disabled="isSubmitting">
                    <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                    {{ $t('chatbotSettings.saveChanges') }}
                  </Button>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <!-- Agents Tab -->
          <TabsContent value="agents">
            <Card>
              <CardHeader>
                <CardTitle>{{ $t('chatbotSettings.agentSettings') }}</CardTitle>
                <CardDescription>{{ $t('chatbotSettings.agentSettingsDesc') }}</CardDescription>
              </CardHeader>
              <CardContent class="space-y-4">
                <div class="flex items-center justify-between py-2">
                  <div>
                    <p class="font-medium">{{ $t('chatbotSettings.allowQueuePickup') }}</p>
                    <p class="text-sm text-muted-foreground">{{ $t('chatbotSettings.allowQueuePickupDesc') }}</p>
                  </div>
                  <Switch
                    :checked="chatbotSettings.allow_agent_queue_pickup"
                    @update:checked="chatbotSettings.allow_agent_queue_pickup = $event"
                  />
                </div>

                <Separator />

                <div class="flex items-center justify-between py-2">
                  <div>
                    <p class="font-medium">{{ $t('chatbotSettings.assignSameAgent') }}</p>
                    <p class="text-sm text-muted-foreground">{{ $t('chatbotSettings.assignSameAgentDesc') }}</p>
                  </div>
                  <Switch
                    :checked="chatbotSettings.assign_to_same_agent"
                    @update:checked="chatbotSettings.assign_to_same_agent = $event"
                  />
                </div>

                <Separator />

                <div class="flex items-center justify-between py-2">
                  <div>
                    <p class="font-medium">{{ $t('chatbotSettings.currentConversationOnly') }}</p>
                    <p class="text-sm text-muted-foreground">{{ $t('chatbotSettings.currentConversationOnlyDesc') }}</p>
                  </div>
                  <Switch
                    :checked="chatbotSettings.agent_current_conversation_only"
                    @update:checked="chatbotSettings.agent_current_conversation_only = $event"
                  />
                </div>

                <div class="flex justify-end pt-4">
                  <Button @click="saveAgentSettings" :disabled="isSubmitting">
                    <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                    {{ $t('chatbotSettings.saveChanges') }}
                  </Button>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <!-- Business Hours Tab -->
          <TabsContent value="hours">
            <Card>
              <CardHeader>
                <CardTitle>{{ $t('chatbotSettings.businessHours') }}</CardTitle>
                <CardDescription>{{ $t('chatbotSettings.businessHoursDesc') }}</CardDescription>
              </CardHeader>
              <CardContent class="space-y-4">
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium">{{ $t('chatbotSettings.enableBusinessHours') }}</p>
                    <p class="text-sm text-muted-foreground">{{ $t('chatbotSettings.enableBusinessHoursDesc') }}</p>
                  </div>
                  <Switch
                    :checked="chatbotSettings.business_hours_enabled"
                    @update:checked="chatbotSettings.business_hours_enabled = $event"
                  />
                </div>

                <div v-if="chatbotSettings.business_hours_enabled" class="space-y-4 pt-2">
                  <Separator />

                  <div class="border rounded-lg p-4 space-y-3">
                    <div
                      v-for="hour in chatbotSettings.business_hours"
                      :key="hour.day"
                      class="flex items-center gap-4"
                    >
                      <div class="w-20">
                        <Switch
                          :checked="hour.enabled"
                          @update:checked="hour.enabled = $event"
                        />
                      </div>
                      <span class="w-24 font-medium" :class="{ 'text-muted-foreground': !hour.enabled }">
                        {{ daysOfWeek[hour.day] }}
                      </span>
                      <div class="flex items-center gap-2" :class="{ 'opacity-50': !hour.enabled }">
                        <Input
                          v-model="hour.start_time"
                          type="time"
                          class="w-28"
                          :disabled="!hour.enabled"
                        />
                        <span class="text-muted-foreground">{{ $t('chatbotSettings.to') }}</span>
                        <Input
                          v-model="hour.end_time"
                          type="time"
                          class="w-28"
                          :disabled="!hour.enabled"
                        />
                      </div>
                    </div>
                  </div>

                  <Separator />

                  <div class="space-y-2">
                    <Label>{{ $t('chatbotSettings.outOfHoursMessage') }}</Label>
                    <Textarea
                      v-model="chatbotSettings.out_of_hours_message"
                      :placeholder="$t('chatbotSettings.outOfHoursPlaceholder') + '...'"
                      :rows="2"
                    />
                  </div>

                  <div class="flex items-center justify-between py-2">
                    <div>
                      <p class="font-medium">{{ $t('chatbotSettings.allowAutomatedOutsideHours') }}</p>
                      <p class="text-sm text-muted-foreground">{{ $t('chatbotSettings.allowAutomatedOutsideHoursDesc') }}</p>
                    </div>
                    <Switch
                      :checked="chatbotSettings.allow_automated_outside_hours"
                      @update:checked="chatbotSettings.allow_automated_outside_hours = $event"
                    />
                  </div>
                </div>

                <div class="flex justify-end pt-2">
                  <Button @click="saveBusinessHoursSettings" :disabled="isSubmitting">
                    <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                    {{ $t('chatbotSettings.saveChanges') }}
                  </Button>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <!-- SLA Tab -->
          <TabsContent value="sla">
            <Card>
              <CardHeader>
                <CardTitle>{{ $t('chatbotSettings.slaSettings') }}</CardTitle>
                <CardDescription>{{ $t('chatbotSettings.slaSettingsDesc') }}</CardDescription>
              </CardHeader>
              <CardContent class="space-y-4">
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium">{{ $t('chatbotSettings.enableSlaTracking') }}</p>
                    <p class="text-sm text-muted-foreground">{{ $t('chatbotSettings.enableSlaTrackingDesc') }}</p>
                  </div>
                  <Switch
                    :checked="isSLAEnabled"
                    @update:checked="(val: boolean) => isSLAEnabled = val"
                  />
                </div>

                <div v-if="isSLAEnabled" class="space-y-4 pt-2">
                  <Separator />

                  <div class="grid grid-cols-2 gap-4">
                    <div class="space-y-2">
                      <Label>{{ $t('chatbotSettings.responseTime') }}</Label>
                      <Input v-model.number="slaSettings.sla_response_minutes" type="number" min="1" max="1440" />
                      <p class="text-xs text-muted-foreground">{{ $t('chatbotSettings.responseTimeHint') }}</p>
                    </div>
                    <div class="space-y-2">
                      <Label>{{ $t('chatbotSettings.escalationTime') }}</Label>
                      <Input v-model.number="slaSettings.sla_escalation_minutes" type="number" min="1" max="1440" />
                      <p class="text-xs text-muted-foreground">{{ $t('chatbotSettings.escalationTimeHint') }}</p>
                    </div>
                  </div>

                  <div class="grid grid-cols-2 gap-4">
                    <div class="space-y-2">
                      <Label>{{ $t('chatbotSettings.resolutionTime') }}</Label>
                      <Input v-model.number="slaSettings.sla_resolution_minutes" type="number" min="1" max="10080" />
                      <p class="text-xs text-muted-foreground">{{ $t('chatbotSettings.resolutionTimeHint') }}</p>
                    </div>
                    <div class="space-y-2">
                      <Label>{{ $t('chatbotSettings.autoCloseHours') }}</Label>
                      <Input v-model.number="slaSettings.sla_auto_close_hours" type="number" min="1" max="168" />
                      <p class="text-xs text-muted-foreground">{{ $t('chatbotSettings.autoCloseHoursHint') }}</p>
                    </div>
                  </div>

                  <div class="space-y-2">
                    <Label>{{ $t('chatbotSettings.autoCloseMessage') }}</Label>
                    <Textarea
                      v-model="slaSettings.sla_auto_close_message"
                      :placeholder="$t('chatbotSettings.autoClosePlaceholder') + '...'"
                      :rows="2"
                    />
                  </div>

                  <div class="space-y-2">
                    <Label>{{ $t('chatbotSettings.customerWarningMessage') }}</Label>
                    <Textarea
                      v-model="slaSettings.sla_warning_message"
                      :placeholder="$t('chatbotSettings.customerWarningPlaceholder') + '...'"
                      :rows="2"
                    />
                  </div>

                  <Separator />

                  <div class="space-y-3">
                    <div class="flex items-center justify-between">
                      <div>
                        <Label>{{ $t('chatbotSettings.escalationNotifyContacts') }}</Label>
                        <p class="text-xs text-muted-foreground">{{ $t('chatbotSettings.escalationNotifyHint') }}</p>
                      </div>
                      <Popover v-model:open="escalationComboboxOpen">
                        <PopoverTrigger as-child>
                          <Button variant="outline" size="sm" class="gap-2" :disabled="unselectedUsers.length === 0">
                            <UserPlus class="h-4 w-4" />
                            {{ $t('chatbotSettings.addUser') }}
                          </Button>
                        </PopoverTrigger>
                        <PopoverContent class="w-[250px] p-0" align="end">
                          <Command>
                            <CommandInput :placeholder="$t('chatbotSettings.searchUsers') + '...'" />
                            <CommandList>
                              <CommandEmpty>{{ $t('chatbotSettings.noUsersFound') }}</CommandEmpty>
                              <CommandGroup>
                                <CommandItem
                                  v-for="user in unselectedUsers"
                                  :key="user.id"
                                  :value="user.full_name"
                                  @select="addEscalationUser(user.id)"
                                  class="cursor-pointer"
                                >
                                  {{ user.full_name }}
                                </CommandItem>
                              </CommandGroup>
                            </CommandList>
                          </Command>
                        </PopoverContent>
                      </Popover>
                    </div>

                    <div v-if="selectedEscalationUsers.length > 0" class="flex flex-wrap gap-2">
                      <div
                        v-for="user in selectedEscalationUsers"
                        :key="user.id"
                        class="flex items-center gap-2 px-3 py-1.5 bg-muted rounded-full text-sm"
                      >
                        <span>{{ user.full_name }}</span>
                        <button type="button" @click="removeEscalationUser(user.id)" class="text-muted-foreground hover:text-foreground">
                          <X class="h-3.5 w-3.5" />
                        </button>
                      </div>
                    </div>
                    <p v-else class="text-sm text-muted-foreground italic">{{ $t('chatbotSettings.noUsersSelected') }}</p>
                  </div>
                </div>

                <Separator class="my-6" />

                <!-- Client Inactivity Settings (Chatbot Only) -->
                <div class="space-y-4">
                  <div class="flex items-center justify-between">
                    <div>
                      <p class="font-medium">{{ $t('chatbotSettings.clientInactivityReminders') }}</p>
                      <p class="text-sm text-muted-foreground">{{ $t('chatbotSettings.clientInactivityRemindersDesc') }}</p>
                    </div>
                    <Switch
                      :checked="isClientReminderEnabled"
                      @update:checked="(val: boolean) => isClientReminderEnabled = val"
                    />
                  </div>

                  <div v-if="isClientReminderEnabled" class="space-y-4 pt-2">
                    <div class="grid grid-cols-2 gap-4">
                      <div class="space-y-2">
                        <Label>{{ $t('chatbotSettings.reminderAfter') }}</Label>
                        <Input v-model.number="slaSettings.client_reminder_minutes" type="number" min="1" max="1440" />
                        <p class="text-xs text-muted-foreground">{{ $t('chatbotSettings.reminderAfterHint') }}</p>
                      </div>
                      <div class="space-y-2">
                        <Label>{{ $t('chatbotSettings.autoCloseAfter') }}</Label>
                        <Input v-model.number="slaSettings.client_auto_close_minutes" type="number" min="1" max="1440" />
                        <p class="text-xs text-muted-foreground">{{ $t('chatbotSettings.autoCloseAfterHint') }}</p>
                      </div>
                    </div>

                    <div class="space-y-2">
                      <Label>{{ $t('chatbotSettings.reminderMessage') }}</Label>
                      <Textarea
                        v-model="slaSettings.client_reminder_message"
                        :placeholder="$t('chatbotSettings.reminderPlaceholder') + '...'"
                        :rows="2"
                      />
                    </div>

                    <div class="space-y-2">
                      <Label>{{ $t('chatbotSettings.clientAutoCloseMessage') }}</Label>
                      <Textarea
                        v-model="slaSettings.client_auto_close_message"
                        :placeholder="$t('chatbotSettings.clientAutoClosePlaceholder') + '...'"
                        :rows="2"
                      />
                    </div>
                  </div>
                </div>

                <div class="flex justify-end pt-2">
                  <Button @click="saveSLASettings" :disabled="isSubmitting">
                    <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                    {{ $t('chatbotSettings.saveChanges') }}
                  </Button>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <!-- AI Tab -->
          <TabsContent value="ai">
            <Card>
              <CardHeader>
                <CardTitle>{{ $t('chatbotSettings.aiSettings') }}</CardTitle>
                <CardDescription>{{ $t('chatbotSettings.aiSettingsDesc') }}</CardDescription>
              </CardHeader>
              <CardContent class="space-y-4">
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium">{{ $t('chatbotSettings.enableAiResponses') }}</p>
                    <p class="text-sm text-muted-foreground">{{ $t('chatbotSettings.enableAiResponsesDesc') }}</p>
                  </div>
                  <Switch
                    :checked="isAIEnabled"
                    @update:checked="(val: boolean) => isAIEnabled = val"
                  />
                </div>

                <div v-if="isAIEnabled" class="space-y-4 pt-2">
                  <Separator />

                  <div class="grid grid-cols-2 gap-4">
                    <div class="space-y-2">
                      <Label>{{ $t('chatbotSettings.aiProvider') }}</Label>
                      <Select v-model="aiSettings.ai_provider">
                        <SelectTrigger>
                          <SelectValue :placeholder="$t('chatbotSettings.selectProvider') + '...'" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem v-for="provider in aiProviders" :key="provider.value" :value="provider.value">
                            {{ provider.label }}
                          </SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div class="space-y-2">
                      <Label>{{ $t('chatbotSettings.model') }}</Label>
                      <Select v-model="aiSettings.ai_model" :disabled="!aiSettings.ai_provider">
                        <SelectTrigger>
                          <SelectValue :placeholder="$t('chatbotSettings.selectModel') + '...'" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem v-for="model in availableModels" :key="model" :value="model">
                            {{ model }}
                          </SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                  </div>

                  <div class="space-y-2">
                    <Label>{{ $t('chatbotSettings.apiKey') }}</Label>
                    <Input
                      v-model="aiSettings.ai_api_key"
                      type="password"
                      :placeholder="$t('chatbotSettings.apiKeyPlaceholder') + '...'"
                    />
                    <p class="text-xs text-muted-foreground">{{ $t('chatbotSettings.apiKeyHint') }}</p>
                  </div>

                  <div class="space-y-2">
                    <Label>{{ $t('chatbotSettings.maxTokens') }}</Label>
                    <Input v-model.number="aiSettings.ai_max_tokens" type="number" min="100" max="4000" class="w-32" />
                  </div>

                  <div class="space-y-2">
                    <Label>{{ $t('chatbotSettings.systemPrompt') }}</Label>
                    <Textarea
                      v-model="aiSettings.ai_system_prompt"
                      :placeholder="$t('chatbotSettings.systemPromptPlaceholder') + '...'"
                      :rows="3"
                    />
                  </div>
                </div>

                <div class="flex justify-end pt-2">
                  <Button @click="saveAISettings" :disabled="isSubmitting">
                    <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                    {{ $t('chatbotSettings.saveChanges') }}
                  </Button>
                </div>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </div>
    </ScrollArea>
  </div>
</template>
