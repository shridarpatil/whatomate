<script setup lang="ts">
import { ref, watch, computed, onMounted } from 'vue'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
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
import { X, ChevronDown, Plus, Check, Tags, Zap, Layout, History, MessageSquare, Shield, Smartphone } from 'lucide-vue-next'
import { TagBadge } from '@/components/ui/tag-badge'
import MetadataSection from '@/components/chat/MetadataSection.vue'
import { getInitials, getAvatarGradient, formatLabel } from '@/lib/utils'
import { getTagColorClass } from '@/lib/constants'
import { useTagsStore } from '@/stores/tags'
import { useAuthStore } from '@/stores/auth'
import { contactsService, type Tag } from '@/services/api'
import { toast } from 'vue-sonner'
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
}>()

const tagsStore = useTagsStore()
const authStore = useAuthStore()

const collapsedSections = ref<Record<string, boolean>>({})
const tagSelectorOpen = ref(false)
const isUpdatingTags = ref(false)

// Resizable panel state
const MIN_WIDTH = 300
const MAX_WIDTH = 480
const panelWidth = ref(340)
const isResizing = ref(false)

// Check if user can edit tags
const canEditTags = computed(() => authStore.hasPermission('contacts', 'write'))

// Fetch tags on mount
onMounted(async () => {
  if (tagsStore.tags.length === 0) {
    try {
      await tagsStore.fetchTags()
    } catch (e) {
      console.error('Failed to fetch tags for ContactInfoPanel:', e)
    }
  }
})

function startResize(e: MouseEvent) {
  isResizing.value = true
  const startX = e.clientX
  const startWidth = panelWidth.value

  function onMouseMove(e: MouseEvent) {
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
    case 'success': return 'bg-emerald-500/10 text-emerald-500 border-emerald-500/20'
    case 'warning': return 'bg-amber-500/10 text-amber-500 border-amber-500/20'
    case 'error': return 'bg-red-500/10 text-red-500 border-red-500/20'
    case 'info': return 'bg-blue-500/10 text-blue-500 border-blue-500/20'
    default: return 'bg-muted text-muted-foreground'
  }
}

const sortedSections = computed(() => {
  if (!props.sessionData?.panel_config?.sections) return []
  return [...props.sessionData.panel_config.sections].sort((a, b) => a.order - b.order)
})

const contactTags = computed(() => (props.contact.tags as string[]) || [])

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

function getTagDetails(tagName: string): Tag | undefined {
  return tagsStore.getTagByName(tagName)
}

async function toggleTag(tagName: string) {
  if (isUpdatingTags.value) return
  const currentTags = [...contactTags.value]
  const newTags = currentTags.includes(tagName)
    ? currentTags.filter(t => t !== tagName)
    : [...currentTags, tagName]

  isUpdatingTags.value = true
  try {
    await contactsService.updateTags(props.contact.id, newTags)
    emit('tagsUpdated', newTags)
    toast.success('Tags updated')
  } catch (e) {
    toast.error('Failed to update tags')
  } finally {
    isUpdatingTags.value = false
  }
}

</script>

<template>
  <div class="flex flex-col bg-background border-l h-full relative overflow-hidden"
    :style="{ width: `${panelWidth}px` }">
    <!-- Resize Handle -->
    <div
      class="absolute left-0 top-0 bottom-0 w-1 cursor-col-resize hover:bg-primary/20 active:bg-primary/30 z-30 border-l border-white/5"
      :class="{ 'bg-primary/30': isResizing }" @mousedown="startResize" />

    <!-- Minimal Header -->
    <div class="h-10 px-4 flex items-center justify-between border-b bg-card/10 backdrop-blur-sm sticky top-0 z-20">
      <div class="flex items-center gap-2 overflow-hidden">
        <span
          class="text-[9px] font-black uppercase tracking-[0.2em] text-muted-foreground/40 whitespace-nowrap">Diagnostic
          Profile</span>
      </div>
      <Button variant="ghost" size="icon" class="h-6 w-6 rounded-lg hover:bg-muted" @click="emit('close')">
        <X class="h-3 w-3" />
      </Button>
    </div>

    <ScrollArea class="flex-1">
      <div class="p-5 space-y-8 pb-10">
        <!-- Contact Profile Hero -->
        <div class="flex flex-col items-center text-center space-y-4">
          <div class="relative group cursor-pointer">
            <Avatar class="h-24 w-24 ring-4 ring-background shadow-2xl transition-transform group-hover:scale-105">
              <AvatarImage :src="contact.avatar_url" />
              <AvatarFallback
                :class="'text-2xl font-bold bg-gradient-to-br text-white ' + getAvatarGradient(contact.name || contact.phone_number)">
                {{ getInitials(contact.name || contact.phone_number) }}
              </AvatarFallback>
            </Avatar>
            <div
              class="absolute -bottom-1 -right-1 h-6 w-6 rounded-full bg-emerald-500 border-4 border-background shadow-sm"
              title="Active Session"></div>
          </div>
          <div class="space-y-1">
            <h4 class="text-xl font-bold tracking-tight">{{ contact.name || contact.phone_number }}</h4>
            <div class="flex flex-col items-center gap-1.5 pt-1">
              <div
                class="flex items-center gap-2 text-xs text-muted-foreground font-medium uppercase tracking-widest opacity-60">
                <Smartphone class="h-3 w-3" />
                <span>{{ contact.phone_number }}</span>
              </div>
            </div>
          </div>
        </div>

        <Separator class="opacity-10" />

        <!-- Tags Area -->
        <div class="space-y-4">
          <div class="flex items-center justify-between px-1">
            <div
              class="text-[10px] font-bold text-primary uppercase tracking-[0.2em] opacity-80 pl-1 flex items-center gap-2">
              <Tags class="h-3 w-3" /> Classification
            </div>
            <Popover v-if="canEditTags" v-model:open="tagSelectorOpen">
              <PopoverTrigger as-child>
                <Button variant="ghost" size="sm" class="h-6 w-6 rounded-lg p-0">
                  <Plus class="h-3.5 w-3.5" />
                </Button>
              </PopoverTrigger>
              <PopoverContent class="w-[200px] p-0" align="end">
                <Command>
                  <CommandInput placeholder="Search tags..." class="h-9" />
                  <CommandList>
                    <CommandEmpty>No tags found</CommandEmpty>
                    <CommandGroup>
                      <CommandItem v-for="tag in tagsStore.tags" :key="tag.name" :value="tag.name"
                        class="flex items-center gap-2 rounded-md m-1" @select="toggleTag(tag.name)">
                        <div :class="['w-2 h-2 rounded-full', getTagColorClass(tag.color).split(' ')[0]]"></div>
                        <span class="text-sm font-medium">{{ tag.name }}</span>
                        <Check v-if="contactTags.includes(tag.name)" class="ml-auto h-3.5 w-3.5 text-primary" />
                      </CommandItem>
                    </CommandGroup>
                  </CommandList>
                </Command>
              </PopoverContent>
            </Popover>
          </div>
          <div class="flex flex-wrap gap-2 px-1">
            <TagBadge v-for="tagName in contactTags" :key="tagName" :color="getTagDetails(tagName)?.color">
              {{ tagName }}
              <button v-if="canEditTags" class="ml-1.5 opacity-60 hover:opacity-100 transition-opacity"
                @click="toggleTag(tagName)">
                <X class="h-2.5 w-2.5" />
              </button>
            </TagBadge>
            <div v-if="contactTags.length === 0" class="text-[11px] text-muted-foreground italic pl-1">No tags assigned.
            </div>
          </div>
        </div>

        <!-- Metadata Section -->
        <div v-if="hasMetadata" class="space-y-4">
          <div
            class="text-[10px] font-bold text-primary uppercase tracking-[0.2em] opacity-80 pl-2 flex items-center gap-2">
            <Layout class="h-3 w-3" /> Knowledge Base
          </div>
          <div class="space-y-3">
            <MetadataSection v-if="metadataPrimitives.length > 0" label="General"
              :data="Object.fromEntries(metadataPrimitives)" />
            <MetadataSection v-for="[key, val] in metadataSections" :key="key" :label="formatLabel(key)" :data="val" />
          </div>
        </div>

        <!-- Flow Session Data -->
        <div v-if="props.sessionData" class="space-y-4 pt-4 border-t border-white/5">
          <div
            class="text-[10px] font-bold text-primary uppercase tracking-[0.2em] opacity-80 pl-2 flex items-center gap-2">
            <Zap class="h-3 w-3" /> Live Session
          </div>
          <div v-if="props.sessionData?.flow_name" class="flex px-2">
            <Badge variant="outline"
              class="text-[10px] font-bold bg-primary/5 text-primary border-primary/20 tracking-wider">
              {{ props.sessionData?.flow_name }}
            </Badge>
          </div>

          <div v-for="section in sortedSections" :key="section.id" class="space-y-2">
            <Collapsible v-if="section.collapsible" :open="!isSectionCollapsed(section.id)"
              @update:open="toggleSection(section.id)">
              <CollapsibleTrigger
                class="flex items-center justify-between w-full p-2 text-xs font-bold hover:text-primary transition-colors bg-muted/20 rounded-lg">
                <span>{{ section.label }}</span>
                <ChevronDown :class="[
                  'h-3.5 w-3.5 transition-transform',
                  isSectionCollapsed(section.id) ? '-rotate-90' : ''
                ]" />
              </CollapsibleTrigger>
              <CollapsibleContent>
                <div :class="[
                  'grid gap-2 pt-2',
                  section.columns === 2 ? 'grid-cols-2' : 'grid-cols-1'
                ]">
                  <div v-for="field in section.fields.sort((a, b) => a.order - b.order)" :key="field.key"
                    class="bg-muted/30 rounded-xl px-3 py-2 border border-white/5">
                    <p class="text-[9px] uppercase tracking-wide text-muted-foreground font-black opacity-40">{{
                      field.label }}</p>
                    <span v-if="field.display_type === 'badge' || field.display_type === 'tag'"
                      :class="['inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-bold mt-1 border', getColorClass(field.color)]">
                      {{ getFieldValue(field.key) }}
                    </span>
                    <p v-else class="text-xs font-bold break-words mt-1">{{ getFieldValue(field.key) }}</p>
                  </div>
                </div>
              </CollapsibleContent>
            </Collapsible>
          </div>
        </div>

        <!-- Timeline -->
        <div class="space-y-5 pt-4 border-t border-white/5">
          <div
            class="text-[10px] font-bold text-primary uppercase tracking-[0.2em] opacity-80 px-2 pl-2 flex items-center gap-2">
            <History class="h-3 w-3" /> Timeline
          </div>
          <div
            class="space-y-6 pl-3 relative before:absolute before:left-[11px] before:top-2 before:bottom-2 before:w-[1px] before:bg-white/5 before:rounded-full">
            <div class="flex gap-4 relative">
              <div
                class="h-6 w-6 rounded-full bg-background border border-primary flex items-center justify-center shrink-0 z-10 shadow-sm shadow-primary/20">
                <MessageSquare class="h-3 w-3 text-primary" />
              </div>
              <div class="space-y-1">
                <p class="text-[11px] font-bold">New Message Received</p>
                <p class="text-[10px] text-muted-foreground font-medium">The customer responded to the campaign.</p>
                <p class="text-[9px] font-black text-primary uppercase tracking-tighter opacity-70">Just Now</p>
              </div>
            </div>
            <div class="flex gap-4 relative opacity-40">
              <div
                class="h-6 w-6 rounded-full bg-background border border-white/10 flex items-center justify-center shrink-0 z-10">
                <Shield class="h-3 w-3 text-muted-foreground" />
              </div>
              <div class="space-y-1">
                <p class="text-[11px] font-bold">Bot Session Resumed</p>
                <p class="text-[10px] text-muted-foreground font-medium">Transitioned from AI Agent to Admin.</p>
                <p class="text-[9px] font-black text-muted-foreground uppercase tracking-tighter">1h ago</p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </ScrollArea>
  </div>
</template>
