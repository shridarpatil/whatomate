<script setup lang="ts">
import { ref, computed, watch, nextTick, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { cannedResponsesService, type CannedResponse } from '@/services/api'
import { MessageSquareText } from '@/lib/icons'

const props = defineProps<{
  externalOpen?: boolean
  externalSearch?: string
}>()

const emit = defineEmits<{
  (e: 'select', response: CannedResponse): void
  (e: 'close'): void
}>()

const { t } = useI18n()

const popoverId = `${useId()}-canned`
const popover = ref<HTMLElement | null>(null)
const search = ref<HTMLInputElement | null>(null)
const isLoading = ref(false)
const searchQuery = ref('')
const responses = ref<CannedResponse[]>([])
// Opened by the composer's "/" command: keep focus in the composer.
let openedExternally = false

function isShown() {
  return !!popover.value?.matches(':popover-open')
}

// The composer opens/closes the picker while the agent types "/…".
watch(() => props.externalOpen, (open) => {
  if (open && !isShown()) {
    openedExternally = true
    popover.value?.showPopover()
  } else if (!open && isShown()) {
    popover.value?.hidePopover()
  }
})

// Sync external search
watch(() => props.externalSearch, (val) => {
  if (val !== undefined) {
    searchQuery.value = val
  }
})

async function onToggle(e: Event) {
  if ((e as ToggleEvent).newState === 'open') {
    if (!openedExternally) nextTick(() => search.value?.focus())
    openedExternally = false
    if (responses.value.length === 0) await fetchResponses()
  } else {
    emit('close')
  }
}

async function fetchResponses() {
  isLoading.value = true
  try {
    const response = await cannedResponsesService.list({ active_only: 'true' })
    const data = (response.data as any).data || response.data
    responses.value = data.canned_responses || []
  } catch (error) {
    console.error('Failed to fetch canned responses:', error)
  } finally {
    isLoading.value = false
  }
}

// Expose fetchResponses for preloading
defineExpose({ fetchResponses })

const filteredResponses = computed(() => {
  if (!searchQuery.value) return responses.value
  const query = searchQuery.value.toLowerCase()
  return responses.value.filter(r =>
    r.name.toLowerCase().includes(query) ||
    r.content.toLowerCase().includes(query) ||
    (r.shortcut && r.shortcut.toLowerCase().includes(query))
  )
})

// Group by category
const groupedResponses = computed(() => {
  const groups: Record<string, CannedResponse[]> = {}
  for (const response of filteredResponses.value) {
    const category = response.category || 'general'
    if (!groups[category]) {
      groups[category] = []
    }
    groups[category].push(response)
  }
  return groups
})

const categoryLabels: Record<string, string> = {
  greeting: 'Greetings',
  support: 'Support',
  sales: 'Sales',
  closing: 'Closing',
  general: 'General'
}

function getCategoryLabel(category: string): string {
  return categoryLabels[category] || category
}

function selectResponse(response: CannedResponse) {
  emit('select', response)
  popover.value?.hidePopover()
  searchQuery.value = ''
}
</script>

<template>
  <ot-dropdown>
    <button
      id="canned-response-picker-button"
      type="button"
      class="ghost icon"
      :popovertarget="popoverId"
      :title="t('chat.cannedResponses')"
      :aria-label="t('chat.cannedResponses')"
    >
      <MessageSquareText :size="20" />
    </button>
    <div :id="popoverId" ref="popover" popover class="picker" data-picker="canned-responses" @toggle="onToggle">
      <div class="vstack gap-2 p-4">
        <input
          ref="search"
          v-model="searchQuery"
          type="search"
          :placeholder="t('cannedResponses.searchResponses') + '...'"
          :aria-label="t('cannedResponses.searchResponses')"
        />

        <div class="scroll-y picker-list vstack gap-1">
          <div v-if="isLoading" class="flex justify-center p-4">
            <span role="status" aria-busy="true" :aria-label="t('common.loading')" />
          </div>

          <p v-else-if="filteredResponses.length === 0" class="align-center text-light p-4">
            <small>{{ t('cannedResponses.noResponsesFound') }}</small>
          </p>

          <template v-else>
            <template v-for="(items, category) in groupedResponses" :key="category">
              <small class="text-light">{{ getCategoryLabel(category) }}</small>
              <button
                v-for="response in items"
                :key="response.id"
                type="button"
                class="ghost w-100 shrink-0"
                data-picker-item
                @click="selectResponse(response)"
              >
                <span class="vstack gap-1 w-100 min-w-0 align-left">
                  <span class="flex justify-between gap-2">
                    <strong class="truncate">{{ response.name }}</strong>
                    <small v-if="response.shortcut" class="text-light"><code>/{{ response.shortcut }}</code></small>
                  </span>
                  <small class="text-light truncate">{{ response.content }}</small>
                </span>
              </button>
            </template>
          </template>
        </div>
      </div>
    </div>
  </ot-dropdown>
</template>
