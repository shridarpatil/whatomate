<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import { useOccurrencesStore } from '@/stores/occurrences'
import type { Occurrence, OccurrenceStage } from '@/services/api'
import OccurrenceCard from './OccurrenceCard.vue'

const props = defineProps<{ protocol?: string }>()

const store = useOccurrencesStore()

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
    loading: true,
    failed: false,
  }))
  await Promise.all(columns.value.map(col => loadColumn(col, 1)))
}

function hasMore(col: ColumnState): boolean {
  return col.items.length < col.total
}

onMounted(async () => {
  if (store.stages.length === 0) await store.fetchStages()
  await loadAll()
})

watch(() => props.protocol, loadAll)
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

      <div class="flex flex-col gap-2 p-2 min-h-24">
        <div v-if="col.failed" class="p-3 text-center">
          <p class="text-xs text-muted-foreground">{{ $t('occurrences.columnLoadFailed') }}</p>
          <Button variant="outline" size="sm" class="mt-2" @click="loadColumn(col, 1)">
            {{ $t('common.retryLoad') }}
          </Button>
        </div>

        <template v-else>
          <OccurrenceCard v-for="occ in col.items" :key="occ.id" :occurrence="occ" />

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
