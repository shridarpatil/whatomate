<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { PageHeader, SearchInput, DataTable, DeleteConfirmDialog, type Column } from '@/components/shared'
import { useAuthStore } from '@/stores/auth'
import { useOrganizationsStore } from '@/stores/organizations'
import { organizationsService, rolesService, usersService } from '@/services/api'
import { toast } from 'vue-sonner'
import { Plus, Pencil, Trash2, Loader2, Building2 } from 'lucide-vue-next'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDate } from '@/lib/utils'
import { useDebounceFn } from '@vueuse/core'

const { t } = useI18n()

const authStore = useAuthStore()
const organizationsStore = useOrganizationsStore()

// Member type from API
interface OrgMember {
  user_id: string
  full_name: string
  email: string
  role_id: string
  role_name: string
  created_at: string
}

// Available users & roles for add/edit dialogs
interface UserOption {
  id: string
  full_name: string
  email: string
}

interface RoleOption {
  id: string
  name: string
}

// State
const members = ref<OrgMember[]>([])
const isLoading = ref(true)
const searchQuery = ref('')
const users = ref<UserOption[]>([])
const roles = ref<RoleOption[]>([])

// Add member dialog
const isAddDialogOpen = ref(false)
const addSelectedUserId = ref('')
const addSelectedRoleId = ref('')
const isAddSubmitting = ref(false)

// Edit role dialog
const isEditDialogOpen = ref(false)
const editMember = ref<OrgMember | null>(null)
const editSelectedRoleId = ref('')
const isEditSubmitting = ref(false)

// Delete member dialog
const deleteDialogOpen = ref(false)
const memberToDelete = ref<OrgMember | null>(null)

const canManageMembers = computed(() =>
  authStore.hasPermission('organizations', 'assign')
)

const breadcrumbs = computed(() => [
  { label: t('nav.settings'), href: '/settings' },
  { label: t('nav.members') },
])

const columns = computed<Column<OrgMember>[]>(() => [
  { key: 'member', label: t('members.member'), width: 'w-[300px]', sortable: true, sortKey: 'full_name' },
  { key: 'role', label: t('members.role'), sortable: true, sortKey: 'role_name' },
  { key: 'joined', label: t('members.joined'), sortable: true, sortKey: 'created_at' },
  { key: 'actions', label: t('common.actions'), align: 'right' },
])

const sortKey = ref('full_name')
const sortDirection = ref<'asc' | 'desc'>('asc')

// Filtered members based on search
const filteredMembers = computed(() => {
  if (!searchQuery.value) return members.value
  const q = searchQuery.value.toLowerCase()
  return members.value.filter(
    m => m.full_name.toLowerCase().includes(q) || m.email.toLowerCase().includes(q)
  )
})

// Users not yet in the org (for add dialog)
const availableUsers = computed(() => {
  const memberUserIds = new Set(members.value.map(m => m.user_id))
  return users.value.filter(u => !memberUserIds.has(u.id))
})

const debouncedSearch = useDebounceFn(() => {
  // Client-side filtering, no API call needed
}, 300)

watch(searchQuery, () => debouncedSearch())
watch(() => organizationsStore.selectedOrgId, () => fetchMembers())

onMounted(async () => {
  await Promise.all([fetchMembers(), fetchUsersAndRoles()])
})

async function fetchMembers() {
  isLoading.value = true
  try {
    const response = await organizationsService.listMembers()
    members.value = (response.data as any).data?.members || []
  } catch {
    toast.error(t('common.failedLoad', { resource: t('resources.members') }))
    members.value = []
  } finally {
    isLoading.value = false
  }
}

async function fetchUsersAndRoles() {
  try {
    const [usersRes, rolesRes] = await Promise.all([
      usersService.list({ limit: 500 }),
      rolesService.list(),
    ])
    users.value = ((usersRes.data as any).data?.users || []).map((u: any) => ({
      id: u.id,
      full_name: u.full_name,
      email: u.email,
    }))
    roles.value = ((rolesRes.data as any).data?.roles || []).map((r: any) => ({
      id: r.id,
      name: r.name,
    }))
  } catch {
    // Non-critical — dialogs just won't have options
  }
}

// Add member
function openAddDialog() {
  addSelectedUserId.value = ''
  addSelectedRoleId.value = ''
  isAddDialogOpen.value = true
}

async function submitAddMember() {
  if (!addSelectedUserId.value) {
    toast.error(t('members.selectUser'))
    return
  }
  isAddSubmitting.value = true
  try {
    await organizationsService.addMember({
      user_id: addSelectedUserId.value,
      role_id: addSelectedRoleId.value || undefined,
    })
    toast.success(t('members.memberAdded'))
    isAddDialogOpen.value = false
    await fetchMembers()
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedSave', { resource: t('resources.member') })))
  } finally {
    isAddSubmitting.value = false
  }
}

// Edit role
function openEditDialog(member: OrgMember) {
  editMember.value = member
  editSelectedRoleId.value = member.role_id || ''
  isEditDialogOpen.value = true
}

async function submitEditRole() {
  if (!editMember.value || !editSelectedRoleId.value) return
  isEditSubmitting.value = true
  try {
    await organizationsService.updateMemberRole(editMember.value.user_id, {
      role_id: editSelectedRoleId.value,
    })
    toast.success(t('members.roleUpdated'))
    isEditDialogOpen.value = false
    await fetchMembers()
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedUpdate', { resource: t('resources.member') })))
  } finally {
    isEditSubmitting.value = false
  }
}

// Remove member
function openDeleteDialog(member: OrgMember) {
  if (member.user_id === authStore.user?.id) {
    toast.error(t('members.cannotRemoveSelf'))
    return
  }
  memberToDelete.value = member
  deleteDialogOpen.value = true
}

async function confirmDelete() {
  if (!memberToDelete.value) return
  try {
    await organizationsService.removeMember(memberToDelete.value.user_id)
    toast.success(t('members.memberRemoved'))
    deleteDialogOpen.value = false
    memberToDelete.value = null
    await fetchMembers()
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.member') })))
  }
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <PageHeader
      :title="$t('members.title')"
      :icon="Building2"
      icon-gradient="bg-gradient-to-br from-violet-500 to-purple-600 shadow-violet-500/20"
      back-link="/settings"
      :breadcrumbs="breadcrumbs"
    >
      <template #actions>
        <Button v-if="canManageMembers" variant="outline" size="sm" @click="openAddDialog">
          <Plus class="h-4 w-4 mr-2" />{{ $t('members.addMember') }}
        </Button>
      </template>
    </PageHeader>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto">
          <Card>
            <CardHeader>
              <div class="flex items-center justify-between flex-wrap gap-4">
                <div>
                  <CardTitle>{{ $t('members.yourMembers') }}</CardTitle>
                  <CardDescription>{{ $t('members.yourMembersDesc') }}</CardDescription>
                </div>
                <SearchInput v-model="searchQuery" :placeholder="$t('members.searchMembers') + '...'" class="w-64" />
              </div>
            </CardHeader>
            <CardContent>
              <DataTable
                :items="filteredMembers"
                :columns="columns"
                :is-loading="isLoading"
                :empty-icon="Building2"
                :empty-title="searchQuery ? $t('members.noMatchingMembers') : $t('members.noMembersYet')"
                :empty-description="searchQuery ? $t('members.noMatchingMembersDesc') : $t('members.noMembersYetDesc')"
                v-model:sort-key="sortKey"
                v-model:sort-direction="sortDirection"
                item-name="members"
              >
                <template #empty-action>
                  <Button v-if="canManageMembers" variant="outline" size="sm" @click="openAddDialog">
                    <Plus class="h-4 w-4 mr-2" />{{ $t('members.addMember') }}
                  </Button>
                </template>
                <template #cell-member="{ item: member }">
                  <div class="flex items-center gap-3">
                    <div class="h-9 w-9 rounded-full bg-primary/10 flex items-center justify-center flex-shrink-0">
                      <span class="text-sm font-medium text-primary">{{ member.full_name?.charAt(0) || '?' }}</span>
                    </div>
                    <div class="min-w-0">
                      <p class="font-medium truncate">{{ member.full_name }}</p>
                      <p class="text-sm text-muted-foreground truncate">{{ member.email }}</p>
                    </div>
                  </div>
                </template>
                <template #cell-role="{ item: member }">
                  <Badge variant="outline">{{ member.role_name || '-' }}</Badge>
                </template>
                <template #cell-joined="{ item: member }">
                  <span class="text-muted-foreground">{{ formatDate(member.created_at) }}</span>
                </template>
                <template #cell-actions="{ item: member }">
                  <div v-if="canManageMembers" class="flex items-center justify-end gap-1">
                    <Tooltip>
                      <TooltipTrigger as-child>
                        <Button variant="ghost" size="icon" class="h-8 w-8" @click="openEditDialog(member)">
                          <Pencil class="h-4 w-4" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>{{ $t('members.editRole') }}</TooltipContent>
                    </Tooltip>
                    <Tooltip>
                      <TooltipTrigger as-child>
                        <Button variant="ghost" size="icon" class="h-8 w-8" @click="openDeleteDialog(member)">
                          <Trash2 class="h-4 w-4 text-destructive" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>{{ $t('members.removeMember') }}</TooltipContent>
                    </Tooltip>
                  </div>
                </template>
              </DataTable>
            </CardContent>
          </Card>
        </div>
      </div>
    </ScrollArea>

    <!-- Add Member Dialog -->
    <Dialog v-model:open="isAddDialogOpen">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>{{ $t('members.addMemberTitle') }}</DialogTitle>
          <DialogDescription>{{ $t('members.addMemberDesc') }}</DialogDescription>
        </DialogHeader>
        <div class="space-y-4 py-4">
          <div class="space-y-2">
            <Label>{{ $t('members.selectUser') }}</Label>
            <Select v-model="addSelectedUserId">
              <SelectTrigger>
                <SelectValue :placeholder="$t('members.selectUser')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="user in availableUsers" :key="user.id" :value="user.id">
                  <div class="flex flex-col">
                    <span>{{ user.full_name }}</span>
                    <span class="text-xs text-muted-foreground">{{ user.email }}</span>
                  </div>
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="space-y-2">
            <Label>{{ $t('members.selectRole') }}</Label>
            <Select v-model="addSelectedRoleId">
              <SelectTrigger>
                <SelectValue :placeholder="$t('members.selectRole')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="role in roles" :key="role.id" :value="role.id">
                  {{ role.name }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="isAddDialogOpen = false">{{ $t('common.cancel') }}</Button>
          <Button @click="submitAddMember" :disabled="isAddSubmitting || !addSelectedUserId">
            <Loader2 v-if="isAddSubmitting" class="h-4 w-4 mr-2 animate-spin" />
            {{ $t('members.addMember') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Edit Role Dialog -->
    <Dialog v-model:open="isEditDialogOpen">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>{{ $t('members.editRoleTitle') }}</DialogTitle>
          <DialogDescription>{{ $t('members.editRoleDesc') }}</DialogDescription>
        </DialogHeader>
        <div class="space-y-4 py-4">
          <div v-if="editMember" class="flex items-center gap-3 p-3 bg-muted/50 rounded-lg">
            <div class="h-9 w-9 rounded-full bg-primary/10 flex items-center justify-center flex-shrink-0">
              <span class="text-sm font-medium text-primary">{{ editMember.full_name?.charAt(0) || '?' }}</span>
            </div>
            <div>
              <p class="font-medium">{{ editMember.full_name }}</p>
              <p class="text-sm text-muted-foreground">{{ editMember.email }}</p>
            </div>
          </div>
          <div class="space-y-2">
            <Label>{{ $t('members.role') }}</Label>
            <Select v-model="editSelectedRoleId">
              <SelectTrigger>
                <SelectValue :placeholder="$t('members.selectRole')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="role in roles" :key="role.id" :value="role.id">
                  {{ role.name }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="isEditDialogOpen = false">{{ $t('common.cancel') }}</Button>
          <Button @click="submitEditRole" :disabled="isEditSubmitting || !editSelectedRoleId">
            <Loader2 v-if="isEditSubmitting" class="h-4 w-4 mr-2 animate-spin" />
            {{ $t('common.update') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Delete Member Confirm -->
    <DeleteConfirmDialog
      v-model:open="deleteDialogOpen"
      :title="$t('members.removeMember')"
      :item-name="memberToDelete?.full_name"
      :description="$t('members.removeMemberWarning')"
      @confirm="confirmDelete"
    />
  </div>
</template>
