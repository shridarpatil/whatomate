<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { watchDebounced } from '@vueuse/core'
import { useRouter } from 'vue-router'
import draggable from 'vuedraggable'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { getErrorMessage } from '@/lib/api-utils'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import { useOccurrencesStore } from '@/stores/occurrences'
import type { Occurrence, OccurrenceStage } from '@/services/api'
import OccurrenceCard from './OccurrenceCard.vue'

const props = defineProps<{ protocol?: string }>()

const store = useOccurrencesStore()
const router = useRouter()
const { t } = useI18n()

// A lista já navega ao clicar numa linha (OccurrencesView.vue); o quadro
// precisa do mesmo caminho para o cartão abrir o detalhe.
function goToDetail(occurrence: Occurrence) {
  // Um cartão travado (requisição em voo) não navega: o detalhe renderizaria
  // a etapa antiga enquanto o PUT ainda está mudando ela por baixo.
  if (pending.value.has(occurrence.id)) return
  router.push({ name: 'occurrence-detail', params: { id: occurrence.id } })
}

/** Ocorrências com requisição em voo. Um cartão travado não aceita novo arrasto. */
const pending = ref(new Set<string>())

/**
 * A origem do arrasto, capturada no início e usada na reversão. Guardar isto
 * explicitamente é o que permite desfazer sem depender do estado visual.
 */
let dragOrigin: { occurrenceId: string; fromStageId: string } | null = null

/** Cartões por página, em cada coluna. */
const PAGE_SIZE = 25
/** Janela da coluna de fechamento. Constante desta fase, não configuração. */
const CLOSED_WINDOW_DAYS = 7

interface ColumnState {
  stage: OccurrenceStage
  items: Occurrence[]
  total: number
  page: number
  loading: boolean
  failed: boolean
}

const columns = ref<ColumnState[]>([])

/**
 * Etapa normal mostra as abertas; etapa de fechamento mostra as fechadas na
 * janela recente. Nunca use `open=false` para a segunda: o handler testa
 * `open == "true"`, então `false` significa *sem filtro* e traria tudo.
 *
 * O corte vai absoluto, em RFC3339, calculado aqui — o backend não sabe o que
 * são "7 dias", ele recebe uma data e compara.
 */
function columnParams(stage: OccurrenceStage, page: number): Record<string, string> {
  const params: Record<string, string> = {
    stage_id: stage.id,
    page: String(page),
    limit: String(PAGE_SIZE),
  }

  if (stage.is_closing) {
    params.closed_since = new Date(Date.now() - CLOSED_WINDOW_DAYS * 24 * 60 * 60 * 1000).toISOString()
  } else {
    params.open = 'true'
  }

  if (props.protocol) params.protocol = props.protocol

  return params
}

async function loadColumn(col: ColumnState, page: number) {
  if (col.loading) return // reentrancy guard: blocks a double-click from double-appending the same page
  col.loading = true
  col.failed = false
  try {
    const { occurrences, total } = await store.fetchColumn(columnParams(col.stage, page))
    col.items = page === 1 ? occurrences : [...col.items, ...occurrences]
    col.total = total
    col.page = page
  } catch {
    // Falha é isolada por coluna: esta avisa, as outras seguem funcionando.
    col.failed = true
  } finally {
    col.loading = false
  }
}

/**
 * Dispara todas as colunas juntas. O tempo de abertura é o da chamada mais
 * lenta, não a soma. `loadColumn` trata o próprio erro, então o Promise.all
 * nunca rejeita — é isso que mantém o isolamento por coluna.
 */
async function loadAll() {
  columns.value = store.stages.map(stage => ({
    stage,
    items: [],
    total: 0,
    page: 1,
    loading: false, // loadColumn sets this synchronously below, before Vue paints
    failed: false,
  }))
  await Promise.all(columns.value.map(col => loadColumn(col, 1)))
}

function hasMore(col: ColumnState): boolean {
  return col.items.length < col.total
}

function onDragStart(col: ColumnState, evt: { oldIndex: number }) {
  const item = col.items[evt.oldIndex]
  dragOrigin = item ? { occurrenceId: item.id, fromStageId: col.stage.id } : null
}

/** Recusa arrastar um cartão cuja própria requisição ainda está em voo. */
function canMove(evt: { draggedContext: { element: Occurrence } }): boolean {
  return !pending.value.has(evt.draggedContext.element.id)
}

function sortByOpenedAtDesc(items: Occurrence[]) {
  items.sort((a, b) => Date.parse(b.opened_at) - Date.parse(a.opened_at))
}

async function onColumnChange(toCol: ColumnState, evt: { added?: { element: Occurrence } }) {
  // Mover dentro da mesma coluna emite `moved`, não `added`: nenhuma
  // requisição sai e nenhum evento de timeline é criado. A ordem, porém,
  // nunca é manual — nem no código nem no estado — então a coluna volta a
  // ficar por opened_at DESC mesmo sem chamada ao servidor.
  if (!evt.added) {
    sortByOpenedAtDesc(toCol.items)
    return
  }

  const origin = dragOrigin
  dragOrigin = null
  if (!origin) return

  const occ = evt.added.element

  // Guarda explícita do no-op, exigida pela spec e não deixada por conta do
  // comportamento da biblioteca.
  if (origin.fromStageId === toCol.stage.id) return

  const fromCol = columns.value.find(c => c.stage.id === origin.fromStageId)

  pending.value.add(occ.id)
  try {
    await store.moveStage(occ.id, toCol.stage.id)
    sortByOpenedAtDesc(toCol.items)
    // Só ajusta as contagens em caso de sucesso. Durante a requisição, o
    // destino mostra N cartões sob um cabeçalho com N-1 — deliberado: é o
    // que permite a reversão do catch abaixo dispensar desfazer contagem.
    toCol.total += 1
    if (fromCol) fromCol.total = Math.max(0, fromCol.total - 1)
  } catch (e) {
    // Reversão pela origem guardada, não pelo que está na tela.
    const idx = toCol.items.findIndex(i => i.id === occ.id)
    if (idx !== -1) toCol.items.splice(idx, 1)
    if (fromCol) {
      fromCol.items.push(occ)
      sortByOpenedAtDesc(fromCol.items)
    }
    // A mensagem é a que o servidor devolveu, não um erro genérico.
    toast.error(getErrorMessage(e, t('occurrences.stageChangeFailed')))
  } finally {
    pending.value.delete(occ.id)
  }
}

onMounted(async () => {
  if (store.stages.length === 0) await store.fetchStages()
  await loadAll()
})

// O quadro espalha uma única busca em N requisições (uma por coluna), então um
// watch sem debounce multiplica cada tecla digitada pelo número de etapas.
// 300ms para bater com o debounce que useSearchPagination já usa na lista.
watchDebounced(() => props.protocol, loadAll, { debounce: 300 })
</script>

<template>
  <div id="occurrences-board" class="flex gap-4 overflow-x-auto p-4">
    <div
      v-for="col in columns"
      :key="col.stage.id"
      :data-board-column="col.stage.name"
      class="flex w-72 shrink-0 flex-col rounded-lg border border-white/[0.08] light:border-gray-200 bg-white/[0.02] light:bg-gray-50"
    >
      <div class="flex items-center justify-between gap-2 border-b border-white/[0.08] light:border-gray-200 p-3">
        <span class="flex items-center gap-2 text-sm font-medium">
          <span class="h-2.5 w-2.5 rounded-full" :style="{ backgroundColor: col.stage.color }" />
          {{ col.stage.name }}
        </span>
        <span class="text-xs text-muted-foreground">{{ col.total }}</span>
      </div>

      <div class="flex flex-1 flex-col gap-2 p-2 min-h-24">
        <div v-if="col.failed" class="p-3 text-center">
          <p class="text-xs text-muted-foreground">{{ $t('occurrences.columnLoadFailed') }}</p>
          <Button variant="outline" size="sm" class="mt-2" @click="loadColumn(col, 1)">
            {{ $t('common.retryLoad') }}
          </Button>
        </div>

        <template v-else>
          <draggable
            v-model="col.items"
            :group="{ name: 'occurrences' }"
            :move="canMove"
            item-key="id"
            class="flex flex-1 flex-col gap-2 min-h-16"
            @start="onDragStart(col, $event)"
            @change="onColumnChange(col, $event)"
            @end="dragOrigin = null"
          >
            <template #item="{ element }">
              <OccurrenceCard :occurrence="element" :disabled="pending.has(element.id)" @click="goToDetail(element)" />
            </template>
          </draggable>

          <div v-if="col.loading" class="flex justify-center p-3">
            <Spinner class="h-4 w-4" />
          </div>

          <p v-else-if="col.items.length === 0" class="p-3 text-center text-xs text-muted-foreground">
            {{ $t('occurrences.columnEmpty') }}
          </p>

          <Button
            v-if="hasMore(col) && !col.loading"
            variant="ghost"
            size="sm"
            @click="loadColumn(col, col.page + 1)"
          >
            {{ $t('occurrences.loadMore') }}
          </Button>
        </template>
      </div>
    </div>
  </div>
</template>
