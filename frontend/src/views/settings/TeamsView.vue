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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { useTeamsStore } from '@/stores/teams'
import { useUsersStore, type User } from '@/stores/users'
import { useAuthStore } from '@/stores/auth'
import { useOrganizationsStore } from '@/stores/organizations'
import { type Team, type TeamMember } from '@/services/api'
import { toast } from 'vue-sonner'
import {
  Plus,
  Pencil,
  Trash2,
  Loader2,
  Search,
  ArrowLeft,
  Users,
  UserPlus,
  UserMinus,
  RotateCcw,
  Scale,
  Hand,
} from 'lucide-vue-next'

const teamsStore = useTeamsStore()
const usersStore = useUsersStore()
const authStore = useAuthStore()
const organizationsStore = useOrganizationsStore()

const isLoading = ref(true)
const isDialogOpen = ref(false)
const isMembersDialogOpen = ref(false)
const isSubmitting = ref(false)
const editingTeam = ref<Team | null>(null)
const deleteDialogOpen = ref(false)
const teamToDelete = ref<Team | null>(null)
const selectedTeam = ref<Team | null>(null)
const teamMembers = ref<TeamMember[]>([])
const loadingMembers = ref(false)

// Search
const searchQuery = ref('')

const formData = ref({
  name: '',
  description: '',
  assignment_strategy: 'round_robin' as 'round_robin' | 'load_balanced' | 'manual',
  is_active: true
})

const isAdmin = computed(() => authStore.userRole === 'admin')

// Filtered teams
const filteredTeams = computed(() => {
  if (!searchQuery.value.trim()) {
    return teamsStore.teams
  }
  const query = searchQuery.value.toLowerCase()
  return teamsStore.teams.filter(team =>
    team.name.toLowerCase().includes(query) ||
    team.description?.toLowerCase().includes(query)
  )
})

// Users available to add to team (not already members)
const availableUsers = computed(() => {
  const memberUserIds = new Set(teamMembers.value.map(m => m.user_id))
  return usersStore.users.filter(u => !memberUserIds.has(u.id) && u.is_active)
})

// Refetch data when organization changes
watch(() => organizationsStore.selectedOrgId, () => {
  fetchTeams()
  usersStore.fetchUsers()
})

onMounted(async () => {
  await Promise.all([
    fetchTeams(),
    usersStore.fetchUsers()
  ])
})

async function fetchTeams() {
  isLoading.value = true
  try {
    await teamsStore.fetchTeams()
  } catch (error: any) {
    toast.error('Failed to load teams')
  } finally {
    isLoading.value = false
  }
}

function openCreateDialog() {
  editingTeam.value = null
  formData.value = {
    name: '',
    description: '',
    assignment_strategy: 'round_robin',
    is_active: true
  }
  isDialogOpen.value = true
}

function openEditDialog(team: Team) {
  editingTeam.value = team
  formData.value = {
    name: team.name,
    description: team.description || '',
    assignment_strategy: team.assignment_strategy,
    is_active: team.is_active
  }
  isDialogOpen.value = true
}

async function saveTeam() {
  if (!formData.value.name.trim()) {
    toast.error('Please enter a team name')
    return
  }

  isSubmitting.value = true
  try {
    if (editingTeam.value) {
      await teamsStore.updateTeam(editingTeam.value.id, {
        name: formData.value.name,
        description: formData.value.description,
        assignment_strategy: formData.value.assignment_strategy,
        is_active: formData.value.is_active
      })
      toast.success('Team updated successfully')
    } else {
      await teamsStore.createTeam({
        name: formData.value.name,
        description: formData.value.description,
        assignment_strategy: formData.value.assignment_strategy
      })
      toast.success('Team created successfully')
    }
    isDialogOpen.value = false
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to save team'
    toast.error(message)
  } finally {
    isSubmitting.value = false
  }
}

function openDeleteDialog(team: Team) {
  teamToDelete.value = team
  deleteDialogOpen.value = true
}

async function confirmDelete() {
  if (!teamToDelete.value) return

  try {
    await teamsStore.deleteTeam(teamToDelete.value.id)
    toast.success('Team deleted')
    deleteDialogOpen.value = false
    teamToDelete.value = null
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to delete team'
    toast.error(message)
  }
}

async function openMembersDialog(team: Team) {
  selectedTeam.value = team
  loadingMembers.value = true
  isMembersDialogOpen.value = true

  try {
    teamMembers.value = await teamsStore.fetchTeamMembers(team.id)
  } catch (error: any) {
    toast.error('Failed to load team members')
  } finally {
    loadingMembers.value = false
  }
}

async function addMember(user: User, role: 'manager' | 'agent' = 'agent') {
  if (!selectedTeam.value) return

  try {
    const member = await teamsStore.addTeamMember(selectedTeam.value.id, user.id, role)
    // Add to local list with user info
    teamMembers.value.push({
      ...member,
      user: {
        id: user.id,
        full_name: user.full_name,
        email: user.email,
        is_available: true
      }
    })
    toast.success(`${user.full_name} added to team`)
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to add member'
    toast.error(message)
  }
}

async function removeMember(member: TeamMember) {
  if (!selectedTeam.value) return

  try {
    await teamsStore.removeTeamMember(selectedTeam.value.id, member.user_id)
    teamMembers.value = teamMembers.value.filter(m => m.user_id !== member.user_id)
    toast.success('Member removed from team')
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to remove member'
    toast.error(message)
  }
}

function getStrategyLabel(strategy: string): string {
  switch (strategy) {
    case 'round_robin':
      return 'Round Robin'
    case 'load_balanced':
      return 'Load Balanced'
    case 'manual':
      return 'Manual Queue'
    default:
      return strategy
  }
}

function getStrategyIcon(strategy: string) {
  switch (strategy) {
    case 'round_robin':
      return RotateCcw
    case 'load_balanced':
      return Scale
    case 'manual':
      return Hand
    default:
      return RotateCcw
  }
}

function formatDate(dateString: string) {
  return new Date(dateString).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  })
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <!-- Header -->
    <header class="border-b border-white/[0.08] light:border-gray-200 bg-[#0a0a0b]/95 light:bg-white/95 backdrop-blur">
      <div class="flex h-16 items-center px-6">
        <RouterLink to="/settings">
          <Button variant="ghost" size="icon" class="mr-3">
            <ArrowLeft class="h-5 w-5" />
          </Button>
        </RouterLink>
        <div class="h-8 w-8 rounded-lg bg-gradient-to-br from-cyan-500 to-blue-600 flex items-center justify-center mr-3 shadow-lg shadow-cyan-500/20">
          <Users class="h-4 w-4 text-white" />
        </div>
        <div class="flex-1">
          <h1 class="text-xl font-semibold text-white light:text-gray-900">Teams</h1>
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem>
                <BreadcrumbLink href="/settings">Settings</BreadcrumbLink>
              </BreadcrumbItem>
              <BreadcrumbSeparator />
              <BreadcrumbItem>
                <BreadcrumbPage>Teams</BreadcrumbPage>
              </BreadcrumbItem>
            </BreadcrumbList>
          </Breadcrumb>
        </div>
        <Button v-if="isAdmin" variant="outline" size="sm" @click="openCreateDialog">
          <Plus class="h-4 w-4 mr-2" />
          Add Team
        </Button>
      </div>
    </header>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto space-y-4">
          <!-- Search -->
          <div class="flex items-center gap-4">
            <div class="relative flex-1 max-w-sm">
              <Search class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                v-model="searchQuery"
                placeholder="Search teams..."
                class="pl-9"
              />
            </div>
            <div class="text-sm text-muted-foreground">
              {{ filteredTeams.length }} team{{ filteredTeams.length !== 1 ? 's' : '' }}
            </div>
          </div>

          <!-- Teams Table -->
          <Card>
            <CardHeader>
              <CardTitle>Your Teams</CardTitle>
              <CardDescription>
                Organize agents into teams with assignment strategies: Round Robin, Load Balanced, or Manual Queue.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
              <TableHeader>
                <TableRow>
                  <TableHead class="w-[250px]">Team</TableHead>
                  <TableHead>Strategy</TableHead>
                  <TableHead>Members</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead class="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-if="isLoading">
                  <TableCell colspan="6" class="h-24 text-center">
                    <Loader2 class="h-6 w-6 animate-spin mx-auto" />
                  </TableCell>
                </TableRow>
                <TableRow v-else-if="filteredTeams.length === 0">
                  <TableCell colspan="6" class="h-24 text-center text-muted-foreground">
                    <Users class="h-8 w-8 mx-auto mb-2 opacity-50" />
                    <p>{{ searchQuery ? 'No teams found matching your search' : 'No teams created yet' }}</p>
                    <Button v-if="isAdmin && !searchQuery" variant="outline" size="sm" class="mt-3" @click="openCreateDialog">
                      <Plus class="h-4 w-4 mr-2" />
                      Create First Team
                    </Button>
                  </TableCell>
                </TableRow>
                <TableRow v-else v-for="team in filteredTeams" :key="team.id">
                  <TableCell>
                    <div class="flex items-center gap-3">
                      <div class="h-9 w-9 rounded-full bg-primary/10 flex items-center justify-center flex-shrink-0">
                        <Users class="h-4 w-4 text-primary" />
                      </div>
                      <div class="min-w-0">
                        <p class="font-medium truncate">{{ team.name }}</p>
                        <p v-if="team.description" class="text-sm text-muted-foreground truncate">{{ team.description }}</p>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div class="flex items-center gap-2">
                      <component :is="getStrategyIcon(team.assignment_strategy)" class="h-4 w-4 text-muted-foreground" />
                      <span class="text-sm">{{ getStrategyLabel(team.assignment_strategy) }}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Button variant="ghost" size="sm" class="h-8 px-2" @click="openMembersDialog(team)">
                      <Users class="h-4 w-4 mr-1" />
                      {{ team.member_count || 0 }}
                    </Button>
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant="outline"
                      :class="team.is_active ? 'border-green-600 text-green-600' : ''"
                    >
                      {{ team.is_active ? 'Active' : 'Inactive' }}
                    </Badge>
                  </TableCell>
                  <TableCell class="text-muted-foreground">
                    {{ formatDate(team.created_at) }}
                  </TableCell>
                  <TableCell class="text-right">
                    <div class="flex items-center justify-end gap-1">
                      <Tooltip>
                        <TooltipTrigger as-child>
                          <Button variant="ghost" size="icon" class="h-8 w-8" @click="openMembersDialog(team)">
                            <UserPlus class="h-4 w-4" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>Manage members</TooltipContent>
                      </Tooltip>
                      <Tooltip>
                        <TooltipTrigger as-child>
                          <Button variant="ghost" size="icon" class="h-8 w-8" @click="openEditDialog(team)">
                            <Pencil class="h-4 w-4" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>Edit team</TooltipContent>
                      </Tooltip>
                      <Tooltip v-if="isAdmin">
                        <TooltipTrigger as-child>
                          <Button
                            variant="ghost"
                            size="icon"
                            class="h-8 w-8"
                            @click="openDeleteDialog(team)"
                          >
                            <Trash2 class="h-4 w-4 text-destructive" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>Delete team</TooltipContent>
                      </Tooltip>
                    </div>
                  </TableCell>
                </TableRow>
              </TableBody>
              </Table>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <!-- Add/Edit Team Dialog -->
    <Dialog v-model:open="isDialogOpen">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>{{ editingTeam ? 'Edit' : 'Create' }} Team</DialogTitle>
          <DialogDescription>
            {{ editingTeam ? 'Update team settings.' : 'Create a new team to organize agents.' }}
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-4">
          <div class="space-y-2">
            <Label for="name">Team Name <span class="text-destructive">*</span></Label>
            <Input
              id="name"
              v-model="formData.name"
              placeholder="e.g., Sales Team"
            />
          </div>

          <div class="space-y-2">
            <Label for="description">Description</Label>
            <Textarea
              id="description"
              v-model="formData.description"
              placeholder="What does this team handle?"
              rows="2"
            />
          </div>

          <div class="space-y-2">
            <Label for="strategy">Assignment Strategy</Label>
            <Select v-model="formData.assignment_strategy">
              <SelectTrigger>
                <SelectValue placeholder="Select strategy" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="round_robin">
                  <div class="flex items-center gap-2">
                    <RotateCcw class="h-4 w-4" />
                    Round Robin
                  </div>
                </SelectItem>
                <SelectItem value="load_balanced">
                  <div class="flex items-center gap-2">
                    <Scale class="h-4 w-4" />
                    Load Balanced
                  </div>
                </SelectItem>
                <SelectItem value="manual">
                  <div class="flex items-center gap-2">
                    <Hand class="h-4 w-4" />
                    Manual Queue
                  </div>
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div v-if="editingTeam" class="flex items-center justify-between">
            <Label for="is_active" class="font-normal cursor-pointer">
              Team Active
            </Label>
            <Switch
              id="is_active"
              :checked="formData.is_active"
              @update:checked="formData.is_active = $event"
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" @click="isDialogOpen = false">Cancel</Button>
          <Button size="sm" @click="saveTeam" :disabled="isSubmitting">
            <Loader2 v-if="isSubmitting" class="h-4 w-4 mr-2 animate-spin" />
            {{ editingTeam ? 'Update' : 'Create' }} Team
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Manage Members Dialog -->
    <Dialog v-model:open="isMembersDialogOpen">
      <DialogContent class="max-w-lg">
        <DialogHeader>
          <DialogTitle>Team Members - {{ selectedTeam?.name }}</DialogTitle>
          <DialogDescription>
            Add or remove team members.
          </DialogDescription>
        </DialogHeader>

        <div class="py-4 space-y-4">
          <!-- Current Members -->
          <div>
            <h4 class="font-medium mb-2">Current Members ({{ teamMembers.length }})</h4>
            <div v-if="loadingMembers" class="flex items-center justify-center py-4">
              <Loader2 class="h-6 w-6 animate-spin" />
            </div>
            <div v-else-if="teamMembers.length === 0" class="text-sm text-muted-foreground py-4 text-center">
              No members yet. Add users below.
            </div>
            <div v-else class="space-y-2 max-h-48 overflow-y-auto">
              <div
                v-for="member in teamMembers"
                :key="member.id"
                class="flex items-center justify-between p-2 rounded-md border"
              >
                <div class="flex items-center gap-3">
                  <div class="h-8 w-8 rounded-full bg-muted flex items-center justify-center">
                    {{ (member.full_name || member.user?.full_name)?.charAt(0) || '?' }}
                  </div>
                  <div>
                    <p class="text-sm font-medium">{{ member.full_name || member.user?.full_name }}</p>
                    <p class="text-xs text-muted-foreground">{{ member.email || member.user?.email }}</p>
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <Badge variant="outline" class="text-xs">{{ member.role }}</Badge>
                  <Button variant="ghost" size="icon" class="h-7 w-7" @click="removeMember(member)">
                    <UserMinus class="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              </div>
            </div>
          </div>

          <!-- Add Members -->
          <div v-if="availableUsers.length > 0">
            <h4 class="font-medium mb-2">Add Members</h4>
            <div class="space-y-2 max-h-48 overflow-y-auto">
              <div
                v-for="user in availableUsers"
                :key="user.id"
                class="flex items-center justify-between p-2 rounded-md border"
              >
                <div class="flex items-center gap-3">
                  <div class="h-8 w-8 rounded-full bg-muted flex items-center justify-center">
                    {{ user.full_name.charAt(0) }}
                  </div>
                  <div>
                    <p class="text-sm font-medium">{{ user.full_name }}</p>
                    <p class="text-xs text-muted-foreground">{{ user.email }}</p>
                  </div>
                </div>
                <div class="flex items-center gap-1">
                  <Button variant="outline" size="sm" class="h-7 text-xs" @click="addMember(user, 'agent')">
                    Add as Agent
                  </Button>
                  <Button v-if="isAdmin" variant="outline" size="sm" class="h-7 text-xs" @click="addMember(user, 'manager')">
                    Add as Manager
                  </Button>
                </div>
              </div>
            </div>
          </div>
          <div v-else-if="!loadingMembers" class="text-sm text-muted-foreground text-center py-2">
            All active users are already members of this team.
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" @click="isMembersDialogOpen = false">Close</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Delete Confirmation Dialog -->
    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete Team</AlertDialogTitle>
          <AlertDialogDescription>
            Are you sure you want to delete "{{ teamToDelete?.name }}"? This action cannot be undone.
            Active transfers will remain but will no longer be associated with this team.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction @click="confirmDelete" class="bg-destructive text-destructive-foreground hover:bg-destructive/90">
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
