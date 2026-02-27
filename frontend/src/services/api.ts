import axios, { type AxiosInstance, type AxiosError, type InternalAxiosRequestConfig } from 'axios'

// Get base path from server-injected config or fallback
const basePath = ((window as any).__BASE_PATH__ ?? '').replace(/\/$/, '')
const API_BASE_URL = import.meta.env.VITE_API_URL || `${basePath}/api`

export const api: AxiosInstance = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  withCredentials: true,
  headers: {
    'Content-Type': 'application/json'
  }
})

// Helper to read a cookie by name
function getCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp('(?:^|; )' + name + '=([^;]*)'))
  return match ? decodeURIComponent(match[1]) : null
}

// Request interceptor to add CSRF token and organization header
api.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    // Add CSRF token on mutating requests (cookie-based auth sends cookies automatically)
    const method = (config.method || '').toUpperCase()
    if (method === 'POST' || method === 'PUT' || method === 'DELETE' || method === 'PATCH') {
      const csrfToken = getCookie('whm_csrf')
      if (csrfToken) {
        config.headers['X-CSRF-Token'] = csrfToken
      }
    }
    // Add organization override header for org switching
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

      try {
        // Browser sends whm_refresh cookie automatically via withCredentials
        await axios.post(`${API_BASE_URL}/auth/refresh`, {}, { withCredentials: true })

        // Cookies are updated by the server response — retry the original request
        return api(originalRequest)
      } catch {
        // Refresh failed, clear user and redirect to login
        localStorage.removeItem('user')
        localStorage.removeItem('auth_token')
        localStorage.removeItem('refresh_token')
        window.location.href = basePath + '/login'
      }
    }

    return Promise.reject(error)
  }
)

// API service methods
export const authService = {
  getWSToken: () => api.get('/auth/ws-token'),
}

export const usersService = {
  list: (params?: { search?: string; page?: number; limit?: number }) =>
    api.get('/users', { params }),
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
    api.put('/me/availability', { is_available: isAvailable }),
  listMyOrganizations: () => api.get('/me/organizations'),
}

export const apiKeysService = {
  list: (params?: { search?: string; page?: number; limit?: number }) =>
    api.get<{ api_keys: any[]; total?: number }>('/api-keys', { params }),
  create: (data: { name: string; expires_at?: string }) =>
    api.post('/api-keys', data),
  delete: (id: string) => api.delete(`/api-keys/${id}`)
}

export const accountsService = {
  list: () => api.get('/accounts')
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
  getSessionData: (id: string) => api.get(`/contacts/${id}/session-data`)
}

// Generic Import/Export Service
export interface ExportColumn {
  key: string
  label: string
}

export interface ExportConfig {
  table: string
  columns: ExportColumn[]
  default_columns: string[]
}

export interface ImportConfig {
  table: string
  required_columns: ExportColumn[]
  optional_columns: ExportColumn[]
  unique_column: string
}

export interface ImportResult {
  created: number
  updated: number
  skipped: number
  errors: number
  messages: string[]
}

export const dataService = {
  // Get export configuration for a table
  getExportConfig: (table: string) => api.get<ExportConfig>(`/export/${table}/config`),

  // Get import configuration for a table
  getImportConfig: (table: string) => api.get<ImportConfig>(`/import/${table}/config`),

  // Export data - returns CSV blob
  exportData: async (table: string, columns?: string[], filters?: Record<string, string>) => {
    const response = await api.post('/export', { table, columns, filters }, {
      responseType: 'blob'
    })
    return response
  },

  // Import data from CSV file
  importData: (table: string, file: File, updateOnDuplicate?: boolean, columnMapping?: Record<string, string>) => {
    const formData = new FormData()
    formData.append('file', file)
    formData.append('table', table)
    if (updateOnDuplicate) {
      formData.append('update_on_duplicate', 'true')
    }
    if (columnMapping) {
      formData.append('column_mapping', JSON.stringify(columnMapping))
    }
    return api.post<ImportResult>('/import', formData, {
      headers: { 'Content-Type': 'multipart/form-data' }
    })
  }
}

export const messagesService = {
  list: (contactId: string, params?: { page?: number; limit?: number; before_id?: string; account?: string }) =>
    api.get(`/contacts/${contactId}/messages`, { params }),
  send: (contactId: string, data: { type: string; content: any; reply_to_message_id?: string; whatsapp_account?: string }) =>
    api.post(`/contacts/${contactId}/messages`, data),
  sendTemplate: (contactId: string, data: { template_name: string; template_params?: Record<string, string>; account_name?: string }) =>
    api.post('/messages/template', { contact_id: contactId, ...data }),
  sendReaction: (contactId: string, messageId: string, emoji: string) =>
    api.post(`/contacts/${contactId}/messages/${messageId}/reaction`, { emoji })
}

export const templatesService = {
  list: (params?: { status?: string; category?: string; account?: string; search?: string; page?: number; limit?: number }) =>
    api.get<{ templates: any[]; total?: number }>('/templates', { params }),
  get: (id: string) => api.get(`/templates/${id}`),
  uploadMedia: (accountName: string, file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    formData.append('account', accountName)
    const csrfToken = getCookie('whm_csrf')
    return axios.post(`${api.defaults.baseURL}/templates/upload-media`, formData, {
      withCredentials: true,
      headers: csrfToken ? { 'X-CSRF-Token': csrfToken } : {}
    })
  }
}

export const flowsService = {
  list: (params?: { account?: string; search?: string; page?: number; limit?: number }) =>
    api.get<{ flows: any[]; total?: number }>('/flows', { params }),
  create: (data: any) => api.post('/flows', data),
  update: (id: string, data: any) => api.put(`/flows/${id}`, data),
  delete: (id: string) => api.delete(`/flows/${id}`),
  saveToMeta: (id: string) => api.post(`/flows/${id}/save-to-meta`),
  publish: (id: string) => api.post(`/flows/${id}/publish`),
  duplicate: (id: string) => api.post(`/flows/${id}/duplicate`),
  sync: (whatsappAccount: string) => api.post('/flows/sync', { whatsapp_account: whatsappAccount })
}

export const campaignsService = {
  list: (params?: { status?: string; from?: string; to?: string; search?: string; page?: number; limit?: number }) =>
    api.get('/campaigns', { params }),
  create: (data: any) => api.post('/campaigns', data),
  update: (id: string, data: any) => api.put(`/campaigns/${id}`, data),
  delete: (id: string) => api.delete(`/campaigns/${id}`),
  start: (id: string) => api.post(`/campaigns/${id}/start`),
  pause: (id: string) => api.post(`/campaigns/${id}/pause`),
  cancel: (id: string) => api.post(`/campaigns/${id}/cancel`),
  retryFailed: (id: string) => api.post(`/campaigns/${id}/retry-failed`),
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
    const csrfToken = getCookie('whm_csrf')
    return axios.post(`${api.defaults.baseURL}/campaigns/${campaignId}/media`, formData, {
      withCredentials: true,
      headers: csrfToken ? { 'X-CSRF-Token': csrfToken } : {}
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
  createAIContext: (data: any) => api.post('/chatbot/ai-contexts', data),
  updateAIContext: (id: string, data: any) => api.put(`/chatbot/ai-contexts/${id}`, data),
  deleteAIContext: (id: string) => api.delete(`/chatbot/ai-contexts/${id}`),

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
  create: (data: { name: string; shortcut?: string; content: string; category?: string }) =>
    api.post('/canned-responses', data),
  update: (id: string, data: { name?: string; shortcut?: string; content?: string; category?: string; is_active?: boolean }) =>
    api.put(`/canned-responses/${id}`, data),
  delete: (id: string) => api.delete(`/canned-responses/${id}`),
  use: (id: string) => api.post(`/canned-responses/${id}/use`)
}

export const agentAnalyticsService = {
  getSummary: (params?: { from?: string; to?: string; agent_id?: string }) =>
    api.get('/analytics/agents', { params })
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

interface MetaConversationDataPoint {
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

interface MetaTemplateCostItem {
  type: string    // amount_spent, cost_per_delivered, cost_per_url_button_click
  value?: number  // The cost value
}

interface MetaTemplateClickItem {
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

interface MetaAnalyticsData {
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

interface DataSourceInfo {
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
    calling_enabled?: boolean
    max_call_duration?: number
    transfer_timeout_secs?: number
    hold_music_file?: string
    ringback_file?: string
  }) => api.put('/org/settings', data),
  uploadOrgAudio: (file: File, type: 'hold_music' | 'ringback') => {
    const formData = new FormData()
    formData.append('file', file)
    return api.post(`/org/audio?type=${type}`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' }
    })
  }
}

// Organizations
export interface Organization {
  id: string
  name: string
  slug?: string
  created_at: string
}

export const organizationsService = {
  list: () => api.get<{ organizations: Organization[] }>('/organizations'),
  create: (data: { name: string }) => api.post('/organizations', data),
  // Members
  addMember: (data: { user_id?: string; email?: string; role_id?: string }) =>
    api.post('/organizations/members', data),
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

// Conversation Notes
export interface ConversationNote {
  id: string
  contact_id: string
  created_by_id: string
  created_by_name: string
  content: string
  created_at: string
  updated_at: string
}

export const notesService = {
  list: (contactId: string, params?: { limit?: number; before?: string }) =>
    api.get<{ notes: ConversationNote[]; total: number; has_more: boolean }>(`/contacts/${contactId}/notes`, { params }),
  create: (contactId: string, data: { content: string }) =>
    api.post<ConversationNote>(`/contacts/${contactId}/notes`, data),
  update: (contactId: string, noteId: string, data: { content: string }) =>
    api.put<ConversationNote>(`/contacts/${contactId}/notes/${noteId}`, data),
  delete: (contactId: string, noteId: string) =>
    api.delete(`/contacts/${contactId}/notes/${noteId}`)
}

// Calling - Call Logs & IVR Flows
export interface CallLog {
  id: string
  organization_id: string
  whatsapp_account: string
  contact_id: string
  whatsapp_call_id: string
  caller_phone: string
  direction: 'incoming' | 'outgoing'
  status: 'ringing' | 'answered' | 'completed' | 'missed' | 'rejected' | 'failed' | 'initiating' | 'accepted'
  duration: number
  ivr_flow_id?: string
  ivr_path?: Record<string, any>
  agent_id?: string
  started_at?: string
  answered_at?: string
  ended_at?: string
  error_message?: string
  recording_s3_key?: string
  recording_duration?: number
  contact?: {
    id: string
    phone_number: string
    profile_name: string
  }
  agent?: {
    id: string
    full_name: string
    email: string
  }
  ivr_flow?: IVRFlow
  created_at: string
  updated_at: string
}

export interface IVRMenuOption {
  label: string
  action: 'transfer' | 'submenu' | 'parent' | 'repeat' | 'hangup' | 'goto_flow'
  target?: string
  menu?: IVRMenu
}

export interface IVRMenu {
  greeting: string
  greeting_text?: string
  options: Record<string, IVRMenuOption>
  timeout_seconds?: number
  max_retries?: number
  invalid_input_message?: string
}

export interface IVRFlow {
  id: string
  organization_id: string
  whatsapp_account: string
  name: string
  description: string
  is_active: boolean
  is_call_start: boolean
  menu: IVRMenu
  welcome_audio_url: string
  created_at: string
  updated_at: string
}

export interface CallTransfer {
  id: string
  organization_id: string
  call_log_id: string
  whatsapp_call_id: string
  caller_phone: string
  contact_id: string
  whatsapp_account: string
  status: 'waiting' | 'connected' | 'completed' | 'abandoned' | 'no_answer'
  team_id?: string
  agent_id?: string
  transferred_at: string
  connected_at?: string
  completed_at?: string
  hold_duration: number
  talk_duration: number
  ivr_path?: Record<string, any>
  contact?: {
    id: string
    phone_number: string
    profile_name: string
  }
  agent?: {
    id: string
    full_name: string
    email: string
  }
  team?: {
    id: string
    name: string
  }
  call_log?: CallLog
  created_at: string
  updated_at: string
}

// Outgoing Calls
export interface CallPermission {
  id: string
  contact_id: string
  whatsapp_account: string
  status: 'pending' | 'accepted' | 'declined' | 'expired'
  message_id?: string
  requested_at: string
  responded_at?: string
  expires_at?: string
}

export const outgoingCallsService = {
  initiate: (data: { contact_id: string; whatsapp_account: string; sdp_offer: string }) =>
    api.post<{ call_log_id: string; sdp_answer: string }>('/calls/outgoing', data),
  hangup: (callLogId: string) =>
    api.post(`/calls/outgoing/${callLogId}/hangup`),
  requestPermission: (data: { contact_id: string; whatsapp_account: string }) =>
    api.post<{ permission_id: string }>('/calls/permission-request', data),
  getPermission: (contactId: string, whatsappAccount: string) =>
    api.get<CallPermission>(`/calls/permission/${contactId}`, { params: { whatsapp_account: whatsappAccount } }),
  getICEServers: () =>
    api.get<{ ice_servers: Array<{ urls: string[]; username?: string; credential?: string }> }>('/calls/ice-servers'),
}

export const callLogsService = {
  list: (params?: { status?: string; account?: string; contact_id?: string; direction?: string; ivr_flow_id?: string; phone?: string; from?: string; to?: string; page?: number; limit?: number }) =>
    api.get<{ call_logs: CallLog[]; total: number }>('/call-logs', { params }),
  get: (id: string) => api.get<CallLog>(`/call-logs/${id}`),
  getRecordingURL: (id: string) =>
    api.get<{ url: string; duration: number }>(`/call-logs/${id}/recording`)
}

export const callTransfersService = {
  list: (params?: { status?: string; page?: number; limit?: number }) =>
    api.get<{ call_transfers: CallTransfer[]; total: number }>('/call-transfers', { params }),
  get: (id: string) => api.get<CallTransfer>(`/call-transfers/${id}`),
  connect: (id: string, sdpOffer: string) =>
    api.post<{ sdp_answer: string }>(`/call-transfers/${id}/connect`, { sdp_offer: sdpOffer }),
  hangup: (id: string) =>
    api.post(`/call-transfers/${id}/hangup`),
}

export const ivrFlowsService = {
  list: (params?: { search?: string; page?: number; limit?: number }) =>
    api.get<{ ivr_flows: IVRFlow[]; total: number }>('/ivr-flows', { params }),
  get: (id: string) => api.get<IVRFlow>(`/ivr-flows/${id}`),
  create: (data: { whatsapp_account: string; name: string; description?: string; is_call_start?: boolean; menu: IVRMenu; welcome_audio_url?: string }) =>
    api.post<IVRFlow>('/ivr-flows', data),
  update: (id: string, data: { name?: string; description?: string; is_active?: boolean; is_call_start?: boolean; menu?: IVRMenu; welcome_audio_url?: string }) =>
    api.put<IVRFlow>(`/ivr-flows/${id}`, data),
  delete: (id: string) => api.delete(`/ivr-flows/${id}`),
  uploadAudio: (file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    const csrfToken = getCookie('whm_csrf')
    const headers: Record<string, string> = {}
    if (csrfToken) headers['X-CSRF-Token'] = csrfToken
    const selectedOrgId = localStorage.getItem('selected_organization_id')
    if (selectedOrgId) headers['X-Organization-ID'] = selectedOrgId
    return axios.post(`${api.defaults.baseURL}/ivr-flows/audio`, formData, {
      withCredentials: true,
      headers,
    })
  },
  getAudioUrl: (filename: string) => `${api.defaults.baseURL}/ivr-flows/audio/${encodeURIComponent(filename)}`
}

export default api
