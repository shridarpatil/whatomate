<script setup lang="ts">
import { computed } from 'vue'
import { Phone, ArrowRight } from '@/lib/icons'

interface Step {
  node?: string
  type?: string
  label?: string
  digit?: string
  option_label?: string
  outcome?: string
  action?: string // flow_start
  flow?: string
}

interface TreeNode {
  step: Step
  children: TreeNode[]
}

const props = defineProps<{
  steps: Step[]
}>()

// Build a tree: flow_start and goto_flow nest subsequent steps as children.
const tree = computed(() => {
  const root: TreeNode[] = []
  const stack: TreeNode[][] = [root]

  for (const step of props.steps) {
    const current = stack[stack.length - 1]

    if (step.action === 'flow_start' || step.type === 'goto_flow') {
      const node: TreeNode = { step, children: [] }
      current.push(node)
      stack.push(node.children)
    } else {
      current.push({ step, children: [] })
    }
  }

  return root
})
</script>

<template>
  <TreeNodes :nodes="tree" :depth="0" />
</template>

<script lang="ts">
import { defineComponent, h, type PropType } from 'vue'

interface TNode {
  step: {
    node?: string; type?: string; label?: string; digit?: string
    option_label?: string; outcome?: string; action?: string; flow?: string
  }
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

      return h('ul', { class: props.depth === 0 ? 'unstyled' : undefined },
        props.nodes.map((node, idx) => {
          const step = node.step
          const isFlow = step.action === 'flow_start' || step.type === 'goto_flow'
          const hasChildren = node.children.length > 0

          const digitBadge = (digit: string) =>
            h('span', { class: 'badge outline' }, digit)

          let rowParts

          if (isFlow) {
            rowParts = []
            if (step.digit) {
              rowParts.push(digitBadge(step.digit))
              rowParts.push(h(ArrowRight, { size: 14 }))
            }
            rowParts.push(h(Phone, { size: 14 }))
            rowParts.push(h('strong', step.flow || step.label || '?'))
          } else if (step.type === 'menu' && step.digit) {
            // Menu with digit: digit badge + option label + node label
            const displayLabel = step.option_label
              ? step.label ? `${step.option_label} (${step.label})` : step.option_label
              : step.label || '-'
            rowParts = [digitBadge(step.digit), h('span', displayLabel)]
          } else if (step.type === 'menu') {
            // Menu without digit (timeout/max_retries)
            rowParts = [
              h('span', { class: 'badge', 'data-variant': 'secondary' }, 'menu'),
              h('span', step.label || '-'),
              step.outcome ? h('small', { class: 'text-light' }, `(${step.outcome})`) : null,
            ]
          } else if (step.type === 'transfer') {
            rowParts = [
              h(ArrowRight, { size: 14 }),
              h('span', step.label || 'Transfer'),
            ]
          } else if (step.type === 'hangup') {
            rowParts = [
              h('span', { class: 'badge', 'data-variant': 'danger' }, 'x'),
              h('span', step.label || 'Hangup'),
            ]
          } else if (step.type) {
            // greeting, timing, http_callback, gather, etc.
            rowParts = [
              h('span', { class: 'badge', 'data-variant': 'secondary' }, step.type.replace('_', ' ')),
              h('span', step.label || '-'),
            ]
          } else {
            // Fallback
            rowParts = [digitBadge(step.digit || '?'), h('span', step.label || '-')]
          }

          const elements = [h('div', { class: 'hstack items-center gap-2' }, rowParts)]

          if (hasChildren) {
            elements.push(h(TreeNodes, { nodes: node.children, depth: props.depth + 1 }))
          }

          return h('li', { key: idx }, elements)
        })
      )
    }
  }
})
</script>
