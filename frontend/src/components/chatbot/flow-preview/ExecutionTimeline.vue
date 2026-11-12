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
} from '@/lib/icons'

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
    let label = ''
    let details = ''

    switch (entry.type) {
      case 'flow_start':
        icon = Play
        label = 'Flow started'
        details = `${entry.details.stepsCount} steps`
        break
      case 'step_enter':
        icon = LogIn
        label = `Enter: ${entry.stepName}`
        details = (entry.details.type as string)
          || `${entry.details.messageType || ''}/${entry.details.inputType || ''}`.replace(/^\/$/, '')
        break
      case 'step_exit':
        icon = LogOut
        label = `Exit: ${entry.stepName}`
        details = (entry.details.outcome as string) || (entry.details.next as string) || ''
        break
      case 'variable_set':
        icon = Tag
        label = `Set: ${entry.details.key}`
        details = String(entry.details.value).substring(0, 30)
        break
      case 'condition_eval':
        icon = GitBranch
        label = `Condition: ${entry.details.type}`
        details = entry.details.result ? 'true' : 'false'
        break
      case 'api_call':
        icon = Globe
        label = 'API call'
        details = `${entry.details.method} ${entry.details.url}`
        break
      case 'validation_pass':
        icon = CheckCircle
        label = 'Validation passed'
        break
      case 'validation_fail':
        icon = XCircle
        label = 'Validation failed'
        details = `Retry ${entry.details.retryCount}/${entry.details.maxRetries}`
        break
      case 'branch':
        icon = GitBranch
        label = 'Branch'
        details = `→ ${entry.details.nextStep || 'end'}`
        break
      case 'flow_complete':
        icon = Flag
        label = 'Flow completed'
        details = entry.details.reason
        break
      case 'flow_error':
        icon = AlertCircle
        label = 'Error'
        details = entry.details.error
        break
    }

    return {
      ...entry,
      time,
      icon,
      label,
      details,
      isCurrent: entry.stepName === props.currentStepName
    }
  }).reverse() // Most recent first
})
</script>

<template>
  <ol class="unstyled vstack gap-2">
    <li
      v-for="entry in formattedEntries"
      :key="entry.id"
      class="hstack gap-2 min-w-0"
      :aria-current="entry.isCurrent ? 'step' : undefined"
    >
      <component :is="entry.icon" :size="14" class="shrink-0" />
      <div class="vstack min-w-0 flex-1">
        <small class="hstack gap-2">
          <strong v-if="entry.isCurrent">{{ entry.label }}</strong>
          <span v-else>{{ entry.label }}</span>
          <span class="text-lighter">{{ entry.time }}</span>
        </small>
        <small v-if="entry.details" class="text-light truncate">{{ entry.details }}</small>
      </div>
    </li>

    <li v-if="formattedEntries.length === 0" class="text-lighter align-center">
      <small>No execution logs yet</small>
    </li>
  </ol>
</template>
