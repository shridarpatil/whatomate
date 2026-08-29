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

  // O quadro carrega cada coluna de forma independente e em paralelo, então
  // isto devolve a página em vez de escrever o array `occurrences`, que a
  // lista possui. Se as colunas escrevessem lá, a última resposta a chegar
  // apagaria as outras.
  async function fetchColumn(params: Record<string, string>) {
    const res = await occurrencesService.list(params)
    return { occurrences: res.data.data.occurrences, total: res.data.data.total }
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

  // O quadro grava a etapa e para por aí. O `changeStage` acima recarrega a
  // timeline porque a tela de detalhe precisa dela; aqui isso seria uma
  // requisição desperdiçada por arrasto e ainda sobrescreveria `events`, que
  // pertence ao detalhe.
  async function moveStage(occurrenceId: string, stageId: string) {
    await occurrencesService.changeStage(occurrenceId, stageId)
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
    fetchStages, stageColor, fetchOccurrences, fetchColumn, fetchContactOccurrences, fetchEvents,
    createOccurrence, changeStage, moveStage, addNote, sendProtocol, clear,
  }
})
