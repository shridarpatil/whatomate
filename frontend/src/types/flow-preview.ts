// Types for Interactive Flow Preview

export interface ButtonConfig {
  id: string
  title: string
  type?: 'reply' | 'url' | 'phone'
  url?: string
  phone_number?: string
}

export interface ApiConfig {
  url: string
  method: string
  headers: Record<string, string>
  body: string
  fallback_message: string
  response_mapping: Record<string, string>
}

export interface TransferConfig {
  team_id: string
  notes: string
}

export interface FlowStep {
  id?: string
  step_name: string
  step_order: number
  message: string
  message_type: 'text' | 'buttons' | 'api_fetch' | 'whatsapp_flow' | 'transfer'
  input_type: 'none' | 'text' | 'number' | 'email' | 'phone' | 'date' | 'select'
  input_config: Record<string, any>
  api_config: ApiConfig
  buttons: ButtonConfig[]
  transfer_config: TransferConfig
  validation_regex: string
  validation_error: string
  store_as: string
  next_step: string
  conditional_next?: Record<string, string>
  retry_on_invalid: boolean
  max_retries: number
  skip_condition: string
}

export interface FlowData {
  name: string
  description: string
  trigger_keywords: string
  initial_message: string
  completion_message: string
  on_complete_action: 'none' | 'webhook'
  completion_config: Record<string, any>
  enabled: boolean
  steps: FlowStep[]
}

// Simulation Types

export type SimulationStatus = 'idle' | 'running' | 'paused' | 'waiting_input' | 'completed' | 'error'
export type MessageType = 'bot' | 'user' | 'system' | 'debug'

export interface SimulationMessage {
  id: string
  type: MessageType
  content: string
  timestamp: Date
  stepName?: string
  buttons?: ButtonConfig[]
  inputType?: string
  inputConfig?: Record<string, any>
  isValidationError?: boolean
  isApiMessage?: boolean
}

export interface SimulationSnapshot {
  stepIndex: number
  stepName: string
  variables: Record<string, any>
  messages: SimulationMessage[]
  retryCount: number
  timestamp: Date
}

export type ExecutionLogType =
  | 'flow_start'
  | 'step_enter'
  | 'step_exit'
  | 'variable_set'
  | 'condition_eval'
  | 'api_call'
  | 'validation_pass'
  | 'validation_fail'
  | 'branch'
  | 'flow_complete'
  | 'flow_error'

export interface ExecutionLogEntry {
  id: string
  timestamp: Date
  type: ExecutionLogType
  stepName?: string
  details: Record<string, any>
}

export interface MockApiResponse {
  stepName: string
  url: string
  statusCode: number
  responseBody: Record<string, any>
  delay: number
}

export interface SimulationState {
  mode: 'edit' | 'preview'
  status: SimulationStatus
  currentStepIndex: number | null
  currentStepName: string | null

  // Variable storage
  variables: Record<string, any>

  // Conversation history
  messages: SimulationMessage[]

  // History for undo
  history: SimulationSnapshot[]
  historyIndex: number

  // Retry tracking
  currentRetryCount: number

  // Execution log
  executionLog: ExecutionLogEntry[]

  // API Mock configuration
  apiMocks: Record<string, MockApiResponse>

  // Error info
  errorMessage?: string
}

// Utility type for user input
export type UserInput = string | ButtonConfig
