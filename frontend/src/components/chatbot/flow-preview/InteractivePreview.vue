<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import type { ChatFlowGraph } from '@/services/api'
import type { FlowData, ButtonConfig } from '@/types/flow-preview'
import { useFlowGraphSimulation } from '@/composables/useFlowGraphSimulation'
import { ScrollArea } from '@/components/ui/scroll-area'
import PreviewMessage from './PreviewMessage.vue'
import PreviewButtonGroup from './PreviewButtonGroup.vue'
import PreviewListPicker from './PreviewListPicker.vue'
import PreviewInputBar from './PreviewInputBar.vue'
import DebugPanel from './DebugPanel.vue'
import ApiMockDialog from './ApiMockDialog.vue'
import { MessageSquare } from 'lucide-vue-next'

const props = defineProps<{
  graph: ChatFlowGraph | null
  flowData: Partial<FlowData>
}>()

const graphRef = computed(() => props.graph)
const flowDataRef = computed(() => props.flowData)

const {
  state,
  currentStep,
  isWaitingForInput,
  expectedInputType,
  canUndo,
  startSimulation,
  pauseSimulation,
  resumeSimulation,
  resetSimulation,
  processUserInput,
  processWhatsAppFlowCompletion,
  undo,
  stepForward,
  goToStep,
  apiMocker,
} = useFlowGraphSimulation(graphRef, flowDataRef)

const chatScrollRef = ref<InstanceType<typeof ScrollArea> | null>(null)

// Auto-scroll to bottom when messages change
watch(
  () => state.messages.length,
  async () => {
    await nextTick()
    if (chatScrollRef.value?.$el) {
      const scrollArea = chatScrollRef.value.$el.querySelector('[data-reka-scroll-area-viewport]') ||
                         chatScrollRef.value.$el.querySelector('[data-radix-scroll-area-viewport]') ||
                         chatScrollRef.value.$el.querySelector('[style*="overflow"]')
      if (scrollArea) {
        scrollArea.scrollTop = scrollArea.scrollHeight
      }
    }
  }
)

// Get the last message that has buttons
const lastButtonMessage = computed(() => {
  if (!isWaitingForInput.value || !currentStep.value) return null
  if (currentStep.value.message_type !== 'buttons') return null

  const lastBotMessage = [...state.messages].reverse().find(m => m.type === 'bot' && m.buttons?.length)
  return lastBotMessage
})

// Shim for DebugPanel which expects FlowStep[]-shaped objects with step_name.
const debugSteps = computed(() => (props.graph?.nodes || []).map((n) => ({
  step_name: n.id,
  message_type: n.type,
})))

// Get current node for API mock dialog (node-shaped, but the dialog only
// reads step_name + api_config). We surface an adapter so existing
// ApiMockDialog props keep working without a refactor.
const currentApiStep = computed(() => {
  const mockingId = apiMocker.currentMockStep.value
  if (!mockingId) return null
  const node = props.graph?.nodes.find((n) => n.id === mockingId)
  if (!node) return null
  return {
    step_name: node.id,
    api_config: {
      url: (node.config?.url as string) || '',
      method: (node.config?.method as string) || 'GET',
      headers: (node.config?.headers as Record<string, string>) || {},
      body: (node.config?.body as string) || '',
      response_mapping: (node.config?.response_mapping as Record<string, string>) || {},
      fallback_message: (node.config?.fallback_message as string) || '',
    },
  } as any
})

function handleButtonSelect(button: ButtonConfig) {
  processUserInput(button)
}

function handleTextSubmit(value: string) {
  processUserInput(value)
}

function handleWhatsAppFlowComplete() {
  processWhatsAppFlowCompletion({})
}

function handleStart() {
  startSimulation()
}

function handlePause() {
  pauseSimulation()
}

function handleResume() {
  resumeSimulation()
}

function handleReset() {
  resetSimulation()
  // "Reset" in the debug panel really means restart — users expect a
  // fresh run, not a frozen idle screen.
  startSimulation()
}

function handleStepForward() {
  stepForward()
}

function handleUndo() {
  undo()
}

function handleGoToStep(stepName: string) {
  goToStep(stepName)
}
</script>

<template>
  <div class="flex-1 flex h-full">
    <!-- Chat Area -->
    <div class="flex-1 flex flex-col min-w-0">
      <!-- Phone Frame Container -->
      <div class="flex-1 flex items-center justify-center p-6 bg-gray-100 dark:bg-gray-900">
        <!-- Device bezel -->
        <div
          id="preview-phone-frame"
          class="w-full max-w-[400px] h-full max-h-[760px] bg-black rounded-[40px] p-[10px] shadow-2xl flex flex-col"
        >
          <!-- Screen -->
          <div class="flex-1 rounded-[32px] overflow-hidden flex flex-col bg-[#efeae2] dark:bg-[#0b141a]">
            <!-- iOS-ish status bar -->
            <div class="bg-[#008069] dark:bg-[#202c33] text-white px-5 py-1 flex items-center justify-between text-[11px] font-medium flex-shrink-0">
              <span>9:41</span>
              <div class="flex items-center gap-1">
                <!-- signal -->
                <svg width="14" height="10" viewBox="0 0 18 12" fill="currentColor"><rect x="0" y="8" width="3" height="4" rx="0.5"/><rect x="5" y="5" width="3" height="7" rx="0.5"/><rect x="10" y="2" width="3" height="10" rx="0.5"/><rect x="15" y="0" width="3" height="12" rx="0.5" opacity="0.4"/></svg>
                <!-- battery -->
                <svg width="22" height="10" viewBox="0 0 28 12" fill="none"><rect x="0.5" y="0.5" width="24" height="11" rx="2" stroke="currentColor"/><rect x="25.5" y="3.5" width="2" height="5" rx="0.5" fill="currentColor"/><rect x="2" y="2" width="19" height="8" rx="1" fill="currentColor"/></svg>
              </div>
            </div>

            <!-- Chat Header -->
            <div class="bg-[#008069] dark:bg-[#202c33] text-white px-3 py-2 flex items-center gap-3 flex-shrink-0">
              <div class="w-9 h-9 rounded-full bg-white/20 flex items-center justify-center flex-shrink-0">
                <MessageSquare class="h-4 w-4" />
              </div>
              <div class="flex-1 min-w-0">
                <p class="font-medium text-sm truncate">{{ flowData.name || 'Flow Preview' }}</p>
                <p class="text-[11px] text-white/80 truncate">
                  <template v-if="state.status === 'idle'">tap Start to begin</template>
                  <template v-else-if="state.currentStepName">{{ state.currentStepName }}</template>
                  <template v-else>{{ state.status }}</template>
                </p>
              </div>
            </div>

            <!-- Chat Messages -->
            <ScrollArea ref="chatScrollRef" class="flex-1 p-4 whatsapp-bg">
            <div class="space-y-3">
              <!-- Idle State -->
              <div v-if="state.status === 'idle' && state.messages.length === 0" class="text-center py-12">
                <MessageSquare class="h-12 w-12 mx-auto text-gray-300 dark:text-gray-600 mb-4" />
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  Start the preview to simulate the flow
                </p>
              </div>

              <!-- Messages -->
              <PreviewMessage
                v-for="message in state.messages"
                :key="message.id"
                :message="message"
              />

              <!-- Interactive WhatsApp Flow CTA -->
              <div
                v-if="isWaitingForInput && currentStep?.message_type === 'whatsapp_flow'"
                class="flex justify-start"
              >
                <div class="max-w-[85%]">
                  <button
                    class="px-4 py-2 bg-[#075e54] text-white text-sm rounded-lg hover:bg-[#064e46] transition-colors"
                    @click="handleWhatsAppFlowComplete"
                  >
                    {{ currentStep.input_config?.flow_cta || 'Open Form' }}
                  </button>
                  <p class="text-[10px] text-gray-500 mt-1 italic">Simulated: clicks complete the flow</p>
                </div>
              </div>

              <!-- Interactive Buttons (show only for last bot message when waiting) -->
              <div
                v-if="lastButtonMessage && lastButtonMessage.buttons"
                class="flex justify-start"
              >
                <div class="max-w-[85%]">
                  <PreviewButtonGroup
                    v-if="lastButtonMessage.buttons.length <= 3"
                    :buttons="lastButtonMessage.buttons"
                    :disabled="!isWaitingForInput"
                    @select="handleButtonSelect"
                  />
                  <PreviewListPicker
                    v-else
                    :buttons="lastButtonMessage.buttons"
                    :disabled="!isWaitingForInput"
                    @select="handleButtonSelect"
                  />
                </div>
              </div>
            </div>
          </ScrollArea>

            <!-- Input Bar -->
            <PreviewInputBar
              :input-type="expectedInputType"
              :disabled="!isWaitingForInput || expectedInputType === 'button' || expectedInputType === 'whatsapp_flow'"
              @submit="handleTextSubmit"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- Debug Panel -->
    <div class="w-64 flex-shrink-0">
      <DebugPanel
        :state="state"
        :steps="debugSteps as any"
        :can-undo="canUndo"
        @start="handleStart"
        @pause="handlePause"
        @resume="handleResume"
        @reset="handleReset"
        @step-forward="handleStepForward"
        @undo="handleUndo"
        @go-to-step="handleGoToStep"
      />
    </div>

    <!-- API Mock Dialog -->
    <ApiMockDialog
      :open="apiMocker.showMockDialog.value"
      :step="currentApiStep || null"
      @update:open="(open) => { if (!open) apiMocker.submitMockConfig(null) }"
      @submit="apiMocker.submitMockConfig"
    />
  </div>
</template>

<style scoped>
/* Faint WhatsApp-style background pattern — speech bubbles + envelopes
   only, at very low opacity. No hearts. */
.whatsapp-bg {
  background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='140' height='140' viewBox='0 0 140 140'><g fill='none' stroke='%2300000010' stroke-width='1.3' stroke-linejoin='round'><path d='M18 24c0-3 2-5 5-5h18c3 0 5 2 5 5v12c0 3-2 5-5 5h-12l-6 6v-6c-3 0-5-2-5-5z'/><rect x='86' y='30' width='32' height='18' rx='1.5'/><path d='M86 30l16 11 16-11'/><path d='M30 92c0-3 2-5 5-5h14c3 0 5 2 5 5v9c0 3-2 5-5 5h-9l-5 5v-5c-3 0-5-2-5-5z'/><rect x='90' y='95' width='30' height='17' rx='1.5'/><path d='M90 95l15 10 15-10'/></g></svg>");
  background-size: 140px 140px;
  background-repeat: repeat;
}

:global(.dark) .whatsapp-bg {
  background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='140' height='140' viewBox='0 0 140 140'><g fill='none' stroke='%23ffffff0a' stroke-width='1.3' stroke-linejoin='round'><path d='M18 24c0-3 2-5 5-5h18c3 0 5 2 5 5v12c0 3-2 5-5 5h-12l-6 6v-6c-3 0-5-2-5-5z'/><rect x='86' y='30' width='32' height='18' rx='1.5'/><path d='M86 30l16 11 16-11'/><path d='M30 92c0-3 2-5 5-5h14c3 0 5 2 5 5v9c0 3-2 5-5 5h-9l-5 5v-5c-3 0-5-2-5-5z'/><rect x='90' y='95' width='30' height='17' rx='1.5'/><path d='M90 95l15 10 15-10'/></g></svg>");
}
</style>

