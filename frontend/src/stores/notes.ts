import { defineStore } from 'pinia'
import { ref } from 'vue'
import { notesService, type ConversationNote } from '@/services/api'

export const useNotesStore = defineStore('notes', () => {
  const notes = ref<ConversationNote[]>([])
  const isLoading = ref(false)
  const isLoadingOlder = ref(false)
  const hasMore = ref(false)
  const currentContactId = ref<string | null>(null)

  // Helper: append a note only if it doesn't already exist
  function pushIfNew(note: ConversationNote) {
    if (!notes.value.some(n => n.id === note.id)) {
      notes.value.push(note)
    }
  }

  async function fetchNotes(contactId: string) {
    isLoading.value = true
    currentContactId.value = contactId
    try {
      const response = await notesService.list(contactId, { limit: 30 })
      const data = (response.data as any).data || response.data
      notes.value = data.notes || []
      hasMore.value = data.has_more ?? false
    } catch {
      notes.value = []
      hasMore.value = false
    } finally {
      isLoading.value = false
    }
  }

  async function fetchOlderNotes(contactId: string) {
    if (isLoadingOlder.value || !hasMore.value || notes.value.length === 0) return
    isLoadingOlder.value = true
    try {
      const oldestNote = notes.value[0]
      const response = await notesService.list(contactId, { limit: 30, before: oldestNote.id })
      const data = (response.data as any).data || response.data
      const olderNotes: ConversationNote[] = data.notes || []
      if (olderNotes.length > 0) {
        notes.value = [...olderNotes, ...notes.value]
      }
      hasMore.value = data.has_more ?? false
    } catch {
      // ignore
    } finally {
      isLoadingOlder.value = false
    }
  }

  async function createNote(contactId: string, content: string) {
    const response = await notesService.create(contactId, { content })
    const note: ConversationNote = (response.data as any).data || response.data
    pushIfNew(note)
    return note
  }

  async function updateNote(contactId: string, noteId: string, content: string) {
    const response = await notesService.update(contactId, noteId, { content })
    const updated: ConversationNote = (response.data as any).data || response.data
    const index = notes.value.findIndex(n => n.id === noteId)
    if (index !== -1) {
      notes.value[index] = updated
    }
    return updated
  }

  async function deleteNote(contactId: string, noteId: string) {
    await notesService.delete(contactId, noteId)
    notes.value = notes.value.filter(n => n.id !== noteId)
  }

  // WebSocket event handlers
  function addNote(note: ConversationNote) {
    pushIfNew(note)
  }

  function onNoteUpdated(note: ConversationNote) {
    const index = notes.value.findIndex(n => n.id === note.id)
    if (index !== -1) {
      notes.value[index] = note
    }
  }

  function onNoteDeleted(noteId: string) {
    notes.value = notes.value.filter(n => n.id !== noteId)
  }

  function clearNotes() {
    notes.value = []
    hasMore.value = false
    currentContactId.value = null
  }

  return {
    notes,
    isLoading,
    isLoadingOlder,
    hasMore,
    currentContactId,
    fetchNotes,
    fetchOlderNotes,
    createNote,
    updateNote,
    deleteNote,
    addNote,
    onNoteUpdated,
    onNoteDeleted,
    clearNotes
  }
})
