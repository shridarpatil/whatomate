import { defineStore } from 'pinia'
import { ref } from 'vue'
import { occurrencesService, type Occurrence, type OccurrenceStage, type OccurrenceEvent } from '@/services/api'

export const useOccurrencesStore = defineStore('occurrences', () => {
  const occurrences = ref<Occurrence[]>([])
  const total = ref(0)
  const contactOccurrences = ref<Occurrence[]>([])
  const stages = ref<OccurrenceStage[]>([])
  const events = ref<OccurrenceEvent[]>([])
  const isLoading = ref(false)

  async function fetchStages() {
    const res = await occurrencesService.listStages()
    stages.value = res.data.data.stages
  }

  // Returns undefined for a stage that is no longer listed (removed after the
  // occurrence was created), so the badge falls back to its default outline.
  function stageColor(stageID: string): string | undefined {
    return stages.value.find(s => s.id === stageID)?.color
  }

  async function fetchOccurrences(params?: Record<string, string>) {
    isLoading.value = true
    try {
      const res = await occurrencesService.list(params)
      occurrences.value = res.data.data.occurrences
      total.value = res.data.data.total
    } finally {
      isLoading.value = false
    }
  }

  async function fetchContactOccurrences(contactId: string) {
    const res = await occurrencesService.listForContact(contactId)
    contactOccurrences.value = res.data.data.occurrences
  }

  async function fetchEvents(occurrenceId: string) {
    const res = await occurrencesService.listEvents(occurrenceId)
    events.value = res.data.data.events
  }

  async function createOccurrence(payload: {
    contact_id: string
    title: string
    description?: string
    priority?: 'low' | 'normal' | 'high' | 'urgent'
    source_transfer_id?: string
  }) {
    const res = await occurrencesService.create(payload)
    await fetchContactOccurrences(payload.contact_id)
    return res.data.data
  }

  async function changeStage(occurrenceId: string, stageId: string) {
    await occurrencesService.changeStage(occurrenceId, stageId)
    await fetchEvents(occurrenceId)
  }

  async function addNote(occurrenceId: string, content: string) {
    await occurrencesService.addNote(occurrenceId, content)
    await fetchEvents(occurrenceId)
  }

  async function sendProtocol(occurrenceId: string) {
    await occurrencesService.sendProtocol(occurrenceId)
    await fetchEvents(occurrenceId)
  }

  function clear() {
    occurrences.value = []
    total.value = 0
    contactOccurrences.value = []
    events.value = []
  }

  return {
    occurrences, total, contactOccurrences, stages, events, isLoading,
    fetchStages, stageColor, fetchOccurrences, fetchContactOccurrences, fetchEvents,
    createOccurrence, changeStage, addNote, sendProtocol, clear,
  }
})
