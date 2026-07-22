<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useUsersStore, type User } from '@/stores/users'
import { useAuthStore } from '@/stores/auth'
import { useRolesStore } from '@/stores/roles'
import { toast } from 'vue-sonner'
import { getErrorMessage } from '@/lib/api-utils'
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
import { Switch } from '@/components/ui/switch'
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
  User as UserIcon,
  Shield,
  ShieldCheck,
  UserCog,
  Trash2,
  Save,
} from 'lucide-vue-next'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const usersStore = useUsersStore()
const authStore = useAuthStore()
const rolesStore = useRolesStore()

const userId = computed(() => route.params.id as string)
const user = ref<User | null>(null)
const isLoading = ref(true)
const isNotFound = ref(false)
const isSaving = ref(false)
const hasChanges = ref(false)
const deleteDialogOpen = ref(false)

const { showLeaveDialog, confirmLeave, cancelLeave } = useUnsavedChangesGuard(hasChanges)

const canWrite = computed(() => authStore.hasPermission('users', 'write'))
const canDelete = computed(() => authStore.hasPermission('users', 'delete'))
const currentUserId = computed(() => authStore.user?.id)
const isSuperAdmin = computed(() => authStore.user?.is_super_admin || false)
const isSelf = computed(() => user.value?.id === currentUserId.value)
const isMember = computed(() => user.value?.is_member || false)

const form = ref({
  full_name: '',
  email: '',
  password: '',
  role_id: '',
  is_active: true,
  is_super_admin: false,
})

const breadcrumbs = computed(() => [
  { label: t('nav.settings'), href: '/settings' },
  { label: t('nav.users'), href: '/settings/users' },
  { label: user.value?.full_name || '' },
])

function getRoleIcon(name: string) {
  return { admin: ShieldCheck, manager: Shield }[name.toLowerCase()] || UserCog
}

async function loadUser() {
  isLoading.value = true
  isNotFound.value = false
  try {
    const data = await usersStore.fetchUser(userId.value)
    user.value = data
    syncForm()
    nextTick(() => { hasChanges.value = false })
  } catch {
    isNotFound.value = true
  } finally {
    isLoading.value = false
  }
}

function syncForm() {
  if (!user.value) return
  form.value = {
    full_name: user.value.full_name,
    email: user.value.email,
    password: '',
    role_id: user.value.role_id || '',
    is_active: user.value.is_active,
    is_super_admin: user.value.is_super_admin || false,
  }
}

watch(form, () => {
  if (!user.value) return
  hasChanges.value = true
}, { deep: true })

async function save() {
  if (!user.value) return
  if (!form.value.full_name.trim() || !form.value.email.trim()) {
    toast.error(t('users.fillEmailName'))
    return
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.value.email.trim())) {
    toast.error(t('validation.email'))
    return
  }
  if (!form.value.role_id) {
    toast.error(t('users.selectRoleRequired'))
    return
  }

  isSaving.value = true
  try {
    const data: Record<string, unknown> = {
      email: form.value.email,
      full_name: form.value.full_name,
      role_id: form.value.role_id,
      is_active: form.value.is_active,
    }
    if (form.value.password) data.password = form.value.password
    if (isSuperAdmin.value) data.is_super_admin = form.value.is_super_admin

    await usersStore.updateUser(user.value.id, data)
    toast.success(t('common.updatedSuccess', { resource: t('resources.User') }))
    await loadUser()
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedSave', { resource: t('resources.user') })))
  } finally {
    isSaving.value = false
  }
}

async function deleteUser() {
  if (!user.value) return
  try {
    await usersStore.deleteUser(user.value.id)
    toast.success(isMember.value
      ? t('users.memberRemoved')
      : t('common.deletedSuccess', { resource: t('resources.User') }))
    router.push('/settings/users')
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.user') })))
  }
  deleteDialogOpen.value = false
}

onMounted(async () => {
  await Promise.all([loadUser(), rolesStore.fetchRoles()])
})
</script>

<template>
  <div class="h-full">
    <DetailPageLayout
      :title="user?.full_name || ''"
      :icon="UserIcon"
      icon-gradient="bg-gradient-to-br from-blue-500 to-indigo-600 shadow-blue-500/20"
      back-link="/settings/users"
      :breadcrumbs="breadcrumbs"
      :is-loading="isLoading"
      :is-not-found="isNotFound"
      :not-found-title="$t('users.notFound', 'User not found')"
    >
      <template #actions>
        <div class="flex items-center gap-2">
          <Button v-if="canWrite && hasChanges" size="sm" @click="save" :disabled="isSaving">
            <Save class="h-4 w-4 mr-1" /> {{ isSaving ? $t('common.saving', 'Saving...') : $t('common.save') }}
          </Button>
          <Button
            v-if="canDelete && !isSelf"
            variant="destructive"
            size="sm"
            @click="deleteDialogOpen = true"
          >
            <Trash2 class="h-4 w-4 mr-1" />
            {{ isMember ? $t('users.removeMember') : $t('common.delete') }}
          </Button>
        </div>
      </template>

      <!-- User Details Card -->
      <Card>
        <CardHeader class="pb-3">
          <div class="flex items-center justify-between">
            <CardTitle class="text-sm font-medium">{{ $t('teams.details', 'Details') }}</CardTitle>
            <div class="flex items-center gap-2">
              <Badge v-if="isSelf" variant="outline">{{ $t('users.you') }}</Badge>
              <Badge v-if="user?.is_super_admin" variant="default">{{ $t('users.superAdmin') }}</Badge>
              <Badge v-if="isMember" variant="secondary">{{ $t('users.member') }}</Badge>
              <Badge :variant="(user?.is_active ?? true) ? 'default' : 'secondary'">
                {{ (user?.is_active ?? true) ? $t('common.active') : $t('common.inactive') }}
              </Badge>
            </div>
          </div>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="flex items-center gap-3 p-3 rounded-lg bg-muted/50">
            <div class="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center shrink-0">
              <component :is="getRoleIcon(user?.role?.name || '')" class="h-5 w-5 text-primary" />
            </div>
            <div class="min-w-0">
              <p class="font-medium truncate">{{ user?.full_name }}</p>
              <p class="text-sm text-muted-foreground truncate">{{ user?.email }}</p>
            </div>
          </div>

          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('users.fullName') }} <span class="text-destructive">*</span></Label>
            <Input v-model="form.full_name" :disabled="!canWrite || isMember" />
          </div>
          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('common.email') }} <span class="text-destructive">*</span></Label>
            <Input v-model="form.email" type="email" :disabled="!canWrite || isMember" />
          </div>
          <div v-if="!isMember" class="space-y-1.5">
            <Label class="text-xs">
              {{ $t('users.password') }}
              <span class="text-muted-foreground">{{ $t('users.keepExisting') }}</span>
            </Label>
            <Input v-model="form.password" type="password" :placeholder="$t('users.passwordPlaceholder')" :disabled="!canWrite" />
          </div>
          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('users.role') }} <span class="text-destructive">*</span></Label>
            <Select v-model="form.role_id" :disabled="!canWrite">
              <SelectTrigger>
                <SelectValue :placeholder="$t('users.selectRole')">
                  <template v-if="form.role_id">
                    <span class="capitalize">{{ rolesStore.roles.find(r => r.id === form.role_id)?.name }}</span>
                  </template>
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="role in rolesStore.roles" :key="role.id" :value="role.id">
                  <div class="flex items-center gap-2">
                    <span class="capitalize">{{ role.name }}</span>
                    <Badge v-if="role.is_system" variant="secondary" class="text-xs">{{ $t('users.system') }}</Badge>
                  </div>
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="flex items-center justify-between">
            <Label class="text-xs font-normal cursor-pointer">{{ $t('users.accountActive') }}</Label>
            <Switch
              :checked="form.is_active"
              @update:checked="form.is_active = $event"
              :disabled="!canWrite || isSelf"
            />
          </div>
          <div v-if="isSuperAdmin" class="flex items-center justify-between border-t pt-4">
            <div>
              <Label class="text-xs font-normal cursor-pointer">{{ $t('users.superAdminLabel') }}</Label>
              <p class="text-[11px] text-muted-foreground">{{ $t('users.superAdminDesc') }}</p>
            </div>
            <Switch
              :checked="form.is_super_admin"
              @update:checked="form.is_super_admin = $event"
              :disabled="!canWrite || (isSelf && user?.is_super_admin)"
            />
          </div>
        </CardContent>
      </Card>

      <!-- Activity Log -->
      <AuditLogPanel
        v-if="user"
        resource-type="user"
        :resource-id="user.id"
      />

      <!-- Sidebar -->
      <template #sidebar>
        <MetadataPanel
          :created-at="user?.created_at"
          :updated-at="user?.updated_at"
        />
      </template>
    </DetailPageLayout>

    <!-- Delete Confirmation -->
    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {{ isMember ? $t('users.removeMember') : $t('users.deleteUser') }}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {{ isMember ? $t('users.removeMemberWarning') : $t('teams.deleteConfirm', 'Are you sure? This action cannot be undone.') }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('common.cancel') }}</AlertDialogCancel>
          <AlertDialogAction @click="deleteUser">
            {{ isMember ? $t('common.remove', 'Remove') : $t('common.delete') }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
