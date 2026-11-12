<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useOrganizationsStore } from '@/stores/organizations'
import { useAuthStore } from '@/stores/auth'
import Modal from '@/components/oat/Modal.vue'
import { organizationsService } from '@/services/api'
import { toast } from '@/lib/toast'
import { Plus } from '@/lib/icons'

const { t } = useI18n()
const organizationsStore = useOrganizationsStore()
const authStore = useAuthStore()

const isSuperAdmin = computed(() => authStore.user?.is_super_admin || false)
const canCreateOrg = computed(() => authStore.hasPermission('organizations', 'write'))

const shouldShowSwitcher = computed(() =>
  isSuperAdmin.value || organizationsStore.isMultiOrg
)

// Build the org list depending on user type
const orgList = computed(() => {
  if (isSuperAdmin.value) {
    return organizationsStore.organizations.map(org => ({ id: org.id, name: org.name }))
  }
  return organizationsStore.myOrganizations.map(org => ({ id: org.organization_id, name: org.name }))
})

const currentOrgId = computed(() => {
  if (isSuperAdmin.value) {
    return organizationsStore.selectedOrgId || ''
  }
  return authStore.user?.organization_id || ''
})

onMounted(async () => {
  // Fetch user's org memberships for all authenticated users
  await organizationsStore.fetchMyOrganizations()

  if (isSuperAdmin.value) {
    organizationsStore.init()
    await organizationsStore.fetchOrganizations()

    // If no org selected, default to user's own org
    if (!organizationsStore.selectedOrgId && authStore.user?.organization_id) {
      organizationsStore.selectOrganization(authStore.user.organization_id)
    }
  }
})

// Watch for auth changes
watch(() => authStore.user?.is_super_admin, async (superAdmin) => {
  if (superAdmin) {
    organizationsStore.init()
    await organizationsStore.fetchOrganizations()
  }
})

const handleOrgChange = async (e: Event) => {
  const select = e.target as HTMLSelectElement
  const value = select.value
  if (!value) return

  // Everyone goes through switch-org, super admins included: the X-Organization-ID
  // header alone only scopes API client calls. The WebSocket token, <img src> media and
  // window.open previews carry no headers, so the JWT itself has to name the target
  // org or those keep talking to the previous one.
  try {
    await authStore.switchOrg(value)
    organizationsStore.selectOrganization(value)
    window.location.reload()
  } catch {
    // If switch fails, don't reload; put the native select back on the active org
    select.value = currentOrgId.value
  }
}

// Create org dialog
const isCreateDialogOpen = ref(false)
const newOrgName = ref('')
const isCreating = ref(false)

async function submitCreateOrg() {
  if (!newOrgName.value.trim()) return
  isCreating.value = true
  try {
    await organizationsService.create({ name: newOrgName.value.trim() })
    toast.success(t('organizations.created'))
    isCreateDialogOpen.value = false
    newOrgName.value = ''
    await refreshOrgs()
  } catch {
    toast.error(t('organizations.createFailed'))
  } finally {
    isCreating.value = false
  }
}

const refreshOrgs = async () => {
  if (isSuperAdmin.value) {
    await organizationsStore.fetchOrganizations()
  } else {
    await organizationsStore.fetchMyOrganizations()
  }
}
</script>

<template>
  <div v-if="shouldShowSwitcher" class="vstack gap-2 p-4">
    <div class="hstack justify-between">
      <small class="text-light">{{ t('organizations.label') }}</small>
      <button
        v-if="canCreateOrg"
        type="button"
        class="ghost small icon"
        :aria-label="t('organizations.createNew')"
        :title="t('organizations.createNew')"
        @click="isCreateDialogOpen = true"
      >
        <Plus :size="14" />
      </button>
    </div>
    <select
      v-if="orgList.length > 0"
      :value="currentOrgId"
      :aria-label="t('organizations.selectPlaceholder')"
      @change="handleOrgChange"
    >
      <option v-for="org in orgList" :key="org.id" :value="org.id">{{ org.name }}</option>
    </select>
    <small v-else-if="organizationsStore.loading" class="text-light">{{ t('organizations.loading') }}</small>
    <div v-else-if="organizationsStore.error" role="alert" data-variant="danger">{{ organizationsStore.error }}</div>
    <small v-else class="text-light">{{ t('organizations.none') }}</small>
  </div>

  <!-- Create Org Dialog -->
  <Modal
    v-model:open="isCreateDialogOpen"
    size="small"
    :title="t('organizations.createTitle')"
    :description="t('organizations.createDesc')"
  >
    <input
      v-model="newOrgName"
      :placeholder="t('organizations.namePlaceholder')"
      :aria-label="t('organizations.namePlaceholder')"
      @keydown.enter="submitCreateOrg"
    />
    <template #footer>
      <button type="button" class="outline" @click="isCreateDialogOpen = false">{{ t('common.cancel') }}</button>
      <button type="button" :aria-busy="isCreating" :disabled="isCreating || !newOrgName.trim()" @click="submitCreateOrg">
        {{ t('common.create') }}
      </button>
    </template>
  </Modal>
</template>
