import type { Node, Edge } from '@vue-flow/core'
import { MarkerType } from '@vue-flow/core'
import type { FlowStep } from '@/types/flow-preview'
import type { ChatFlowGraph, ChatNode, ChatEdge, ChatNodeType } from '@/services/api'

function getNodeType(messageType: string): string {
  return 'chatbot_' + messageType
}

// Human-readable fallback when a step has no explicit `label`. Used by
// stepsToNodesAndEdges so legacy flows show "Text" / "Buttons" on the
// canvas instead of the internal "step_1" / "step_2" identifiers.
const defaultLabels: Record<string, string> = {
  text: 'Text',
  buttons: 'Buttons',
  api_fetch: 'API Fetch',
  whatsapp_flow: 'WhatsApp Flow',
  transfer: 'Transfer',
  end: 'End',
  condition: 'Condition',
}

function fallbackLabel(step: { message_type: string; step_name: string }): string {
  return defaultLabels[step.message_type] || step.step_name
}

/**
 * Canvas layout storing node positions.
 */
export interface CanvasLayout {
  node_positions?: Record<string, { x: number; y: number }>
}

/**
 * Convert a flat array of FlowStep objects into Vue Flow nodes and edges.
 */
export function stepsToNodesAndEdges(steps: FlowStep[], canvasLayout?: CanvasLayout): { nodes: Node[]; edges: Edge[] } {
  if (!steps || steps.length === 0) {
    return { nodes: [], edges: [] }
  }

  // Sort steps by step_order
  const sorted = [...steps].sort((a, b) => a.step_order - b.step_order)

  // Build a set of step names that are targeted by non-sequential jumps (for offset)
  const nonSequentialTargets = new Set<string>()
  sorted.forEach((step, index) => {
    const nextSequentialName = index < sorted.length - 1 ? sorted[index + 1].step_name : null

    if (step.message_type === 'buttons' && step.conditional_next) {
      for (const targetStep of Object.values(step.conditional_next)) {
        if (targetStep && targetStep !== nextSequentialName) {
          nonSequentialTargets.add(targetStep)
        }
      }
    } else if (step.message_type !== 'transfer' && step.next_step) {
      if (step.next_step !== nextSequentialName) {
        nonSequentialTargets.add(step.next_step)
      }
    }
  })

  // Create nodes
  const savedPositions = canvasLayout?.node_positions || {}
  const nodes: Node[] = sorted.map((step, index) => {
    const saved = savedPositions[step.step_name]
    const isNonSequentialTarget = nonSequentialTargets.has(step.step_name)
    return {
      id: step.step_name,
      type: getNodeType(step.message_type),
      position: saved
        ? { x: saved.x, y: saved.y }
        : { x: isNonSequentialTarget ? 500 : 300, y: index * 150 },
      data: {
        label: step.label || fallbackLabel(step),
        config: { ...step },
        isEntryNode: index === 0,
      },
    }
  })

  // Create edges
  const edges: Edge[] = []
  const stepNameSet = new Set(sorted.map((s) => s.step_name))

  sorted.forEach((step, index) => {
    const nextSequentialStep = index < sorted.length - 1 ? sorted[index + 1].step_name : null

    if (step.message_type === 'transfer' || step.message_type === 'end') {
      // Terminal node -- no outgoing edges
      return
    }

    if (step.message_type === 'buttons') {
      const conditionalNext = step.conditional_next || {}
      const mappedButtonIds = new Set(Object.keys(conditionalNext))

      // Build a lookup of button id -> title for edge labels
      const buttonTitleMap = new Map<string, string>()
      if (step.buttons) {
        for (const btn of step.buttons) {
          buttonTitleMap.set(btn.id, btn.title || btn.id)
        }
      }

      // Edges for buttons that have explicit conditional_next entries
      for (const [buttonId, targetStep] of Object.entries(conditionalNext)) {
        if (targetStep && stepNameSet.has(targetStep)) {
          edges.push({
            id: `e-${step.step_name}-${targetStep}-${buttonId}`,
            source: step.step_name,
            target: targetStep,
            sourceHandle: buttonId,
            label: buttonTitleMap.get(buttonId) || buttonId,
            animated: true,
            markerEnd: MarkerType.ArrowClosed,
          })
        }
      }

      // Buttons without a conditional_next entry fall through to the next sequential step
      if (step.buttons && nextSequentialStep) {
        for (const btn of step.buttons) {
          const buttonId = btn.id
          if (buttonId && !mappedButtonIds.has(buttonId) && stepNameSet.has(nextSequentialStep)) {
            edges.push({
              id: `e-${step.step_name}-${nextSequentialStep}-${buttonId}`,
              source: step.step_name,
              target: nextSequentialStep,
              sourceHandle: buttonId,
              label: btn.title || buttonId,
              animated: true,
              markerEnd: MarkerType.ArrowClosed,
            })
          }
        }
      }
    } else if (step.next_step && stepNameSet.has(step.next_step)) {
      // Explicit next_step
      edges.push({
        id: `e-${step.step_name}-${step.next_step}-default`,
        source: step.step_name,
        target: step.next_step,
        sourceHandle: 'default',
        animated: true,
        markerEnd: MarkerType.ArrowClosed,
      })
    } else if (nextSequentialStep) {
      // Implicit fallthrough to next sequential step
      edges.push({
        id: `e-${step.step_name}-${nextSequentialStep}-default`,
        source: step.step_name,
        target: nextSequentialStep,
        sourceHandle: 'default',
        animated: true,
        markerEnd: MarkerType.ArrowClosed,
      })
    }
  })

  return { nodes, edges }
}

/**
 * Extract node positions from Vue Flow nodes for persistence.
 */
export function extractCanvasLayout(nodes: Node[]): CanvasLayout {
  const node_positions: Record<string, { x: number; y: number }> = {}
  for (const node of nodes) {
    node_positions[node.id] = { x: node.position.x, y: node.position.y }
  }
  return { node_positions }
}

// Message types in the v1 editor that map cleanly to v2 graph node types
// the backend executor implements today. Anything outside this set forces
// the flow to keep using the legacy steps[] wire format until the matching
// node type lands.
const V2_SUPPORTED_MESSAGE_TYPES = new Set(['text', 'buttons', 'end', 'condition'])

function messageTypeToNodeType(messageType: string): ChatNodeType | null {
  switch (messageType) {
    case 'text':
      return 'message'
    case 'buttons':
      return 'buttons'
    case 'end':
      return 'end'
    case 'condition':
      return 'condition'
    default:
      return null
  }
}

function nodeTypeToMessageType(nodeType: string): string | null {
  switch (nodeType) {
    case 'message':
      return 'text'
    case 'buttons':
      return 'buttons'
    case 'end':
      return 'end'
    case 'condition':
      return 'condition'
    default:
      return null
  }
}

// stepsToGraph reads only a subset of FlowStep fields; relaxing the
// parameter to a structural type lets callers pass FlowStep variants
// where message_type is `string` rather than the strict union (the chat
// flow builder defines a local relaxed FlowStep on top of v1 fields).
type StepInput = Pick<FlowStep, 'step_name' | 'step_order' | 'message' | 'next_step' | 'conditional_next' | 'buttons' | 'label' | 'input_config'> & { message_type: string }

/**
 * Convert the v1 FlowStep[] model to a v2 graph payload. Returns null
 * when any step uses a message_type the v2 executor does not yet support
 * — callers should fall back to saving as legacy steps in that case.
 */
export function stepsToGraph(steps: StepInput[], canvasLayout?: CanvasLayout): ChatFlowGraph | null {
  if (!steps || steps.length === 0) {
    return null
  }
  for (const step of steps) {
    if (!V2_SUPPORTED_MESSAGE_TYPES.has(step.message_type)) {
      return null
    }
  }

  const sorted = [...steps].sort((a, b) => a.step_order - b.step_order)
  const positions = canvasLayout?.node_positions || {}

  const nodes: ChatNode[] = sorted.map((step, index) => {
    const nodeType = messageTypeToNodeType(step.message_type)!
    const pos = positions[step.step_name] || { x: 300, y: index * 150 }

    const config: Record<string, any> = {}
    if (nodeType === 'message') {
      config.message = step.message
    } else if (nodeType === 'buttons') {
      config.body = step.message
      config.buttons = step.buttons || []
    } else if (nodeType === 'end' && step.message) {
      // Optional completion message; backend execChatEnd sends it before
      // marking the session complete.
      config.message = step.message
    } else if (nodeType === 'condition') {
      // Condition stores its operands in input_config so the v1 FlowStep
      // model didn't need new fields. See onChangeStepType in
      // ChatbotFlowBuilderView for the reset semantics.
      const ic = step.input_config || {}
      config.variable = (ic.variable as string) || ''
      config.operator = (ic.operator as string) || 'eq'
      config.value = (ic.value as string) || ''
    }

    return {
      id: step.step_name,
      type: nodeType,
      label: step.label || step.step_name,
      position: { x: pos.x, y: pos.y },
      config,
    }
  })

  const stepNames = new Set(sorted.map(s => s.step_name))
  const edges: ChatEdge[] = []

  sorted.forEach((step, index) => {
    const nextSequential = index < sorted.length - 1 ? sorted[index + 1].step_name : null

    if (step.message_type === 'buttons') {
      const conditional = step.conditional_next || {}
      const mapped = new Set(Object.keys(conditional))
      for (const [buttonId, target] of Object.entries(conditional)) {
        if (target && stepNames.has(target as string)) {
          edges.push({ from: step.step_name, to: target as string, condition: `button:${buttonId}` })
        }
      }
      if (step.buttons && nextSequential && stepNames.has(nextSequential)) {
        for (const btn of step.buttons) {
          if (btn.id && !mapped.has(btn.id)) {
            edges.push({ from: step.step_name, to: nextSequential, condition: `button:${btn.id}` })
          }
        }
      }
    } else if (step.message_type === 'condition') {
      // Two explicit branches via conditional_next.true / .false; no
      // sequential fallthrough — author must wire both handles.
      const conditional = step.conditional_next || {}
      for (const branch of ['true', 'false'] as const) {
        const target = conditional[branch]
        if (target && stepNames.has(target)) {
          edges.push({ from: step.step_name, to: target, condition: branch })
        }
      }
    } else if (step.next_step && stepNames.has(step.next_step)) {
      edges.push({ from: step.step_name, to: step.next_step, condition: 'default' })
    } else if (nextSequential) {
      edges.push({ from: step.step_name, to: nextSequential, condition: 'default' })
    }
  })

  return {
    version: 2,
    nodes,
    edges,
    entry_node: sorted[0]?.step_name || '',
  }
}

/**
 * Convert a v2 graph payload back to the v1 FlowStep[] + CanvasLayout
 * shape that the existing editor binds against. Returns null when any
 * node type can't be expressed in v1 — caller should treat the flow as
 * v2-only and skip the legacy editor.
 *
 * Step order is derived by walking edges from entry_node depth-first;
 * any unreachable nodes are appended in declared order so they remain
 * editable.
 */
export function graphToSteps(graph: ChatFlowGraph): { steps: FlowStep[]; canvas_layout: CanvasLayout } | null {
  if (!graph || !graph.nodes || graph.nodes.length === 0) {
    return null
  }
  for (const node of graph.nodes) {
    if (nodeTypeToMessageType(node.type) === null) {
      return null
    }
  }

  // Walk from entry node, then append unvisited nodes for completeness.
  const nodeById = new Map(graph.nodes.map(n => [n.id, n]))
  const outgoing = new Map<string, ChatEdge[]>()
  for (const edge of graph.edges || []) {
    if (!outgoing.has(edge.from)) outgoing.set(edge.from, [])
    outgoing.get(edge.from)!.push(edge)
  }
  const visited = new Set<string>()
  const order: ChatNode[] = []
  function walk(id: string) {
    if (visited.has(id)) return
    const node = nodeById.get(id)
    if (!node) return
    visited.add(id)
    order.push(node)
    for (const edge of outgoing.get(id) || []) {
      walk(edge.to)
    }
  }
  if (graph.entry_node) walk(graph.entry_node)
  for (const node of graph.nodes) {
    if (!visited.has(node.id)) {
      visited.add(node.id)
      order.push(node)
    }
  }

  const positions: Record<string, { x: number; y: number }> = {}
  const steps: FlowStep[] = order.map((node, index) => {
    positions[node.id] = { x: node.position.x, y: node.position.y }

    const cfg = node.config || {}
    const step: FlowStep = {
      step_name: node.id,
      label: node.label || node.id,
      step_order: index + 1,
      message_type: nodeTypeToMessageType(node.type) as string,
      message: '',
      input_type: 'none',
      input_config: {},
      api_config: {},
      buttons: [],
      transfer_config: {},
      validation_regex: '',
      validation_error: '',
      store_as: '',
      next_step: '',
      conditional_next: {},
      retry_on_invalid: true,
      max_retries: 3,
      skip_condition: '',
    } as unknown as FlowStep

    if (node.type === 'message') {
      step.message = (cfg.message as string) || ''
    } else if (node.type === 'buttons') {
      step.message = (cfg.body as string) || (cfg.message as string) || ''
      step.buttons = (cfg.buttons as any[]) || []
    } else if (node.type === 'end') {
      step.message = (cfg.message as string) || ''
    } else if (node.type === 'condition') {
      step.input_config = {
        variable: (cfg.variable as string) || '',
        operator: (cfg.operator as string) || 'eq',
        value: (cfg.value as string) || '',
      }
    }

    // Translate outgoing edges back into next_step / conditional_next.
    const outs = outgoing.get(node.id) || []
    const conditional: Record<string, string> = {}
    for (const edge of outs) {
      if (edge.condition.startsWith('button:')) {
        const buttonId = edge.condition.slice('button:'.length)
        conditional[buttonId] = edge.to
      } else if (edge.condition === 'true' || edge.condition === 'false') {
        // Condition-node branches surface to the editor as
        // conditional_next entries keyed by the handle id.
        conditional[edge.condition] = edge.to
      } else if (edge.condition === 'default') {
        step.next_step = edge.to
      }
    }
    if (Object.keys(conditional).length > 0) {
      step.conditional_next = conditional
    }
    return step
  })

  return { steps, canvas_layout: { node_positions: positions } }
}

