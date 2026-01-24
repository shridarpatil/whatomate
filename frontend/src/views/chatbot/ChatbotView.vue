<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { chatbotService } from '@/services/api'
import { toast } from 'vue-sonner'
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
} from 'lucide-vue-next'

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

onMounted(async () => {
  try {
    const response = await chatbotService.getSettings()
    // API response is wrapped in { status: "success", data: { settings: {...}, stats: {...} } }
    const data = response.data.data || response.data
    settings.value = data.settings || settings.value
    stats.value = data.stats || stats.value
  } catch (error) {
    console.error('Failed to load chatbot settings:', error)
    // Keep default values on error
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
    toast.success(newState ? 'Chatbot enabled' : 'Chatbot disabled')
  } catch (error) {
    toast.error('Failed to toggle chatbot')
  } finally {
    isToggling.value = false
  }
}

const statCards = [
  { title: 'Total Sessions', key: 'total_sessions', icon: Users, color: 'text-blue-500' },
  { title: 'Active Sessions', key: 'active_sessions', icon: MessageSquare, color: 'text-green-500' },
  { title: 'Messages Handled', key: 'messages_handled', icon: TrendingUp, color: 'text-purple-500' },
  { title: 'AI Responses', key: 'ai_responses', icon: Sparkles, color: 'text-orange-500' }
]
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <!-- Header -->
    <header class="border-b border-white/[0.08] light:border-gray-200 bg-[#0a0a0b]/95 light:bg-white/95 backdrop-blur">
      <div class="flex h-16 items-center px-6">
        <div class="h-8 w-8 rounded-lg bg-gradient-to-br from-purple-500 to-pink-600 flex items-center justify-center mr-3 shadow-lg shadow-purple-500/20">
          <Bot class="h-4 w-4 text-white" />
        </div>
        <div class="flex-1">
          <h1 class="text-xl font-semibold text-white light:text-gray-900">Chatbot</h1>
          <p class="text-sm text-white/50 light:text-gray-500">Manage automated responses and AI conversations</p>
        </div>
        <div class="flex items-center gap-3">
          <Badge
            :class="settings.enabled ? 'bg-emerald-500/20 text-emerald-400 light:bg-emerald-100 light:text-emerald-700' : 'bg-white/[0.08] text-white/50 light:bg-gray-100 light:text-gray-500'"
          >
            {{ settings.enabled ? 'Active' : 'Inactive' }}
          </Badge>
          <Button
            variant="outline"
            size="sm"
            @click="toggleChatbot"
            :disabled="isToggling"
            :class="settings.enabled ? 'border-red-500/50 text-red-400 hover:bg-red-500/10' : 'border-emerald-500/50 text-emerald-400 hover:bg-emerald-500/10'"
          >
            <Power class="h-4 w-4 mr-2" />
            {{ settings.enabled ? 'Disable' : 'Enable' }}
          </Button>
        </div>
      </div>
    </header>

    <!-- Content -->
    <ScrollArea class="flex-1">
      <div class="p-6 space-y-6">
        <!-- Stats -->
        <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          <!-- Skeleton Loading State -->
          <template v-if="isLoading">
            <div v-for="i in 4" :key="i" class="rounded-xl border border-white/[0.08] bg-white/[0.02] p-6 light:bg-white light:border-gray-200">
              <div class="flex flex-row items-center justify-between space-y-0 pb-2">
                <Skeleton class="h-4 w-24 bg-white/[0.08] light:bg-gray-200" />
                <Skeleton class="h-10 w-10 rounded-lg bg-white/[0.08] light:bg-gray-200" />
              </div>
              <div class="pt-2">
                <Skeleton class="h-8 w-16 bg-white/[0.08] light:bg-gray-200" />
              </div>
            </div>
          </template>
          <!-- Actual Stats -->
          <template v-else>
            <div v-for="card in statCards" :key="card.key" class="card-depth rounded-xl border border-white/[0.08] bg-white/[0.04] p-6 light:bg-white light:border-gray-200">
              <div class="flex flex-row items-center justify-between space-y-0 pb-2">
                <span class="text-sm font-medium text-white/50 light:text-gray-500">{{ card.title }}</span>
                <div :class="[
                  'h-10 w-10 rounded-lg flex items-center justify-center',
                  card.key === 'total_sessions' ? 'bg-blue-500/20' : '',
                  card.key === 'active_sessions' ? 'bg-emerald-500/20' : '',
                  card.key === 'messages_handled' ? 'bg-purple-500/20' : '',
                  card.key === 'ai_responses' ? 'bg-orange-500/20' : ''
                ]">
                  <component :is="card.icon" :class="[
                    'h-5 w-5',
                    card.key === 'total_sessions' ? 'text-blue-400' : '',
                    card.key === 'active_sessions' ? 'text-emerald-400' : '',
                    card.key === 'messages_handled' ? 'text-purple-400' : '',
                    card.key === 'ai_responses' ? 'text-orange-400' : ''
                  ]" />
                </div>
              </div>
              <div class="pt-2">
                <div class="text-3xl font-bold text-white light:text-gray-900">
                  {{ stats[card.key as keyof Stats].toLocaleString() }}
                </div>
              </div>
            </div>
          </template>
        </div>

        <!-- Quick Actions -->
        <div class="grid gap-4 md:grid-cols-3">
          <RouterLink to="/chatbot/keywords" class="card-interactive rounded-xl border border-white/[0.08] bg-white/[0.02] h-full light:bg-white light:border-gray-200">
            <div class="p-6">
              <div class="flex items-center gap-3">
                <div class="h-10 w-10 rounded-lg bg-gradient-to-br from-blue-500 to-cyan-600 flex items-center justify-center shadow-lg shadow-blue-500/20">
                  <Key class="h-5 w-5 text-white" />
                </div>
                <div>
                  <h3 class="text-lg font-semibold text-white light:text-gray-900">Keyword Rules</h3>
                  <p class="text-sm text-white/40 light:text-gray-500">{{ stats.keywords_count }} rules configured</p>
                </div>
              </div>
            </div>
            <div class="px-6 pb-6">
              <p class="text-sm text-white/50 light:text-gray-600">
                Create automated responses triggered by specific keywords or phrases.
              </p>
            </div>
          </RouterLink>

          <RouterLink to="/chatbot/flows" class="card-interactive rounded-xl border border-white/[0.08] bg-white/[0.02] h-full light:bg-white light:border-gray-200">
            <div class="p-6">
              <div class="flex items-center gap-3">
                <div class="h-10 w-10 rounded-lg bg-gradient-to-br from-purple-500 to-pink-600 flex items-center justify-center shadow-lg shadow-purple-500/20">
                  <Workflow class="h-5 w-5 text-white" />
                </div>
                <div>
                  <h3 class="text-lg font-semibold text-white light:text-gray-900">Conversation Flows</h3>
                  <p class="text-sm text-white/40 light:text-gray-500">{{ stats.flows_count }} flows created</p>
                </div>
              </div>
            </div>
            <div class="px-6 pb-6">
              <p class="text-sm text-white/50 light:text-gray-600">
                Design multi-step conversation flows with branching logic.
              </p>
            </div>
          </RouterLink>

          <RouterLink to="/chatbot/ai" class="card-interactive rounded-xl border border-white/[0.08] bg-white/[0.02] h-full light:bg-white light:border-gray-200">
            <div class="p-6">
              <div class="flex items-center gap-3">
                <div class="h-10 w-10 rounded-lg bg-gradient-to-br from-orange-500 to-amber-600 flex items-center justify-center shadow-lg shadow-orange-500/20">
                  <Sparkles class="h-5 w-5 text-white" />
                </div>
                <div>
                  <h3 class="text-lg font-semibold text-white light:text-gray-900">AI Contexts</h3>
                  <p class="text-sm text-white/40 light:text-gray-500">{{ stats.ai_contexts_count }} contexts active</p>
                </div>
              </div>
            </div>
            <div class="px-6 pb-6">
              <p class="text-sm text-white/50 light:text-gray-600">
                Configure AI-powered responses with custom knowledge bases.
              </p>
            </div>
          </RouterLink>
        </div>

        <!-- Current Settings -->
        <div class="rounded-xl border border-white/[0.08] bg-white/[0.02] light:bg-white light:border-gray-200">
          <div class="p-6">
            <div class="flex items-center justify-between">
              <div>
                <h3 class="text-lg font-semibold text-white light:text-gray-900">Current Configuration</h3>
                <p class="text-sm text-white/40 light:text-gray-500">Overview of your chatbot settings</p>
              </div>
              <RouterLink to="/settings/chatbot">
                <Button variant="outline" size="sm">
                  <Settings class="h-4 w-4 mr-2" />
                  Edit Settings
                </Button>
              </RouterLink>
            </div>
          </div>
          <div class="px-6 pb-6">
            <div class="grid gap-4 md:grid-cols-2">
              <div class="space-y-2">
                <h4 class="font-medium text-sm text-white/70 light:text-gray-700">Greeting Message</h4>
                <p class="text-sm text-white/50 light:text-gray-600 bg-white/[0.04] light:bg-gray-100 p-3 rounded-lg">
                  {{ settings.greeting_message || 'Not configured' }}
                </p>
              </div>
              <div class="space-y-2">
                <h4 class="font-medium text-sm text-white/70 light:text-gray-700">Fallback Message</h4>
                <p class="text-sm text-white/50 light:text-gray-600 bg-white/[0.04] light:bg-gray-100 p-3 rounded-lg">
                  {{ settings.fallback_message || 'Not configured' }}
                </p>
              </div>
              <div class="space-y-2">
                <h4 class="font-medium text-sm text-white/70 light:text-gray-700">Session Timeout</h4>
                <div class="flex items-center gap-2 text-sm text-white/50 light:text-gray-600">
                  <Clock class="h-4 w-4" />
                  {{ settings.session_timeout_minutes }} minutes
                </div>
              </div>
              <div class="space-y-2">
                <h4 class="font-medium text-sm text-white/70 light:text-gray-700">AI Provider</h4>
                <div class="flex items-center gap-2">
                  <Badge v-if="settings.ai_enabled" class="bg-emerald-500/20 text-emerald-400 light:bg-emerald-100 light:text-emerald-700">
                    {{ settings.ai_provider || 'Not configured' }}
                  </Badge>
                  <Badge v-else class="bg-white/[0.08] text-white/50 light:bg-gray-100 light:text-gray-500">Disabled</Badge>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </ScrollArea>
  </div>
</template>
