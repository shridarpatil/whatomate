<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { flowsService } from '@/services/api'
import { Workflow, Search, Loader2 } from 'lucide-vue-next'

const props = defineProps<{
  selectedAccount?: string | null
}>()

const emit = defineEmits<{
  (e: 'select', flow: any): void
}>()

const { t } = useI18n()

const isOpen = ref(false)
const isLoading = ref(false)
const searchQuery = ref('')
const flows = ref<any[]>([])

// Fetch flows when popover opens
watch(isOpen, async (open) => {
  if (open) {
    await fetchFlows()
  }
})

async function fetchFlows() {
  isLoading.value = true
  try {
    const params: any = { status: 'PUBLISHED' }
    if (props.selectedAccount) {
      params.account = props.selectedAccount
    }
    const response = await flowsService.list(params)
    const data = (response.data as any).data || response.data
    flows.value = data.flows || []
  } catch (error) {
    console.error('Failed to fetch flows:', error)
  } finally {
    isLoading.value = false
  }
}

const filteredFlows = computed(() => {
  if (!searchQuery.value) return flows.value
  const query = searchQuery.value.toLowerCase()
  return flows.value.filter((fl: any) =>
    (fl.name || '').toLowerCase().includes(query) ||
    (fl.category || '').toLowerCase().includes(query)
  )
})

// Group by category
const groupedFlows = computed(() => {
  const groups: Record<string, any[]> = {}
  for (const fl of filteredFlows.value) {
    const category = fl.category || 'OTHER'
    if (!groups[category]) {
      groups[category] = []
    }
    groups[category].push(fl)
  }
  return groups
})

const categoryLabels: Record<string, string> = {
  SIGN_UP: 'Sign Up',
  LEAD_GENERATION: 'Lead Generation',
  CUSTOMER_SUPPORT: 'Customer Support',
  SURVEY: 'Survey',
  OTHER: 'Other'
}

function getCategoryLabel(category: string): string {
  return categoryLabels[category] || category
}

function selectFlow(fl: any) {
  emit('select', fl)
  isOpen.value = false
  searchQuery.value = ''
}
</script>

<template>
  <Popover v-model:open="isOpen">
    <PopoverTrigger as-child>
      <Button type="button" variant="ghost" size="icon">
        <Workflow class="h-5 w-5" />
      </Button>
    </PopoverTrigger>
    <PopoverContent side="top" align="start" class="w-80 p-0">
      <div class="p-3 border-b">
        <div class="relative">
          <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            v-model="searchQuery"
            :placeholder="t('chat.searchFlows', 'Search flows...')"
            class="pl-8 h-9"
            @keydown.stop
          />
        </div>
      </div>

      <ScrollArea class="h-[300px]">
        <div v-if="isLoading" class="flex items-center justify-center py-8">
          <Loader2 class="h-6 w-6 animate-spin text-muted-foreground" />
        </div>

        <div v-else-if="filteredFlows.length === 0" class="py-8 text-center text-muted-foreground text-sm">
          {{ t('chat.noPublishedFlows', 'No published flows found') }}
        </div>

        <div v-else class="p-2">
          <template v-for="(items, category) in groupedFlows" :key="category">
            <div class="px-2 py-1.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">
              {{ getCategoryLabel(category as string) }}
            </div>
            <button
              v-for="fl in items"
              :key="fl.id"
              @click="selectFlow(fl)"
              class="w-full text-left px-3 py-2 rounded-md hover:bg-accent transition-colors"
            >
              <div class="flex items-center justify-between">
                <span class="font-medium text-sm">{{ fl.name }}</span>
                <span class="text-xs text-muted-foreground">{{ fl.whatsapp_account || '' }}</span>
              </div>
              <p class="text-xs text-muted-foreground mt-0.5 line-clamp-1">
                ID: {{ fl.meta_flow_id }}
              </p>
            </button>
          </template>
        </div>
      </ScrollArea>
    </PopoverContent>
  </Popover>
</template>
