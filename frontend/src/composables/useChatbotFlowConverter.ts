import type { Node, Edge } from '@vue-flow/core'
import { MarkerType } from '@vue-flow/core'
import type { FlowStep } from '@/types/flow-preview'

function getNodeType(messageType: string): string {
  return 'chatbot_' + messageType
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
        label: step.step_name,
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

    if (step.message_type === 'transfer') {
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

