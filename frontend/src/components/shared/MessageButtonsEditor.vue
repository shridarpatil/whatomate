<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Reply, ExternalLink, Phone, PhoneCall, Trash2 } from 'lucide-vue-next'
import type { ButtonConfig } from '@/types/flow-preview'

type ButtonType = 'reply' | 'url' | 'phone' | 'voice_call'

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

const effectiveMax = computed(() => {
  if (hasVoiceCallButton.value) return 1
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
    id: `btn_${props.buttons.length + 1}`,
    title: '',
    type,
  }
  if (type === 'url') newButton.url = ''
  else if (type === 'phone') newButton.phone_number = ''
  else if (type === 'voice_call') newButton.ttl_minutes = 15
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
  if (hasVoiceCallButton.value) return false
  if (type === 'voice_call') return props.buttons.length === 0
  if (props.buttons.length >= effectiveMax.value) return false
  if (type === 'reply') return !hasCtaButtons.value
  // url / phone
  return !hasReplyButtons.value && !ctaLimitReached.value
}

function typeLabel(type?: string): string {
  if (type === 'url') return 'URL'
  if (type === 'phone') return t('flowBuilder.phoneButton', 'Phone')
  if (type === 'voice_call') return t('flowBuilder.voiceCallButton', 'Call')
  return t('flowBuilder.replyButton', 'Reply')
}

function typeIcon(type?: string) {
  if (type === 'url') return ExternalLink
  if (type === 'phone') return Phone
  if (type === 'voice_call') return PhoneCall
  return Reply
}
</script>

<template>
  <div class="space-y-3">
    <div class="flex items-center justify-between flex-wrap gap-2">
      <Label class="text-xs">
        {{ $t('flowBuilder.buttonOptions', 'Buttons') }} ({{ buttons.length }}/{{ effectiveMax }})
      </Label>
      <div class="flex gap-1">
        <Button
          v-if="allowedTypes.includes('reply')"
          variant="outline"
          size="sm"
          class="h-6 text-xs"
          :disabled="!canAdd('reply')"
          @click="addButton('reply')"
        >
          <Reply class="h-3 w-3 mr-1" />
          {{ $t('flowBuilder.replyButton', 'Reply') }}
        </Button>
        <Button
          v-if="allowedTypes.includes('url')"
          variant="outline"
          size="sm"
          class="h-6 text-xs"
          :disabled="!canAdd('url')"
          @click="addButton('url')"
        >
          <ExternalLink class="h-3 w-3 mr-1" />
          {{ $t('flowBuilder.urlButton', 'URL') }}
        </Button>
        <Button
          v-if="allowedTypes.includes('phone')"
          variant="outline"
          size="sm"
          class="h-6 text-xs"
          :disabled="!canAdd('phone')"
          @click="addButton('phone')"
        >
          <Phone class="h-3 w-3 mr-1" />
          {{ $t('flowBuilder.phoneButton', 'Phone') }}
        </Button>
        <Button
          v-if="allowedTypes.includes('voice_call')"
          variant="outline"
          size="sm"
          class="h-6 text-xs"
          :disabled="!canAdd('voice_call')"
          @click="addButton('voice_call')"
        >
          <PhoneCall class="h-3 w-3 mr-1" />
          {{ $t('flowBuilder.voiceCallButton', 'Call') }}
        </Button>
      </div>
    </div>

    <div class="space-y-2">
      <div
        v-for="(btn, idx) in buttons"
        :key="idx"
        class="p-2 border rounded-md bg-muted/30 space-y-2"
      >
        <div class="flex items-center gap-2">
          <Badge variant="outline" class="text-[10px] px-1.5">
            <component :is="typeIcon(btn.type)" class="h-2.5 w-2.5 mr-1" />
            {{ typeLabel(btn.type) }}
          </Badge>
          <Input
            :model-value="btn.title"
            :placeholder="$t('flowBuilder.buttonTitle', 'Button title')"
            class="h-7 flex-1 text-xs"
            :disabled="disabled"
            @update:model-value="updateButton(idx, { title: String($event) })"
          />
          <Button
            variant="ghost"
            size="icon"
            class="h-7 w-7"
            :disabled="disabled"
            @click="removeButton(idx)"
          >
            <Trash2 class="h-3 w-3 text-destructive" />
          </Button>
        </div>

        <div v-if="btn.type === 'url'">
          <Input
            :model-value="btn.url"
            :placeholder="$t('flowBuilder.exampleUrlPlaceholder', 'https://example.com')"
            class="h-7 text-xs"
            :disabled="disabled"
            @update:model-value="updateButton(idx, { url: String($event) })"
          />
        </div>
        <div v-else-if="btn.type === 'phone'">
          <Input
            :model-value="btn.phone_number"
            :placeholder="$t('flowBuilder.phoneNumberPlaceholder', '+1234567890')"
            class="h-7 text-xs"
            :disabled="disabled"
            @update:model-value="updateButton(idx, { phone_number: String($event) })"
          />
        </div>
        <div v-else-if="btn.type === 'voice_call'" class="flex items-center gap-2">
          <Label class="text-[10px] text-muted-foreground shrink-0">
            {{ $t('flowBuilder.voiceCallTtl', 'Expires after') }}
          </Label>
          <Input
            type="number"
            min="1"
            max="60"
            :model-value="btn.ttl_minutes ?? 15"
            class="h-7 w-16 text-xs"
            :disabled="disabled"
            @update:model-value="updateButton(idx, { ttl_minutes: Number($event) || 0 })"
          />
          <span class="text-[10px] text-muted-foreground">
            {{ $t('flowBuilder.voiceCallTtlSuffix', 'minutes (1–60)') }}
          </span>
        </div>
        <div v-else-if="showIdField">
          <Input
            :model-value="btn.id"
            :placeholder="$t('flowBuilder.buttonIdPlaceholder', 'button-id')"
            class="h-7 text-xs"
            :disabled="disabled"
            @update:model-value="updateButton(idx, { id: String($event) })"
          />
        </div>

        <!-- Per-button extras (e.g. chatbot conditional-routing select) -->
        <slot name="button-extra" :button="btn" :index="idx" />
      </div>
    </div>
  </div>
</template>
