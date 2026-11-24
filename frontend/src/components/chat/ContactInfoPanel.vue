<script setup lang="ts">
import { ref, watch, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { X, Phone, User, Tags } from '@/lib/icons'
import Combobox from '@/components/oat/Combobox.vue'
import MetadataSection from '@/components/chat/MetadataSection.vue'
import { getInitials, formatLabel } from '@/lib/utils'
import { useTagsStore } from '@/stores/tags'
import { useAuthStore } from '@/stores/auth'
import { contactsService } from '@/services/api'
import { toast } from '@/lib/toast'
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
  /** Phone layout: the panel fills the screen and is not resizable. */
  narrow?: boolean
}>()

const emit = defineEmits<{
  close: []
  tagsUpdated: [tags: string[]]
}>()

const { t } = useI18n()
const tagsStore = useTagsStore()
const authStore = useAuthStore()
const collapsedSections = ref<Record<string, boolean>>({})
const isUpdatingTags = ref(false)

// Resizable panel state
const MIN_WIDTH = 280
const MAX_WIDTH = 480
const panelWidth = ref(MAX_WIDTH) // Default to max width
const avatarFailed = ref(false)
watch(() => props.contact.avatar_url, () => { avatarFailed.value = false })

// Check if user can edit tags
const canEditTags = computed(() => authStore.hasPermission('contacts', 'write'))

// Fetch tags on mount
onMounted(async () => {
  if (tagsStore.tags.length === 0) {
    try { await tagsStore.fetchTags() } catch { /* tags won't be available */ }
  }
})

function startResize(e: MouseEvent) {
  const startX = e.clientX
  const startWidth = panelWidth.value

  function onMouseMove(e: MouseEvent) {
    // Dragging left increases width, dragging right decreases
    const delta = startX - e.clientX
    const newWidth = Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, startWidth + delta))
    panelWidth.value = newWidth
  }

  function onMouseUp() {
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

function onSectionToggle(sectionId: string, e: Event) {
  collapsedSections.value[sectionId] = !(e.target as HTMLDetailsElement).open
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

// Panel field colours map onto Oat badge variants ('info' keeps the primary badge).
function getBadgeVariant(color?: string): string | undefined {
  switch (color) {
    case 'success':
    case 'warning':
      return color
    case 'error':
      return 'danger'
    case 'info':
      return undefined
    default:
      return 'secondary'
  }
}

function sortedFields(section: PanelSection): PanelFieldConfig[] {
  return [...section.fields].sort((a, b) => a.order - b.order)
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

// Tags not yet on the contact; chosen ones are removed via their badge.
const tagOptions = computed(() =>
  tagsStore.tags
    .filter(tag => !contactTags.value.includes(tag.name))
    .map(tag => ({ value: tag.name, label: tag.name })),
)

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
  <div class="flex h-full min-h-0 chat-col" :style="narrow ? undefined : { width: `${panelWidth}px` }">
    <!-- Resize handle: drag left to widen -->
    <div v-if="!narrow" class="resize-handle" role="separator" aria-orientation="vertical" @mousedown="startResize" />

    <article data-testid="contact-info" class="card p-4 flex-1 min-w-0 flex flex-col min-h-0 gap-4">
      <header class="hstack justify-between shrink-0">
        <h4>{{ t('chat.contactInfo') }}</h4>
        <button type="button" class="ghost small icon" :title="t('common.close')" :aria-label="t('common.close')" @click="emit('close')">
          <X />
        </button>
      </header>

      <div class="flex-1 scroll-y vstack gap-4">
        <!-- Contact Header -->
        <div class="vstack gap-1 items-center align-center">
          <figure data-variant="avatar" class="large">
            <img v-if="contact.avatar_url && !avatarFailed" :src="contact.avatar_url" alt="" @error="avatarFailed = true" />
            <abbr v-else>{{ getInitials(contact.name || contact.phone_number) }}</abbr>
          </figure>
          <strong>{{ contact.name || contact.phone_number }}</strong>
          <small class="hstack gap-1 text-light">
            <Phone :size="12" />
            <span>{{ contact.phone_number }}</span>
          </small>
        </div>

        <hr />

        <!-- Tags Section (always shown) -->
        <section class="vstack gap-2">
          <div class="hstack justify-between">
            <strong class="hstack gap-2"><Tags class="text-light" />{{ t('contacts.tags') }}</strong>
            <Combobox
              v-if="canEditTags"
              class="small"
              :model-value="null"
              :options="tagOptions"
              :placeholder="t('contacts.addTag')"
              :search-placeholder="t('contacts.searchTags')"
              :empty-text="t('contacts.noTagsFound')"
              :disabled="isUpdatingTags"
              @update:model-value="toggleTag"
            />
          </div>

          <div class="hstack gap-2">
            <template v-if="contactTags.length > 0">
              <span v-for="tagName in contactTags" :key="tagName" class="badge" data-variant="secondary">
                {{ tagName }}
                <button
                  v-if="canEditTags"
                  type="button"
                  :disabled="isUpdatingTags"
                  :aria-label="`${t('contacts.removeTag')}: ${tagName}`"
                  @click.stop="removeTag(tagName)"
                >
                  <X :size="12" />
                </button>
              </span>
            </template>
            <small v-else class="text-light">{{ t('chat.noTags') }}</small>
            <span v-if="isUpdatingTags" role="status" aria-busy="true" data-spinner="small" />
          </div>
        </section>

        <!-- Contact Metadata -->
        <div v-if="hasMetadata">
          <!-- General section: top-level primitives -->
          <MetadataSection
            v-if="metadataPrimitives.length > 0"
            :label="t('chat.general')"
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
        <div v-if="!props.sessionData || sortedSections.length === 0" class="vstack gap-1 items-center align-center text-light p-4">
          <User :size="32" />
          <small>{{ t('chat.noPanelData') }}</small>
          <small>{{ t('chat.noPanelDataHint') }}</small>
        </div>

        <!-- Session Data with panel config -->
        <template v-else>
          <!-- Flow Name Badge -->
          <div v-if="props.sessionData?.flow_name">
            <span class="badge outline">{{ props.sessionData?.flow_name }}</span>
          </div>

          <!-- Dynamic Sections -->
          <template v-for="section in sortedSections" :key="section.id">
            <details
              v-if="section.collapsible"
              :open="!isSectionCollapsed(section.id)"
              @toggle="onSectionToggle(section.id, $event)"
            >
              <summary>{{ section.label }}</summary>
              <div class="row gap-2">
                <div v-for="field in sortedFields(section)" :key="field.key" :class="section.columns === 2 ? 'col-6' : 'col-12'">
                  <small class="text-light">{{ field.label }}</small>
                  <div v-if="field.display_type === 'badge' || field.display_type === 'tag'">
                    <span class="badge" :data-variant="getBadgeVariant(field.color)">{{ getFieldValue(field.key) }}</span>
                  </div>
                  <p v-else><strong>{{ getFieldValue(field.key) }}</strong></p>
                </div>
              </div>
            </details>

            <!-- Non-collapsible section -->
            <section v-else class="vstack gap-2">
              <strong>{{ section.label }}</strong>
              <div class="row gap-2">
                <div v-for="field in sortedFields(section)" :key="field.key" :class="section.columns === 2 ? 'col-6' : 'col-12'">
                  <small class="text-light">{{ field.label }}</small>
                  <div v-if="field.display_type === 'badge' || field.display_type === 'tag'">
                    <span class="badge" :data-variant="getBadgeVariant(field.color)">{{ getFieldValue(field.key) }}</span>
                  </div>
                  <p v-else><strong>{{ getFieldValue(field.key) }}</strong></p>
                </div>
              </div>
            </section>
          </template>
        </template>
      </div>
    </article>
  </div>
</template>
