<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { FlowStep, FlowData } from '@/types/flow-preview'
import type { ChatFlowGraph } from '@/services/api'
import Modal from '@/components/oat/Modal.vue'
import InteractivePreview from './InteractivePreview.vue'
import {
  MessageSquare,
  Users,
  Globe,
  Edit3,
  ExternalLink,
  Play,
  GitBranch,
  Clock,
  StopCircle,
  Variable,
  List,
  CheckCheck,
} from '@/lib/icons'

const props = defineProps<{
  steps: FlowStep[]
  flowData: Partial<FlowData>
  selectedStep: FlowStep | null
  selectedStepIndex: number | null
  listPickerOpen: boolean
  teams: Array<{ id: string; name: string }>
  initialMode?: 'edit' | 'preview'
  // v2 graph used by the interactive simulator. Edit-mode static preview
  // continues to read selectedStep for the per-step WhatsApp render.
  graph?: ChatFlowGraph | null
}>()

const emit = defineEmits<{
  'update:listPickerOpen': [value: boolean]
}>()

const mode = ref<'edit' | 'preview'>(props.initialMode || 'edit')

watch(() => props.initialMode, (newMode) => {
  if (newMode) {
    mode.value = newMode
  }
})

// The active mode tab is outlined, the other is ghost.
const tabClass = computed(() => ({
  edit: mode.value === 'edit' ? 'outline' : 'ghost',
  preview: mode.value === 'preview' ? 'outline' : 'ghost',
}))

// Step types that don't render as a WhatsApp message bubble — control-
// flow nodes that operate on session state without sending anything.
const controlNodeMeta: Record<string, { label: string; icon: any; description: string }> = {
  condition: {
    label: 'Condition',
    icon: GitBranch,
    description: 'Branches the flow based on a boolean expression. No WhatsApp message is sent.',
  },
  timing: {
    label: 'Timing',
    icon: Clock,
    description: 'Routes between in-hours and out-of-hours based on the schedule. No WhatsApp message is sent.',
  },
  goto_flow: {
    label: 'Go to Flow',
    icon: ExternalLink,
    description: 'Jumps execution to another flow. Session variables carry forward.',
  },
  set_variable: {
    label: 'Set Variable',
    icon: Variable,
    description: 'Assigns values to session variables. No WhatsApp message is sent.',
  },
  end: {
    label: 'End',
    icon: StopCircle,
    description: 'Terminates the flow. The optional message below renders in WhatsApp on completion.',
  },
}

const controlNodeInfo = computed(() => {
  if (!props.selectedStep) return null
  return controlNodeMeta[props.selectedStep.message_type] || null
})

// Sync listPickerOpen with parent
const localListPickerOpen = computed({
  get: () => props.listPickerOpen,
  set: (val) => emit('update:listPickerOpen', val)
})
</script>

<template>
  <div class="flex flex-col flex-1 h-full min-h-0 gap-4">
    <!-- Mode Toggle Header -->
    <div class="hstack justify-between gap-2 shrink-0">
      <div role="tablist" class="hstack gap-1">
        <button
          type="button"
          role="tab"
          class="small"
          :class="tabClass.edit"
          :aria-selected="mode === 'edit' ? 'true' : 'false'"
          @click="mode = 'edit'"
        >
          <Edit3 :size="14" />
          Edit
        </button>
        <button
          type="button"
          role="tab"
          class="small"
          :class="tabClass.preview"
          :aria-selected="mode === 'preview' ? 'true' : 'false'"
          @click="mode = 'preview'"
        >
          <Play :size="14" />
          Preview
        </button>
      </div>

      <small class="text-light">
        {{ mode === 'edit' ? 'Read-only preview of selected step' : 'Interactive flow simulation' }}
      </small>
    </div>

    <!-- Content Area -->
    <div role="tabpanel" class="flex flex-1 min-h-0">
      <!-- Edit Mode: Read-only Static Preview -->
      <div v-if="mode === 'edit'" class="flex flex-1 justify-center items-center min-h-0">
        <!-- Control-flow nodes don't render as WhatsApp bubbles — show
             a typed placeholder card instead so authors aren't confused. -->
        <article v-if="selectedStep && controlNodeInfo" class="card side-panel vstack items-center gap-2 align-center">
          <component :is="controlNodeInfo.icon" :size="24" />
          <strong>{{ controlNodeInfo.label }}</strong>
          <small class="text-light">{{ controlNodeInfo.description }}</small>
          <p v-if="selectedStep.message" class="align-left"><small>{{ selectedStep.message }}</small></p>
          <small v-else class="text-lighter"><em>Use the right panel to configure this step.</em></small>
        </article>

        <!-- WhatsApp Static Preview (for message / buttons / api_fetch / whatsapp_flow / transfer) -->
        <article v-else class="card p-4 side-panel h-full flex flex-col gap-4 min-h-0">
          <!-- Chat Header -->
          <header class="hstack gap-2 shrink-0 min-w-0">
            <MessageSquare :size="20" class="shrink-0" />
            <div class="vstack min-w-0">
              <strong class="truncate">{{ flowData.name || 'WhatsApp Preview' }}</strong>
              <small v-if="selectedStep" class="truncate text-light">Step {{ (selectedStepIndex ?? 0) + 1 }}: {{ selectedStep.step_name }}</small>
              <small v-else class="text-light">Select a step</small>
            </div>
          </header>

          <!-- Chat Messages -->
          <div v-if="selectedStep" class="flex-1 scroll-y vstack gap-4">
            <!-- Bot Message Bubble -->
            <div class="vstack gap-1" data-bubble="incoming">
              <article class="card p-4 vstack gap-1">
                <div v-if="selectedStep.message" class="pre-wrap">{{ selectedStep.message }}</div>
                <small v-else class="text-lighter"><em>No message configured</em></small>
                <small class="text-lighter align-right">12:00 PM</small>
              </article>

              <!-- Interactive Buttons (up to 3) -->
              <div v-if="selectedStep.message_type === 'buttons' && selectedStep.buttons.length > 0 && selectedStep.buttons.length <= 3" class="vstack gap-1">
                <button v-for="(btn, idx) in selectedStep.buttons" :key="idx" type="button" class="outline w-100">
                  <ExternalLink v-if="btn.type === 'url'" />
                  {{ btn.title || `Option ${idx + 1}` }}
                </button>
              </div>

              <!-- List Button (more than 3 options) -->
              <button
                v-if="selectedStep.message_type === 'buttons' && selectedStep.buttons.length > 3"
                type="button"
                class="outline w-100"
                @click="localListPickerOpen = !localListPickerOpen"
              >
                <List />
                Select an option
              </button>

              <!-- WhatsApp Flow Button -->
              <button v-if="selectedStep.message_type === 'whatsapp_flow'" type="button" class="outline w-100">
                {{ selectedStep.input_config?.flow_cta || 'Open Form' }}
              </button>
            </div>

            <!-- User Response Placeholder -->
            <div v-if="selectedStep.message_type !== 'transfer'" class="flex justify-end">
              <article class="card p-4 vstack gap-1" data-bubble="outgoing">
                <small><em>
                  <template v-if="selectedStep.input_type === 'none'">(No response needed)</template>
                  <template v-else-if="selectedStep.message_type === 'buttons'">User taps a button...</template>
                  <template v-else-if="selectedStep.message_type === 'whatsapp_flow'">User completes form...</template>
                  <template v-else>User types {{ selectedStep.input_type }}...</template>
                </em></small>
                <small class="hstack gap-1 justify-end text-lighter">12:01 PM <CheckCheck :size="14" /></small>
              </article>
            </div>

            <!-- Store As Info -->
            <p v-if="selectedStep.store_as" class="align-center text-light">
              <small>Response saved as <code>{{ selectedStep.store_as }}</code></small>
            </p>

            <!-- Transfer System Message -->
            <div v-if="selectedStep.message_type === 'transfer'" role="alert" data-variant="warning" class="items-center">
              <Users :size="12" />
              <small>Conversation transferred to {{ selectedStep?.transfer_config?.team_id === '_general' ? 'General Queue' : teams.find(t => t.id === selectedStep?.transfer_config?.team_id)?.name || 'Team' }}</small>
            </div>

            <!-- API Info -->
            <div v-if="selectedStep.message_type === 'api_fetch'" role="alert" class="items-center">
              <Globe :size="12" />
              <small>Message populated from API</small>
            </div>
          </div>
          <div v-else class="flex flex-1 items-center justify-center text-light">
            <small>Select a step to view preview</small>
          </div>

          <!-- Input Bar -->
          <input class="shrink-0" disabled placeholder="Type a message" aria-label="Type a message">
        </article>

        <!-- List Picker -->
        <Modal
          v-if="selectedStep && selectedStep.message_type === 'buttons' && selectedStep.buttons.length > 3"
          v-model:open="localListPickerOpen"
          size="small"
          title="Select an option"
        >
          <ul class="unstyled vstack gap-2">
            <li v-for="(btn, idx) in selectedStep.buttons" :key="idx">
              <button type="button" class="outline w-100 justify-between" @click="localListPickerOpen = false">
                <span class="hstack gap-2 min-w-0">
                  <ExternalLink v-if="btn.type === 'url'" />
                  <span v-else class="badge outline">{{ idx + 1 }}</span>
                  <span class="truncate">{{ btn.title || `Option ${idx + 1}` }}</span>
                </span>
              </button>
            </li>
          </ul>
        </Modal>
      </div>

      <!-- Preview Mode: Interactive Simulation (graph-based) -->
      <InteractivePreview
        v-else
        :graph="graph || null"
        :flow-data="flowData"
      />
    </div>
  </div>
</template>
