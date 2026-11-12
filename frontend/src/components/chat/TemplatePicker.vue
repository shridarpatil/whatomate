<script setup lang="ts">
import { ref, computed, nextTick, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { templatesService } from '@/services/api'
import { LayoutTemplate } from '@/lib/icons'

const props = defineProps<{
  selectedAccount?: string | null
}>()

const emit = defineEmits<{
  (e: 'select-with-params', template: any, paramNames: string[]): void
}>()

const { t } = useI18n()

const popoverId = `${useId()}-templates`
const popover = ref<HTMLElement | null>(null)
const trigger = ref<HTMLButtonElement | null>(null)
const search = ref<HTMLInputElement | null>(null)
const isLoading = ref(false)
const searchQuery = ref('')
const templates = ref<any[]>([])

// Fetch templates whenever the popover opens
function onToggle(e: Event) {
  if ((e as ToggleEvent).newState !== 'open') return
  nextTick(() => search.value?.focus())
  fetchTemplates()
}

// Lets the parent open the picker (e.g. from the service-window banner).
function open() {
  // Open directly (not via a simulated trigger click, which toggles and can
  // immediately close the menu again).
  const el = popover.value
  if (el && !el.matches(':popover-open')) el.showPopover()
}
defineExpose({ open })

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

function extractParamNames(content: string): string[] {
  const matches = content.match(/\{\{([^}]+)\}\}/g)
  if (!matches) return []
  return [...new Set(matches.map(m => m.replace(/\{\{|\}\}/g, '').trim()))]
}

function selectTemplate(tpl: any) {
  // Emit body variables only — the header variable is handled separately by
  // the parent so positional {{1}} in header and body don't collapse into a
  // single input (Meta indexes positional vars per component).
  const bodyNames = extractParamNames(getBodyContent(tpl))

  // Always show preview dialog before sending
  emit('select-with-params', tpl, bodyNames)

  popover.value?.hidePopover()
  searchQuery.value = ''
}
</script>

<template>
  <ot-dropdown>
    <button
      ref="trigger"
      type="button"
      class="ghost icon"
      :popovertarget="popoverId"
      :title="t('chat.sendTemplate')"
      :aria-label="t('chat.sendTemplate')"
    >
      <LayoutTemplate :size="20" />
    </button>
    <div :id="popoverId" ref="popover" popover class="picker" data-picker="templates" @toggle="onToggle">
      <div class="vstack gap-2 p-4">
        <input
          ref="search"
          v-model="searchQuery"
          type="search"
          :placeholder="t('chat.searchTemplates')"
          :aria-label="t('chat.searchTemplates')"
        />

        <div class="scroll-y picker-list vstack gap-1">
          <div v-if="isLoading" class="flex justify-center p-4">
            <span role="status" aria-busy="true" :aria-label="t('common.loading')" />
          </div>

          <p v-else-if="filteredTemplates.length === 0" class="align-center text-light p-4">
            <small>{{ t('chat.noApprovedTemplates') }}</small>
          </p>

          <template v-else>
            <template v-for="(items, category) in groupedTemplates" :key="category">
              <small class="text-light">{{ getCategoryLabel(category as string) }}</small>
              <button
                v-for="tpl in items"
                :key="tpl.id"
                type="button"
                class="ghost w-100 shrink-0"
                data-picker-item
                @click="selectTemplate(tpl)"
              >
                <span class="vstack gap-1 w-100 min-w-0 align-left">
                  <span class="flex justify-between gap-2">
                    <strong class="truncate">{{ tpl.display_name || tpl.name }}</strong>
                    <small class="text-light">{{ tpl.language || '' }}</small>
                  </span>
                  <small class="text-light truncate">{{ getBodyContent(tpl) }}</small>
                </span>
              </button>
            </template>
          </template>
        </div>
      </div>
    </div>
  </ot-dropdown>
</template>
