<script setup lang="ts">
import { ref, computed, onMounted, markRaw } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { VueFlow, useVueFlow, MarkerType, type NodeMouseEvent, type Edge, type EdgeMouseEvent, type Connection } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import { MiniMap } from '@vue-flow/minimap'
import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'
import '@vue-flow/minimap/dist/style.css'
import { useCallingStore } from '@/stores/calling'
import { useTeamsStore } from '@/stores/teams'
import { ivrFlowsService, type IVRNode, type IVREdge, type IVRFlowData, type IVRNodeType } from '@/services/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { ArrowLeft, Save, Volume2, Grid3X3, Hash, Globe, Users, ExternalLink, Clock, PhoneOff } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import IVRNodeProperties from '@/components/calling/IVRNodeProperties.vue'
import GreetingNode from '@/components/calling/nodes/GreetingNode.vue'
import MenuNode from '@/components/calling/nodes/MenuNode.vue'
import GatherNode from '@/components/calling/nodes/GatherNode.vue'
import HTTPCallbackNode from '@/components/calling/nodes/HTTPCallbackNode.vue'
import TransferNode from '@/components/calling/nodes/TransferNode.vue'
import GotoFlowNode from '@/components/calling/nodes/GotoFlowNode.vue'
import TimingNode from '@/components/calling/nodes/TimingNode.vue'
import HangupNode from '@/components/calling/nodes/HangupNode.vue'

const route = useRoute()
const router = useRouter()
const callingStore = useCallingStore()
const teamsStore = useTeamsStore()

const flowId = computed(() => route.params.id as string)
const flowName = ref('')
const isActive = ref(true)
const isCallStart = ref(false)
const isOutgoingEnd = ref(false)
const saving = ref(false)
const loading = ref(true)

// Vue Flow custom node types — cast to any to avoid strict NodeComponent type checks
const nodeTypes: any = {
  greeting: markRaw(GreetingNode),
  menu: markRaw(MenuNode),
  gather: markRaw(GatherNode),
  http_callback: markRaw(HTTPCallbackNode),
  transfer: markRaw(TransferNode),
  goto_flow: markRaw(GotoFlowNode),
  timing: markRaw(TimingNode),
  hangup: markRaw(HangupNode),
}

// Node palette
const palette: { type: IVRNodeType; label: string; icon: any; color: string }[] = [
  { type: 'greeting', label: 'Greeting', icon: Volume2, color: 'bg-green-600' },
  { type: 'menu', label: 'Menu', icon: Grid3X3, color: 'bg-purple-600' },
  { type: 'gather', label: 'Gather', icon: Hash, color: 'bg-blue-600' },
  { type: 'http_callback', label: 'HTTP', icon: Globe, color: 'bg-orange-600' },
  { type: 'transfer', label: 'Transfer', icon: Users, color: 'bg-amber-600' },
  { type: 'goto_flow', label: 'Goto Flow', icon: ExternalLink, color: 'bg-teal-600' },
  { type: 'timing', label: 'Timing', icon: Clock, color: 'bg-cyan-600' },
  { type: 'hangup', label: 'Hangup', icon: PhoneOff, color: 'bg-red-600' },
]

// Vue Flow instance
const { nodes, edges, addNodes, addEdges, removeNodes, removeEdges, onConnect, project, fitView } = useVueFlow({
  defaultEdgeOptions: {
    type: 'default',
    animated: true,
    markerEnd: MarkerType.ArrowClosed,
  },
})

// Selected node for properties panel
const selectedNodeId = ref<string | null>(null)

const selectedNode = computed(() => {
  if (!selectedNodeId.value) return null
  return nodes.value.find(n => n.id === selectedNodeId.value) || null
})

function onNodeClick(event: NodeMouseEvent) {
  selectedNodeId.value = event.node.id
}

function onPaneClick() {
  selectedNodeId.value = null
}

// Add node from palette
const entryNodeId = ref<string>('')

let nodeCounter = 0

function addNodeFromPalette(type: IVRNodeType) {
  // Place node at center of viewport
  const pos = project({ x: window.innerWidth / 2 - 100, y: window.innerHeight / 2 - 50 })
  const id = `node_${Date.now()}_${nodeCounter++}`
  const isFirst = nodes.value.length === 0

  const defaultConfigs: Record<IVRNodeType, Record<string, any>> = {
    greeting: { audio_file: '', interruptible: false },
    menu: { audio_file: '', timeout_seconds: 10, max_retries: 3, options: {} },
    gather: { audio_file: '', max_digits: 10, terminator: '#', timeout_seconds: 10, max_retries: 3, store_as: '' },
    http_callback: { url: '', method: 'GET', headers: {}, body_template: '', timeout_seconds: 10 },
    transfer: { team_id: '' },
    goto_flow: { flow_id: '' },
    timing: { schedule: [
      { day: 'monday', enabled: true, start_time: '09:00', end_time: '17:00' },
      { day: 'tuesday', enabled: true, start_time: '09:00', end_time: '17:00' },
      { day: 'wednesday', enabled: true, start_time: '09:00', end_time: '17:00' },
      { day: 'thursday', enabled: true, start_time: '09:00', end_time: '17:00' },
      { day: 'friday', enabled: true, start_time: '09:00', end_time: '17:00' },
      { day: 'saturday', enabled: false, start_time: '09:00', end_time: '17:00' },
      { day: 'sunday', enabled: false, start_time: '09:00', end_time: '17:00' },
    ]},
    hangup: { audio_file: '' },
  }

  addNodes([{
    id,
    type,
    position: { x: pos.x, y: pos.y },
    data: {
      label: type.replace('_', ' ').replace(/\b\w/g, (c: string) => c.toUpperCase()),
      config: defaultConfigs[type] || {},
      isEntryNode: isFirst,
    },
  }])

  if (isFirst) entryNodeId.value = id
}

// Handle new connections — one edge per output handle
onConnect((params) => {
  // Remove existing edge from this source handle (enforce single connection)
  const existing = edges.value.filter(
    e => e.source === params.source && e.sourceHandle === params.sourceHandle,
  )
  if (existing.length > 0) removeEdges(existing)

  addEdges([{
    ...params,
    type: 'default',
    animated: true,
    markerEnd: MarkerType.ArrowClosed,
    label: params.sourceHandle || 'default',
  }])
  spreadParallelLabels()
})

// Spread labels vertically when multiple edges share the same source→target.
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

// Select edge on click (Backspace/Delete will remove it)
function onEdgeClick({ edge }: EdgeMouseEvent) {
  // Deselect all nodes/edges, then select this edge
  nodes.value.forEach(n => (n.selected = false))
  edges.value.forEach(e => (e.selected = false))
  edge.selected = true
  selectedNodeId.value = null
}

// Handle edge reconnection (drag endpoint to a different node)
function onEdgeUpdate({ edge, connection }: { edge: Edge; connection: Connection }) {
  // Remove old edge, add new one
  removeEdges([edge])
  addEdges([{
    ...connection,
    type: 'default',
    animated: true,
    markerEnd: MarkerType.ArrowClosed,
    label: connection.sourceHandle || 'default',
  }])
  spreadParallelLabels()
}

// Update node data from properties panel
function onUpdateNode(updatedIVRNode: IVRNode) {
  const node = nodes.value.find(n => n.id === updatedIVRNode.id)
  if (!node) return

  node.data = {
    ...node.data,
    label: updatedIVRNode.label,
    config: updatedIVRNode.config,
  }
}

// Delete selected node
function deleteSelectedNode() {
  const node = selectedNode.value
  if (!node) return
  const nodeId = node.id

  // Remove connected edges first
  const connectedEdges = edges.value.filter(e => e.source === nodeId || e.target === nodeId)
  if (connectedEdges.length > 0) {
    removeEdges(connectedEdges)
  }

  removeNodes([nodeId])
  selectedNodeId.value = null

  // If the deleted node was the entry, pick another
  if (entryNodeId.value === nodeId && nodes.value.length > 0) {
    const newEntry = nodes.value[0]
    entryNodeId.value = newEntry.id
    newEntry.data = { ...newEntry.data, isEntryNode: true }
  }
}

// Convert Vue Flow state to IVRFlowData for saving
function toFlowData(): IVRFlowData {
  const ivrNodes: IVRNode[] = nodes.value.map(n => ({
    id: n.id,
    type: n.type as IVRNodeType,
    label: n.data?.label || '',
    position: { x: n.position.x, y: n.position.y },
    config: n.data?.config || {},
  }))

  const ivrEdges: IVREdge[] = edges.value.map(e => ({
    from: e.source,
    to: e.target,
    condition: e.sourceHandle || (e as any).label || 'default',
  }))

  // Find entry node: first node or the one without incoming edges
  const nodesWithIncoming = new Set(ivrEdges.map(e => e.to))
  const entryNode = ivrNodes.find(n => !nodesWithIncoming.has(n.id))?.id || ivrNodes[0]?.id || ''

  return {
    version: 2,
    nodes: ivrNodes,
    edges: ivrEdges,
    entry_node: entryNode,
  }
}

// Load flow data into Vue Flow
function loadFlowData(data: IVRFlowData) {
  entryNodeId.value = data.entry_node || ''

  const vfNodes = data.nodes.map(n => ({
    id: n.id,
    type: n.type,
    position: { x: n.position.x, y: n.position.y },
    data: {
      label: n.label,
      config: n.config,
      isEntryNode: n.id === data.entry_node,
    },
  }))

  const vfEdges = data.edges.map((e, idx) => ({
    id: `edge_${idx}`,
    source: e.from,
    target: e.to,
    sourceHandle: e.condition,
    type: 'default' as const,
    animated: true,
    markerEnd: MarkerType.ArrowClosed,
    label: e.condition !== 'default' ? e.condition : '',
  }))

  addNodes(vfNodes)
  addEdges(vfEdges)
  spreadParallelLabels()

  setTimeout(() => fitView({ padding: 0.2 }), 100)
}

// Save flow
async function saveFlow() {
  if (!flowName.value.trim()) {
    toast.error('Flow name is required')
    return
  }
  if (nodes.value.length === 0) {
    toast.error('Add at least one node')
    return
  }

  saving.value = true
  try {
    const flowData = toFlowData()
    const updated = await callingStore.updateIVRFlow(flowId.value, {
      name: flowName.value,
      is_active: isActive.value,
      is_call_start: isCallStart.value,
      is_outgoing_end: isOutgoingEnd.value,
      menu: flowData,
    })

    // Sync server-generated fields (e.g. TTS audio_file) back to canvas nodes
    const serverMenu = updated?.menu as IVRFlowData | undefined
    if (serverMenu?.nodes) {
      const serverConfigs = new Map(serverMenu.nodes.map((n: IVRNode) => [n.id, n.config]))
      for (const node of nodes.value) {
        const serverConfig = serverConfigs.get(node.id)
        if (serverConfig) {
          node.data = { ...node.data, config: serverConfig }
        }
      }
    }

    toast.success('Flow saved')
  } catch (e: any) {
    const msg = e?.response?.data?.message || 'Save failed'
    toast.error(msg)
  } finally {
    saving.value = false
  }
}

// Computed IVR node for properties panel
const selectedIVRNode = computed<IVRNode | null>(() => {
  const node = selectedNode.value
  if (!node) return null
  return {
    id: node.id,
    type: node.type as IVRNodeType,
    label: node.data?.label || '',
    position: node.position,
    config: node.data?.config || {},
  }
})

// Load flow on mount
onMounted(async () => {
  loading.value = true
  try {
    await Promise.all([callingStore.fetchIVRFlows(), teamsStore.fetchTeams()])
    const res = await ivrFlowsService.get(flowId.value)
    const flow = (res.data as any)?.data || res.data
    flowName.value = flow.name
    isActive.value = flow.is_active
    isCallStart.value = flow.is_call_start
    isOutgoingEnd.value = flow.is_outgoing_end

    if (flow.menu && flow.menu.version === 2) {
      loadFlowData(flow.menu)
    }
  } catch {
    toast.error('Failed to load flow')
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="h-screen flex flex-col">
    <!-- Toolbar -->
    <div class="flex items-center gap-3 px-4 py-2 border-b bg-background shrink-0">
      <Button variant="ghost" size="icon" class="h-8 w-8" @click="router.push({ name: 'ivr-flows' })">
        <ArrowLeft class="h-4 w-4" />
      </Button>
      <Input v-model="flowName" placeholder="Flow Name" class="h-8 text-sm max-w-[250px]" />
      <div class="flex items-center gap-2 ml-4">
        <Switch v-model:checked="isActive" />
        <Label class="text-xs whitespace-nowrap">Active</Label>
      </div>
      <div class="flex items-center gap-2 ml-2">
        <Switch v-model:checked="isCallStart" :disabled="!isActive" />
        <Label class="text-xs whitespace-nowrap">Incoming Call Start</Label>
      </div>
      <div class="flex items-center gap-2 ml-2">
        <Switch v-model:checked="isOutgoingEnd" :disabled="!isActive" />
        <Label class="text-xs whitespace-nowrap">Outgoing Post-Call</Label>
      </div>
      <div class="flex-1" />
      <Button :disabled="saving" size="sm" @click="saveFlow">
        <Save class="h-4 w-4 mr-1" />
        {{ saving ? 'Saving...' : 'Save' }}
      </Button>
    </div>

    <!-- Node Palette -->
    <div class="flex items-center gap-2 px-4 py-2 border-b bg-muted/30 overflow-x-auto shrink-0">
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

    <!-- Main content -->
    <div class="flex-1 flex overflow-hidden">
      <!-- Canvas -->
      <div class="flex-1 relative">
        <div v-if="loading" class="absolute inset-0 flex items-center justify-center bg-background/80 z-10">
          <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
        </div>
        <VueFlow
          :node-types="nodeTypes"
          :default-viewport="{ x: 0, y: 0, zoom: 1 }"
          :snap-to-grid="true"
          :snap-grid="[20, 20]"
          :edges-updatable="true"
          :delete-key-code="['Backspace', 'Delete']"
          class="h-full"
          @node-click="onNodeClick"
          @pane-click="onPaneClick"
          @edge-update="onEdgeUpdate"
          @edge-click="onEdgeClick"
        >
          <Background :gap="20" />
          <Controls />
          <MiniMap />
        </VueFlow>
      </div>

      <!-- Properties Panel -->
      <div
        v-if="selectedIVRNode"
        class="w-[320px] border-l bg-background overflow-y-auto shrink-0"
      >
        <IVRNodeProperties
          :node="selectedIVRNode"
          :current-flow-id="flowId"
          @update:node="onUpdateNode"
          @delete="deleteSelectedNode"
        />
      </div>
    </div>
  </div>
</template>

