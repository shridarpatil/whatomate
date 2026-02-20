<script setup lang="ts">
import { ref, computed, watch, onMounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useNotesStore } from '@/stores/notes'
import { useAuthStore } from '@/stores/auth'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Textarea } from '@/components/ui/textarea'
import { toast } from 'vue-sonner'
import { useInfiniteScroll } from '@/composables/useInfiniteScroll'
import { getInitials, getAvatarGradient } from '@/lib/utils'
import {
  StickyNote, Pencil, Trash2, X, Check, Loader2, Send
} from 'lucide-vue-next'

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
    // Delay setup like messages do to ensure ScrollArea is fully rendered
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
  <div id="notes-panel" class="w-80 border-l border-white/[0.08] light:border-gray-200 bg-[#111113] light:bg-white flex flex-col">
    <!-- Header -->
    <div class="px-4 py-3 border-b border-white/[0.08] light:border-gray-200 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <div class="h-7 w-7 rounded-lg bg-amber-500/15 flex items-center justify-center">
          <StickyNote class="h-4 w-4 text-amber-400 light:text-amber-600" />
        </div>
        <span class="text-sm font-semibold text-white light:text-gray-900">{{ t('chat.internalNotes') }}</span>
        <Badge v-if="notesStore.notes.length > 0" class="bg-amber-500/20 text-amber-400 light:bg-amber-100 light:text-amber-700 border-0 text-[10px] px-1.5 py-0">
          {{ notesStore.notes.length }}
        </Badge>
      </div>
      <Button
        variant="ghost"
        size="icon"
        class="h-7 w-7 text-white/40 hover:text-white hover:bg-white/[0.08] light:text-gray-500 light:hover:text-gray-900 light:hover:bg-gray-100"
        @click="emit('close')"
      >
        <X class="h-4 w-4" />
      </Button>
    </div>

    <!-- Notes list -->
    <ScrollArea :ref="(el: any) => notesScroll.scrollAreaRef.value = el" class="flex-1 p-3">
      <div class="space-y-3">
        <!-- Loading older notes -->
        <div v-if="notesStore.isLoadingOlder" class="flex justify-center py-2">
          <Loader2 class="h-4 w-4 animate-spin text-white/30 light:text-gray-400" />
        </div>

        <!-- Initial loading state -->
        <div v-if="notesStore.isLoading" class="flex justify-center py-8">
          <Loader2 class="h-5 w-5 animate-spin text-white/30 light:text-gray-400" />
        </div>

        <!-- Notes (chronological: oldest first, latest last) -->
        <template v-else-if="notesStore.notes.length > 0">
          <div
            v-for="note in notesStore.notes"
            :key="note.id"
            class="group relative rounded-xl p-3 backdrop-blur-sm border border-white/[0.06] light:border-gray-200 bg-gradient-to-br from-white/[0.04] to-white/[0.02] light:from-gray-50 light:to-white hover:from-white/[0.06] hover:to-white/[0.03] light:hover:from-gray-100 light:hover:to-gray-50 transition-all duration-200"
          >
            <!-- Gradient accent line -->
            <div class="absolute top-0 left-3 right-3 h-[2px] rounded-full bg-gradient-to-r from-amber-500/60 via-orange-500/40 to-transparent" />

            <!-- Editing mode -->
            <template v-if="editingNoteId === note.id">
              <Textarea
                v-model="editingContent"
                class="min-h-[60px] max-h-[100px] resize-none text-sm bg-white/[0.04] light:bg-gray-50 border-amber-500/20 light:border-amber-200 mt-1"
                :rows="2"
                @keydown.meta.enter.prevent="saveEdit(note.id)"
                @keydown.ctrl.enter.prevent="saveEdit(note.id)"
              />
              <div class="flex justify-end gap-1.5 mt-2">
                <Button variant="ghost" size="sm" class="h-7 text-xs" @click="cancelEditing">
                  {{ t('common.cancel') }}
                </Button>
                <Button
                  size="sm"
                  class="h-7 text-xs bg-amber-600 hover:bg-amber-500 text-white"
                  :disabled="!editingContent.trim() || isSaving"
                  @click="saveEdit(note.id)"
                >
                  <Loader2 v-if="isSaving" class="h-3 w-3 mr-1 animate-spin" />
                  <Check v-else class="h-3 w-3 mr-1" />
                  {{ t('common.save') }}
                </Button>
              </div>
            </template>

            <!-- Display mode -->
            <template v-else>
              <div class="flex items-start gap-2.5 mt-1">
                <Avatar class="h-6 w-6 shrink-0 ring-1 ring-white/[0.08] light:ring-gray-200">
                  <AvatarFallback :class="'text-[10px] bg-gradient-to-br text-white ' + getAvatarGradient(note.created_by_name)">
                    {{ getInitials(note.created_by_name) }}
                  </AvatarFallback>
                </Avatar>
                <div class="flex-1 min-w-0">
                  <div class="flex items-center justify-between mb-1">
                    <span class="text-xs font-medium text-white/70 light:text-gray-700">{{ note.created_by_name }}</span>
                    <span class="text-[10px] text-white/30 light:text-gray-400">{{ formatNoteTime(note.created_at) }}</span>
                  </div>
                  <p class="text-[13px] text-white/60 light:text-gray-600 leading-relaxed whitespace-pre-wrap break-words">{{ note.content }}</p>
                </div>
              </div>

              <!-- Hover actions (own notes only) -->
              <div
                v-if="note.created_by_id === authStore.user?.id"
                class="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity flex gap-0.5"
              >
                <button
                  class="h-6 w-6 rounded-md flex items-center justify-center hover:bg-white/[0.08] light:hover:bg-gray-200 text-white/30 hover:text-white/60 light:text-gray-400 light:hover:text-gray-600 transition-colors"
                  @click="startEditing(note.id, note.content)"
                >
                  <Pencil class="h-3 w-3" />
                </button>
                <button
                  class="h-6 w-6 rounded-md flex items-center justify-center hover:bg-red-500/10 text-white/30 hover:text-red-400 light:text-gray-400 light:hover:text-red-500 transition-colors"
                  @click="deleteNote(note.id)"
                >
                  <Trash2 class="h-3 w-3" />
                </button>
              </div>
            </template>
          </div>
        </template>

        <!-- Empty state -->
        <div v-else class="flex flex-col items-center justify-center py-12 text-center">
          <div class="h-12 w-12 rounded-xl bg-amber-500/10 light:bg-amber-50 flex items-center justify-center mb-3">
            <StickyNote class="h-6 w-6 text-amber-400/50 light:text-amber-400" />
          </div>
          <p class="text-sm font-medium text-white/40 light:text-gray-500 mb-1">{{ t('chat.noNotes') }}</p>
          <p class="text-xs text-white/25 light:text-gray-400">{{ t('chat.writeNote') }}</p>
        </div>

        <!-- Scroll anchor -->
        <div ref="notesEndRef" />
      </div>
    </ScrollArea>

    <!-- Add note input -->
    <div class="p-4 border-t border-white/[0.08] light:border-gray-200">
      <div class="flex items-center gap-2 p-2 rounded-xl bg-white/[0.06] light:bg-gray-100 border border-white/[0.08] light:border-gray-200">
        <textarea
          v-model="newNoteContent"
          :placeholder="t('chat.writeNote') + '...'"
          class="flex-1 bg-transparent text-[14px] text-white light:text-gray-900 placeholder:text-white/30 light:placeholder:text-gray-400 focus:outline-none resize-none min-h-[36px] max-h-[120px] py-2 overflow-y-auto"
          rows="1"
          @keydown.enter.exact.prevent="addNote"
        />
        <button
          class="w-9 h-9 rounded-lg bg-amber-600 hover:bg-amber-500 light:bg-amber-500 light:hover:bg-amber-600 flex items-center justify-center transition-colors disabled:opacity-50"
          :disabled="!newNoteContent.trim() || isSaving"
          @click="addNote"
        >
          <Loader2 v-if="isSaving" class="h-4 w-4 animate-spin text-white" />
          <Send v-else class="h-4 w-4 text-white" />
        </button>
      </div>
    </div>
  </div>
</template>
