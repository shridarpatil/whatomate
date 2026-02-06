<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useOrganizationsStore } from '@/stores/organizations'
import { useAuthStore } from '@/stores/auth'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Building2, RefreshCw } from 'lucide-vue-next'

const props = defineProps<{
  collapsed?: boolean
}>()

const organizationsStore = useOrganizationsStore()
const authStore = useAuthStore()
const isRefreshing = ref(false)

const isSuperAdmin = computed(() => authStore.user?.is_super_admin || false)

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

const handleOrgChange = async (value: string | number | bigint | Record<string, any> | null) => {
  if (!value || typeof value !== 'string') return

  if (isSuperAdmin.value) {
    // Super admins: set localStorage header and reload
    organizationsStore.selectOrganization(value)
    window.location.reload()
  } else {
    // Multi-org users: call switchOrg API for new JWT tokens, then reload
    try {
      await authStore.switchOrg(value)
      window.location.reload()
    } catch {
      // If switch fails, don't reload
    }
  }
}

const refreshOrgs = async () => {
  isRefreshing.value = true
  if (isSuperAdmin.value) {
    await organizationsStore.fetchOrganizations()
  } else {
    await organizationsStore.fetchMyOrganizations()
  }
  isRefreshing.value = false
}
</script>

<template>
  <div v-if="shouldShowSwitcher" class="px-2 py-2 border-b">
    <div v-if="!collapsed" class="space-y-1">
      <div class="flex items-center justify-between">
        <span class="text-[11px] font-medium text-muted-foreground uppercase tracking-wide px-1">
          Organization
        </span>
        <Button
          variant="ghost"
          size="icon"
          class="h-5 w-5"
          @click="refreshOrgs"
          :disabled="isRefreshing"
        >
          <RefreshCw :class="['h-3 w-3', isRefreshing && 'animate-spin']" />
        </Button>
      </div>
      <Select
        v-if="orgList.length > 0"
        :model-value="currentOrgId"
        @update:model-value="handleOrgChange"
      >
        <SelectTrigger class="h-8 text-[13px]">
          <SelectValue placeholder="Select organization" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem
            v-for="org in orgList"
            :key="org.id"
            :value="org.id"
          >
            <div class="flex items-center gap-2">
              <Building2 class="h-3.5 w-3.5 text-muted-foreground" />
              <span>{{ org.name }}</span>
            </div>
          </SelectItem>
        </SelectContent>
      </Select>
      <div v-else-if="organizationsStore.loading" class="text-[12px] text-muted-foreground px-1">
        Loading...
      </div>
      <div v-else-if="organizationsStore.error" class="text-[12px] text-destructive px-1">
        {{ organizationsStore.error }}
      </div>
      <div v-else class="text-[12px] text-muted-foreground px-1">
        No organizations found
      </div>
    </div>

    <!-- Collapsed view - just show icon with selected org initial -->
    <div v-else class="flex justify-center">
      <Button
        variant="ghost"
        size="icon"
        class="h-8 w-8"
        :title="organizationsStore.selectedOrganization?.name || 'All Organizations'"
      >
        <Building2 class="h-4 w-4" />
      </Button>
    </div>
  </div>
</template>
