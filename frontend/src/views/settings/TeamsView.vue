<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { PageHeader, SearchInput, DataTable, CrudFormDialog, DeleteConfirmDialog, type Column } from '@/components/shared'
import { useTeamsStore } from '@/stores/teams'
import { useUsersStore, type User } from '@/stores/users'
import { useAuthStore } from '@/stores/auth'
import { useOrganizationsStore } from '@/stores/organizations'
import { type Team, type TeamMember } from '@/services/api'
import { toast } from 'vue-sonner'
import { Plus, Pencil, Trash2, Loader2, Users, UserPlus, UserMinus, RotateCcw, Scale, Hand } from 'lucide-vue-next'
import { useCrudState } from '@/composables/useCrudState'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDate } from '@/lib/utils'
import { ASSIGNMENT_STRATEGIES, getLabelFromValue } from '@/lib/constants'
import { useDebounceFn } from '@vueuse/core'

const teamsStore = useTeamsStore()
const usersStore = useUsersStore()
const authStore = useAuthStore()
const organizationsStore = useOrganizationsStore()

interface TeamFormData {
  name: string
  description: string
  assignment_strategy: 'round_robin' | 'load_balanced' | 'manual'
  is_active: boolean
}

const defaultFormData: TeamFormData = { name: '', description: '', assignment_strategy: 'round_robin', is_active: true }

const {
  isLoading, isSubmitting, isDialogOpen, editingItem: editingTeam, deleteDialogOpen, itemToDelete: teamToDelete,
  formData, openCreateDialog, openEditDialog: baseOpenEditDialog, openDeleteDialog, closeDialog, closeDeleteDialog,
} = useCrudState<Team, TeamFormData>(defaultFormData)

const teams = ref<Team[]>([])
const searchQuery = ref('')

// Pagination state
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 20

// Debounced search
const debouncedSearch = useDebounceFn(() => {
  currentPage.value = 1
  fetchTeams()
}, 300)

watch(searchQuery, () => debouncedSearch())

function handlePageChange(page: number) {
  currentPage.value = page
  fetchTeams()
}

// Members dialog state
const isMembersDialogOpen = ref(false)
const selectedTeam = ref<Team | null>(null)
const teamMembers = ref<TeamMember[]>([])
const loadingMembers = ref(false)

const isAdmin = computed(() => authStore.userRole === 'admin')
const breadcrumbs = [{ label: 'Settings', href: '/settings' }, { label: 'Teams' }]

const availableUsers = computed(() => {
  const memberUserIds = new Set(teamMembers.value.map(m => m.user_id))
  return usersStore.users.filter(u => !memberUserIds.has(u.id) && u.is_active)
})

const columns: Column<Team>[] = [
  { key: 'team', label: 'Team', width: 'w-[250px]', sortable: true, sortKey: 'name' },
  { key: 'strategy', label: 'Strategy', sortable: true, sortKey: 'assignment_strategy' },
  { key: 'members', label: 'Members', sortable: true, sortKey: 'member_count' },
  { key: 'status', label: 'Status', sortable: true, sortKey: 'is_active' },
  { key: 'created', label: 'Created', sortable: true, sortKey: 'created_at' },
  { key: 'actions', label: 'Actions', align: 'right' },
]

// Sorting state
const sortKey = ref('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

function openEditDialog(team: Team) {
  baseOpenEditDialog(team, (t) => ({ name: t.name, description: t.description || '', assignment_strategy: t.assignment_strategy, is_active: t.is_active }))
}

watch(() => organizationsStore.selectedOrgId, () => { fetchTeams(); usersStore.fetchUsers() })
onMounted(() => { fetchTeams(); usersStore.fetchUsers() })

async function fetchTeams() {
  isLoading.value = true
  try {
    const response = await teamsStore.fetchTeams({
      search: searchQuery.value || undefined,
      page: currentPage.value,
      limit: pageSize
    })
    teams.value = response.teams
    totalItems.value = response.total
  } catch { toast.error('Failed to load teams') }
  finally { isLoading.value = false }
}

async function saveTeam() {
  if (!formData.value.name.trim()) { toast.error('Please enter a team name'); return }
  isSubmitting.value = true
  try {
    if (editingTeam.value) {
      await teamsStore.updateTeam(editingTeam.value.id, { name: formData.value.name, description: formData.value.description, assignment_strategy: formData.value.assignment_strategy, is_active: formData.value.is_active })
      toast.success('Team updated successfully')
    } else {
      await teamsStore.createTeam({ name: formData.value.name, description: formData.value.description, assignment_strategy: formData.value.assignment_strategy })
      toast.success('Team created successfully')
    }
    closeDialog()
    await fetchTeams()
  } catch (e) { toast.error(getErrorMessage(e, 'Failed to save team')) }
  finally { isSubmitting.value = false }
}

async function confirmDelete() {
  if (!teamToDelete.value) return
  try { await teamsStore.deleteTeam(teamToDelete.value.id); toast.success('Team deleted'); closeDeleteDialog(); await fetchTeams() }
  catch (e) { toast.error(getErrorMessage(e, 'Failed to delete team')) }
}

async function openMembersDialog(team: Team) {
  selectedTeam.value = team
  loadingMembers.value = true
  isMembersDialogOpen.value = true
  try { teamMembers.value = await teamsStore.fetchTeamMembers(team.id) }
  catch { toast.error('Failed to load team members') }
  finally { loadingMembers.value = false }
}

async function addMember(user: User, role: 'manager' | 'agent' = 'agent') {
  if (!selectedTeam.value) return
  try {
    const member = await teamsStore.addTeamMember(selectedTeam.value.id, user.id, role)
    teamMembers.value.push({ ...member, user: { id: user.id, full_name: user.full_name, email: user.email, is_available: true } })
    toast.success(`${user.full_name} added to team`)
  } catch (e) { toast.error(getErrorMessage(e, 'Failed to add member')) }
}

async function removeMember(member: TeamMember) {
  if (!selectedTeam.value) return
  try {
    await teamsStore.removeTeamMember(selectedTeam.value.id, member.user_id)
    teamMembers.value = teamMembers.value.filter(m => m.user_id !== member.user_id)
    toast.success('Member removed from team')
  } catch (e) { toast.error(getErrorMessage(e, 'Failed to remove member')) }
}

function getStrategyLabel(strategy: string): string { return getLabelFromValue(ASSIGNMENT_STRATEGIES, strategy) }
function getStrategyIcon(strategy: string) { return { round_robin: RotateCcw, load_balanced: Scale, manual: Hand }[strategy] || RotateCcw }
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader title="Teams" :icon="Users" icon-gradient="bg-gradient-to-br from-cyan-500 to-blue-600 shadow-cyan-500/20" back-link="/settings" :breadcrumbs="breadcrumbs">
      <template #actions>
        <Button v-if="isAdmin" variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />Add Team</Button>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto">
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>Your Teams</CardTitle>
                  <CardDescription>Organize agents into teams with assignment strategies: Round Robin, Load Balanced, or Manual Queue.</CardDescription>
                </div>
                <SearchInput v-model="searchQuery" placeholder="Search teams..." class="w-64" />
              </div>
            </CardHeader>
            <CardContent>
              <DataTable :items="teams" :columns="columns" :is-loading="isLoading" :empty-icon="Users" :empty-title="searchQuery ? 'No matching teams' : 'No teams created yet'" :empty-description="searchQuery ? 'No teams match your search.' : 'Create your first team to organize agents.'" v-model:sort-key="sortKey" v-model:sort-direction="sortDirection" server-pagination :current-page="currentPage" :total-items="totalItems" :page-size="pageSize" item-name="teams" @page-change="handlePageChange">
                <template #empty-action>
                  <Button v-if="isAdmin" variant="outline" size="sm" @click="openCreateDialog"><Plus class="h-4 w-4 mr-2" />Add Team</Button>
                </template>
                <template #cell-team="{ item: team }">
                  <div class="flex items-center gap-3">
                    <div class="h-9 w-9 rounded-full bg-primary/10 flex items-center justify-center flex-shrink-0"><Users class="h-4 w-4 text-primary" /></div>
                    <div class="min-w-0">
                      <p class="font-medium truncate">{{ team.name }}</p>
                      <p v-if="team.description" class="text-sm text-muted-foreground truncate">{{ team.description }}</p>
                    </div>
                  </div>
                </template>
                <template #cell-strategy="{ item: team }">
                  <div class="flex items-center gap-2">
                    <component :is="getStrategyIcon(team.assignment_strategy)" class="h-4 w-4 text-muted-foreground" />
                    <span class="text-sm">{{ getStrategyLabel(team.assignment_strategy) }}</span>
                  </div>
                </template>
                <template #cell-members="{ item: team }">
                  <Button variant="ghost" size="sm" class="h-8 px-2" @click="openMembersDialog(team)"><Users class="h-4 w-4 mr-1" />{{ team.member_count || 0 }}</Button>
                </template>
                <template #cell-status="{ item: team }">
                  <Badge variant="outline" :class="team.is_active ? 'border-green-600 text-green-600' : ''">{{ team.is_active ? 'Active' : 'Inactive' }}</Badge>
                </template>
                <template #cell-created="{ item: team }">
                  <span class="text-muted-foreground">{{ formatDate(team.created_at) }}</span>
                </template>
                <template #cell-actions="{ item: team }">
                  <div class="flex items-center justify-end gap-1">
                    <Tooltip><TooltipTrigger as-child><Button variant="ghost" size="icon" class="h-8 w-8" @click="openMembersDialog(team)"><UserPlus class="h-4 w-4" /></Button></TooltipTrigger><TooltipContent>Manage members</TooltipContent></Tooltip>
                    <Tooltip><TooltipTrigger as-child><Button variant="ghost" size="icon" class="h-8 w-8" @click="openEditDialog(team)"><Pencil class="h-4 w-4" /></Button></TooltipTrigger><TooltipContent>Edit team</TooltipContent></Tooltip>
                    <Tooltip v-if="isAdmin"><TooltipTrigger as-child><Button variant="ghost" size="icon" class="h-8 w-8" @click="openDeleteDialog(team)"><Trash2 class="h-4 w-4 text-destructive" /></Button></TooltipTrigger><TooltipContent>Delete team</TooltipContent></Tooltip>
                  </div>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <CrudFormDialog v-model:open="isDialogOpen" :is-editing="!!editingTeam" :is-submitting="isSubmitting" edit-title="Edit Team" create-title="Create Team" edit-description="Update team settings." create-description="Create a new team to organize agents." edit-submit-label="Update Team" create-submit-label="Create Team" @submit="saveTeam">
      <div class="space-y-4">
        <div class="space-y-2"><Label for="name">Team Name <span class="text-destructive">*</span></Label><Input id="name" v-model="formData.name" placeholder="e.g., Sales Team" /></div>
        <div class="space-y-2"><Label for="description">Description</Label><Textarea id="description" v-model="formData.description" placeholder="What does this team handle?" :rows="2" /></div>
        <div class="space-y-2">
          <Label for="strategy">Assignment Strategy</Label>
          <Select v-model="formData.assignment_strategy"><SelectTrigger><SelectValue placeholder="Select strategy" /></SelectTrigger><SelectContent><SelectItem value="round_robin"><div class="flex items-center gap-2"><RotateCcw class="h-4 w-4" />Round Robin</div></SelectItem><SelectItem value="load_balanced"><div class="flex items-center gap-2"><Scale class="h-4 w-4" />Load Balanced</div></SelectItem><SelectItem value="manual"><div class="flex items-center gap-2"><Hand class="h-4 w-4" />Manual Queue</div></SelectItem></SelectContent></Select>
        </div>
        <div v-if="editingTeam" class="flex items-center justify-between"><Label for="is_active" class="font-normal cursor-pointer">Team Active</Label><Switch id="is_active" :checked="formData.is_active" @update:checked="formData.is_active = $event" /></div>
      </div>
    </CrudFormDialog>

    <!-- Team Members Dialog (team-specific) -->
    <Dialog v-model:open="isMembersDialogOpen">
      <DialogContent class="max-w-lg">
        <DialogHeader>
          <DialogTitle>Team Members - {{ selectedTeam?.name }}</DialogTitle>
          <DialogDescription>Add or remove team members.</DialogDescription>
        </DialogHeader>
        <div class="py-4 space-y-4">
          <div>
            <h4 class="font-medium mb-2">Current Members ({{ teamMembers.length }})</h4>
            <div v-if="loadingMembers" class="flex items-center justify-center py-4"><Loader2 class="h-6 w-6 animate-spin" /></div>
            <div v-else-if="teamMembers.length === 0" class="text-sm text-muted-foreground py-4 text-center">No members yet. Add users below.</div>
            <div v-else class="space-y-2 max-h-48 overflow-y-auto">
              <div v-for="member in teamMembers" :key="member.id" class="flex items-center justify-between p-2 rounded-md border">
                <div class="flex items-center gap-3">
                  <div class="h-8 w-8 rounded-full bg-muted flex items-center justify-center">{{ (member.full_name || member.user?.full_name)?.charAt(0) || '?' }}</div>
                  <div><p class="text-sm font-medium">{{ member.full_name || member.user?.full_name }}</p><p class="text-xs text-muted-foreground">{{ member.email || member.user?.email }}</p></div>
                </div>
                <div class="flex items-center gap-2">
                  <Badge variant="outline" class="text-xs">{{ member.role }}</Badge>
                  <Button variant="ghost" size="icon" class="h-7 w-7" @click="removeMember(member)"><UserMinus class="h-4 w-4 text-destructive" /></Button>
                </div>
              </div>
            </div>
          </div>
          <div v-if="availableUsers.length > 0">
            <h4 class="font-medium mb-2">Add Members</h4>
            <div class="space-y-2 max-h-48 overflow-y-auto">
              <div v-for="user in availableUsers" :key="user.id" class="flex items-center justify-between p-2 rounded-md border">
                <div class="flex items-center gap-3">
                  <div class="h-8 w-8 rounded-full bg-muted flex items-center justify-center">{{ user.full_name.charAt(0) }}</div>
                  <div><p class="text-sm font-medium">{{ user.full_name }}</p><p class="text-xs text-muted-foreground">{{ user.email }}</p></div>
                </div>
                <div class="flex items-center gap-1">
                  <Button variant="outline" size="sm" class="h-7 text-xs" @click="addMember(user, 'agent')">Add as Agent</Button>
                  <Button v-if="isAdmin" variant="outline" size="sm" class="h-7 text-xs" @click="addMember(user, 'manager')">Add as Manager</Button>
                </div>
              </div>
            </div>
          </div>
          <div v-else-if="!loadingMembers" class="text-sm text-muted-foreground text-center py-2">All active users are already members of this team.</div>
        </div>
        <DialogFooter><Button variant="outline" size="sm" @click="isMembersDialogOpen = false">Close</Button></DialogFooter>
      </DialogContent>
    </Dialog>

    <DeleteConfirmDialog v-model:open="deleteDialogOpen" title="Delete Team" :item-name="teamToDelete?.name" description="Active transfers will remain but will no longer be associated with this team." @confirm="confirmDelete" />
  </div>
</template>
