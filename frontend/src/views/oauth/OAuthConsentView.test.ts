// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import OAuthConsentView from './OAuthConsentView.vue'
import { pickInitialOrganization, readConsentParams } from './consent'

const mocks = vi.hoisted(() => ({
  getOAuthClient: vi.fn(),
  submitConsent: vi.fn(),
  listMyOrganizations: vi.fn(),
  navigateTo: vi.fn(),
  query: {} as Record<string, string>,
}))

const translate = (key: string, params?: Record<string, string>) =>
  params ? `${key} ${JSON.stringify(params)}` : key

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: translate }) }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: mocks.query }) }))
vi.mock('vue-sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: { email: 'owner@example.com' }, organizationId: 'org-1' }),
}))
vi.mock('@/services/api', () => ({
  mcpService: { getOAuthClient: mocks.getOAuthClient, submitConsent: mocks.submitConsent },
  usersService: { listMyOrganizations: mocks.listMyOrganizations },
}))
vi.mock('./consent', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./consent')>()),
  navigateTo: mocks.navigateTo,
}))

const baseQuery = {
  client_id: 'client-abc',
  redirect_uri: 'https://claude.ai/api/mcp/auth_callback',
  state: 'state-123',
  code_challenge: 'challenge-xyz',
  code_challenge_method: 'S256',
  resource: 'https://wa.example.com/mcp',
}

function mountView() {
  return mount(OAuthConsentView, { global: { mocks: { $t: translate } } })
}

describe('OAuthConsentView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    mocks.query = { ...baseQuery }
    mocks.getOAuthClient.mockResolvedValue({ data: { status: 'success', data: { client_name: 'Claude', redirect_uri_valid: true } } })
    mocks.listMyOrganizations.mockResolvedValue({ data: { status: 'success', data: { organizations: [
      { organization_id: 'org-1', name: 'Acme', is_default: true },
    ] } } })
    mocks.submitConsent.mockResolvedValue({ data: { status: 'success', data: { redirect_url: 'https://claude.ai/cb?code=abc&state=state-123' } } })
  })

  it('renders the client name, user email and organisation', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(mocks.getOAuthClient).toHaveBeenCalledWith({ client_id: 'client-abc', redirect_uri: baseQuery.redirect_uri })
    expect(wrapper.find('[data-testid="consent-app"]').text()).toBe('Claude')
    expect(wrapper.text()).toContain('owner@example.com')
    expect(wrapper.text()).toContain('Acme')
    expect(wrapper.find('select').exists()).toBe(false) // one org: picker hidden
    expect(wrapper.find('[data-testid="consent-approve"]').exists()).toBe(true)
  })

  it('approve posts the original params with the organisation and navigates back', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-testid="consent-approve"]').trigger('click')
    await flushPromises()

    expect(mocks.submitConsent).toHaveBeenCalledWith({
      ...baseQuery,
      organization_id: 'org-1',
      approve: true,
    })
    expect(mocks.navigateTo).toHaveBeenCalledWith('https://claude.ai/cb?code=abc&state=state-123')
  })

  it('deny posts approve:false and follows the returned redirect', async () => {
    mocks.submitConsent.mockResolvedValue({ data: { data: { redirect_url: 'https://claude.ai/cb?error=access_denied&state=state-123' } } })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-testid="consent-deny"]').trigger('click')
    await flushPromises()

    expect(mocks.submitConsent).toHaveBeenCalledWith(expect.objectContaining({ approve: false, organization_id: 'org-1', state: 'state-123' }))
    expect(mocks.navigateTo).toHaveBeenCalledWith('https://claude.ai/cb?error=access_denied&state=state-123')
  })

  it('shows the picker with the selected organisation preselected when there are several', async () => {
    localStorage.setItem('selected_organization_id', 'org-2')
    mocks.listMyOrganizations.mockResolvedValue({ data: { data: { organizations: [
      { organization_id: 'org-1', name: 'Acme', is_default: true },
      { organization_id: 'org-2', name: 'Beta' },
    ] } } })
    const wrapper = mountView()
    await flushPromises()

    expect((wrapper.find('select').element as HTMLSelectElement).value).toBe('org-2')
  })

  it('shows a clear error and no buttons for an invalid redirect URI', async () => {
    mocks.getOAuthClient.mockResolvedValue({ data: { data: { client_name: 'Evil', redirect_uri_valid: false } } })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="consent-error"]').text()).toContain('oauthConsent.invalidClient')
    expect(wrapper.find('[data-testid="consent-approve"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="consent-deny"]').exists()).toBe(false)
  })

  it('shows the invalid-client error when the client is unknown', async () => {
    mocks.getOAuthClient.mockRejectedValue({ response: { status: 404 } })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="consent-error"]').text()).toContain('oauthConsent.invalidClient')
    expect(wrapper.find('[data-testid="consent-approve"]').exists()).toBe(false)
  })

  it('rejects an incomplete request without calling the API', async () => {
    mocks.query = { client_id: 'client-abc' }
    const wrapper = mountView()
    await flushPromises()

    expect(mocks.getOAuthClient).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="consent-error"]').text()).toContain('oauthConsent.invalidRequest')
  })
})

describe('consent helpers', () => {
  it('keeps optional scope/resource only when present', () => {
    const { resource: _r, ...noResource } = baseQuery
    expect(readConsentParams(noResource)).not.toHaveProperty('resource')
    expect(readConsentParams({ ...baseQuery, scope: 'mcp' })?.scope).toBe('mcp')
    expect(readConsentParams({ ...baseQuery, code_challenge: '' })).toBeNull()
  })

  it('preselects the active org, then the default, then the first', () => {
    const orgs = [{ organization_id: 'a', name: 'A' }, { organization_id: 'b', name: 'B', is_default: true }]
    expect(pickInitialOrganization(orgs, ['zz', 'a'])).toBe('a')
    expect(pickInitialOrganization(orgs, [null])).toBe('b')
    expect(pickInitialOrganization([{ organization_id: 'c', name: 'C' }], [])).toBe('c')
    expect(pickInitialOrganization([], ['a'])).toBe('')
  })
})
