import { APIRequestContext } from '@playwright/test'

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080'

export interface Permission {
  id: string
  resource: string
  action: string
}

export interface Role {
  id: string
  name: string
  description: string
}

export interface User {
  id: string
  email: string
  full_name: string
  role_id?: string
  organization_id?: string
}

export interface Organization {
  id: string
  name: string
  slug?: string
}

/**
 * Extract the whm_csrf cookie value from a response's Set-Cookie headers.
 * Playwright's APIRequestContext auto-persists cookies for subsequent requests,
 * but we need the CSRF token value to send as X-CSRF-Token header.
 */
function extractCSRFToken(response: { headers: () => Record<string, string>; headersArray: () => Array<{ name: string; value: string }> }): string | null {
  const cookieHeaders = response.headersArray().filter(h => h.name.toLowerCase() === 'set-cookie')
  for (const header of cookieHeaders) {
    const match = header.value.match(/whm_csrf=([^;]+)/)
    if (match) return match[1]
  }
  return null
}

export class ApiHelper {
  private request: APIRequestContext
  private csrfToken: string | null = null

  constructor(request: APIRequestContext) {
    this.request = request
  }

  /** Headers for mutating requests (POST/PUT/DELETE/PATCH) — includes CSRF token */
  private get csrfHeaders(): Record<string, string> {
    return this.csrfToken ? { 'X-CSRF-Token': this.csrfToken } : {}
  }

  async login(email: string, password: string): Promise<void> {
    const response = await this.request.post(`${BASE_URL}/api/auth/login`, {
      data: { email, password }
    })
    if (!response.ok()) {
      throw new Error(`Login failed: ${await response.text()}`)
    }
    // Cookies (whm_access, whm_refresh) are auto-persisted by Playwright.
    // Extract CSRF token for mutating requests.
    this.csrfToken = extractCSRFToken(response)
  }

  async loginAsAdmin(): Promise<void> {
    await this.login('admin@test.com', 'password')
  }

  // Register a user into an existing organization
  async register(data: {
    email: string
    password: string
    full_name: string
    organization_id: string
  }): Promise<{ user: User }> {
    const response = await this.request.post(`${BASE_URL}/api/auth/register`, {
      data
    })
    if (!response.ok()) {
      throw new Error(`Registration failed: ${await response.text()}`)
    }
    const result = await response.json()
    this.csrfToken = extractCSRFToken(response)
    return { user: result.data.user }
  }

  // Create a new organization (requires organizations:write permission)
  async createOrganization(name: string): Promise<Organization> {
    const response = await this.request.post(`${BASE_URL}/api/organizations`, {
      headers: this.csrfHeaders,
      data: { name }
    })
    if (!response.ok()) {
      throw new Error(`Failed to create organization: ${await response.text()}`)
    }
    const result = await response.json()
    return result.data
  }

  // Switch to a different organization
  async switchOrg(organizationId: string): Promise<void> {
    const response = await this.request.post(`${BASE_URL}/api/auth/switch-org`, {
      headers: this.csrfHeaders,
      data: { organization_id: organizationId }
    })
    if (!response.ok()) {
      throw new Error(`Failed to switch org: ${await response.text()}`)
    }
    // New cookies are set by the server, auto-persisted by Playwright
    this.csrfToken = extractCSRFToken(response)
  }

  // List the current user's organization memberships
  async getMyOrganizations(): Promise<Array<{ organization_id: string; name: string; slug: string; role_name: string; is_default: boolean }>> {
    const response = await this.request.get(`${BASE_URL}/api/me/organizations`)
    if (!response.ok()) {
      throw new Error(`Failed to get my organizations: ${await response.text()}`)
    }
    const data = await response.json()
    return data.data?.organizations || []
  }

  // List members of the current organization
  async getOrgMembers(orgId?: string): Promise<any[]> {
    const hdrs: Record<string, string> = {}
    if (orgId) hdrs['X-Organization-ID'] = orgId
    const response = await this.request.get(`${BASE_URL}/api/organizations/members`, {
      headers: hdrs
    })
    if (!response.ok()) {
      throw new Error(`Failed to get org members: ${await response.text()}`)
    }
    const data = await response.json()
    return data.data?.members || []
  }

  // Add a member to the current organization
  async addOrgMember(userId: string, roleId?: string, orgId?: string): Promise<void> {
    const hdrs: Record<string, string> = { ...this.csrfHeaders }
    if (orgId) hdrs['X-Organization-ID'] = orgId
    const body: Record<string, string> = { user_id: userId }
    if (roleId) body.role_id = roleId
    const response = await this.request.post(`${BASE_URL}/api/organizations/members`, {
      headers: hdrs,
      data: body
    })
    if (!response.ok()) {
      throw new Error(`Failed to add org member: ${await response.text()}`)
    }
  }

  // Remove a member from the current organization
  async removeOrgMember(userId: string, orgId?: string): Promise<void> {
    const hdrs: Record<string, string> = { ...this.csrfHeaders }
    if (orgId) hdrs['X-Organization-ID'] = orgId
    const response = await this.request.delete(`${BASE_URL}/api/organizations/members/${userId}`, {
      headers: hdrs
    })
    if (!response.ok()) {
      throw new Error(`Failed to remove org member: ${await response.text()}`)
    }
  }

  async getOrganizations(): Promise<Organization[]> {
    const response = await this.request.get(`${BASE_URL}/api/organizations`)
    if (!response.ok()) {
      throw new Error(`Failed to get organizations: ${await response.text()}`)
    }
    const data = await response.json()
    return data.data?.organizations || []
  }

  async getUsersWithOrgHeader(orgId: string): Promise<User[]> {
    const response = await this.request.get(`${BASE_URL}/api/users`, {
      headers: { 'X-Organization-ID': orgId }
    })
    if (!response.ok()) {
      throw new Error(`Failed to get users: ${await response.text()}`)
    }
    const data = await response.json()
    return data.data?.users || []
  }

  async getPermissions(): Promise<Permission[]> {
    const response = await this.request.get(`${BASE_URL}/api/permissions`)
    if (!response.ok()) {
      throw new Error(`Failed to get permissions: ${await response.text()}`)
    }
    const data = await response.json()
    return data.data?.permissions || []
  }

  // Returns permission keys like "users:read", "contacts:write"
  async findPermissionKeys(filters: { resource: string; action: string }[]): Promise<string[]> {
    return filters.map(f => `${f.resource}:${f.action}`)
  }

  async createRole(data: { name: string; description: string; permissions: string[] }): Promise<Role> {
    const response = await this.request.post(`${BASE_URL}/api/roles`, {
      headers: this.csrfHeaders,
      data
    })
    const responseText = await response.text()
    if (!response.ok()) {
      throw new Error(`Failed to create role: ${responseText}`)
    }
    const result = JSON.parse(responseText)
    return result.data
  }

  async deleteRole(roleId: string): Promise<void> {
    await this.request.delete(`${BASE_URL}/api/roles/${roleId}`, {
      headers: this.csrfHeaders
    })
  }

  async createUser(data: {
    email: string
    password: string
    full_name: string
    role_id: string
    is_active?: boolean
  }): Promise<User> {
    const response = await this.request.post(`${BASE_URL}/api/users`, {
      headers: this.csrfHeaders,
      data: { ...data, is_active: data.is_active ?? true }
    })
    const responseText = await response.text()
    if (!response.ok()) {
      throw new Error(`Failed to create user: ${responseText}`)
    }
    const result = JSON.parse(responseText)
    return result.data
  }

  async deleteUser(userId: string): Promise<void> {
    await this.request.delete(`${BASE_URL}/api/users/${userId}`, {
      headers: this.csrfHeaders
    })
  }

  async updateUserRole(userId: string, roleId: string): Promise<User> {
    const response = await this.request.put(`${BASE_URL}/api/users/${userId}`, {
      headers: this.csrfHeaders,
      data: { role_id: roleId }
    })
    if (!response.ok()) {
      throw new Error(`Failed to update user role: ${await response.text()}`)
    }
    const result = await response.json()
    return result.data.user
  }

  // Contacts
  async createContact(phoneNumber: string, profileName?: string): Promise<any> {
    const response = await this.request.post(`${BASE_URL}/api/contacts`, {
      headers: this.csrfHeaders,
      data: { phone_number: phoneNumber, profile_name: profileName || '' }
    })
    if (!response.ok()) {
      throw new Error(`Failed to create contact: ${await response.text()}`)
    }
    const data = await response.json()
    return data.data
  }

  async getContacts(): Promise<any[]> {
    const response = await this.request.get(`${BASE_URL}/api/contacts`)
    if (!response.ok()) {
      throw new Error(`Failed to get contacts: ${await response.text()}`)
    }
    const data = await response.json()
    return data.data?.contacts || []
  }

  async updateContact(contactId: string, data: Record<string, any>): Promise<any> {
    const response = await this.request.put(`${BASE_URL}/api/contacts/${contactId}`, {
      headers: this.csrfHeaders,
      data
    })
    if (!response.ok()) {
      throw new Error(`Failed to update contact: ${await response.text()}`)
    }
    const result = await response.json()
    return result.data
  }

  // Conversation Notes
  async listNotes(contactId: string): Promise<any[]> {
    const response = await this.request.get(`${BASE_URL}/api/contacts/${contactId}/notes`)
    if (!response.ok()) {
      throw new Error(`Failed to list notes: ${await response.text()}`)
    }
    const data = await response.json()
    return data.data?.notes || []
  }

  async createNote(contactId: string, content: string): Promise<any> {
    const response = await this.request.post(`${BASE_URL}/api/contacts/${contactId}/notes`, {
      headers: this.csrfHeaders,
      data: { content }
    })
    if (!response.ok()) {
      throw new Error(`Failed to create note: ${await response.text()}`)
    }
    const data = await response.json()
    return data.data
  }

  async updateNote(contactId: string, noteId: string, content: string): Promise<any> {
    const response = await this.request.put(`${BASE_URL}/api/contacts/${contactId}/notes/${noteId}`, {
      headers: this.csrfHeaders,
      data: { content }
    })
    if (!response.ok()) {
      throw new Error(`Failed to update note: ${await response.text()}`)
    }
    const data = await response.json()
    return data.data
  }

  async deleteNote(contactId: string, noteId: string): Promise<void> {
    const response = await this.request.delete(`${BASE_URL}/api/contacts/${contactId}/notes/${noteId}`, {
      headers: this.csrfHeaders
    })
    if (!response.ok()) {
      throw new Error(`Failed to delete note: ${await response.text()}`)
    }
  }

  // Generic authenticated requests — use these instead of raw request calls
  async get(path: string, extraHeaders?: Record<string, string>) {
    return this.request.get(`${BASE_URL}${path}`, {
      headers: extraHeaders
    })
  }

  async post(path: string, data?: any, extraHeaders?: Record<string, string>) {
    return this.request.post(`${BASE_URL}${path}`, {
      headers: { ...this.csrfHeaders, ...extraHeaders },
      data
    })
  }

  async put(path: string, data?: any, extraHeaders?: Record<string, string>) {
    return this.request.put(`${BASE_URL}${path}`, {
      headers: { ...this.csrfHeaders, ...extraHeaders },
      data
    })
  }

  async del(path: string, extraHeaders?: Record<string, string>) {
    return this.request.delete(`${BASE_URL}${path}`, {
      headers: { ...this.csrfHeaders, ...extraHeaders }
    })
  }

  async getUsers(): Promise<User[]> {
    const response = await this.get('/api/users')
    if (!response.ok()) {
      throw new Error(`Failed to get users: ${await response.text()}`)
    }
    const data = await response.json()
    return data.data?.users || []
  }

  async getCurrentOrg(): Promise<Organization> {
    const response = await this.get('/api/organizations/current')
    if (!response.ok()) {
      throw new Error(`Failed to get current org: ${await response.text()}`)
    }
    const data = await response.json()
    return data.data
  }
}
