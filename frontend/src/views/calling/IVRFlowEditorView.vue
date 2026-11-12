<script setup lang="ts">
import { ref, computed, onMounted, markRaw } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useVueFlow, MarkerType, type NodeMouseEvent, type Edge, type EdgeMouseEvent, type Connection } from '@vue-flow/core'
import FlowCanvas from '@/components/shared/FlowCanvas.vue'
import { useCallingStore } from '@/stores/calling'
import { useTeamsStore } from '@/stores/teams'
import { ivrFlowsService, type IVRNode, type IVREdge, type IVRFlowData, type IVRNodeType } from '@/services/api'
import { ArrowLeft, Save, Volume2, Grid3X3, Hash, Globe, Users, ExternalLink, Clock, PhoneOff } from '@/lib/icons'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import { toast } from '@/lib/toast'
import ConfirmDialog from '@/components/shared/ConfirmDialog.vue'
import ErrorState from '@/components/shared/ErrorState.vue'
import IVRNodeProperties from '@/components/calling/IVRNodeProperties.vue'
import GreetingNode from '@/components/calling/nodes/GreetingNode.vue'
import MenuNode from '@/components/calling/nodes/MenuNode.vue'
import GatherNode from '@/components/calling/nodes/GatherNode.vue'
import HTTPCallbackNode from '@/components/calling/nodes/HTTPCallbackNode.vue'
import TransferNode from '@/components/calling/nodes/TransferNode.vue'
import GotoFlowNode from '@/components/calling/nodes/GotoFlowNode.vue'
import TimingNode from '@/components/calling/nodes/TimingNode.vue'
import HangupNode from '@/components/calling/nodes/HangupNode.vue'

const { t } = useI18n()
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
const auditRefreshKey = ref(0)
const flowCreatedAt = ref('')
const flowUpdatedAt = ref('')
const flowCreatedByName = ref('')
const flowUpdatedByName = ref('')
const loading = ref(true)
const loadError = ref(false)

// Node deletion confirmation
const showDeleteNodeConfirm = ref(false)

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
const palette: { type: IVRNodeType; label: string; icon: any }[] = [
  { type: 'greeting', label: 'Greeting', icon: Volume2 },
  { type: 'menu', label: 'Menu', icon: Grid3X3 },
  { type: 'gather', label: 'Gather', icon: Hash },
  { type: 'http_callback', label: 'HTTP', icon: Globe },
  { type: 'transfer', label: 'Transfer', icon: Users },
  { type: 'goto_flow', label: 'Goto Flow', icon: ExternalLink },
  { type: 'timing', label: 'Timing', icon: Clock },
  { type: 'hangup', label: 'Hangup', icon: PhoneOff },
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

  nodes.value.forEach(n => (n.selected = false))
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

  const created = nodes.value.find((n) => n.id === id)
  if (created) created.selected = true
  selectedNodeId.value = id
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
      group[i].labelStyle = { transform: `translateY(${yOffset}px)`, fill: 'var(--foreground)' }
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

// Delete selected node — show confirmation first
function requestDeleteSelectedNode() {
  if (!selectedNode.value) return
  showDeleteNodeConfirm.value = true
}

function confirmDeleteSelectedNode() {
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
  showDeleteNodeConfirm.value = false

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
    toast.error(t('calling.nameRequired'))
    return
  }
  if (nodes.value.length === 0) {
    toast.error(t('calling.noIVRFlows'))
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

    toast.success(t('calling.flowUpdated'))
    auditRefreshKey.value++
  } catch (e: any) {
    const msg = e?.response?.data?.message || t('calling.flowSaveFailed')
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

// Load flow data from server
async function loadFlow() {
  loading.value = true
  loadError.value = false
  try {
    await Promise.all([callingStore.fetchIVRFlows(), teamsStore.fetchTeams()])
    const res = await ivrFlowsService.get(flowId.value)
    const flow = (res.data as any)?.data || res.data
    flowName.value = flow.name
    isActive.value = flow.is_active
    isCallStart.value = flow.is_call_start
    isOutgoingEnd.value = flow.is_outgoing_end
    flowCreatedAt.value = flow.created_at || ''
    flowUpdatedAt.value = flow.updated_at || ''
    flowCreatedByName.value = flow.created_by?.full_name || ''
    flowUpdatedByName.value = flow.updated_by?.full_name || ''

    if (flow.menu && flow.menu.version === 2) {
      loadFlowData(flow.menu)
    }
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

// Load flow on mount
onMounted(() => {
  loadFlow()
})
</script>

<template>
  <div class="flex flex-col h-full min-h-0">
    <div class="vstack gap-4 p-4 shrink-0">
      <!-- Toolbar -->
      <div class="hstack gap-4 flex-wrap">
        <button
          type="button"
          class="ghost icon"
          :aria-label="t('calling.backToFlows')"
          :title="t('calling.backToFlows')"
          @click="router.push({ name: 'ivr-flows' })"
        >
          <ArrowLeft />
        </button>
        <input v-model="flowName" class="flex-1 min-w-0" placeholder="Flow Name" aria-label="Flow Name">
        <label class="hstack gap-2">
          <input v-model="isActive" type="checkbox" role="switch">
          <small>Active</small>
        </label>
        <label class="hstack gap-2">
          <input v-model="isCallStart" type="checkbox" role="switch" :disabled="!isActive">
          <small>Incoming Call Start</small>
        </label>
        <label class="hstack gap-2">
          <input v-model="isOutgoingEnd" type="checkbox" role="switch" :disabled="!isActive">
          <small>Outgoing Post-Call</small>
        </label>
        <button type="button" class="small" :disabled="saving" :aria-busy="saving ? 'true' : undefined" data-spinner="small" @click="saveFlow">
          <Save />
          {{ saving ? t('calling.flowSaving') : t('calling.flowSave') }}
        </button>
      </div>

      <!-- Node Palette -->
      <div class="hstack gap-2 scroll-x" role="toolbar" :aria-label="t('flowBuilder.add')">
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

    <!-- Main content -->
    <div class="flex flex-1 min-h-0">
      <!-- Canvas -->
      <div class="flex-1 relative min-w-0">
        <FlowCanvas
          :node-types="nodeTypes"
          edge-type="default"
          @node-click="onNodeClick"
          @pane-click="onPaneClick"
          @edge-update="onEdgeUpdate"
          @edge-click="onEdgeClick"
        />
        <div v-if="loading" class="absolute-fill card flex items-center justify-center">
          <div role="status" aria-busy="true" data-spinner="large" :aria-label="t('common.loading')" />
        </div>
        <div v-else-if="loadError" class="absolute-fill card flex items-center justify-center">
          <ErrorState :title="t('calling.flowLoadFailed')" :description="t('calling.flowLoadFailedDesc')">
            <template #action>
              <div class="hstack gap-2 justify-center">
                <button type="button" class="outline small" @click="router.push({ name: 'ivr-flows' })">
                  {{ t('calling.goBack') }}
                </button>
                <button type="button" class="small" @click="loadFlow">
                  {{ t('common.retry') }}
                </button>
              </div>
            </template>
          </ErrorState>
        </div>
      </div>

      <!-- Right Panel -->
      <aside class="side-panel shrink-0 scroll-y card p-4 min-w-0">
        <!-- Node Properties (when a node is selected) -->
        <IVRNodeProperties
          v-if="selectedIVRNode"
          :node="selectedIVRNode"
          :current-flow-id="flowId"
          @update:node="onUpdateNode"
          @delete="requestDeleteSelectedNode"
        />

        <!-- Metadata + Activity Log (when no node is selected) -->
        <div v-else class="vstack gap-4 min-w-0">
          <MetadataPanel
            :created-at="flowCreatedAt"
            :updated-at="flowUpdatedAt"
            :created-by-name="flowCreatedByName"
            :updated-by-name="flowUpdatedByName"
          />
          <AuditLogPanel :key="auditRefreshKey" resource-type="ivr_flow" :resource-id="flowId" />
        </div>
      </aside>
    </div>

    <!-- Node Delete Confirmation -->
    <ConfirmDialog
      v-model:open="showDeleteNodeConfirm"
      :title="t('calling.deleteNodeConfirmTitle')"
      :description="t('calling.deleteNodeConfirmDesc')"
      :confirm-label="t('common.delete')"
      variant="destructive"
      @confirm="confirmDeleteSelectedNode"
    />
  </div>
</template>
