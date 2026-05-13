<script setup lang="ts">
import { ref, watch, markRaw, nextTick } from 'vue'
import { useVueFlow, MarkerType } from '@vue-flow/core'
import type { Node, Edge, NodeMouseEvent, Connection } from '@vue-flow/core'
import { Button } from '@/components/ui/button'
import { GitBranch, Play, Plus, MessageSquare, MousePointerClick, Globe, MessageCircle, UserPlus } from 'lucide-vue-next'
import { stepsToNodesAndEdges, extractCanvasLayout } from '@/composables/useChatbotFlowConverter'
import type { CanvasLayout } from '@/composables/useChatbotFlowConverter'
import FlowCanvas from '@/components/shared/FlowCanvas.vue'
import ChatbotTextNode from '@/components/chatbot/nodes/ChatbotTextNode.vue'
import ChatbotButtonsNode from '@/components/chatbot/nodes/ChatbotButtonsNode.vue'
import ChatbotApiNode from '@/components/chatbot/nodes/ChatbotApiNode.vue'
import ChatbotWhatsAppFlowNode from '@/components/chatbot/nodes/ChatbotWhatsAppFlowNode.vue'
import ChatbotTransferNode from '@/components/chatbot/nodes/ChatbotTransferNode.vue'

interface FlowChartStep {
  step_name: string
  step_order: number
  message: string
  message_type: string
  input_type: string
  buttons: { id: string; title: string; type?: string }[]
  conditional_next?: Record<string, string>
  next_step: string
  [key: string]: any
}

const props = defineProps<{
  steps: FlowChartStep[]
  selectedStepIndex: number | null
  flowName: string
  initialMessage: string
  completionMessage: string
  teams?: { id: string; name: string }[]
  canvasLayout?: CanvasLayout
}>()

const emit = defineEmits<{
  selectStep: [index: number]
  addStep: [type?: string]
  selectFlowSettings: []
  openPreview: []
  connectSteps: [sourceStep: string, targetStep: string, sourceHandle: string]
  disconnectSteps: [sourceStep: string, sourceHandle: string]
  changeStepType: [stepIndex: number, newType: string]
  updateCanvasLayout: [layout: CanvasLayout]
}>()

const messageTypePalette = [
  { type: 'text', label: 'Text', icon: MessageSquare, color: 'bg-blue-600' },
  { type: 'buttons', label: 'Buttons', icon: MousePointerClick, color: 'bg-purple-600' },
  { type: 'api_fetch', label: 'API Fetch', icon: Globe, color: 'bg-orange-600' },
  { type: 'whatsapp_flow', label: 'WA Flow', icon: MessageCircle, color: 'bg-green-600' },
  { type: 'transfer', label: 'Transfer', icon: UserPlus, color: 'bg-amber-600' },
]

// Track which step is selected on the canvas (index)
const selectedOnCanvas = ref<number | null>(null)

function getSelectedStepType(): string | null {
  if (selectedOnCanvas.value === null) return null
  const sorted = [...(props.steps || [])].sort((a, b) => a.step_order - b.step_order)
  return sorted[selectedOnCanvas.value]?.message_type || null
}

const nodeTypes: Record<string, any> = {
  chatbot_text: markRaw(ChatbotTextNode),
  chatbot_buttons: markRaw(ChatbotButtonsNode),
  chatbot_api: markRaw(ChatbotApiNode),
  chatbot_api_fetch: markRaw(ChatbotApiNode),
  chatbot_whatsapp_flow: markRaw(ChatbotWhatsAppFlowNode),
  chatbot_transfer: markRaw(ChatbotTransferNode),
}

const flowNodes = ref<Node[]>([])
const flowEdges = ref<Edge[]>([])
// Prevent rebuild while we're applying a connection change from the canvas
let skipNextRebuild = false

const { fitView } = useVueFlow()

function rebuildGraph() {
  if (skipNextRebuild) {
    skipNextRebuild = false
    return
  }

  const steps = props.steps || []
  if (steps.length === 0) {
    flowNodes.value = []
    flowEdges.value = []
    return
  }

  const { nodes: n, edges: e } = stepsToNodesAndEdges(steps as any, props.canvasLayout)

  // Build team name lookup and inject into transfer nodes
  const teamMap = new Map<string, string>()
  if (props.teams) {
    for (const t of props.teams) {
      teamMap.set(t.id, t.name)
    }
  }
  n.forEach((node) => {
    if (node.type === 'chatbot_transfer' && node.data?.config?.transfer_config?.team_id) {
      const teamId = node.data.config.transfer_config.team_id
      node.data.config.transfer_config.team_name = teamMap.get(teamId) || ''
    }
  })

  // Mark selected
  const sorted = [...steps].sort((a, b) => a.step_order - b.step_order)
  n.forEach((node) => {
    const stepIdx = sorted.findIndex((s) => s.step_name === node.id)
    const isSelected = props.selectedStepIndex !== null && stepIdx === props.selectedStepIndex
    node.data = { ...node.data, selected: isSelected }
    node.class = isSelected ? 'selected-node' : ''
  })

  flowNodes.value = n
  flowEdges.value = e
  nextTick(() => fitView({ padding: 0.2 }))
}

// Rebuild when steps change
watch(() => props.steps, rebuildGraph, { immediate: true, deep: true })

// Update selection highlight
watch(
  () => props.selectedStepIndex,
  (idx) => {
    const sorted = [...(props.steps || [])].sort((a, b) => a.step_order - b.step_order)
    flowNodes.value = flowNodes.value.map((node) => {
      const stepIdx = sorted.findIndex((s) => s.step_name === node.id)
      const isSelected = idx !== null && stepIdx === idx
      return {
        ...node,
        data: { ...node.data, selected: isSelected },
        class: isSelected ? 'selected-node' : '',
      }
    })
  },
)

function onNodeClick(event: NodeMouseEvent) {
  const sorted = [...(props.steps || [])].sort((a, b) => a.step_order - b.step_order)
  const idx = sorted.findIndex((s) => s.step_name === event.node.id)
  if (idx !== -1) {
    selectedOnCanvas.value = idx
    emit('selectStep', idx)
  }
}

function onPaneClick() {
  selectedOnCanvas.value = null
  emit('selectFlowSettings')
}

function onPaletteClick(type: string) {
  if (selectedOnCanvas.value !== null) {
    emit('changeStepType', selectedOnCanvas.value, type)
  }
}

function onConnect(connection: Connection) {
  if (!connection.source || !connection.target) return
  const handle = connection.sourceHandle || 'default'

  // Remove existing edge from same source handle (enforce single connection per handle)
  flowEdges.value = flowEdges.value.filter(
    (e) => !(e.source === connection.source && e.sourceHandle === handle)
  )

  // Find button title for label
  let label = ''
  if (handle !== 'default') {
    const sourceStep = (props.steps || []).find((s) => s.step_name === connection.source)
    const btn = sourceStep?.buttons?.find((b: any) => b.id === handle)
    if (btn) label = btn.title || handle
  }

  // Add new edge
  flowEdges.value = [
    ...flowEdges.value,
    {
      id: `e-${connection.source}-${connection.target}-${handle}`,
      source: connection.source,
      target: connection.target,
      sourceHandle: handle,
      targetHandle: connection.targetHandle || undefined,
      label,
      animated: true,
      markerEnd: MarkerType.ArrowClosed,
    },
  ]

  // Tell parent to update the step data
  skipNextRebuild = true
  emit('connectSteps', connection.source, connection.target, handle)
}

function onNodeDragStop() {
  emit('updateCanvasLayout', extractCanvasLayout(flowNodes.value))
}

function onEdgeRemove(edges: Edge[]) {
  for (const edge of edges) {
    const handle = edge.sourceHandle || 'default'
    skipNextRebuild = true
    emit('disconnectSteps', edge.source, handle)
  }
}
</script>

<template>
  <div class="h-full flex flex-col overflow-hidden">
    <!-- Header / Toolbar -->
    <div class="px-4 py-3 border-b flex items-center justify-between flex-shrink-0">
      <div class="flex items-center gap-2">
        <GitBranch class="h-4 w-4 text-muted-foreground" />
        <span class="text-sm font-medium">Flow Diagram</span>
      </div>
      <div class="flex items-center gap-2">
        <Button variant="outline" size="sm" @click="emit('openPreview')">
          <Play class="h-4 w-4 mr-1" />
          Preview
        </Button>
      </div>
    </div>

    <!-- Message Type Palette -->
    <div class="flex items-center gap-2 px-4 py-2 border-b bg-muted/30 overflow-x-auto shrink-0">
      <span class="text-xs text-muted-foreground shrink-0">
        {{ selectedOnCanvas !== null ? 'Change type:' : 'Add step:' }}
      </span>
      <Button
        v-for="p in messageTypePalette"
        :key="p.type"
        :variant="getSelectedStepType() === p.type ? 'active' : 'outline'"
        size="sm"
        class="h-7 text-xs gap-1.5 shrink-0"
        @click="selectedOnCanvas !== null ? onPaletteClick(p.type) : emit('addStep', p.type)"
      >
        <div :class="['w-2 h-2 rounded-full', p.color]" />
        <component :is="p.icon" class="w-3.5 h-3.5" />
        {{ p.label }}
      </Button>
    </div>

    <!-- Vue Flow Canvas -->
    <div class="flex-1">
      <FlowCanvas
        :nodes="flowNodes"
        :edges="flowEdges"
        :node-types="nodeTypes"
        edge-type="default"
        fit-view-on-init
        @update:nodes="flowNodes = $event"
        @update:edges="flowEdges = $event"
        @node-click="onNodeClick"
        @node-drag-stop="onNodeDragStop"
        @pane-click="onPaneClick"
        @connect="onConnect"
        @edges-change="(changes) => {
          const removals = changes.filter((c: any) => c.type === 'remove')
          if (removals.length) onEdgeRemove(removals.map((r: any) => flowEdges.find((e) => e.id === r.id)).filter(Boolean) as Edge[])
        }"
      />

      <!-- Empty state overlay -->
      <div
        v-if="steps.length === 0"
        class="absolute inset-0 flex items-center justify-center pointer-events-none"
      >
        <div
          class="w-72 py-12 rounded-xl border-2 border-dashed border-muted-foreground/30 flex flex-col items-center justify-center cursor-pointer hover:border-primary hover:bg-primary/5 transition-all pointer-events-auto"
          @click="emit('addStep')"
        >
          <Plus class="h-10 w-10 text-muted-foreground mb-3" />
          <span class="text-sm font-medium text-muted-foreground">Add your first step</span>
        </div>
      </div>
    </div>
  </div>
</template>
