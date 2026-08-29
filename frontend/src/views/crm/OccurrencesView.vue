<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Badge } from '@/components/ui/badge'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { PageHeader, SearchInput, DataTable, ErrorState, type Column } from '@/components/shared'
import { useOccurrencesStore } from '@/stores/occurrences'
import type { Occurrence } from '@/services/api'
import { formatDate } from '@/lib/utils'
import { getErrorMessage } from '@/lib/api-utils'
import { toast } from 'vue-sonner'
import { useSearchPagination } from '@/composables/useSearchPagination'
import { ClipboardList, List, LayoutGrid } from 'lucide-vue-next'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { useOccurrenceViewMode } from '@/composables/useOccurrenceViewMode'
import OccurrenceBoard from '@/components/crm/OccurrenceBoard.vue'

const { t } = useI18n()
const router = useRouter()
const store = useOccurrencesStore()
const { mode } = useOccurrenceViewMode()

const stageFilter = ref('all')
const error = ref(false)

const columns = computed<Column<Occurrence>[]>(() => [
  { key: 'protocol_number', label: t('occurrences.columnProtocol') },
  { key: 'title', label: t('occurrences.columnTitle') },
  { key: 'contact_name', label: t('occurrences.columnContact') },
  { key: 'stage_name', label: t('occurrences.columnStage') },
  { key: 'assigned_user_name', label: t('occurrences.columnAssignee') },
  { key: 'opened_at', label: t('occurrences.columnOpenedAt') },
])

async function fetchOccurrences() {
  error.value = false
  try {
    await store.fetchOccurrences({
      protocol: searchQuery.value || undefined,
      stage_id: stageFilter.value !== 'all' ? stageFilter.value : undefined,
      page: String(currentPage.value),
      limit: String(pageSize),
    } as Record<string, string>)
    totalItems.value = store.total
  } catch (e) {
    error.value = true
    toast.error(getErrorMessage(e, t('common.failedLoad', { resource: t('resources.occurrences') })))
  }
}

const { searchQuery, currentPage, totalItems, pageSize, handlePageChange } = useSearchPagination({
  fetchFn: fetchOccurrences,
})

function onStageFilterChange() {
  currentPage.value = 1
  fetchOccurrences()
}

function goToDetail(occurrence: Occurrence) {
  router.push({ name: 'occurrence-detail', params: { id: occurrence.id } })
}

onMounted(async () => {
  await store.fetchStages()
  await fetchOccurrences()
})
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader :title="$t('occurrences.title')" :description="$t('occurrences.subtitle')" :icon="ClipboardList" icon-gradient="bg-gradient-to-br from-violet-500 to-purple-600 shadow-violet-500/20" />

    <ErrorState
      v-if="error"
      :title="$t('common.loadErrorTitle')"
      :description="$t('common.loadErrorDescription')"
      :retry-label="$t('common.retryLoad')"
      class="flex-1"
      @retry="fetchOccurrences"
    />

    <ScrollArea v-else orientation="vertical" class="flex-1">
      <div class="p-6">
        <Card>
          <CardHeader>
            <div class="flex items-center justify-between flex-wrap gap-4">
              <CardTitle>{{ $t('occurrences.title') }}</CardTitle>
              <div class="flex items-center gap-2">
                <SearchInput v-model="searchQuery" :placeholder="$t('occurrences.searchPlaceholder')" class="w-64" />
                <Select v-if="mode === 'list'" v-model="stageFilter" @update:model-value="onStageFilterChange">
                  <SelectTrigger id="occurrences-stage-filter" class="w-48">
                    <SelectValue :placeholder="$t('occurrences.filterByStage')" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">{{ $t('occurrences.allStages') }}</SelectItem>
                    <SelectItem v-for="stage in store.stages" :key="stage.id" :value="stage.id">
                      {{ stage.name }}
                    </SelectItem>
                  </SelectContent>
                </Select>
                <ToggleGroup v-model="mode" type="single" :aria-label="$t('occurrences.viewModeLabel')">
                  <ToggleGroupItem value="list" :aria-label="$t('occurrences.viewList')">
                    <List class="h-4 w-4 mr-1.5" />
                    {{ $t('occurrences.viewList') }}
                  </ToggleGroupItem>
                  <ToggleGroupItem value="board" :aria-label="$t('occurrences.viewBoard')">
                    <LayoutGrid class="h-4 w-4 mr-1.5" />
                    {{ $t('occurrences.viewBoard') }}
                  </ToggleGroupItem>
                </ToggleGroup>
              </div>
            </div>
          </CardHeader>
          <CardContent>
            <div v-if="mode === 'list'" id="occurrences-list">
              <DataTable
                :items="store.occurrences"
                :columns="columns"
                :is-loading="store.isLoading"
                :empty-icon="ClipboardList"
                :empty-title="searchQuery || stageFilter !== 'all' ? $t('occurrences.noMatchingOccurrences') : $t('occurrences.noOccurrencesYet')"
                :empty-description="searchQuery || stageFilter !== 'all' ? $t('occurrences.noMatchingOccurrencesDesc') : $t('occurrences.noOccurrencesYetDesc')"
                server-pagination
                :current-page="currentPage"
                :total-items="totalItems"
                :page-size="pageSize"
                item-name="occurrences"
                @page-change="handlePageChange"
              >
                <template #cell-protocol_number="{ item: occ }">
                  <div class="cursor-pointer font-mono text-sm" @click="goToDetail(occ)">
                    {{ occ.protocol_number }}
                  </div>
                </template>
                <template #cell-title="{ item: occ }">
                  <div class="cursor-pointer font-medium truncate max-w-[240px]" @click="goToDetail(occ)">
                    {{ occ.title }}
                  </div>
                </template>
                <template #cell-contact_name="{ item: occ }">
                  <div class="cursor-pointer" @click="goToDetail(occ)">
                    {{ occ.contact_name }}
                  </div>
                </template>
                <template #cell-stage_name="{ item: occ }">
                  <div class="cursor-pointer" @click="goToDetail(occ)">
                    <Badge
                      variant="outline"
                      :style="{ borderColor: store.stageColor(occ.stage_id), color: store.stageColor(occ.stage_id) }"
                    >{{ occ.stage_name }}</Badge>
                  </div>
                </template>
                <template #cell-assigned_user_name="{ item: occ }">
                  <div class="cursor-pointer text-muted-foreground" @click="goToDetail(occ)">
                    {{ occ.assigned_user_name || $t('occurrences.unassigned') }}
                  </div>
                </template>
                <template #cell-opened_at="{ item: occ }">
                  <div class="cursor-pointer text-muted-foreground" @click="goToDetail(occ)">
                    {{ formatDate(occ.opened_at) }}
                  </div>
                </template>
              </DataTable>
            </div>
            <OccurrenceBoard v-else :protocol="searchQuery || undefined" />
          </CardContent>
        </Card>
      </div>
    </ScrollArea>
  </div>
</template>
