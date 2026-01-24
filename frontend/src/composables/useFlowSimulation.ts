import { reactive, computed, type Ref } from 'vue'
import type {
  FlowStep,
  FlowData,
  SimulationState,
  SimulationMessage,
  ExecutionLogType,
  ButtonConfig,
  UserInput
} from '@/types/flow-preview'
import { useConditionEvaluator } from './useConditionEvaluator'
import { useFlowHistory } from './useFlowHistory'
import { useApiMocker } from './useApiMocker'

function generateId(): string {
  return Math.random().toString(36).substring(2, 9)
}

function delay(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}

export function useFlowSimulation(
  steps: Ref<FlowStep[]>,
  flowData: Ref<Partial<FlowData>>
) {
  const { evaluateCondition, interpolateVariables, validateInput } = useConditionEvaluator()
  const { saveSnapshot, undo: historyUndo, clearHistory, history, historyIndex, canUndo } = useFlowHistory()
  const apiMocker = useApiMocker()

  // Simulation state
  const state = reactive<SimulationState>({
    mode: 'edit',
    status: 'idle',
    currentStepIndex: null,
    currentStepName: null,
    variables: {},
    messages: [],
    history: [],
    historyIndex: -1,
    currentRetryCount: 0,
    executionLog: [],
    apiMocks: {}
  })

  // Computed properties
  const currentStep = computed(() => {
    if (state.currentStepIndex === null || state.currentStepIndex >= steps.value.length) {
      return null
    }
    return steps.value[state.currentStepIndex]
  })

  const isWaitingForInput = computed(() => {
    return state.status === 'waiting_input'
  })

  const expectedInputType = computed(() => {
    if (!currentStep.value) return null
    if (currentStep.value.message_type === 'buttons') return 'button'
    if (currentStep.value.input_type !== 'none') return currentStep.value.input_type
    return null
  })

  // Logging
  function log(type: ExecutionLogType, stepName?: string, details: Record<string, any> = {}): void {
    state.executionLog.push({
      id: generateId(),
      timestamp: new Date(),
      type,
      stepName,
      details
    })

    // Limit log size
    if (state.executionLog.length > 200) {
      state.executionLog = state.executionLog.slice(-200)
    }
  }

  // Message handling
  function addMessage(
    type: SimulationMessage['type'],
    content: string,
    options: Partial<SimulationMessage> = {}
  ): void {
    state.messages.push({
      id: generateId(),
      type,
      content,
      timestamp: new Date(),
      ...options
    })
  }

  // Variable handling
  function setVariable(key: string, value: any): void {
    state.variables[key] = value
    log('variable_set', state.currentStepName || undefined, { key, value })
  }

  // Find step by name
  function findStepByName(stepName: string): FlowStep | undefined {
    return steps.value.find(s => s.step_name === stepName)
  }

  function findStepIndex(stepName: string): number {
    return steps.value.findIndex(s => s.step_name === stepName)
  }

  // Start simulation
  async function startSimulation(): Promise<void> {
    if (steps.value.length === 0) {
      state.status = 'error'
      state.errorMessage = 'No steps defined in flow'
      return
    }

    // Reset state
    state.status = 'running'
    state.variables = {}
    state.messages = []
    state.executionLog = []
    state.currentRetryCount = 0
    clearHistory()

    log('flow_start', undefined, { stepsCount: steps.value.length })

    // Show initial message if configured
    if (flowData.value.initial_message) {
      addMessage('bot', flowData.value.initial_message)
      await delay(300)
    }

    // Start from first step
    const firstStep = steps.value[0]
    state.currentStepIndex = 0
    state.currentStepName = firstStep.step_name

    await processStep(firstStep)
  }

  // Process a step
  async function processStep(step: FlowStep): Promise<void> {
    log('step_enter', step.step_name, { messageType: step.message_type, inputType: step.input_type })

    // Save snapshot before processing (for undo)
    saveSnapshot(
      state.currentStepIndex!,
      step.step_name,
      state.variables,
      state.messages,
      state.currentRetryCount
    )

    // Check skip condition
    if (step.skip_condition && step.skip_condition.trim()) {
      const shouldSkip = evaluateCondition(step.skip_condition, state.variables)
      log('condition_eval', step.step_name, {
        condition: step.skip_condition,
        result: shouldSkip,
        type: 'skip'
      })

      if (shouldSkip) {
        await moveToNextStep(step)
        return
      }
    }

    // Process based on message type
    let messageContent = step.message

    if (step.message_type === 'api_fetch') {
      messageContent = await processApiStep(step)
    }

    // Interpolate variables
    messageContent = interpolateVariables(messageContent, state.variables)

    // Add bot message
    addMessage('bot', messageContent || 'No message configured', {
      stepName: step.step_name,
      buttons: step.message_type === 'buttons' ? step.buttons : undefined,
      inputType: step.input_type !== 'none' ? step.input_type : undefined,
      inputConfig: step.input_config,
      isApiMessage: step.message_type === 'api_fetch'
    })

    // Handle transfer type
    if (step.message_type === 'transfer') {
      const teamName = step.transfer_config.team_id === '_general'
        ? 'General Queue'
        : step.transfer_config.team_id || 'Team'

      addMessage('system', `Conversation transferred to ${teamName}`)
      log('flow_complete', step.step_name, { reason: 'transfer' })
      state.status = 'completed'
      return
    }

    // Determine if we need user input
    const needsInput = step.input_type !== 'none' || step.message_type === 'buttons' || step.message_type === 'whatsapp_flow'

    if (needsInput) {
      state.status = 'waiting_input'
    } else {
      // Auto-advance after short delay
      await delay(500)
      await moveToNextStep(step)
    }
  }

  // Process API step
  async function processApiStep(step: FlowStep): Promise<string> {
    log('api_call', step.step_name, {
      url: step.api_config.url,
      method: step.api_config.method
    })

    addMessage('system', `Calling API: ${step.api_config.method} ${step.api_config.url}`)

    const result = await apiMocker.executeMockedApiCall(step, state.variables)

    if (result.success && result.data) {
      // Extract variables from response
      const extracted = apiMocker.extractVariablesFromResponse(
        result.data,
        step.api_config.response_mapping || {}
      )

      for (const [key, value] of Object.entries(extracted)) {
        setVariable(key, value)
      }

      addMessage('debug', `API Response (${result.duration}ms): ${JSON.stringify(result.data)}`)

      // Use message template with extracted variables
      return interpolateVariables(step.message, { ...state.variables, ...extracted })
    } else {
      addMessage('debug', `API Error: ${result.error}`)
      return step.api_config.fallback_message || step.message || 'Sorry, there was an error fetching data.'
    }
  }

  // Process user input
  async function processUserInput(input: UserInput): Promise<void> {
    if (state.status !== 'waiting_input' || !currentStep.value) {
      return
    }

    const step = currentStep.value

    if (typeof input === 'string') {
      // Text input
      // Validate input if pattern exists
      if (step.validation_regex && !validateInput(input, step.validation_regex)) {
        handleValidationError(step)
        return
      }

      log('validation_pass', step.step_name, { input })

      // Store value
      if (step.store_as) {
        setVariable(step.store_as, input)
      }

      // Add user message
      addMessage('user', input)

      state.currentRetryCount = 0
      await moveToNextStep(step)
    } else {
      // Button click
      const button = input as ButtonConfig
      addMessage('user', button.title)

      if (step.store_as) {
        setVariable(step.store_as, button.id)
      }

      log('branch', step.step_name, { buttonId: button.id, buttonTitle: button.title })

      state.currentRetryCount = 0
      await moveToNextStep(step, button.id)
    }
  }

  // Handle WhatsApp Flow completion
  async function processWhatsAppFlowCompletion(data: Record<string, any>): Promise<void> {
    if (state.status !== 'waiting_input' || !currentStep.value) {
      return
    }

    const step = currentStep.value

    addMessage('user', 'Form completed')
    addMessage('debug', `Form data: ${JSON.stringify(data)}`)

    // Store all form data
    for (const [key, value] of Object.entries(data)) {
      setVariable(key, value)
    }

    if (step.store_as) {
      setVariable(step.store_as, data)
    }

    await moveToNextStep(step)
  }

  // Handle validation error
  function handleValidationError(step: FlowStep): void {
    state.currentRetryCount++
    log('validation_fail', step.step_name, {
      retryCount: state.currentRetryCount,
      maxRetries: step.max_retries
    })

    if (step.retry_on_invalid && state.currentRetryCount < step.max_retries) {
      addMessage('bot', step.validation_error || 'Invalid input. Please try again.', {
        isValidationError: true
      })
    } else {
      addMessage('system', 'Maximum retries reached. Moving to next step.')
      state.currentRetryCount = 0
      moveToNextStep(step)
    }
  }

  // Move to next step
  async function moveToNextStep(currentStepObj: FlowStep, buttonId?: string): Promise<void> {
    let nextStepName: string | null = null

    // Check conditional routing first (for button selections)
    if (buttonId && currentStepObj.conditional_next && currentStepObj.conditional_next[buttonId]) {
      nextStepName = currentStepObj.conditional_next[buttonId]
      log('branch', currentStepObj.step_name, { type: 'conditional', buttonId, nextStep: nextStepName })
    } else if (currentStepObj.next_step) {
      nextStepName = currentStepObj.next_step
      log('branch', currentStepObj.step_name, { type: 'explicit', nextStep: nextStepName })
    } else {
      // Default: next in order
      const currentIndex = findStepIndex(currentStepObj.step_name)
      if (currentIndex < steps.value.length - 1) {
        nextStepName = steps.value[currentIndex + 1].step_name
        log('branch', currentStepObj.step_name, { type: 'sequential', nextStep: nextStepName })
      }
    }

    log('step_exit', currentStepObj.step_name)

    if (nextStepName) {
      const nextStep = findStepByName(nextStepName)
      if (nextStep) {
        state.currentStepName = nextStepName
        state.currentStepIndex = findStepIndex(nextStepName)
        state.currentRetryCount = 0
        state.status = 'running'

        await delay(300)
        await processStep(nextStep)
        return
      }
    }

    // No next step - flow completed
    completeFlow()
  }

  // Complete flow
  function completeFlow(): void {
    if (flowData.value.completion_message) {
      addMessage('bot', flowData.value.completion_message)
    }

    addMessage('system', 'Flow completed')
    log('flow_complete', state.currentStepName || undefined, { reason: 'end' })
    state.status = 'completed'
  }

  // Pause simulation
  function pauseSimulation(): void {
    if (state.status === 'running' || state.status === 'waiting_input') {
      state.status = 'paused'
    }
  }

  // Resume simulation
  function resumeSimulation(): void {
    if (state.status === 'paused') {
      if (currentStep.value) {
        const needsInput = currentStep.value.input_type !== 'none' ||
          currentStep.value.message_type === 'buttons' ||
          currentStep.value.message_type === 'whatsapp_flow'

        state.status = needsInput ? 'waiting_input' : 'running'
      }
    }
  }

  // Reset simulation
  function resetSimulation(): void {
    state.status = 'idle'
    state.currentStepIndex = null
    state.currentStepName = null
    state.variables = {}
    state.messages = []
    state.executionLog = []
    state.currentRetryCount = 0
    state.errorMessage = undefined
    clearHistory()
  }

  // Undo to previous state
  function undo(): boolean {
    const snapshot = historyUndo()
    if (!snapshot) return false

    state.currentStepIndex = snapshot.stepIndex
    state.currentStepName = snapshot.stepName
    state.variables = { ...snapshot.variables }
    state.messages = snapshot.messages.map(m => ({ ...m }))
    state.currentRetryCount = snapshot.retryCount

    // Determine if we need input at this step
    const step = steps.value[snapshot.stepIndex]
    if (step) {
      const needsInput = step.input_type !== 'none' ||
        step.message_type === 'buttons' ||
        step.message_type === 'whatsapp_flow'

      state.status = needsInput ? 'waiting_input' : 'running'
    }

    return true
  }

  // Step forward (for debugging)
  async function stepForward(): Promise<void> {
    if (state.status === 'paused' && currentStep.value) {
      state.status = 'running'
      await moveToNextStep(currentStep.value)
      if (state.status === 'running') {
        state.status = 'paused'
      }
    }
  }

  // Go to specific step (for debugging)
  async function goToStep(stepName: string): Promise<void> {
    const stepIndex = findStepIndex(stepName)
    if (stepIndex === -1) return

    state.currentStepIndex = stepIndex
    state.currentStepName = stepName
    state.status = 'running'

    await processStep(steps.value[stepIndex])
  }

  return {
    state,
    currentStep,
    isWaitingForInput,
    expectedInputType,
    history,
    historyIndex,
    canUndo,

    // Actions
    startSimulation,
    pauseSimulation,
    resumeSimulation,
    resetSimulation,
    processUserInput,
    processWhatsAppFlowCompletion,
    undo,
    stepForward,
    goToStep,
    setVariable,

    // API mocking
    apiMocker
  }
}
