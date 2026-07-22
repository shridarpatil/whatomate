<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import type { FlowStep, FlowData, ButtonConfig } from '@/types/flow-preview'
import { useFlowSimulation } from '@/composables/useFlowSimulation'
import { ScrollArea } from '@/components/ui/scroll-area'
import PreviewMessage from './PreviewMessage.vue'
import PreviewButtonGroup from './PreviewButtonGroup.vue'
import PreviewListPicker from './PreviewListPicker.vue'
import PreviewInputBar from './PreviewInputBar.vue'
import DebugPanel from './DebugPanel.vue'
import ApiMockDialog from './ApiMockDialog.vue'
import { MessageSquare } from 'lucide-vue-next'

const props = defineProps<{
  steps: FlowStep[]
  flowData: Partial<FlowData>
}>()

const stepsRef = computed(() => props.steps)
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
  apiMocker
} = useFlowSimulation(stepsRef, flowDataRef)

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

// Get current step for API mock dialog
const currentApiStep = computed(() => {
  if (apiMocker.currentMockStep.value) {
    return props.steps.find(s => s.step_name === apiMocker.currentMockStep.value)
  }
  return null
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
      <div class="flex-1 flex items-center justify-center p-4 bg-[#efeae2] dark:bg-[#0b141a]">
        <div id="preview-phone-frame" class="w-full max-w-sm bg-[#efeae2] dark:bg-[#0b141a] rounded-2xl overflow-hidden shadow-xl flex flex-col h-[600px] relative">
          <!-- Chat Header -->
          <div class="bg-[#075e54] dark:bg-[#202c33] text-white px-4 py-3 flex items-center gap-3 flex-shrink-0">
            <div class="w-10 h-10 rounded-full bg-white/20 flex items-center justify-center">
              <MessageSquare class="h-5 w-5" />
            </div>
            <div>
              <p class="font-medium text-sm">{{ flowData.name || 'Flow Preview' }}</p>
              <p class="text-xs text-white/70">
                <template v-if="state.status === 'idle'">
                  Click Start to begin
                </template>
                <template v-else-if="state.currentStepName">
                  Step {{ (state.currentStepIndex ?? 0) + 1 }}: {{ state.currentStepName }}
                </template>
                <template v-else>
                  {{ state.status }}
                </template>
              </p>
            </div>
          </div>

          <!-- Chat Messages -->
          <ScrollArea ref="chatScrollRef" class="flex-1 p-4">
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

    <!-- Debug Panel -->
    <div class="w-64 flex-shrink-0">
      <DebugPanel
        :state="state"
        :steps="steps"
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
