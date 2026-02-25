<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { IVRMenu, IVRMenuOption } from '@/services/api'
import { ivrFlowsService } from '@/services/api'
import { useCallingStore } from '@/stores/calling'
import { useTeamsStore } from '@/stores/teams'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { Plus, Trash2, Upload, X, Play, Pause, Loader2, Type } from 'lucide-vue-next'
import { useToast } from '@/components/ui/toast'

const props = defineProps<{
  modelValue: IVRMenu
  depth?: number
  /** ID of the flow being edited, to exclude from goto_flow targets */
  currentFlowId?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: IVRMenu]
}>()

const { t } = useI18n()
const { toast } = useToast()
const currentDepth = computed(() => props.depth ?? 0)

// Audio upload state
const audioFileInput = ref<HTMLInputElement | null>(null)
const isUploading = ref(false)
const isPlaying = ref(false)
const audioElement = ref<HTMLAudioElement | null>(null)

const menu = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

// Greeting mode: 'audio' for uploaded files, 'text' for TTS
const greetingTab = computed(() =>
  menu.value.greeting_text ? 'text' : 'audio'
)

const optionEntries = computed(() => {
  const opts = menu.value.options || {}
  return Object.entries(opts).sort(([a], [b]) => {
    // Sort: digits first (0-9), then * and #
    const order = '1234567890*#'
    return order.indexOf(a) - order.indexOf(b)
  })
})

const availableDigits = computed(() => {
  const used = new Set(Object.keys(menu.value.options || {}))
  return ['1', '2', '3', '4', '5', '6', '7', '8', '9', '0', '*', '#'].filter(d => !used.has(d))
})

const callingStore = useCallingStore()
const teamsStore = useTeamsStore()

// Load teams if not already fetched
if (teamsStore.teams.length === 0) {
  teamsStore.fetchTeams()
}

// Other IVR flows available as goto targets (exclude current flow)
const gotoFlowTargets = computed(() =>
  callingStore.ivrFlows.filter(f => f.id !== props.currentFlowId)
)

const actionOptions = [
  { value: 'transfer', label: 'Transfer to team' },
  { value: 'submenu', label: 'Go to submenu' },
  { value: 'goto_flow', label: 'Go to another IVR flow' },
  { value: 'parent', label: 'Go back (parent)' },
  { value: 'repeat', label: 'Repeat menu' },
  { value: 'hangup', label: 'Hang up' }
]

function onGreetingTabChange(tab: string | number) {
  if (tab === 'text') {
    // Switching to text mode: clear uploaded audio
    stopAudio()
    emit('update:modelValue', { ...menu.value, greeting: '', greeting_text: menu.value.greeting_text || '' })
  } else {
    // Switching to audio mode: clear greeting text
    emit('update:modelValue', { ...menu.value, greeting_text: undefined })
  }
}

function updateGreetingText(text: string) {
  emit('update:modelValue', { ...menu.value, greeting_text: text, greeting: '' })
}

function triggerFileUpload() {
  audioFileInput.value?.click()
}

async function handleFileSelect(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return

  // Check size (5MB)
  if (file.size > 5 * 1024 * 1024) {
    toast({ title: t('calling.audioTooLarge'), variant: 'destructive' })
    input.value = ''
    return
  }

  isUploading.value = true
  try {
    const res = await ivrFlowsService.uploadAudio(file)
    const filename = res.data?.data?.filename
    if (filename) {
      emit('update:modelValue', { ...menu.value, greeting: filename, greeting_text: undefined })
      toast({ title: t('calling.audioUploaded') })
    }
  } catch {
    toast({ title: t('calling.audioUploadFailed'), variant: 'destructive' })
  } finally {
    isUploading.value = false
    input.value = ''
  }
}

function removeAudio() {
  stopAudio()
  emit('update:modelValue', { ...menu.value, greeting: '' })
}

function togglePlayback() {
  if (isPlaying.value) {
    stopAudio()
  } else {
    playAudio()
  }
}

function playAudio() {
  if (!menu.value.greeting) return
  stopAudio()
  const audio = new Audio(ivrFlowsService.getAudioUrl(menu.value.greeting))
  audio.onended = () => { isPlaying.value = false }
  audio.onpause = () => { isPlaying.value = false }
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

function updateTimeout(val: string) {
  const num = parseInt(val) || 10
  emit('update:modelValue', { ...menu.value, timeout_seconds: num })
}

function updateMaxRetries(val: string) {
  const num = parseInt(val) || 3
  emit('update:modelValue', { ...menu.value, max_retries: num })
}

function addOption() {
  if (availableDigits.value.length === 0) return
  const digit = availableDigits.value[0]
  const newOptions = { ...menu.value.options }
  newOptions[digit] = { label: '', action: 'transfer' }
  emit('update:modelValue', { ...menu.value, options: newOptions })
}

function removeOption(digit: string) {
  const newOptions = { ...menu.value.options }
  delete newOptions[digit]
  emit('update:modelValue', { ...menu.value, options: newOptions })
}

function updateOptionDigit(oldDigit: string, newDigit: string) {
  if (oldDigit === newDigit) return
  const newOptions = { ...menu.value.options }
  newOptions[newDigit] = newOptions[oldDigit]
  delete newOptions[oldDigit]
  emit('update:modelValue', { ...menu.value, options: newOptions })
}

function updateOption(digit: string, field: keyof IVRMenuOption, value: any) {
  const newOptions = { ...menu.value.options }
  const opt = { ...newOptions[digit] }

  if (field === 'action' && value === 'submenu' && !opt.menu) {
    opt.menu = { greeting: '', options: {}, timeout_seconds: 10, max_retries: 3 }
  }
  if (field === 'action' && value !== 'submenu') {
    delete opt.menu
  }

  ;(opt as any)[field] = value
  newOptions[digit] = opt
  emit('update:modelValue', { ...menu.value, options: newOptions })
}

function updateSubmenu(digit: string, submenu: IVRMenu) {
  const newOptions = { ...menu.value.options }
  newOptions[digit] = { ...newOptions[digit], menu: submenu }
  emit('update:modelValue', { ...menu.value, options: newOptions })
}
</script>

<template>
  <Card :class="currentDepth > 0 ? 'border-dashed' : ''">
    <CardHeader class="pb-3">
      <CardTitle class="text-sm flex items-center gap-2">
        <Badge variant="outline" v-if="currentDepth > 0">{{ t('calling.submenu') }}</Badge>
        <span v-else>{{ t('calling.rootMenu') }}</span>
      </CardTitle>
    </CardHeader>
    <CardContent class="space-y-4">
      <!-- Greeting: Upload Audio or Text to Speech -->
      <div class="space-y-2">
        <Label class="text-xs">{{ t('calling.greeting') }}</Label>
        <Tabs :default-value="greetingTab" @update:model-value="onGreetingTabChange">
          <TabsList class="h-8">
            <TabsTrigger value="audio" class="text-xs h-7 px-3">
              <Upload class="h-3 w-3 mr-1" />
              {{ t('calling.uploadAudioTab') }}
            </TabsTrigger>
            <TabsTrigger value="text" class="text-xs h-7 px-3">
              <Type class="h-3 w-3 mr-1" />
              {{ t('calling.textToSpeechTab') }}
            </TabsTrigger>
          </TabsList>

          <!-- Upload Audio tab -->
          <TabsContent value="audio" class="mt-2">
            <div class="flex items-center gap-2">
              <div v-if="menu.greeting && !menu.greeting_text" class="flex items-center gap-2 flex-1 min-w-0 px-3 py-1.5 border rounded-md bg-muted/50">
                <Button variant="ghost" size="icon" class="h-6 w-6 shrink-0" @click="togglePlayback">
                  <Pause v-if="isPlaying" class="h-3 w-3" />
                  <Play v-else class="h-3 w-3" />
                </Button>
                <span class="text-sm truncate">{{ menu.greeting }}</span>
                <Button variant="ghost" size="icon" class="h-6 w-6 shrink-0 ml-auto" @click="removeAudio">
                  <X class="h-3 w-3 text-destructive" />
                </Button>
              </div>
              <div v-else class="flex-1 text-sm text-muted-foreground px-3 py-1.5 border rounded-md border-dashed">
                {{ t('calling.greetingPlaceholder') }}
              </div>
              <Button variant="outline" size="sm" class="h-8 shrink-0" @click="triggerFileUpload" :disabled="isUploading">
                <Loader2 v-if="isUploading" class="h-3 w-3 mr-1 animate-spin" />
                <Upload v-else class="h-3 w-3 mr-1" />
                {{ t('calling.uploadAudio') }}
              </Button>
              <input
                ref="audioFileInput"
                type="file"
                accept="audio/*"
                class="hidden"
                @change="handleFileSelect"
              />
            </div>
          </TabsContent>

          <!-- Text to Speech tab -->
          <TabsContent value="text" class="mt-2">
            <div class="relative">
              <Textarea
                :model-value="menu.greeting_text || ''"
                @update:model-value="updateGreetingText"
                :placeholder="t('calling.greetingTextPlaceholder')"
                class="min-h-[80px] text-sm resize-none"
                :maxlength="500"
              />
              <span class="absolute bottom-2 right-2 text-xs text-muted-foreground">
                {{ (menu.greeting_text || '').length }}/500
              </span>
            </div>
          </TabsContent>
        </Tabs>
      </div>

      <!-- Timeout & Retries (root only) -->
      <div v-if="currentDepth === 0" class="grid grid-cols-2 gap-3">
        <div class="space-y-1">
          <Label class="text-xs">{{ t('calling.timeout') }}</Label>
          <Input
            type="number"
            :model-value="String(menu.timeout_seconds || 10)"
            @update:model-value="updateTimeout"
            min="1"
            max="60"
            class="text-sm"
          />
        </div>
        <div class="space-y-1">
          <Label class="text-xs">{{ t('calling.maxRetries') }}</Label>
          <Input
            type="number"
            :model-value="String(menu.max_retries || 3)"
            @update:model-value="updateMaxRetries"
            min="1"
            max="10"
            class="text-sm"
          />
        </div>
      </div>

      <!-- Options -->
      <div class="space-y-3">
        <div class="flex items-center justify-between">
          <Label class="text-xs">{{ t('calling.menuOptions') }}</Label>
          <Button
            variant="outline"
            size="sm"
            @click="addOption"
            :disabled="availableDigits.length === 0"
            class="h-7 text-xs"
          >
            <Plus class="h-3 w-3 mr-1" />
            {{ t('calling.addOption') }}
          </Button>
        </div>

        <div v-for="[digit, option] in optionEntries" :key="digit" class="space-y-2 p-3 border rounded-lg">
          <div class="flex items-center gap-2">
            <!-- Digit selector -->
            <Select :model-value="digit" @update:model-value="(v: any) => updateOptionDigit(digit, String(v))">
              <SelectTrigger class="w-20 h-8 text-sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem :value="digit">{{ digit }}</SelectItem>
                <SelectItem v-for="d in availableDigits" :key="d" :value="d">{{ d }}</SelectItem>
              </SelectContent>
            </Select>

            <!-- Label -->
            <Input
              :model-value="option.label"
              @update:model-value="(v: string) => updateOption(digit, 'label', v)"
              :placeholder="t('calling.optionLabel')"
              class="flex-1 h-8 text-sm"
            />

            <!-- Action -->
            <Select :model-value="option.action" @update:model-value="(v: any) => updateOption(digit, 'action', String(v))">
              <SelectTrigger class="w-40 h-8 text-sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="act in actionOptions" :key="act.value" :value="act.value">
                  {{ act.label }}
                </SelectItem>
              </SelectContent>
            </Select>

            <!-- Remove -->
            <Button variant="ghost" size="icon" class="h-8 w-8" @click="removeOption(digit)">
              <Trash2 class="h-3 w-3 text-destructive" />
            </Button>
          </div>

          <!-- Transfer target (team) -->
          <div v-if="option.action === 'transfer'" class="pl-8">
            <Select
              :model-value="option.target || 'none'"
              @update:model-value="(v: any) => updateOption(digit, 'target', String(v) === 'none' ? '' : String(v))"
            >
              <SelectTrigger class="h-8 text-sm">
                <SelectValue :placeholder="t('calling.transferTarget')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="none">{{ t('calling.transferTarget') }}</SelectItem>
                <SelectItem v-for="team in teamsStore.teams" :key="team.id" :value="team.id">
                  {{ team.name }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <!-- Goto flow target -->
          <div v-if="option.action === 'goto_flow'" class="pl-8">
            <Select
              :model-value="option.target || 'none'"
              @update:model-value="(v: any) => updateOption(digit, 'target', String(v) === 'none' ? '' : String(v))"
            >
              <SelectTrigger class="h-8 text-sm">
                <SelectValue :placeholder="t('calling.selectTargetFlow')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="none">{{ t('calling.selectTargetFlow') }}</SelectItem>
                <SelectItem v-for="flow in gotoFlowTargets" :key="flow.id" :value="flow.id">
                  {{ flow.name }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <!-- Submenu (recursive) -->
          <div v-if="option.action === 'submenu' && option.menu" class="pl-4 mt-2">
            <IVRMenuEditor
              :model-value="option.menu"
              @update:model-value="(v: IVRMenu) => updateSubmenu(digit, v)"
              :depth="currentDepth + 1"
              :current-flow-id="currentFlowId"
            />
          </div>
        </div>

        <p v-if="optionEntries.length === 0" class="text-sm text-muted-foreground text-center py-2">
          {{ t('calling.noOptions') }}
        </p>
      </div>
    </CardContent>
  </Card>
</template>
