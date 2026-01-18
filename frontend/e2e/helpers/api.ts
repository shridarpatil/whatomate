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
}
