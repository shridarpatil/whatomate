<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useTeamsStore } from '@/stores/teams'
import { useUsersStore } from '@/stores/users'
import { useAuthStore } from '@/stores/auth'
import { teamsService, type Team, type TeamMember } from '@/services/api'
import { toast } from 'vue-sonner'
import { ASSIGNMENT_STRATEGIES } from '@/lib/constants'
import { useDebounceFn } from '@vueuse/core'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Separator } from '@/components/ui/separator'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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
  Users,
  Trash2,
  Save,
  UserPlus,
  UserMinus,
  Shield,
} from 'lucide-vue-next'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const teamsStore = useTeamsStore()
const usersStore = useUsersStore()
const authStore = useAuthStore()

const teamId = computed(() => route.params.id as string)
const isNew = computed(() => teamId.value === 'new')
const team = ref<Team | null>(null)
const members = ref<TeamMember[]>([])
const isLoading = ref(true)
const isNotFound = ref(false)
const isSaving = ref(false)
const hasChanges = ref(false)
const deleteDialogOpen = ref(false)
const memberSearch = ref('')
const removeMemberDialogOpen = ref(false)
const memberToRemove = ref<TeamMember | null>(null)

const { showLeaveDialog, confirmLeave, cancelLeave } = useUnsavedChangesGuard(hasChanges)

const canWrite = computed(() => authStore.hasPermission('teams', 'write'))
const canDelete = computed(() => authStore.hasPermission('teams', 'delete'))

// Edit form
const form = ref({
  name: '',
  description: '',
  assignment_strategy: 'round_robin' as 'round_robin' | 'load_balanced' | 'manual',
  per_agent_timeout_secs: 0,
  is_active: true,
})

const breadcrumbs = computed(() => [
  { label: t('nav.settings'), href: '/settings' },
  { label: t('nav.teams'), href: '/settings/teams' },
  { label: isNew.value ? t('teams.newTeam', 'New Team') : (team.value?.name || '') },
])

const availableUsers = computed(() => {
  const memberUserIds = new Set(members.value.map(m => m.user_id))
  return usersStore.users.filter(u => !memberUserIds.has(u.id) && u.is_active)
})

async function loadTeam() {
  isLoading.value = true
  isNotFound.value = false
  try {
    const response = await teamsService.get(teamId.value)
    const data = (response.data as any).data?.team || response.data?.team
    team.value = data
    members.value = data.members || []
    syncForm()
    // Reset after syncForm so the watcher doesn't trigger
    nextTick(() => { hasChanges.value = false })
  } catch {
    isNotFound.value = true
  } finally {
    isLoading.value = false
  }
}

function syncForm() {
  if (!team.value) return
  form.value = {
    name: team.value.name,
    description: team.value.description || '',
    assignment_strategy: team.value.assignment_strategy,
    per_agent_timeout_secs: team.value.per_agent_timeout_secs,
    is_active: team.value.is_active,
  }
}

// Track form changes
watch(form, () => {
  if (!team.value) return
  hasChanges.value = true
}, { deep: true })

async function save() {
  if (!form.value.name.trim()) {
    toast.error(t('teams.nameRequired', 'Team name is required'))
    return
  }
  isSaving.value = true
  try {
    if (isNew.value) {
      const created = await teamsStore.createTeam({
        name: form.value.name,
        description: form.value.description,
        assignment_strategy: form.value.assignment_strategy,
        per_agent_timeout_secs: form.value.per_agent_timeout_secs,
      })
      hasChanges.value = false
      toast.success(t('teams.created', 'Team created'))
      router.replace(`/settings/teams/${created.id}`)
    } else {
      await teamsStore.updateTeam(team.value!.id, {
        name: form.value.name,
        description: form.value.description,
        assignment_strategy: form.value.assignment_strategy,
        per_agent_timeout_secs: form.value.per_agent_timeout_secs,
        is_active: form.value.is_active,
      })
      await loadTeam()
      hasChanges.value = false
      toast.success(t('teams.updated', 'Team updated'))
    }
  } catch {
    toast.error(isNew.value ? t('teams.createFailed', 'Failed to create team') : t('teams.updateFailed', 'Failed to update team'))
  } finally {
    isSaving.value = false
  }
}

async function deleteTeam() {
  if (!team.value) return
  try {
    await teamsStore.deleteTeam(team.value.id)
    toast.success(t('teams.deleted', 'Team deleted'))
    router.push('/settings/teams')
  } catch {
    toast.error(t('teams.deleteFailed', 'Failed to delete team'))
  }
  deleteDialogOpen.value = false
}

async function addMember(userId: string, role: 'manager' | 'agent') {
  if (!team.value) return
  try {
    const member = await teamsStore.addTeamMember(team.value.id, userId, role)
    const user = usersStore.users.find(u => u.id === userId)
    members.value.push({
      ...member,
      full_name: user?.full_name || '',
      email: user?.email || '',
      is_available: (user as any)?.is_available ?? false,
    })
    toast.success(t('teams.memberAdded', 'Member added'))
  } catch {
    toast.error(t('teams.memberAddFailed', 'Failed to add member'))
  }
}

function openRemoveMemberDialog(member: TeamMember) {
  memberToRemove.value = member
  removeMemberDialogOpen.value = true
}

async function confirmRemoveMember() {
  if (!team.value || !memberToRemove.value) return
  try {
    await teamsStore.removeTeamMember(team.value.id, memberToRemove.value.user_id)
    members.value = members.value.filter(m => m.user_id !== memberToRemove.value!.user_id)
    toast.success(t('teams.memberRemoved', 'Member removed'))
  } catch {
    toast.error(t('teams.memberRemoveFailed', 'Failed to remove member'))
  }
  removeMemberDialogOpen.value = false
  memberToRemove.value = null
}

async function searchUsers() {
  await usersStore.fetchUsers({ search: memberSearch.value || undefined, limit: 20 })
}

const debouncedSearchUsers = useDebounceFn(searchUsers, 300)

watch(memberSearch, () => debouncedSearchUsers())

onMounted(async () => {
  if (isNew.value) {
    isLoading.value = false
    hasChanges.value = false
  } else {
    await loadTeam()
  }
  await usersStore.fetchUsers({ limit: 20 })
})

</script>

<template>
  <div class="h-full">
  <DetailPageLayout
    :title="isNew ? $t('teams.newTeam', 'New Team') : (team?.name || '')"
    :icon="Users"
    icon-gradient="bg-gradient-to-br from-cyan-500 to-blue-600 shadow-cyan-500/20"
    back-link="/settings/teams"
    :breadcrumbs="breadcrumbs"
    :is-loading="isLoading"
    :is-not-found="isNotFound"
    :not-found-title="$t('teams.notFound', 'Team not found')"
  >
    <template #actions>
      <div class="flex items-center gap-2">
        <Button v-if="canWrite && (hasChanges || isNew)" size="sm" @click="save" :disabled="isSaving">
          <Save class="h-4 w-4 mr-1" /> {{ isSaving ? $t('common.saving', 'Saving...') : isNew ? $t('common.create') : $t('common.save') }}
        </Button>
        <Button v-if="canDelete && !isNew" variant="destructive" size="sm" @click="deleteDialogOpen = true">
          <Trash2 class="h-4 w-4 mr-1" /> {{ $t('common.delete') }}
        </Button>
      </div>
    </template>

    <!-- Team Details Card -->
    <Card>
      <CardHeader class="pb-3">
        <div class="flex items-center justify-between">
          <CardTitle class="text-sm font-medium">{{ $t('teams.details', 'Details') }}</CardTitle>
          <Badge :variant="(team?.is_active ?? true) ? 'default' : 'secondary'">
            {{ (team?.is_active ?? true) ? $t('common.active', 'Active') : $t('common.inactive', 'Inactive') }}
          </Badge>
        </div>
      </CardHeader>
      <CardContent class="space-y-4">
        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('teams.name', 'Name') }} *</Label>
          <Input v-model="form.name" :disabled="!canWrite" />
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('common.description', 'Description') }}</Label>
          <Textarea v-model="form.description" :rows="2" :disabled="!canWrite" />
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs">{{ $t('teams.assignmentStrategy', 'Assignment Strategy') }}</Label>
          <Select v-model="form.assignment_strategy" :disabled="!canWrite">
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="s in ASSIGNMENT_STRATEGIES" :key="s.value" :value="s.value">
                {{ s.label }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div v-if="form.assignment_strategy !== 'manual'" class="space-y-1.5">
          <Label class="text-xs">{{ $t('teams.perAgentTimeout', 'Per-Agent Timeout') }} ({{ $t('common.seconds', 'seconds') }})</Label>
          <Input v-model.number="form.per_agent_timeout_secs" type="number" min="0" max="300" :disabled="!canWrite" />
        </div>
        <div class="flex items-center gap-2">
          <Switch :checked="form.is_active" @update:checked="form.is_active = $event" :disabled="!canWrite" />
          <Label class="text-xs">{{ $t('common.active', 'Active') }}</Label>
        </div>
      </CardContent>
    </Card>

    <!-- Members Card -->
    <Card>
      <CardHeader class="pb-3">
        <div class="flex items-center justify-between">
          <CardTitle class="text-sm font-medium">{{ $t('teams.members', 'Members') }} ({{ members.length }})</CardTitle>
        </div>
      </CardHeader>
      <CardContent class="space-y-4">
        <!-- Current Members -->
        <div v-if="members.length > 0" class="space-y-2">
          <div
            v-for="member in members"
            :key="member.user_id"
            class="flex items-center gap-3 p-2 rounded-lg hover:bg-muted/50 transition-colors"
          >
            <div class="h-8 w-8 rounded-full bg-primary/10 flex items-center justify-center shrink-0">
              <span class="text-xs font-medium text-primary">{{ (member.full_name || '?')[0].toUpperCase() }}</span>
            </div>
            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium truncate">{{ member.full_name }}</p>
              <p class="text-xs text-muted-foreground truncate">{{ member.email }}</p>
            </div>
            <Badge variant="outline" class="text-xs shrink-0">
              <Shield v-if="member.role === 'manager'" class="h-3 w-3 mr-1" />
              {{ member.role }}
            </Badge>
            <Button
              v-if="canWrite"
              variant="ghost"
              size="icon"
              class="h-7 w-7 shrink-0 text-destructive"
              @click="openRemoveMemberDialog(member)"
            >
              <UserMinus class="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>
        <p v-else class="text-sm text-muted-foreground text-center py-4">{{ $t('teams.noMembers', 'No members yet') }}</p>

        <!-- Add Members -->
        <template v-if="canWrite">
          <Separator />
          <div>
            <div class="flex items-center justify-between mb-2">
              <Label class="text-xs text-muted-foreground">{{ $t('teams.addMembers', 'Add Members') }}</Label>
              <Input v-model="memberSearch" :placeholder="$t('teams.searchUsers', 'Search...')" class="h-6 text-[10px] w-40" />
            </div>
            <div v-if="availableUsers.length > 0" class="space-y-1 max-h-48 overflow-y-auto">
              <div
                v-for="user in availableUsers"
                :key="user.id"
                class="flex items-center gap-3 p-2 rounded-lg hover:bg-muted/50 transition-colors"
              >
                <div class="h-8 w-8 rounded-full bg-muted flex items-center justify-center shrink-0">
                  <span class="text-xs font-medium">{{ (user.full_name || '?')[0].toUpperCase() }}</span>
                </div>
                <div class="flex-1 min-w-0">
                  <p class="text-sm font-medium truncate">{{ user.full_name }}</p>
                  <p class="text-xs text-muted-foreground truncate">{{ user.email }}</p>
                </div>
                <div class="flex gap-1 shrink-0">
                  <Button variant="outline" size="sm" class="h-7 text-xs" @click="addMember(user.id, 'agent')">
                    <UserPlus class="h-3 w-3 mr-1" /> {{ $t('teams.agent', 'Agent') }}
                  </Button>
                  <Button variant="outline" size="sm" class="h-7 text-xs" @click="addMember(user.id, 'manager')">
                    <Shield class="h-3 w-3 mr-1" /> {{ $t('teams.manager', 'Manager') }}
                  </Button>
                </div>
              </div>
            </div>
            <p v-else-if="memberSearch" class="text-xs text-muted-foreground text-center py-2">{{ $t('teams.noMatchingUsers', 'No matching users') }}</p>
            <p v-else class="text-xs text-muted-foreground text-center py-2">{{ $t('teams.allUsersInTeam', 'All users are already in this team') }}</p>
          </div>
        </template>
      </CardContent>
    </Card>

    <!-- Activity Log -->
    <AuditLogPanel
      v-if="team && !isNew"
      resource-type="team"
      :resource-id="team.id"
    />

    <!-- Sidebar -->
    <template v-if="!isNew" #sidebar>
      <MetadataPanel
        :created-at="team?.created_at"
        :updated-at="team?.updated_at"
        :created-by-name="team?.created_by_name"
        :updated-by-name="team?.updated_by_name"
      />
    </template>
  </DetailPageLayout>

  <!-- Delete Confirmation -->
  <AlertDialog v-model:open="deleteDialogOpen">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ $t('teams.deleteTeam', 'Delete Team') }}</AlertDialogTitle>
        <AlertDialogDescription>
          {{ $t('teams.deleteConfirm', 'Are you sure? This will remove all team members and cannot be undone.') }}
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>{{ $t('common.cancel') }}</AlertDialogCancel>
        <AlertDialogAction @click="deleteTeam">{{ $t('common.delete') }}</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>

  <!-- Remove Member Confirmation -->
  <AlertDialog v-model:open="removeMemberDialogOpen">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ $t('teams.removeMember', 'Remove Member') }}</AlertDialogTitle>
        <AlertDialogDescription>
          {{ $t('teams.removeMemberConfirm', { name: memberToRemove?.full_name || '' }) }}
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>{{ $t('common.cancel') }}</AlertDialogCancel>
        <AlertDialogAction @click="confirmRemoveMember">{{ $t('common.remove', 'Remove') }}</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>

  <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
