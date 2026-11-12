<script setup lang="ts">
import { computed } from 'vue'
import type { SimulationMessage } from '@/types/flow-preview'
import { Bug, CheckCheck, Info } from '@/lib/icons'

const props = defineProps<{
  message: SimulationMessage
}>()

const formattedTime = computed(() => {
  return props.message.timestamp.toLocaleTimeString('en-US', {
    hour: 'numeric',
    minute: '2-digit',
    hour12: true
  })
})

const isBot = computed(() => props.message.type === 'bot')
const isUser = computed(() => props.message.type === 'user')
const isSystem = computed(() => props.message.type === 'system')
const isDebug = computed(() => props.message.type === 'debug')
</script>

<template>
  <!-- Bot Message -->
  <div v-if="isBot" class="vstack gap-1">
    <div class="flex">
      <div v-if="message.isValidationError" role="alert" data-variant="danger" data-bubble="incoming" class="flex-col gap-1">
        <div class="pre-wrap">{{ message.content }}</div>
        <small class="align-right">{{ formattedTime }}</small>
      </div>
      <article v-else class="card p-4 vstack gap-1" data-bubble="incoming">
        <div class="pre-wrap">{{ message.content }}</div>
        <small class="text-lighter align-right">{{ formattedTime }}</small>
      </article>
    </div>

    <!-- Show step name for debugging -->
    <small v-if="message.stepName" class="text-lighter">Step: {{ message.stepName }}</small>
  </div>

  <!-- User Message -->
  <div v-else-if="isUser" class="flex justify-end">
    <article class="card p-4 vstack gap-1" data-bubble="outgoing">
      <div class="pre-wrap">{{ message.content }}</div>
      <small class="hstack gap-1 justify-end text-lighter">
        {{ formattedTime }}
        <CheckCheck :size="14" />
      </small>
    </article>
  </div>

  <!-- System Message -->
  <div v-else-if="isSystem" role="alert" data-variant="warning" class="items-center">
    <Info :size="12" />
    <small>{{ message.content }}</small>
  </div>

  <!-- Debug Message -->
  <p v-else-if="isDebug" class="hstack gap-1 justify-center text-light">
    <Bug :size="12" class="shrink-0" />
    <small>{{ message.content }}</small>
  </p>
</template>
