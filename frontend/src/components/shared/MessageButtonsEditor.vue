<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Reply, ExternalLink, Phone, PhoneCall, Workflow, Trash2 } from '@/lib/icons'
import type { ButtonConfig } from '@/types/flow-preview'
import { flowsService } from '@/services/api'
import { MAX_BUTTON_TITLE_LENGTH, nextButtonId } from '@/lib/whatsappButtons'

type ButtonType = 'reply' | 'url' | 'phone' | 'voice_call' | 'flow'

const props = withDefaults(
  defineProps<{
    buttons: ButtonConfig[]
    allowedTypes?: ButtonType[]
    /** Cap when no CTA buttons are present. WhatsApp allows up to 10 reply buttons. */
    maxButtons?: number
    disabled?: boolean
    showIdField?: boolean
  }>(),
  {
    allowedTypes: () => ['reply', 'url', 'phone'],
    maxButtons: 10,
    disabled: false,
    showIdField: false,
  },
)

const emit = defineEmits<{
  'update:buttons': [buttons: ButtonConfig[]]
  /** Per-button title edit — host views (e.g. chatbot select input) listen to keep state in sync. */
  'change': [buttons: ButtonConfig[]]
}>()

const { t } = useI18n()

// WhatsApp rules: reply buttons (1-3) and CTA buttons (url/phone, max 2) can't mix.
// voice_call (interactive.type:"voice_call") is exclusive — it can't coexist with
// any other button type and only one is allowed per message.
const hasReplyButtons = computed(() =>
  props.buttons.some((b) => !b.type || b.type === 'reply'),
)
const ctaCount = computed(() =>
  props.buttons.filter((b) => b.type === 'url' || b.type === 'phone').length,
)
const hasCtaButtons = computed(() => ctaCount.value > 0)
const ctaLimitReached = computed(() => ctaCount.value >= 2)
const hasVoiceCallButton = computed(() => props.buttons.some((b) => b.type === 'voice_call'))
// flow, like voice_call, renders as the whole interactive message — exclusive.
const hasFlowButton = computed(() => props.buttons.some((b) => b.type === 'flow'))
const isExclusive = computed(() => hasVoiceCallButton.value || hasFlowButton.value)

// Published flows for the picker. Loaded once when the flow type is allowed.
const publishedFlows = ref<{ meta_flow_id: string; name: string }[]>([])
onMounted(async () => {
  if (!props.allowedTypes.includes('flow')) return
  try {
    const res = await flowsService.list({ limit: 100 })
    publishedFlows.value = (res.data?.flows || [])
      .filter((f: any) => f.status === 'PUBLISHED' && f.meta_flow_id)
      .map((f: any) => ({ meta_flow_id: f.meta_flow_id, name: f.name }))
  } catch {
    publishedFlows.value = []
  }
})

const effectiveMax = computed(() => {
  if (isExclusive.value) return 1
  if (hasCtaButtons.value) return 2
  return props.maxButtons
})

function emitButtons(next: ButtonConfig[]) {
  emit('update:buttons', next)
  emit('change', next)
}

function addButton(type: ButtonType) {
  if (props.buttons.length >= effectiveMax.value) return
  const newButton: ButtonConfig = {
    id: nextButtonId(props.buttons),
    title: '',
    type,
  }
  if (type === 'url') newButton.url = ''
  else if (type === 'phone') newButton.phone_number = ''
  else if (type === 'voice_call') newButton.ttl_minutes = 15
  else if (type === 'flow') {
    newButton.flow_id = ''
    if (!newButton.title) newButton.title = 'Open'
  }
  emitButtons([...props.buttons, newButton])
}

function removeButton(index: number) {
  const next = [...props.buttons]
  next.splice(index, 1)
  emitButtons(next)
}

function updateButton(index: number, patch: Partial<ButtonConfig>) {
  const next = props.buttons.map((b, i) => (i === index ? { ...b, ...patch } : b))
  emitButtons(next)
}

function canAdd(type: ButtonType): boolean {
  if (props.disabled) return false
  if (isExclusive.value) return false
  // Exclusive types can only be the sole button on the message.
  if (type === 'voice_call' || type === 'flow') return props.buttons.length === 0
  if (props.buttons.length >= effectiveMax.value) return false
  if (type === 'reply') return !hasCtaButtons.value
  // url / phone
  return !hasReplyButtons.value && !ctaLimitReached.value
}

function typeLabel(type?: string): string {
  if (type === 'url') return 'URL'
  if (type === 'phone') return t('flowBuilder.phoneButton', 'Phone')
  if (type === 'voice_call') return t('flowBuilder.voiceCallButton', 'Call')
  if (type === 'flow') return t('flowBuilder.flowButton', 'Flow')
  return t('flowBuilder.replyButton', 'Reply')
}

function typeIcon(type?: string) {
  if (type === 'url') return ExternalLink
  if (type === 'phone') return Phone
  if (type === 'voice_call') return PhoneCall
  if (type === 'flow') return Workflow
  return Reply
}
</script>

<template>
  <div class="vstack gap-4">
    <div class="hstack justify-between flex-wrap gap-2">
      <strong>
        {{ $t('flowBuilder.buttonOptions', 'Buttons') }} ({{ buttons.length }}/{{ effectiveMax }})
      </strong>
      <div class="hstack gap-1">
        <button
          v-if="allowedTypes.includes('reply')"
          type="button"
          class="outline small"
          :disabled="!canAdd('reply')"
          @click="addButton('reply')"
        >
          <Reply :size="12" />
          {{ $t('flowBuilder.replyButton', 'Reply') }}
        </button>
        <button
          v-if="allowedTypes.includes('url')"
          type="button"
          class="outline small"
          :disabled="!canAdd('url')"
          @click="addButton('url')"
        >
          <ExternalLink :size="12" />
          {{ $t('flowBuilder.urlButton', 'URL') }}
        </button>
        <button
          v-if="allowedTypes.includes('phone')"
          type="button"
          class="outline small"
          :disabled="!canAdd('phone')"
          @click="addButton('phone')"
        >
          <Phone :size="12" />
          {{ $t('flowBuilder.phoneButton', 'Phone') }}
        </button>
        <button
          v-if="allowedTypes.includes('voice_call')"
          type="button"
          class="outline small"
          :disabled="!canAdd('voice_call')"
          @click="addButton('voice_call')"
        >
          <PhoneCall :size="12" />
          {{ $t('flowBuilder.voiceCallButton', 'Call') }}
        </button>
        <button
          v-if="allowedTypes.includes('flow')"
          type="button"
          class="outline small"
          :disabled="!canAdd('flow')"
          @click="addButton('flow')"
        >
          <Workflow :size="12" />
          {{ $t('flowBuilder.flowButton', 'Flow') }}
        </button>
      </div>
    </div>

    <div class="vstack gap-2">
      <div
        v-for="(btn, idx) in buttons"
        :key="idx"
        class="vstack gap-2"
      >
        <div class="hstack gap-2">
          <span class="badge" data-variant="secondary">
            <component :is="typeIcon(btn.type)" :size="10" />
            {{ typeLabel(btn.type) }}
          </span>
          <input
            :value="btn.title"
            :placeholder="$t('flowBuilder.buttonTitle', 'Button title')"
            :aria-label="$t('flowBuilder.buttonTitle', 'Button title')"
            :maxlength="MAX_BUTTON_TITLE_LENGTH"
            class="flex-1"
            :disabled="disabled"
            @input="updateButton(idx, { title: ($event.target as HTMLInputElement).value })"
          >
          <button
            type="button"
            class="ghost small icon"
            data-variant="danger"
            :title="$t('common.delete')"
            :aria-label="$t('common.delete')"
            :disabled="disabled"
            @click="removeButton(idx)"
          >
            <Trash2 :size="12" />
          </button>
        </div>

        <input
          v-if="btn.type === 'url'"
          :value="btn.url"
          :placeholder="$t('flowBuilder.exampleUrlPlaceholder', 'https://example.com')"
          :aria-label="$t('flowBuilder.urlButton', 'URL')"
          :disabled="disabled"
          @input="updateButton(idx, { url: ($event.target as HTMLInputElement).value })"
        >
        <input
          v-else-if="btn.type === 'phone'"
          :value="btn.phone_number"
          :placeholder="$t('flowBuilder.phoneNumberPlaceholder', '+1234567890')"
          :aria-label="$t('flowBuilder.phoneButton', 'Phone')"
          :disabled="disabled"
          @input="updateButton(idx, { phone_number: ($event.target as HTMLInputElement).value })"
        >
        <div v-else-if="btn.type === 'voice_call'" class="hstack gap-2">
          <label class="hstack gap-2">
            <small class="text-light">{{ $t('flowBuilder.voiceCallTtl', 'Expires after') }}</small>
            <input
              type="number"
              min="1"
              max="60"
              :value="btn.ttl_minutes ?? 15"
              :disabled="disabled"
              @input="updateButton(idx, { ttl_minutes: Number(($event.target as HTMLInputElement).value) || 0 })"
            >
          </label>
          <small class="text-light">
            {{ $t('flowBuilder.voiceCallTtlSuffix', 'minutes (1–60)') }}
          </small>
        </div>
        <div v-else-if="btn.type === 'flow'" class="vstack gap-1">
          <select
            :value="btn.flow_id || ''"
            :aria-label="$t('flowBuilder.selectFlow', 'Select a published flow…')"
            :disabled="disabled"
            @change="updateButton(idx, { flow_id: ($event.target as HTMLSelectElement).value })"
          >
            <option value="" disabled>{{ $t('flowBuilder.selectFlow', 'Select a published flow…') }}</option>
            <option v-for="f in publishedFlows" :key="f.meta_flow_id" :value="f.meta_flow_id">{{ f.name }}</option>
          </select>
          <small v-if="publishedFlows.length === 0" class="text-light">
            {{ $t('flowBuilder.noPublishedFlows', 'No published flows available. Publish a flow first.') }}
          </small>
          <small v-else class="text-light">
            {{ $t('flowBuilder.flowCtaHint', 'The title above is the button label shown to the customer.') }}
          </small>
        </div>
        <input
          v-else-if="showIdField"
          :value="btn.id"
          :placeholder="$t('flowBuilder.buttonIdPlaceholder', 'button-id')"
          :aria-label="$t('flowBuilder.buttonIdPlaceholder', 'button-id')"
          :disabled="disabled"
          @input="updateButton(idx, { id: ($event.target as HTMLInputElement).value })"
        >

        <!-- Per-button extras (e.g. chatbot conditional-routing select) -->
        <slot name="button-extra" :button="btn" :index="idx" />
      </div>
    </div>
  </div>
</template>
