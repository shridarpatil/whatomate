<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Card, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Switch } from '@/components/ui/switch'
import { Separator } from '@/components/ui/separator'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { chatbotService, flowsService, teamsService, type Team } from '@/services/api'
import { toast } from 'vue-sonner'
import {
  ArrowLeft,
  Plus,
  Trash2,
  GripVertical,
  MessageSquare,
  MousePointerClick,
  Globe,
  MessageCircle,
  Users,
  ChevronDown,
  ChevronRight,
  Save,
  Settings,
  ExternalLink,
  Reply,
} from 'lucide-vue-next'
import draggable from 'vuedraggable'
import FlowChart from '@/components/chatbot/flow-builder/FlowChart.vue'
import FlowPreviewPanel from '@/components/chatbot/flow-preview/FlowPreviewPanel.vue'

interface ApiConfig {
  url: string
  method: string
  headers: Record<string, string>
  body: string
  fallback_message: string
  response_mapping: Record<string, string>
}

interface ButtonConfig {
  id: string
  title: string
  type?: 'reply' | 'url'
  url?: string
}

interface TransferConfig {
  team_id: string
  notes: string
}

interface FlowStep {
  id?: string
  step_name: string
  step_order: number
  message: string
  message_type: string
  input_type: string
  input_config: Record<string, any>
  api_config: ApiConfig
  buttons: ButtonConfig[]
  transfer_config: TransferConfig
  validation_regex: string
  validation_error: string
  store_as: string
  next_step: string
  conditional_next?: Record<string, string>  // Button ID -> target step name
  retry_on_invalid: boolean
  max_retries: number
  skip_condition: string
}

interface WebhookConfig {
  url: string
  method: string
  headers: Record<string, string>
  body: string
}

interface PanelFieldConfig {
  key: string
  label: string
  order: number
  display_type?: string
  color?: string
}

interface PanelSection {
  id: string
  label: string
  columns: number
  collapsible: boolean
  default_collapsed: boolean
  order: number
  fields: PanelFieldConfig[]
}

interface PanelConfig {
  sections: PanelSection[]
}

interface WhatsAppFlow {
  id: string
  name: string
  status: string
  meta_flow_id: string
}

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const isLoading = ref(true)
const isSaving = ref(false)
const flowId = computed(() => route.params.id as string | undefined)
const isNewFlow = computed(() => !flowId.value || flowId.value === 'new')

const whatsappFlows = ref<WhatsAppFlow[]>([])
const teams = ref<Team[]>([])

const selectedStepIndex = ref<number | null>(null)
const showFlowSettings = ref(false)
const previewMode = ref<'edit' | 'preview'>('edit')
const deleteStepDialogOpen = ref(false)
const stepToDeleteIndex = ref<number | null>(null)
const hasUnsavedChanges = ref(false)
const cancelDialogOpen = ref(false)
const webhookHeadersOpen = ref(false)
const listPickerOpen = ref(false)

// Panel resize
const propertiesPanelWidth = ref(500)
const stepsPanelWidth = ref(400)
const isResizingRight = ref(false)
const isResizingLeft = ref(false)
const minPanelWidth = 200
const maxPanelWidth = 500
const minStepsPanelWidth = 200
const maxStepsPanelWidth = 400

function startResizeRight(_e: MouseEvent) {
  isResizingRight.value = true
  document.addEventListener('mousemove', handleResizeRight)
  document.addEventListener('mouseup', stopResizeRight)
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
}

function handleResizeRight(e: MouseEvent) {
  if (!isResizingRight.value) return
  const newWidth = window.innerWidth - e.clientX
  propertiesPanelWidth.value = Math.min(Math.max(newWidth, minPanelWidth), maxPanelWidth)
}

function stopResizeRight() {
  isResizingRight.value = false
  document.removeEventListener('mousemove', handleResizeRight)
  document.removeEventListener('mouseup', stopResizeRight)
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
}

function startResizeLeft(_e: MouseEvent) {
  isResizingLeft.value = true
  document.addEventListener('mousemove', handleResizeLeft)
  document.addEventListener('mouseup', stopResizeLeft)
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
}

function handleResizeLeft(e: MouseEvent) {
  if (!isResizingLeft.value) return
  const newWidth = e.clientX
  stepsPanelWidth.value = Math.min(Math.max(newWidth, minStepsPanelWidth), maxStepsPanelWidth)
}

function stopResizeLeft() {
  isResizingLeft.value = false
  document.removeEventListener('mousemove', handleResizeLeft)
  document.removeEventListener('mouseup', stopResizeLeft)
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
}

// Collapsible states for properties panel
const messagesOpen = ref(true)
const inputOpen = ref(true)
const validationOpen = ref(true)
const advancedOpen = ref(false)
const panelConfigOpen = ref(false)

const defaultApiConfig: ApiConfig = {
  url: '',
  method: 'GET',
  headers: {},
  body: '',
  fallback_message: '',
  response_mapping: {}
}

const defaultTransferConfig: TransferConfig = {
  team_id: '_general',
  notes: ''
}

const defaultWebhookConfig: WebhookConfig = {
  url: '',
  method: 'POST',
  headers: {},
  body: ''
}

const defaultStep: FlowStep = {
  step_name: '',
  step_order: 0,
  message: '',
  message_type: 'text',
  input_type: 'text',
  input_config: {},
  api_config: { ...defaultApiConfig },
  buttons: [],
  transfer_config: { ...defaultTransferConfig },
  validation_regex: '',
  validation_error: 'Invalid input. Please try again.',
  store_as: '',
  next_step: '',
  conditional_next: {},
  retry_on_invalid: true,
  max_retries: 3,
  skip_condition: ''
}

const formData = ref({
  name: '',
  description: '',
  trigger_keywords: '',
  initial_message: 'Hi! Let me help you with that.',
  completion_message: 'Thank you! We have all the information we need.',
  on_complete_action: 'none',
  completion_config: { ...defaultWebhookConfig },
  panel_config: { sections: [] } as PanelConfig,
  enabled: true,
  steps: [] as FlowStep[]
})

const selectedStep = computed(() => {
  if (selectedStepIndex.value === null || selectedStepIndex.value >= formData.value.steps.length) {
    return null
  }
  return formData.value.steps[selectedStepIndex.value]
})

// All steps with valid names for branching dropdowns
const stepsWithNames = computed(() => {
  return formData.value.steps.filter(s => s.step_name && s.step_name.trim() !== '')
})

// Extract available variables for panel configuration
const availableVariables = computed(() => {
  const variables: { key: string; source: string; stepName: string }[] = []

  for (const step of formData.value.steps) {
    // Add store_as variables
    if (step.store_as && step.store_as.trim()) {
      variables.push({
        key: step.store_as.trim(),
        source: 'StoreAs',
        stepName: step.step_name || 'Unknown'
      })
    }

    // Add response_mapping variables from api_fetch steps
    if (step.message_type === 'api_fetch' && step.api_config?.response_mapping) {
      for (const key of Object.keys(step.api_config.response_mapping)) {
        if (key && key.trim()) {
          variables.push({
            key: key.trim(),
            source: 'Response Mapping',
            stepName: step.step_name || 'Unknown'
          })
        }
      }
    }
  }

  return variables
})

// Variables already assigned to panel sections
const assignedVariables = computed(() => {
  const assigned = new Set<string>()
  for (const section of formData.value.panel_config.sections) {
    for (const field of section.fields) {
      assigned.add(field.key)
    }
  }
  return assigned
})

// Variables not yet assigned to any section
const unassignedVariables = computed(() => {
  return availableVariables.value.filter(v => !assignedVariables.value.has(v.key))
})

const messageTypes = computed(() => [
  { value: 'text', label: t('flowBuilder.messageTypeText'), icon: MessageSquare },
  { value: 'buttons', label: t('flowBuilder.messageTypeButtons'), icon: MousePointerClick },
  { value: 'api_fetch', label: t('flowBuilder.messageTypeApi'), icon: Globe },
  { value: 'whatsapp_flow', label: t('flowBuilder.messageTypeWhatsappFlow'), icon: MessageCircle },
  { value: 'transfer', label: t('flowBuilder.messageTypeTransfer'), icon: Users }
])

const inputTypes = computed(() => [
  { value: 'none', label: t('flowBuilder.noInputRequired') },
  { value: 'text', label: t('flowBuilder.textInput') },
  { value: 'number', label: t('flowBuilder.numberInput') },
  { value: 'email', label: t('flowBuilder.emailInput') },
  { value: 'phone', label: t('flowBuilder.phoneInput') },
  { value: 'date', label: t('flowBuilder.dateInput') },
  { value: 'select', label: t('flowBuilder.selectionInput') }
])

const httpMethods = ['GET', 'POST', 'PUT', 'PATCH']

function getStepIcon(messageType: string) {
  const type = messageTypes.value.find(t => t.value === messageType)
  return type?.icon || MessageSquare
}

function getStepLabel(messageType: string) {
  const type = messageTypes.value.find(t => t.value === messageType)
  return type?.label || t('flowBuilder.messageTypeText')
}

// Watch for changes to mark unsaved
watch(formData, () => {
  hasUnsavedChanges.value = true
}, { deep: true })

onMounted(async () => {
  await Promise.all([fetchWhatsAppFlows(), fetchTeams()])

  if (!isNewFlow.value && flowId.value) {
    await loadFlow(flowId.value)
  } else {
    // Initialize with one default step
    formData.value.steps = [{
      ...defaultStep,
      step_name: 'step_1',
      step_order: 1,
      message: 'What is your name?',
      store_as: 'name'
    }]
    isLoading.value = false
  }
  // Default to Flow Settings view
  showFlowSettings.value = true
  selectedStepIndex.value = null
  hasUnsavedChanges.value = false
})

async function fetchWhatsAppFlows() {
  try {
    const response = await flowsService.list()
    const data = response.data.data || response.data
    const allFlows = data.flows || []
    whatsappFlows.value = allFlows.filter(
      (f: WhatsAppFlow) => f.meta_flow_id && f.status?.toUpperCase() === 'PUBLISHED'
    )
  } catch (error) {
    console.error('Failed to load WhatsApp flows:', error)
    whatsappFlows.value = []
  }
}

async function fetchTeams() {
  try {
    const response = await teamsService.list()
    const data = (response.data as any).data || response.data
    teams.value = (data.teams || []).filter((t: Team) => t.is_active)
  } catch (error) {
    console.error('Failed to load teams:', error)
    teams.value = []
  }
}

async function loadFlow(id: string) {
  isLoading.value = true
  try {
    const response = await chatbotService.getFlow(id)
    const flow = response.data.data || response.data

    formData.value = {
      name: flow.name || flow.Name || '',
      description: flow.description || flow.Description || '',
      trigger_keywords: (flow.trigger_keywords || flow.TriggerKeywords || []).join(', '),
      initial_message: flow.initial_message || flow.InitialMessage || '',
      completion_message: flow.completion_message || flow.CompletionMessage || '',
      on_complete_action: flow.on_complete_action || flow.OnCompleteAction || 'none',
      completion_config: {
        ...defaultWebhookConfig,
        ...(flow.completion_config || flow.CompletionConfig || {}),
        headers: (flow.completion_config || flow.CompletionConfig || {}).headers || {}
      },
      panel_config: {
        sections: (flow.panel_config || flow.PanelConfig || {}).sections || []
      },
      enabled: flow.is_enabled ?? flow.IsEnabled ?? flow.enabled ?? true,
      steps: (flow.steps || flow.Steps || []).map((s: any, idx: number) => ({
        id: s.id || s.ID,
        step_name: s.step_name || s.StepName || `step_${idx + 1}`,
        step_order: s.step_order ?? s.StepOrder ?? idx + 1,
        message: s.message || s.Message || '',
        message_type: s.message_type || s.MessageType || 'text',
        input_type: s.input_type || s.InputType || 'text',
        input_config: s.input_config || s.InputConfig || {},
        api_config: {
          ...defaultApiConfig,
          ...(s.api_config || s.ApiConfig || {}),
          headers: (s.api_config || s.ApiConfig || {}).headers || {},
          response_mapping: (s.api_config || s.ApiConfig || {}).response_mapping || {}
        },
        buttons: s.buttons || s.Buttons || [],
        transfer_config: {
          ...defaultTransferConfig,
          ...(s.transfer_config || s.TransferConfig || {}),
          team_id: (s.transfer_config || s.TransferConfig || {}).team_id || '_general'
        },
        validation_regex: s.validation_regex || s.ValidationRegex || '',
        validation_error: s.validation_error || s.ValidationError || 'Invalid input. Please try again.',
        store_as: s.store_as || s.StoreAs || '',
        next_step: s.next_step || s.NextStep || '',
        conditional_next: s.conditional_next || s.ConditionalNext || {},
        retry_on_invalid: s.retry_on_invalid ?? s.RetryOnInvalid ?? true,
        max_retries: s.max_retries ?? s.MaxRetries ?? 3,
        skip_condition: s.skip_condition || s.SkipCondition || ''
      }))
    }

    // Flow Settings will be selected by default in onMounted
  } catch (error) {
    toast.error(t('common.failedLoad', { resource: t('resources.flow') }))
    router.push('/chatbot/flows')
  } finally {
    isLoading.value = false
  }
}

function addStep() {
  const newOrder = formData.value.steps.length + 1
  formData.value.steps.push({
    ...defaultStep,
    step_name: `step_${newOrder}`,
    step_order: newOrder,
  })
  selectedStepIndex.value = formData.value.steps.length - 1
}

function selectStep(index: number) {
  selectedStepIndex.value = index
  showFlowSettings.value = false
  previewMode.value = 'edit'
}

function selectFlowSettings() {
  showFlowSettings.value = true
  selectedStepIndex.value = null
  previewMode.value = 'edit'
}

function openPreview() {
  showFlowSettings.value = false
  previewMode.value = 'preview'
}

function confirmDeleteStep(index: number) {
  stepToDeleteIndex.value = index
  deleteStepDialogOpen.value = true
}

function deleteStep() {
  if (stepToDeleteIndex.value === null) return

  formData.value.steps.splice(stepToDeleteIndex.value, 1)
  // Reorder remaining steps
  formData.value.steps.forEach((step, idx) => {
    step.step_order = idx + 1
    if (step.step_name.startsWith('step_')) {
      step.step_name = `step_${idx + 1}`
    }
  })

  // Adjust selection
  if (selectedStepIndex.value !== null) {
    if (selectedStepIndex.value >= formData.value.steps.length) {
      selectedStepIndex.value = formData.value.steps.length > 0 ? formData.value.steps.length - 1 : null
    } else if (selectedStepIndex.value === stepToDeleteIndex.value) {
      selectedStepIndex.value = formData.value.steps.length > 0 ? Math.min(stepToDeleteIndex.value, formData.value.steps.length - 1) : null
    }
  }

  deleteStepDialogOpen.value = false
  stepToDeleteIndex.value = null
}

function updateStepOrders() {
  formData.value.steps.forEach((step, idx) => {
    step.step_order = idx + 1
  })
}

function setMessageType(type: string) {
  if (selectedStep.value) {
    selectedStep.value.message_type = type
  }
}

function setInputType(type: string | number | bigint | Record<string, any> | null) {
  if (!selectedStep.value || typeof type !== 'string') return

  selectedStep.value.input_type = type

  // Auto-fill selection options from button titles if:
  // - Input type is 'select'
  // - Message type is 'buttons'
  // - Buttons have titles
  if (type === 'select' && selectedStep.value.message_type === 'buttons') {
    syncButtonTitlesToOptions()
  }
}

// Button helpers
function addButton(type: 'reply' | 'url' = 'reply') {
  if (!selectedStep.value) return
  if (selectedStep.value.buttons.length >= 10) {
    toast.error(t('flowBuilder.maxOptionsError'))
    return
  }
  const newButton: ButtonConfig = {
    id: `btn_${selectedStep.value.buttons.length + 1}`,
    title: '',
    type
  }
  if (type === 'url') {
    newButton.url = ''
  }
  selectedStep.value.buttons.push(newButton)
}

function removeButton(index: number) {
  if (!selectedStep.value) return
  selectedStep.value.buttons.splice(index, 1)
  // Sync options if input type is select
  syncButtonTitlesToOptions()
}

function updateButtonTitle(index: number, title: string) {
  if (!selectedStep.value) return
  selectedStep.value.buttons[index].title = title
  // Sync options if input type is select
  syncButtonTitlesToOptions()
}

function syncButtonTitlesToOptions() {
  if (!selectedStep.value) return
  if (selectedStep.value.input_type !== 'select') return
  if (selectedStep.value.message_type !== 'buttons') return

  const buttonTitles = selectedStep.value.buttons
    ?.filter(btn => btn.title?.trim())
    .map(btn => btn.title.trim()) || []

  selectedStep.value.input_config = {
    ...selectedStep.value.input_config,
    options: buttonTitles
  }
}

// Button conditional branching helpers
function getButtonId(btn: ButtonConfig, index: number): string {
  // Match backend logic: use btn.id if set, otherwise generate btn_1, btn_2, etc.
  return btn.id || `btn_${index + 1}`
}

function getButtonNextStep(buttonId: string): string {
  const target = selectedStep.value?.conditional_next?.[buttonId]
  return target || '__default__'
}

function setButtonNextStep(buttonId: string, targetStep: string | number | bigint | Record<string, any> | null) {
  if (!selectedStep.value || typeof targetStep !== 'string') return
  if (!selectedStep.value.conditional_next) {
    selectedStep.value.conditional_next = {}
  }
  // __default__ means no conditional routing (sequential flow)
  if (targetStep && targetStep !== '__default__') {
    selectedStep.value.conditional_next[buttonId] = targetStep
  } else {
    delete selectedStep.value.conditional_next[buttonId]
  }
}

// API header helpers
function addHeader() {
  if (!selectedStep.value) return
  const headerNum = Object.keys(selectedStep.value.api_config.headers).length + 1
  selectedStep.value.api_config.headers[`Header-${headerNum}`] = ''
}

function updateHeaderKey(oldKey: string, newKey: string) {
  if (!selectedStep.value || oldKey === newKey) return
  const value = selectedStep.value.api_config.headers[oldKey]
  delete selectedStep.value.api_config.headers[oldKey]
  selectedStep.value.api_config.headers[newKey] = value
}

function removeHeader(key: string) {
  if (!selectedStep.value) return
  delete selectedStep.value.api_config.headers[key]
}

// Response mapping helpers
function addResponseMapping() {
  if (!selectedStep.value) return
  const mappingNum = Object.keys(selectedStep.value.api_config.response_mapping).length + 1
  selectedStep.value.api_config.response_mapping[`var_${mappingNum}`] = ''
}

function updateResponseMappingKey(oldKey: string, newKey: string) {
  if (!selectedStep.value || oldKey === newKey) return
  const value = selectedStep.value.api_config.response_mapping[oldKey]
  delete selectedStep.value.api_config.response_mapping[oldKey]
  selectedStep.value.api_config.response_mapping[newKey] = value
}

function removeResponseMapping(key: string) {
  if (!selectedStep.value) return
  delete selectedStep.value.api_config.response_mapping[key]
}

// Completion webhook header helpers
function addCompletionHeader() {
  const headerNum = Object.keys(formData.value.completion_config.headers).length + 1
  formData.value.completion_config.headers[`Header-${headerNum}`] = ''
}

function updateCompletionHeaderKey(oldKey: string, newKey: string) {
  if (oldKey === newKey) return
  const value = formData.value.completion_config.headers[oldKey]
  delete formData.value.completion_config.headers[oldKey]
  formData.value.completion_config.headers[newKey] = value
}

function removeCompletionHeader(key: string) {
  delete formData.value.completion_config.headers[key]
}

// Panel config helpers
function addPanelSection() {
  const newId = `section_${Date.now()}`
  formData.value.panel_config.sections.push({
    id: newId,
    label: t('flowBuilder.newSection'),
    columns: 1,
    collapsible: true,
    default_collapsed: false,
    order: formData.value.panel_config.sections.length + 1,
    fields: []
  })
}

function removePanelSection(index: number) {
  formData.value.panel_config.sections.splice(index, 1)
  // Update order
  formData.value.panel_config.sections.forEach((s, i) => s.order = i + 1)
}

function addFieldToSection(sectionIndex: number, variableKey: string | number | bigint | Record<string, any> | null) {
  if (typeof variableKey !== 'string') return
  const variable = availableVariables.value.find(v => v.key === variableKey)
  if (!variable) return

  const section = formData.value.panel_config.sections[sectionIndex]
  if (!section) return

  section.fields.push({
    key: variableKey,
    label: variableKey.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase()),
    order: section.fields.length + 1
  } as any)
}

function removeFieldFromSection(sectionIndex: number, fieldIndex: number) {
  const section = formData.value.panel_config.sections[sectionIndex]
  if (!section) return

  section.fields.splice(fieldIndex, 1)
  // Update order
  section.fields.forEach((f, i) => f.order = i + 1)
}

async function saveFlow() {
  if (!formData.value.name.trim()) {
    toast.error(t('flowBuilder.enterFlowName'))
    return
  }
  if (formData.value.steps.length === 0) {
    toast.error(t('flowBuilder.addAtLeastOneStep'))
    return
  }

  // Validate button titles and URLs
  for (let i = 0; i < formData.value.steps.length; i++) {
    const step = formData.value.steps[i]
    if (step.message_type === 'buttons' && step.buttons?.length > 0) {
      for (const btn of step.buttons) {
        // Check title
        if (!btn.title?.trim()) {
          toast.error(t('flowBuilder.buttonTitleRequired', { step: step.step_name || `Step ${i + 1}` }))
          selectStep(i)
          return
        }
        // Check URL for URL buttons
        if (btn.type === 'url') {
          if (!btn.url?.trim()) {
            toast.error(t('flowBuilder.urlButtonWithoutUrl', { step: step.step_name || `Step ${i + 1}`, title: btn.title }))
            selectStep(i)
            return
          }
          // Validate URL format
          try {
            new URL(btn.url)
          } catch {
            toast.error(t('flowBuilder.invalidUrl', { step: step.step_name || `Step ${i + 1}`, title: btn.title }))
            selectStep(i)
            return
          }
        }
      }
    }
  }

  isSaving.value = true
  try {
    const data = {
      name: formData.value.name,
      description: formData.value.description,
      trigger_keywords: formData.value.trigger_keywords.split(',').map(k => k.trim()).filter(Boolean),
      initial_message: formData.value.initial_message,
      completion_message: formData.value.completion_message,
      on_complete_action: formData.value.on_complete_action,
      completion_config: formData.value.on_complete_action === 'webhook' ? formData.value.completion_config : {},
      panel_config: formData.value.panel_config,
      enabled: formData.value.enabled,
      steps: formData.value.steps.map((step, idx) => ({
        ...step,
        step_order: idx + 1,
        step_name: step.step_name || `step_${idx + 1}`
      }))
    }

    if (isNewFlow.value) {
      const response = await chatbotService.createFlow(data)
      const newFlow = response.data.data || response.data
      toast.success(t('common.createdSuccess', { resource: t('resources.Flow') }))
      // Update URL to edit mode so subsequent saves work correctly
      router.replace(`/chatbot/flows/${newFlow.id}/edit`)
    } else {
      await chatbotService.updateFlow(flowId.value!, data)
      toast.success(t('common.savedSuccess', { resource: t('resources.Flow') }))
    }

    hasUnsavedChanges.value = false
    // Stay on page - don't navigate away
  } catch (error) {
    toast.error(t('common.failedSave', { resource: t('resources.flow') }))
  } finally {
    isSaving.value = false
  }
}

function handleCancel() {
  if (hasUnsavedChanges.value) {
    cancelDialogOpen.value = true
  } else {
    router.push('/chatbot/flows')
  }
}

function confirmCancel() {
  cancelDialogOpen.value = false
  router.push('/chatbot/flows')
}
</script>

<template>
  <div class="flex flex-col h-full bg-muted/30">
    <!-- Header -->
    <header class="border-b bg-background px-4 py-3 flex-shrink-0">
      <div class="flex items-center gap-4">
        <Button variant="ghost" size="icon" @click="handleCancel">
          <ArrowLeft class="h-5 w-5" />
        </Button>

        <div class="flex-1 flex items-center gap-6">
          <div class="flex items-center gap-2">
            <Label class="text-sm text-muted-foreground whitespace-nowrap">{{ $t('flowBuilder.name') }}</Label>
            <Input
              v-model="formData.name"
              :placeholder="$t('flowBuilder.namePlaceholder')"
              class="w-48 font-medium"
            />
          </div>
          <div class="flex items-center gap-2">
            <Label class="text-sm text-muted-foreground whitespace-nowrap">{{ $t('flowBuilder.description') }}</Label>
            <Input
              v-model="formData.description"
              :placeholder="$t('flowBuilder.optional')"
              class="w-64"
            />
          </div>
        </div>

        <div class="flex items-center gap-3">
          <div class="flex items-center gap-2">
            <Switch
              :checked="formData.enabled"
              @update:checked="formData.enabled = $event"
            />
            <span class="text-sm">{{ formData.enabled ? $t('flowBuilder.enabled') : $t('flowBuilder.disabled') }}</span>
          </div>

          <Button variant="outline" @click="handleCancel">{{ $t('flowBuilder.cancel') }}</Button>
          <Button @click="saveFlow" :disabled="isSaving">
            <Save class="h-4 w-4 mr-2" />
            {{ isSaving ? $t('flowBuilder.saving') + '...' : $t('flowBuilder.saveFlow') }}
          </Button>
        </div>
      </div>
    </header>

    <!-- Loading state -->
    <div v-if="isLoading" class="flex-1 flex items-center justify-center">
      <div class="text-muted-foreground">{{ $t('flowBuilder.loading') }}...</div>
    </div>

    <!-- Main 3-panel layout -->
    <div v-else class="flex-1 flex overflow-hidden">
      <!-- Steps Panel (Left) -->
      <Card
        class="flex-shrink-0 rounded-none border-y-0 border-l-0 flex flex-col"
        :style="{ width: stepsPanelWidth + 'px' }"
      >
        <CardHeader class="py-3 px-4 border-b">
          <div class="flex items-center justify-between">
            <CardTitle class="text-sm font-medium">{{ $t('flowBuilder.steps') }}</CardTitle>
            <Button variant="outline" size="sm" @click="addStep">
              <Plus class="h-4 w-4 mr-1" />
              {{ $t('flowBuilder.add') }}
            </Button>
          </div>
        </CardHeader>
        <ScrollArea class="flex-1">
          <div class="p-2">
            <!-- Flow Settings Option -->
            <div
              :class="[
                'flex items-center gap-2 p-2 rounded-md cursor-pointer transition-colors mb-2',
                showFlowSettings ? 'bg-primary/10 border border-primary/20' : 'hover:bg-muted'
              ]"
              @click="selectFlowSettings"
            >
              <Settings class="h-4 w-4 text-muted-foreground flex-shrink-0" />
              <div class="flex-1 min-w-0">
                <span class="text-sm font-medium">{{ $t('flowBuilder.flowSettings') }}</span>
                <p class="text-xs text-muted-foreground">{{ $t('flowBuilder.messagesWebhook') }}</p>
              </div>
            </div>

            <Separator class="my-2" />

            <draggable
              v-model="formData.steps"
              item-key="step_name"
              handle=".drag-handle"
              class="space-y-1"
              @end="updateStepOrders"
            >
              <template #item="{ element: step, index }">
                <div
                  :class="[
                    'group flex items-center gap-2 p-2 rounded-md cursor-pointer transition-colors',
                    selectedStepIndex === index ? 'bg-primary/10 border border-primary/20' : 'hover:bg-muted'
                  ]"
                  @click="selectStep(index)"
                >
                  <GripVertical class="h-4 w-4 text-muted-foreground cursor-grab drag-handle flex-shrink-0" />
                  <div class="flex-1 min-w-0">
                    <div class="flex items-center gap-2">
                      <Badge variant="outline" class="font-mono text-xs px-1.5">{{ index + 1 }}</Badge>
                      <span class="text-sm font-medium truncate">{{ step.step_name || `Step ${index + 1}` }}</span>
                    </div>
                    <div class="flex items-center gap-1 mt-1 text-xs text-muted-foreground">
                      <component :is="getStepIcon(step.message_type)" class="h-3 w-3" />
                      <span>{{ getStepLabel(step.message_type) }}</span>
                    </div>
                  </div>
                  <Button
                    variant="ghost"
                    size="icon"
                    class="h-7 w-7 opacity-0 group-hover:opacity-100 text-destructive flex-shrink-0"
                    @click.stop="confirmDeleteStep(index)"
                  >
                    <Trash2 class="h-4 w-4" />
                  </Button>
                </div>
              </template>
            </draggable>

            <div v-if="formData.steps.length === 0" class="text-center py-8 text-muted-foreground text-sm">
              {{ $t('flowBuilder.noStepsYet') }}<br />{{ $t('flowBuilder.clickAddToCreate') }}
            </div>
          </div>
        </ScrollArea>
      </Card>

      <!-- Left Resize Handle -->
      <div
        class="w-1 hover:w-1.5 bg-transparent hover:bg-primary/20 cursor-col-resize transition-all flex-shrink-0 group relative"
        @mousedown="startResizeLeft"
      >
        <div class="absolute inset-y-0 -left-1 -right-1"></div>
        <div class="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-1 h-8 rounded-full bg-border group-hover:bg-primary/40 transition-colors"></div>
      </div>

      <!-- Center Panel -->
      <div class="flex-1 flex flex-col overflow-hidden">
        <!-- Flow Chart (when viewing flow settings) -->
        <template v-if="showFlowSettings">
          <FlowChart
            :steps="formData.steps"
            :selected-step-index="selectedStepIndex"
            :flow-name="formData.name"
            :initial-message="formData.initial_message"
            :completion-message="formData.completion_message"
            @select-step="selectStep"
            @add-step="addStep"
            @select-flow-settings="selectFlowSettings"
            @open-preview="openPreview"
          />
        </template>

        <!-- Step Preview (when editing a step) -->
        <template v-else>
          <FlowPreviewPanel
            :steps="formData.steps as any"
            :flow-data="formData as any"
            :selected-step="selectedStep as any"
            :selected-step-index="selectedStepIndex"
            :list-picker-open="listPickerOpen"
            :teams="teams"
            :initial-mode="previewMode"
            @update:list-picker-open="listPickerOpen = $event"
            @select-message-type="setMessageType"
          />
        </template>
      </div>

      <!-- Right Resize Handle -->
      <div
        class="w-1 hover:w-1.5 bg-transparent hover:bg-primary/20 cursor-col-resize transition-all flex-shrink-0 group relative"
        @mousedown="startResizeRight"
      >
        <div class="absolute inset-y-0 -left-1 -right-1"></div>
        <div class="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-1 h-8 rounded-full bg-border group-hover:bg-primary/40 transition-colors"></div>
      </div>

      <!-- Properties Panel (Right) -->
      <Card
        class="flex-shrink-0 rounded-none border-y-0 border-r-0 flex flex-col"
        :style="{ width: propertiesPanelWidth + 'px' }"
      >
        <CardHeader class="py-3 px-4 border-b">
          <CardTitle class="text-sm font-medium">
            {{ showFlowSettings ? $t('flowBuilder.flowSettings') : $t('flowBuilder.stepProperties') }}
          </CardTitle>
        </CardHeader>

        <!-- Flow Settings -->
        <ScrollArea class="flex-1" v-if="showFlowSettings">
          <div class="p-4 space-y-4">
            <!-- Trigger Keywords -->
            <div class="space-y-1.5">
              <Label class="text-xs">{{ $t('flowBuilder.triggerKeywords') }}</Label>
              <Input
                v-model="formData.trigger_keywords"
                :placeholder="$t('flowBuilder.triggerKeywordsPlaceholder')"
                class="h-8 text-xs"
              />
              <p class="text-[10px] text-muted-foreground">{{ $t('flowBuilder.triggerKeywordsHint') }}</p>
            </div>

            <Separator />

            <!-- Initial Message -->
            <div class="space-y-1.5">
              <Label class="text-xs">{{ $t('flowBuilder.initialMessage') }}</Label>
              <Textarea
                v-model="formData.initial_message"
                :placeholder="$t('flowBuilder.initialMessagePlaceholder')"
                :rows="3"
                class="text-xs"
              />
              <p class="text-[10px] text-muted-foreground">{{ $t('flowBuilder.initialMessageHint') }}</p>
            </div>

            <Separator />

            <!-- Completion Message -->
            <div class="space-y-1.5">
              <Label class="text-xs">{{ $t('flowBuilder.completionMessage') }}</Label>
              <Textarea
                v-model="formData.completion_message"
                :placeholder="$t('flowBuilder.completionMessagePlaceholder')"
                :rows="3"
                class="text-xs"
              />
              <p class="text-[10px] text-muted-foreground">{{ $t('flowBuilder.completionMessageHint') }}</p>
            </div>

            <Separator />

            <!-- On Complete Action -->
            <Collapsible v-model:open="webhookHeadersOpen">
              <CollapsibleTrigger class="flex items-center justify-between w-full py-1 text-sm font-medium">
                {{ $t('flowBuilder.onCompletion') }}
                <component :is="webhookHeadersOpen ? ChevronDown : ChevronRight" class="h-4 w-4" />
              </CollapsibleTrigger>
              <CollapsibleContent class="pt-3 space-y-3">
                <div class="space-y-1.5">
                  <Label class="text-xs">{{ $t('flowBuilder.action') }}</Label>
                  <Select v-model="formData.on_complete_action">
                    <SelectTrigger class="h-8 text-xs">
                      <SelectValue :placeholder="$t('flowBuilder.selectAction')" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="none">{{ $t('flowBuilder.noAction') }}</SelectItem>
                      <SelectItem value="webhook">{{ $t('flowBuilder.sendToWebhook') }}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                <!-- Webhook Configuration -->
                <template v-if="formData.on_complete_action === 'webhook'">
                  <div class="space-y-3 p-3 border rounded-lg bg-muted/30">
                    <div class="flex gap-2">
                      <div class="w-16">
                        <Label class="text-[10px]">{{ $t('flowBuilder.method') }}</Label>
                        <Select v-model="formData.completion_config.method">
                          <SelectTrigger class="h-7 text-xs">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem v-for="method in httpMethods" :key="method" :value="method">
                              {{ method }}
                            </SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <div class="flex-1">
                        <Label class="text-[10px]">{{ $t('flowBuilder.url') }}</Label>
                        <Input
                          v-model="formData.completion_config.url"
                          :placeholder="$t('flowBuilder.urlPlaceholder')"
                          class="h-7 text-xs"
                        />
                      </div>
                    </div>

                    <!-- Headers -->
                    <div class="space-y-2">
                      <div class="flex items-center justify-between">
                        <Label class="text-[10px]">{{ $t('flowBuilder.headers') }}</Label>
                        <Button variant="ghost" size="sm" class="h-5 text-[10px] px-1" @click="addCompletionHeader">
                          <Plus class="h-3 w-3" />
                        </Button>
                      </div>
                      <div v-for="(_value, key) in formData.completion_config.headers" :key="key" class="flex gap-1">
                        <Input
                          :model-value="key"
                          :placeholder="$t('flowBuilder.keyPlaceholder')"
                          class="h-6 text-[10px] flex-1"
                          @update:model-value="updateCompletionHeaderKey(key as string, $event)"
                        />
                        <Input
                          v-model="formData.completion_config.headers[key as string]"
                          :placeholder="$t('flowBuilder.valuePlaceholder')"
                          class="h-6 text-[10px] flex-1"
                        />
                        <Button variant="ghost" size="icon" class="h-6 w-6" @click="removeCompletionHeader(key as string)">
                          <Trash2 class="h-3 w-3 text-destructive" />
                        </Button>
                      </div>
                    </div>

                    <div class="space-y-1">
                      <Label class="text-[10px]">{{ $t('flowBuilder.bodyOptional') }}</Label>
                      <Textarea
                        v-model="formData.completion_config.body"
                        :placeholder="$t('flowBuilder.jsonBodyExample')"
                        :rows="2"
                        class="text-[10px] font-mono"
                      />
                    </div>
                  </div>
                </template>
              </CollapsibleContent>
            </Collapsible>

            <Separator />

            <!-- Panel Display Settings -->
            <Collapsible v-model:open="panelConfigOpen">
              <CollapsibleTrigger class="flex items-center justify-between w-full py-1 text-sm font-medium">
                {{ $t('flowBuilder.panelDisplaySettings') }}
                <component :is="panelConfigOpen ? ChevronDown : ChevronRight" class="h-4 w-4" />
              </CollapsibleTrigger>
              <CollapsibleContent class="pt-3 space-y-3">
                <p class="text-[10px] text-muted-foreground">
                  {{ $t('flowBuilder.panelConfigHint') }}
                </p>

                <!-- Available Variables -->
                <div v-if="availableVariables.length > 0" class="space-y-2">
                  <Label class="text-xs">{{ $t('flowBuilder.availableVariables') }}</Label>
                  <div class="flex flex-wrap gap-1">
                    <Badge
                      v-for="variable in unassignedVariables"
                      :key="variable.key"
                      variant="outline"
                      class="text-[10px] cursor-pointer hover:bg-primary/10"
                      :title="`From ${variable.source} in ${variable.stepName}`"
                    >
                      {{ variable.key }}
                    </Badge>
                    <span v-if="unassignedVariables.length === 0" class="text-[10px] text-muted-foreground">
                      {{ $t('flowBuilder.allVariablesAssigned') }}
                    </span>
                  </div>
                </div>

                <div v-else class="text-[10px] text-muted-foreground p-2 border rounded bg-muted/30">
                  {{ $t('flowBuilder.noVariablesAvailable') }}
                </div>

                <!-- Sections -->
                <div class="space-y-2">
                  <div class="flex items-center justify-between">
                    <Label class="text-xs">{{ $t('flowBuilder.sections') }}</Label>
                    <Button variant="outline" size="sm" class="h-6 text-xs" @click="addPanelSection">
                      <Plus class="h-3 w-3 mr-1" />
                      {{ $t('flowBuilder.addSection') }}
                    </Button>
                  </div>

                  <div v-if="formData.panel_config.sections.length === 0" class="text-[10px] text-muted-foreground p-2 border rounded bg-muted/30 text-center">
                    {{ $t('flowBuilder.noSectionsConfigured') }}
                  </div>

                  <div v-for="(section, sectionIdx) in formData.panel_config.sections" :key="section.id" class="border rounded-md p-2 space-y-2 bg-muted/20">
                    <div class="flex items-center gap-2">
                      <Input
                        v-model="section.label"
                        :placeholder="$t('flowBuilder.sectionLabelPlaceholder')"
                        class="h-7 text-xs flex-1"
                      />
                      <Button variant="ghost" size="icon" class="h-7 w-7" @click="removePanelSection(sectionIdx)">
                        <Trash2 class="h-3 w-3 text-destructive" />
                      </Button>
                    </div>

                    <div class="flex items-center gap-3 text-[10px]">
                      <div class="flex items-center gap-1">
                        <span class="text-muted-foreground">{{ $t('flowBuilder.columns') }}:</span>
                        <Select v-model="section.columns">
                          <SelectTrigger class="h-6 w-14 text-[10px]">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem :value="1">1</SelectItem>
                            <SelectItem :value="2">2</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <div class="flex items-center gap-1">
                        <Switch
                          :checked="section.collapsible"
                          @update:checked="section.collapsible = $event"
                          class="scale-75"
                        />
                        <span class="text-muted-foreground">{{ $t('flowBuilder.collapsible') }}</span>
                      </div>
                      <div v-if="section.collapsible" class="flex items-center gap-1">
                        <Switch
                          :checked="section.default_collapsed"
                          @update:checked="section.default_collapsed = $event"
                          class="scale-75"
                        />
                        <span class="text-muted-foreground">{{ $t('flowBuilder.collapsed') }}</span>
                      </div>
                    </div>

                    <!-- Fields in section -->
                    <div class="space-y-1">
                      <div class="flex items-center justify-between">
                        <span class="text-[10px] text-muted-foreground">{{ $t('flowBuilder.fields') }}:</span>
                        <Select @update:model-value="addFieldToSection(sectionIdx, $event)">
                          <SelectTrigger class="h-6 w-24 text-[10px]">
                            <SelectValue :placeholder="$t('flowBuilder.addFieldPlaceholder')" />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem
                              v-for="variable in unassignedVariables"
                              :key="variable.key"
                              :value="variable.key"
                            >
                              {{ variable.key }}
                            </SelectItem>
                            <div v-if="unassignedVariables.length === 0" class="p-2 text-[10px] text-muted-foreground">
                              {{ $t('flowBuilder.noVariablesAvailable') }}
                            </div>
                          </SelectContent>
                        </Select>
                      </div>

                      <div v-if="section.fields.length === 0" class="text-[10px] text-muted-foreground text-center py-1">
                        {{ $t('flowBuilder.noFieldsAdded') }}
                      </div>

                      <div v-for="(field, fieldIdx) in section.fields" :key="field.key" class="bg-background rounded p-2 space-y-2">
                        <div class="flex items-center gap-1">
                          <Badge variant="secondary" class="text-[10px] font-mono">{{ field.key }}</Badge>
                          <Input
                            v-model="field.label"
                            :placeholder="$t('flowBuilder.displayLabel')"
                            class="h-6 text-[10px] flex-1"
                          />
                          <Button variant="ghost" size="icon" class="h-6 w-6" @click="removeFieldFromSection(sectionIdx, fieldIdx)">
                            <Trash2 class="h-3 w-3 text-destructive" />
                          </Button>
                        </div>
                        <div class="flex items-center gap-2">
                          <Select v-model="field.display_type">
                            <SelectTrigger class="h-6 text-[10px] w-20">
                              <SelectValue :placeholder="$t('flowBuilder.displayType')" />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="text">{{ $t('flowBuilder.textType') }}</SelectItem>
                              <SelectItem value="badge">{{ $t('flowBuilder.badgeType') }}</SelectItem>
                              <SelectItem value="tag">{{ $t('flowBuilder.tagType') }}</SelectItem>
                            </SelectContent>
                          </Select>
                          <Select v-model="field.color" :disabled="field.display_type === 'text'">
                            <SelectTrigger class="h-6 text-[10px] flex-1">
                              <SelectValue :placeholder="$t('flowBuilder.colorLabel')" />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="default">{{ $t('flowBuilder.defaultColor') }}</SelectItem>
                              <SelectItem value="success">{{ $t('flowBuilder.successColor') }}</SelectItem>
                              <SelectItem value="warning">{{ $t('flowBuilder.warningColor') }}</SelectItem>
                              <SelectItem value="error">{{ $t('flowBuilder.errorColor') }}</SelectItem>
                              <SelectItem value="info">{{ $t('flowBuilder.infoColor') }}</SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </CollapsibleContent>
            </Collapsible>
          </div>
        </ScrollArea>

        <!-- Step Properties -->
        <ScrollArea class="flex-1" v-else-if="selectedStep">
          <div class="p-4 space-y-4">
            <!-- Basic Properties -->
            <div class="space-y-3">
              <div class="space-y-1.5">
                <Label class="text-xs">{{ $t('flowBuilder.stepName') }}</Label>
                <Input v-model="selectedStep.step_name" :placeholder="$t('flowBuilder.stepNamePlaceholder')" class="h-8" />
              </div>
              <div class="space-y-1.5">
                <Label class="text-xs">{{ $t('flowBuilder.storeResponseAs') }}</Label>
                <Input v-model="selectedStep.store_as" :placeholder="$t('flowBuilder.variableNamePlaceholder')" class="h-8" />
                <p class="text-xs text-muted-foreground">{{ $t('flowBuilder.storeResponseHint') }}</p>
              </div>
            </div>

            <Separator />

            <!-- Message Configuration -->
            <Collapsible v-model:open="messagesOpen">
              <CollapsibleTrigger class="flex items-center justify-between w-full py-1 text-sm font-medium">
                {{ $t('flowBuilder.message') }}
                <component :is="messagesOpen ? ChevronDown : ChevronRight" class="h-4 w-4" />
              </CollapsibleTrigger>
              <CollapsibleContent class="pt-3 space-y-3">
                <!-- Text / Buttons Message -->
                <template v-if="selectedStep.message_type === 'text' || selectedStep.message_type === 'buttons'">
                  <div class="space-y-1.5">
                    <Label class="text-xs">{{ $t('flowBuilder.messageText') }}</Label>
                    <Textarea
                      v-model="selectedStep.message"
                      :placeholder="$t('flowBuilder.messagePlaceholder')"
                      :rows="3"
                      class="text-sm"
                    />
                    <p class="text-xs text-muted-foreground">
                      {{ $t('flowBuilder.dynamicValuesHint') }}
                    </p>
                  </div>
                </template>

                <!-- Buttons Configuration -->
                <template v-if="selectedStep.message_type === 'buttons'">
                  <div class="space-y-3">
                    <div class="flex items-center justify-between">
                      <Label class="text-xs">{{ $t('flowBuilder.buttonOptions') }} ({{ selectedStep.buttons.length }}/10)</Label>
                      <div class="flex gap-1">
                        <Button variant="outline" size="sm" class="h-6 text-xs" @click="addButton('reply')" :disabled="selectedStep.buttons.length >= 10">
                          <Reply class="h-3 w-3 mr-1" />
                          {{ $t('flowBuilder.replyButton') }}
                        </Button>
                        <Button variant="outline" size="sm" class="h-6 text-xs" @click="addButton('url')" :disabled="selectedStep.buttons.length >= 10">
                          <ExternalLink class="h-3 w-3 mr-1" />
                          {{ $t('flowBuilder.urlButton') }}
                        </Button>
                      </div>
                    </div>
                    <div class="space-y-2">
                      <div v-for="(btn, idx) in selectedStep.buttons" :key="idx" class="p-2 border rounded-md bg-muted/30 space-y-2">
                        <div class="flex items-center gap-2">
                          <Badge variant="outline" class="text-[10px] px-1.5">
                            <component :is="btn.type === 'url' ? ExternalLink : Reply" class="h-2.5 w-2.5 mr-1" />
                            {{ btn.type === 'url' ? 'URL' : $t('flowBuilder.replyButton') }}
                          </Badge>
                          <Input :model-value="btn.title" @update:model-value="updateButtonTitle(idx, $event)" :placeholder="$t('flowBuilder.buttonTitle')" class="h-7 flex-1 text-xs" />
                          <Button variant="ghost" size="icon" class="h-7 w-7" @click="removeButton(idx)">
                            <Trash2 class="h-3 w-3 text-destructive" />
                          </Button>
                        </div>
                        <div v-if="btn.type === 'url'" class="flex gap-2">
                          <Input v-model="btn.url" :placeholder="$t('flowBuilder.exampleUrlPlaceholder')" class="h-7 text-xs flex-1" />
                        </div>
                        <div v-else class="space-y-2">
                          <Input v-model="btn.id" :placeholder="$t('flowBuilder.buttonIdPlaceholder')" class="h-7 text-xs" />
                          <div class="flex items-center gap-2">
                            <Label class="text-xs text-muted-foreground whitespace-nowrap">{{ $t('flowBuilder.goTo') }}:</Label>
                            <Select
                              :model-value="getButtonNextStep(getButtonId(btn, idx))"
                              @update:model-value="setButtonNextStep(getButtonId(btn, idx), $event)"
                            >
                              <SelectTrigger class="h-7 text-xs flex-1">
                                <SelectValue :placeholder="$t('flowBuilder.nextStepSequential')" />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="__default__">{{ $t('flowBuilder.nextStepSequential') }}</SelectItem>
                                <SelectItem
                                  v-for="step in stepsWithNames"
                                  :key="`goto-${step.step_name}`"
                                  :value="step.step_name"
                                >
                                  {{ step.step_name }}
                                </SelectItem>
                              </SelectContent>
                            </Select>
                          </div>
                        </div>
                      </div>
                    </div>
                    <p class="text-[10px] text-muted-foreground">
                      {{ $t('flowBuilder.buttonsHint') }}
                    </p>
                  </div>
                </template>

                <!-- API Fetch Configuration -->
                <template v-if="selectedStep.message_type === 'api_fetch'">
                  <div class="space-y-3">
                    <div class="flex gap-2">
                      <div class="w-20">
                        <Label class="text-xs">{{ $t('flowBuilder.method') }}</Label>
                        <Select v-model="selectedStep.api_config.method">
                          <SelectTrigger class="h-8 text-xs">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem v-for="m in httpMethods" :key="m" :value="m">{{ m }}</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <div class="flex-1">
                        <Label class="text-xs">{{ $t('flowBuilder.url') }}</Label>
                        <Input v-model="selectedStep.api_config.url" :placeholder="$t('flowBuilder.urlPlaceholder')" class="h-8 text-xs" />
                      </div>
                    </div>

                    <!-- Headers -->
                    <div class="space-y-2">
                      <div class="flex items-center justify-between">
                        <Label class="text-xs">{{ $t('flowBuilder.headers') }}</Label>
                        <Button variant="ghost" size="sm" class="h-6 text-xs" @click="addHeader">
                          <Plus class="h-3 w-3" />
                        </Button>
                      </div>
                      <div v-for="(_value, key) in selectedStep.api_config.headers" :key="key" class="flex gap-1">
                        <Input
                          :model-value="key"
                          :placeholder="$t('flowBuilder.keyPlaceholder')"
                          class="h-7 text-xs flex-1"
                          @update:model-value="updateHeaderKey(key as string, $event)"
                        />
                        <Input
                          v-model="selectedStep.api_config.headers[key as string]"
                          :placeholder="$t('flowBuilder.valuePlaceholder')"
                          class="h-7 text-xs flex-1"
                        />
                        <Button variant="ghost" size="icon" class="h-7 w-7" @click="removeHeader(key as string)">
                          <Trash2 class="h-3 w-3 text-destructive" />
                        </Button>
                      </div>
                    </div>

                    <!-- Body -->
                    <div v-if="selectedStep.api_config.method !== 'GET'" class="space-y-1.5">
                      <Label class="text-xs">{{ $t('flowBuilder.requestBody') }}</Label>
                      <Textarea v-model="selectedStep.api_config.body" :rows="2" class="text-xs font-mono" />
                    </div>

                    <!-- Response Mapping -->
                    <div class="space-y-2">
                      <div class="flex items-center justify-between">
                        <Label class="text-xs">{{ $t('flowBuilder.responseMapping') }}</Label>
                        <Button variant="ghost" size="sm" class="h-6 text-xs" @click="addResponseMapping">
                          <Plus class="h-3 w-3" />
                        </Button>
                      </div>
                      <div v-for="(_value, key) in selectedStep.api_config.response_mapping" :key="key" class="flex gap-1 items-center">
                        <Input
                          :model-value="key"
                          :placeholder="$t('flowBuilder.variable')"
                          class="h-7 text-xs flex-1"
                          @update:model-value="updateResponseMappingKey(key as string, $event)"
                        />
                        <span class="text-xs text-muted-foreground">=</span>
                        <Input
                          v-model="selectedStep.api_config.response_mapping[key as string]"
                          :placeholder="$t('flowBuilder.dataPathPlaceholder')"
                          class="h-7 text-xs flex-1"
                        />
                        <Button variant="ghost" size="icon" class="h-7 w-7" @click="removeResponseMapping(key as string)">
                          <Trash2 class="h-3 w-3 text-destructive" />
                        </Button>
                      </div>
                    </div>

                    <!-- Message Template -->
                    <div class="space-y-1.5">
                      <Label class="text-xs">{{ $t('flowBuilder.messageTemplate') }}</Label>
                      <Textarea
                        v-model="selectedStep.message"
                        :placeholder="$t('flowBuilder.messageTemplatePlaceholder')"
                        :rows="4"
                        class="text-xs"
                      />
                    </div>

                    <!-- Fallback -->
                    <div class="space-y-1.5">
                      <Label class="text-xs">{{ $t('flowBuilder.fallbackMessage') }}</Label>
                      <Input v-model="selectedStep.api_config.fallback_message" class="h-8 text-xs" />
                    </div>
                  </div>
                </template>

                <!-- WhatsApp Flow Configuration -->
                <template v-if="selectedStep.message_type === 'whatsapp_flow'">
                  <div class="space-y-3">
                    <div class="space-y-1.5">
                      <Label class="text-xs">{{ $t('flowBuilder.whatsappFlow') }}</Label>
                      <Select v-model="selectedStep.input_config.whatsapp_flow_id">
                        <SelectTrigger class="h-8 text-xs">
                          <SelectValue :placeholder="whatsappFlows.length === 0 ? $t('flowBuilder.noFlowsAvailable') : $t('flowBuilder.selectFlow')" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem v-for="wf in whatsappFlows" :key="wf.id" :value="wf.meta_flow_id">
                            {{ wf.name }}
                          </SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div class="space-y-1.5">
                      <Label class="text-xs">{{ $t('flowBuilder.headerText') }}</Label>
                      <Input v-model="selectedStep.input_config.flow_header" class="h-8 text-xs" />
                    </div>
                    <div class="space-y-1.5">
                      <Label class="text-xs">{{ $t('flowBuilder.bodyText') }}</Label>
                      <Textarea v-model="selectedStep.message" :rows="2" class="text-xs" />
                    </div>
                    <div class="space-y-1.5">
                      <Label class="text-xs">{{ $t('flowBuilder.buttonText') }}</Label>
                      <Input v-model="selectedStep.input_config.flow_cta" :placeholder="$t('flowBuilder.buttonTextPlaceholder')" maxlength="20" class="h-8 text-xs" />
                    </div>
                  </div>
                </template>

                <!-- Transfer Configuration -->
                <template v-if="selectedStep.message_type === 'transfer'">
                  <div class="space-y-3">
                    <div class="space-y-1.5">
                      <Label class="text-xs">{{ $t('flowBuilder.transferMessage') }}</Label>
                      <Textarea v-model="selectedStep.message" :rows="2" class="text-xs" />
                    </div>
                    <div class="space-y-1.5">
                      <Label class="text-xs">{{ $t('flowBuilder.assignToTeam') }}</Label>
                      <Select v-model="selectedStep.transfer_config.team_id">
                        <SelectTrigger class="h-8 text-xs">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="_general">{{ $t('agentTransfers.generalQueue') }}</SelectItem>
                          <SelectItem v-for="team in teams" :key="team.id" :value="team.id">
                            {{ team.name }}
                          </SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div class="space-y-1.5">
                      <Label class="text-xs">{{ $t('flowBuilder.transferNotes') }}</Label>
                      <Input v-model="selectedStep.transfer_config.notes" class="h-8 text-xs" />
                    </div>
                  </div>
                </template>
              </CollapsibleContent>
            </Collapsible>

            <Separator v-if="selectedStep.message_type !== 'transfer'" />

            <!-- Input Configuration (not for transfer) -->
            <Collapsible v-if="selectedStep.message_type !== 'transfer'" v-model:open="inputOpen">
              <CollapsibleTrigger class="flex items-center justify-between w-full py-1 text-sm font-medium">
                {{ $t('flowBuilder.input') }}
                <component :is="inputOpen ? ChevronDown : ChevronRight" class="h-4 w-4" />
              </CollapsibleTrigger>
              <CollapsibleContent class="pt-3 space-y-3">
                <div class="space-y-1.5">
                  <Label class="text-xs">{{ $t('flowBuilder.expectedInputType') }}</Label>
                  <Select
                    :model-value="selectedStep.input_type"
                    @update:model-value="setInputType($event)"
                  >
                    <SelectTrigger class="h-8 text-xs">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem v-for="type in inputTypes" :key="type.value" :value="type.value">
                        {{ type.label }}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                <div v-if="selectedStep.input_type === 'select'" class="space-y-1.5">
                  <Label class="text-xs">{{ $t('flowBuilder.optionsPerLine') }}</Label>
                  <Textarea
                    :model-value="(selectedStep.input_config.options || []).join('\n')"
                    @update:model-value="selectedStep.input_config = { ...selectedStep.input_config, options: ($event as string).split('\n').filter(Boolean) }"
                    :rows="3"
                    class="text-xs"
                  />
                </div>
              </CollapsibleContent>
            </Collapsible>

            <Separator v-if="selectedStep.message_type !== 'transfer'" />

            <!-- Validation (not for transfer) -->
            <Collapsible v-if="selectedStep.message_type !== 'transfer'" v-model:open="validationOpen">
              <CollapsibleTrigger class="flex items-center justify-between w-full py-1 text-sm font-medium">
                {{ $t('flowBuilder.validation') }}
                <component :is="validationOpen ? ChevronDown : ChevronRight" class="h-4 w-4" />
              </CollapsibleTrigger>
              <CollapsibleContent class="pt-3 space-y-3">
                <div class="space-y-1.5">
                  <Label class="text-xs">{{ $t('flowBuilder.validationRegex') }}</Label>
                  <Input v-model="selectedStep.validation_regex" :placeholder="$t('flowBuilder.validationRegexPlaceholder')" class="h-8 text-xs font-mono" />
                </div>
                <div class="space-y-1.5">
                  <Label class="text-xs">{{ $t('flowBuilder.errorMessage') }}</Label>
                  <Input v-model="selectedStep.validation_error" class="h-8 text-xs" />
                </div>
                <div class="flex items-center gap-2">
                  <Switch
                    :checked="selectedStep.retry_on_invalid"
                    @update:checked="selectedStep.retry_on_invalid = $event"
                  />
                  <Label class="text-xs">{{ $t('flowBuilder.retryOnInvalid') }}</Label>
                  <Input
                    v-if="selectedStep.retry_on_invalid"
                    v-model.number="selectedStep.max_retries"
                    type="number"
                    min="1"
                    max="10"
                    class="h-7 w-16 text-xs ml-auto"
                  />
                </div>
              </CollapsibleContent>
            </Collapsible>

            <Separator v-if="selectedStep.message_type !== 'transfer'" />

            <!-- Advanced (not for transfer) -->
            <Collapsible v-if="selectedStep.message_type !== 'transfer'" v-model:open="advancedOpen">
              <CollapsibleTrigger class="flex items-center justify-between w-full py-1 text-sm font-medium">
                {{ $t('flowBuilder.advanced') }}
                <component :is="advancedOpen ? ChevronDown : ChevronRight" class="h-4 w-4" />
              </CollapsibleTrigger>
              <CollapsibleContent class="pt-3 space-y-3">
                <div class="space-y-1.5">
                  <Label class="text-xs">{{ $t('flowBuilder.skipCondition') }}</Label>
                  <Input v-model="selectedStep.skip_condition" :placeholder="$t('flowBuilder.skipConditionPlaceholder')" class="h-8 text-xs font-mono" />
                  <p class="text-xs text-muted-foreground">{{ $t('flowBuilder.skipConditionHint') }}</p>
                </div>
              </CollapsibleContent>
            </Collapsible>
          </div>
        </ScrollArea>
        <div v-else class="flex-1 flex items-center justify-center text-muted-foreground text-sm p-4">
          {{ $t('flowBuilder.selectStepToEdit') }}
        </div>
      </Card>
    </div>

    <!-- Delete Step Dialog -->
    <AlertDialog v-model:open="deleteStepDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('flowBuilder.deleteStep') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('flowBuilder.deleteStepConfirm') }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('common.cancel') }}</AlertDialogCancel>
          <AlertDialogAction @click="deleteStep">{{ $t('common.delete') }}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <!-- Cancel Dialog -->
    <AlertDialog v-model:open="cancelDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('flowBuilder.unsavedChanges') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('flowBuilder.unsavedChangesConfirm') }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('flowBuilder.stay') }}</AlertDialogCancel>
          <AlertDialogAction @click="confirmCancel">{{ $t('flowBuilder.leave') }}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
