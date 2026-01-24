<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import type { MockApiResponse, FlowStep } from '@/types/flow-preview'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Globe, AlertCircle } from 'lucide-vue-next'

const props = defineProps<{
  open: boolean
  step: FlowStep | null
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  submit: [response: MockApiResponse | null]
}>()

const statusCode = ref('200')
const responseBody = ref('')
const delay = ref('100')
const parseError = ref('')

watch(() => props.open, (isOpen) => {
  if (isOpen && props.step) {
    // Reset form with sample data
    statusCode.value = '200'
    delay.value = '100'
    parseError.value = ''

    // Generate sample response based on response mapping
    if (props.step.api_config?.response_mapping) {
      const sample: Record<string, any> = {}
      for (const [varName, path] of Object.entries(props.step.api_config.response_mapping)) {
        // Build nested structure from path
        const parts = path.split('.')
        let current = sample

        for (let i = 0; i < parts.length - 1; i++) {
          const part = parts[i]
          if (!current[part]) {
            current[part] = {}
          }
          current = current[part]
        }

        current[parts[parts.length - 1]] = `sample_${varName}_value`
      }
      responseBody.value = JSON.stringify(sample, null, 2)
    } else {
      responseBody.value = '{\n  "success": true,\n  "data": {}\n}'
    }
  }
})

// Validate JSON and set error message
watch(responseBody, (body) => {
  try {
    JSON.parse(body)
    parseError.value = ''
  } catch {
    parseError.value = 'Invalid JSON'
  }
}, { immediate: true })

const parsedResponse = computed(() => {
  try {
    return JSON.parse(responseBody.value)
  } catch {
    return null
  }
})

const responseMappingInfo = computed(() => {
  if (!props.step?.api_config?.response_mapping) return []

  return Object.entries(props.step.api_config.response_mapping).map(([varName, path]) => ({
    varName,
    path
  }))
})

function handleClose() {
  emit('update:open', false)
  emit('submit', null)
}

function handleSubmit() {
  if (!props.step || !parsedResponse.value) return

  const mock: MockApiResponse = {
    stepName: props.step.step_name,
    url: props.step.api_config?.url || '',
    statusCode: parseInt(statusCode.value, 10),
    responseBody: parsedResponse.value,
    delay: parseInt(delay.value, 10)
  }

  emit('submit', mock)
  emit('update:open', false)
}
</script>

<template>
  <Dialog :open="open" @update:open="handleClose">
    <DialogContent class="sm:max-w-[500px]">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2">
          <Globe class="h-5 w-5 text-blue-500" />
          Configure API Mock
        </DialogTitle>
        <DialogDescription>
          Configure the mock response for this API step. The response will be used to simulate the API call.
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-4">
        <!-- API Info -->
        <div v-if="step" class="p-3 bg-gray-50 dark:bg-gray-800 rounded-lg text-sm">
          <div class="font-medium text-gray-700 dark:text-gray-300 mb-1">
            {{ step.step_name }}
          </div>
          <div class="text-gray-500 dark:text-gray-400 font-mono text-xs">
            {{ step.api_config?.method || 'GET' }} {{ step.api_config?.url || 'N/A' }}
          </div>
        </div>

        <!-- Response Mapping Info -->
        <div v-if="responseMappingInfo.length > 0" class="text-xs">
          <Label class="text-gray-500">Variables to extract:</Label>
          <div class="mt-1 flex flex-wrap gap-2">
            <span
              v-for="mapping in responseMappingInfo"
              :key="mapping.varName"
              class="px-2 py-0.5 bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-400 rounded font-mono"
            >
              {{ mapping.varName }} ← {{ mapping.path }}
            </span>
          </div>
        </div>

        <!-- Status Code -->
        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-2">
            <Label>Status Code</Label>
            <Select v-model="statusCode">
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="200">200 OK</SelectItem>
                <SelectItem value="201">201 Created</SelectItem>
                <SelectItem value="400">400 Bad Request</SelectItem>
                <SelectItem value="401">401 Unauthorized</SelectItem>
                <SelectItem value="404">404 Not Found</SelectItem>
                <SelectItem value="500">500 Server Error</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div class="space-y-2">
            <Label>Delay (ms)</Label>
            <Input
              v-model="delay"
              type="number"
              min="0"
              max="5000"
              step="100"
            />
          </div>
        </div>

        <!-- Response Body -->
        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <Label>Response Body (JSON)</Label>
            <span
              v-if="parseError"
              class="text-xs text-red-500 flex items-center gap-1"
            >
              <AlertCircle class="h-3 w-3" />
              {{ parseError }}
            </span>
          </div>
          <Textarea
            v-model="responseBody"
            :class="'font-mono text-sm min-h-[150px]' + (parseError ? ' border-red-500' : '')"
            placeholder='{"key": "value"}'
          />
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="handleClose">
          Cancel
        </Button>
        <Button
          :disabled="!!parseError || !parsedResponse"
          @click="handleSubmit"
        >
          Use Mock Response
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
