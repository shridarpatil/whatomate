<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { FlowStep, FlowData } from '@/types/flow-preview'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import InteractivePreview from './InteractivePreview.vue'
import {
  MessageSquare,
  MousePointerClick,
  Globe,
  MessageCircle,
  Users,
  Edit3,
  ExternalLink,
  Play
} from 'lucide-vue-next'

const props = defineProps<{
  steps: FlowStep[]
  flowData: Partial<FlowData>
  selectedStep: FlowStep | null
  selectedStepIndex: number | null
  listPickerOpen: boolean
  teams: Array<{ id: string; name: string }>
  initialMode?: 'edit' | 'preview'
}>()

const emit = defineEmits<{
  'update:listPickerOpen': [value: boolean]
  selectMessageType: [type: string]
}>()

const mode = ref<'edit' | 'preview'>(props.initialMode || 'edit')

// Watch for initialMode changes
watch(() => props.initialMode, (newMode) => {
  if (newMode) {
    mode.value = newMode
  }
})

const messageTypeIcons: Record<string, any> = {
  text: MessageSquare,
  buttons: MousePointerClick,
  api_fetch: Globe,
  whatsapp_flow: MessageCircle,
  transfer: Users
}

function handleSelectMessageType(type: string) {
  emit('selectMessageType', type)
}

// Sync listPickerOpen with parent
const localListPickerOpen = computed({
  get: () => props.listPickerOpen,
  set: (val) => emit('update:listPickerOpen', val)
})
</script>

<template>
  <div class="flex-1 flex flex-col h-full overflow-hidden">
    <!-- Mode Toggle Header -->
    <div class="px-4 py-2 border-b bg-white dark:bg-[#111b21] flex items-center justify-between">
      <div class="flex items-center gap-1 p-0.5 bg-gray-100 dark:bg-gray-800 rounded-lg">
        <Button
          variant="ghost"
          size="sm"
          class="h-7 px-3 rounded-md transition-all"
          :class="{
            'bg-white dark:bg-gray-700 shadow-sm': mode === 'edit',
            'hover:bg-gray-50 dark:hover:bg-gray-700': mode !== 'edit'
          }"
          @click="mode = 'edit'"
        >
          <Edit3 class="h-3.5 w-3.5 mr-1.5" />
          Edit
        </Button>
        <Button
          variant="ghost"
          size="sm"
          class="h-7 px-3 rounded-md transition-all"
          :class="{
            'bg-white dark:bg-gray-700 shadow-sm': mode === 'preview',
            'hover:bg-gray-50 dark:hover:bg-gray-700': mode !== 'preview'
          }"
          @click="mode = 'preview'"
        >
          <Play class="h-3.5 w-3.5 mr-1.5" />
          Preview
        </Button>
      </div>

      <div class="text-xs text-gray-500 dark:text-gray-400">
        {{ mode === 'edit' ? 'Static preview of selected step' : 'Interactive flow simulation' }}
      </div>
    </div>

    <!-- Content Area -->
    <div class="flex-1 overflow-hidden">
      <!-- Edit Mode: Step Type Palette + Static Preview -->
      <template v-if="mode === 'edit'">
        <div class="h-full flex flex-col">
          <!-- Step Type Palette -->
          <div class="p-4 border-b bg-white dark:bg-[#111b21]">
            <p class="text-xs text-muted-foreground mb-2">Message Type</p>
            <div class="flex flex-wrap gap-2">
              <Button
                v-for="(icon, type) in messageTypeIcons"
                :key="type"
                :variant="selectedStep?.message_type === type ? 'active' : 'outline'"
                size="sm"
                class="h-8 text-xs"
                @click="handleSelectMessageType(type)"
              >
                <component :is="icon" class="h-3.5 w-3.5 mr-1.5" />
                {{ type === 'api_fetch' ? 'API' : type === 'whatsapp_flow' ? 'Flow' : type.charAt(0).toUpperCase() + type.slice(1) }}
              </Button>
            </div>
          </div>

          <!-- Static WhatsApp Preview -->
          <div class="flex-1 flex items-center justify-center p-4 bg-[#efeae2] dark:bg-[#0b141a] overflow-auto">
            <div v-if="selectedStep" class="w-full max-w-sm">
              <!-- Phone Frame -->
              <div class="bg-[#efeae2] dark:bg-[#0b141a] rounded-2xl overflow-hidden shadow-xl flex flex-col h-[600px] relative">
                <!-- Chat Header -->
                <div class="bg-[#075e54] dark:bg-[#202c33] text-white px-4 py-3 flex items-center gap-3 flex-shrink-0">
                  <div class="w-10 h-10 rounded-full bg-white/20 flex items-center justify-center">
                    <MessageSquare class="h-5 w-5" />
                  </div>
                  <div>
                    <p class="font-medium text-sm">{{ flowData.name || 'WhatsApp Preview' }}</p>
                    <p class="text-xs text-white/70">Step {{ (selectedStepIndex ?? 0) + 1 }}: {{ selectedStep.step_name }}</p>
                  </div>
                </div>

                <!-- Chat Messages -->
                <ScrollArea class="flex-1 p-4">
                  <div class="space-y-3">
                  <!-- Bot Message Bubble -->
                  <div class="flex justify-start">
                    <div class="max-w-[85%]">
                      <div class="bg-white dark:bg-[#202c33] rounded-lg rounded-tl-none shadow-sm p-3">
                        <p v-if="selectedStep.message" class="text-sm text-gray-800 dark:text-gray-200 whitespace-pre-wrap">{{ selectedStep.message }}</p>
                        <p v-else class="text-sm text-gray-400 italic">No message configured</p>
                        <p class="text-[10px] text-gray-400 text-right mt-1">12:00 PM</p>
                      </div>

                      <!-- Interactive Buttons (up to 3) -->
                      <div v-if="selectedStep.message_type === 'buttons' && selectedStep.buttons.length > 0 && selectedStep.buttons.length <= 3" class="mt-1 space-y-1">
                        <button
                          v-for="(btn, idx) in selectedStep.buttons"
                          :key="idx"
                          class="w-full bg-white dark:bg-[#202c33] text-[#00a884] text-sm font-medium py-2.5 rounded-lg shadow-sm border-0 flex items-center justify-center gap-1.5"
                        >
                          <ExternalLink v-if="btn.type === 'url'" class="h-4 w-4" />
                          {{ btn.title || `Option ${idx + 1}` }}
                        </button>
                      </div>

                      <!-- List Button (more than 3 options) -->
                      <div v-if="selectedStep.message_type === 'buttons' && selectedStep.buttons.length > 3" class="mt-1">
                        <button
                          class="w-full bg-white dark:bg-[#202c33] text-[#00a884] text-sm font-medium py-2.5 rounded-lg shadow-sm border-0 flex items-center justify-center gap-2"
                          @click="localListPickerOpen = !localListPickerOpen"
                        >
                          <svg class="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
                            <path d="M3 4h18v2H3V4zm0 7h18v2H3v-2zm0 7h18v2H3v-2z"/>
                          </svg>
                          Select an option
                        </button>
                      </div>

                      <!-- WhatsApp Flow Button -->
                      <div v-if="selectedStep.message_type === 'whatsapp_flow'" class="mt-1">
                        <button class="w-full bg-white dark:bg-[#202c33] text-[#00a884] text-sm font-medium py-2.5 rounded-lg shadow-sm border-0">
                          {{ selectedStep.input_config?.flow_cta || 'Open Form' }}
                        </button>
                      </div>
                    </div>
                  </div>

                  <!-- User Response Placeholder -->
                  <div v-if="selectedStep.message_type !== 'transfer'" class="flex justify-end">
                    <div class="max-w-[85%]">
                      <div class="bg-[#005c4b] light:bg-[#d9fdd3] rounded-lg rounded-tr-none shadow-sm p-3">
                        <p class="text-sm text-gray-200 light:text-gray-800 italic">
                          <template v-if="selectedStep.input_type === 'none'">
                            (No response needed)
                          </template>
                          <template v-else-if="selectedStep.message_type === 'buttons'">
                            User taps a button...
                          </template>
                          <template v-else-if="selectedStep.message_type === 'whatsapp_flow'">
                            User completes form...
                          </template>
                          <template v-else>
                            User types {{ selectedStep.input_type }}...
                          </template>
                        </p>
                        <p class="text-[10px] text-gray-500 dark:text-gray-400 text-right mt-1 flex items-center justify-end gap-1">
                          12:01 PM
                          <svg class="h-4 w-4 text-[#53bdeb]" viewBox="0 0 24 24" fill="currentColor">
                            <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41L9 16.17z"/>
                          </svg>
                        </p>
                      </div>
                    </div>
                  </div>

                  <!-- Store As Info -->
                  <div v-if="selectedStep.store_as" class="flex justify-center">
                    <div class="bg-white/80 dark:bg-[#202c33]/80 text-xs text-gray-500 dark:text-gray-400 px-3 py-1 rounded-full">
                      Response saved as <code class="font-mono text-[#00a884]">{{ selectedStep.store_as }}</code>
                    </div>
                  </div>

                  <!-- Transfer System Message -->
                  <div v-if="selectedStep.message_type === 'transfer'" class="flex justify-center">
                    <div class="bg-amber-100 dark:bg-amber-900/30 text-xs text-amber-700 dark:text-amber-400 px-3 py-1.5 rounded-lg flex items-center gap-1.5">
                      <Users class="h-3 w-3" />
                      <span>Conversation transferred to {{ selectedStep?.transfer_config?.team_id === '_general' ? 'General Queue' : teams.find(t => t.id === selectedStep?.transfer_config?.team_id)?.name || 'Team' }}</span>
                    </div>
                  </div>

                  <!-- API Info -->
                  <div v-if="selectedStep.message_type === 'api_fetch'" class="flex justify-center">
                    <div class="bg-blue-100 dark:bg-blue-900/30 text-xs text-blue-700 dark:text-blue-400 px-3 py-1.5 rounded-lg flex items-center gap-1.5">
                      <Globe class="h-3 w-3" />
                      <span>Message populated from API</span>
                    </div>
                  </div>
                  </div>
                </ScrollArea>

                <!-- Input Bar -->
                <div class="bg-[#f0f2f5] dark:bg-[#202c33] px-3 py-2 flex items-center gap-2 flex-shrink-0">
                  <div class="flex-1 bg-white dark:bg-[#2a3942] rounded-full px-4 py-2">
                    <p class="text-sm text-gray-400">Type a message</p>
                  </div>
                  <div class="w-10 h-10 rounded-full bg-gray-300 dark:bg-gray-600 flex items-center justify-center">
                    <svg class="h-5 w-5 text-white" viewBox="0 0 24 24" fill="currentColor">
                      <path d="M12 14.5L7 10l1.4-1.4 3.6 3.6 3.6-3.6L17 10l-5 4.5z"/>
                    </svg>
                  </div>
                </div>

                <!-- List Picker Overlay -->
                <div
                  v-if="localListPickerOpen && selectedStep.message_type === 'buttons' && selectedStep.buttons.length > 3"
                  class="absolute inset-0 z-10 flex flex-col"
                >
                  <div class="flex-1 bg-black/50" @click="localListPickerOpen = false"></div>
                  <div class="bg-white dark:bg-[#1f2c34] rounded-t-2xl overflow-hidden">
                    <div class="bg-[#075e54] dark:bg-[#00a884] text-white px-4 py-3 flex items-center justify-between">
                      <button class="p-1 hover:bg-white/10 rounded" @click="localListPickerOpen = false">
                        <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <path d="M6 18L18 6M6 6l12 12"/>
                        </svg>
                      </button>
                      <span class="font-medium text-sm">Select an option</span>
                      <div class="w-5"></div>
                    </div>
                    <div class="max-h-[250px] overflow-y-auto">
                      <div
                        v-for="(btn, idx) in selectedStep.buttons"
                        :key="idx"
                        class="px-4 py-3 border-b border-gray-100 dark:border-gray-700 last:border-0 hover:bg-gray-50 dark:hover:bg-[#2a3942] cursor-pointer flex items-center gap-3"
                        @click="localListPickerOpen = false"
                      >
                        <div v-if="btn.type === 'url'" class="w-5 h-5 flex items-center justify-center flex-shrink-0 text-[#00a884]">
                          <ExternalLink class="h-4 w-4" />
                        </div>
                        <div v-else class="w-5 h-5 rounded-full border-2 border-[#00a884] flex items-center justify-center flex-shrink-0">
                          <span class="text-[10px] text-[#00a884] font-medium">{{ idx + 1 }}</span>
                        </div>
                        <span class="text-sm text-gray-800 dark:text-gray-200 flex-1">{{ btn.title || `Option ${idx + 1}` }}</span>
                        <ExternalLink v-if="btn.type === 'url'" class="h-3 w-3 text-gray-400" />
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div v-else class="w-full max-w-sm flex flex-col items-center justify-center">
              <!-- Empty State Phone Frame -->
              <div class="bg-[#efeae2] dark:bg-[#0b141a] rounded-2xl overflow-hidden shadow-xl flex flex-col h-[600px] w-full">
                <div class="bg-[#075e54] dark:bg-[#202c33] text-white px-4 py-3 flex items-center gap-3 flex-shrink-0">
                  <div class="w-10 h-10 rounded-full bg-white/20 flex items-center justify-center">
                    <MessageSquare class="h-5 w-5" />
                  </div>
                  <div>
                    <p class="font-medium text-sm">{{ flowData.name || 'WhatsApp Preview' }}</p>
                    <p class="text-xs text-white/70">Select a step</p>
                  </div>
                </div>
                <div class="flex-1 flex items-center justify-center">
                  <p class="text-sm text-gray-500 dark:text-gray-400">Select a step to view preview</p>
                </div>
                <div class="bg-[#f0f2f5] dark:bg-[#202c33] px-3 py-2 flex items-center gap-2 flex-shrink-0">
                  <div class="flex-1 bg-white dark:bg-[#2a3942] rounded-full px-4 py-2">
                    <p class="text-sm text-gray-400">Type a message</p>
                  </div>
                  <div class="w-10 h-10 rounded-full bg-gray-300 dark:bg-gray-600 flex items-center justify-center">
                    <svg class="h-5 w-5 text-white" viewBox="0 0 24 24" fill="currentColor">
                      <path d="M12 14.5L7 10l1.4-1.4 3.6 3.6 3.6-3.6L17 10l-5 4.5z"/>
                    </svg>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>

      <!-- Preview Mode: Interactive Simulation -->
      <template v-else>
        <InteractivePreview
          :steps="steps"
          :flow-data="flowData"
        />
      </template>
    </div>
  </div>
</template>
