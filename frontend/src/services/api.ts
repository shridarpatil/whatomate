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

// Request interceptor to add auth token and organization header
api.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const token = localStorage.getItem('auth_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    // Add organization override header for super admins
    const selectedOrgId = localStorage.getItem('selected_organization_id')
    if (selectedOrgId) {
      config.headers['X-Organization-ID'] = selectedOrgId
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
  list: (params?: { search?: string; page?: number; limit?: number }) =>
    api.get('/users', { params }),
  get: (id: string) => api.get(`/users/${id}`),
  create: (data: { email: string; password: string; full_name: string; role_id?: string }) =>
    api.post('/users', data),
  update: (id: string, data: { email?: string; password?: string; full_name?: string; role_id?: string; is_active?: boolean }) =>
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
  list: (params?: { search?: string; page?: number; limit?: number }) =>
    api.get<{ api_keys: any[]; total?: number }>('/api-keys', { params }),
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
  list: (params?: { search?: string; page?: number; limit?: number; tags?: string }) =>
    api.get('/contacts', { params }),
  get: (id: string) => api.get(`/contacts/${id}`),
  create: (data: any) => api.post('/contacts', data),
  update: (id: string, data: any) => api.put(`/contacts/${id}`, data),
  delete: (id: string) => api.delete(`/contacts/${id}`),
  assign: (id: string, userId: string | null) =>
    api.put(`/contacts/${id}/assign`, { user_id: userId }),
  updateTags: (id: string, tags: string[]) =>
    api.put(`/contacts/${id}/tags`, { tags }),
  getSessionData: (id: string) => api.get(`/contacts/${id}/session-data`),
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
  list: (params?: { status?: string; category?: string; account?: string; search?: string; page?: number; limit?: number }) =>
    api.get<{ templates: any[]; total?: number }>('/templates', { params }),
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
  list: (params?: { account?: string; search?: string; page?: number; limit?: number }) =>
    api.get<{ flows: any[]; total?: number }>('/flows', { params }),
  get: (id: string) => api.get(`/flows/${id}`),
  create: (data: any) => api.post('/flows', data),
  update: (id: string, data: any) => api.put(`/flows/${id}`, data),
  delete: (id: string) => api.delete(`/flows/${id}`),
  saveToMeta: (id: string) => api.post(`/flows/${id}/save-to-meta`),
  publish: (id: string) => api.post(`/flows/${id}/publish`),
  deprecate: (id: string) => api.post(`/flows/${id}/deprecate`),
  duplicate: (id: string) => api.post(`/flows/${id}/duplicate`),
  sync: (whatsappAccount: string) => api.post('/flows/sync', { whatsapp_account: whatsappAccount })
}

export const campaignsService = {
  list: (params?: { status?: string; from?: string; to?: string; search?: string; page?: number; limit?: number }) =>
    api.get('/campaigns', { params }),
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
  listKeywords: (params?: { search?: string; page?: number; limit?: number }) =>
    api.get<{ rules: any[]; total?: number }>('/chatbot/keywords', { params }),
  getKeyword: (id: string) => api.get(`/chatbot/keywords/${id}`),
  createKeyword: (data: any) => api.post('/chatbot/keywords', data),
  updateKeyword: (id: string, data: any) => api.put(`/chatbot/keywords/${id}`, data),
  deleteKeyword: (id: string) => api.delete(`/chatbot/keywords/${id}`),

  // Flows
  listFlows: (params?: { search?: string; page?: number; limit?: number }) =>
    api.get<{ flows: any[]; total?: number }>('/chatbot/flows', { params }),
  getFlow: (id: string) => api.get(`/chatbot/flows/${id}`),
  createFlow: (data: any) => api.post('/chatbot/flows', data),
  updateFlow: (id: string, data: any) => api.put(`/chatbot/flows/${id}`, data),
  deleteFlow: (id: string) => api.delete(`/chatbot/flows/${id}`),

  // AI Contexts
  listAIContexts: (params?: { search?: string; page?: number; limit?: number }) =>
    api.get<{ contexts: any[]; total?: number }>('/chatbot/ai-contexts', { params }),
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
  list: (params?: { category?: string; search?: string; active_only?: string; page?: number; limit?: number }) =>
    api.get<{ canned_responses: CannedResponse[]; total?: number }>('/canned-responses', { params }),
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

// Meta WhatsApp Analytics Types
export type MetaAnalyticsType =
  | 'analytics'
  | 'conversation_analytics'
  | 'pricing_analytics'
  | 'template_analytics'
  | 'call_analytics'

export type MetaGranularity = 'HALF_HOUR' | 'DAY' | 'MONTH'

export interface MetaAnalyticsAccount {
  id: string
  name: string
  phone_id: string
}

export interface MetaMessagingDataPoint {
  start: number
  end: number
  sent: number
  delivered: number
}

export interface MetaConversationDataPoint {
  start: number
  end: number
  conversation: number
  conversation_type: string
  conversation_direction: string
  conversation_category: string
  cost: number
}

export interface MetaPricingDataPoint {
  start: number
  end: number
  volume: number
  cost: number
  country?: string              // Country code (IN, US, etc.)
  pricing_type?: string         // FREE_CUSTOMER_SERVICE, FREE_ENTRY_POINT, REGULAR
  pricing_category?: string     // MARKETING, UTILITY, AUTHENTICATION, SERVICE, etc.
  tier?: string                 // Pricing tier
}

export interface MetaTemplateCostItem {
  type: string    // amount_spent, cost_per_delivered, cost_per_url_button_click
  value?: number  // The cost value
}

export interface MetaTemplateClickItem {
  type: string           // quick_reply_button, unique_url_button
  button_content: string // The button text
  count: number          // Number of clicks
}

export interface MetaTemplateDataPoint {
  start: number
  end: number
  template_id: string
  sent: number
  delivered: number
  read: number
  replied?: number
  clicked?: MetaTemplateClickItem[]  // Array of button click details
  cost?: MetaTemplateCostItem[]
}

export interface MetaCallDataPoint {
  start: number
  end: number
  total_calls: number
  call_duration: number
  call_type: string
  call_direction: string
}

export interface MetaAnalyticsData {
  id: string
  analytics?: {
    granularity: string
    data_points: MetaMessagingDataPoint[]
  }
  conversation_analytics?: {
    granularity: string
    data_points: MetaConversationDataPoint[]
  }
  pricing_analytics?: {
    granularity: string
    data_points: MetaPricingDataPoint[]
  }
  template_analytics?: {
    granularity: string
    data_points: MetaTemplateDataPoint[]
  }
  call_analytics?: {
    granularity: string
    data_points: MetaCallDataPoint[]
  }
}

export interface MetaAnalyticsResponse {
  account_id: string
  account_name: string
  data: MetaAnalyticsData | null
  template_names?: Record<string, string> // meta_template_id -> template name
}

export const metaAnalyticsService = {
  get: (params: {
    account_id?: string
    analytics_type: MetaAnalyticsType
    start: string
    end: string
    granularity?: MetaGranularity
    template_ids?: string
  }) => api.get<{ accounts: MetaAnalyticsResponse[]; cached: boolean }>('/analytics/meta', { params }),

  getAccounts: () => api.get<{ accounts: MetaAnalyticsAccount[] }>('/analytics/meta/accounts'),

  refresh: () => api.post('/analytics/meta/refresh')
}

// Dashboard Widgets (customizable analytics)
export interface DashboardWidget {
  id: string
  name: string
  description: string
  data_source: string
  metric: string
  field: string
  filters: Array<{ field: string; operator: string; value: string }>
  display_type: string
  chart_type: string
  group_by_field: string
  show_change: boolean
  color: string
  size: string
  display_order: number
  grid_x: number
  grid_y: number
  grid_w: number
  grid_h: number
  config: Record<string, any>
  is_shared: boolean
  is_default: boolean
  is_owner: boolean
  created_by: string
  created_at: string
  updated_at: string
}

export interface WidgetData {
  widget_id: string
  value: number
  change: number
  prev_value: number
  chart_data: Array<{ label: string; value: number }>
  data_points: Array<{ label: string; value: number; color?: string }>
  grouped_series?: {
    labels: string[]
    datasets: Array<{ label: string; data: number[] }>
  }
  table_rows?: Array<{
    id: string
    label: string
    sub_label: string
    status: string
    direction?: string
    created_at: string
  }>
}

export interface DataSourceInfo {
  name: string
  label: string
  fields: string[]
}

export interface LayoutItem {
  id: string
  grid_x: number
  grid_y: number
  grid_w: number
  grid_h: number
}

export const widgetsService = {
  list: () => api.get<{ widgets: DashboardWidget[] }>('/widgets'),
  get: (id: string) => api.get<DashboardWidget>(`/widgets/${id}`),
  create: (data: {
    name: string
    description?: string
    data_source: string
    metric: string
    field?: string
    filters?: Array<{ field: string; operator: string; value: string }>
    display_type?: string
    chart_type?: string
    group_by_field?: string
    show_change?: boolean
    color?: string
    size?: string
    config?: Record<string, any>
    is_shared?: boolean
  }) => api.post<DashboardWidget>('/widgets', data),
  update: (id: string, data: Partial<{
    name: string
    description: string
    data_source: string
    metric: string
    field: string
    filters: Array<{ field: string; operator: string; value: string }>
    display_type: string
    chart_type: string
    group_by_field: string
    show_change: boolean
    color: string
    size: string
    config: Record<string, any>
    is_shared: boolean
  }>) => api.put<DashboardWidget>(`/widgets/${id}`, data),
  delete: (id: string) => api.delete(`/widgets/${id}`),
  getData: (id: string, params?: { from?: string; to?: string }) =>
    api.get<WidgetData>(`/widgets/${id}/data`, { params }),
  getAllData: (params?: { from?: string; to?: string }) =>
    api.get<{ data: Record<string, WidgetData> }>('/widgets/data', { params }),
  getDataSources: () => api.get<{
    data_sources: DataSourceInfo[]
    metrics: string[]
    display_types: string[]
    operators: Array<{ value: string; label: string }>
  }>('/widgets/data-sources'),
  saveLayout: (layout: LayoutItem[]) =>
    api.post('/widgets/layout', { layout })
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

// Organizations (super admin only)
export interface Organization {
  id: string
  name: string
  slug?: string
  created_at: string
}

export const organizationsService = {
  list: () => api.get<{ organizations: Organization[] }>('/organizations'),
  getCurrent: () => api.get<Organization>('/organizations/current')
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
  team_id?: string
  user_id: string
  role: 'manager' | 'agent'
  last_assigned_at: string | null
  // Flat structure from API
  full_name: string
  email: string
  is_available: boolean
  // Optional nested user for local additions
  user?: {
    id: string
    full_name: string
    email: string
    is_available: boolean
  }
}

export const teamsService = {
  list: (params?: { search?: string; page?: number; limit?: number }) =>
    api.get<{ teams: Team[] }>('/teams', { params }),
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
  list: (params?: { search?: string; page?: number; limit?: number }) =>
    api.get<{ webhooks: Webhook[]; available_events: WebhookEvent[]; total?: number }>('/webhooks', { params }),
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
  list: (params?: { search?: string; page?: number; limit?: number }) =>
    api.get<{ custom_actions: CustomAction[]; total?: number }>('/custom-actions', { params }),
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

// Roles and Permissions
export interface Permission {
  id: string
  resource: string
  action: string
  description: string
  key: string // "resource:action"
}

export interface Role {
  id: string
  name: string
  description: string
  is_system: boolean
  is_default: boolean
  permissions: string[] // ["resource:action", ...]
  user_count: number
  created_at: string
  updated_at: string
}

export const rolesService = {
  list: (params?: { search?: string; page?: number; limit?: number }) =>
    api.get<{ roles: Role[] }>('/roles', { params }),
  get: (id: string) => api.get<Role>(`/roles/${id}`),
  create: (data: { name: string; description?: string; is_default?: boolean; permissions: string[] }) =>
    api.post<Role>('/roles', data),
  update: (id: string, data: { name?: string; description?: string; is_default?: boolean; permissions?: string[] }) =>
    api.put<Role>(`/roles/${id}`, data),
  delete: (id: string) => api.delete(`/roles/${id}`)
}

export const permissionsService = {
  list: () => api.get<{ permissions: Permission[] }>('/permissions')
}

// Tags
export interface Tag {
  name: string
  color: string
  created_at: string
  updated_at: string
}

export const tagsService = {
  list: (params?: { search?: string; page?: number; limit?: number }) =>
    api.get<{ tags: Tag[]; total?: number; page?: number; limit?: number }>('/tags', { params }),
  create: (data: { name: string; color?: string }) =>
    api.post<Tag>('/tags', data),
  update: (name: string, data: { name?: string; color?: string }) =>
    api.put<Tag>(`/tags/${encodeURIComponent(name)}`, data),
  delete: (name: string) => api.delete(`/tags/${encodeURIComponent(name)}`)
}

export default api
