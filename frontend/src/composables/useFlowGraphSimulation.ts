import { reactive, computed, ref, type Ref } from 'vue'
import type { ChatFlowGraph, ChatNode } from '@/services/api'
import type {
  FlowData,
  SimulationState,
  SimulationMessage,
  ExecutionLogType,
  ButtonConfig,
  UserInput,
} from '@/types/flow-preview'
import { useApiMocker } from './useApiMocker'

function generateId(): string {
  return Math.random().toString(36).substring(2, 9)
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

const TEMPLATE_RE = /\{\{\s*([^}]+?)\s*\}\}/g

// Lightweight `{{var}}` interpolation; matches the backend's processTemplate
// for the simple case (no helpers or filters). Sufficient for previewing the
// node-config message templates.
function interpolate(template: string, vars: Record<string, any>): string {
  if (!template) return ''
  return template.replace(TEMPLATE_RE, (_, expr: string) => {
    const path = expr.trim()
    const value = path.split('.').reduce<any>((acc, key) => (acc == null ? acc : acc[key]), vars)
    return value == null ? '' : String(value)
  })
}

/**
 * useFlowGraphSimulation walks a v2 ChatFlowGraph node by node, mirroring
 * the backend runChatGraph executor. It powers the in-browser flow preview
 * so authors can simulate exactly the runtime the server uses.
 *
 * The exposed shape intentionally matches the legacy useFlowSimulation
 * (state, currentStep, isWaitingForInput, expectedInputType, actions)
 * so InteractivePreview can swap composables with minimal churn.
 */
export function useFlowGraphSimulation(
  graph: Ref<ChatFlowGraph | null>,
  flowData: Ref<Partial<FlowData>>,
) {
  const apiMocker = useApiMocker()

  const state = reactive<SimulationState>({
    mode: 'preview',
    status: 'idle',
    currentStepIndex: null,
    currentStepName: null,
    variables: {},
    messages: [],
    history: [],
    historyIndex: -1,
    currentRetryCount: 0,
    executionLog: [],
    apiMocks: {},
  })

  // currentStepName actually holds the node id during graph runs. The
  // legacy bindings in InteractivePreview reference `state.currentStepName`
  // and `state.currentStepIndex` — we surface the equivalents below.
  const currentNode = computed<ChatNode | null>(() => {
    const g = graph.value
    if (!g || !state.currentStepName) return null
    return g.nodes.find((n) => n.id === state.currentStepName) || null
  })

  // Legacy alias used by InteractivePreview templates that reference
  // currentStep.message_type / .input_config / etc. We surface a thin
  // adapter so the existing template bindings keep rendering.
  const currentStep = computed(() => {
    const node = currentNode.value
    if (!node) return null
    return adaptNodeAsStep(node)
  })

  const isWaitingForInput = computed(() => state.status === 'waiting_input')

  const expectedInputType = computed<string | null>(() => {
    const node = currentNode.value
    if (!node) return null
    switch (node.type) {
      case 'buttons':
        return 'button'
      case 'whatsapp_flow':
        return 'whatsapp_flow'
      case 'prompt':
        return 'text'
      default:
        return null
    }
  })

  // History of state snapshots taken before each node executes, so the
  // user can step backwards through a simulation.
  const snapshots = ref<string[]>([])
  const canUndo = computed(() => snapshots.value.length > 0)

  function snapshot(): void {
    snapshots.value.push(JSON.stringify({
      status: state.status,
      currentStepIndex: state.currentStepIndex,
      currentStepName: state.currentStepName,
      variables: state.variables,
      messages: state.messages,
      executionLog: state.executionLog,
      currentRetryCount: state.currentRetryCount,
    }))
    if (snapshots.value.length > 50) snapshots.value.shift()
  }

  function log(type: ExecutionLogType, nodeId?: string, details: Record<string, any> = {}): void {
    state.executionLog.push({
      id: generateId(),
      timestamp: new Date(),
      type,
      stepName: nodeId,
      details,
    })
    if (state.executionLog.length > 200) {
      state.executionLog = state.executionLog.slice(-200)
    }
  }

  function addMessage(
    type: SimulationMessage['type'],
    content: string,
    options: Partial<SimulationMessage> = {},
  ): void {
    state.messages.push({
      id: generateId(),
      type,
      content,
      timestamp: new Date(),
      ...options,
    })
  }

  function setVariable(key: string, value: any): void {
    state.variables[key] = value
    log('variable_set', state.currentStepName || undefined, { key, value })
  }

  // ---- Edge resolution ---------------------------------------------------

  function resolveEdge(fromId: string, outcome: string): string {
    const g = graph.value
    if (!g) return ''
    let fallback = ''
    for (const e of g.edges) {
      if (e.from !== fromId) continue
      if (e.condition === outcome) return e.to
      if (e.condition === 'default') fallback = e.to
    }
    return fallback
  }

  function nodeById(id: string): ChatNode | null {
    return graph.value?.nodes.find((n) => n.id === id) || null
  }

  // ---- Lifecycle ---------------------------------------------------------

  async function startSimulation(): Promise<void> {
    const g = graph.value
    if (!g || g.nodes.length === 0) {
      state.status = 'error'
      state.errorMessage = 'Flow has no v2 graph'
      return
    }

    state.status = 'running'
    // Seed built-in template variables so preview matches what the
    // backend's runChatGraph will produce (phone_number is always
    // populated server-side from the session).
    state.variables = { phone_number: '+15555550100', contact_name: 'Preview User' }
    state.messages = []
    state.executionLog = []
    state.currentRetryCount = 0
    snapshots.value = []

    log('flow_start', undefined, { nodeCount: g.nodes.length })

    if (flowData.value.initial_message) {
      addMessage('bot', flowData.value.initial_message)
      await delay(300)
    }

    state.currentStepName = g.entry_node
    state.currentStepIndex = g.nodes.findIndex((n) => n.id === g.entry_node)
    if (state.currentStepIndex < 0) state.currentStepIndex = 0
    await runFrom(g.entry_node)
  }

  // runFrom keeps stepping through non-yielding nodes until it hits a
  // yielding one (buttons / prompt / whatsapp_flow / end / transfer) or
  // walks off the graph.
  async function runFrom(startId: string): Promise<void> {
    let currentId = startId
    let safety = 100

    while (currentId && safety-- > 0) {
      const node = nodeById(currentId)
      if (!node) {
        addMessage('system', `Node "${currentId}" not found`)
        state.status = 'error'
        return
      }
      // Snapshot before executing this node so Undo can rewind to it.
      snapshot()
      state.currentStepName = currentId
      state.currentStepIndex = graph.value?.nodes.findIndex((n) => n.id === currentId) ?? null
      log('step_enter', currentId, { type: node.type })

      const outcome = await execute(node)
      if (outcome === '__yield__') return
      if (outcome === '__end__') {
        complete()
        return
      }

      const next = resolveEdge(currentId, outcome || 'default')
      log('step_exit', currentId, { outcome, next })
      if (!next) {
        complete()
        return
      }
      currentId = next
      await delay(150)
    }

    if (safety <= 0) {
      addMessage('system', 'Aborted: too many non-blocking nodes (cycle?)')
      state.status = 'error'
    }
  }

  // ---- Per-node executors ------------------------------------------------

  async function execute(node: ChatNode): Promise<string> {
    // Universal pre-step: node.config.set (matches backend applyNodeSetConfig).
    const set = node.config?.set as Record<string, any> | undefined
    if (set && typeof set === 'object') {
      for (const [k, v] of Object.entries(set)) {
        const value = typeof v === 'string' ? interpolate(v, state.variables) : v
        setVariable(k, value)
      }
    }

    switch (node.type) {
      case 'start':
        // Entry sentinel — no side effect; advance through the default edge,
        // matching the backend runner's ChatNodeStart handling.
        return 'default'
      case 'message':
        return execMessage(node)
      case 'buttons':
        return execButtons(node)
      case 'end':
        return execEnd(node)
      case 'condition':
        return execCondition(node)
      case 'timing':
        return execTiming(node)
      case 'prompt':
        return execPrompt(node)
      case 'set_variable':
        // The pre-step already applied; nothing more to do.
        return 'default'
      case 'transfer':
        return execTransfer(node)
      case 'api_call':
        return execApiCall(node)
      case 'whatsapp_flow':
        return execWhatsAppFlow(node)
      case 'goto_flow':
        // In a single-flow preview we can't actually jump; show a system note.
        addMessage('system', `[goto_flow] would jump to flow ${node.config?.flow_id || '?'}`)
        return '__end__'
      case 'ai_response':
        addMessage('system', '[ai_response] simulated — backend AI is not invoked in preview')
        return 'default'
      case 'webhook':
        addMessage('system', '[webhook] simulated — request not sent in preview')
        return 'default'
      default:
        addMessage('system', `Unknown node type "${node.type}"`)
        return '__end__'
    }
  }

  function execMessage(node: ChatNode): string {
    const text = interpolate(stringField(node, 'message', 'text'), state.variables)
    if (text) {
      addMessage('bot', text, { stepName: node.id })
    }
    return 'default'
  }

  function execButtons(node: ChatNode): string {
    const body = interpolate(stringField(node, 'body', 'message', 'text') || node.label, state.variables)
    const buttons = (node.config?.buttons as ButtonConfig[] | undefined) || []
    addMessage('bot', body, { stepName: node.id, buttons })
    state.status = 'waiting_input'
    return '__yield__'
  }

  function execEnd(node: ChatNode): string {
    const text = interpolate(stringField(node, 'message'), state.variables)
    if (text) addMessage('bot', text, { stepName: node.id })
    return '__end__'
  }

  function execPrompt(node: ChatNode): string {
    const body = interpolate(stringField(node, 'body', 'message', 'text'), state.variables)
    if (body) addMessage('bot', body, { stepName: node.id, inputType: 'text' })
    state.status = 'waiting_input'
    return '__yield__'
  }

  function execTransfer(node: ChatNode): string {
    const body = interpolate(stringField(node, 'body', 'message', 'text'), state.variables)
    if (body) addMessage('bot', body, { stepName: node.id })
    const teamID = stringField(node, 'team_id') || '_general'
    const label = teamID === '_general' ? 'General Queue' : teamID
    addMessage('system', `Conversation transferred to ${label}`)
    log('flow_complete', node.id, { reason: 'transfer' })
    state.status = 'completed'
    return '__yield__'
  }

  function execCondition(node: ChatNode): string {
    const expression = stringField(node, 'expression')
    if (!expression) return 'false'
    const result = evalCondition(expression, state.variables)
    log('condition_eval', node.id, { expression, result })
    return result ? 'true' : 'false'
  }

  function execTiming(node: ChatNode): string {
    const schedule = (node.config?.schedule as any[] | undefined) || []
    const now = new Date()
    const dayName = ['sunday', 'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday'][now.getDay()]
    for (const entry of schedule) {
      if (!entry || typeof entry !== 'object') continue
      if (String(entry.day).toLowerCase() !== dayName) continue
      if (!entry.enabled) return 'out_of_hours'
      const nowMinutes = now.getHours() * 60 + now.getMinutes()
      const [sh, sm] = String(entry.start_time || '00:00').split(':').map(Number)
      const [eh, em] = String(entry.end_time || '23:59').split(':').map(Number)
      const startMin = (sh || 0) * 60 + (sm || 0)
      const endMin = (eh || 23) * 60 + (em || 59)
      return nowMinutes >= startMin && nowMinutes < endMin ? 'in_hours' : 'out_of_hours'
    }
    return 'out_of_hours'
  }

  async function execApiCall(node: ChatNode): Promise<string> {
    log('api_call', node.id, { url: node.config?.url, method: node.config?.method })
    addMessage('system', `Calling API: ${node.config?.method || 'GET'} ${node.config?.url || ''}`)

    const fakeStep: any = {
      step_name: node.id,
      message: stringField(node, 'message_template') || '',
      api_config: {
        url: stringField(node, 'url'),
        method: stringField(node, 'method') || 'GET',
        headers: (node.config?.headers as Record<string, string>) || {},
        body: stringField(node, 'body'),
        response_mapping: (node.config?.response_mapping as Record<string, string>) || {},
        fallback_message: stringField(node, 'fallback_message'),
      },
    }

    const result = await apiMocker.executeMockedApiCall(fakeStep, state.variables)
    if (result.success && result.data) {
      const extracted = apiMocker.extractVariablesFromResponse(
        result.data,
        fakeStep.api_config.response_mapping || {},
      )
      for (const [k, v] of Object.entries(extracted)) setVariable(k, v)
      addMessage('debug', `API Response (${result.duration}ms): ${JSON.stringify(result.data)}`)
      const template = stringField(node, 'message_template')
      if (template) {
        const rendered = interpolate(template, { ...state.variables, ...extracted })
        if (rendered) addMessage('bot', rendered, { stepName: node.id, isApiMessage: true })
      }
      return 'http:2xx'
    }
    addMessage('debug', `API Error: ${result.error || 'request failed'}`)
    const fb = stringField(node, 'fallback_message')
    if (fb) addMessage('bot', fb, { stepName: node.id, isApiMessage: true })
    return 'http:non2xx'
  }

  function execWhatsAppFlow(node: ChatNode): string {
    const body = interpolate(stringField(node, 'body', 'message', 'text'), state.variables)
    if (body) {
      addMessage('bot', body, {
        stepName: node.id,
        inputConfig: {
          flow_id: stringField(node, 'flow_id'),
          flow_cta: stringField(node, 'cta'),
          flow_header: stringField(node, 'header'),
        },
      })
    }
    state.status = 'waiting_input'
    return '__yield__'
  }

  // ---- Inputs from the UI ------------------------------------------------

  async function processUserInput(input: UserInput): Promise<void> {
    if (state.status !== 'waiting_input' || !state.currentStepName) return
    const node = nodeById(state.currentStepName)
    if (!node) return

    if (typeof input === 'string') {
      addMessage('user', input)
      if (node.type === 'prompt') {
        const regex = stringField(node, 'validation_regex')
        if (regex) {
          try {
            if (!new RegExp(regex).test(input)) {
              const max = (node.config?.max_retries as number) || 3
              state.currentRetryCount++
              if (state.currentRetryCount < max) {
                addMessage('bot', stringField(node, 'validation_error') || 'Invalid input. Please try again.', {
                  isValidationError: true,
                })
                return
              }
              state.currentRetryCount = 0
              await advance(node, 'max_retries')
              return
            }
          } catch {
            // bad regex: skip validation
          }
        }
        const storeAs = stringField(node, 'store_as')
        if (storeAs) setVariable(storeAs, input)
        state.currentRetryCount = 0
        await advance(node, 'default')
        return
      }
      // Treat free-text into a buttons node as no-op (re-yield).
      state.status = 'waiting_input'
    } else {
      const btn = input
      addMessage('user', btn.title)
      log('branch', node.id, { buttonId: btn.id })
      await advance(node, `button:${btn.id}`)
    }
  }

  async function processWhatsAppFlowCompletion(data: Record<string, any>): Promise<void> {
    if (state.status !== 'waiting_input' || !state.currentStepName) return
    const node = nodeById(state.currentStepName)
    if (!node || node.type !== 'whatsapp_flow') return
    addMessage('user', 'Form completed')
    addMessage('debug', `Form data: ${JSON.stringify(data)}`)
    for (const [k, v] of Object.entries(data)) setVariable(k, v)
    await advance(node, 'default')
  }

  async function advance(node: ChatNode, outcome: string): Promise<void> {
    const next = resolveEdge(node.id, outcome)
    log('step_exit', node.id, { outcome, next })
    if (!next) {
      complete()
      return
    }
    state.status = 'running'
    await delay(150)
    await runFrom(next)
  }

  function complete(): void {
    if (flowData.value.completion_message) {
      addMessage('bot', flowData.value.completion_message)
    }
    addMessage('system', 'Flow completed')
    log('flow_complete', state.currentStepName || undefined, { reason: 'end' })
    state.status = 'completed'
  }

  function pauseSimulation(): void {
    if (state.status === 'running' || state.status === 'waiting_input') {
      state.status = 'paused'
    }
  }

  function resumeSimulation(): void {
    if (state.status === 'paused') state.status = 'running'
  }

  function resetSimulation(): void {
    state.status = 'idle'
    state.currentStepIndex = null
    state.currentStepName = null
    state.variables = {}
    state.messages = []
    state.executionLog = []
    state.currentRetryCount = 0
    state.errorMessage = undefined
    snapshots.value = []
  }

  function undo(): boolean {
    const snap = snapshots.value.pop()
    if (!snap) return false
    const restored = JSON.parse(snap)
    state.status = 'paused'
    state.currentStepIndex = restored.currentStepIndex
    state.currentStepName = restored.currentStepName
    state.variables = restored.variables
    state.messages = (restored.messages || []).map((m: any) => ({
      ...m,
      timestamp: new Date(m.timestamp),
    }))
    state.executionLog = (restored.executionLog || []).map((e: any) => ({
      ...e,
      timestamp: new Date(e.timestamp),
    }))
    state.currentRetryCount = restored.currentRetryCount || 0
    state.errorMessage = undefined
    return true
  }

  async function stepForward(): Promise<void> {
    if (state.status !== 'paused' || !state.currentStepName) return
    state.status = 'running'
    await runFrom(state.currentStepName)
  }

  async function goToStep(nodeId: string): Promise<void> {
    if (!nodeById(nodeId)) return
    state.currentStepName = nodeId
    state.status = 'running'
    await runFrom(nodeId)
  }

  return {
    state,
    currentStep,
    currentNode,
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
    setVariable,
    apiMocker,
  }
}

// adaptNodeAsStep surfaces enough FlowStep-shaped fields that the existing
// InteractivePreview template bindings (currentStep.message_type, etc.)
// keep working without changes. Read-only — mutations bypass this adapter.
function adaptNodeAsStep(node: ChatNode): Record<string, any> {
  const cfg = node.config || {}
  const messageType = nodeTypeToMessageType(node.type)
  const out: Record<string, any> = {
    step_name: node.id,
    message_type: messageType,
    message: stringFromConfig(cfg, 'message', 'body', 'text'),
    input_type: messageType === 'prompt' ? 'text' : 'none',
    buttons: (cfg.buttons as any[]) || [],
    input_config: {
      flow_cta: stringFromConfig(cfg, 'cta'),
      flow_id: stringFromConfig(cfg, 'flow_id'),
      flow_header: stringFromConfig(cfg, 'header'),
    },
    transfer_config: {
      team_id: stringFromConfig(cfg, 'team_id'),
      notes: stringFromConfig(cfg, 'notes'),
    },
  }
  return out
}

function nodeTypeToMessageType(t: string): string {
  switch (t) {
    case 'message':
      return 'text'
    case 'buttons':
      return 'buttons'
    case 'api_call':
      return 'api_fetch'
    case 'whatsapp_flow':
      return 'whatsapp_flow'
    case 'transfer':
      return 'transfer'
    case 'end':
      return 'end'
    case 'prompt':
      return 'prompt'
    default:
      return t
  }
}

function stringField(node: ChatNode, ...keys: string[]): string {
  return stringFromConfig(node.config || {}, ...keys)
}

function stringFromConfig(cfg: Record<string, any>, ...keys: string[]): string {
  for (const k of keys) {
    if (typeof cfg[k] === 'string' && cfg[k] !== '') return cfg[k]
  }
  return ''
}

// evalCondition runs the expression in a sandboxed Function call with the
// session variables bound. Mirrors the backend's expr-lang/expr semantics
// well enough for the most common operators (==, !=, &&, ||, !, parens).
// Falls back to false on any compile/runtime error.
function evalCondition(expression: string, vars: Record<string, any>): boolean {
  try {
    // Translate expr-lang keywords to JS equivalents for client-side eval.
    const js = expression
      .replace(/\band\b/gi, '&&')
      .replace(/\bor\b/gi, '||')
      .replace(/\bnot\b/gi, '!')
      .replace(/\bcontains\b/gi, '.includes')
    const keys = Object.keys(vars)
    // eslint-disable-next-line @typescript-eslint/no-implied-eval, no-new-func
    const fn = new Function(...keys, `try { return !!(${js}); } catch (_) { return false; }`)
    return !!fn(...keys.map((k) => vars[k]))
  } catch {
    return false
  }
}
