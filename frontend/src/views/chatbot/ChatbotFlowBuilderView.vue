<script setup lang="ts">
import { ref, computed, onMounted, markRaw, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useVueFlow, MarkerType, type NodeMouseEvent, type Edge, type EdgeMouseEvent, type Connection } from '@vue-flow/core'
import { toast } from '@/lib/toast'

import FlowCanvas from '@/components/shared/FlowCanvas.vue'
import { chatbotService } from '@/services/api'
import type { ChatFlowGraph, ChatNode, ChatEdge, ChatNodeType } from '@/services/api'

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
  Plus,
  Trash2,
  Play,
} from '@/lib/icons'

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
import Modal from '@/components/oat/Modal.vue'

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
// The activity log loads only once its section is expanded.
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
const palette: { type: ChatNodeType; label: string; icon: any }[] = [
  { type: 'message', label: 'Text', icon: MessageSquare },
  { type: 'buttons', label: 'Buttons', icon: MousePointerClick },
  { type: 'api_call', label: 'API', icon: Globe },
  { type: 'whatsapp_flow', label: 'WA Flow', icon: MessageCircle },
  { type: 'transfer', label: 'Transfer', icon: Users },
  { type: 'condition', label: 'Condition', icon: GitBranch },
  { type: 'timing', label: 'Timing', icon: Clock },
  { type: 'goto_flow', label: 'Go to Flow', icon: ExternalLink },
  { type: 'end', label: 'End', icon: StopCircle },
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
  nodes.value.forEach((n) => (n.selected = false))
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
  const created = nodes.value.find((n) => n.id === id)
  if (created) created.selected = true
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
      group[i].labelStyle = { transform: `translateY(${yOffset}px)`, fill: 'var(--foreground)' }
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

function val(e: Event): string {
  return (e.target as HTMLInputElement).value
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
  <div class="flex flex-col h-full min-h-0">
    <div class="vstack gap-4 p-4 shrink-0">
    <!-- Header -->
    <header class="hstack gap-4 flex-wrap">
      <button type="button" class="ghost icon" :aria-label="$t('common.back')" :title="$t('common.back')" @click="handleCancel">
        <ArrowLeft :size="20" />
      </button>

      <label class="hstack gap-2 flex-1 min-w-0">
        <small class="text-light">{{ $t('flowBuilder.name') }}</small>
        <input v-model="name" :placeholder="$t('flowBuilder.namePlaceholder')">
      </label>
      <label class="hstack gap-2 flex-1 min-w-0">
        <small class="text-light">{{ $t('flowBuilder.description') }}</small>
        <input v-model="description" :placeholder="$t('flowBuilder.optional')">
      </label>

      <div class="hstack gap-2 flex-wrap">
        <label class="hstack gap-2">
          <input v-model="enabled" type="checkbox" role="switch">
          <small>{{ enabled ? $t('flowBuilder.enabled') : $t('flowBuilder.disabled') }}</small>
        </label>

        <button type="button" class="outline small" :disabled="nodes.length === 0" @click="showPreview = true">
          <Play />
          {{ $t('flowBuilder.preview', 'Preview') }}
        </button>
        <button type="button" class="outline small" @click="handleCancel">{{ $t('flowBuilder.cancel') }}</button>
        <button type="button" class="small" :disabled="isSaving" :aria-busy="isSaving ? 'true' : undefined" data-spinner="small" @click="saveFlow">
          <Save />
          {{ isSaving ? $t('flowBuilder.saving') + '...' : $t('flowBuilder.saveFlow') }}
        </button>
      </div>
    </header>

    <!-- Node palette -->
    <div class="hstack gap-2 scroll-x" role="toolbar" aria-labelledby="chat-flow-palette-label">
      <small id="chat-flow-palette-label" class="text-light shrink-0">Add node:</small>
      <button
        v-for="p in palette"
        :key="p.type"
        type="button"
        class="outline small shrink-0"
        @click="addNodeFromPalette(p.type)"
      >
        <component :is="p.icon" />
        {{ p.label }}
      </button>
    </div>
    </div>

    <!-- Main: canvas + right panel -->
    <div class="flex flex-1 min-h-0">
      <!-- Canvas -->
      <div class="flex-1 relative min-w-0">
        <FlowCanvas
          :node-types="nodeTypes"
          edge-type="default"
          @node-click="onNodeClick"
          @pane-click="onPaneClick"
          @edge-click="onEdgeClick"
          @edge-update="onEdgeUpdate"
          @node-drag-stop="onNodeDragStop"
        />
        <div v-if="isLoading" class="absolute-fill card flex items-center justify-center">
          <div role="status" aria-busy="true" data-spinner="large" :aria-label="$t('flowBuilder.loading')" />
        </div>
        <div v-else-if="loadError" class="absolute-fill card flex items-center justify-center">
          <ErrorState
            :title="$t('flowBuilder.loadFailed', 'Failed to load flow')"
            :description="$t('flowBuilder.loadFailedDesc', 'Try reloading or go back to the flow list.')"
          >
            <template #action>
              <div class="hstack gap-2 justify-center">
                <button type="button" class="outline small" @click="router.push('/chatbot/flows')">
                  {{ $t('common.goBack', 'Go back') }}
                </button>
                <button type="button" class="small" @click="loadFlow">
                  {{ $t('common.retry') }}
                </button>
              </div>
            </template>
          </ErrorState>
        </div>
      </div>

      <!-- Right panel -->
      <aside class="side-panel shrink-0 scroll-y card p-4 min-w-0">
        <!-- Node properties when a node is selected -->
        <ChatNodeProperties
          v-if="selectedChatNode && selectedChatNode.type !== 'start'"
          :node="selectedChatNode"
          :current-flow-id="flowId"
          :available-flows="availableFlows"
          @update:node="onUpdateNode"
          @delete="requestDeleteSelectedNode"
        />

        <!-- Flow settings when nothing is selected -->
        <div v-else class="min-w-0">
          <h4 class="mb-4">{{ $t('flowBuilder.flowSettings') }}</h4>

          <!-- Trigger keywords -->
          <label data-field>
            {{ $t('flowBuilder.triggerKeywords') }}
            <input v-model="triggerKeywords" :placeholder="$t('flowBuilder.triggerKeywordsPlaceholder')">
            <small data-hint>{{ $t('flowBuilder.triggerKeywordsHint') }}</small>
          </label>

          <hr>

          <!-- Initial message -->
          <label data-field>
            {{ $t('flowBuilder.initialMessage') }}
            <textarea v-model="initialMessage" :placeholder="$t('flowBuilder.initialMessagePlaceholder')" rows="2" />
          </label>

          <!-- Completion message -->
          <label data-field>
            {{ $t('flowBuilder.completionMessage') }}
            <textarea v-model="completionMessage" :placeholder="$t('flowBuilder.completionMessagePlaceholder')" rows="2" />
          </label>

          <!-- On complete action -->
          <details>
            <summary>{{ $t('flowBuilder.onCompletion') }}</summary>
            <div>
              <label data-field>
                {{ $t('flowBuilder.action') }}
                <select v-model="onCompleteAction">
                  <option value="none">{{ $t('flowBuilder.noAction') }}</option>
                  <option value="webhook">{{ $t('flowBuilder.sendToWebhook') }}</option>
                </select>
              </label>

              <template v-if="onCompleteAction === 'webhook'">
                <div class="hstack gap-2">
                  <label data-field>
                    {{ $t('flowBuilder.method') }}
                    <select v-model="completionConfig.method">
                      <option value="GET">GET</option>
                      <option value="POST">POST</option>
                      <option value="PUT">PUT</option>
                      <option value="PATCH">PATCH</option>
                    </select>
                  </label>
                  <label data-field class="flex-1 min-w-0">
                    URL
                    <input v-model="completionConfig.url" type="url" placeholder="https://example.com/hook">
                  </label>
                </div>
                <div class="vstack gap-2 mb-4">
                  <div class="hstack justify-between">
                    <span>{{ $t('flowBuilder.headers') }}</span>
                    <button
                      type="button"
                      class="ghost small icon"
                      :aria-label="$t('common.add')"
                      :title="$t('common.add')"
                      @click="addCompletionHeader"
                    >
                      <Plus />
                    </button>
                  </div>
                  <div v-for="(v, key) in completionConfig.headers" :key="key" class="hstack gap-1">
                    <input
                      class="flex-1 min-w-0"
                      :value="String(key)"
                      placeholder="Key"
                      aria-label="Key"
                      @input="updateCompletionHeaderKey(String(key), val($event))"
                    >
                    <input
                      class="flex-1 min-w-0"
                      :value="String(v)"
                      placeholder="Value"
                      aria-label="Value"
                      @input="updateCompletionHeaderValue(String(key), val($event))"
                    >
                    <button
                      type="button"
                      class="ghost small icon"
                      data-variant="danger"
                      :aria-label="$t('common.delete')"
                      :title="$t('common.delete')"
                      @click="removeCompletionHeader(String(key))"
                    >
                      <Trash2 />
                    </button>
                  </div>
                </div>
                <label data-field>
                  Body
                  <textarea v-model="completionConfig.body" rows="2" />
                </label>
              </template>
            </div>
          </details>

          <!-- Contact panel display config -->
          <details>
            <summary>Contact panel display</summary>
            <div>
              <PanelConfigEditor
                :panel-config="panelConfig"
                :available-variables="availableVariables"
                @update:panel-config="panelConfig = $event"
              />
            </div>
          </details>

          <details v-if="!isNewFlow" @toggle="activityOpen = ($event.target as HTMLDetailsElement).open">
            <summary>Activity</summary>
            <div v-if="activityOpen" class="vstack gap-4">
              <MetadataPanel
                :created-at="createdAt"
                :updated-at="updatedAt"
                :created-by-name="createdByName"
                :updated-by-name="updatedByName"
              />
              <AuditLogPanel :key="auditRefreshKey" resource-type="chatbot_flow" :resource-id="flowId" />
            </div>
          </details>
        </div>
      </aside>
    </div>

    <!-- Preview overlay -->
    <Modal v-model:open="showPreview" class="modal-wide" :title="$t('flowBuilder.previewTitle')">
      <div v-if="showPreview" class="h-preview flex">
        <InteractivePreview
          :graph="previewGraph"
          :flow-data="{ name, description, trigger_keywords: triggerKeywords, initial_message: initialMessage, completion_message: completionMessage, enabled, steps: [] } as any"
        />
      </div>
    </Modal>

    <!-- Dialogs -->
    <ConfirmDialog
      v-model:open="showDeleteNodeConfirm"
      :title="$t('flowBuilder.deleteNodeConfirmTitle', 'Delete node?')"
      :description="$t('flowBuilder.deleteNodeConfirmDesc', 'The node and all its connections will be removed.')"
      :confirm-label="$t('common.delete')"
      variant="destructive"
      @confirm="confirmDeleteSelectedNode"
    />
    <UnsavedChangesDialog :open="cancelDialogOpen" @stay="cancelDialogOpen = false" @leave="confirmCancel" />
  </div>
</template>
