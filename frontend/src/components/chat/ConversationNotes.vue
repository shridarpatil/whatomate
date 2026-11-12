<script setup lang="ts">
import { ref, computed, watch, onMounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useNotesStore } from '@/stores/notes'
import { useAuthStore } from '@/stores/auth'
import { toast } from '@/lib/toast'
import { useInfiniteScroll } from '@/composables/useInfiniteScroll'
import { getInitials } from '@/lib/utils'
import {
  StickyNote, Pencil, Trash2, X, Check, Send
} from '@/lib/icons'

const props = defineProps<{
  contactId: string
}>()

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()
const notesStore = useNotesStore()
const authStore = useAuthStore()

const newNoteContent = ref('')
const editingNoteId = ref<string | null>(null)
const editingContent = ref('')
const isSaving = ref(false)
const notesEndRef = ref<HTMLElement | null>(null)

// Infinite scroll for older notes (scroll up to load more)
const notesScroll = useInfiniteScroll({
  direction: 'top',
  onLoadMore: async () => {
    await notesScroll.preserveScrollPosition(async () => {
      await notesStore.fetchOlderNotes(props.contactId)
      await nextTick()
    })
  },
  hasMore: computed(() => notesStore.hasMore),
  isLoading: computed(() => notesStore.isLoadingOlder)
})

function scrollToBottom(instant = false) {
  nextTick(() => {
    if (notesEndRef.value) {
      notesEndRef.value.scrollIntoView({
        behavior: instant ? 'instant' : 'smooth',
        block: 'end'
      })
    }
  })
}

onMounted(async () => {
  if (props.contactId) {
    // Only fetch if not already loaded for this contact (ChatView pre-fetches for badge count)
    if (notesStore.currentContactId !== props.contactId) {
      await notesStore.fetchNotes(props.contactId)
    }
    await nextTick()
    // Delay setup like messages do to ensure the list is fully rendered
    setTimeout(() => {
      scrollToBottom(true)
      notesScroll.setup()
    }, 50)
  }
})

watch(() => props.contactId, async (newId) => {
  if (newId) {
    notesScroll.cleanup()
    if (notesStore.currentContactId !== newId) {
      await notesStore.fetchNotes(newId)
    }
    await nextTick()
    setTimeout(() => {
      scrollToBottom(true)
      notesScroll.setup()
    }, 50)
  }
})

// Auto-scroll when new notes are added at the bottom
watch(() => notesStore.notes.length, (_newLen, oldLen) => {
  if (oldLen !== undefined && _newLen > oldLen) {
    scrollToBottom()
  }
})

async function addNote() {
  if (!newNoteContent.value.trim()) return
  isSaving.value = true
  try {
    await notesStore.createNote(props.contactId, newNoteContent.value.trim())
    newNoteContent.value = ''
    toast.success(t('chat.noteAdded'))
    scrollToBottom()
  } catch {
    toast.error(t('chat.noteAddFailed'))
  } finally {
    isSaving.value = false
  }
}

function startEditing(noteId: string, content: string) {
  editingNoteId.value = noteId
  editingContent.value = content
}

function cancelEditing() {
  editingNoteId.value = null
  editingContent.value = ''
}

async function saveEdit(noteId: string) {
  if (!editingContent.value.trim()) return
  isSaving.value = true
  try {
    await notesStore.updateNote(props.contactId, noteId, editingContent.value.trim())
    editingNoteId.value = null
    editingContent.value = ''
    toast.success(t('chat.noteUpdated'))
  } catch {
    toast.error(t('chat.noteUpdateFailed'))
  } finally {
    isSaving.value = false
  }
}

async function deleteNote(noteId: string) {
  if (!confirm(t('chat.confirmDeleteNote'))) return
  try {
    await notesStore.deleteNote(props.contactId, noteId)
    toast.success(t('chat.noteDeleted'))
  } catch {
    toast.error(t('chat.noteDeleteFailed'))
  }
}

function formatNoteTime(dateStr: string) {
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMins = Math.floor(diffMs / 60000)
  const diffHours = Math.floor(diffMs / 3600000)
  const diffDays = Math.floor(diffMs / 86400000)

  if (diffMins < 1) return 'Just now'
  if (diffMins < 60) return `${diffMins}m ago`
  if (diffHours < 24) return `${diffHours}h ago`
  if (diffDays < 7) return `${diffDays}d ago`
  return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}
</script>

<template>
  <article id="notes-panel" class="card p-4 flex flex-col min-h-0 h-full gap-4 chat-col">
    <!-- Header -->
    <header class="hstack justify-between shrink-0">
      <div class="hstack gap-2">
        <StickyNote class="text-light" />
        <h4>{{ t('chat.internalNotes') }}</h4>
        <span v-if="notesStore.notes.length > 0" class="badge" data-variant="warning">{{ notesStore.notes.length }}</span>
      </div>
      <button type="button" class="ghost small icon" :title="t('common.close')" :aria-label="t('common.close')" @click="emit('close')">
        <X />
      </button>
    </header>

    <!-- Notes list -->
    <div :ref="(el: any) => notesScroll.scrollAreaRef.value = el" class="flex-1 scroll-y vstack gap-2">
      <!-- Loading older notes -->
      <div v-if="notesStore.isLoadingOlder" class="flex justify-center">
        <span role="status" aria-busy="true" data-spinner="small" :aria-label="t('common.loading')" />
      </div>

      <!-- Initial loading state -->
      <div v-if="notesStore.isLoading" class="flex justify-center p-4">
        <span role="status" aria-busy="true" :aria-label="t('common.loading')" />
      </div>

      <!-- Notes (chronological: oldest first, latest last) -->
      <template v-else-if="notesStore.notes.length > 0">
        <article
          v-for="note in notesStore.notes"
          :key="note.id"
          data-note
          data-hover-group
          class="card p-4 shrink-0"
        >
          <!-- Editing mode -->
          <div v-if="editingNoteId === note.id" class="vstack gap-2">
            <textarea
              v-model="editingContent"
              :aria-label="t('common.edit')"
              rows="2"
              @keydown.meta.enter.prevent="saveEdit(note.id)"
              @keydown.ctrl.enter.prevent="saveEdit(note.id)"
            />
            <div class="hstack gap-2 justify-end">
              <button type="button" class="ghost small" @click="cancelEditing">{{ t('common.cancel') }}</button>
              <button
                type="button"
                class="small"
                :disabled="!editingContent.trim() || isSaving"
                :aria-busy="isSaving ? 'true' : undefined"
                data-spinner="small"
                @click="saveEdit(note.id)"
              >
                <Check v-if="!isSaving" />
                {{ t('common.save') }}
              </button>
            </div>
          </div>

          <!-- Display mode -->
          <div v-else class="flex gap-2">
            <figure data-variant="avatar" class="small shrink-0">
              <abbr>{{ getInitials(note.created_by_name) }}</abbr>
            </figure>
            <div class="flex-1 min-w-0">
              <div class="flex items-center justify-between gap-2">
                <small class="truncate"><strong>{{ note.created_by_name }}</strong></small>
                <div class="hstack gap-1 shrink-0">
                  <!-- Hover actions (own notes only) -->
                  <template v-if="note.created_by_id === authStore.user?.id">
                    <button
                      type="button"
                      class="ghost small icon"
                      data-hover-reveal
                      :title="t('common.edit')"
                      :aria-label="t('common.edit')"
                      @click="startEditing(note.id, note.content)"
                    >
                      <Pencil :size="12" />
                    </button>
                    <button
                      type="button"
                      class="ghost small icon"
                      data-hover-reveal
                      :title="t('common.delete')"
                      :aria-label="t('common.delete')"
                      @click="deleteNote(note.id)"
                    >
                      <Trash2 :size="12" />
                    </button>
                  </template>
                  <small class="text-lighter">{{ formatNoteTime(note.created_at) }}</small>
                </div>
              </div>
              <p class="pre-wrap text-light">{{ note.content }}</p>
            </div>
          </div>
        </article>
      </template>

      <!-- Empty state -->
      <div v-else class="vstack gap-1 items-center align-center text-light p-4">
        <StickyNote :size="24" />
        <strong>{{ t('chat.noNotes') }}</strong>
        <small>{{ t('chat.writeNote') }}</small>
      </div>

      <!-- Scroll anchor -->
      <div ref="notesEndRef" />
    </div>

    <!-- Add note input -->
    <div class="flex items-center gap-2 shrink-0">
      <textarea
        v-model="newNoteContent"
        class="min-h-0"
        :placeholder="t('chat.writeNote') + '...'"
        :aria-label="t('chat.writeNote')"
        rows="1"
        @keydown.enter.exact.prevent="addNote"
      />
      <button
        type="button"
        class="icon shrink-0"
        :disabled="!newNoteContent.trim() || isSaving"
        :aria-busy="isSaving ? 'true' : undefined"
        data-spinner="small"
        :title="t('chat.addNote')"
        :aria-label="t('chat.addNote')"
        @click="addNote"
      >
        <Send v-if="!isSaving" />
      </button>
    </div>
  </article>
</template>
