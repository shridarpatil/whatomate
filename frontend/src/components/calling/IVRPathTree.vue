<script setup lang="ts">
import { computed } from 'vue'
import { Phone, ArrowRight } from 'lucide-vue-next'

interface Step {
  digit?: string
  label?: string
  action?: string
  flow?: string
}

interface TreeNode {
  step: Step
  children: TreeNode[]
}

const props = defineProps<{
  steps: Step[]
}>()

function isFlowStep(step: Step) {
  return step.action === 'flow_start' || (step.action === 'goto_flow' && step.flow)
}

// Build a tree from the flat step list.
// flow_start / goto_flow (with flow field) → creates a group, children nested under it.
// submenu → creates a node, children nested under it.
// parent → leaf, pops one level.
// everything else → leaf node.
const tree = computed(() => {
  const root: TreeNode[] = []
  const stack: TreeNode[][] = [root]

  for (const step of props.steps) {
    const current = stack[stack.length - 1]

    if (isFlowStep(step)) {
      // Flow nodes nest all subsequent steps as children
      const node: TreeNode = { step, children: [] }
      current.push(node)
      stack.push(node.children)
    } else if (step.action === 'submenu') {
      const node: TreeNode = { step, children: [] }
      current.push(node)
      stack.push(node.children)
    } else if (step.action === 'parent') {
      current.push({ step, children: [] })
      if (stack.length > 1) stack.pop()
    } else {
      current.push({ step, children: [] })
    }
  }

  return root
})
</script>

<template>
  <div class="ivr-path-tree">
    <TreeNodes :nodes="tree" :depth="0" />
  </div>
</template>

<script lang="ts">
import { defineComponent, h, type PropType } from 'vue'

interface TNode {
  step: { digit?: string; label?: string; action?: string; flow?: string }
  children: TNode[]
}

const TreeNodes = defineComponent({
  name: 'TreeNodes',
  props: {
    nodes: { type: Array as PropType<TNode[]>, required: true },
    depth: { type: Number, default: 0 }
  },
  setup(props) {
    return () => {
      if (!props.nodes.length) return null

      return h('div', { class: 'flex flex-col gap-1' },
        props.nodes.map((node, idx) => {
          const isFlow = node.step.action === 'flow_start' ||
            (node.step.action === 'goto_flow' && node.step.flow)
          const isGotoFlow = node.step.action === 'goto_flow' && node.step.flow
          const hasChildren = node.children.length > 0

          // The row content
          let rowContent
          if (isFlow) {
            const flowParts = []

            // For goto_flow that has a digit, show the digit badge first
            if (isGotoFlow && node.step.digit) {
              flowParts.push(
                h('div', {
                  class: 'flex items-center justify-center h-6 w-6 rounded border bg-muted text-xs font-mono font-bold shrink-0'
                }, node.step.digit)
              )
              flowParts.push(
                h(ArrowRight, { class: 'h-3.5 w-3.5 text-muted-foreground shrink-0' })
              )
            }

            flowParts.push(
              h('div', {
                class: 'flex items-center justify-center h-6 w-6 rounded-full bg-primary shrink-0'
              }, [h(Phone, { class: 'h-3 w-3 text-primary-foreground' })])
            )
            flowParts.push(
              h('span', { class: 'font-semibold text-sm' }, node.step.flow || '?')
            )

            rowContent = h('div', { class: 'flex items-center gap-2 py-1' }, flowParts)
          } else if (node.step.action === 'submenu') {
            rowContent = h('div', { class: 'flex items-center gap-2 py-1' }, [
              h('div', {
                class: 'flex items-center justify-center h-6 w-6 rounded border bg-muted text-xs font-mono font-bold shrink-0'
              }, node.step.digit || '?'),
              h('span', { class: 'text-sm font-medium' }, node.step.label || '-'),
            ])
          } else {
            rowContent = h('div', { class: 'flex items-center gap-2 py-1' }, [
              h('div', {
                class: 'flex items-center justify-center h-6 w-6 rounded border bg-muted text-xs font-mono font-bold shrink-0'
              }, node.step.digit || '?'),
              h('span', { class: 'text-sm' }, node.step.label || '-'),
            ])
          }

          // Build the node with optional children
          const elements = [rowContent]

          if (hasChildren) {
            elements.push(
              h('div', {
                class: 'ml-3 pl-4 border-l-2 border-muted-foreground/30'
              }, [
                h(TreeNodes, { nodes: node.children, depth: props.depth + 1 })
              ])
            )
          }

          return h('div', { key: idx }, elements)
        })
      )
    }
  }
})
</script>
