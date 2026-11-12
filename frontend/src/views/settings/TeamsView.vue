<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { PageHeader, SearchInput, DataTable, DeleteConfirmDialog, IconButton, ErrorState, type Column } from '@/components/shared'
import { useTeamsStore } from '@/stores/teams'
import { useAuthStore } from '@/stores/auth'
import { useOrganizationsStore } from '@/stores/organizations'
import { type Team } from '@/services/api'
import { toast } from '@/lib/toast'
import { Plus, Pencil, Trash2, Users, RotateCcw, Scale, Hand } from '@/lib/icons'
import { getErrorMessage } from '@/lib/api-utils'
import { debounce, formatDate } from '@/lib/utils'
import { ASSIGNMENT_STRATEGIES, getLabelFromValue } from '@/lib/constants'

const { t } = useI18n()

const teamsStore = useTeamsStore()
const authStore = useAuthStore()
const organizationsStore = useOrganizationsStore()

const teams = ref<Team[]>([])
const searchQuery = ref('')
const isLoading = ref(true)
const fetchError = ref(false)

// Pagination
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 20

// Delete
const deleteDialogOpen = ref(false)
const teamToDelete = ref<Team | null>(null)
const isDeletingTeam = ref(false)

// Sorting
const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

const canWriteTeams = computed(() => authStore.hasPermission('teams', 'write'))
const canDeleteTeams = computed(() => authStore.hasPermission('teams', 'delete'))
const breadcrumbs = computed(() => [{ label: t('nav.settings'), href: '/settings' }, { label: t('nav.teams') }])

const debouncedSearch = debounce(() => {
  currentPage.value = 1
  fetchTeams()
}, 300)

watch(searchQuery, () => debouncedSearch())

const columns = computed<Column<Team>[]>(() => [
  { key: 'team', label: t('teams.team'), sortable: true, sortKey: 'name' },
  { key: 'strategy', label: t('teams.strategy'), sortable: true, sortKey: 'assignment_strategy' },
  { key: 'members', label: t('teams.members'), sortable: true, sortKey: 'member_count' },
  { key: 'status', label: t('teams.status'), sortable: true, sortKey: 'is_active' },
  { key: 'created', label: t('teams.created'), sortable: true, sortKey: 'created_at' },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

function getStrategyLabel(strategy: string): string { return getLabelFromValue(ASSIGNMENT_STRATEGIES, strategy) }
function getStrategyIcon(strategy: string) { return { round_robin: RotateCcw, load_balanced: Scale, manual: Hand }[strategy] || RotateCcw }

function handlePageChange(page: number) {
  currentPage.value = page
  fetchTeams()
}

function openDeleteDialog(team: Team) {
  teamToDelete.value = team
  deleteDialogOpen.value = true
}

watch(() => organizationsStore.selectedOrgId, () => fetchTeams())
onMounted(() => fetchTeams())

async function fetchTeams() {
  isLoading.value = true
  fetchError.value = false
  try {
    const response = await teamsStore.fetchTeams({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    teams.value = response.teams
    totalItems.value = response.total
  } catch {
    fetchError.value = true
    toast.error(t('common.failedLoad', { resource: t('resources.teams') }))
  } finally {
    isLoading.value = false
  }
}

async function confirmDelete() {
  if (!teamToDelete.value) return
  isDeletingTeam.value = true
  try {
    await teamsStore.deleteTeam(teamToDelete.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.Team') }))
    deleteDialogOpen.value = false
    teamToDelete.value = null
    await fetchTeams()
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.team') })))
  } finally {
    isDeletingTeam.value = false
  }
}
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader :title="$t('teams.title')" :icon="Users" back-link="/settings" :breadcrumbs="breadcrumbs">
      <template #actions>
        <RouterLink v-if="canWriteTeams" to="/settings/teams/new" class="button outline small"><Plus :size="16" />{{ $t('teams.addTeam') }}</RouterLink>
      </template>
    </PageHeader>

    <ErrorState
      v-if="fetchError && !isLoading"
      :title="$t('teams.fetchErrorTitle')"
      :description="$t('teams.fetchErrorDescription')"
      :retry-label="$t('common.retry')"
      class="flex-1"
      @retry="fetchTeams"
    />

    <div v-else class="flex-1 scroll-y p-4">
      <article class="card">
        <header class="hstack justify-between flex-wrap gap-4">
          <h3>{{ $t('teams.yourTeams') }}</h3>
          <SearchInput v-model="searchQuery" :placeholder="$t('teams.searchTeams') + '...'" />
        </header>
        <DataTable :items="teams" :columns="columns" :is-loading="isLoading" :empty-icon="Users" :empty-title="searchQuery ? $t('teams.noMatchingTeams') : $t('teams.noTeamsYet')" :empty-description="searchQuery ? $t('teams.noMatchingTeamsDesc') : $t('teams.noTeamsYetDesc')" v-model:sort-key="sortKey" v-model:sort-direction="sortDirection" server-pagination :current-page="currentPage" :total-items="totalItems" :page-size="pageSize" item-name="teams" @page-change="handlePageChange">
          <template #empty-action>
            <RouterLink v-if="canWriteTeams" to="/settings/teams/new" class="button outline small"><Plus :size="16" />{{ $t('teams.addTeam') }}</RouterLink>
          </template>
          <template #cell-team="{ item: team }">
            <RouterLink :to="`/settings/teams/${team.id}`" class="hstack gap-2">
              <Users :size="20" />
              <strong class="truncate">{{ team.name }}</strong>
            </RouterLink>
          </template>
          <template #cell-strategy="{ item: team }">
            <div class="hstack gap-2">
              <component :is="getStrategyIcon(team.assignment_strategy)" :size="16" />
              <span>{{ getStrategyLabel(team.assignment_strategy) }}</span>
            </div>
          </template>
          <template #cell-members="{ item: team }">
            <span class="badge outline">{{ team.member_count || 0 }}</span>
          </template>
          <template #cell-status="{ item: team }">
            <span class="badge" :data-variant="team.is_active ? 'success' : 'secondary'">{{ team.is_active ? $t('common.active') : $t('common.inactive') }}</span>
          </template>
          <template #cell-created="{ item: team }">
            <span class="text-light">{{ formatDate(team.created_at) }}</span>
          </template>
          <template #cell-actions="{ item: team }">
            <div class="hstack gap-1 justify-end">
              <RouterLink :to="`/settings/teams/${team.id}`" class="button ghost icon" :title="$t('teams.editTeamTooltip')" :aria-label="$t('teams.editTeamTooltip')"><Pencil :size="16" /></RouterLink>
              <IconButton v-if="canDeleteTeams" :icon="Trash2" :label="$t('teams.deleteTeamTooltip')" @click="openDeleteDialog(team)" />
            </div>
          </template>
        </DataTable>
      </article>
    </div>

    <DeleteConfirmDialog v-model:open="deleteDialogOpen" :title="$t('teams.deleteTeam')" :item-name="teamToDelete?.name" :description="$t('teams.deleteTeamWarning')" :is-submitting="isDeletingTeam" @confirm="confirmDelete" />
  </div>
</template>
