import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '@/services/api'

export interface UserSettings {
  email_notifications?: boolean
  new_message_alerts?: boolean
  campaign_updates?: boolean
}

export interface Permission {
  id: string
  resource: string
  action: string
  description?: string
}

export interface UserRole {
  id: string
  name: string
  description?: string
  is_system: boolean
  permissions?: Permission[]
}

export interface User {
  id: string
  email: string
  full_name: string
  role_id?: string
  role?: UserRole
  organization_id: string
  organization_name?: string
  settings?: UserSettings
  is_available?: boolean
  is_super_admin?: boolean
}

export interface AuthState {
  user: User | null
  token: string | null
  refreshToken: string | null
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const token = ref<string | null>(null)
  const refreshToken = ref<string | null>(null)
  const breakStartedAt = ref<string | null>(null)

  const isAuthenticated = computed(() => !!token.value && !!user.value)
  const userRole = computed(() => user.value?.role?.name || 'agent')
  const organizationId = computed(() => user.value?.organization_id || '')
  const userSettings = computed(() => user.value?.settings || {})
  const isAvailable = computed(() => user.value?.is_available ?? true)

  function setAuth(authData: { user: User; access_token: string; refresh_token: string }) {
    user.value = authData.user
    token.value = authData.access_token
    refreshToken.value = authData.refresh_token

    // Store in localStorage
    localStorage.setItem('auth_token', authData.access_token)
    localStorage.setItem('refresh_token', authData.refresh_token)
    localStorage.setItem('user', JSON.stringify(authData.user))
  }

  function clearAuth() {
    user.value = null
    token.value = null
    refreshToken.value = null

    localStorage.removeItem('auth_token')
    localStorage.removeItem('refresh_token')
    localStorage.removeItem('user')
  }

  function restoreSession(): boolean {
    const storedToken = localStorage.getItem('auth_token')
    const storedRefreshToken = localStorage.getItem('refresh_token')
    const storedUser = localStorage.getItem('user')

    if (storedToken && storedUser) {
      try {
        token.value = storedToken
        refreshToken.value = storedRefreshToken
        user.value = JSON.parse(storedUser)
        // Fetch fresh user data in background to get updated permissions
        refreshUserData()
        return true
      } catch {
        clearAuth()
        return false
      }
    }
    return false
  }

  // Fetch fresh user data from API (including updated permissions)
  async function refreshUserData(): Promise<boolean> {
    if (!token.value) return false

    try {
      const response = await api.get('/me')
      const freshUser = response.data.data
      user.value = freshUser
      localStorage.setItem('user', JSON.stringify(freshUser))
      return true
    } catch {
      // If unauthorized, clear auth
      return false
    }
  }

  async function login(email: string, password: string): Promise<void> {
    const response = await api.post('/auth/login', { email, password })
    // fastglue wraps response in { status: "success", data: {...} }
    setAuth(response.data.data)
  }

  async function register(data: {
    email: string
    password: string
    full_name: string
    organization_id: string
  }): Promise<void> {
    const response = await api.post('/auth/register', data)
    // fastglue wraps response in { status: "success", data: {...} }
    setAuth(response.data.data)
  }

  async function switchOrg(organizationId: string): Promise<void> {
    const response = await api.post('/auth/switch-org', { organization_id: organizationId })
    setAuth(response.data.data)
    // Update localStorage org override
    localStorage.setItem('selected_organization_id', organizationId)
  }

  async function logout(): Promise<void> {
    try {
      await api.post('/auth/logout')
    } catch {
      // Ignore logout errors
    } finally {
      clearAuth()
    }
  }

  async function refreshAccessToken(): Promise<boolean> {
    if (!refreshToken.value) return false

    try {
      const response = await api.post('/auth/refresh', {
        refresh_token: refreshToken.value
      })
      // fastglue wraps response in { status: "success", data: {...} }
      const data = response.data.data
      token.value = data.access_token
      localStorage.setItem('auth_token', data.access_token)
      return true
    } catch {
      clearAuth()
      return false
    }
  }

  function setAvailability(available: boolean, breakStart?: string | null) {
    if (user.value) {
      user.value = { ...user.value, is_available: available }
      localStorage.setItem('user', JSON.stringify(user.value))
    }
    // Track break start time
    if (!available && breakStart) {
      breakStartedAt.value = breakStart
      localStorage.setItem('break_started_at', breakStart)
    } else if (available) {
      breakStartedAt.value = null
      localStorage.removeItem('break_started_at')
    }
  }

  function restoreBreakTime() {
    const stored = localStorage.getItem('break_started_at')
    if (stored && !isAvailable.value) {
      breakStartedAt.value = stored
    }
  }

  // Check if user has a specific permission
  function hasPermission(resource: string, action: string = 'read'): boolean {
    // Super admins have all permissions
    if (user.value?.is_super_admin) {
      return true
    }

    const permissions = user.value?.role?.permissions
    if (!permissions || permissions.length === 0) {
      return false
    }

    return permissions.some(p => p.resource === resource && p.action === action)
  }

  // Check if user has any permission for a resource
  function hasAnyPermission(resource: string): boolean {
    // Super admins have all permissions
    if (user.value?.is_super_admin) {
      return true
    }

    const permissions = user.value?.role?.permissions
    if (!permissions || permissions.length === 0) {
      return false
    }

    return permissions.some(p => p.resource === resource)
  }

  return {
    user,
    token,
    refreshToken,
    breakStartedAt,
    isAuthenticated,
    userRole,
    organizationId,
    userSettings,
    isAvailable,
    setAuth,
    clearAuth,
    restoreSession,
    restoreBreakTime,
    refreshUserData,
    login,
    register,
    switchOrg,
    logout,
    refreshAccessToken,
    setAvailability,
    hasPermission,
    hasAnyPermission
  }
})
