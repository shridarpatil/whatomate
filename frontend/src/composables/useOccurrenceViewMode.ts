import { ref, watch, type Ref } from 'vue'

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
export function useOccurrenceViewMode(): { mode: Ref<OccurrenceViewMode> } {
  const mode = ref<OccurrenceViewMode>('list')

  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'list' || saved === 'board') {
      mode.value = saved
    }
  } catch {
    // Entrada ausente ou bloqueada — o padrão acima já vale.
  }

  watch(mode, value => {
    try {
      localStorage.setItem(STORAGE_KEY, value)
    } catch {
      // Modo privado ou cota estourada — a preferência só não sobrevive.
    }
  })

  return { mode }
}
