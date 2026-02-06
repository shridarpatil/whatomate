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

export class ApiHelper {
  private request: APIRequestContext
  private accessToken: string | null = null

  constructor(request: APIRequestContext) {
    this.request = request
  }

  private get headers() {
    return this.accessToken
      ? { Authorization: `Bearer ${this.accessToken}` }
      : {}
  }

  async login(email: string, password: string): Promise<string> {
    const response = await this.request.post(`${BASE_URL}/api/auth/login`, {
      data: { email, password }
    })
    if (!response.ok()) {
      throw new Error(`Login failed: ${await response.text()}`)
    }
    const data = await response.json()
    this.accessToken = data.data.access_token
    return this.accessToken
  }

  async loginAsAdmin(): Promise<string> {
    return this.login('admin@test.com', 'password')
  }

  // Register a user into an existing organization
  async register(data: {
    email: string
    password: string
    full_name: string
    organization_id: string
  }): Promise<{ user: User; access_token: string }> {
    const response = await this.request.post(`${BASE_URL}/api/auth/register`, {
      data
    })
    if (!response.ok()) {
      throw new Error(`Registration failed: ${await response.text()}`)
    }
    const result = await response.json()
    this.accessToken = result.data.access_token
    return {
      user: result.data.user,
      access_token: result.data.access_token
    }
  }

  // Create a new organization (requires organizations:write permission)
  async createOrganization(name: string): Promise<Organization> {
    const response = await this.request.post(`${BASE_URL}/api/organizations`, {
      headers: this.headers,
      data: { name }
    })
    if (!response.ok()) {
      throw new Error(`Failed to create organization: ${await response.text()}`)
    }
    const result = await response.json()
    return result.data
  }

  // Switch to a different organization (returns new tokens)
  async switchOrg(organizationId: string): Promise<string> {
    const response = await this.request.post(`${BASE_URL}/api/auth/switch-org`, {
      headers: this.headers,
      data: { organization_id: organizationId }
    })
    if (!response.ok()) {
      throw new Error(`Failed to switch org: ${await response.text()}`)
    }
    const result = await response.json()
    this.accessToken = result.data.access_token
    return this.accessToken
  }

  // List the current user's organization memberships
  async getMyOrganizations(): Promise<Array<{ organization_id: string; name: string; slug: string; role_name: string; is_default: boolean }>> {
    const response = await this.request.get(`${BASE_URL}/api/me/organizations`, {
      headers: this.headers
    })
    if (!response.ok()) {
      throw new Error(`Failed to get my organizations: ${await response.text()}`)
    }
    const data = await response.json()
    return data.data?.organizations || []
  }

  // List members of the current organization
  async getOrgMembers(orgId?: string): Promise<any[]> {
    const hdrs = { ...this.headers } as Record<string, string>
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
    const hdrs = { ...this.headers } as Record<string, string>
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
    const hdrs = { ...this.headers } as Record<string, string>
    if (orgId) hdrs['X-Organization-ID'] = orgId
    const response = await this.request.delete(`${BASE_URL}/api/organizations/members/${userId}`, {
      headers: hdrs
    })
    if (!response.ok()) {
      throw new Error(`Failed to remove org member: ${await response.text()}`)
    }
  }

  async getOrganizations(): Promise<Organization[]> {
    const response = await this.request.get(`${BASE_URL}/api/organizations`, {
      headers: this.headers
    })
    if (!response.ok()) {
      throw new Error(`Failed to get organizations: ${await response.text()}`)
    }
    const data = await response.json()
    return data.data?.organizations || []
  }

  async getUsersWithOrgHeader(orgId: string): Promise<User[]> {
    const response = await this.request.get(`${BASE_URL}/api/users`, {
      headers: {
        ...this.headers,
        'X-Organization-ID': orgId
      }
    })
    if (!response.ok()) {
      throw new Error(`Failed to get users: ${await response.text()}`)
    }
    const data = await response.json()
    return data.data?.users || []
  }

  getToken(): string | null {
    return this.accessToken
  }

  async getPermissions(): Promise<Permission[]> {
    const response = await this.request.get(`${BASE_URL}/api/permissions`, {
      headers: this.headers
    })
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
      headers: this.headers,
      data
    })
    const responseText = await response.text()
    if (!response.ok()) {
      throw new Error(`Failed to create role: ${responseText}`)
    }
    const result = JSON.parse(responseText)
    // Response is directly the role data, not nested under .role
    return result.data
  }

  async deleteRole(roleId: string): Promise<void> {
    await this.request.delete(`${BASE_URL}/api/roles/${roleId}`, {
      headers: this.headers
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
      headers: this.headers,
      data: { ...data, is_active: data.is_active ?? true }
    })
    const responseText = await response.text()
    if (!response.ok()) {
      throw new Error(`Failed to create user: ${responseText}`)
    }
    const result = JSON.parse(responseText)
    // Response is directly the user data, not nested under .user
    return result.data
  }

  async deleteUser(userId: string): Promise<void> {
    await this.request.delete(`${BASE_URL}/api/users/${userId}`, {
      headers: this.headers
    })
  }

  async updateUserRole(userId: string, roleId: string): Promise<User> {
    const response = await this.request.put(`${BASE_URL}/api/users/${userId}`, {
      headers: this.headers,
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
      headers: this.headers,
      data: { phone_number: phoneNumber, profile_name: profileName || '' }
    })
    if (!response.ok()) {
      throw new Error(`Failed to create contact: ${await response.text()}`)
    }
    const data = await response.json()
    return data.data
  }

  async getContacts(): Promise<any[]> {
    const response = await this.request.get(`${BASE_URL}/api/contacts`, {
      headers: this.headers
    })
    if (!response.ok()) {
      throw new Error(`Failed to get contacts: ${await response.text()}`)
    }
    const data = await response.json()
    return data.data?.contacts || []
  }

  // Conversation Notes
  async listNotes(contactId: string): Promise<any[]> {
    const response = await this.request.get(`${BASE_URL}/api/contacts/${contactId}/notes`, {
      headers: this.headers
    })
    if (!response.ok()) {
      throw new Error(`Failed to list notes: ${await response.text()}`)
    }
    const data = await response.json()
    return data.data?.notes || []
  }

  async createNote(contactId: string, content: string): Promise<any> {
    const response = await this.request.post(`${BASE_URL}/api/contacts/${contactId}/notes`, {
      headers: this.headers,
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
      headers: this.headers,
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
      headers: this.headers
    })
    if (!response.ok()) {
      throw new Error(`Failed to delete note: ${await response.text()}`)
    }
  }
}
