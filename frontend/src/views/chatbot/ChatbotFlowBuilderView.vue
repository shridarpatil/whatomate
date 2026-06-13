<script setup lang="ts">
import { ref, computed, onMounted, markRaw, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useVueFlow, MarkerType, type NodeMouseEvent, type Edge, type EdgeMouseEvent, type Connection } from '@vue-flow/core'
import { toast } from 'vue-sonner'

import FlowCanvas from '@/components/shared/FlowCanvas.vue'
import { chatbotService } from '@/services/api'
import type { ChatFlowGraph, ChatNode, ChatEdge, ChatNodeType } from '@/services/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Card, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  ArrowLeft,
  Save,
  MessageSquare,
  MousePointerClick,
  Globe,
  MessageCircle,
  Users,
  GitBranch,
  Clock,
  ExternalLink,
  StopCircle,
  ChevronDown,
  ChevronRight,
  Plus,
  Trash2,
  Play,
} from 'lucide-vue-next'

import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import ConfirmDialog from '@/components/shared/ConfirmDialog.vue'
import ErrorState from '@/components/shared/ErrorState.vue'
import ChatNodeProperties from '@/components/chatbot/ChatNodeProperties.vue'
import PanelConfigEditor from '@/components/chatbot/PanelConfigEditor.vue'
import type { PanelConfig, AvailableVariable } from '@/components/chatbot/PanelConfigEditor.vue'

import ChatbotTextNode from '@/components/chatbot/nodes/ChatbotTextNode.vue'
import ChatbotButtonsNode from '@/components/chatbot/nodes/ChatbotButtonsNode.vue'
import ChatbotApiNode from '@/components/chatbot/nodes/ChatbotApiNode.vue'
import ChatbotWhatsAppFlowNode from '@/components/chatbot/nodes/ChatbotWhatsAppFlowNode.vue'
import ChatbotTransferNode from '@/components/chatbot/nodes/ChatbotTransferNode.vue'
import ChatbotConditionNode from '@/components/chatbot/nodes/ChatbotConditionNode.vue'
import ChatbotTimingNode from '@/components/chatbot/nodes/ChatbotTimingNode.vue'
import ChatbotGotoFlowNode from '@/components/chatbot/nodes/ChatbotGotoFlowNode.vue'
import ChatbotEndNode from '@/components/chatbot/nodes/ChatbotEndNode.vue'
import ChatbotStartNode from '@/components/chatbot/nodes/ChatbotStartNode.vue'

import InteractivePreview from '@/components/chatbot/flow-preview/InteractivePreview.vue'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const flowId = computed(() => (route.params.id as string) || '')
const isNewFlow = computed(() => !flowId.value || route.path.endsWith('/new'))

// Top-level flow metadata
const name = ref('')
const description = ref('')
const enabled = ref(true)
const triggerKeywords = ref('')
const initialMessage = ref('Hi! Let me help you with that.')
const completionMessage = ref('Thank you! We have all the information we need.')
const onCompleteAction = ref<'none' | 'webhook'>('none')
const completionConfig = ref<{
  url: string
  method: string
  headers: Record<string, string>
  body: string
}>({ url: '', method: 'POST', headers: {}, body: '' })
const panelConfig = ref<PanelConfig>({ sections: [] })

const isLoading = ref(true)
const loadError = ref(false)
const isSaving = ref(false)
const hasUnsavedChanges = ref(false)
const showDeleteNodeConfirm = ref(false)
const cancelDialogOpen = ref(false)
const showPreview = ref(false)
const auditRefreshKey = ref(0)
const completionConfigOpen = ref(false)
const panelConfigOpen = ref(false)
const activityOpen = ref(false)

const createdAt = ref('')
const updatedAt = ref('')
const createdByName = ref('')
const updatedByName = ref('')

// All flows for goto_flow target picker
const availableFlows = ref<{ id: string; name: string }[]>([])

// Vue Flow custom node types (cast to `any` to bypass strict NodeComponent check)
const START_NODE_ID = '__start__'

const nodeTypes: any = {
  start: markRaw(ChatbotStartNode),
  message: markRaw(ChatbotTextNode),
  prompt: markRaw(ChatbotTextNode),
  buttons: markRaw(ChatbotButtonsNode),
  api_call: markRaw(ChatbotApiNode),
  whatsapp_flow: markRaw(ChatbotWhatsAppFlowNode),
  transfer: markRaw(ChatbotTransferNode),
  condition: markRaw(ChatbotConditionNode),
  timing: markRaw(ChatbotTimingNode),
  goto_flow: markRaw(ChatbotGotoFlowNode),
  end: markRaw(ChatbotEndNode),
  webhook: markRaw(ChatbotApiNode),
}

// Palette: 'prompt' and 'webhook' are internal-only — a Text node
// becomes a prompt when the author sets an expected response.
const palette: { type: ChatNodeType; label: string; icon: any; color: string }[] = [
  { type: 'message', label: 'Text', icon: MessageSquare, color: 'bg-blue-600' },
  { type: 'buttons', label: 'Buttons', icon: MousePointerClick, color: 'bg-purple-600' },
  { type: 'api_call', label: 'API', icon: Globe, color: 'bg-orange-600' },
  { type: 'whatsapp_flow', label: 'WA Flow', icon: MessageCircle, color: 'bg-green-600' },
  { type: 'transfer', label: 'Transfer', icon: Users, color: 'bg-amber-600' },
  { type: 'condition', label: 'Condition', icon: GitBranch, color: 'bg-indigo-600' },
  { type: 'timing', label: 'Timing', icon: Clock, color: 'bg-cyan-600' },
  { type: 'goto_flow', label: 'Go to Flow', icon: ExternalLink, color: 'bg-teal-600' },
  { type: 'end', label: 'End', icon: StopCircle, color: 'bg-slate-600' },
]

const {
  nodes,
  edges,
  addNodes,
  addEdges,
  removeNodes,
  removeEdges,
  onConnect,
  project,
  fitView,
} = useVueFlow({
  defaultEdgeOptions: {
    type: 'default',
    animated: true,
    markerEnd: MarkerType.ArrowClosed,
  },
})

const entryNodeId = ref<string>('')
const selectedNodeId = ref<string | null>(null)

const selectedNode = computed(() => {
  if (!selectedNodeId.value) return null
  return nodes.value.find((n) => n.id === selectedNodeId.value) || null
})

// Properties panel reads a ChatNode-shaped object derived from the Vue Flow node.
const selectedChatNode = computed<ChatNode | null>(() => {
  const node = selectedNode.value
  if (!node) return null
  return {
    id: node.id,
    type: node.type as ChatNodeType,
    label: node.data?.label || '',
    position: node.position,
    config: node.data?.config || {},
  }
})

function onNodeClick(event: NodeMouseEvent) {
  selectedNodeId.value = event.node.id
}

function onPaneClick() {
  selectedNodeId.value = null
}

let nodeCounter = 0

function defaultConfigFor(type: ChatNodeType): Record<string, any> {
  switch (type) {
    case 'message':
      return { message: '' }
    case 'prompt':
      return { body: '', store_as: '', validation_regex: '', validation_error: 'Invalid input. Please try again.', max_retries: 3 }
    case 'buttons':
      return { body: '', buttons: [] }
    case 'api_call':
      return { url: '', method: 'GET', headers: {}, body: '', response_mapping: {}, message_template: '' }
    case 'whatsapp_flow':
      return { flow_id: '', header: '', body: '', cta: 'Open' }
    case 'transfer':
      return { body: '', team_id: '_general', notes: '' }
    case 'condition':
      return { expression: '' }
    case 'timing':
      return {
        schedule: [
          { day: 'monday', enabled: true, start_time: '09:00', end_time: '18:00' },
          { day: 'tuesday', enabled: true, start_time: '09:00', end_time: '18:00' },
          { day: 'wednesday', enabled: true, start_time: '09:00', end_time: '18:00' },
          { day: 'thursday', enabled: true, start_time: '09:00', end_time: '18:00' },
          { day: 'friday', enabled: true, start_time: '09:00', end_time: '18:00' },
          { day: 'saturday', enabled: false, start_time: '09:00', end_time: '18:00' },
          { day: 'sunday', enabled: false, start_time: '09:00', end_time: '18:00' },
        ],
      }
    case 'goto_flow':
      return { flow_id: '' }
    case 'webhook':
      return { url: '', method: 'POST', headers: {}, body: '' }
    case 'end':
      return { message: '' }
    default:
      return {}
  }
}

const paletteLabels: Record<string, string> = {
  message: 'Message',
  prompt: 'Prompt',
  buttons: 'Buttons',
  api_call: 'API',
  whatsapp_flow: 'WhatsApp Flow',
  transfer: 'Transfer',
  condition: 'Condition',
  timing: 'Timing',
  goto_flow: 'Go to Flow',
  webhook: 'Webhook',
  end: 'End',
}

function addNodeFromPalette(type: ChatNodeType) {
  const pos = project({ x: window.innerWidth / 2 - 200, y: window.innerHeight / 2 - 200 })
  const id = `node_${Date.now()}_${nodeCounter++}`
  addNodes([
    {
      id,
      type,
      position: { x: pos.x, y: pos.y },
      data: {
        label: paletteLabels[type] || type,
        config: defaultConfigFor(type),
        isEntryNode: false,
      },
    },
  ])
  // First action node added → wire start → it so the flow has an entry
  // path. Subsequent nodes are wired manually.
  const startHasOutgoing = edges.value.some((e) => e.source === START_NODE_ID)
  if (!startHasOutgoing && nodes.value.some((n) => n.id === START_NODE_ID)) {
    addEdges([{
      id: `edge_start_${id}`,
      source: START_NODE_ID,
      target: id,
      sourceHandle: undefined,
      type: 'default',
      animated: true,
      markerEnd: MarkerType.ArrowClosed,
      label: '',
    }])
  }
  selectedNodeId.value = id
  hasUnsavedChanges.value = true
}

function ensureStartNode() {
  if (nodes.value.some((n) => n.type === 'start')) return
  addNodes([
    {
      id: START_NODE_ID,
      type: 'start',
      position: { x: 100, y: 100 },
      data: { label: 'Start', config: {}, isEntryNode: true },
      deletable: false,
    },
  ])
  entryNodeId.value = START_NODE_ID
}

onConnect((params) => {
  // Enforce single edge per source handle.
  const existing = edges.value.filter(
    (e) => e.source === params.source && e.sourceHandle === params.sourceHandle,
  )
  if (existing.length > 0) removeEdges(existing)

  addEdges([
    {
      ...params,
      type: 'default',
      animated: true,
      markerEnd: MarkerType.ArrowClosed,
      label: params.sourceHandle || 'default',
    },
  ])
  spreadParallelLabels()
  hasUnsavedChanges.value = true
})

function spreadParallelLabels() {
  const groups = new Map<string, Edge[]>()
  for (const e of edges.value) {
    const key = `${e.source}→${e.target}`
    if (!groups.has(key)) groups.set(key, [])
    groups.get(key)!.push(e)
  }
  for (const group of groups.values()) {
    for (let i = 0; i < group.length; i++) {
      const yOffset = group.length > 1 ? (i - (group.length - 1) / 2) * 22 : 0
      group[i].labelStyle = { transform: `translateY(${yOffset}px)` }
      group[i].labelBgStyle = { fill: 'none', fillOpacity: 0 }
      group[i].labelBgPadding = [0, 0] as [number, number]
    }
  }
}

function onEdgeClick({ edge }: EdgeMouseEvent) {
  nodes.value.forEach((n) => (n.selected = false))
  edges.value.forEach((e) => (e.selected = false))
  edge.selected = true
  selectedNodeId.value = null
}

function onEdgeUpdate({ edge, connection }: { edge: Edge; connection: Connection }) {
  removeEdges([edge])
  addEdges([
    {
      ...connection,
      type: 'default',
      animated: true,
      markerEnd: MarkerType.ArrowClosed,
      label: connection.sourceHandle || 'default',
    },
  ])
  spreadParallelLabels()
  hasUnsavedChanges.value = true
}

function onUpdateNode(updated: ChatNode) {
  const node = nodes.value.find((n) => n.id === updated.id)
  if (!node) return
  if (updated.type !== node.type) {
    // "Text" nodes flip between v2 message / prompt when the author
    // sets an expected response. Vue Flow keys components off node.type
    // so this swap has to land back in the canvas state too.
    node.type = updated.type
  }
  node.data = {
    ...node.data,
    label: updated.label,
    config: updated.config,
  }
  hasUnsavedChanges.value = true
}

function requestDeleteSelectedNode() {
  if (!selectedNode.value) return
  if (selectedNode.value.type === 'start') return // Start node is fixed.
  showDeleteNodeConfirm.value = true
}

function confirmDeleteSelectedNode() {
  const node = selectedNode.value
  if (!node) return
  if (node.type === 'start') return
  const nodeId = node.id
  const connected = edges.value.filter((e) => e.source === nodeId || e.target === nodeId)
  if (connected.length > 0) removeEdges(connected)
  removeNodes([nodeId])
  selectedNodeId.value = null
  showDeleteNodeConfirm.value = false
  hasUnsavedChanges.value = true
}

// Track node moves so unsaved-changes nags appear.
function onNodeDragStop() {
  hasUnsavedChanges.value = true
}

// Build the v2 graph payload to ship to the API.
function toGraphPayload(): ChatFlowGraph {
  const ivrNodes: ChatNode[] = nodes.value.map((n) => ({
    id: n.id,
    type: n.type as ChatNodeType,
    label: n.data?.label || '',
    position: { x: n.position.x, y: n.position.y },
    config: n.data?.config || {},
  }))

  const ivrEdges: ChatEdge[] = edges.value.map((e) => ({
    from: e.source,
    to: e.target,
    condition: e.sourceHandle || (e as any).label || 'default',
  }))

  // Entry node is always the start sentinel when present. Fall back to
  // the node without incoming edges for any legacy graph that doesn't
  // have one (loadGraph will have repaired most of those already).
  const start = ivrNodes.find((n) => n.type === 'start')
  let entry: string
  if (start) {
    entry = start.id
  } else {
    const nodesWithIncoming = new Set(ivrEdges.map((e) => e.to))
    entry = ivrNodes.find((n) => !nodesWithIncoming.has(n.id))?.id || ivrNodes[0]?.id || ''
  }

  return {
    version: 2,
    nodes: ivrNodes,
    edges: ivrEdges,
    entry_node: entry,
  }
}

// Variables available to the contact-panel editor — captured from
// prompt nodes (store_as) and api_call nodes (response_mapping keys).
const availableVariables = computed<AvailableVariable[]>(() => {
  const out: AvailableVariable[] = []
  for (const n of nodes.value) {
    const cfg = (n.data?.config || {}) as Record<string, any>
    if (n.type === 'prompt' && typeof cfg.store_as === 'string' && cfg.store_as.trim()) {
      out.push({ key: cfg.store_as.trim(), source: 'Store as', stepName: n.id })
    }
    if (n.type === 'api_call' && cfg.response_mapping && typeof cfg.response_mapping === 'object') {
      for (const k of Object.keys(cfg.response_mapping)) {
        if (k && k.trim()) out.push({ key: k.trim(), source: 'Response mapping', stepName: n.id })
      }
    }
  }
  return out
})

// Reactive preview graph — InteractivePreview consumes this for simulation.
const previewGraph = computed<ChatFlowGraph | null>(() => {
  if (nodes.value.length === 0) return null
  return toGraphPayload()
})

function loadGraph(graph: ChatFlowGraph) {
  // Legacy graphs (saved before the start sentinel landed) may have an
  // entry_node that points at a real action node. Inject a start node
  // and rewire so the editor always shows a fixed entry point.
  const hasStart = graph.nodes.some((n) => n.type === 'start')
  const originalEntry = graph.entry_node || graph.nodes[0]?.id || ''
  // Once we inject a start, only the start is the entry — the legacy
  // entry node needs its target handle restored so the edge can land.
  const effectiveEntry = hasStart ? graph.entry_node : START_NODE_ID

  const vfNodes = graph.nodes.map((n) => ({
    id: n.id,
    type: n.type,
    position: { x: n.position?.x ?? 0, y: n.position?.y ?? 0 },
    data: {
      label: n.label,
      config: n.config,
      isEntryNode: n.id === effectiveEntry,
    },
    deletable: n.type !== 'start',
  }))

  // Self-heal legacy button edges saved before the handle carried the
  // "button:" prefix: any non-default edge leaving a buttons node whose
  // condition is a bare button id is normalized to "button:<id>" so it
  // attaches to the prefixed handle and re-saves in canonical form.
  const buttonNodeIds = new Set(
    graph.nodes.filter((n) => n.type === 'buttons').map((n) => n.id),
  )
  const normalizeCondition = (e: ChatEdge) => {
    if (
      e.condition !== 'default' &&
      e.condition &&
      !e.condition.startsWith('button:') &&
      buttonNodeIds.has(e.from)
    ) {
      return `button:${e.condition}`
    }
    return e.condition
  }

  const vfEdges = (graph.edges || []).map((e, idx) => {
    const condition = normalizeCondition(e)
    return {
      id: `edge_${idx}`,
      source: e.from,
      target: e.to,
      // BaseNode's plain source handle has no id, so only attach a
      // sourceHandle for branch conditions (button:*, true/false,
      // in_hours/out_of_hours). Plain "default" edges leave it
      // undefined and Vue Flow routes to the node's only target.
      sourceHandle: condition !== 'default' ? condition : undefined,
      type: 'default' as const,
      animated: true,
      markerEnd: MarkerType.ArrowClosed,
      label: condition !== 'default' ? condition : '',
    }
  })

  if (!hasStart) {
    // Place start directly above the original entry so the auto-wired
    // edge flows top → bottom naturally on the canvas.
    const entry = graph.nodes.find((n) => n.id === originalEntry)
    const startPos = entry
      ? { x: entry.position?.x ?? 100, y: (entry.position?.y ?? 100) - 140 }
      : { x: 100, y: 100 }
    vfNodes.unshift({
      id: START_NODE_ID,
      type: 'start',
      position: startPos,
      data: { label: 'Start', config: {}, isEntryNode: true },
      deletable: false,
    })
    if (originalEntry) {
      vfEdges.unshift({
        id: 'edge_start',
        source: START_NODE_ID,
        target: originalEntry,
        sourceHandle: undefined,
        type: 'default' as const,
        animated: true,
        markerEnd: MarkerType.ArrowClosed,
        label: '',
      })
    }
    entryNodeId.value = START_NODE_ID
  } else {
    entryNodeId.value = graph.entry_node || ''
  }

  addNodes(vfNodes)
  addEdges(vfEdges)
  spreadParallelLabels()

  setTimeout(() => fitView({ padding: 0.2 }), 100)
}

async function loadFlow() {
  if (isNewFlow.value) {
    ensureStartNode()
    isLoading.value = false
    return
  }
  isLoading.value = true
  loadError.value = false
  try {
    const response = await chatbotService.getFlow(flowId.value)
    const flow = response.data.data || response.data

    name.value = flow.name || flow.Name || ''
    description.value = flow.description || flow.Description || ''
    enabled.value = flow.is_enabled ?? flow.IsEnabled ?? flow.enabled ?? true
    triggerKeywords.value = (flow.trigger_keywords || flow.TriggerKeywords || []).join(', ')
    initialMessage.value = flow.initial_message || flow.InitialMessage || ''
    completionMessage.value = flow.completion_message || flow.CompletionMessage || ''
    onCompleteAction.value = (flow.on_complete_action || flow.OnCompleteAction || 'none') as 'none' | 'webhook'
    const wc = flow.completion_config || flow.CompletionConfig || {}
    completionConfig.value = {
      url: wc.url || '',
      method: wc.method || 'POST',
      headers: wc.headers || {},
      body: wc.body || '',
    }
    const pc = flow.panel_config || flow.PanelConfig || { sections: [] }
    panelConfig.value = { sections: pc.sections || [] }

    createdAt.value = flow.created_at || ''
    updatedAt.value = flow.updated_at || ''
    createdByName.value = flow.created_by_name || flow.created_by?.full_name || ''
    updatedByName.value = flow.updated_by_name || flow.updated_by?.full_name || ''

    const graph = flow.graph || flow.Graph
    if (graph && graph.version === 2) {
      loadGraph(graph)
    }
  } catch {
    loadError.value = true
  } finally {
    isLoading.value = false
  }
}

async function loadAvailableFlows() {
  try {
    const res = await chatbotService.listFlows({ limit: 200 })
    const list = (res.data as any)?.data?.flows || (res.data as any)?.flows || []
    availableFlows.value = list.map((f: any) => ({ id: f.id || f.ID, name: f.name || f.Name }))
  } catch {
    availableFlows.value = []
  }
}

async function saveFlow() {
  if (!name.value.trim()) {
    toast.error(t('flowBuilder.nameRequired', 'Name is required'))
    return
  }

  isSaving.value = true
  try {
    const graph = toGraphPayload()
    const data: Record<string, any> = {
      name: name.value,
      description: description.value,
      trigger_keywords: triggerKeywords.value.split(',').map((k) => k.trim()).filter(Boolean),
      initial_message: initialMessage.value,
      completion_message: completionMessage.value,
      on_complete_action: onCompleteAction.value,
      completion_config: onCompleteAction.value === 'webhook' ? completionConfig.value : {},
      panel_config: panelConfig.value,
      enabled: enabled.value,
      graph,
    }

    if (isNewFlow.value) {
      const response = await chatbotService.createFlow(data)
      const newFlow = response.data.data || response.data
      toast.success(t('common.createdSuccess', { resource: t('resources.Flow') }))
      router.replace(`/chatbot/flows/${newFlow.id}/edit`)
    } else {
      await chatbotService.updateFlow(flowId.value, data)
      toast.success(t('common.savedSuccess', { resource: t('resources.Flow') }))
    }

    hasUnsavedChanges.value = false
    auditRefreshKey.value++
  } catch {
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

// Webhook headers helpers (flow-level completion)
function addCompletionHeader() {
  completionConfig.value.headers = { ...(completionConfig.value.headers || {}), '': '' }
  hasUnsavedChanges.value = true
}

function updateCompletionHeaderKey(oldKey: string, newKey: string) {
  if (oldKey === newKey) return
  const h = { ...(completionConfig.value.headers || {}) }
  h[newKey] = h[oldKey]
  delete h[oldKey]
  completionConfig.value.headers = h
  hasUnsavedChanges.value = true
}

function updateCompletionHeaderValue(key: string, value: string) {
  completionConfig.value.headers = { ...(completionConfig.value.headers || {}), [key]: value }
  hasUnsavedChanges.value = true
}

function removeCompletionHeader(key: string) {
  const h = { ...(completionConfig.value.headers || {}) }
  delete h[key]
  completionConfig.value.headers = h
  hasUnsavedChanges.value = true
}

// Mark changes from text inputs.
watch([name, description, enabled, triggerKeywords, initialMessage, completionMessage, onCompleteAction, panelConfig], () => {
  if (!isLoading.value) hasUnsavedChanges.value = true
})

onMounted(async () => {
  loadAvailableFlows()
  await loadFlow()
})
</script>

<template>
  <div class="flex flex-col h-screen bg-muted/30">
    <!-- Header -->
    <header class="border-b bg-background px-4 py-3 flex-shrink-0">
      <div class="flex items-center gap-4">
        <Button variant="ghost" size="icon" @click="handleCancel">
          <ArrowLeft class="h-5 w-5" />
        </Button>

        <div class="flex-1 flex items-center gap-6">
          <div class="flex items-center gap-2">
            <Label class="text-sm text-muted-foreground whitespace-nowrap">{{ $t('flowBuilder.name') }}</Label>
            <Input v-model="name" :placeholder="$t('flowBuilder.namePlaceholder')" class="w-48 font-medium" />
          </div>
          <div class="flex items-center gap-2">
            <Label class="text-sm text-muted-foreground whitespace-nowrap">{{ $t('flowBuilder.description') }}</Label>
            <Input v-model="description" :placeholder="$t('flowBuilder.optional')" class="w-64" />
          </div>
        </div>

        <div class="flex items-center gap-3">
          <div class="flex items-center gap-2">
            <Switch :checked="enabled" @update:checked="enabled = $event" />
            <span class="text-sm">{{ enabled ? $t('flowBuilder.enabled') : $t('flowBuilder.disabled') }}</span>
          </div>

          <Button variant="outline" size="sm" @click="showPreview = true" :disabled="nodes.length === 0">
            <Play class="h-4 w-4 mr-1" />
            {{ $t('flowBuilder.preview', 'Preview') }}
          </Button>
          <Button variant="outline" @click="handleCancel">{{ $t('flowBuilder.cancel') }}</Button>
          <Button @click="saveFlow" :disabled="isSaving">
            <Save class="h-4 w-4 mr-2" />
            {{ isSaving ? $t('flowBuilder.saving') + '...' : $t('flowBuilder.saveFlow') }}
          </Button>
        </div>
      </div>
    </header>

    <!-- Node palette -->
    <div class="flex items-center gap-2 px-4 py-2 border-b bg-muted/30 overflow-x-auto shrink-0">
      <span class="text-xs text-muted-foreground shrink-0">Add node:</span>
      <Button
        v-for="p in palette"
        :key="p.type"
        variant="outline"
        size="sm"
        class="h-7 text-xs gap-1.5 shrink-0"
        @click="addNodeFromPalette(p.type)"
      >
        <div :class="['w-2 h-2 rounded-full', p.color]" />
        <component :is="p.icon" class="w-3 h-3" />
        {{ p.label }}
      </Button>
    </div>

    <!-- Main: canvas + right panel -->
    <div class="flex-1 flex overflow-hidden">
      <!-- Canvas -->
      <div class="flex-1 relative">
        <div v-if="isLoading" class="absolute inset-0 flex items-center justify-center bg-background/80 z-10">
          <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
        </div>
        <ErrorState
          v-else-if="loadError"
          :title="$t('flowBuilder.loadFailed', 'Failed to load flow')"
          :description="$t('flowBuilder.loadFailedDesc', 'Try reloading or go back to the flow list.')"
          class="absolute inset-0 z-10 bg-background"
        >
          <template #action>
            <div class="flex gap-2">
              <Button variant="outline" size="sm" @click="router.push('/chatbot/flows')">
                {{ $t('common.goBack', 'Go back') }}
              </Button>
              <Button size="sm" @click="loadFlow">
                {{ $t('common.retry') }}
              </Button>
            </div>
          </template>
        </ErrorState>
        <FlowCanvas
          :node-types="nodeTypes"
          edge-type="default"
          @node-click="onNodeClick"
          @pane-click="onPaneClick"
          @edge-click="onEdgeClick"
          @edge-update="onEdgeUpdate"
          @node-drag-stop="onNodeDragStop"
        />
      </div>

      <!-- Right panel -->
      <Card class="w-[420px] min-w-0 border-y-0 border-r-0 rounded-none shrink-0 flex flex-col">
        <!-- Node properties when a node is selected -->
        <div v-if="selectedChatNode && selectedChatNode.type !== 'start'" class="flex-1 overflow-y-auto">
          <ChatNodeProperties
            :node="selectedChatNode"
            :current-flow-id="flowId"
            :available-flows="availableFlows"
            @update:node="onUpdateNode"
            @delete="requestDeleteSelectedNode"
          />
        </div>

        <!-- Flow settings when nothing is selected -->
        <ScrollArea v-else orientation="vertical" class="flex-1">
          <div class="p-4 space-y-4 min-w-0">
            <CardHeader class="p-0 pb-2">
              <CardTitle class="text-sm font-medium">{{ $t('flowBuilder.flowSettings') }}</CardTitle>
            </CardHeader>

            <!-- Trigger keywords -->
            <div class="space-y-1.5">
              <Label class="text-xs">{{ $t('flowBuilder.triggerKeywords') }}</Label>
              <Input v-model="triggerKeywords" :placeholder="$t('flowBuilder.triggerKeywordsPlaceholder')" class="h-8 text-xs" />
              <p class="text-[10px] text-muted-foreground">{{ $t('flowBuilder.triggerKeywordsHint') }}</p>
            </div>

            <Separator />

            <!-- Initial message -->
            <div class="space-y-1.5">
              <Label class="text-xs">{{ $t('flowBuilder.initialMessage') }}</Label>
              <Textarea v-model="initialMessage" :placeholder="$t('flowBuilder.initialMessagePlaceholder')" :rows="2" class="text-xs" />
            </div>

            <!-- Completion message -->
            <div class="space-y-1.5">
              <Label class="text-xs">{{ $t('flowBuilder.completionMessage') }}</Label>
              <Textarea v-model="completionMessage" :placeholder="$t('flowBuilder.completionMessagePlaceholder')" :rows="2" class="text-xs" />
            </div>

            <Separator />

            <!-- On complete action -->
            <Collapsible v-model:open="completionConfigOpen">
              <CollapsibleTrigger class="flex items-center justify-between w-full py-1 text-sm font-medium">
                {{ $t('flowBuilder.onCompletion') }}
                <component :is="completionConfigOpen ? ChevronDown : ChevronRight" class="h-4 w-4" />
              </CollapsibleTrigger>
              <CollapsibleContent class="pt-3 space-y-3">
                <div class="space-y-1.5">
                  <Label class="text-xs">{{ $t('flowBuilder.action') }}</Label>
                  <Select v-model="onCompleteAction">
                    <SelectTrigger class="h-8 text-xs"><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="none">{{ $t('flowBuilder.noAction') }}</SelectItem>
                      <SelectItem value="webhook">{{ $t('flowBuilder.sendToWebhook') }}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                <template v-if="onCompleteAction === 'webhook'">
                  <div class="space-y-3 p-3 border rounded-lg bg-muted/30">
                    <div class="flex gap-2">
                      <div class="w-20">
                        <Label class="text-[10px]">{{ $t('flowBuilder.method') }}</Label>
                        <Select v-model="completionConfig.method">
                          <SelectTrigger class="h-7 text-xs"><SelectValue /></SelectTrigger>
                          <SelectContent>
                            <SelectItem value="GET">GET</SelectItem>
                            <SelectItem value="POST">POST</SelectItem>
                            <SelectItem value="PUT">PUT</SelectItem>
                            <SelectItem value="PATCH">PATCH</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <div class="flex-1">
                        <Label class="text-[10px]">URL</Label>
                        <Input v-model="completionConfig.url" placeholder="https://example.com/hook" class="h-7 text-xs font-mono" />
                      </div>
                    </div>
                    <div class="space-y-2">
                      <div class="flex items-center justify-between">
                        <Label class="text-[10px]">{{ $t('flowBuilder.headers') }}</Label>
                        <Button variant="ghost" size="sm" class="h-5 text-[10px] px-1" @click="addCompletionHeader">
                          <Plus class="h-3 w-3" />
                        </Button>
                      </div>
                      <div v-for="(val, key) in completionConfig.headers" :key="key" class="flex gap-1">
                        <Input
                          :model-value="String(key)"
                          @update:model-value="(v: string) => updateCompletionHeaderKey(String(key), v)"
                          placeholder="Key"
                          class="h-6 text-[10px] flex-1"
                        />
                        <Input
                          :model-value="String(val)"
                          @update:model-value="(v: string) => updateCompletionHeaderValue(String(key), v)"
                          placeholder="Value"
                          class="h-6 text-[10px] flex-1"
                        />
                        <Button variant="ghost" size="icon" class="h-5 w-5" @click="removeCompletionHeader(String(key))">
                          <Trash2 class="h-3 w-3 text-destructive" />
                        </Button>
                      </div>
                    </div>
                    <div class="space-y-1">
                      <Label class="text-[10px]">Body</Label>
                      <Textarea v-model="completionConfig.body" :rows="2" class="text-[10px] font-mono" />
                    </div>
                  </div>
                </template>
              </CollapsibleContent>
            </Collapsible>

            <Separator />

            <!-- Contact panel display config -->
            <Collapsible v-model:open="panelConfigOpen">
              <CollapsibleTrigger class="flex items-center justify-between w-full py-1 text-sm font-medium">
                Contact panel display
                <component :is="panelConfigOpen ? ChevronDown : ChevronRight" class="h-4 w-4" />
              </CollapsibleTrigger>
              <CollapsibleContent class="pt-3">
                <PanelConfigEditor
                  :panel-config="panelConfig"
                  :available-variables="availableVariables"
                  @update:panel-config="panelConfig = $event"
                />
              </CollapsibleContent>
            </Collapsible>

            <template v-if="!isNewFlow">
              <Separator />
              <Collapsible v-model:open="activityOpen">
                <CollapsibleTrigger class="flex items-center justify-between w-full py-1 text-sm font-medium">
                  Activity
                  <component :is="activityOpen ? ChevronDown : ChevronRight" class="h-4 w-4" />
                </CollapsibleTrigger>
                <CollapsibleContent class="pt-3 space-y-3">
                  <MetadataPanel
                    :created-at="createdAt"
                    :updated-at="updatedAt"
                    :created-by-name="createdByName"
                    :updated-by-name="updatedByName"
                  />
                  <AuditLogPanel :key="auditRefreshKey" resource-type="chatbot_flow" :resource-id="flowId" />
                </CollapsibleContent>
              </Collapsible>
            </template>
          </div>
        </ScrollArea>
      </Card>
    </div>

    <!-- Preview overlay -->
    <Dialog v-model:open="showPreview">
      <DialogContent class="max-w-[1100px] w-[95vw] h-[92vh] p-0 flex flex-col">
        <DialogTitle class="sr-only">Flow preview</DialogTitle>
        <InteractivePreview
          :graph="previewGraph"
          :flow-data="{ name, description, trigger_keywords: triggerKeywords, initial_message: initialMessage, completion_message: completionMessage, enabled, steps: [] } as any"
        />
      </DialogContent>
    </Dialog>

    <!-- Dialogs -->
    <ConfirmDialog
      v-model:open="showDeleteNodeConfirm"
      :title="$t('flowBuilder.deleteNodeConfirmTitle', 'Delete node?')"
      :description="$t('flowBuilder.deleteNodeConfirmDesc', 'The node and all its connections will be removed.')"
      :confirm-label="$t('common.delete')"
      variant="destructive"
      @confirm="confirmDeleteSelectedNode"
    />
    <UnsavedChangesDialog v-model:open="cancelDialogOpen" @confirm="confirmCancel" />
  </div>
</template>
