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
import { templatesService } from '@/services/api'
import { LayoutTemplate, Search, Loader2 } from 'lucide-vue-next'

const props = defineProps<{
  selectedAccount?: string | null
}>()

const emit = defineEmits<{
  (e: 'select-with-params', template: any, paramNames: string[]): void
}>()

const { t } = useI18n()

const isOpen = ref(false)
const isLoading = ref(false)
const searchQuery = ref('')
const templates = ref<any[]>([])

// Fetch templates when popover opens
watch(isOpen, async (open) => {
  if (open) {
    await fetchTemplates()
  }
})

async function fetchTemplates() {
  isLoading.value = true
  try {
    const params: any = { status: 'APPROVED' }
    if (props.selectedAccount) {
      params.account = props.selectedAccount
    }
    const response = await templatesService.list(params)
    const data = (response.data as any).data || response.data
    templates.value = data.templates || []
  } catch (error) {
    console.error('Failed to fetch templates:', error)
  } finally {
    isLoading.value = false
  }
}

const filteredTemplates = computed(() => {
  if (!searchQuery.value) return templates.value
  const query = searchQuery.value.toLowerCase()
  return templates.value.filter((tpl: any) =>
    (tpl.display_name || '').toLowerCase().includes(query) ||
    (tpl.name || '').toLowerCase().includes(query) ||
    getBodyContent(tpl).toLowerCase().includes(query)
  )
})

// Group by category
const groupedTemplates = computed(() => {
  const groups: Record<string, any[]> = {}
  for (const tpl of filteredTemplates.value) {
    const category = tpl.category || 'OTHER'
    if (!groups[category]) {
      groups[category] = []
    }
    groups[category].push(tpl)
  }
  return groups
})

const categoryLabels: Record<string, string> = {
  UTILITY: 'Utility',
  MARKETING: 'Marketing',
  AUTHENTICATION: 'Authentication',
  OTHER: 'Other'
}

function getCategoryLabel(category: string): string {
  return categoryLabels[category] || category
}

function getBodyContent(tpl: any): string {
  return tpl.body_content || ''
}

function extractParamNames(bodyContent: string): string[] {
  const matches = bodyContent.match(/\{\{([^}]+)\}\}/g)
  if (!matches) return []
  return [...new Set(matches.map(m => m.replace(/\{\{|\}\}/g, '').trim()))]
}

function selectTemplate(tpl: any) {
  const bodyContent = getBodyContent(tpl)
  const paramNames = extractParamNames(bodyContent)

  // Always show preview dialog before sending
  emit('select-with-params', tpl, paramNames)

  isOpen.value = false
  searchQuery.value = ''
}
</script>

<template>
  <Popover v-model:open="isOpen">
    <PopoverTrigger as-child>
      <Button type="button" variant="ghost" size="icon">
        <LayoutTemplate class="h-5 w-5" />
      </Button>
    </PopoverTrigger>
    <PopoverContent side="top" align="start" class="w-80 p-0">
      <div class="p-3 border-b">
        <div class="relative">
          <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            v-model="searchQuery"
            :placeholder="t('chat.searchTemplates')"
            class="pl-8 h-9"
            @keydown.stop
          />
        </div>
      </div>

      <ScrollArea class="h-[300px]">
        <div v-if="isLoading" class="flex items-center justify-center py-8">
          <Loader2 class="h-6 w-6 animate-spin text-muted-foreground" />
        </div>

        <div v-else-if="filteredTemplates.length === 0" class="py-8 text-center text-muted-foreground text-sm">
          {{ t('chat.noApprovedTemplates') }}
        </div>

        <div v-else class="p-2">
          <template v-for="(items, category) in groupedTemplates" :key="category">
            <div class="px-2 py-1.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">
              {{ getCategoryLabel(category as string) }}
            </div>
            <button
              v-for="tpl in items"
              :key="tpl.id"
              @click="selectTemplate(tpl)"
              class="w-full text-left px-3 py-2 rounded-md hover:bg-accent transition-colors"
            >
              <div class="flex items-center justify-between">
                <span class="font-medium text-sm">{{ tpl.display_name || tpl.name }}</span>
                <span class="text-xs text-muted-foreground">{{ tpl.language || '' }}</span>
              </div>
              <p class="text-xs text-muted-foreground mt-0.5 line-clamp-2">
                {{ getBodyContent(tpl) }}
              </p>
            </button>
          </template>
        </div>
      </ScrollArea>
    </PopoverContent>
  </Popover>
</template>
