<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import type { MockApiResponse, FlowStep } from '@/types/flow-preview'
import Modal from '@/components/oat/Modal.vue'

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
  <Modal
    :open="open"
    :title="$t('flowBuilder.configureApiMock')"
    description="Configure the mock response for this API step. The response will be used to simulate the API call."
    @update:open="(v) => { if (!v) handleClose() }"
  >
    <form id="api-mock-form" class="vstack gap-4" @submit.prevent="handleSubmit">
      <!-- API Info -->
      <article v-if="step" class="card p-4 vstack gap-1">
        <strong>{{ step.step_name }}</strong>
        <small class="text-light"><code>{{ step.api_config?.method || 'GET' }} {{ step.api_config?.url || 'N/A' }}</code></small>
      </article>

      <!-- Response Mapping Info -->
      <div v-if="responseMappingInfo.length > 0" class="vstack gap-1">
        <small class="text-light">Variables to extract:</small>
        <div class="hstack gap-2 flex-wrap">
          <code v-for="mapping in responseMappingInfo" :key="mapping.varName">
            {{ mapping.varName }} ← {{ mapping.path }}
          </code>
        </div>
      </div>

      <div class="row">
        <label data-field class="col-6">
          Status Code
          <select v-model="statusCode">
            <option value="200">200 OK</option>
            <option value="201">201 Created</option>
            <option value="400">400 Bad Request</option>
            <option value="401">401 Unauthorized</option>
            <option value="404">404 Not Found</option>
            <option value="500">500 Server Error</option>
          </select>
        </label>

        <label data-field class="col-6">
          Delay (ms)
          <input v-model="delay" type="number" min="0" max="5000" step="100">
        </label>
      </div>

      <!-- Response Body -->
      <label data-field>
        Response Body (JSON)
        <textarea
          v-model="responseBody"
          rows="8"
          placeholder='{"key": "value"}'
          :aria-invalid="parseError ? 'true' : undefined"
          aria-describedby="api-mock-json-error"
        />
        <div id="api-mock-json-error" class="error" role="status">{{ parseError }}</div>
      </label>
    </form>

    <template #footer>
      <button type="button" class="outline" @click="handleClose">
        Cancel
      </button>
      <button type="submit" form="api-mock-form" :disabled="!!parseError || !parsedResponse">
        Use Mock Response
      </button>
    </template>
  </Modal>
</template>
