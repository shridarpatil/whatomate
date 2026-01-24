<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
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

// Only show for super admins
const isSuperAdmin = () => authStore.user?.is_super_admin || false

onMounted(async () => {
  if (isSuperAdmin()) {
    organizationsStore.init()
    await organizationsStore.fetchOrganizations()

    // If no org selected, default to user's own org
    if (!organizationsStore.selectedOrgId && authStore.user?.organization_id) {
      organizationsStore.selectOrganization(authStore.user.organization_id)
    }
  }
})

// Watch for auth changes
watch(() => authStore.user?.is_super_admin, async (isSuperAdmin) => {
  if (isSuperAdmin) {
    organizationsStore.init()
    await organizationsStore.fetchOrganizations()
  } else {
    organizationsStore.reset()
  }
})

const handleOrgChange = (value: string | number | bigint | Record<string, any> | null) => {
  if (!value || typeof value !== 'string') return
  organizationsStore.selectOrganization(value)
  // Reload the page to refresh data with new org context
  window.location.reload()
}

const refreshOrgs = async () => {
  isRefreshing.value = true
  await organizationsStore.fetchOrganizations()
  isRefreshing.value = false
}
</script>

<template>
  <div v-if="isSuperAdmin()" class="px-2 py-2 border-b">
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
        v-if="organizationsStore.organizations.length > 0"
        :model-value="organizationsStore.selectedOrgId || ''"
        @update:model-value="handleOrgChange"
      >
        <SelectTrigger class="h-8 text-[13px]">
          <SelectValue placeholder="Select organization" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem
            v-for="org in organizationsStore.organizations"
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
