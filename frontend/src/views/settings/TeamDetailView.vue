<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useTeamsStore } from '@/stores/teams'
import { useUsersStore } from '@/stores/users'
import { useAuthStore } from '@/stores/auth'
import { teamsService, type Team, type TeamMember } from '@/services/api'
import { toast } from '@/lib/toast'
import { IconButton } from '@/components/shared'
import { ASSIGNMENT_STRATEGIES } from '@/lib/constants'
import { debounce } from '@/lib/utils'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import Modal from '@/components/oat/Modal.vue'
import {
  Users,
  Trash2,
  Save,
  UserPlus,
  UserMinus,
  Shield,
} from '@/lib/icons'

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

const debouncedSearchUsers = debounce(searchUsers, 300)

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
    back-link="/settings/teams"
    :breadcrumbs="breadcrumbs"
    :is-loading="isLoading"
    :is-not-found="isNotFound"
    :not-found-title="$t('teams.notFound', 'Team not found')"
  >
    <template #actions>
      <div class="hstack gap-2">
        <button v-if="canWrite && (hasChanges || isNew)" type="button" class="small" :disabled="isSaving" :aria-busy="isSaving ? 'true' : undefined" data-spinner="small" @click="save">
          <Save v-if="!isSaving" :size="16" /> {{ isSaving ? $t('common.saving', 'Saving...') : isNew ? $t('common.create') : $t('common.save') }}
        </button>
        <button v-if="canDelete && !isNew" type="button" class="small" data-variant="danger" @click="deleteDialogOpen = true">
          <Trash2 :size="16" /> {{ $t('common.delete') }}
        </button>
      </div>
    </template>

    <!-- Team Details Card -->
    <article class="card">
      <header class="hstack justify-between">
        <h3>{{ $t('teams.details', 'Details') }}</h3>
        <span class="badge" :data-variant="(team?.is_active ?? true) ? 'success' : 'secondary'">
          {{ (team?.is_active ?? true) ? $t('common.active', 'Active') : $t('common.inactive', 'Inactive') }}
        </span>
      </header>
      <div class="vstack gap-4">
        <label data-field>
          {{ $t('teams.name', 'Name') }} *
          <input v-model="form.name" :disabled="!canWrite" />
        </label>
        <label data-field>
          {{ $t('common.description', 'Description') }}
          <textarea v-model="form.description" rows="2" :disabled="!canWrite" />
        </label>
        <label data-field>
          {{ $t('teams.assignmentStrategy', 'Assignment Strategy') }}
          <select v-model="form.assignment_strategy" :disabled="!canWrite">
            <option v-for="s in ASSIGNMENT_STRATEGIES" :key="s.value" :value="s.value">{{ s.label }}</option>
          </select>
        </label>
        <label v-if="form.assignment_strategy !== 'manual'" data-field>
          {{ $t('teams.perAgentTimeout', 'Per-Agent Timeout') }} ({{ $t('common.seconds', 'seconds') }})
          <input v-model.number="form.per_agent_timeout_secs" type="number" min="0" max="300" :disabled="!canWrite" />
        </label>
        <label class="hstack gap-2">
          <input v-model="form.is_active" type="checkbox" role="switch" :disabled="!canWrite" />
          {{ $t('common.active', 'Active') }}
        </label>
      </div>
    </article>

    <!-- Members Card -->
    <article class="card">
      <header>
        <h3>{{ $t('teams.members', 'Members') }} ({{ members.length }})</h3>
      </header>
      <div class="vstack gap-4">
        <!-- Current Members -->
        <div v-if="members.length > 0" class="vstack gap-2">
          <div
            v-for="member in members"
            :key="member.user_id"
            class="hstack gap-2"
          >
            <figure data-variant="avatar" class="small" :aria-label="member.full_name">
              <abbr :title="member.full_name">{{ (member.full_name || '?')[0].toUpperCase() }}</abbr>
            </figure>
            <div class="flex-1 min-w-0">
              <p class="truncate"><strong>{{ member.full_name }}</strong></p>
              <small class="truncate text-light">{{ member.email }}</small>
            </div>
            <span class="badge outline shrink-0">
              <Shield v-if="member.role === 'manager'" :size="12" />
              {{ member.role }}
            </span>
            <IconButton
              v-if="canWrite"
              :icon="UserMinus"
              :label="$t('teams.removeMember', 'Remove Member')"
              @click="openRemoveMemberDialog(member)"
            />
          </div>
        </div>
        <p v-else class="text-light align-center">{{ $t('teams.noMembers', 'No members yet') }}</p>

        <!-- Add Members -->
        <template v-if="canWrite">
          <hr>
          <div class="vstack gap-2">
            <div class="hstack justify-between gap-2">
              <strong>{{ $t('teams.addMembers', 'Add Members') }}</strong>
              <input v-model="memberSearch" type="search" :placeholder="$t('teams.searchUsers', 'Search...')" :aria-label="$t('teams.searchUsers', 'Search...')" />
            </div>
            <div v-if="availableUsers.length > 0" class="vstack gap-2 scroll-y">
              <div
                v-for="user in availableUsers"
                :key="user.id"
                class="hstack gap-2"
              >
                <figure data-variant="avatar" class="small" :aria-label="user.full_name">
                  <abbr :title="user.full_name">{{ (user.full_name || '?')[0].toUpperCase() }}</abbr>
                </figure>
                <div class="flex-1 min-w-0">
                  <p class="truncate"><strong>{{ user.full_name }}</strong></p>
                  <small class="truncate text-light">{{ user.email }}</small>
                </div>
                <div class="hstack gap-1 shrink-0">
                  <button type="button" class="outline small" @click="addMember(user.id, 'agent')">
                    <UserPlus :size="12" /> {{ $t('teams.agent', 'Agent') }}
                  </button>
                  <button type="button" class="outline small" @click="addMember(user.id, 'manager')">
                    <Shield :size="12" /> {{ $t('teams.manager', 'Manager') }}
                  </button>
                </div>
              </div>
            </div>
            <small v-else-if="memberSearch" class="text-light align-center">{{ $t('teams.noMatchingUsers', 'No matching users') }}</small>
            <small v-else class="text-light align-center">{{ $t('teams.allUsersInTeam', 'All users are already in this team') }}</small>
          </div>
        </template>
      </div>
    </article>

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
  <Modal v-model:open="deleteDialogOpen" alert :title="$t('teams.deleteTeam', 'Delete Team')">
    <p>{{ $t('teams.deleteConfirm', 'Are you sure? This will remove all team members and cannot be undone.') }}</p>
    <template #footer>
      <button type="button" class="outline" @click="deleteDialogOpen = false">{{ $t('common.cancel') }}</button>
      <button type="button" data-variant="danger" @click="deleteTeam">{{ $t('common.delete') }}</button>
    </template>
  </Modal>

  <!-- Remove Member Confirmation -->
  <Modal v-model:open="removeMemberDialogOpen" alert :title="$t('teams.removeMember', 'Remove Member')">
    <p>{{ $t('teams.removeMemberConfirm', { name: memberToRemove?.full_name || '' }) }}</p>
    <template #footer>
      <button type="button" class="outline" @click="removeMemberDialogOpen = false">{{ $t('common.cancel') }}</button>
      <button type="button" data-variant="danger" @click="confirmRemoveMember">{{ $t('common.remove', 'Remove') }}</button>
    </template>
  </Modal>

  <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
