import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { organizationsService, type Organization } from '@/services/api'

const SELECTED_ORG_KEY = 'selected_organization_id'

export const useOrganizationsStore = defineStore('organizations', () => {
  const organizations = ref<Organization[]>([])
  const selectedOrgId = ref<string | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  const selectedOrganization = computed(() => {
    if (!selectedOrgId.value) return null
    return organizations.value.find(org => org.id === selectedOrgId.value) || null
  })

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
    selectedOrgId,
    selectedOrganization,
    loading,
    error,
    init,
    fetchOrganizations,
    selectOrganization,
    clearSelection,
    reset
  }
})
