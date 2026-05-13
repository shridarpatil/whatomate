<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useRolesStore } from '@/stores/roles'
import { useAuthStore } from '@/stores/auth'
import type { Role } from '@/services/api'
import { toast } from 'vue-sonner'
import { getErrorMessage } from '@/lib/api-utils'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import UnsavedChangesDialog from '@/components/shared/UnsavedChangesDialog.vue'
import PermissionMatrix from '@/components/roles/PermissionMatrix.vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
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
import { Shield, Lock, Star, Save, Trash2, Loader2 } from 'lucide-vue-next'

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
      icon-gradient="bg-gradient-to-br from-purple-500 to-indigo-600 shadow-purple-500/20"
      back-link="/settings/roles"
      :breadcrumbs="breadcrumbs"
      :is-loading="isLoading"
      :is-not-found="isNotFound"
      :not-found-title="$t('roles.notFound', 'Role not found')"
    >
      <template #actions>
        <div class="flex items-center gap-2">
          <Button v-if="canEditForm && (hasChanges || isNew)" size="sm" @click="save" :disabled="isSaving">
            <Save class="h-4 w-4 mr-1" />
            {{ isSaving ? $t('common.saving', 'Saving...') : isNew ? $t('common.create') : $t('common.save') }}
          </Button>
          <Button
            v-if="canDelete && !isNew && !isSystem"
            variant="destructive"
            size="sm"
            @click="deleteDialogOpen = true"
          >
            <Trash2 class="h-4 w-4 mr-1" /> {{ $t('common.delete') }}
          </Button>
        </div>
      </template>

      <!-- Role Details Card -->
      <Card>
        <CardHeader class="pb-3">
          <div class="flex items-center justify-between">
            <CardTitle class="text-sm font-medium">{{ $t('teams.details', 'Details') }}</CardTitle>
            <div class="flex items-center gap-2">
              <Badge v-if="isSystem" variant="secondary">
                <Lock class="h-3 w-3 mr-1" />{{ $t('roles.system') }}
              </Badge>
              <Badge v-if="role?.is_default" variant="outline">
                <Star class="h-3 w-3 mr-1" />{{ $t('roles.default') }}
              </Badge>
            </div>
          </div>
        </CardHeader>
        <CardContent class="space-y-4">
          <p v-if="isSystem" class="text-xs text-muted-foreground">
            {{ isSuperAdmin ? $t('roles.superAdminCanEdit') : $t('roles.systemRoleViewOnly') }}
          </p>
          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('roles.name') }} <span class="text-destructive">*</span></Label>
            <Input v-model="form.name" :placeholder="$t('roles.namePlaceholder')" :disabled="!canEditForm || isSystem" />
          </div>
          <div class="space-y-1.5">
            <Label class="text-xs">{{ $t('roles.description') }}</Label>
            <Textarea v-model="form.description" :placeholder="$t('roles.descriptionPlaceholder')" :rows="2" :disabled="!canEditForm" />
          </div>
          <div v-if="!isSystem" class="flex items-center justify-between">
            <div class="space-y-0.5">
              <Label class="text-xs font-normal cursor-pointer">{{ $t('roles.defaultRole') }}</Label>
              <p class="text-[11px] text-muted-foreground">{{ $t('roles.defaultRoleDesc') }}</p>
            </div>
            <Switch :checked="form.is_default" @update:checked="form.is_default = $event" :disabled="!canEditForm" />
          </div>
        </CardContent>
      </Card>

      <!-- Permissions Card -->
      <Card>
        <CardHeader class="pb-3">
          <div class="flex items-center justify-between">
            <CardTitle class="text-sm font-medium">{{ $t('roles.permissions') }}</CardTitle>
            <span class="text-xs text-muted-foreground">
              {{ form.permissions.length }} {{ $t('common.selected') || 'selected' }}
            </span>
          </div>
        </CardHeader>
        <CardContent>
          <p class="text-sm text-muted-foreground mb-3">{{ $t('roles.selectPermissions') }}</p>
          <div v-if="rolesStore.permissions.length === 0" class="text-center py-8 text-muted-foreground border rounded-lg">
            <Loader2 class="h-6 w-6 animate-spin mx-auto mb-2" />
            <p>{{ $t('roles.loadingPermissions') }}...</p>
          </div>
          <PermissionMatrix
            v-else
            :key="role?.id || 'new'"
            :permission-groups="rolesStore.permissionGroups"
            v-model:selected-permissions="form.permissions"
            :disabled="!canEditPermissions"
          />
        </CardContent>
      </Card>

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
    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('roles.deleteRole') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('teams.deleteConfirm', 'Are you sure? This action cannot be undone.') }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('common.cancel') }}</AlertDialogCancel>
          <AlertDialogAction @click="deleteRole">{{ $t('common.delete') }}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <UnsavedChangesDialog :open="showLeaveDialog" @stay="cancelLeave" @leave="confirmLeave" />
  </div>
</template>
