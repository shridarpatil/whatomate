import { computed, ref, watch, type WritableComputedRef } from 'vue'

export type OccurrenceViewMode = 'list' | 'board'

const STORAGE_KEY = 'occurrences:view-mode'

/**
 * A preferência lista/quadro, lembrada por dispositivo. Modo de exibição é o
 * tipo de preferência que faz sentido variar entre a mesa e o celular, então
 * ela vive no localStorage e não no perfil do usuário.
 *
 * Qualquer coisa ilegível lá cai na lista, que é o modo que existia antes do
 * quadro. Leitura e escrita ficam dentro de try/catch porque em janela privada
 * o próprio acessador lança, em vez de devolver vazio.
 */
export function useOccurrenceViewMode(): { mode: WritableComputedRef<OccurrenceViewMode> } {
  const state = ref<OccurrenceViewMode>('list')

  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'list' || saved === 'board') {
      state.value = saved
    }
  } catch {
    // Entrada ausente ou bloqueada — o padrão acima já vale.
  }

  // reka-ui's single-select ToggleGroupRoot deselects the active item on a
  // repeat click, emitting `undefined` through update:modelValue. A plain
  // ref bound with v-model would take that undefined as-is, blanking the
  // toggle and falling through to the board placeholder. The setter here
  // ignores anything that isn't a real mode, leaving `state` — and whatever
  // was last persisted — untouched, so a repeat click is a no-op.
  const mode = computed<OccurrenceViewMode>({
    get: () => state.value,
    set: value => {
      if (value === 'list' || value === 'board') {
        state.value = value
      }
    },
  })

  watch(state, value => {
    try {
      localStorage.setItem(STORAGE_KEY, value)
    } catch {
      // Modo privado ou cota estourada — a preferência só não sobrevive.
    }
  })

  return { mode }
}
