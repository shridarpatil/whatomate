import axios, { type AxiosInstance, type AxiosError, type InternalAxiosRequestConfig } from 'axios'

// Get base path from server-injected config or fallback
const basePath = ((window as any).__BASE_PATH__ ?? '').replace(/\/$/, '')
const API_BASE_URL = import.meta.env.VITE_API_URL || `${basePath}/api`

export const api: AxiosInstance = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// Request interceptor to add auth token
api.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const token = localStorage.getItem('auth_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error: AxiosError) => {
    return Promise.reject(error)
  }
)

// Response interceptor for error handling
api.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const originalRequest = error.config as InternalAxiosRequestConfig & { _retry?: boolean }

    // Skip token refresh logic for auth endpoints
    const isAuthEndpoint = originalRequest?.url?.startsWith('/auth/')

    // Handle 401 errors - try to refresh token (but not for auth endpoints)
    if (error.response?.status === 401 && !originalRequest._retry && !isAuthEndpoint) {
      originalRequest._retry = true

      const refreshToken = localStorage.getItem('refresh_token')
      if (refreshToken) {
        try {
          const response = await axios.post(`${API_BASE_URL}/auth/refresh`, {
            refresh_token: refreshToken
          })

          // fastglue wraps response in { status: "success", data: {...} }
          const newToken = response.data.data.access_token
          localStorage.setItem('auth_token', newToken)

          originalRequest.headers.Authorization = `Bearer ${newToken}`
          return api(originalRequest)
        } catch {
          // Refresh failed, clear auth and redirect to login
          localStorage.removeItem('auth_token')
          localStorage.removeItem('refresh_token')
          localStorage.removeItem('user')
          window.location.href = '/login'
        }
      } else {
        window.location.href = '/login'
      }
    }

    return Promise.reject(error)
  }
)

// API service methods
export const authService = {
  login: (email: string, password: string) =>
    api.post('/auth/login', { email, password }),

  register: (data: { email: string; password: string; full_name: string; organization_name: string }) =>
    api.post('/auth/register', data),

  logout: () => api.post('/auth/logout'),

  refreshToken: (refreshToken: string) =>
    api.post('/auth/refresh', { refresh_token: refreshToken }),

  me: () => api.get('/auth/me')
}

export const usersService = {
  list: () => api.get('/users'),
  get: (id: string) => api.get(`/users/${id}`),
  create: (data: { email: string; password: string; full_name: string; role?: string }) =>
    api.post('/users', data),
  update: (id: string, data: { email?: string; password?: string; full_name?: string; role?: string; is_active?: boolean }) =>
    api.put(`/users/${id}`, data),
  delete: (id: string) => api.delete(`/users/${id}`),
  me: () => api.get('/me'),
  updateSettings: (data: { email_notifications: boolean; new_message_alerts: boolean; campaign_updates: boolean }) =>
    api.put('/me/settings', data),
  changePassword: (data: { current_password: string; new_password: string }) =>
    api.put('/me/password', data),
  updateAvailability: (isAvailable: boolean) =>
    api.put('/me/availability', { is_available: isAvailable })
}

export const apiKeysService = {
  list: () => api.get('/api-keys'),
  create: (data: { name: string; expires_at?: string }) =>
    api.post('/api-keys', data),
  delete: (id: string) => api.delete(`/api-keys/${id}`)
}

export const accountsService = {
  list: () => api.get('/accounts'),
  get: (id: string) => api.get(`/accounts/${id}`),
  create: (data: any) => api.post('/accounts', data),
  update: (id: string, data: any) => api.put(`/accounts/${id}`, data),
  delete: (id: string) => api.delete(`/accounts/${id}`)
}

export const contactsService = {
  list: (params?: { search?: string; page?: number; limit?: number }) =>
    api.get('/contacts', { params }),
  get: (id: string) => api.get(`/contacts/${id}`),
  create: (data: any) => api.post('/contacts', data),
  update: (id: string, data: any) => api.put(`/contacts/${id}`, data),
  delete: (id: string) => api.delete(`/contacts/${id}`),
  assign: (id: string, userId: string | null) =>
    api.put(`/contacts/${id}/assign`, { user_id: userId }),
  import: (file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    return api.post('/contacts/import', formData, {
      headers: { 'Content-Type': 'multipart/form-data' }
    })
  }
}

export const messagesService = {
  list: (contactId: string, params?: { page?: number; limit?: number; before_id?: string }) =>
    api.get(`/contacts/${contactId}/messages`, { params }),
  send: (contactId: string, data: { type: string; content: any; reply_to_message_id?: string }) =>
    api.post(`/contacts/${contactId}/messages`, data),
  sendTemplate: (contactId: string, data: { template_name: string; components?: any[] }) =>
    api.post(`/contacts/${contactId}/messages/template`, data),
  sendReaction: (contactId: string, messageId: string, emoji: string) =>
    api.post(`/contacts/${contactId}/messages/${messageId}/reaction`, { emoji })
}

export const templatesService = {
  list: (params?: { status?: string; category?: string }) =>
    api.get('/templates', { params }),
  get: (id: string) => api.get(`/templates/${id}`),
  create: (data: any) => api.post('/templates', data),
  update: (id: string, data: any) => api.put(`/templates/${id}`, data),
  delete: (id: string) => api.delete(`/templates/${id}`),
  sync: () => api.post('/templates/sync'),
  uploadMedia: (accountName: string, file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    formData.append('account', accountName)
    return axios.post(`${api.defaults.baseURL}/templates/upload-media`, formData, {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('auth_token')}`
      }
    })
  }
}

export const flowsService = {
  list: () => api.get('/flows'),
  get: (id: string) => api.get(`/flows/${id}`),
  create: (data: any) => api.post('/flows', data),
  update: (id: string, data: any) => api.put(`/flows/${id}`, data),
  delete: (id: string) => api.delete(`/flows/${id}`),
  saveToMeta: (id: string) => api.post(`/flows/${id}/save-to-meta`),
  publish: (id: string) => api.post(`/flows/${id}/publish`),
  deprecate: (id: string) => api.post(`/flows/${id}/deprecate`),
  sync: (whatsappAccount: string) => api.post('/flows/sync', { whatsapp_account: whatsappAccount })
}

export const campaignsService = {
  list: (params?: { status?: string; from?: string; to?: string }) => api.get('/campaigns', { params }),
  get: (id: string) => api.get(`/campaigns/${id}`),
  create: (data: any) => api.post('/campaigns', data),
  update: (id: string, data: any) => api.put(`/campaigns/${id}`, data),
  delete: (id: string) => api.delete(`/campaigns/${id}`),
  start: (id: string) => api.post(`/campaigns/${id}/start`),
  pause: (id: string) => api.post(`/campaigns/${id}/pause`),
  cancel: (id: string) => api.post(`/campaigns/${id}/cancel`),
  retryFailed: (id: string) => api.post(`/campaigns/${id}/retry-failed`),
  stats: (id: string) => api.get(`/campaigns/${id}/stats`),
  // Recipients
  getRecipients: (id: string) => api.get(`/campaigns/${id}/recipients`),
  addRecipients: (id: string, recipients: Array<{ phone_number: string; recipient_name?: string; template_params?: Record<string, any> }>) =>
    api.post(`/campaigns/${id}/recipients/import`, { recipients }),
  deleteRecipient: (campaignId: string, recipientId: string) =>
    api.delete(`/campaigns/${campaignId}/recipients/${recipientId}`),
  // Media
  uploadMedia: (campaignId: string, file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    return axios.post(`${api.defaults.baseURL}/campaigns/${campaignId}/media`, formData, {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('auth_token')}`
      }
    })
  },
  getMedia: (campaignId: string) =>
    api.get(`/campaigns/${campaignId}/media`, { responseType: 'arraybuffer' })
}

export const chatbotService = {
  // Settings
  getSettings: () => api.get('/chatbot/settings'),
  updateSettings: (data: any) => api.put('/chatbot/settings', data),

  // Keywords
  listKeywords: () => api.get('/chatbot/keywords'),
  getKeyword: (id: string) => api.get(`/chatbot/keywords/${id}`),
  createKeyword: (data: any) => api.post('/chatbot/keywords', data),
  updateKeyword: (id: string, data: any) => api.put(`/chatbot/keywords/${id}`, data),
  deleteKeyword: (id: string) => api.delete(`/chatbot/keywords/${id}`),

  // Flows
  listFlows: () => api.get('/chatbot/flows'),
  getFlow: (id: string) => api.get(`/chatbot/flows/${id}`),
  createFlow: (data: any) => api.post('/chatbot/flows', data),
  updateFlow: (id: string, data: any) => api.put(`/chatbot/flows/${id}`, data),
  deleteFlow: (id: string) => api.delete(`/chatbot/flows/${id}`),

  // AI Contexts
  listAIContexts: () => api.get('/chatbot/ai-contexts'),
  getAIContext: (id: string) => api.get(`/chatbot/ai-contexts/${id}`),
  createAIContext: (data: any) => api.post('/chatbot/ai-contexts', data),
  updateAIContext: (id: string, data: any) => api.put(`/chatbot/ai-contexts/${id}`, data),
  deleteAIContext: (id: string) => api.delete(`/chatbot/ai-contexts/${id}`),

  // Sessions
  listSessions: (params?: { status?: string; contact_id?: string }) =>
    api.get('/chatbot/sessions', { params }),
  getSession: (id: string) => api.get(`/chatbot/sessions/${id}`),

  // Agent Transfers
  listTransfers: (params?: {
    status?: string
    agent_id?: string
    team_id?: string
    limit?: number
    offset?: number
    include?: string // 'all' | 'contact,agent,team' etc.
  }) => api.get('/chatbot/transfers', { params }),
  createTransfer: (data: {
    contact_id: string
    whatsapp_account: string
    agent_id?: string
    notes?: string
    source?: string
  }) => api.post('/chatbot/transfers', data),
  pickNextTransfer: () => api.post('/chatbot/transfers/pick'),
  resumeTransfer: (id: string) => api.put(`/chatbot/transfers/${id}/resume`),
  assignTransfer: (id: string, agentId: string | null, teamId?: string | null) =>
    api.put(`/chatbot/transfers/${id}/assign`, { agent_id: agentId, team_id: teamId })
}

export interface CannedResponse {
  id: string
  name: string
  shortcut: string
  content: string
  category: string
  is_active: boolean
  usage_count: number
  created_at: string
  updated_at: string
}

export const cannedResponsesService = {
  list: (params?: { category?: string; search?: string; active_only?: string }) =>
    api.get('/canned-responses', { params }),
  get: (id: string) => api.get(`/canned-responses/${id}`),
  create: (data: { name: string; shortcut?: string; content: string; category?: string }) =>
    api.post('/canned-responses', data),
  update: (id: string, data: { name?: string; shortcut?: string; content?: string; category?: string; is_active?: boolean }) =>
    api.put(`/canned-responses/${id}`, data),
  delete: (id: string) => api.delete(`/canned-responses/${id}`),
  use: (id: string) => api.post(`/canned-responses/${id}/use`)
}

export const analyticsService = {
  dashboard: (params?: { from?: string; to?: string }) =>
    api.get('/analytics/dashboard', { params }),
  messages: (params?: { from?: string; to?: string; group_by?: string }) =>
    api.get('/analytics/messages', { params }),
  campaigns: (params?: { from?: string; to?: string }) =>
    api.get('/analytics/campaigns', { params }),
  chatbot: (params?: { from?: string; to?: string }) =>
    api.get('/analytics/chatbot', { params })
}

export const agentAnalyticsService = {
  getSummary: (params?: { from?: string; to?: string; agent_id?: string }) =>
    api.get('/analytics/agents', { params }),
  getAgentDetails: (id: string, params?: { from?: string; to?: string }) =>
    api.get(`/analytics/agents/${id}`, { params }),
  getComparison: (params?: { from?: string; to?: string }) =>
    api.get('/analytics/agents/comparison', { params })
}

export const organizationService = {
  getSettings: () => api.get('/org/settings'),
  updateSettings: (data: {
    mask_phone_numbers?: boolean
    timezone?: string
    date_format?: string
    name?: string
  }) => api.put('/org/settings', data)
}

export interface Webhook {
  id: string
  name: string
  url: string
  events: string[]
  headers: Record<string, string>
  is_active: boolean
  has_secret: boolean
  created_at: string
  updated_at: string
}

export interface WebhookEvent {
  value: string
  label: string
  description: string
}

export interface Team {
  id: string
  name: string
  description: string
  assignment_strategy: 'round_robin' | 'load_balanced' | 'manual'
  is_active: boolean
  member_count: number
  created_at: string
  updated_at: string
}

export interface TeamMember {
  id: string
  team_id: string
  user_id: string
  role: 'manager' | 'agent'
  last_assigned_at: string | null
  user: {
    id: string
    full_name: string
    email: string
    is_available: boolean
  }
}

export const teamsService = {
  list: () => api.get<{ teams: Team[] }>('/teams'),
  get: (id: string) => api.get<{ team: Team }>(`/teams/${id}`),
  create: (data: {
    name: string
    description?: string
    assignment_strategy?: 'round_robin' | 'load_balanced' | 'manual'
  }) => api.post<{ team: Team }>('/teams', data),
  update: (id: string, data: {
    name?: string
    description?: string
    assignment_strategy?: 'round_robin' | 'load_balanced' | 'manual'
    is_active?: boolean
  }) => api.put<{ team: Team }>(`/teams/${id}`, data),
  delete: (id: string) => api.delete(`/teams/${id}`),
  // Members
  listMembers: (teamId: string) => api.get<{ members: TeamMember[] }>(`/teams/${teamId}/members`),
  addMember: (teamId: string, data: { user_id: string; role?: 'manager' | 'agent' }) =>
    api.post<{ member: TeamMember }>(`/teams/${teamId}/members`, data),
  removeMember: (teamId: string, userId: string) =>
    api.delete(`/teams/${teamId}/members/${userId}`)
}

export const webhooksService = {
  list: () => api.get<{ webhooks: Webhook[]; available_events: WebhookEvent[] }>('/webhooks'),
  get: (id: string) => api.get<Webhook>(`/webhooks/${id}`),
  create: (data: {
    name: string
    url: string
    events: string[]
    headers?: Record<string, string>
    secret?: string
  }) => api.post<Webhook>('/webhooks', data),
  update: (id: string, data: {
    name?: string
    url?: string
    events?: string[]
    headers?: Record<string, string>
    secret?: string
    is_active?: boolean
  }) => api.put<Webhook>(`/webhooks/${id}`, data),
  delete: (id: string) => api.delete(`/webhooks/${id}`),
  test: (id: string) => api.post(`/webhooks/${id}/test`)
}

export interface CustomAction {
  id: string
  name: string
  icon: string
  action_type: 'webhook' | 'url' | 'javascript'
  config: {
    url?: string
    method?: string
    headers?: Record<string, string>
    body?: string
    open_in_new_tab?: boolean
    code?: string
  }
  is_active: boolean
  display_order: number
  created_at: string
  updated_at: string
}

export interface ActionResult {
  success: boolean
  message?: string
  redirect_url?: string
  clipboard?: string
  toast?: {
    message: string
    type: 'success' | 'error' | 'info' | 'warning'
  }
  data?: Record<string, any>
}

export const customActionsService = {
  list: () => api.get<{ custom_actions: CustomAction[] }>('/custom-actions'),
  get: (id: string) => api.get<CustomAction>(`/custom-actions/${id}`),
  create: (data: {
    name: string
    icon?: string
    action_type: 'webhook' | 'url' | 'javascript'
    config: Record<string, any>
    is_active?: boolean
    display_order?: number
  }) => api.post<CustomAction>('/custom-actions', data),
  update: (id: string, data: {
    name?: string
    icon?: string
    action_type?: 'webhook' | 'url' | 'javascript'
    config?: Record<string, any>
    is_active?: boolean
    display_order?: number
  }) => api.put<CustomAction>(`/custom-actions/${id}`, data),
  delete: (id: string) => api.delete(`/custom-actions/${id}`),
  execute: (id: string, contactId: string) =>
    api.post<ActionResult>(`/custom-actions/${id}/execute`, { contact_id: contactId })
}

export default api
