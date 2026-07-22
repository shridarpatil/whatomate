<script setup lang="ts">
import { computed } from 'vue'
import type { ChatNode } from '@/services/api'
import { useTeamsStore } from '@/stores/teams'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { Trash2, Plus } from 'lucide-vue-next'

const props = defineProps<{
  node: ChatNode
  currentFlowId?: string
  availableFlows?: { id: string; name: string }[]
}>()

const emit = defineEmits<{
  'update:node': [node: ChatNode]
  'delete': []
}>()

const teamsStore = useTeamsStore()
if (teamsStore.teams.length === 0) teamsStore.fetchTeams()

const config = computed(() => props.node.config || {})

function updateConfig(key: string, value: any) {
  emit('update:node', {
    ...props.node,
    config: { ...props.node.config, [key]: value },
  })
}

function updateLabel(label: string) {
  emit('update:node', { ...props.node, label })
}

// "Text" in the palette covers both v2 `message` (fire-and-forget) and
// v2 `prompt` (blocking + validating). The author chooses by picking an
// expected response type — anything other than "none" flips the node to
// `prompt` under the hood and exposes validation + store_as fields.
const isTextNode = computed(() => props.node.type === 'message' || props.node.type === 'prompt')

const expectedResponse = computed<string>(() => {
  if (props.node.type === 'message') return 'none'
  return (props.node.config?.input_type as string) || 'text'
})

function setExpectedResponse(value: string) {
  if (value === 'none') {
    // Drop prompt-only fields, switch type back to message.
    const { input_type: _unused, validation_regex: _r, validation_error: _e, store_as: _s, max_retries: _m, body, ...rest } = props.node.config || {}
    void _unused; void _r; void _e; void _s; void _m
    emit('update:node', {
      ...props.node,
      type: 'message',
      config: {
        ...rest,
        // The text in the message body lived under either `body` (prompt
        // shape) or `message` (message shape) depending on history —
        // collapse to `message` for fire-and-forget.
        message: body || props.node.config?.message || '',
      },
    })
    return
  }
  // Switch to prompt and remember the response variant.
  const { message, ...rest } = props.node.config || {}
  emit('update:node', {
    ...props.node,
    type: 'prompt',
    config: {
      ...rest,
      body: rest.body || message || '',
      input_type: value,
    },
  })
}

const textBodyValue = computed(() => {
  if (props.node.type === 'prompt') return (props.node.config?.body as string) || ''
  return (props.node.config?.message as string) || ''
})

function updateTextBody(value: string) {
  const key = props.node.type === 'prompt' ? 'body' : 'message'
  updateConfig(key, value)
}

// Buttons helpers
function addReplyButton() {
  const buttons = [...(config.value.buttons || [])]
  const id = `btn_${Date.now()}_${buttons.length}`
  buttons.push({ id, title: '', type: 'reply' })
  updateConfig('buttons', buttons)
}

function addUrlButton() {
  const buttons = [...(config.value.buttons || [])]
  const id = `btn_${Date.now()}_${buttons.length}`
  buttons.push({ id, title: '', type: 'url', url: '' })
  updateConfig('buttons', buttons)
}

function addPhoneButton() {
  const buttons = [...(config.value.buttons || [])]
  const id = `btn_${Date.now()}_${buttons.length}`
  buttons.push({ id, title: '', type: 'phone', phone_number: '' })
  updateConfig('buttons', buttons)
}

function updateButton(idx: number, field: string, value: any) {
  const buttons = [...(config.value.buttons || [])]
  buttons[idx] = { ...buttons[idx], [field]: value }
  updateConfig('buttons', buttons)
}

function removeButton(idx: number) {
  const buttons = [...(config.value.buttons || [])]
  buttons.splice(idx, 1)
  updateConfig('buttons', buttons)
}

const hasReplyButtons = computed(() =>
  (config.value.buttons || []).some((b: any) => !b.type || b.type === 'reply'),
)
const hasCtaButtons = computed(() =>
  (config.value.buttons || []).some((b: any) => b.type === 'url' || b.type === 'phone'),
)
const replyCount = computed(() =>
  (config.value.buttons || []).filter((b: any) => !b.type || b.type === 'reply').length,
)
const ctaCount = computed(() =>
  (config.value.buttons || []).filter((b: any) => b.type === 'url' || b.type === 'phone').length,
)

// HTTP headers helpers (api_call / webhook)
function addHeader() {
  const headers = { ...(config.value.headers || {}) }
  headers[''] = ''
  updateConfig('headers', headers)
}

function removeHeader(key: string) {
  const headers = { ...(config.value.headers || {}) }
  delete headers[key]
  updateConfig('headers', headers)
}

function updateHeaderKey(oldKey: string, newKey: string) {
  if (oldKey === newKey) return
  const headers = { ...(config.value.headers || {}) }
  headers[newKey] = headers[oldKey]
  delete headers[oldKey]
  updateConfig('headers', headers)
}

function updateHeaderValue(key: string, value: string) {
  const headers = { ...(config.value.headers || {}) }
  headers[key] = value
  updateConfig('headers', headers)
}

// Response mapping helpers (api_call)
function addResponseMapping() {
  const m = { ...(config.value.response_mapping || {}) }
  m[''] = ''
  updateConfig('response_mapping', m)
}

function removeResponseMapping(key: string) {
  const m = { ...(config.value.response_mapping || {}) }
  delete m[key]
  updateConfig('response_mapping', m)
}

function updateResponseMappingKey(oldKey: string, newKey: string) {
  if (oldKey === newKey) return
  const m = { ...(config.value.response_mapping || {}) }
  m[newKey] = m[oldKey]
  delete m[oldKey]
  updateConfig('response_mapping', m)
}

function updateResponseMappingValue(key: string, value: string) {
  const m = { ...(config.value.response_mapping || {}) }
  m[key] = value
  updateConfig('response_mapping', m)
}

// Timing schedule
const defaultSchedule = [
  { day: 'monday', enabled: true, start_time: '09:00', end_time: '17:00' },
  { day: 'tuesday', enabled: true, start_time: '09:00', end_time: '17:00' },
  { day: 'wednesday', enabled: true, start_time: '09:00', end_time: '17:00' },
  { day: 'thursday', enabled: true, start_time: '09:00', end_time: '17:00' },
  { day: 'friday', enabled: true, start_time: '09:00', end_time: '17:00' },
  { day: 'saturday', enabled: false, start_time: '09:00', end_time: '17:00' },
  { day: 'sunday', enabled: false, start_time: '09:00', end_time: '17:00' },
]
const schedule = computed(() => config.value.schedule || defaultSchedule)

function updateScheduleEntry(idx: number, field: string, value: any) {
  const sched = [...schedule.value]
  sched[idx] = { ...sched[idx], [field]: value }
  updateConfig('schedule', sched)
}

const gotoFlowTargets = computed(() =>
  (props.availableFlows || []).filter((f) => f.id !== props.currentFlowId),
)

const typeLabel: Record<string, string> = {
  start: 'Start',
  message: 'Message',
  prompt: 'Prompt',
  buttons: 'Buttons',
  api_call: 'API Call',
  condition: 'Condition',
  timing: 'Timing',
  transfer: 'Transfer',
  end: 'End',
  goto_flow: 'Go to Flow',
  whatsapp_flow: 'WhatsApp Flow',
  webhook: 'Webhook',
}
</script>

<template>
  <div class="space-y-4 p-4">
    <div class="flex items-center justify-between">
      <h3 class="font-semibold text-sm">{{ typeLabel[node.type] || node.type }}</h3>
      <Button v-if="node.type !== 'start'" variant="ghost" size="icon" class="h-7 w-7" @click="emit('delete')">
        <Trash2 class="h-3.5 w-3.5 text-destructive" />
      </Button>
    </div>

    <!-- Start: nothing to configure beyond the label. -->
    <p v-if="node.type === 'start'" class="text-xs text-muted-foreground">
      Flow entry point. Wire its output to the first node that should run.
    </p>

    <!-- Label -->
    <div v-if="node.type !== 'start'" class="space-y-1.5">
      <Label class="text-xs">Label</Label>
      <Input :model-value="node.label" @update:model-value="(v) => updateLabel(String(v ?? ''))" class="h-8 text-sm" />
    </div>

    <!-- text (message OR prompt) -->
    <template v-if="isTextNode">
      <div class="space-y-1.5">
        <Label class="text-xs">Message</Label>
        <Textarea
          :model-value="textBodyValue"
          @update:model-value="(v: string) => updateTextBody(String(v ?? ''))"
          placeholder="Enter your message"
          class="min-h-[80px] text-xs"
        />
        <p class="text-[10px] text-muted-foreground">Use double-brace placeholders (e.g. <code>customer_name</code>) to interpolate session variables.</p>
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Expected response</Label>
        <Select :model-value="expectedResponse" @update:model-value="(v: any) => setExpectedResponse(v)">
          <SelectTrigger class="h-8 text-sm"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="none">None (fire-and-forget)</SelectItem>
            <SelectItem value="text">Text</SelectItem>
            <SelectItem value="number">Number</SelectItem>
            <SelectItem value="email">Email</SelectItem>
            <SelectItem value="phone">Phone</SelectItem>
            <SelectItem value="date">Date</SelectItem>
            <SelectItem value="select">Selection</SelectItem>
          </SelectContent>
        </Select>
        <p class="text-[10px] text-muted-foreground">When set, the flow waits for the user's reply before continuing.</p>
      </div>
      <template v-if="node.type === 'prompt'">
        <div class="space-y-1.5">
          <Label class="text-xs">Store response as</Label>
          <Input
            :model-value="config.store_as || ''"
            @update:model-value="(v: string) => updateConfig('store_as', v)"
            placeholder="variable_name"
            class="h-8 text-sm font-mono"
          />
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">Validation regex (optional)</Label>
          <Input
            :model-value="config.validation_regex || ''"
            @update:model-value="(v: string) => updateConfig('validation_regex', v)"
            placeholder="^[0-9]+$"
            class="h-8 text-xs font-mono"
          />
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">Validation error message</Label>
          <Input
            :model-value="config.validation_error || ''"
            @update:model-value="(v: string) => updateConfig('validation_error', v)"
            placeholder="Invalid input. Please try again."
            class="h-8 text-xs"
          />
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">Max retries</Label>
          <Input
            type="number"
            :model-value="String(config.max_retries ?? 3)"
            @update:model-value="(v: string) => updateConfig('max_retries', parseInt(v) || 3)"
            class="h-8 text-sm"
            min="1"
            max="10"
          />
        </div>
      </template>
    </template>

    <!-- buttons -->
    <template v-if="node.type === 'buttons'">
      <div class="space-y-1.5">
        <Label class="text-xs">Body</Label>
        <Textarea
          :model-value="config.body || ''"
          @update:model-value="(v: string) => updateConfig('body', v)"
          placeholder="Message shown above the buttons"
          class="min-h-[60px] text-xs"
        />
      </div>
      <div class="space-y-1.5">
        <div class="flex items-center justify-between">
          <Label class="text-xs">Button Options ({{ (config.buttons || []).length }}/{{ hasCtaButtons ? 2 : 10 }})</Label>
        </div>
        <div class="flex gap-1">
          <Button variant="outline" size="sm" class="h-7 text-xs" :disabled="hasCtaButtons || replyCount >= 10" @click="addReplyButton">
            <Plus class="h-3 w-3 mr-0.5" /> Reply
          </Button>
          <Button variant="outline" size="sm" class="h-7 text-xs" :disabled="hasReplyButtons || ctaCount >= 2" @click="addUrlButton">
            <Plus class="h-3 w-3 mr-0.5" /> URL
          </Button>
          <Button variant="outline" size="sm" class="h-7 text-xs" :disabled="hasReplyButtons || ctaCount >= 2" @click="addPhoneButton">
            <Plus class="h-3 w-3 mr-0.5" /> Phone
          </Button>
        </div>
        <div v-for="(btn, idx) in (config.buttons || [])" :key="btn.id || idx" class="p-2 border rounded-md space-y-2 bg-muted/30">
          <div class="flex items-center gap-1">
            <span class="text-[10px] uppercase text-muted-foreground w-12">{{ btn.type || 'reply' }}</span>
            <Input
              :model-value="btn.title || ''"
              @update:model-value="(v: string) => updateButton(Number(idx), 'title', v)"
              placeholder="Button Title"
              class="h-7 text-xs flex-1"
            />
            <Button variant="ghost" size="icon" class="h-6 w-6" @click="removeButton(Number(idx))">
              <Trash2 class="h-3 w-3 text-destructive" />
            </Button>
          </div>
          <Input
            :model-value="btn.id || ''"
            @update:model-value="(v: string) => updateButton(Number(idx), 'id', v)"
            placeholder="button_id"
            class="h-7 text-xs font-mono"
          />
          <Input
            v-if="btn.type === 'url'"
            :model-value="btn.url || ''"
            @update:model-value="(v: string) => updateButton(Number(idx), 'url', v)"
            placeholder="https://example.com"
            class="h-7 text-xs font-mono"
          />
          <Input
            v-if="btn.type === 'phone'"
            :model-value="btn.phone_number || ''"
            @update:model-value="(v: string) => updateButton(Number(idx), 'phone_number', v)"
            placeholder="+1234567890"
            class="h-7 text-xs font-mono"
          />
        </div>
        <p class="text-[10px] text-muted-foreground">Reply buttons (max 10) send the user's choice back. URL / Phone buttons (max 2 per node, mutually exclusive with Reply) open a link or call. Wire reply buttons to next nodes by dragging from the button handle on the canvas.</p>
      </div>

      <!-- Input — buttons always expect a button selection; surface this
           for visual consistency with text nodes. -->
      <div class="pt-2 border-t space-y-1.5">
        <Label class="text-xs">Expected response</Label>
        <Select model-value="button" disabled>
          <SelectTrigger class="h-8 text-sm"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="button">Selection (buttons)</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <div class="space-y-1.5">
        <Label class="text-xs">Store response as (optional)</Label>
        <Input
          :model-value="config.store_as || ''"
          @update:model-value="(v: string) => updateConfig('store_as', v)"
          placeholder="variable_name"
          class="h-8 text-sm font-mono"
        />
        <p class="text-[10px] text-muted-foreground">Saves the tapped button's title into this variable so later nodes can reference it.</p>
      </div>

    </template>

    <!-- api_call -->
    <template v-if="node.type === 'api_call'">
      <div class="space-y-1.5">
        <Label class="text-xs">URL</Label>
        <Input
          :model-value="config.url || ''"
          @update:model-value="(v: string) => updateConfig('url', v)"
          placeholder="https://api.example.com/endpoint"
          class="h-8 text-xs font-mono"
        />
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Method</Label>
        <Select :model-value="config.method || 'GET'" @update:model-value="(v: any) => updateConfig('method', v)">
          <SelectTrigger class="h-8 text-sm"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="GET">GET</SelectItem>
            <SelectItem value="POST">POST</SelectItem>
            <SelectItem value="PUT">PUT</SelectItem>
            <SelectItem value="PATCH">PATCH</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div class="space-y-1.5">
        <div class="flex items-center justify-between">
          <Label class="text-xs">Headers</Label>
          <Button variant="outline" size="sm" class="h-6 text-xs" @click="addHeader">
            <Plus class="h-3 w-3 mr-1" /> Add
          </Button>
        </div>
        <div v-for="(val, key) in (config.headers || {})" :key="String(key)" class="flex items-center gap-1">
          <Input :model-value="String(key)" @update:model-value="(v: string) => updateHeaderKey(String(key), v)" placeholder="Key" class="h-7 text-xs flex-1" />
          <Input :model-value="String(val)" @update:model-value="(v: string) => updateHeaderValue(String(key), v)" placeholder="Value" class="h-7 text-xs flex-1" />
          <Button variant="ghost" size="icon" class="h-6 w-6" @click="removeHeader(String(key))">
            <Trash2 class="h-3 w-3 text-destructive" />
          </Button>
        </div>
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Body</Label>
        <Textarea
          :model-value="config.body || ''"
          @update:model-value="(v: string) => updateConfig('body', v)"
          placeholder='{"phone": "{{phone_number}}"}'
          class="min-h-[60px] text-xs font-mono"
        />
      </div>
      <div class="space-y-1.5">
        <div class="flex items-center justify-between">
          <Label class="text-xs">Response mapping</Label>
          <Button variant="outline" size="sm" class="h-6 text-xs" @click="addResponseMapping">
            <Plus class="h-3 w-3 mr-1" /> Add
          </Button>
        </div>
        <p class="text-[10px] text-muted-foreground">Map JSON paths into session variables (e.g. <code>data.user.name</code>).</p>
        <div v-for="(val, key) in (config.response_mapping || {})" :key="String(key)" class="flex items-center gap-1">
          <Input :model-value="String(key)" @update:model-value="(v: string) => updateResponseMappingKey(String(key), v)" placeholder="var_name" class="h-7 text-xs flex-1 font-mono" />
          <Input :model-value="String(val)" @update:model-value="(v: string) => updateResponseMappingValue(String(key), v)" placeholder="path.to.field" class="h-7 text-xs flex-1 font-mono" />
          <Button variant="ghost" size="icon" class="h-6 w-6" @click="removeResponseMapping(String(key))">
            <Trash2 class="h-3 w-3 text-destructive" />
          </Button>
        </div>
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Message template (optional)</Label>
        <Textarea
          :model-value="config.message_template || ''"
          @update:model-value="(v: string) => updateConfig('message_template', v)"
          placeholder="Hello {{user_name}}!"
          class="min-h-[50px] text-xs"
        />
        <p class="text-[10px] text-muted-foreground">Sent on 2xx response after mappings are applied.</p>
      </div>
    </template>

    <!-- condition -->
    <template v-if="node.type === 'condition'">
      <div class="space-y-1.5">
        <Label class="text-xs">Expression</Label>
        <Textarea
          :model-value="config.expression || ''"
          @update:model-value="(v: string) => updateConfig('expression', v)"
          placeholder='tier == "premium" and amount > 100'
          class="min-h-[60px] text-xs font-mono"
        />
        <p class="text-[10px] text-muted-foreground">Routes via the <code>true</code> / <code>false</code> handle. Uses expr-lang syntax.</p>
      </div>
    </template>

    <!-- timing -->
    <template v-if="node.type === 'timing'">
      <div class="space-y-1.5">
        <Label class="text-xs">Schedule</Label>
        <div v-for="(entry, idx) in schedule" :key="idx" class="flex items-center gap-1.5 text-xs">
          <span class="w-12 capitalize">{{ entry.day.slice(0, 3) }}</span>
          <Switch :checked="entry.enabled" @update:checked="(v: boolean) => updateScheduleEntry(Number(idx), 'enabled', v)" />
          <Input
            v-if="entry.enabled"
            type="time"
            :model-value="entry.start_time"
            @update:model-value="(v: string) => updateScheduleEntry(Number(idx), 'start_time', v)"
            class="h-8 text-xs w-28"
          />
          <span v-if="entry.enabled" class="text-muted-foreground">-</span>
          <Input
            v-if="entry.enabled"
            type="time"
            :model-value="entry.end_time"
            @update:model-value="(v: string) => updateScheduleEntry(Number(idx), 'end_time', v)"
            class="h-8 text-xs w-28"
          />
        </div>
        <p class="text-[10px] text-muted-foreground">Routes via <code>in_hours</code> / <code>out_of_hours</code>.</p>
      </div>
    </template>

    <!-- transfer -->
    <template v-if="node.type === 'transfer'">
      <div class="space-y-1.5">
        <Label class="text-xs">Body (sent before handoff)</Label>
        <Textarea
          :model-value="config.body || ''"
          @update:model-value="(v: string) => updateConfig('body', v)"
          placeholder="Connecting you with a human..."
          class="min-h-[50px] text-xs"
        />
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Team</Label>
        <Select :model-value="config.team_id || '_general'" @update:model-value="(v: any) => updateConfig('team_id', v)">
          <SelectTrigger class="h-8 text-sm"><SelectValue placeholder="General queue" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="_general">General queue</SelectItem>
            <SelectItem v-for="team in teamsStore.teams" :key="team.id" :value="team.id">
              {{ team.name }}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Notes (for agents)</Label>
        <Textarea
          :model-value="config.notes || ''"
          @update:model-value="(v: string) => updateConfig('notes', v)"
          placeholder="Customer asked about: {{topic}}"
          class="min-h-[50px] text-xs"
        />
      </div>
    </template>

    <!-- end -->
    <template v-if="node.type === 'end'">
      <div class="space-y-1.5">
        <Label class="text-xs">Final message (optional)</Label>
        <Textarea
          :model-value="config.message || ''"
          @update:model-value="(v: string) => updateConfig('message', v)"
          placeholder="Sent when the flow ends. Leave blank for silent terminal."
          class="min-h-[60px] text-xs"
        />
      </div>
    </template>

    <!-- goto_flow -->
    <template v-if="node.type === 'goto_flow'">
      <div class="space-y-1.5">
        <Label class="text-xs">Target flow</Label>
        <Select :model-value="config.flow_id || 'none'" @update:model-value="(v: any) => updateConfig('flow_id', v === 'none' ? '' : v)">
          <SelectTrigger class="h-8 text-sm"><SelectValue placeholder="Select flow" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="none">Select flow…</SelectItem>
            <SelectItem v-for="flow in gotoFlowTargets" :key="flow.id" :value="flow.id">
              {{ flow.name }}
            </SelectItem>
          </SelectContent>
        </Select>
        <p class="text-[10px] text-muted-foreground">Session variables carry forward into the target flow.</p>
      </div>
    </template>

    <!-- whatsapp_flow -->
    <template v-if="node.type === 'whatsapp_flow'">
      <div class="space-y-1.5">
        <Label class="text-xs">WhatsApp Flow ID</Label>
        <Input
          :model-value="config.flow_id || ''"
          @update:model-value="(v: string) => updateConfig('flow_id', v)"
          placeholder="Meta flow id"
          class="h-8 text-xs font-mono"
        />
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Header</Label>
        <Input
          :model-value="config.header || ''"
          @update:model-value="(v: string) => updateConfig('header', v)"
          class="h-8 text-xs"
        />
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Body</Label>
        <Textarea
          :model-value="config.body || ''"
          @update:model-value="(v: string) => updateConfig('body', v)"
          class="min-h-[50px] text-xs"
        />
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">CTA label</Label>
        <Input
          :model-value="config.cta || ''"
          @update:model-value="(v: string) => updateConfig('cta', v)"
          placeholder="Open form"
          class="h-8 text-xs"
        />
      </div>
    </template>

    <!-- webhook -->
    <template v-if="node.type === 'webhook'">
      <div class="space-y-1.5">
        <Label class="text-xs">URL</Label>
        <Input
          :model-value="config.url || ''"
          @update:model-value="(v: string) => updateConfig('url', v)"
          placeholder="https://example.com/hook"
          class="h-8 text-xs font-mono"
        />
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Method</Label>
        <Select :model-value="config.method || 'POST'" @update:model-value="(v: any) => updateConfig('method', v)">
          <SelectTrigger class="h-8 text-sm"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="GET">GET</SelectItem>
            <SelectItem value="POST">POST</SelectItem>
            <SelectItem value="PUT">PUT</SelectItem>
            <SelectItem value="PATCH">PATCH</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div class="space-y-1.5">
        <div class="flex items-center justify-between">
          <Label class="text-xs">Headers</Label>
          <Button variant="outline" size="sm" class="h-6 text-xs" @click="addHeader">
            <Plus class="h-3 w-3 mr-1" /> Add
          </Button>
        </div>
        <div v-for="(val, key) in (config.headers || {})" :key="String(key)" class="flex items-center gap-1">
          <Input :model-value="String(key)" @update:model-value="(v: string) => updateHeaderKey(String(key), v)" placeholder="Key" class="h-7 text-xs flex-1" />
          <Input :model-value="String(val)" @update:model-value="(v: string) => updateHeaderValue(String(key), v)" placeholder="Value" class="h-7 text-xs flex-1" />
          <Button variant="ghost" size="icon" class="h-6 w-6" @click="removeHeader(String(key))">
            <Trash2 class="h-3 w-3 text-destructive" />
          </Button>
        </div>
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Body</Label>
        <Textarea
          :model-value="config.body || ''"
          @update:model-value="(v: string) => updateConfig('body', v)"
          class="min-h-[50px] text-xs font-mono"
        />
      </div>
    </template>

    <!-- Skip condition. Evaluated by the runner before executing the
         node; truthy → fall through via the default edge without
         sending anything.

         Hidden for nodes whose whole purpose is branching (condition /
         buttons / timing — they have no default edge) and terminal
         nodes (end / transfer / goto_flow) where there's nothing to
         skip past. -->
    <div
      v-if="!['start', 'end', 'transfer', 'goto_flow', 'condition', 'buttons', 'timing'].includes(node.type)"
      class="pt-2 border-t space-y-1.5"
    >
      <Label class="text-xs">Skip condition (optional)</Label>
      <Input
        :model-value="config.skip_condition || ''"
        @update:model-value="(v: string) => updateConfig('skip_condition', v)"
        placeholder='tier == "premium"'
        class="h-8 text-xs font-mono"
      />
      <p class="text-[10px] text-muted-foreground">Skip this node when the expression evaluates truthy — execution continues via the default edge.</p>
    </div>
  </div>
</template>
