<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { IVRNode, IVRNodeType } from '@/services/api'
import { ivrFlowsService } from '@/services/api'
import { useCallingStore } from '@/stores/calling'
import { useTeamsStore } from '@/stores/teams'
import { Trash2, Plus, Upload, Play, Pause, X, Type } from '@/lib/icons'
import { toast } from '@/lib/toast'

const props = defineProps<{
  node: IVRNode
  currentFlowId?: string
}>()

const emit = defineEmits<{
  'update:node': [node: IVRNode]
  'delete': []
}>()

const callingStore = useCallingStore()
const teamsStore = useTeamsStore()

if (teamsStore.teams.length === 0) teamsStore.fetchTeams()

const config = computed(() => props.node.config || {})

function updateConfig(key: string, value: any) {
  emit('update:node', {
    ...props.node,
    config: { ...props.node.config, [key]: value }
  })
}

// Set multiple config keys in a single emit. Calling updateConfig twice in a
// row loses the first change: the prop doesn't update synchronously after
// emit, so the second call rebuilds from a stale props.node and clobbers it.
function updateConfigEntries(entries: Record<string, any>) {
  emit('update:node', {
    ...props.node,
    config: { ...props.node.config, ...entries }
  })
}

function updateLabel(label: string) {
  emit('update:node', { ...props.node, label })
}

// Audio upload state
const audioFileInput = ref<HTMLInputElement | null>(null)
const isUploading = ref(false)
const isPlaying = ref(false)
const audioElement = ref<HTMLAudioElement | null>(null)

function triggerFileUpload() {
  audioFileInput.value?.click()
}

async function handleFileSelect(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (file.size > 5 * 1024 * 1024) {
    toast.error('File too large (max 5MB)')
    input.value = ''
    return
  }
  isUploading.value = true
  try {
    const res = await ivrFlowsService.uploadAudio(file)
    const filename = res.data?.data?.filename
    if (filename) {
      // Uploading a clip and TTS text are mutually exclusive; clear greeting_text
      // so the backend doesn't TTS-regenerate and overwrite the uploaded audio_file.
      updateConfigEntries({ audio_file: filename, greeting_text: undefined })
      toast.success('Audio uploaded')
    }
  } catch {
    toast.error('Upload failed')
  } finally {
    isUploading.value = false
    input.value = ''
  }
}

function removeAudio() {
  stopAudio()
  updateConfig('audio_file', '')
}

function togglePlayback() {
  if (isPlaying.value) stopAudio()
  else playAudio()
}

function playAudio() {
  if (!config.value.audio_file) return
  stopAudio()
  const audio = new Audio(ivrFlowsService.getAudioUrl(config.value.audio_file))
  audio.onended = () => { isPlaying.value = false }
  audio.onerror = () => { isPlaying.value = false }
  audio.play()
  audioElement.value = audio
  isPlaying.value = true
}

function stopAudio() {
  if (audioElement.value) {
    audioElement.value.pause()
    audioElement.value = null
  }
  isPlaying.value = false
}

// Menu options helpers
function addMenuOption() {
  const opts = { ...(config.value.options || {}) }
  const used = new Set(Object.keys(opts))
  const digit = ['1','2','3','4','5','6','7','8','9','0','*','#'].find(d => !used.has(d))
  if (!digit) return
  opts[digit] = { label: '' }
  updateConfig('options', opts)
}

function removeMenuOption(digit: string) {
  const opts = { ...(config.value.options || {}) }
  delete opts[digit]
  updateConfig('options', opts)
}

function updateMenuOption(digit: string, field: string, value: string) {
  const opts = { ...(config.value.options || {}) }
  opts[digit] = { ...opts[digit], [field]: value }
  updateConfig('options', opts)
}

// Timing schedule helpers
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

// HTTP headers helpers
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

// Transfer callback helpers
const callbackEvents = ['on_waiting', 'on_connect'] as const
type CallbackEvent = typeof callbackEvents[number]

const callbackLabels: Record<CallbackEvent, string> = {
  on_waiting: 'On Waiting',
  on_connect: 'On Connect',
}

function getCallbackConfig(event: CallbackEvent) {
  return (config.value[event] as Record<string, any>) || {}
}

function updateCallbackField(event: CallbackEvent, field: string, value: any) {
  const cb = { ...getCallbackConfig(event), [field]: value }
  updateConfig(event, cb)
}

function addCallbackHeader(event: CallbackEvent) {
  const cb = getCallbackConfig(event)
  const headers = { ...(cb.headers || {}), '': '' }
  updateCallbackField(event, 'headers', headers)
}

function removeCallbackHeader(event: CallbackEvent, key: string) {
  const cb = getCallbackConfig(event)
  const headers = { ...(cb.headers || {}) }
  delete headers[key]
  updateCallbackField(event, 'headers', headers)
}

function updateCallbackHeaderKey(event: CallbackEvent, oldKey: string, newKey: string) {
  if (oldKey === newKey) return
  const cb = getCallbackConfig(event)
  const headers = { ...(cb.headers || {}) }
  headers[newKey] = headers[oldKey]
  delete headers[oldKey]
  updateCallbackField(event, 'headers', headers)
}

function updateCallbackHeaderValue(event: CallbackEvent, key: string, value: string) {
  const cb = getCallbackConfig(event)
  const headers = { ...(cb.headers || {}) }
  headers[key] = value
  updateCallbackField(event, 'headers', headers)
}

// Goto flow targets
const gotoFlowTargets = computed(() =>
  callingStore.ivrFlows.filter(f => f.id !== props.currentFlowId)
)

// Audio section used by greeting, menu, gather, hangup
const audioNodeTypes: IVRNodeType[] = ['greeting', 'menu', 'gather', 'hangup']
const hasAudio = computed(() => audioNodeTypes.includes(props.node.type))

const greetingTab = computed(() =>
  config.value.greeting_text ? 'text' : 'audio'
)

// Upload / TTS tab: opens on whichever the node uses, reset when another node is selected.
const audioTab = ref<'audio' | 'text'>(greetingTab.value)
watch(() => props.node.id, () => { audioTab.value = greetingTab.value })

// The active audio tab is outlined, the other is ghost.
const audioTabClass = computed(() => ({
  audio: audioTab.value === 'audio' ? 'outline' : 'ghost',
  text: audioTab.value === 'text' ? 'outline' : 'ghost',
}))

const typeTitle = computed(() => props.node.type.replace('_', ' ').replace(/\b\w/g, (c) => c.toUpperCase()))

function val(e: Event): string {
  return (e.target as HTMLInputElement).value
}

function checked(e: Event): boolean {
  return (e.target as HTMLInputElement).checked
}

const transferVariables = ['caller_phone', 'contact_id', 'call_log_id', 'transfer_id', 'team_id', 'whatsapp_account', 'status', 'transferred_at', 'ivr_path', 'agent_id *', 'agent_email *', 'agent_name *', 'hold_duration **', 'talk_duration **']
</script>

<template>
  <div>
    <div class="hstack justify-between mb-4">
      <h4>{{ typeTitle }}</h4>
      <button
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

    <!-- Label -->
    <label data-field>
      Label
      <input :value="node.label" @input="updateLabel(val($event))">
    </label>

    <!-- Audio Section (greeting, menu, gather, hangup) -->
    <div v-if="hasAudio" class="vstack gap-2 mb-4">
      <span>Audio</span>
      <div role="tablist" class="hstack gap-1">
        <button
          type="button"
          role="tab"
          class="small"
          :class="audioTabClass.audio"
          :aria-selected="audioTab === 'audio' ? 'true' : 'false'"
          aria-controls="ivr-audio-upload"
          @click="audioTab = 'audio'"
        >
          <Upload :size="14" /> Upload
        </button>
        <button
          type="button"
          role="tab"
          class="small"
          :class="audioTabClass.text"
          :aria-selected="audioTab === 'text' ? 'true' : 'false'"
          aria-controls="ivr-audio-tts"
          @click="audioTab = 'text'"
        >
          <Type :size="14" /> TTS
        </button>
      </div>
      <div v-if="audioTab === 'audio'" id="ivr-audio-upload" role="tabpanel">
        <article v-if="config.audio_file" class="card p-4 hstack gap-1 min-w-0">
          <button
            type="button"
            class="ghost small icon shrink-0"
            :aria-label="isPlaying ? $t('settings.pauseAudio') : $t('settings.playAudio')"
            :aria-pressed="isPlaying ? 'true' : 'false'"
            @click="togglePlayback"
          >
            <Pause v-if="isPlaying" />
            <Play v-else />
          </button>
          <small class="truncate flex-1">{{ config.audio_file }}</small>
          <button
            type="button"
            class="ghost small icon shrink-0"
            data-variant="danger"
            :aria-label="$t('common.remove')"
            :title="$t('common.remove')"
            @click="removeAudio"
          >
            <X />
          </button>
        </article>
        <button
          v-else
          type="button"
          class="outline small w-100"
          :disabled="isUploading"
          :aria-busy="isUploading ? 'true' : undefined"
          data-spinner="small"
          @click="triggerFileUpload"
        >
          <Upload v-if="!isUploading" />
          Upload Audio
        </button>
        <input ref="audioFileInput" type="file" accept="audio/*" hidden @change="handleFileSelect">
      </div>
      <div v-else id="ivr-audio-tts" role="tabpanel">
        <textarea
          :value="config.greeting_text || ''"
          placeholder="Enter text for TTS..."
          :aria-label="$t('calling.ttsText')"
          rows="3"
          maxlength="500"
          @input="updateConfigEntries(val($event) ? { greeting_text: val($event), audio_file: undefined } : { greeting_text: val($event) })"
        />
      </div>
    </div>

    <!-- Greeting: interruptible -->
    <label v-if="node.type === 'greeting'" class="hstack gap-2 mb-4">
      <input type="checkbox" role="switch" :checked="!!config.interruptible" @change="updateConfig('interruptible', checked($event))">
      <small>Interruptible by DTMF</small>
    </label>

    <!-- Menu: options -->
    <template v-if="node.type === 'menu'">
      <label data-field>
        Timeout (seconds)
        <input type="number" :value="String(config.timeout_seconds || 10)" min="1" max="60" @input="updateConfig('timeout_seconds', parseInt(val($event)) || 10)">
      </label>
      <label data-field>
        Max Retries
        <input type="number" :value="String(config.max_retries || 3)" min="1" max="10" @input="updateConfig('max_retries', parseInt(val($event)) || 3)">
      </label>
      <div class="vstack gap-2 mb-4">
        <div class="hstack justify-between">
          <span>Menu Options</span>
          <button type="button" class="outline small" @click="addMenuOption">
            <Plus /> Add
          </button>
        </div>
        <div v-for="(opt, digit) in (config.options || {})" :key="String(digit)" class="hstack gap-2">
          <code class="shrink-0">{{ digit }}</code>
          <input
            class="flex-1 min-w-0"
            :value="(opt as any)?.label || ''"
            placeholder="Label"
            :aria-label="`${digit} Label`"
            @input="updateMenuOption(String(digit), 'label', val($event))"
          >
          <button
            type="button"
            class="ghost small icon"
            data-variant="danger"
            :aria-label="$t('common.delete')"
            :title="$t('common.delete')"
            @click="removeMenuOption(String(digit))"
          >
            <Trash2 />
          </button>
        </div>
      </div>
    </template>

    <!-- Gather: config -->
    <template v-if="node.type === 'gather'">
      <label data-field>
        Max Digits
        <input type="number" :value="String(config.max_digits || 10)" min="1" max="20" @input="updateConfig('max_digits', parseInt(val($event)) || 10)">
      </label>
      <label data-field>
        Terminator
        <input :value="config.terminator || '#'" @input="updateConfig('terminator', val($event))">
      </label>
      <label data-field>
        Store As (variable name)
        <input :value="config.store_as || ''" placeholder="e.g. account_number" @input="updateConfig('store_as', val($event))">
      </label>
      <label data-field>
        Timeout (seconds)
        <input type="number" :value="String(config.timeout_seconds || 10)" min="1" max="60" @input="updateConfig('timeout_seconds', parseInt(val($event)) || 10)">
      </label>
      <label data-field>
        Max Retries
        <input type="number" :value="String(config.max_retries || 3)" min="1" max="10" @input="updateConfig('max_retries', parseInt(val($event)) || 3)">
      </label>
    </template>

    <!-- HTTP Callback: config -->
    <template v-if="node.type === 'http_callback'">
      <label data-field>
        URL
        <input :value="config.url || ''" placeholder="https://api.example.com/ivr" @input="updateConfig('url', val($event))">
      </label>
      <label data-field>
        Method
        <select :value="config.method || 'GET'" @change="updateConfig('method', val($event))">
          <option value="GET">GET</option>
          <option value="POST">POST</option>
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
          <input class="flex-1 min-w-0" :value="String(key)" placeholder="Key" :aria-label="$t('flowBuilder.keyPlaceholder')" @input="updateHeaderKey(String(key), val($event))">
          <input class="flex-1 min-w-0" :value="String(v)" placeholder="Value" :aria-label="$t('flowBuilder.valuePlaceholder')" @input="updateHeaderValue(String(key), val($event))">
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
        Body Template
        <textarea
          :value="config.body_template || ''"
          placeholder='{"phone": "{{caller_phone}}"}'
          rows="3"
          @input="updateConfig('body_template', val($event))"
        />
      </label>
      <label data-field>
        Timeout (seconds)
        <input type="number" :value="String(config.timeout_seconds || 10)" min="1" max="30" @input="updateConfig('timeout_seconds', parseInt(val($event)) || 10)">
      </label>
      <label data-field>
        Store Response As (variable name)
        <input :value="config.response_store_as || ''" placeholder="e.g. api_response" @input="updateConfig('response_store_as', val($event))">
      </label>
    </template>

    <!-- Transfer: team selector -->
    <template v-if="node.type === 'transfer'">
      <label data-field>
        Team
        <select :value="config.team_id || 'none'" @change="updateConfig('team_id', val($event) === 'none' ? '' : val($event))">
          <option value="none">Select team...</option>
          <option v-for="team in teamsStore.teams" :key="team.id" :value="team.id">
            {{ team.name }}
          </option>
        </select>
      </label>

      <!-- HTTP Callbacks per lifecycle event -->
      <div class="vstack gap-2 mb-4">
        <strong>HTTP Callbacks</strong>
        <small class="text-light">Configure API calls to your CRM at each transfer stage.</small>

        <article v-for="event in callbackEvents" :key="event" class="card p-4 vstack gap-2">
          <button
            type="button"
            class="ghost small w-100 justify-between"
            :aria-expanded="getCallbackConfig(event)._expanded ? 'true' : 'false'"
            @click="updateCallbackField(event, '_expanded', !getCallbackConfig(event)._expanded)"
          >
            <span>{{ callbackLabels[event] }}</span>
            <span v-if="getCallbackConfig(event).url" class="badge" data-variant="success">Configured</span>
          </button>

          <div v-if="getCallbackConfig(event)._expanded">
            <label data-field>
              URL
              <input
                :value="getCallbackConfig(event).url || ''"
                placeholder="https://crm.example.com/api/call"
                @input="updateCallbackField(event, 'url', val($event))"
              >
            </label>
            <label data-field>
              Method
              <select :value="getCallbackConfig(event).method || 'POST'" @change="updateCallbackField(event, 'method', val($event))">
                <option value="GET">GET</option>
                <option value="POST">POST</option>
              </select>
            </label>
            <div class="vstack gap-2 mb-4">
              <div class="hstack justify-between">
                <span>Headers</span>
                <button type="button" class="outline small" @click="addCallbackHeader(event)">
                  <Plus /> Add
                </button>
              </div>
              <div v-for="(v, key) in (getCallbackConfig(event).headers || {})" :key="String(key)" class="hstack gap-1">
                <input
                  class="flex-1 min-w-0"
                  :value="String(key)"
                  placeholder="Key"
                  :aria-label="$t('flowBuilder.keyPlaceholder')"
                  @input="updateCallbackHeaderKey(event, String(key), val($event))"
                >
                <input
                  class="flex-1 min-w-0"
                  :value="String(v)"
                  placeholder="Value"
                  :aria-label="$t('flowBuilder.valuePlaceholder')"
                  @input="updateCallbackHeaderValue(event, String(key), val($event))"
                >
                <button
                  type="button"
                  class="ghost small icon"
                  data-variant="danger"
                  :aria-label="$t('common.delete')"
                  :title="$t('common.delete')"
                  @click="removeCallbackHeader(event, String(key))"
                >
                  <Trash2 />
                </button>
              </div>
            </div>
            <label v-if="(getCallbackConfig(event).method || 'POST') === 'POST'" data-field>
              Body Template
              <textarea
                :value="getCallbackConfig(event).body_template || ''"
                :placeholder='`{"phone": "{{caller_phone}}", "transfer_id": "{{transfer_id}}"}`'
                rows="3"
                @input="updateCallbackField(event, 'body_template', val($event))"
              />
            </label>
          </div>
        </article>
      </div>

      <!-- Available variables reference -->
      <article class="card p-4 vstack gap-2 mb-4">
        <button
          type="button"
          class="ghost small w-100 justify-between"
          :aria-expanded="config._vars_expanded ? 'true' : 'false'"
          @click="updateConfig('_vars_expanded', !config._vars_expanded)"
        >
          <span>Available Variables</span>
        </button>
        <div v-if="config._vars_expanded" class="vstack gap-2">
          <div class="hstack gap-1 flex-wrap">
            <code
              v-for="v in transferVariables"
              :key="v"
              :title="v.includes('*') ? 'Available on connect/complete only' : ''"
            >{{ v.replace(' *', '').replace(' **', '') }}</code>
          </div>
          <small class="text-light">* on_connect/on_complete only &nbsp; ** on_complete only</small>
        </div>
      </article>
    </template>

    <!-- Goto Flow: flow selector -->
    <template v-if="node.type === 'goto_flow'">
      <label data-field>
        Target Flow
        <select :value="config.flow_id || 'none'" @change="updateConfig('flow_id', val($event) === 'none' ? '' : val($event))">
          <option value="none">Select flow...</option>
          <option v-for="flow in gotoFlowTargets" :key="flow.id" :value="flow.id">
            {{ flow.name }}
          </option>
        </select>
      </label>
    </template>

    <!-- Timing: schedule -->
    <template v-if="node.type === 'timing'">
      <fieldset class="vstack gap-2 mb-4">
        <legend>Schedule</legend>
        <div v-for="(entry, idx) in schedule" :key="idx" class="hstack gap-2">
          <label class="hstack gap-2 shrink-0">
            <input
              type="checkbox"
              role="switch"
              :checked="entry.enabled"
              @change="updateScheduleEntry(Number(idx), 'enabled', checked($event))"
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
      </fieldset>
    </template>
  </div>
</template>
