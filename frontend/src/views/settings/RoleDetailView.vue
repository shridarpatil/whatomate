<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useRolesStore } from '@/stores/roles'
import { useAuthStore } from '@/stores/auth'
import type { Role } from '@/services/api'
import { toast } from '@/lib/toast'
import { getErrorMessage } from '@/lib/api-utils'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import PermissionMatrix from '@/components/roles/PermissionMatrix.vue'
import Modal from '@/components/oat/Modal.vue'
import { Shield, Lock, Star, Save, Trash2 } from '@/lib/icons'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const rolesStore = useRolesStore()
const authStore = useAuthStore()

const roleId = computed(() => route.params.id as string)
const isNew = computed(() => roleId.value === 'new')
const role = ref<Role | null>(null)
const isLoading = ref(true)
const isNotFound = ref(false)
const isSaving = ref(false)
const hasChanges = ref(false)
const deleteDialogOpen = ref(false)

const { showLeaveDialog, confirmLeave, cancelLeave } = useUnsavedChangesGuard(hasChanges)

const isSuperAdmin = computed(() => authStore.user?.is_super_admin ?? false)
const canWrite = computed(() => authStore.hasPermission('roles', 'write'))
const canDelete = computed(() => authStore.hasPermission('roles', 'delete'))
const isSystem = computed(() => role.value?.is_system ?? false)
const canEditPermissions = computed(() => !isSystem.value || isSuperAdmin.value)
const canEditForm = computed(() => canWrite.value && (!isSystem.value || isSuperAdmin.value))

const form = ref({
  name: '',
  description: '',
  is_default: false,
  permissions: [] as string[],
})

const breadcrumbs = computed(() => [
  { label: t('nav.settings'), href: '/settings' },
  { label: t('nav.roles'), href: '/settings/roles' },
  { label: isNew.value ? t('roles.createRole') : (role.value?.name || '') },
])

async function loadRole() {
  isLoading.value = true
  isNotFound.value = false
  try {
    const data = await rolesStore.fetchRole(roleId.value)
    role.value = data
    syncForm()
    nextTick(() => { hasChanges.value = false })
  } catch {
    isNotFound.value = true
  } finally {
    isLoading.value = false
  }
}

function syncForm() {
  if (!role.value) return
  form.value = {
    name: role.value.name,
    description: role.value.description || '',
    is_default: role.value.is_default,
    permissions: [...role.value.permissions],
  }
}

watch(form, () => {
  if (!role.value && !isNew.value) return
  hasChanges.value = true
}, { deep: true })

async function save() {
  if (!form.value.name.trim()) {
    toast.error(t('roles.roleNameRequired'))
    return
  }
  isSaving.value = true
  try {
    if (isNew.value) {
      const created = await rolesStore.createRole({
        name: form.value.name,
        description: form.value.description,
        is_default: form.value.is_default,
        permissions: form.value.permissions,
      })
      hasChanges.value = false
      toast.success(t('common.createdSuccess', { resource: t('resources.Role') }))
      router.replace(`/settings/roles/${created.id}`)
    } else {
      await rolesStore.updateRole(role.value!.id, {
        name: form.value.name,
        description: form.value.description,
        is_default: form.value.is_default,
        permissions: form.value.permissions,
      })
      await loadRole()
      hasChanges.value = false
      toast.success(t('common.updatedSuccess', { resource: t('resources.Role') }))
    }
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedSave', { resource: t('resources.role') })))
  } finally {
    isSaving.value = false
  }
}

async function deleteRole() {
  if (!role.value) return
  try {
    await rolesStore.deleteRole(role.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.Role') }))
    router.push('/settings/roles')
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.role') })))
  }
  deleteDialogOpen.value = false
}

onMounted(async () => {
  await rolesStore.fetchPermissions()
  if (isNew.value) {
    isLoading.value = false
    hasChanges.value = false
  } else {
    await loadRole()
  }
})
</script>

<template>
  <div class="h-full">
    <DetailPageLayout
      :title="isNew ? t('roles.createRole') : (role?.name || '')"
      :icon="Shield"
      back-link="/settings/roles"
      :breadcrumbs="breadcrumbs"
      :is-loading="isLoading"
      :is-not-found="isNotFound"
      :not-found-title="$t('roles.notFound', 'Role not found')"
    >
      <template #actions>
        <div class="hstack gap-2">
          <button v-if="canEditForm && (hasChanges || isNew)" type="button" class="small" :disabled="isSaving" :aria-busy="isSaving ? 'true' : undefined" data-spinner="small" @click="save">
            <Save v-if="!isSaving" :size="16" />
            {{ isSaving ? $t('common.saving', 'Saving...') : isNew ? $t('common.create') : $t('common.save') }}
          </button>
          <button v-if="canDelete && !isNew && !isSystem" type="button" class="small" data-variant="danger" @click="deleteDialogOpen = true">
            <Trash2 :size="16" /> {{ $t('common.delete') }}
          </button>
        </div>
      </template>

      <!-- Role Details Card -->
      <article class="card">
        <header class="hstack justify-between">
          <h3>{{ $t('teams.details', 'Details') }}</h3>
          <div class="hstack gap-2">
            <span v-if="isSystem" class="badge" data-variant="secondary"><Lock :size="12" />{{ $t('roles.system') }}</span>
            <span v-if="role?.is_default" class="badge outline"><Star :size="12" />{{ $t('roles.default') }}</span>
          </div>
        </header>
        <div class="vstack gap-4">
          <p v-if="isSystem" class="text-light">
            {{ isSuperAdmin ? $t('roles.superAdminCanEdit') : $t('roles.systemRoleViewOnly') }}
          </p>
          <label data-field>
            {{ $t('roles.name') }} *
            <input v-model="form.name" :placeholder="$t('roles.namePlaceholder')" :disabled="!canEditForm || isSystem" />
          </label>
          <label data-field>
            {{ $t('roles.description') }}
            <textarea v-model="form.description" :placeholder="$t('roles.descriptionPlaceholder')" rows="2" :disabled="!canEditForm" />
          </label>
          <label v-if="!isSystem" data-field>
            <span class="hstack justify-between">
              <span>{{ $t('roles.defaultRole') }}</span>
              <input v-model="form.is_default" type="checkbox" role="switch" :disabled="!canEditForm" />
            </span>
            <small data-hint>{{ $t('roles.defaultRoleDesc') }}</small>
          </label>
        </div>
      </article>

      <!-- Permissions Card -->
      <article class="card">
        <header class="hstack justify-between">
          <h3>{{ $t('roles.permissions') }}</h3>
          <small class="text-light">
            {{ form.permissions.length }} {{ $t('common.selected') || 'selected' }}
          </small>
        </header>
        <p class="text-light mb-4">{{ $t('roles.selectPermissions') }}</p>
        <div v-if="rolesStore.permissions.length === 0" class="vstack items-center gap-2 p-4" role="status" aria-busy="true">
          <span data-spinner="large" aria-hidden="true" />
          <p class="text-light">{{ $t('roles.loadingPermissions') }}...</p>
        </div>
        <PermissionMatrix
          v-else
          :key="role?.id || 'new'"
          :permission-groups="rolesStore.permissionGroups"
          v-model:selected-permissions="form.permissions"
          :disabled="!canEditPermissions"
        />
      </article>

      <!-- Activity Log -->
      <AuditLogPanel
        v-if="role && !isNew"
        resource-type="role"
        :resource-id="role.id"
      />

      <!-- Sidebar -->
      <template v-if="!isNew" #sidebar>
        <MetadataPanel
          :created-at="role?.created_at"
          :updated-at="role?.updated_at"
        />
      </template>
    </DetailPageLayout>

    <!-- Delete Confirmation -->
    <Modal v-model:open="deleteDialogOpen" alert :title="$t('roles.deleteRole')">
      <p>{{ $t('teams.deleteConfirm', 'Are you sure? This action cannot be undone.') }}</p>
      <template #footer>
        <button type="button" class="outline" @click="deleteDialogOpen = false">{{ $t('common.cancel') }}</button>
        <button type="button" data-variant="danger" @click="deleteRole">{{ $t('common.delete') }}</button>
      </template>
    </Modal>

    <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
