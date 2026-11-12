<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import type { ChatFlowGraph } from '@/services/api'
import type { FlowData, ButtonConfig } from '@/types/flow-preview'
import { useFlowGraphSimulation } from '@/composables/useFlowGraphSimulation'
import PreviewMessage from './PreviewMessage.vue'
import PreviewButtonGroup from './PreviewButtonGroup.vue'
import PreviewListPicker from './PreviewListPicker.vue'
import PreviewInputBar from './PreviewInputBar.vue'
import DebugPanel from './DebugPanel.vue'
import ApiMockDialog from './ApiMockDialog.vue'
import { MessageSquare } from '@/lib/icons'

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

const chatScrollRef = ref<HTMLElement | null>(null)

// Auto-scroll to bottom when messages change
watch(
  () => state.messages.length,
  async () => {
    await nextTick()
    const el = chatScrollRef.value
    if (el) el.scrollTop = el.scrollHeight
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
  <div class="flex flex-wrap flex-1 gap-4 h-full min-h-0 min-w-0">
    <!-- Chat Area -->
    <div class="flex flex-1 justify-center min-w-0 min-h-0">
      <article class="card p-4 side-panel flex flex-col gap-4 min-h-0">
        <!-- Chat Header -->
        <header class="hstack gap-2 shrink-0 min-w-0">
          <MessageSquare :size="20" class="shrink-0" />
          <div class="vstack min-w-0">
            <strong class="truncate">{{ flowData.name || 'Flow Preview' }}</strong>
            <small class="truncate text-light">
              <template v-if="state.status === 'idle'">tap Start to begin</template>
              <template v-else-if="state.currentStepName">{{ state.currentStepName }}</template>
              <template v-else>{{ state.status }}</template>
            </small>
          </div>
        </header>

        <!-- Chat Messages -->
        <div ref="chatScrollRef" class="flex-1 scroll-y vstack gap-4" aria-live="polite">
          <!-- Idle State -->
          <div v-if="state.status === 'idle' && state.messages.length === 0" class="vstack items-center justify-center gap-2 p-4 text-light align-center">
            <MessageSquare :size="48" />
            <small>Start the preview to simulate the flow</small>
          </div>

          <!-- Messages -->
          <PreviewMessage
            v-for="message in state.messages"
            :key="message.id"
            :message="message"
          />

          <!-- Interactive WhatsApp Flow CTA -->
          <div v-if="isWaitingForInput && currentStep?.message_type === 'whatsapp_flow'" class="vstack gap-1" data-bubble="incoming">
            <button type="button" class="outline" @click="handleWhatsAppFlowComplete">
              {{ currentStep.input_config?.flow_cta || 'Open Form' }}
            </button>
            <small class="text-lighter"><em>Simulated: clicks complete the flow</em></small>
          </div>

          <!-- Interactive Buttons (show only for last bot message when waiting) -->
          <div v-if="lastButtonMessage && lastButtonMessage.buttons" data-bubble="incoming">
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

        <!-- Input Bar -->
        <PreviewInputBar
          class="shrink-0"
          :input-type="expectedInputType"
          :disabled="!isWaitingForInput || expectedInputType === 'button' || expectedInputType === 'whatsapp_flow'"
          @submit="handleTextSubmit"
        />
      </article>
    </div>

    <!-- Debug Panel -->
    <DebugPanel
      class="side-panel shrink-0"
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

    <!-- API Mock Dialog -->
    <ApiMockDialog
      :open="apiMocker.showMockDialog.value"
      :step="currentApiStep || null"
      @update:open="(open) => { if (!open) apiMocker.submitMockConfig(null) }"
      @submit="apiMocker.submitMockConfig"
    />
  </div>
</template>
