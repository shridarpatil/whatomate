<script setup lang="ts">
import { ref, computed, nextTick, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useOrganizationsStore } from '@/stores/organizations'
import { useAuthStore } from '@/stores/auth'
import Modal from '@/components/oat/Modal.vue'
import { organizationsService } from '@/services/api'
import { toast } from '@/lib/toast'
import { filterOptions } from '@/components/oat/combobox'
import { Plus, Check, ChevronsUpDown } from '@/lib/icons'

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

const SEARCH_THRESHOLD = 8
const menuId = 'org-switcher-menu'
const menu = ref<HTMLElement | null>(null)
const searchInput = ref<HTMLInputElement | null>(null)
const query = ref('')

const showSearch = computed(() => orgList.value.length > SEARCH_THRESHOLD)
const currentOrgName = computed(() => orgList.value.find(o => o.id === currentOrgId.value)?.name || '')
const triggerLabel = computed(() => {
  if (currentOrgName.value) return currentOrgName.value
  if (organizationsStore.loading) return t('organizations.loading')
  return t('organizations.selectPlaceholder')
})
const filteredOrgs = computed(() =>
  filterOptions(orgList.value.map(o => ({ value: o.id, label: o.name })), query.value),
)

function onToggle(e: Event) {
  if ((e as ToggleEvent).newState === 'closed') query.value = ''
}

// Oat's dropdown snapshots its menu items when it opens, so after the search filter
// changes the list its arrow-key handling is stale. Handle arrows here instead.
function menuItems(): HTMLElement[] {
  return Array.from(menu.value?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])
}

function onListKeydown(e: KeyboardEvent) {
  const items = menuItems()
  const idx = items.indexOf(e.target as HTMLElement)
  let next = -1
  if (e.key === 'ArrowDown') next = idx < 0 ? 0 : (idx + 1) % items.length
  else if (e.key === 'ArrowUp') {
    if (idx === 0 && showSearch.value) {
      e.preventDefault()
      e.stopPropagation()
      searchInput.value?.focus()
      return
    }
    next = idx <= 0 ? items.length - 1 : idx - 1
  } else if (e.key === 'Home') next = 0
  else if (e.key === 'End') next = items.length - 1
  if (next < 0 || !items.length) return
  e.preventDefault()
  e.stopPropagation()
  items[next].focus()
}

async function onSearchKeydown(e: KeyboardEvent) {
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    await nextTick()
    menuItems()[0]?.focus()
  }
}

async function selectOrg(value: string) {
  menu.value?.hidePopover()
  if (!value || value === currentOrgId.value) return

  // Everyone goes through switch-org, super admins included: the X-Organization-ID
  // header alone only scopes API client calls. The WebSocket token, <img src> media and
  // window.open previews carry no headers, so the JWT itself has to name the target
  // org or those keep talking to the previous one.
  try {
    await authStore.switchOrg(value)
    organizationsStore.selectOrganization(value)
    window.location.reload()
  } catch {
    // If switch fails, don't reload; the current org stays selected.
  }
}

function openCreateDialog() {
  menu.value?.hidePopover()
  isCreateDialogOpen.value = true
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
  <div v-if="shouldShowSwitcher" class="vstack gap-2 p-4" data-testid="org-switcher">
    <ot-dropdown>
      <button
        type="button"
        class="outline small w-100 justify-between"
        :popovertarget="menuId"
        aria-haspopup="menu"
        :aria-label="t('organizations.label') + ': ' + triggerLabel"
        :title="triggerLabel"
      >
        <span class="truncate">{{ triggerLabel }}</span>
        <ChevronsUpDown :size="14" aria-hidden="true" />
      </button>
      <div :id="menuId" ref="menu" popover class="picker" @toggle="onToggle">
        <input
          v-if="showSearch"
          ref="searchInput"
          v-model="query"
          type="search"
          :placeholder="t('common.search')"
          :aria-label="t('common.search')"
          @keydown="onSearchKeydown"
        >
        <div class="scroll-y picker-list" role="group" :aria-label="t('organizations.selectPlaceholder')" @keydown="onListKeydown">
          <button
            v-for="org in filteredOrgs"
            :key="org.value"
            type="button"
            role="menuitem"
            class="ghost"
            :aria-current="org.value === currentOrgId ? 'true' : undefined"
            :title="org.label"
            @click="selectOrg(org.value)"
          >
            <Check v-if="org.value === currentOrgId" :size="14" aria-hidden="true" />
            <span class="truncate">{{ org.label }}</span>
          </button>
          <p v-if="!filteredOrgs.length" role="status" class="text-light p-4">
            {{ organizationsStore.loading ? t('organizations.loading') : orgList.length ? t('common.noResults') : t('organizations.none') }}
          </p>
        </div>
        <template v-if="canCreateOrg">
          <hr>
          <button type="button" role="menuitem" class="ghost" @click="openCreateDialog">
            <Plus :size="14" aria-hidden="true" />
            <span>{{ t('organizations.createNew') }}</span>
          </button>
        </template>
      </div>
    </ot-dropdown>
    <div v-if="organizationsStore.error && !orgList.length" role="alert" data-variant="danger">{{ organizationsStore.error }}</div>
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
