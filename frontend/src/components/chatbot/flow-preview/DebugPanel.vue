<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SimulationState, FlowStep } from '@/types/flow-preview'
import ExecutionTimeline from './ExecutionTimeline.vue'
import {
  Play,
  Pause,
  RotateCcw,
  SkipForward,
  Undo2,
  Braces,
  ListTree,
  CircleDot
} from '@/lib/icons'

const { t } = useI18n()

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

// Oat badge variant for the status pill (undefined = primary).
const statusVariant = computed(() => {
  switch (props.state.status) {
    case 'running':
    case 'completed':
      return 'success'
    case 'paused':
      return 'warning'
    case 'waiting_input':
      return undefined
    case 'error':
      return 'danger'
    default:
      return 'secondary'
  }
})

const isRunning = computed(() => props.state.status === 'running' || props.state.status === 'waiting_input')

const playPauseLabel = computed(() => {
  if (props.state.status === 'completed') return t('flowBuilder.previewRestart')
  return isRunning.value ? t('flowBuilder.previewPause') : t('flowBuilder.previewStart')
})

const variableEntries = computed(() => {
  return Object.entries(props.state.variables)
})

function handlePlayPause() {
  // 'completed' restarts the flow (startSimulation wipes state).
  if (props.state.status === 'idle' || props.state.status === 'completed') {
    emit('start')
  } else if (props.state.status === 'paused') {
    emit('resume')
  } else if (props.state.status === 'running' || props.state.status === 'waiting_input') {
    emit('pause')
  }
}
</script>

<template>
  <article class="card p-4 flex flex-col gap-4 min-h-0">
    <!-- Header -->
    <header class="vstack gap-2 shrink-0">
      <div class="hstack justify-between gap-2">
        <span class="badge" :data-variant="statusVariant" role="status">{{ statusLabel }}</span>

        <!-- Control Buttons -->
        <div class="hstack gap-1">
          <button
            type="button"
            class="ghost small icon"
            :disabled="state.status === 'error'"
            :title="playPauseLabel"
            :aria-label="playPauseLabel"
            @click="handlePlayPause"
          >
            <Pause v-if="isRunning" />
            <Play v-else />
          </button>

          <button
            type="button"
            class="ghost small icon"
            :disabled="state.status === 'idle' || state.status === 'completed' || state.status === 'error' || state.status === 'waiting_input'"
            :title="$t('flowBuilder.previewStepForward')"
            :aria-label="$t('flowBuilder.previewStepForward')"
            @click="emit('stepForward')"
          >
            <SkipForward />
          </button>

          <button
            type="button"
            class="ghost small icon"
            :disabled="!canUndo"
            :title="$t('flowBuilder.previewStepBack')"
            :aria-label="$t('flowBuilder.previewStepBack')"
            @click="emit('undo')"
          >
            <Undo2 />
          </button>

          <button
            type="button"
            class="ghost small icon"
            :title="$t('flowBuilder.previewRestart')"
            :aria-label="$t('flowBuilder.previewRestart')"
            @click="emit('reset')"
          >
            <RotateCcw />
          </button>
        </div>
      </div>

      <!-- Current Step -->
      <small v-if="state.currentStepName" class="text-light">
        Step {{ (state.currentStepIndex ?? 0) + 1 }}: <code>{{ state.currentStepName }}</code>
      </small>
    </header>

    <div class="flex-1 scroll-y">
      <!-- Variables Section -->
      <details open>
        <summary>
          <span class="hstack gap-2"><Braces /> Variables</span>
          <span class="badge" data-variant="secondary">{{ variableEntries.length }}</span>
        </summary>
        <div class="vstack gap-1">
          <small v-if="variableEntries.length === 0" class="text-light align-center">No variables set</small>
          <p v-for="[key, value] in variableEntries" :key="key" class="hstack gap-2 min-w-0">
            <code class="shrink-0">{{ key }}:</code>
            <small class="min-w-0">{{ typeof value === 'object' ? JSON.stringify(value) : value }}</small>
          </p>
        </div>
      </details>

      <!-- Steps Section -->
      <details>
        <summary>
          <span class="hstack gap-2"><ListTree /> Steps</span>
          <span class="badge" data-variant="secondary">{{ steps.length }}</span>
        </summary>
        <ul class="unstyled vstack gap-1">
          <li v-for="(step, idx) in steps" :key="step.step_name">
            <button
              type="button"
              class="small w-100 justify-between"
              :class="state.currentStepName === step.step_name ? 'outline' : 'ghost'"
              :aria-current="state.currentStepName === step.step_name ? 'step' : undefined"
              @click="emit('goToStep', step.step_name)"
            >
              <span class="hstack gap-2 min-w-0">
                <CircleDot :size="12" />
                <span>{{ idx + 1 }}.</span>
                <code class="truncate">{{ step.step_name }}</code>
              </span>
            </button>
          </li>
        </ul>
      </details>

      <!-- Timeline Section -->
      <details open>
        <summary>
          Timeline
          <span class="badge" data-variant="secondary">{{ state.executionLog.length }}</span>
        </summary>
        <div>
          <ExecutionTimeline
            :entries="state.executionLog"
            :current-step-name="state.currentStepName"
          />
        </div>
      </details>
    </div>
  </article>
</template>
