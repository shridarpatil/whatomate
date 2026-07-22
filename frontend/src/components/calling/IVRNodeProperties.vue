<script setup lang="ts">
import { computed, ref } from 'vue'
import type { IVRNode, IVRNodeType } from '@/services/api'
import { ivrFlowsService } from '@/services/api'
import { useCallingStore } from '@/stores/calling'
import { useTeamsStore } from '@/stores/teams'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Trash2, Plus, Upload, Play, Pause, X, Loader2, Type } from 'lucide-vue-next'
import { toast } from 'vue-sonner'

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
      updateConfig('audio_file', filename)
      updateConfig('greeting_text', undefined)
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
</script>

<template>
  <div class="space-y-4 p-4">
    <div class="flex items-center justify-between">
      <h3 class="font-semibold text-sm capitalize">{{ node.type.replace('_', ' ') }}</h3>
      <Button variant="ghost" size="icon" class="h-7 w-7" @click="emit('delete')">
        <Trash2 class="h-3.5 w-3.5 text-destructive" />
      </Button>
    </div>

    <!-- Label -->
    <div class="space-y-1.5">
      <Label class="text-xs">Label</Label>
      <Input :model-value="node.label" @update:model-value="updateLabel" class="h-8 text-sm" />
    </div>

    <!-- Audio Section (greeting, menu, gather, hangup) -->
    <div v-if="hasAudio" class="space-y-1.5">
      <Label class="text-xs">Audio</Label>
      <Tabs :default-value="greetingTab">
        <TabsList class="h-8">
          <TabsTrigger value="audio" class="text-xs h-7 px-2">
            <Upload class="h-3 w-3 mr-1" /> Upload
          </TabsTrigger>
          <TabsTrigger value="text" class="text-xs h-7 px-2">
            <Type class="h-3 w-3 mr-1" /> TTS
          </TabsTrigger>
        </TabsList>
        <TabsContent value="audio" class="mt-2">
          <div class="flex items-center gap-2">
            <div v-if="config.audio_file" class="flex items-center gap-1 flex-1 min-w-0 px-2 py-1 border rounded-md bg-muted/50">
              <Button variant="ghost" size="icon" class="h-5 w-5 shrink-0" @click="togglePlayback">
                <Pause v-if="isPlaying" class="h-3 w-3" />
                <Play v-else class="h-3 w-3" />
              </Button>
              <span class="text-xs truncate">{{ config.audio_file }}</span>
              <Button variant="ghost" size="icon" class="h-5 w-5 shrink-0 ml-auto" @click="removeAudio">
                <X class="h-3 w-3 text-destructive" />
              </Button>
            </div>
            <Button v-else variant="outline" size="sm" class="h-7 text-xs w-full" @click="triggerFileUpload" :disabled="isUploading">
              <Loader2 v-if="isUploading" class="h-3 w-3 mr-1 animate-spin" />
              <Upload v-else class="h-3 w-3 mr-1" />
              Upload Audio
            </Button>
            <input ref="audioFileInput" type="file" accept="audio/*" class="hidden" @change="handleFileSelect" />
          </div>
        </TabsContent>
        <TabsContent value="text" class="mt-2">
          <Textarea
            :model-value="config.greeting_text || ''"
            @update:model-value="(v: string) => updateConfig('greeting_text', v)"
            placeholder="Enter text for TTS..."
            class="min-h-[60px] text-xs resize-none"
            :maxlength="500"
          />
        </TabsContent>
      </Tabs>
    </div>

    <!-- Greeting: interruptible -->
    <div v-if="node.type === 'greeting'" class="flex items-center gap-2">
      <Switch :checked="!!config.interruptible" @update:checked="(v: boolean) => updateConfig('interruptible', v)" />
      <Label class="text-xs">Interruptible by DTMF</Label>
    </div>

    <!-- Menu: options -->
    <template v-if="node.type === 'menu'">
      <div class="space-y-1.5">
        <Label class="text-xs">Timeout (seconds)</Label>
        <Input type="number" :model-value="String(config.timeout_seconds || 10)" @update:model-value="(v: string) => updateConfig('timeout_seconds', parseInt(v) || 10)" class="h-8 text-sm" min="1" max="60" />
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Max Retries</Label>
        <Input type="number" :model-value="String(config.max_retries || 3)" @update:model-value="(v: string) => updateConfig('max_retries', parseInt(v) || 3)" class="h-8 text-sm" min="1" max="10" />
      </div>
      <div class="space-y-1.5">
        <div class="flex items-center justify-between">
          <Label class="text-xs">Menu Options</Label>
          <Button variant="outline" size="sm" class="h-6 text-xs" @click="addMenuOption">
            <Plus class="h-3 w-3 mr-1" /> Add
          </Button>
        </div>
        <div v-for="(opt, digit) in (config.options || {})" :key="String(digit)" class="flex items-center gap-1.5">
          <span class="font-mono text-xs font-bold w-5 text-center">{{ digit }}</span>
          <Input :model-value="(opt as any)?.label || ''" @update:model-value="(v: string) => updateMenuOption(String(digit), 'label', v)" placeholder="Label" class="h-7 text-xs flex-1" />
          <Button variant="ghost" size="icon" class="h-6 w-6" @click="removeMenuOption(String(digit))">
            <Trash2 class="h-3 w-3 text-destructive" />
          </Button>
        </div>
      </div>
    </template>

    <!-- Gather: config -->
    <template v-if="node.type === 'gather'">
      <div class="space-y-1.5">
        <Label class="text-xs">Max Digits</Label>
        <Input type="number" :model-value="String(config.max_digits || 10)" @update:model-value="(v: string) => updateConfig('max_digits', parseInt(v) || 10)" class="h-8 text-sm" min="1" max="20" />
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Terminator</Label>
        <Input :model-value="config.terminator || '#'" @update:model-value="(v: string) => updateConfig('terminator', v)" class="h-8 text-sm" />
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Store As (variable name)</Label>
        <Input :model-value="config.store_as || ''" @update:model-value="(v: string) => updateConfig('store_as', v)" placeholder="e.g. account_number" class="h-8 text-sm" />
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Timeout (seconds)</Label>
        <Input type="number" :model-value="String(config.timeout_seconds || 10)" @update:model-value="(v: string) => updateConfig('timeout_seconds', parseInt(v) || 10)" class="h-8 text-sm" min="1" max="60" />
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Max Retries</Label>
        <Input type="number" :model-value="String(config.max_retries || 3)" @update:model-value="(v: string) => updateConfig('max_retries', parseInt(v) || 3)" class="h-8 text-sm" min="1" max="10" />
      </div>
    </template>

    <!-- HTTP Callback: config -->
    <template v-if="node.type === 'http_callback'">
      <div class="space-y-1.5">
        <Label class="text-xs">URL</Label>
        <Input :model-value="config.url || ''" @update:model-value="(v: string) => updateConfig('url', v)" placeholder="https://api.example.com/ivr" class="h-8 text-xs font-mono" />
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Method</Label>
        <Select :model-value="config.method || 'GET'" @update:model-value="(v: any) => updateConfig('method', v)">
          <SelectTrigger class="h-8 text-sm"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="GET">GET</SelectItem>
            <SelectItem value="POST">POST</SelectItem>
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
        <Label class="text-xs">Body Template</Label>
        <Textarea :model-value="config.body_template || ''" @update:model-value="(v: string) => updateConfig('body_template', v)" placeholder='{"phone": "{{caller_phone}}"}' class="min-h-[60px] text-xs font-mono resize-none" />
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Timeout (seconds)</Label>
        <Input type="number" :model-value="String(config.timeout_seconds || 10)" @update:model-value="(v: string) => updateConfig('timeout_seconds', parseInt(v) || 10)" class="h-8 text-sm" min="1" max="30" />
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs">Store Response As (variable name)</Label>
        <Input :model-value="config.response_store_as || ''" @update:model-value="(v: string) => updateConfig('response_store_as', v)" placeholder="e.g. api_response" class="h-8 text-sm" />
      </div>
    </template>

    <!-- Transfer: team selector -->
    <template v-if="node.type === 'transfer'">
      <div class="space-y-1.5">
        <Label class="text-xs">Team</Label>
        <Select :model-value="config.team_id || 'none'" @update:model-value="(v: any) => updateConfig('team_id', v === 'none' ? '' : v)">
          <SelectTrigger class="h-8 text-sm"><SelectValue placeholder="Select team" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="none">Select team...</SelectItem>
            <SelectItem v-for="team in teamsStore.teams" :key="team.id" :value="team.id">
              {{ team.name }}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>

      <!-- HTTP Callbacks per lifecycle event -->
      <div class="space-y-2 mt-3">
        <Label class="text-xs font-medium">HTTP Callbacks</Label>
        <p class="text-[10px] text-muted-foreground">Configure API calls to your CRM at each transfer stage.</p>

        <div v-for="event in callbackEvents" :key="event" class="border rounded-md">
          <button class="w-full flex items-center justify-between px-3 py-1.5 text-xs font-medium hover:bg-muted/50" @click="updateCallbackField(event, '_expanded', !getCallbackConfig(event)._expanded)">
            <span>{{ callbackLabels[event] }}</span>
            <span v-if="getCallbackConfig(event).url" class="text-[10px] text-emerald-500">Configured</span>
          </button>

          <div v-if="getCallbackConfig(event)._expanded" class="px-3 pb-3 space-y-1.5 border-t">
            <div class="space-y-1 pt-2">
              <Label class="text-[10px]">URL</Label>
              <Input :model-value="getCallbackConfig(event).url || ''" @update:model-value="(v: string) => updateCallbackField(event, 'url', v)" placeholder="https://crm.example.com/api/call" class="h-7 text-xs font-mono" />
            </div>
            <div class="space-y-1">
              <Label class="text-[10px]">Method</Label>
              <Select :model-value="getCallbackConfig(event).method || 'POST'" @update:model-value="(v: any) => updateCallbackField(event, 'method', v)">
                <SelectTrigger class="h-7 text-xs"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="GET">GET</SelectItem>
                  <SelectItem value="POST">POST</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div class="space-y-1">
              <div class="flex items-center justify-between">
                <Label class="text-[10px]">Headers</Label>
                <Button variant="outline" size="sm" class="h-5 text-[10px] px-1.5" @click="addCallbackHeader(event)">
                  <Plus class="h-2.5 w-2.5 mr-0.5" /> Add
                </Button>
              </div>
              <div v-for="(val, key) in (getCallbackConfig(event).headers || {})" :key="String(key)" class="flex items-center gap-1">
                <Input :model-value="String(key)" @update:model-value="(v: string) => updateCallbackHeaderKey(event, String(key), v)" placeholder="Key" class="h-6 text-[10px] flex-1" />
                <Input :model-value="String(val)" @update:model-value="(v: string) => updateCallbackHeaderValue(event, String(key), v)" placeholder="Value" class="h-6 text-[10px] flex-1" />
                <Button variant="ghost" size="icon" class="h-5 w-5" @click="removeCallbackHeader(event, String(key))">
                  <Trash2 class="h-2.5 w-2.5 text-destructive" />
                </Button>
              </div>
            </div>
            <div v-if="(getCallbackConfig(event).method || 'POST') === 'POST'" class="space-y-1">
              <Label class="text-[10px]">Body Template</Label>
              <Textarea :model-value="getCallbackConfig(event).body_template || ''" @update:model-value="(v: string) => updateCallbackField(event, 'body_template', v)" :placeholder='`{"phone": "{{caller_phone}}", "transfer_id": "{{transfer_id}}"}`' class="min-h-[50px] text-[10px] font-mono resize-none" />
            </div>
          </div>
        </div>
      </div>

      <!-- Available variables reference -->
      <div class="border rounded-md mt-2">
        <button class="w-full flex items-center justify-between px-3 py-1.5 text-xs font-medium hover:bg-muted/50" @click="updateConfig('_vars_expanded', !config._vars_expanded)">
          <span>Available Variables</span>
        </button>
        <div v-if="config._vars_expanded" class="px-3 pb-2 border-t">
          <div class="flex flex-wrap gap-1 pt-2">
            <code v-for="v in ['caller_phone', 'contact_id', 'call_log_id', 'transfer_id', 'team_id', 'whatsapp_account', 'status', 'transferred_at', 'ivr_path', 'agent_id *', 'agent_email *', 'agent_name *', 'hold_duration **', 'talk_duration **']" :key="v" class="bg-muted px-1.5 py-0.5 rounded text-[10px] cursor-pointer hover:bg-muted/80" :title="v.includes('*') ? 'Available on connect/complete only' : ''">{{ v.replace(' *', '').replace(' **', '') }}</code>
          </div>
          <p class="text-[9px] text-muted-foreground mt-1.5">* on_connect/on_complete only &nbsp; ** on_complete only</p>
        </div>
      </div>
    </template>

    <!-- Goto Flow: flow selector -->
    <template v-if="node.type === 'goto_flow'">
      <div class="space-y-1.5">
        <Label class="text-xs">Target Flow</Label>
        <Select :model-value="config.flow_id || 'none'" @update:model-value="(v: any) => updateConfig('flow_id', v === 'none' ? '' : v)">
          <SelectTrigger class="h-8 text-sm"><SelectValue placeholder="Select flow" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="none">Select flow...</SelectItem>
            <SelectItem v-for="flow in gotoFlowTargets" :key="flow.id" :value="flow.id">
              {{ flow.name }}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
    </template>

    <!-- Timing: schedule -->
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
      </div>
    </template>
  </div>
</template>
