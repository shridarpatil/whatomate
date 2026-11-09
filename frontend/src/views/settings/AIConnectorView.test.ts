// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import en from '@/i18n/locales/en.json'
import es from '@/i18n/locales/es.json'
import pt from '@/i18n/locales/pt.json'
import ar from '@/i18n/locales/ar.json'
import hi from '@/i18n/locales/hi.json'
import ta from '@/i18n/locales/ta.json'
import { navigationSections } from '@/components/layout/navigation'
import { ConfirmDialog } from '@/components/shared'
import AIConnectorView from './AIConnectorView.vue'

const mocks = vi.hoisted(() => ({
  info: vi.fn(),
  listGrants: vi.fn(),
  revokeGrant: vi.fn(),
}))

const translate = (key: string) => key

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: translate }) }))
vi.mock('vue-sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ hasPermission: () => true }) }))
vi.mock('@/services/api', () => ({
  mcpService: { info: mocks.info, listGrants: mocks.listGrants, revokeGrant: mocks.revokeGrant },
}))

const grants = [
  {
    id: 'grant-1',
    client_name: 'Claude',
    organization_id: 'org-1',
    organization_name: 'Acme',
    created_at: '2026-11-01T10:00:00',
    last_used_at: '2026-11-08T14:05:00',
  },
  {
    id: 'grant-2',
    client_name: 'Claude Desktop',
    organization_id: 'org-2',
    organization_name: 'Beta',
    created_at: '2026-10-09T09:00:00',
    last_used_at: null,
  },
]

function mountView() {
  return mount(AIConnectorView, {
    attachTo: document.body,
    global: { mocks: { $t: translate }, stubs: { RouterLink: true, Teleport: true } },
  })
}

describe('AIConnectorView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.info.mockResolvedValue({ data: { status: 'success', data: { connector_url: 'https://wa.example.com/mcp' } } })
    mocks.listGrants.mockResolvedValue({ data: { status: 'success', data: { grants } } })
    mocks.revokeGrant.mockResolvedValue({ data: { status: 'success', data: {} } })
  })

  it('shows the connector URL and the Claude Code command', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="connector-url"]').text()).toBe('https://wa.example.com/mcp')
    expect(wrapper.find('[data-testid="claude-code-command"]').text()).toBe(
      'claude mcp add --transport http whatomate https://wa.example.com/mcp --header "X-API-Key: whm_..."',
    )
    wrapper.unmount()
  })

  it('lists connected apps with dd/mm/yyyy dates', async () => {
    const wrapper = mountView()
    await flushPromises()

    // DataTable renders a mobile card list and a desktop table; check the table.
    const table = wrapper.find('table')
    expect(table.findAll('[data-testid="grant-app"]').map(n => n.text())).toEqual(['Claude', 'Claude Desktop'])
    expect(table.text()).toContain('Acme')
    expect(table.findAll('[data-testid="grant-created"]').map(n => n.text())).toEqual(['01/11/2026', '09/10/2026'])
    expect(table.findAll('[data-testid="grant-last-used"]').map(n => n.text())).toEqual(['08/11/2026 14:05', 'aiConnector.never'])
    wrapper.unmount()
  })

  it('revokes a grant after confirmation and reloads the list', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('table').findAll('[data-testid="grant-revoke"]')[1].trigger('click')
    const dialog = wrapper.findComponent(ConfirmDialog)
    expect(dialog.props('open')).toBe(true)
    expect(mocks.revokeGrant).not.toHaveBeenCalled()

    dialog.vm.$emit('confirm')
    await flushPromises()

    expect(mocks.revokeGrant).toHaveBeenCalledWith('grant-2')
    expect(mocks.listGrants).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('shows the empty state when nothing is connected', async () => {
    mocks.listGrants.mockResolvedValue({ data: { data: { grants: [] } } })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('aiConnector.noApps')
    wrapper.unmount()
  })
})

describe('AI connector navigation and i18n', () => {
  it('lists the page under Settings for every logged-in user', () => {
    const settings = navigationSections.flatMap(s => s.items).find(i => i.path === '/settings')
    const item = settings?.children?.find(c => c.path === '/settings/ai-connector')
    expect(item).toBeDefined()
    expect(item?.permission).toBeUndefined()
  })

  const locales: Record<string, any> = { en, es, pt, ar, hi, ta }
  it.each(Object.keys(locales))('%s has every aiConnector and oauthConsent key', (name) => {
    for (const block of ['aiConnector', 'oauthConsent'] as const) {
      for (const key of Object.keys(en[block])) {
        expect(typeof locales[name][block]?.[key], `${name}.${block}.${key}`).toBe('string')
      }
    }
    expect(typeof locales[name].nav.aiConnector).toBe('string')
    expect(locales[name].aiConnector.revokeDesc).toContain('{app}')
    expect(locales[name].oauthConsent.wantsAccess).toContain('{email}')
  })
})
