<script setup lang="ts">
import { VueFlow, MarkerType } from '@vue-flow/core'
import type { Node, Edge, NodeMouseEvent, Connection, EdgeMouseEvent } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import { MiniMap } from '@vue-flow/minimap'

withDefaults(
  defineProps<{
    nodes?: Node[]
    edges?: Edge[]
    nodeTypes: Record<string, any>
    edgeType?: string
    fitViewOnInit?: boolean
    controlsPosition?: string
  }>(),
  {
    edgeType: 'smoothstep',
    fitViewOnInit: false,
    controlsPosition: 'bottom-left',
  },
)

defineEmits<{
  'update:nodes': [nodes: Node[]]
  'update:edges': [edges: Edge[]]
  'node-click': [event: NodeMouseEvent]
  'pane-click': []
  'connect': [connection: Connection]
  'edge-click': [event: EdgeMouseEvent]
  'edge-update': [payload: { edge: Edge; connection: Connection }]
  'node-drag-stop': []
  'edges-change': [changes: any[]]
}>()
</script>

<template>
  <div class="flow-canvas h-full relative">
    <VueFlow
      :nodes="nodes"
      :edges="edges"
      :node-types="nodeTypes"
      :nodes-draggable="true"
      :nodes-connectable="true"
      :edges-updatable="true"
      :zoom-on-scroll="true"
      :zoom-on-pinch="true"
      :pan-on-drag="true"
      :pan-on-scroll="false"
      :snap-to-grid="true"
      :snap-grid="[20, 20]"
      :min-zoom="0.2"
      :max-zoom="2"
      :delete-key-code="['Backspace', 'Delete']"
      :default-edge-options="{ type: edgeType, animated: true, markerEnd: MarkerType.ArrowClosed }"
      :fit-view-on-init="fitViewOnInit"
      class="h-full"
      @update:nodes="$emit('update:nodes', $event)"
      @update:edges="$emit('update:edges', $event)"
      @node-click="$emit('node-click', $event)"
      @pane-click="$emit('pane-click')"
      @connect="$emit('connect', $event)"
      @edge-click="$emit('edge-click', $event)"
      @edge-update="$emit('edge-update', $event)"
      @node-drag-stop="$emit('node-drag-stop')"
      @edges-change="$emit('edges-change', $event)"
    >
      <Background
        pattern-color="hsl(var(--muted-foreground) / 0.15)"
        :gap="20"
        :size="1"
        variant="dots"
      />
      <Controls :position="controlsPosition as any" />
      <MiniMap />
      <slot />
    </VueFlow>
  </div>
</template>

<style>
@import '@vue-flow/core/dist/style.css';
@import '@vue-flow/core/dist/theme-default.css';
@import '@vue-flow/controls/dist/style.css';
@import '@vue-flow/minimap/dist/style.css';

.vue-flow__edge-textbg {
  fill: transparent;
}

.selected-node {
  outline: none;
  border-radius: 0.5rem;
  box-shadow:
    0 0 0 2px hsl(var(--background)),
    0 0 0 4px hsl(var(--primary)),
    0 0 16px hsl(var(--primary) / 0.35);
}

.selected-node .base-node {
  border-color: hsl(var(--primary));
}
</style>
