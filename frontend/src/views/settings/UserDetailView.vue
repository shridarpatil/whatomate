<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useUsersStore, type User } from '@/stores/users'
import { useAuthStore } from '@/stores/auth'
import { useRolesStore } from '@/stores/roles'
import { toast } from '@/lib/toast'
import { getErrorMessage } from '@/lib/api-utils'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import Modal from '@/components/oat/Modal.vue'
import {
  User as UserIcon,
  Shield,
  ShieldCheck,
  UserCog,
  Trash2,
  Save,
} from '@/lib/icons'

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
      back-link="/settings/users"
      :breadcrumbs="breadcrumbs"
      :is-loading="isLoading"
      :is-not-found="isNotFound"
      :not-found-title="$t('users.notFound', 'User not found')"
    >
      <template #actions>
        <div class="hstack gap-2">
          <button v-if="canWrite && hasChanges" type="button" class="small" :disabled="isSaving" :aria-busy="isSaving ? 'true' : undefined" data-spinner="small" @click="save">
            <Save v-if="!isSaving" :size="16" /> {{ isSaving ? $t('common.saving', 'Saving...') : $t('common.save') }}
          </button>
          <button v-if="canDelete && !isSelf" type="button" class="small" data-variant="danger" @click="deleteDialogOpen = true">
            <Trash2 :size="16" />
            {{ isMember ? $t('users.removeMember') : $t('common.delete') }}
          </button>
        </div>
      </template>

      <!-- User Details Card -->
      <article class="card">
        <header class="hstack justify-between">
          <h3>{{ $t('teams.details', 'Details') }}</h3>
          <div class="hstack gap-2">
            <span v-if="isSelf" class="badge outline">{{ $t('users.you') }}</span>
            <span v-if="user?.is_super_admin" class="badge">{{ $t('users.superAdmin') }}</span>
            <span v-if="isMember" class="badge" data-variant="secondary">{{ $t('users.member') }}</span>
            <span class="badge" :data-variant="(user?.is_active ?? true) ? 'success' : 'secondary'">
              {{ (user?.is_active ?? true) ? $t('common.active') : $t('common.inactive') }}
            </span>
          </div>
        </header>
        <div class="vstack gap-4">
          <div class="hstack gap-2">
            <component :is="getRoleIcon(user?.role?.name || '')" :size="24" />
            <div class="min-w-0">
              <p class="truncate"><strong>{{ user?.full_name }}</strong></p>
              <p class="truncate text-light">{{ user?.email }}</p>
            </div>
          </div>

          <label data-field>
            {{ $t('users.fullName') }} *
            <input v-model="form.full_name" :disabled="!canWrite || isMember" />
          </label>
          <label data-field>
            {{ $t('common.email') }} *
            <input v-model="form.email" type="email" :disabled="!canWrite || isMember" />
          </label>
          <label v-if="!isMember" data-field>
            <span>
              {{ $t('users.password') }}
              <small class="text-light">{{ $t('users.keepExisting') }}</small>
            </span>
            <input v-model="form.password" type="password" :placeholder="$t('users.passwordPlaceholder')" :disabled="!canWrite" />
          </label>
          <label data-field>
            {{ $t('users.role') }} *
            <select v-model="form.role_id" :disabled="!canWrite">
              <option value="" disabled>{{ $t('users.selectRole') }}</option>
              <option v-for="role in rolesStore.roles" :key="role.id" :value="role.id">{{ role.name }}{{ role.is_system ? ` (${$t('users.system')})` : '' }}</option>
            </select>
          </label>
          <label data-field>
            <span class="hstack justify-between">
              <span>{{ $t('users.accountActive') }}</span>
              <input v-model="form.is_active" type="checkbox" role="switch" :disabled="!canWrite || isSelf" />
            </span>
          </label>
          <label v-if="isSuperAdmin" data-field>
            <span class="hstack justify-between">
              <span>{{ $t('users.superAdminLabel') }}</span>
              <input v-model="form.is_super_admin" type="checkbox" role="switch" :disabled="!canWrite || (isSelf && user?.is_super_admin)" />
            </span>
            <small data-hint>{{ $t('users.superAdminDesc') }}</small>
          </label>
        </div>
      </article>

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
    <Modal v-model:open="deleteDialogOpen" alert :title="isMember ? $t('users.removeMember') : $t('users.deleteUser')">
      <p>{{ isMember ? $t('users.removeMemberWarning') : $t('teams.deleteConfirm', 'Are you sure? This action cannot be undone.') }}</p>
      <template #footer>
        <button type="button" class="outline" @click="deleteDialogOpen = false">{{ $t('common.cancel') }}</button>
        <button type="button" data-variant="danger" @click="deleteUser">
          {{ isMember ? $t('common.remove', 'Remove') : $t('common.delete') }}
        </button>
      </template>
    </Modal>

    <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
