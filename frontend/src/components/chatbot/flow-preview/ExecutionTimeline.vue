<script setup lang="ts">
import { computed } from 'vue'
import type { ExecutionLogEntry } from '@/types/flow-preview'
import {
  Play,
  LogIn,
  LogOut,
  Tag,
  GitBranch,
  Globe,
  CheckCircle,
  XCircle,
  Flag,
  AlertCircle
} from 'lucide-vue-next'

const props = defineProps<{
  entries: ExecutionLogEntry[]
  currentStepName?: string | null
}>()

const formattedEntries = computed(() => {
  return props.entries.map(entry => {
    const time = entry.timestamp.toLocaleTimeString('en-US', {
      hour: 'numeric',
      minute: '2-digit',
      second: '2-digit',
      hour12: false
    })

    let icon = Play
    let color = 'text-gray-500'
    let label = ''
    let details = ''

    switch (entry.type) {
      case 'flow_start':
        icon = Play
        color = 'text-green-500'
        label = 'Flow started'
        details = `${entry.details.stepsCount} steps`
        break
      case 'step_enter':
        icon = LogIn
        color = 'text-blue-500'
        label = `Enter: ${entry.stepName}`
        details = (entry.details.type as string)
          || `${entry.details.messageType || ''}/${entry.details.inputType || ''}`.replace(/^\/$/, '')
        break
      case 'step_exit':
        icon = LogOut
        color = 'text-gray-400'
        label = `Exit: ${entry.stepName}`
        details = (entry.details.outcome as string) || (entry.details.next as string) || ''
        break
      case 'variable_set':
        icon = Tag
        color = 'text-purple-500'
        label = `Set: ${entry.details.key}`
        details = String(entry.details.value).substring(0, 30)
        break
      case 'condition_eval':
        icon = GitBranch
        color = entry.details.result ? 'text-green-500' : 'text-orange-500'
        label = `Condition: ${entry.details.type}`
        details = entry.details.result ? 'true' : 'false'
        break
      case 'api_call':
        icon = Globe
        color = 'text-cyan-500'
        label = 'API call'
        details = `${entry.details.method} ${entry.details.url}`
        break
      case 'validation_pass':
        icon = CheckCircle
        color = 'text-green-500'
        label = 'Validation passed'
        break
      case 'validation_fail':
        icon = XCircle
        color = 'text-red-500'
        label = 'Validation failed'
        details = `Retry ${entry.details.retryCount}/${entry.details.maxRetries}`
        break
      case 'branch':
        icon = GitBranch
        color = 'text-amber-500'
        label = 'Branch'
        details = `→ ${entry.details.nextStep || 'end'}`
        break
      case 'flow_complete':
        icon = Flag
        color = 'text-green-600'
        label = 'Flow completed'
        details = entry.details.reason
        break
      case 'flow_error':
        icon = AlertCircle
        color = 'text-red-600'
        label = 'Error'
        details = entry.details.error
        break
    }

    return {
      ...entry,
      time,
      icon,
      color,
      label,
      details,
      isCurrent: entry.stepName === props.currentStepName
    }
  }).reverse() // Most recent first
})
</script>

<template>
  <div class="space-y-1 text-xs">
    <div
      v-for="entry in formattedEntries"
      :key="entry.id"
      class="flex items-start gap-2 py-1 px-2 rounded transition-colors"
      :class="{ 'bg-blue-50 dark:bg-blue-900/20': entry.isCurrent }"
    >
      <component
        :is="entry.icon"
        class="h-3.5 w-3.5 flex-shrink-0 mt-0.5"
        :class="entry.color"
      />
      <div class="flex-1 min-w-0">
        <div class="flex items-center gap-2">
          <span class="font-medium text-gray-700 dark:text-gray-300">{{ entry.label }}</span>
          <span class="text-gray-400">{{ entry.time }}</span>
        </div>
        <p v-if="entry.details" class="text-gray-500 dark:text-gray-400 truncate">
          {{ entry.details }}
        </p>
      </div>
    </div>

    <div v-if="formattedEntries.length === 0" class="text-center text-gray-400 py-4">
      No execution logs yet
    </div>
  </div>
</template>
