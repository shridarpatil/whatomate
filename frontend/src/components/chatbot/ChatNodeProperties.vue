<script setup lang="ts">
import { computed } from 'vue'
import type { ChatNode } from '@/services/api'
import { useTeamsStore } from '@/stores/teams'
import { Trash2, Plus } from '@/lib/icons'

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

// Value of the input/select/textarea that fired an event.
function val(e: Event): string {
  return (e.target as HTMLInputElement).value
}

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
  <div>
    <div class="hstack justify-between mb-4">
      <h4>{{ typeLabel[node.type] || node.type }}</h4>
      <button
        v-if="node.type !== 'start'"
        type="button"
        class="ghost small icon"
        data-variant="danger"
        :aria-label="$t('common.delete')"
        :title="$t('common.delete')"
        @click="emit('delete')"
      >
        <Trash2 />
      </button>
    </div>

    <!-- Start: nothing to configure beyond the label. -->
    <p v-if="node.type === 'start'" class="text-light">
      <small>Flow entry point. Wire its output to the first node that should run.</small>
    </p>

    <!-- Label -->
    <label v-if="node.type !== 'start'" data-field>
      Label
      <input :value="node.label" @input="updateLabel(val($event))">
    </label>

    <!-- text (message OR prompt) -->
    <template v-if="isTextNode">
      <label data-field>
        Message
        <textarea
          :value="textBodyValue"
          placeholder="Enter your message"
          rows="3"
          @input="updateTextBody(val($event))"
        />
        <small data-hint>Use double-brace placeholders (e.g. <code>customer_name</code>) to interpolate session variables.</small>
      </label>
      <label data-field>
        Expected response
        <select :value="expectedResponse" @change="setExpectedResponse(val($event))">
          <option value="none">None (fire-and-forget)</option>
          <option value="text">Text</option>
          <option value="number">Number</option>
          <option value="email">Email</option>
          <option value="phone">Phone</option>
          <option value="date">Date</option>
          <option value="select">Selection</option>
        </select>
        <small data-hint>When set, the flow waits for the user's reply before continuing.</small>
      </label>
      <template v-if="node.type === 'prompt'">
        <label data-field>
          Store response as
          <input :value="config.store_as || ''" placeholder="variable_name" @input="updateConfig('store_as', val($event))">
        </label>
        <label data-field>
          Validation regex (optional)
          <input :value="config.validation_regex || ''" placeholder="^[0-9]+$" @input="updateConfig('validation_regex', val($event))">
        </label>
        <label data-field>
          Validation error message
          <input
            :value="config.validation_error || ''"
            placeholder="Invalid input. Please try again."
            @input="updateConfig('validation_error', val($event))"
          >
        </label>
        <label data-field>
          Max retries
          <input
            type="number"
            :value="String(config.max_retries ?? 3)"
            min="1"
            max="10"
            @input="updateConfig('max_retries', parseInt(val($event)) || 3)"
          >
        </label>
      </template>
    </template>

    <!-- buttons -->
    <template v-if="node.type === 'buttons'">
      <label data-field>
        Body
        <textarea
          :value="config.body || ''"
          placeholder="Message shown above the buttons"
          rows="2"
          @input="updateConfig('body', val($event))"
        />
      </label>
      <div class="vstack gap-2 mb-4">
        <span>Button Options ({{ (config.buttons || []).length }}/{{ hasCtaButtons ? 2 : 10 }})</span>
        <div class="hstack gap-1">
          <button type="button" class="outline small" :disabled="hasCtaButtons || replyCount >= 10" @click="addReplyButton">
            <Plus /> Reply
          </button>
          <button type="button" class="outline small" :disabled="hasReplyButtons || ctaCount >= 2" @click="addUrlButton">
            <Plus /> URL
          </button>
          <button type="button" class="outline small" :disabled="hasReplyButtons || ctaCount >= 2" @click="addPhoneButton">
            <Plus /> Phone
          </button>
        </div>
        <article
          v-for="(btn, idx) in (config.buttons || [])"
          :key="btn.id || idx"
          class="card p-4 vstack gap-2"
          data-button-config
        >
          <div class="hstack gap-1">
            <span class="badge" data-variant="secondary">{{ btn.type || 'reply' }}</span>
            <input
              class="flex-1 min-w-0"
              :value="btn.title || ''"
              placeholder="Button Title"
              aria-label="Button Title"
              @input="updateButton(Number(idx), 'title', val($event))"
            >
            <button
              type="button"
              class="ghost small icon"
              data-variant="danger"
              :aria-label="$t('common.delete')"
              :title="$t('common.delete')"
              @click="removeButton(Number(idx))"
            >
              <Trash2 />
            </button>
          </div>
          <input
            :value="btn.id || ''"
            placeholder="button_id"
            aria-label="button_id"
            @input="updateButton(Number(idx), 'id', val($event))"
          >
          <input
            v-if="btn.type === 'url'"
            type="url"
            :value="btn.url || ''"
            placeholder="https://example.com"
            aria-label="URL"
            @input="updateButton(Number(idx), 'url', val($event))"
          >
          <input
            v-if="btn.type === 'phone'"
            type="tel"
            :value="btn.phone_number || ''"
            placeholder="+1234567890"
            aria-label="Phone"
            @input="updateButton(Number(idx), 'phone_number', val($event))"
          >
        </article>
        <small class="text-light">Reply buttons (max 10) send the user's choice back. URL / Phone buttons (max 2 per node, mutually exclusive with Reply) open a link or call. Wire reply buttons to next nodes by dragging from the button handle on the canvas.</small>
      </div>

      <hr>

      <!-- Input — buttons always expect a button selection; surface this
           for visual consistency with text nodes. -->
      <label data-field>
        Expected response
        <select value="button" disabled>
          <option value="button">Selection (buttons)</option>
        </select>
      </label>

      <label data-field>
        Store response as (optional)
        <input :value="config.store_as || ''" placeholder="variable_name" @input="updateConfig('store_as', val($event))">
        <small data-hint>Saves the tapped button's title into this variable so later nodes can reference it.</small>
      </label>
    </template>

    <!-- api_call -->
    <template v-if="node.type === 'api_call'">
      <label data-field>
        URL
        <input :value="config.url || ''" placeholder="https://api.example.com/endpoint" @input="updateConfig('url', val($event))">
      </label>
      <label data-field>
        Method
        <select :value="config.method || 'GET'" @change="updateConfig('method', val($event))">
          <option value="GET">GET</option>
          <option value="POST">POST</option>
          <option value="PUT">PUT</option>
          <option value="PATCH">PATCH</option>
        </select>
      </label>
      <div class="vstack gap-2 mb-4">
        <div class="hstack justify-between">
          <span>Headers</span>
          <button type="button" class="outline small" @click="addHeader">
            <Plus /> Add
          </button>
        </div>
        <div v-for="(v, key) in (config.headers || {})" :key="String(key)" class="hstack gap-1">
          <input class="flex-1 min-w-0" :value="String(key)" placeholder="Key" aria-label="Key" @input="updateHeaderKey(String(key), val($event))">
          <input class="flex-1 min-w-0" :value="String(v)" placeholder="Value" aria-label="Value" @input="updateHeaderValue(String(key), val($event))">
          <button
            type="button"
            class="ghost small icon"
            data-variant="danger"
            :aria-label="$t('common.delete')"
            :title="$t('common.delete')"
            @click="removeHeader(String(key))"
          >
            <Trash2 />
          </button>
        </div>
      </div>
      <label data-field>
        Body
        <textarea
          :value="config.body || ''"
          placeholder='{"phone": "{{phone_number}}"}'
          rows="3"
          @input="updateConfig('body', val($event))"
        />
      </label>
      <div class="vstack gap-2 mb-4">
        <div class="hstack justify-between">
          <span>Response mapping</span>
          <button type="button" class="outline small" @click="addResponseMapping">
            <Plus /> Add
          </button>
        </div>
        <small class="text-light">Map JSON paths into session variables (e.g. <code>data.user.name</code>).</small>
        <div v-for="(v, key) in (config.response_mapping || {})" :key="String(key)" class="hstack gap-1">
          <input class="flex-1 min-w-0" :value="String(key)" placeholder="var_name" aria-label="var_name" @input="updateResponseMappingKey(String(key), val($event))">
          <input class="flex-1 min-w-0" :value="String(v)" placeholder="path.to.field" aria-label="path.to.field" @input="updateResponseMappingValue(String(key), val($event))">
          <button
            type="button"
            class="ghost small icon"
            data-variant="danger"
            :aria-label="$t('common.delete')"
            :title="$t('common.delete')"
            @click="removeResponseMapping(String(key))"
          >
            <Trash2 />
          </button>
        </div>
      </div>
      <label data-field>
        Message template (optional)
        <textarea
          :value="config.message_template || ''"
          placeholder="Hello {{user_name}}!"
          rows="2"
          @input="updateConfig('message_template', val($event))"
        />
        <small data-hint>Sent on 2xx response after mappings are applied.</small>
      </label>
    </template>

    <!-- condition -->
    <template v-if="node.type === 'condition'">
      <label data-field>
        Expression
        <textarea
          :value="config.expression || ''"
          placeholder='tier == "premium" and amount > 100'
          rows="3"
          @input="updateConfig('expression', val($event))"
        />
        <small data-hint>Routes via the <code>true</code> / <code>false</code> handle. Uses expr-lang syntax.</small>
      </label>
    </template>

    <!-- timing -->
    <template v-if="node.type === 'timing'">
      <fieldset class="vstack gap-2 mb-4">
        <legend>Schedule</legend>
        <div v-for="(entry, idx) in schedule" :key="idx" class="hstack gap-2">
          <label class="hstack gap-2 shrink-0">
            <input
              type="checkbox"
              role="switch"
              :checked="entry.enabled"
              @change="updateScheduleEntry(Number(idx), 'enabled', ($event.target as HTMLInputElement).checked)"
            >
            <small>{{ entry.day.slice(0, 3) }}</small>
          </label>
          <template v-if="entry.enabled">
            <input
              type="time"
              class="flex-1 min-w-0"
              :value="entry.start_time"
              :aria-label="`${entry.day} start`"
              @input="updateScheduleEntry(Number(idx), 'start_time', val($event))"
            >
            <span class="text-light">-</span>
            <input
              type="time"
              class="flex-1 min-w-0"
              :value="entry.end_time"
              :aria-label="`${entry.day} end`"
              @input="updateScheduleEntry(Number(idx), 'end_time', val($event))"
            >
          </template>
        </div>
        <small class="text-light">Routes via <code>in_hours</code> / <code>out_of_hours</code>.</small>
      </fieldset>
    </template>

    <!-- transfer -->
    <template v-if="node.type === 'transfer'">
      <label data-field>
        Body (sent before handoff)
        <textarea
          :value="config.body || ''"
          placeholder="Connecting you with a human..."
          rows="2"
          @input="updateConfig('body', val($event))"
        />
      </label>
      <label data-field>
        Team
        <select :value="config.team_id || '_general'" @change="updateConfig('team_id', val($event))">
          <option value="_general">General queue</option>
          <option v-for="team in teamsStore.teams" :key="team.id" :value="team.id">
            {{ team.name }}
          </option>
        </select>
      </label>
      <label data-field>
        Notes (for agents)
        <textarea
          :value="config.notes || ''"
          placeholder="Customer asked about: {{topic}}"
          rows="2"
          @input="updateConfig('notes', val($event))"
        />
      </label>
    </template>

    <!-- end -->
    <template v-if="node.type === 'end'">
      <label data-field>
        Final message (optional)
        <textarea
          :value="config.message || ''"
          placeholder="Sent when the flow ends. Leave blank for silent terminal."
          rows="3"
          @input="updateConfig('message', val($event))"
        />
      </label>
    </template>

    <!-- goto_flow -->
    <template v-if="node.type === 'goto_flow'">
      <label data-field>
        Target flow
        <select :value="config.flow_id || 'none'" @change="updateConfig('flow_id', val($event) === 'none' ? '' : val($event))">
          <option value="none">Select flow…</option>
          <option v-for="flow in gotoFlowTargets" :key="flow.id" :value="flow.id">
            {{ flow.name }}
          </option>
        </select>
        <small data-hint>Session variables carry forward into the target flow.</small>
      </label>
    </template>

    <!-- whatsapp_flow -->
    <template v-if="node.type === 'whatsapp_flow'">
      <label data-field>
        WhatsApp Flow ID
        <input :value="config.flow_id || ''" placeholder="Meta flow id" @input="updateConfig('flow_id', val($event))">
      </label>
      <label data-field>
        Header
        <input :value="config.header || ''" @input="updateConfig('header', val($event))">
      </label>
      <label data-field>
        Body
        <textarea :value="config.body || ''" rows="2" @input="updateConfig('body', val($event))" />
      </label>
      <label data-field>
        CTA label
        <input :value="config.cta || ''" placeholder="Open form" @input="updateConfig('cta', val($event))">
      </label>
    </template>

    <!-- webhook -->
    <template v-if="node.type === 'webhook'">
      <label data-field>
        URL
        <input :value="config.url || ''" placeholder="https://example.com/hook" @input="updateConfig('url', val($event))">
      </label>
      <label data-field>
        Method
        <select :value="config.method || 'POST'" @change="updateConfig('method', val($event))">
          <option value="GET">GET</option>
          <option value="POST">POST</option>
          <option value="PUT">PUT</option>
          <option value="PATCH">PATCH</option>
        </select>
      </label>
      <div class="vstack gap-2 mb-4">
        <div class="hstack justify-between">
          <span>Headers</span>
          <button type="button" class="outline small" @click="addHeader">
            <Plus /> Add
          </button>
        </div>
        <div v-for="(v, key) in (config.headers || {})" :key="String(key)" class="hstack gap-1">
          <input class="flex-1 min-w-0" :value="String(key)" placeholder="Key" aria-label="Key" @input="updateHeaderKey(String(key), val($event))">
          <input class="flex-1 min-w-0" :value="String(v)" placeholder="Value" aria-label="Value" @input="updateHeaderValue(String(key), val($event))">
          <button
            type="button"
            class="ghost small icon"
            data-variant="danger"
            :aria-label="$t('common.delete')"
            :title="$t('common.delete')"
            @click="removeHeader(String(key))"
          >
            <Trash2 />
          </button>
        </div>
      </div>
      <label data-field>
        Body
        <textarea :value="config.body || ''" rows="2" @input="updateConfig('body', val($event))" />
      </label>
    </template>

    <!-- Skip condition. Evaluated by the runner before executing the
         node; truthy → fall through via the default edge without
         sending anything.

         Hidden for nodes whose whole purpose is branching (condition /
         buttons / timing — they have no default edge) and terminal
         nodes (end / transfer / goto_flow) where there's nothing to
         skip past. -->
    <template v-if="!['start', 'end', 'transfer', 'goto_flow', 'condition', 'buttons', 'timing'].includes(node.type)">
      <hr>
      <label data-field>
        Skip condition (optional)
        <input :value="config.skip_condition || ''" placeholder='tier == "premium"' @input="updateConfig('skip_condition', val($event))">
        <small data-hint>Skip this node when the expression evaluates truthy — execution continues via the default edge.</small>
      </label>
    </template>
  </div>
</template>
