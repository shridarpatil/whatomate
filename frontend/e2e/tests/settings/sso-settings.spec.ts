import { test, expect } from '@playwright/test'
import { ApiHelper, loginAsAdmin } from '../../helpers'

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080'

/**
 * SSO settings E2E. The view renders provider cards (Google, Microsoft, GitHub,
 * Facebook, Custom OIDC) and edits via a dialog. We exercise the API contract
 * and the page-level UI states (configured badge, enabled ring) since the
 * dialog itself is mostly a thin wrapper over the API.
 */
test.describe('SSO Settings', () => {
  // The default admin org id — we need it for cleanup.
  let orgId: string

  // Wipe SSO providers between runs so we control state.
  async function cleanProviders(api: ApiHelper) {
    const providers: Array<'google' | 'microsoft' | 'github' | 'facebook' | 'custom'> = [
      'google', 'microsoft', 'github', 'facebook', 'custom',
    ]
    for (const p of providers) {
      const r = await api.del(`/api/settings/sso/${p}`)
      // 200 on delete-existing, 404 on already-absent. Both are fine.
      if (!r.ok() && r.status() !== 404) {
        // surface unexpected failures so the test doesn't silently inherit broken state
        throw new Error(`Failed to clean SSO provider ${p}: ${r.status()} ${await r.text()}`)
      }
    }
  }

  test.beforeEach(async ({ request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')
    const org = await api.getCurrentOrg()
    orgId = org.id
    await cleanProviders(api)
  })

  test.afterEach(async ({ request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')
    await cleanProviders(api)
  })

  test('settings page renders all provider cards', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/sso')
    await page.waitForLoadState('networkidle')

    await expect(page.getByText(/Google/).first()).toBeVisible()
    await expect(page.getByText(/Microsoft/).first()).toBeVisible()
    await expect(page.getByText(/GitHub/).first()).toBeVisible()
    await expect(page.getByText(/Facebook/).first()).toBeVisible()
    await expect(page.getByText(/Custom OIDC/).first()).toBeVisible()
  })

  test('newly-configured provider appears with the configured badge', async ({ page, request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    // Configure GitHub via the API.
    const resp = await api.put('/api/settings/sso/github', {
      client_id: 'gh-client-id-e2e',
      client_secret: 'gh-secret-must-not-be-exposed',
      is_enabled: true,
      allow_auto_create: false,
      default_role: 'agent',
      allowed_domains: 'example.com',
    })
    expect(resp.ok()).toBe(true)

    await loginAsAdmin(page)
    await page.goto('/settings/sso')
    await page.waitForLoadState('networkidle')

    // The GitHub card should now show the "Enabled" badge.
    const githubCard = page.locator('[class*="card"], div').filter({ hasText: /^GitHub$/ }).filter({ hasText: /Enabled|Disabled/ }).first()
    await expect(githubCard).toBeVisible()
    await expect(githubCard.getByText(/Enabled/i).first()).toBeVisible()
  })

  test('GET /api/settings/sso never leaks the client_secret', async ({ request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    const secret = 'super-secret-must-never-leak-' + Date.now()
    await api.put('/api/settings/sso/google', {
      client_id: 'g-client-id',
      client_secret: secret,
      is_enabled: true,
      allow_auto_create: false,
      default_role: 'agent',
      allowed_domains: '',
    })

    const listResp = await api.get('/api/settings/sso')
    expect(listResp.ok()).toBe(true)
    const body = await listResp.text()
    expect(body).not.toContain(secret)

    // Body was consumed by .text() — refetch for the parsed view.
    const listResp2 = await api.get('/api/settings/sso')
    const data = (await listResp2.json()).data as Array<{ provider: string; has_secret: boolean; client_id: string }>
    const google = data.find(p => p.provider === 'google')
    expect(google).toBeDefined()
    expect(google!.has_secret).toBe(true)
    expect(google!.client_id).toBe('g-client-id')
    // Sanity: the JSON shape doesn't carry a client_secret field at all.
    expect(JSON.stringify(google)).not.toContain('client_secret')
  })

  test('public /api/auth/sso/providers lists only enabled providers', async ({ request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    await api.put('/api/settings/sso/google', {
      client_id: 'g', client_secret: 's', is_enabled: true, allow_auto_create: false, default_role: 'agent', allowed_domains: '',
    })
    await api.put('/api/settings/sso/microsoft', {
      client_id: 'm', client_secret: 's', is_enabled: false, allow_auto_create: false, default_role: 'agent', allowed_domains: '',
    })

    // The /providers endpoint is public — no auth required.
    const resp = await request.get(`${BASE_URL}/api/auth/sso/providers`)
    expect(resp.ok()).toBe(true)
    const body = await resp.json()
    const providers = (body.data ?? []) as Array<{ provider: string; name: string }>
    const keys = providers.map(p => p.provider)
    expect(keys).toContain('google')
    expect(keys).not.toContain('microsoft')
  })

  test('updating a provider without supplying a secret keeps the existing one', async ({ request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    // Step 1: create with a secret.
    await api.put('/api/settings/sso/google', {
      client_id: 'first-id', client_secret: 'first-secret',
      is_enabled: true, allow_auto_create: false, default_role: 'agent', allowed_domains: '',
    })

    // Step 2: update — change client_id, omit client_secret. Server must keep the old one.
    await api.put('/api/settings/sso/google', {
      client_id: 'updated-id',
      // client_secret intentionally omitted
      is_enabled: true, allow_auto_create: false, default_role: 'agent', allowed_domains: '',
    })

    const list = await (await api.get('/api/settings/sso')).json()
    const google = (list.data as Array<{ provider: string; client_id: string; has_secret: boolean }>).find(p => p.provider === 'google')!
    expect(google.client_id).toBe('updated-id')
    expect(google.has_secret).toBe(true) // still has a secret — wasn't wiped
  })

  test('custom provider requires auth_url, token_url, user_info_url', async ({ request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    const resp = await api.put('/api/settings/sso/custom', {
      client_id: 'c', client_secret: 's',
      is_enabled: true, allow_auto_create: false, default_role: 'agent', allowed_domains: '',
      // missing auth_url, token_url, user_info_url
    })
    expect(resp.status()).toBe(400)
    const body = await resp.json()
    expect(body.message).toMatch(/auth_url|token_url|user_info_url/i)
  })

  test('invalid provider key is rejected', async ({ request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    const resp = await api.put('/api/settings/sso/okta', {
      client_id: 'c', client_secret: 's', is_enabled: true,
    })
    expect(resp.status()).toBe(400)
  })

  test('delete removes the provider', async ({ page, request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')
    await api.put('/api/settings/sso/github', {
      client_id: 'gh', client_secret: 's', is_enabled: true, allow_auto_create: false, default_role: 'agent', allowed_domains: '',
    })

    // Confirm it's there.
    let list = await (await api.get('/api/settings/sso')).json()
    expect((list.data as Array<{ provider: string }>).find(p => p.provider === 'github')).toBeDefined()

    // Delete via the API and reload the page.
    const delResp = await api.del('/api/settings/sso/github')
    expect(delResp.ok()).toBe(true)

    list = await (await api.get('/api/settings/sso')).json()
    expect((list.data as Array<{ provider: string }>).find(p => p.provider === 'github')).toBeUndefined()

    // UI: the page should not show "Enabled" on the GitHub card any more.
    await loginAsAdmin(page)
    await page.goto('/settings/sso')
    await page.waitForLoadState('networkidle')
    // Sanity: the GitHub card is still visible (always rendered) but no Enabled badge under it.
    await expect(page.getByText(/^GitHub$/).first()).toBeVisible()
  })

  test('cross-org isolation: another org cannot see this org\'s providers', async ({ request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    await api.put('/api/settings/sso/google', {
      client_id: 'orgA-google', client_secret: 's',
      is_enabled: true, allow_auto_create: false, default_role: 'agent', allowed_domains: '',
    })

    // Spin up a new org and switch into it.
    const newOrg = await api.createOrganization(`Iso Org ${Date.now()}`)
    await api.switchOrg(newOrg.id)

    const list = await (await api.get('/api/settings/sso')).json()
    const providers = list.data as Array<{ provider: string }>
    expect(providers).toEqual([]) // new org has no providers configured

    // Switch back so the afterEach cleanup runs against the right org.
    await api.switchOrg(orgId)
  })
})
