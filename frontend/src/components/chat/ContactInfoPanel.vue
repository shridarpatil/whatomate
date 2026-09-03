<script setup lang="ts">
import { ref, watch, computed, onMounted } from 'vue'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Popover,
  PopoverContent,
  PopoverTrigger
} from '@/components/ui/popover'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList
} from '@/components/ui/command'
import { X, ChevronDown, Phone, User, Plus, Check, Tags, Loader2, Copy, Pencil } from 'lucide-vue-next'
import { TagBadge } from '@/components/ui/tag-badge'
import { IconButton } from '@/components/shared'
import { Input } from '@/components/ui/input'
import MetadataSection from '@/components/chat/MetadataSection.vue'
import { getInitials, getAvatarGradient, formatLabel } from '@/lib/utils'
import { getTagColorClass } from '@/lib/constants'
import { useTagsStore } from '@/stores/tags'
import { useAuthStore } from '@/stores/auth'
import { useI18n } from 'vue-i18n'
import { contactsService, type Tag } from '@/services/api'
import { toast } from 'vue-sonner'
import { getErrorMessage } from '@/lib/api-utils'
import type { Contact } from '@/stores/contacts'

interface PanelFieldConfig {
  key: string
  label: string
  order: number
  display_type?: 'text' | 'badge' | 'tag'
  color?: 'default' | 'success' | 'warning' | 'error' | 'info'
}

interface PanelSection {
  id: string
  label: string
  columns: number
  collapsible: boolean
  default_collapsed: boolean
  order: number
  fields: PanelFieldConfig[]
}

interface PanelConfig {
  sections: PanelSection[]
}

interface SessionData {
  session_id?: string
  flow_id?: string
  flow_name?: string
  session_data: Record<string, any>
  panel_config: PanelConfig
}

const props = defineProps<{
  contact: Contact
  sessionData?: SessionData | null
}>()

const emit = defineEmits<{
  close: []
  tagsUpdated: [tags: string[]]
  nameUpdated: [name: string]
}>()

const { t } = useI18n()
const tagsStore = useTagsStore()
const authStore = useAuthStore()
const collapsedSections = ref<Record<string, boolean>>({})
const tagSelectorOpen = ref(false)
const isUpdatingTags = ref(false)

// Resizable panel state
const MIN_WIDTH = 280
const MAX_WIDTH = 480
const panelWidth = ref(MAX_WIDTH) // Default to max width
const isResizing = ref(false)

// Check if user can edit tags
const canEditTags = computed(() => authStore.hasPermission('contacts', 'write'))

const canRenameContact = computed(() => authStore.hasPermission('contacts.name', 'write'))

const isEditingName = ref(false)
const nameDraft = ref('')
const isSavingName = ref(false)

const displayName = computed(() => props.contact.name || props.contact.phone_number)

function startEditName() {
  // Pre-fill from contact.name, never from displayName: displayName falls
  // back to the phone number (masked, when the org masks it) when there's
  // no name, and that shouldn't be offered as a name to save.
  nameDraft.value = props.contact.name || ''
  isEditingName.value = true
}

function copyText(value: string) {
  navigator.clipboard.writeText(value)
  toast.success(t('common.copiedToClipboard'))
}

async function saveName() {
  const novo = nameDraft.value.trim()
  if (!novo) return
  isSavingName.value = true
  try {
    await contactsService.updateName(props.contact.id, novo)
    emit('nameUpdated', novo)
    isEditingName.value = false
    toast.success(t('contacts.nameUpdated'))
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedSave', { resource: t('resources.contact') })))
  } finally {
    isSavingName.value = false
  }
}

// Fetch tags on mount
onMounted(async () => {
  if (tagsStore.tags.length === 0) {
    try { await tagsStore.fetchTags() } catch { /* tags won't be available */ }
  }
})

function startResize(e: MouseEvent) {
  isResizing.value = true
  const startX = e.clientX
  const startWidth = panelWidth.value

  function onMouseMove(e: MouseEvent) {
    // Dragging left increases width, dragging right decreases
    const delta = startX - e.clientX
    const newWidth = Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, startWidth + delta))
    panelWidth.value = newWidth
  }

  function onMouseUp() {
    isResizing.value = false
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
  }

  document.addEventListener('mousemove', onMouseMove)
  document.addEventListener('mouseup', onMouseUp)
}

// Initialize collapsed state when session data changes
watch(() => props.sessionData, (newData) => {
  if (newData?.panel_config?.sections) {
    for (const section of newData.panel_config.sections) {
      if (collapsedSections.value[section.id] === undefined) {
        collapsedSections.value[section.id] = section.default_collapsed
      }
    }
  }
}, { immediate: true })

function toggleSection(sectionId: string) {
  collapsedSections.value[sectionId] = !collapsedSections.value[sectionId]
}

function isSectionCollapsed(sectionId: string): boolean {
  return collapsedSections.value[sectionId] ?? false
}

function getFieldValue(key: string): string {
  if (!props.sessionData?.session_data) return '-'
  const value = props.sessionData.session_data[key]
  if (value === undefined || value === null || value === '') return '-'
  return String(value)
}

function getColorClass(color?: string): string {
  switch (color) {
    case 'success':
      return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
    case 'warning':
      return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400'
    case 'error':
      return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
    case 'info':
      return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
    default:
      return 'bg-muted text-muted-foreground'
  }
}

// Sort sections by order
const sortedSections = computed(() => {
  if (!props.sessionData?.panel_config?.sections) return []
  return [...props.sessionData.panel_config.sections].sort((a, b) => a.order - b.order)
})

// Get tags from contact
const contactTags = computed(() => {
  if (!props.contact.tags || !Array.isArray(props.contact.tags)) return []
  return props.contact.tags as string[]
})

// Contact metadata
const hasMetadata = computed(() => {
  const md = props.contact.metadata
  return md && typeof md === 'object' && Object.keys(md).length > 0
})

const metadataPrimitives = computed(() => {
  if (!hasMetadata.value) return []
  return Object.entries(props.contact.metadata).filter(
    ([, v]) => v === null || typeof v !== 'object'
  )
})

const metadataSections = computed(() => {
  if (!hasMetadata.value) return []
  return Object.entries(props.contact.metadata).filter(
    ([, v]) => v !== null && typeof v === 'object'
  )
})

// Get tag details for display
function getTagDetails(tagName: string): Tag | undefined {
  return tagsStore.getTagByName(tagName)
}

// Check if a tag is selected
function isTagSelected(tagName: string): boolean {
  return contactTags.value.includes(tagName)
}

// Toggle tag on contact
async function toggleTag(tagName: string) {
  if (isUpdatingTags.value) return

  const currentTags = [...contactTags.value]
  let newTags: string[]

  if (currentTags.includes(tagName)) {
    newTags = currentTags.filter(t => t !== tagName)
  } else {
    newTags = [...currentTags, tagName]
  }

  await updateContactTags(newTags)
}

// Remove tag from contact
async function removeTag(tagName: string) {
  if (isUpdatingTags.value) return

  const newTags = contactTags.value.filter(t => t !== tagName)
  await updateContactTags(newTags)
}

// Update contact tags via API
async function updateContactTags(tags: string[]) {
  isUpdatingTags.value = true
  try {
    await contactsService.updateTags(props.contact.id, tags)
    emit('tagsUpdated', tags)
    toast.success('Tags updated')
  } catch (e: any) {
    toast.error(e.response?.data?.message || 'Failed to update tags')
  } finally {
    isUpdatingTags.value = false
  }
}

</script>

<template>
  <div
    class="flex flex-col bg-card h-full relative"
    :style="{ width: `${panelWidth}px` }"
  >
    <!-- Resize Handle -->
    <div
      class="absolute left-0 top-0 bottom-0 w-1 cursor-col-resize hover:bg-primary/20 active:bg-primary/30 z-10 border-l"
      :class="{ 'bg-primary/30': isResizing }"
      @mousedown="startResize"
    />

    <!-- Header -->
    <div class="h-12 px-3 border-b flex items-center justify-between">
      <h3 class="font-medium text-sm">{{ $t('chat.contactInfo') }}</h3>
      <Button variant="ghost" size="icon" class="h-8 w-8" @click="emit('close')">
        <X class="h-4 w-4" />
      </Button>
    </div>

    <ScrollArea class="flex-1">
      <div class="p-4 space-y-4">
        <!-- Contact Header -->
        <div class="flex flex-col items-center text-center pb-4 border-b">
          <Avatar class="h-16 w-16 mb-3">
            <AvatarImage :src="contact.avatar_url" />
            <AvatarFallback :class="'text-lg bg-gradient-to-br text-white ' + getAvatarGradient(contact.name || contact.phone_number)">
              {{ getInitials(contact.name || contact.phone_number) }}
            </AvatarFallback>
          </Avatar>
          <div v-if="!isEditingName" class="flex items-center gap-1">
            <h4 id="contact-info-name" class="font-medium">{{ displayName }}</h4>
            <IconButton
              id="contact-info-copy-name"
              :icon="Copy"
              :label="$t('contacts.copyName')"
              class="h-6 w-6"
              @click="copyText(displayName)"
            />
            <IconButton
              v-if="canRenameContact"
              id="contact-info-edit-name"
              :icon="Pencil"
              :label="$t('contacts.editName')"
              class="h-6 w-6"
              @click="startEditName"
            />
          </div>
          <div v-else class="flex items-center gap-1">
            <Input
              id="contact-info-name-input"
              v-model="nameDraft"
              class="h-8 w-44"
              :disabled="isSavingName"
              @keyup.enter="saveName"
            />
            <IconButton
              id="contact-info-save-name"
              :icon="Check"
              :label="$t('contacts.saveName')"
              class="h-6 w-6"
              :disabled="isSavingName"
              @click="saveName"
            />
            <IconButton
              :icon="X"
              :label="$t('contacts.cancelEdit')"
              class="h-6 w-6"
              @click="isEditingName = false"
            />
          </div>
          <div class="flex items-center gap-1 text-sm text-muted-foreground mt-1">
            <Phone class="h-3 w-3" />
            <span id="contact-info-phone">{{ contact.phone_number }}</span>
            <IconButton
              id="contact-info-copy-phone"
              :icon="Copy"
              :label="$t('contacts.copyPhone')"
              class="h-6 w-6"
              @click="copyText(contact.phone_number)"
            />
          </div>
        </div>

        <!-- Tags Section (always shown) -->
        <div class="pb-4">
          <div class="flex items-center justify-between py-2">
            <h5 class="text-sm font-medium flex items-center gap-2">
              <Tags class="h-4 w-4 text-muted-foreground" />
              {{ $t('contacts.tags') }}
            </h5>
            <Popover v-if="canEditTags" v-model:open="tagSelectorOpen">
              <PopoverTrigger as-child>
                <Button variant="ghost" size="sm" class="h-7 px-2">
                  <Plus class="h-3.5 w-3.5" />
                </Button>
              </PopoverTrigger>
              <PopoverContent class="w-[200px] p-0" align="end">
                <Command>
                  <CommandInput :placeholder="$t('contacts.searchTags')" />
                  <CommandList>
                    <CommandEmpty>
                      <div class="py-4 text-center text-sm text-muted-foreground">
                        {{ $t('contacts.noTagsFound') }}
                      </div>
                    </CommandEmpty>
                    <CommandGroup>
                      <CommandItem
                        v-for="tag in tagsStore.tags"
                        :key="tag.name"
                        :value="tag.name"
                        class="flex items-center gap-2"
                        @select="toggleTag(tag.name)"
                      >
                        <div class="flex items-center gap-2 flex-1">
                          <span :class="['w-2 h-2 rounded-full', getTagColorClass(tag.color).split(' ')[0]]"></span>
                          <span>{{ tag.name }}</span>
                        </div>
                        <Check v-if="isTagSelected(tag.name)" class="h-4 w-4 text-primary" />
                      </CommandItem>
                    </CommandGroup>
                  </CommandList>
                </Command>
              </PopoverContent>
            </Popover>
          </div>

          <div class="flex flex-wrap gap-2 mt-2">
            <template v-if="contactTags.length > 0">
              <TagBadge
                v-for="tagName in contactTags"
                :key="tagName"
                :color="getTagDetails(tagName)?.color"
              >
                {{ tagName }}
                <button
                  v-if="canEditTags"
                  type="button"
                  class="ml-1 rounded-full hover:bg-black/10 dark:hover:bg-white/10 p-0.5 transition-colors"
                  :disabled="isUpdatingTags"
                  @click.stop="removeTag(tagName)"
                >
                  <X class="h-3 w-3" />
                </button>
              </TagBadge>
            </template>
            <span v-else class="text-sm text-muted-foreground">{{ $t('contacts.noTags') }}</span>
            <Loader2 v-if="isUpdatingTags" class="h-4 w-4 animate-spin text-muted-foreground" />
          </div>
        </div>

        <!-- Contact Metadata -->
        <div v-if="hasMetadata" class="space-y-3">
          <!-- General section: top-level primitives -->
          <MetadataSection
            v-if="metadataPrimitives.length > 0"
            :label="$t('chat.generalSection')"
            :data="Object.fromEntries(metadataPrimitives)"
          />
          <!-- Object / array sections -->
          <MetadataSection
            v-for="[key, val] in metadataSections"
            :key="key"
            :label="formatLabel(key)"
            :data="val"
          />
        </div>

        <!-- No Session Data or no panel config -->
        <div v-if="!props.sessionData || sortedSections.length === 0" class="text-center py-6 text-muted-foreground border-t">
          <User class="h-8 w-8 mx-auto mb-2 opacity-50" />
          <p class="text-sm">{{ $t('chat.noDataConfigured') }}</p>
          <p class="text-xs mt-1">{{ $t('chat.configurePanelHint') }}</p>
        </div>

        <!-- Session Data with panel config -->
        <template v-else>
          <!-- Flow Name Badge -->
          <div v-if="props.sessionData?.flow_name" class="flex items-center gap-2">
            <Badge variant="outline" class="text-xs">
              {{ props.sessionData?.flow_name }}
            </Badge>
          </div>

          <!-- Dynamic Sections -->
          <div v-for="section in sortedSections" :key="section.id" class="space-y-2 border-t pt-4">
            <Collapsible
              v-if="section.collapsible"
              :open="!isSectionCollapsed(section.id)"
              @update:open="toggleSection(section.id)"
            >
              <CollapsibleTrigger class="flex items-center justify-between w-full py-2 text-sm font-medium hover:text-primary transition-colors">
                <span>{{ section.label }}</span>
                <ChevronDown
                  :class="[
                    'h-4 w-4 transition-transform',
                    isSectionCollapsed(section.id) ? '-rotate-90' : ''
                  ]"
                />
              </CollapsibleTrigger>
              <CollapsibleContent>
                <div
                  :class="[
                    'grid gap-2 pt-2',
                    section.columns === 2 ? 'grid-cols-2' : 'grid-cols-1'
                  ]"
                >
                  <div
                    v-for="field in section.fields.sort((a, b) => a.order - b.order)"
                    :key="field.key"
                    class="bg-muted/50 rounded-md px-3 py-2"
                  >
                    <p class="text-[10px] uppercase tracking-wide text-muted-foreground font-medium">{{ field.label }}</p>
                    <!-- Badge display -->
                    <span
                      v-if="field.display_type === 'badge'"
                      :class="['inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold mt-1', getColorClass(field.color)]"
                    >
                      {{ getFieldValue(field.key) }}
                    </span>
                    <!-- Tag display -->
                    <span
                      v-else-if="field.display_type === 'tag'"
                      :class="['inline-flex items-center rounded-md px-2 py-1 text-xs font-medium mt-1', getColorClass(field.color)]"
                    >
                      {{ getFieldValue(field.key) }}
                    </span>
                    <!-- Default text display -->
                    <p v-else class="text-sm font-semibold break-words mt-0.5">{{ getFieldValue(field.key) }}</p>
                  </div>
                </div>
              </CollapsibleContent>
            </Collapsible>

            <!-- Non-collapsible section -->
            <div v-else>
              <h5 class="py-2 text-sm font-medium">{{ section.label }}</h5>
              <div
                :class="[
                  'grid gap-2',
                  section.columns === 2 ? 'grid-cols-2' : 'grid-cols-1'
                ]"
              >
                <div
                  v-for="field in section.fields.sort((a, b) => a.order - b.order)"
                  :key="field.key"
                  class="bg-muted/50 rounded-md px-3 py-2"
                >
                  <p class="text-[10px] uppercase tracking-wide text-muted-foreground font-medium">{{ field.label }}</p>
                  <!-- Badge display -->
                  <span
                    v-if="field.display_type === 'badge'"
                    :class="['inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold mt-1', getColorClass(field.color)]"
                  >
                    {{ getFieldValue(field.key) }}
                  </span>
                  <!-- Tag display -->
                  <span
                    v-else-if="field.display_type === 'tag'"
                    :class="['inline-flex items-center rounded-md px-2 py-1 text-xs font-medium mt-1', getColorClass(field.color)]"
                  >
                    {{ getFieldValue(field.key) }}
                  </span>
                  <!-- Default text display -->
                  <p v-else class="text-sm font-semibold break-words mt-0.5">{{ getFieldValue(field.key) }}</p>
                </div>
              </div>
            </div>
          </div>
        </template>
      </div>
    </ScrollArea>
  </div>
</template>
