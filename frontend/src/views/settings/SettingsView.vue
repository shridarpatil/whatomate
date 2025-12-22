<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { Slider } from '@/components/ui/slider'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { toast } from 'vue-sonner'
import { Settings, Bot, Bell, Loader2, Brain } from 'lucide-vue-next'
import { chatbotService, usersService } from '@/services/api'

const isSubmitting = ref(false)

const generalSettings = ref({
  organization_name: 'My Organization',
  default_timezone: 'UTC',
  date_format: 'YYYY-MM-DD'
})

const chatbotSettings = ref({
  greeting_message: '',
  fallback_message: '',
  session_timeout_minutes: 30,
  transfer_message: ''
})

const aiSettings = ref({
  ai_enabled: false,
  ai_provider: '',
  ai_api_key: '',
  ai_model: '',
  ai_max_tokens: 500,
  ai_system_prompt: ''
})

// Separate ref for Switch to ensure reactivity
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

// Keep aiSettings in sync with isAIEnabled
watch(isAIEnabled, (newValue) => {
  aiSettings.value.ai_enabled = newValue
})

onMounted(async () => {
  // Load chatbot settings
  try {
    const response = await chatbotService.getSettings()
    const data = response.data.data || response.data
    if (data.settings) {
      chatbotSettings.value = {
        greeting_message: data.settings.greeting_message || '',
        fallback_message: data.settings.fallback_message || '',
        session_timeout_minutes: data.settings.session_timeout_minutes || 30,
        transfer_message: ''
      }
      const aiEnabledValue = data.settings.ai_enabled === true
      isAIEnabled.value = aiEnabledValue
      aiSettings.value = {
        ai_enabled: aiEnabledValue,
        ai_provider: data.settings.ai_provider || '',
        ai_api_key: '', // Don't load API key for security
        ai_model: data.settings.ai_model || '',
        ai_max_tokens: data.settings.ai_max_tokens || 500,
        ai_system_prompt: data.settings.ai_system_prompt || ''
      }
    }
  } catch (error) {
    console.error('Failed to load chatbot settings:', error)
  }

  // Load user notification settings
  try {
    const response = await usersService.me()
    const user = response.data.data || response.data
    if (user.settings) {
      notificationSettings.value = {
        email_notifications: user.settings.email_notifications ?? true,
        new_message_alerts: user.settings.new_message_alerts ?? true,
        campaign_updates: user.settings.campaign_updates ?? true
      }
    }
  } catch (error) {
    console.error('Failed to load user settings:', error)
  }
})

const notificationSettings = ref({
  email_notifications: true,
  new_message_alerts: true,
  campaign_updates: true
})

async function saveGeneralSettings() {
  isSubmitting.value = true
  try {
    // API call would go here
    await new Promise(resolve => setTimeout(resolve, 500))
    toast.success('General settings saved')
  } catch (error) {
    toast.error('Failed to save settings')
  } finally {
    isSubmitting.value = false
  }
}

async function saveChatbotSettings() {
  isSubmitting.value = true
  try {
    await chatbotService.updateSettings({
      greeting_message: chatbotSettings.value.greeting_message,
      fallback_message: chatbotSettings.value.fallback_message,
      session_timeout_minutes: chatbotSettings.value.session_timeout_minutes
    })
    toast.success('Chatbot settings saved')
  } catch (error) {
    toast.error('Failed to save settings')
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
    // Only send API key if it's been changed (not empty)
    if (aiSettings.value.ai_api_key) {
      payload.ai_api_key = aiSettings.value.ai_api_key
    }
    await chatbotService.updateSettings(payload)
    toast.success('AI settings saved')
    aiSettings.value.ai_api_key = '' // Clear the API key field after save
  } catch (error) {
    toast.error('Failed to save AI settings')
  } finally {
    isSubmitting.value = false
  }
}

async function saveNotificationSettings() {
  isSubmitting.value = true
  try {
    await usersService.updateSettings({
      email_notifications: notificationSettings.value.email_notifications,
      new_message_alerts: notificationSettings.value.new_message_alerts,
      campaign_updates: notificationSettings.value.campaign_updates
    })
    toast.success('Notification settings saved')
  } catch (error) {
    toast.error('Failed to save notification settings')
  } finally {
    isSubmitting.value = false
  }
}
</script>

<template>
  <div class="flex flex-col h-full">
    <!-- Header -->
    <header class="border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div class="flex h-16 items-center px-6">
        <Settings class="h-5 w-5 mr-3" />
        <div>
          <h1 class="text-xl font-semibold">Settings</h1>
          <p class="text-sm text-muted-foreground">Manage your account and application settings</p>
        </div>
      </div>
    </header>

    <!-- Content -->
    <ScrollArea class="flex-1">
      <div class="p-6 space-y-4 max-w-4xl mx-auto">
        <Tabs default-value="general" class="w-full">
          <TabsList class="grid w-full grid-cols-4 mb-6">
            <TabsTrigger value="general">
              <Settings class="h-4 w-4 mr-2" />
              General
            </TabsTrigger>
            <TabsTrigger value="chatbot">
              <Bot class="h-4 w-4 mr-2" />
              Chatbot
            </TabsTrigger>
            <TabsTrigger value="ai">
              <Brain class="h-4 w-4 mr-2" />
              AI
            </TabsTrigger>
            <TabsTrigger value="notifications">
              <Bell class="h-4 w-4 mr-2" />
              Notifications
            </TabsTrigger>
          </TabsList>

          <!-- General Settings Tab -->
          <TabsContent value="general">
            <Card>
              <CardHeader>
                <CardTitle>General Settings</CardTitle>
                <CardDescription>Basic organization and display settings</CardDescription>
              </CardHeader>
              <CardContent class="space-y-4">
                <div class="space-y-2">
                  <Label for="org_name">Organization Name</Label>
                  <Input
                    id="org_name"
                    v-model="generalSettings.organization_name"
                    placeholder="Your Organization"
                  />
                </div>
                <div class="grid grid-cols-2 gap-4">
                  <div class="space-y-2">
                    <Label for="timezone">Default Timezone</Label>
                    <Select v-model="generalSettings.default_timezone">
                      <SelectTrigger>
                        <SelectValue placeholder="Select timezone" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="UTC">UTC</SelectItem>
                        <SelectItem value="America/New_York">Eastern Time</SelectItem>
                        <SelectItem value="America/Los_Angeles">Pacific Time</SelectItem>
                        <SelectItem value="Europe/London">London</SelectItem>
                        <SelectItem value="Asia/Tokyo">Tokyo</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div class="space-y-2">
                    <Label for="date_format">Date Format</Label>
                    <Select v-model="generalSettings.date_format">
                      <SelectTrigger>
                        <SelectValue placeholder="Select format" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="YYYY-MM-DD">YYYY-MM-DD</SelectItem>
                        <SelectItem value="DD/MM/YYYY">DD/MM/YYYY</SelectItem>
                        <SelectItem value="MM/DD/YYYY">MM/DD/YYYY</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </div>
                <div class="flex justify-end">
                  <Button @click="saveGeneralSettings" :disabled="isSubmitting">
                    <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                    Save Changes
                  </Button>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <!-- Chatbot Settings Tab -->
          <TabsContent value="chatbot">
            <Card>
              <CardHeader>
                <CardTitle>Chatbot Settings</CardTitle>
                <CardDescription>Configure default chatbot behavior</CardDescription>
              </CardHeader>
              <CardContent class="space-y-4">
                <div class="space-y-2">
                  <Label for="greeting">Greeting Message</Label>
                  <Textarea
                    id="greeting"
                    v-model="chatbotSettings.greeting_message"
                    placeholder="Hello! How can I help you?"
                    :rows="2"
                  />
                </div>
                <div class="space-y-2">
                  <Label for="fallback">Fallback Message</Label>
                  <Textarea
                    id="fallback"
                    v-model="chatbotSettings.fallback_message"
                    placeholder="Sorry, I didn't understand that."
                    :rows="2"
                  />
                </div>
                <div class="space-y-2">
                  <Label for="transfer">Transfer Message</Label>
                  <Textarea
                    id="transfer"
                    v-model="chatbotSettings.transfer_message"
                    placeholder="Transferring you to a human agent..."
                    :rows="2"
                  />
                </div>
                <div class="space-y-2">
                  <Label for="timeout">Session Timeout (minutes)</Label>
                  <Input
                    id="timeout"
                    v-model.number="chatbotSettings.session_timeout_minutes"
                    type="number"
                    min="5"
                    max="120"
                  />
                </div>
                <div class="flex justify-end">
                  <Button @click="saveChatbotSettings" :disabled="isSubmitting">
                    <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                    Save Changes
                  </Button>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <!-- AI Settings Tab -->
          <TabsContent value="ai">
            <Card>
              <CardHeader>
                <CardTitle>AI Settings</CardTitle>
                <CardDescription>Configure AI-powered responses for your chatbot</CardDescription>
              </CardHeader>
              <CardContent class="space-y-4">
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium">Enable AI Responses</p>
                    <p class="text-sm text-muted-foreground">Use AI to generate responses when no flow matches</p>
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
                      <Label for="ai_provider">AI Provider</Label>
                      <Select v-model="aiSettings.ai_provider">
                        <SelectTrigger>
                          <SelectValue placeholder="Select provider..." />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem v-for="provider in aiProviders" :key="provider.value" :value="provider.value">
                            {{ provider.label }}
                          </SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div class="space-y-2">
                      <Label for="ai_model">Model</Label>
                      <Select v-model="aiSettings.ai_model" :disabled="!aiSettings.ai_provider">
                        <SelectTrigger>
                          <SelectValue placeholder="Select model..." />
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
                    <Label for="ai_api_key">API Key</Label>
                    <Input
                      id="ai_api_key"
                      v-model="aiSettings.ai_api_key"
                      type="password"
                      placeholder="Enter API key (leave empty to keep existing)"
                    />
                    <p class="text-xs text-muted-foreground">Your API key is encrypted and stored securely</p>
                  </div>

                  <div class="space-y-2">
                    <Label for="ai_max_tokens">Max Tokens</Label>
                    <Input
                      id="ai_max_tokens"
                      v-model.number="aiSettings.ai_max_tokens"
                      type="number"
                      min="100"
                      max="4000"
                    />
                    <p class="text-xs text-muted-foreground">Maximum number of tokens for AI responses (100-4000)</p>
                  </div>

                  <div class="space-y-2">
                    <Label for="ai_system_prompt">System Prompt (optional)</Label>
                    <Textarea
                      id="ai_system_prompt"
                      v-model="aiSettings.ai_system_prompt"
                      placeholder="You are a helpful customer service assistant..."
                      :rows="3"
                    />
                    <p class="text-xs text-muted-foreground">Instructions for the AI on how to respond</p>
                  </div>
                </div>

                <div class="flex justify-end pt-2">
                  <Button @click="saveAISettings" :disabled="isSubmitting">
                    <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                    Save Changes
                  </Button>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <!-- Notification Settings Tab -->
          <TabsContent value="notifications">
            <Card>
              <CardHeader>
                <CardTitle>Notifications</CardTitle>
                <CardDescription>Manage how you receive notifications</CardDescription>
              </CardHeader>
              <CardContent class="space-y-4">
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium">Email Notifications</p>
                    <p class="text-sm text-muted-foreground">Receive important updates via email</p>
                  </div>
                  <Switch
                    :checked="notificationSettings.email_notifications"
                    @update:checked="notificationSettings.email_notifications = $event"
                  />
                </div>
                <Separator />
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium">New Message Alerts</p>
                    <p class="text-sm text-muted-foreground">Get notified when new messages arrive</p>
                  </div>
                  <Switch
                    :checked="notificationSettings.new_message_alerts"
                    @update:checked="notificationSettings.new_message_alerts = $event"
                  />
                </div>
                <Separator />
                <div class="flex items-center justify-between">
                  <div>
                    <p class="font-medium">Campaign Updates</p>
                    <p class="text-sm text-muted-foreground">Receive campaign status notifications</p>
                  </div>
                  <Switch
                    :checked="notificationSettings.campaign_updates"
                    @update:checked="notificationSettings.campaign_updates = $event"
                  />
                </div>
                <div class="flex justify-end pt-4">
                  <Button @click="saveNotificationSettings" :disabled="isSubmitting">
                    <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                    Save Changes
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
