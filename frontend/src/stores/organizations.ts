import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { organizationsService, usersService, type Organization } from '@/services/api'

const SELECTED_ORG_KEY = 'selected_organization_id'

export const useOrganizationsStore = defineStore('organizations', () => {
  const organizations = ref<Organization[]>([])
  const myOrganizations = ref<Array<{
    organization_id: string
    name: string
    slug: string
    role_name: string
    is_default: boolean
  }>>([])
  const selectedOrgId = ref<string | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  const selectedOrganization = computed(() => {
    if (!selectedOrgId.value) return null
    return organizations.value.find(org => org.id === selectedOrgId.value) || null
  })

  const isMultiOrg = computed(() => myOrganizations.value.length > 1)

  // Initialize from localStorage
  function init() {
    const stored = localStorage.getItem(SELECTED_ORG_KEY)
    if (stored) {
      selectedOrgId.value = stored
    }
  }

  async function fetchOrganizations(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const response = await organizationsService.list()
      organizations.value = (response.data as any).data?.organizations || response.data?.organizations || []
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to fetch organizations'
      organizations.value = []
    } finally {
      loading.value = false
    }
  }

  async function fetchMyOrganizations(): Promise<void> {
    try {
      const response = await usersService.listMyOrganizations()
      myOrganizations.value = (response.data as any).data?.organizations || []
    } catch {
      myOrganizations.value = []
    }
  }

  function selectOrganization(orgId: string | null) {
    selectedOrgId.value = orgId
    if (orgId) {
      localStorage.setItem(SELECTED_ORG_KEY, orgId)
    } else {
      localStorage.removeItem(SELECTED_ORG_KEY)
    }
  }

  function clearSelection() {
    selectedOrgId.value = null
    localStorage.removeItem(SELECTED_ORG_KEY)
  }

  function reset() {
    organizations.value = []
    selectedOrgId.value = null
    localStorage.removeItem(SELECTED_ORG_KEY)
  }

  return {
    organizations,
    myOrganizations,
    isMultiOrg,
    selectedOrgId,
    selectedOrganization,
    loading,
    error,
    init,
    fetchOrganizations,
    fetchMyOrganizations,
    selectOrganization,
    clearSelection,
    reset
  }
})
