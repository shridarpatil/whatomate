<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { cannedResponsesService, type CannedResponse } from '@/services/api'
import { MessageSquareText, Search, Loader2 } from 'lucide-vue-next'
import type { Contact } from '@/stores/contacts'

const props = defineProps<{
  contact: Contact | null
  externalOpen?: boolean
  externalSearch?: string
}>()

const emit = defineEmits<{
  (e: 'select', content: string): void
  (e: 'close'): void
}>()

const internalOpen = ref(false)
const isLoading = ref(false)
const searchQuery = ref('')
const responses = ref<CannedResponse[]>([])

// Sync external open state - use external if true, otherwise use internal
const isOpen = computed({
  get: () => props.externalOpen || internalOpen.value,
  set: (val) => {
    internalOpen.value = val
    if (!val) {
      emit('close')
    }
  }
})

// Sync external search
watch(() => props.externalSearch, (val) => {
  if (val !== undefined) {
    searchQuery.value = val
  }
})

// Fetch responses when popover opens
watch(isOpen, async (open) => {
  if (open && responses.value.length === 0) {
    await fetchResponses()
  }
})

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

function replacePlaceholders(content: string): string {
  if (!props.contact) return content

  return content
    .replace(/\{\{contact_name\}\}/gi, props.contact.profile_name || props.contact.name || 'there')
    .replace(/\{\{phone_number\}\}/gi, props.contact.phone_number || '')
}

function selectResponse(response: CannedResponse) {
  const content = replacePlaceholders(response.content)
  emit('select', content)
  // Track usage
  cannedResponsesService.use(response.id).catch(() => {})
  isOpen.value = false
  searchQuery.value = ''
}
</script>

<template>
  <Popover v-model:open="isOpen">
    <PopoverTrigger as-child>
      <Button type="button" variant="ghost" size="icon">
        <MessageSquareText class="h-5 w-5" />
      </Button>
    </PopoverTrigger>
    <PopoverContent side="top" align="start" class="w-80 p-0">
      <div class="p-3 border-b">
        <div class="relative">
          <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            v-model="searchQuery"
            placeholder="Search responses..."
            class="pl-8 h-9"
            @keydown.stop
          />
        </div>
      </div>

      <ScrollArea class="h-[300px]">
        <div v-if="isLoading" class="flex items-center justify-center py-8">
          <Loader2 class="h-6 w-6 animate-spin text-muted-foreground" />
        </div>

        <div v-else-if="filteredResponses.length === 0" class="py-8 text-center text-muted-foreground text-sm">
          No canned responses found
        </div>

        <div v-else class="p-2">
          <template v-for="(items, category) in groupedResponses" :key="category">
            <div class="px-2 py-1.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">
              {{ getCategoryLabel(category) }}
            </div>
            <button
              v-for="response in items"
              :key="response.id"
              @click="selectResponse(response)"
              class="w-full text-left px-3 py-2 rounded-md hover:bg-accent transition-colors"
            >
              <div class="flex items-center justify-between">
                <span class="font-medium text-sm">{{ response.name }}</span>
                <span v-if="response.shortcut" class="text-xs font-mono text-muted-foreground">
                  /{{ response.shortcut }}
                </span>
              </div>
              <p class="text-xs text-muted-foreground mt-0.5 line-clamp-2">
                {{ response.content }}
              </p>
            </button>
          </template>
        </div>
      </ScrollArea>
    </PopoverContent>
  </Popover>
</template>
