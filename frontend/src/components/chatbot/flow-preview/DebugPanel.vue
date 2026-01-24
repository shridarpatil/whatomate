<script setup lang="ts">
import { computed } from 'vue'
import type { SimulationState, FlowStep } from '@/types/flow-preview'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import ExecutionTimeline from './ExecutionTimeline.vue'
import {
  Play,
  Pause,
  RotateCcw,
  SkipForward,
  Undo2,
  ChevronDown,
  ChevronRight,
  Braces,
  ListTree,
  CircleDot
} from 'lucide-vue-next'
import { ref } from 'vue'

const props = defineProps<{
  state: SimulationState
  steps: FlowStep[]
  canUndo: boolean
}>()

const emit = defineEmits<{
  start: []
  pause: []
  resume: []
  reset: []
  stepForward: []
  undo: []
  goToStep: [stepName: string]
}>()

const variablesExpanded = ref(true)
const timelineExpanded = ref(true)
const stepsExpanded = ref(false)

const statusLabel = computed(() => {
  switch (props.state.status) {
    case 'idle':
      return 'Ready'
    case 'running':
      return 'Running'
    case 'paused':
      return 'Paused'
    case 'waiting_input':
      return 'Waiting for input'
    case 'completed':
      return 'Completed'
    case 'error':
      return 'Error'
    default:
      return props.state.status
  }
})

const statusColor = computed(() => {
  switch (props.state.status) {
    case 'idle':
      return 'bg-gray-500'
    case 'running':
      return 'bg-green-500'
    case 'paused':
      return 'bg-yellow-500'
    case 'waiting_input':
      return 'bg-blue-500'
    case 'completed':
      return 'bg-green-600'
    case 'error':
      return 'bg-red-500'
    default:
      return 'bg-gray-500'
  }
})

const variableEntries = computed(() => {
  return Object.entries(props.state.variables)
})

function handlePlayPause() {
  if (props.state.status === 'idle') {
    emit('start')
  } else if (props.state.status === 'paused') {
    emit('resume')
  } else if (props.state.status === 'running' || props.state.status === 'waiting_input') {
    emit('pause')
  }
}
</script>

<template>
  <div class="h-full flex flex-col bg-gray-50 dark:bg-[#111b21] border-l border-gray-200 dark:border-gray-700">
    <!-- Header -->
    <div class="px-3 py-2 border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-[#202c33]">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <div
            class="w-2 h-2 rounded-full animate-pulse"
            :class="statusColor"
          />
          <span class="text-xs font-medium text-gray-600 dark:text-gray-300">
            {{ statusLabel }}
          </span>
        </div>

        <!-- Control Buttons -->
        <div class="flex items-center gap-1">
          <Button
            variant="ghost"
            size="icon"
            class="h-7 w-7"
            :disabled="state.status === 'completed' || state.status === 'error'"
            @click="handlePlayPause"
          >
            <Pause v-if="state.status === 'running' || state.status === 'waiting_input'" class="h-4 w-4" />
            <Play v-else class="h-4 w-4" />
          </Button>

          <Button
            variant="ghost"
            size="icon"
            class="h-7 w-7"
            :disabled="state.status !== 'paused'"
            @click="emit('stepForward')"
          >
            <SkipForward class="h-4 w-4" />
          </Button>

          <Button
            variant="ghost"
            size="icon"
            class="h-7 w-7"
            :disabled="!canUndo"
            @click="emit('undo')"
          >
            <Undo2 class="h-4 w-4" />
          </Button>

          <Button
            variant="ghost"
            size="icon"
            class="h-7 w-7"
            @click="emit('reset')"
          >
            <RotateCcw class="h-4 w-4" />
          </Button>
        </div>
      </div>

      <!-- Current Step -->
      <div v-if="state.currentStepName" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
        Step {{ (state.currentStepIndex ?? 0) + 1 }}: <span class="font-mono">{{ state.currentStepName }}</span>
      </div>
    </div>

    <ScrollArea class="flex-1">
      <div class="p-2 space-y-2">
        <!-- Variables Section -->
        <Collapsible v-model:open="variablesExpanded">
          <CollapsibleTrigger class="flex items-center gap-2 w-full px-2 py-1.5 hover:bg-gray-100 dark:hover:bg-gray-800 rounded text-sm font-medium text-gray-700 dark:text-gray-300">
            <ChevronDown v-if="variablesExpanded" class="h-4 w-4" />
            <ChevronRight v-else class="h-4 w-4" />
            <Braces class="h-4 w-4" />
            Variables
            <span class="ml-auto text-xs text-gray-400">{{ variableEntries.length }}</span>
          </CollapsibleTrigger>
          <CollapsibleContent>
            <div class="mt-1 px-2 py-1 bg-white dark:bg-[#202c33] rounded border border-gray-200 dark:border-gray-700">
              <div v-if="variableEntries.length === 0" class="text-xs text-gray-400 py-2 text-center">
                No variables set
              </div>
              <div v-else class="space-y-1">
                <div
                  v-for="[key, value] in variableEntries"
                  :key="key"
                  class="flex items-start gap-2 text-xs py-1"
                >
                  <span class="font-mono text-purple-600 dark:text-purple-400 flex-shrink-0">{{ key }}:</span>
                  <span class="text-gray-700 dark:text-gray-300 break-all">
                    {{ typeof value === 'object' ? JSON.stringify(value) : value }}
                  </span>
                </div>
              </div>
            </div>
          </CollapsibleContent>
        </Collapsible>

        <!-- Steps Section -->
        <Collapsible v-model:open="stepsExpanded">
          <CollapsibleTrigger class="flex items-center gap-2 w-full px-2 py-1.5 hover:bg-gray-100 dark:hover:bg-gray-800 rounded text-sm font-medium text-gray-700 dark:text-gray-300">
            <ChevronDown v-if="stepsExpanded" class="h-4 w-4" />
            <ChevronRight v-else class="h-4 w-4" />
            <ListTree class="h-4 w-4" />
            Steps
            <span class="ml-auto text-xs text-gray-400">{{ steps.length }}</span>
          </CollapsibleTrigger>
          <CollapsibleContent>
            <div class="mt-1 px-2 py-1 bg-white dark:bg-[#202c33] rounded border border-gray-200 dark:border-gray-700 max-h-40 overflow-y-auto">
              <div
                v-for="(step, idx) in steps"
                :key="step.step_name"
                class="flex items-center gap-2 text-xs py-1.5 px-1 rounded cursor-pointer transition-colors"
                :class="{
                  'bg-blue-50 dark:bg-blue-900/30': state.currentStepName === step.step_name,
                  'hover:bg-gray-50 dark:hover:bg-gray-800': state.currentStepName !== step.step_name
                }"
                @click="emit('goToStep', step.step_name)"
              >
                <CircleDot
                  class="h-3 w-3"
                  :class="{
                    'text-green-500': state.currentStepName === step.step_name,
                    'text-gray-300 dark:text-gray-600': state.currentStepName !== step.step_name
                  }"
                />
                <span class="text-gray-500">{{ idx + 1 }}.</span>
                <span class="font-mono text-gray-700 dark:text-gray-300">{{ step.step_name }}</span>
              </div>
            </div>
          </CollapsibleContent>
        </Collapsible>

        <!-- Timeline Section -->
        <Collapsible v-model:open="timelineExpanded">
          <CollapsibleTrigger class="flex items-center gap-2 w-full px-2 py-1.5 hover:bg-gray-100 dark:hover:bg-gray-800 rounded text-sm font-medium text-gray-700 dark:text-gray-300">
            <ChevronDown v-if="timelineExpanded" class="h-4 w-4" />
            <ChevronRight v-else class="h-4 w-4" />
            Timeline
            <span class="ml-auto text-xs text-gray-400">{{ state.executionLog.length }}</span>
          </CollapsibleTrigger>
          <CollapsibleContent>
            <div class="mt-1 bg-white dark:bg-[#202c33] rounded border border-gray-200 dark:border-gray-700 max-h-60 overflow-y-auto">
              <ExecutionTimeline
                :entries="state.executionLog"
                :current-step-name="state.currentStepName"
              />
            </div>
          </CollapsibleContent>
        </Collapsible>
      </div>
    </ScrollArea>
  </div>
</template>
