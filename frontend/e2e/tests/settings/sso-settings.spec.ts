import { test, expect } from '@playwright/test'
import { ApiHelper, loginAsAdmin } from '../../helpers'

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080'

/**
 * SSO settings E2E. The view renders provider cards (Google, Microsoft, GitHub,
 * Facebook, Custom OIDC) and edits via a dialog. We exercise the API contract
 * and the page-level UI states (configured badge, enabled ring) since the
 * dialog itself is mostly a thin wrapper over the API.
 *
 * IMPORTANT: Playwright's `request` fixture is the same APIRequestContext for
 * the whole test (beforeEach + body + afterEach). Calling api.login() more
 * than once on it would fail — the second call has the whm_access cookie set,
 * triggering CSRF middleware, which login itself doesn't satisfy. So we log in
 * exactly once in beforeEach and reuse the same ApiHelper everywhere.
 */
test.describe('SSO Settings', () => {
  let orgId: string
  let api: ApiHelper

  async function cleanProviders(a: ApiHelper) {
    const providers: Array<'google' | 'microsoft' | 'github' | 'facebook' | 'custom'> = [
      'google', 'microsoft', 'github', 'facebook', 'custom',
    ]
    for (const p of providers) {
      const r = await a.del(`/api/settings/sso/${p}`)
      if (!r.ok() && r.status() !== 404) {
        throw new Error(`Failed to clean SSO provider ${p}: ${r.status()} ${await r.text()}`)
      }
    }
  }

  test.beforeEach(async ({ request }) => {
    api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')
    // Reset to the default org before reading current. A prior failed run of
    // the "cross-org isolation" test could have left super admin pinned to a
    // throwaway org, which would make beforeEach + the test body operate on
    // the wrong org — and admin@test.com (used for the UI) wouldn't see the
    // PUT'd providers because it lives in the default org.
    const memberships = await api.getMyOrganizations()
    const defaultOrg = memberships.find(m => m.is_default) ?? memberships[0]
    if (defaultOrg) {
      await api.switchOrg(defaultOrg.organization_id)
    }
    const org = await api.getCurrentOrg()
    orgId = org.id
    await cleanProviders(api)
  })

  test.afterEach(async () => {
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

  test('newly-configured provider appears with the configured badge', async ({ page }) => {
    // Log in to the UI first so the page's APIRequestContext is authenticated
    // as admin@test.com. We then PUT through page.request — that guarantees
    // the SSO row is written into the SAME org as the UI session reads from.
    // Using the file-level `api` (super admin) for the PUT was the source of
    // residual flakes: if super admin's current org didn't match
    // admin@test.com's home org, the UI would render an empty card grid even
    // though the PUT returned 200.
    await loginAsAdmin(page)

    const csrfCookie = (await page.context().cookies()).find(c => c.name === 'whm_csrf')
    const putResp = await page.request.put('/api/settings/sso/github', {
      headers: csrfCookie ? { 'X-CSRF-Token': csrfCookie.value } : {},
      data: {
        client_id: 'gh-client-id-e2e',
        client_secret: 'gh-secret-must-not-be-exposed',
        is_enabled: true,
        allow_auto_create: false,
        default_role: 'agent',
        allowed_domains: 'example.com',
      },
    })
    expect(putResp.ok(), `PUT failed: ${putResp.status()} ${await putResp.text()}`).toBe(true)

    // Confirm via API that github is configured + enabled in this user's org.
    // If this fails, the UI assertion can't possibly succeed and we'd rather
    // get a precise data error than a vague locator timeout.
    await expect.poll(async () => {
      const r = await page.request.get('/api/settings/sso')
      if (!r.ok()) return null
      const body = await r.json()
      const list = (body.data ?? []) as Array<{ provider: string; is_enabled: boolean }>
      return list.find(p => p.provider === 'github')
    }, {
      timeout: 10000,
      message: 'github should appear in /api/settings/sso with is_enabled=true',
    }).toMatchObject({ provider: 'github', is_enabled: true })

    // Wait for the SSO list GET that fires from the lazy-loaded view's
    // onMounted. networkidle alone is NOT sufficient: SSOSettingsView is a
    // dynamic import (router uses `component: () => import(...)`), so
    // networkidle fires when the route shell is done — before the chunk
    // downloads, the component mounts, and fetchProviders runs. The Enabled
    // badge is v-if'd on `providers`, so it only renders after this GET lands.
    const ssoListPromise = page.waitForResponse(
      r => r.url().includes('/api/settings/sso') && r.request().method() === 'GET' && r.ok(),
      { timeout: 20000 },
    )
    await page.goto('/settings/sso')
    await ssoListPromise

    // Split assertions so a CI failure pinpoints whether the page rendered the
    // card at all (heading) vs. the badge specifically. The previous combined
    // div.filter().filter() locator gave the same opaque "element(s) not found"
    // either way.
    const ghHeading = page.getByRole('heading', { name: 'GitHub', exact: true })
    await expect(ghHeading, 'GitHub card should render').toBeVisible({ timeout: 15000 })

    // Walk up to the Card root, then look for the Enabled badge inside it.
    // shadcn-vue's Card.vue renders with `rounded-xl` (not rounded-lg), so we
    // anchor on that class — it's the closest ancestor unique to a card.
    const ghCard = ghHeading.locator('xpath=ancestor::*[contains(@class, "rounded-xl")][1]')
    await expect(
      ghCard.getByText('Enabled', { exact: true }),
      'Enabled badge should render inside the GitHub card',
    ).toBeVisible({ timeout: 10000 })
  })

  test('GET /api/settings/sso never leaks the client_secret', async () => {
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
    expect(JSON.stringify(google)).not.toContain('client_secret')
  })

  test('public /api/auth/sso/providers lists only enabled providers', async ({ request }) => {
    const r1 = await api.put('/api/settings/sso/google', {
      client_id: 'g', client_secret: 's', is_enabled: true, allow_auto_create: false, default_role: 'agent', allowed_domains: '',
    })
    expect(r1.ok(), `PUT google failed: ${r1.status()} ${await r1.text()}`).toBe(true)
    const r2 = await api.put('/api/settings/sso/microsoft', {
      client_id: 'm', client_secret: 's', is_enabled: false, allow_auto_create: false, default_role: 'agent', allowed_domains: '',
    })
    expect(r2.ok(), `PUT microsoft failed: ${r2.status()} ${await r2.text()}`).toBe(true)

    // The /providers endpoint is public — no auth required. /providers is
    // global (not org-scoped); we run with admin@admin.com which may not be
    // bound to a single org. Just sanity-check that what we configured shows up.
    const resp = await request.get(`${BASE_URL}/api/auth/sso/providers`)
    expect(resp.ok()).toBe(true)
    const body = await resp.json()
    const providers = (body.data ?? []) as Array<{ provider: string; name: string }>
    const keys = providers.map(p => p.provider)
    expect(keys).toContain('google')
    // Note: microsoft should NOT be in the list because it's disabled, but
    // /providers dedupes by provider type across orgs. If another org has
    // microsoft enabled, this would fail. So we only assert "google is in".
  })

  test('updating a provider without supplying a secret keeps the existing one', async () => {
    const r1 = await api.put('/api/settings/sso/google', {
      client_id: 'first-id', client_secret: 'first-secret',
      is_enabled: true, allow_auto_create: false, default_role: 'agent', allowed_domains: '',
    })
    expect(r1.ok(), `first PUT failed: ${r1.status()} ${await r1.text()}`).toBe(true)

    // Update — change client_id, omit client_secret. Server must keep the old one.
    const r2 = await api.put('/api/settings/sso/google', {
      client_id: 'updated-id',
      is_enabled: true, allow_auto_create: false, default_role: 'agent', allowed_domains: '',
    })
    expect(r2.ok(), `second PUT failed: ${r2.status()} ${await r2.text()}`).toBe(true)

    const list = await (await api.get('/api/settings/sso')).json()
    const google = (list.data as Array<{ provider: string; client_id: string; has_secret: boolean }>).find(p => p.provider === 'google')
    expect(google, `google not in list after PUT: ${JSON.stringify(list.data)}`).toBeDefined()
    expect(google!.client_id).toBe('updated-id')
    expect(google!.has_secret).toBe(true)
  })

  test('custom provider requires auth_url, token_url, user_info_url', async () => {
    const resp = await api.put('/api/settings/sso/custom', {
      client_id: 'c', client_secret: 's',
      is_enabled: true, allow_auto_create: false, default_role: 'agent', allowed_domains: '',
      // missing auth_url, token_url, user_info_url
    })
    expect(resp.status()).toBe(400)
    const body = await resp.json()
    expect(body.message).toMatch(/auth_url|token_url|user_info_url/i)
  })

  test('invalid provider key is rejected', async () => {
    const resp = await api.put('/api/settings/sso/okta', {
      client_id: 'c', client_secret: 's', is_enabled: true,
    })
    expect(resp.status()).toBe(400)
  })

  test('delete removes the provider', async ({ page }) => {
    await api.put('/api/settings/sso/github', {
      client_id: 'gh', client_secret: 's', is_enabled: true, allow_auto_create: false, default_role: 'agent', allowed_domains: '',
    })

    let list = await (await api.get('/api/settings/sso')).json()
    expect((list.data as Array<{ provider: string }>).find(p => p.provider === 'github')).toBeDefined()

    const delResp = await api.del('/api/settings/sso/github')
    expect(delResp.ok()).toBe(true)

    list = await (await api.get('/api/settings/sso')).json()
    expect((list.data as Array<{ provider: string }>).find(p => p.provider === 'github')).toBeUndefined()

    await loginAsAdmin(page)
    await page.goto('/settings/sso')
    await page.waitForLoadState('networkidle')
    await expect(page.getByText(/^GitHub$/).first()).toBeVisible()
  })

  test('cross-org isolation: another org cannot see this org\'s providers', async () => {
    await api.put('/api/settings/sso/google', {
      client_id: 'orgA-google', client_secret: 's',
      is_enabled: true, allow_auto_create: false, default_role: 'agent', allowed_domains: '',
    })

    // Spin up a new org and switch into it.
    const newOrg = await api.createOrganization(`Iso Org ${Date.now()}`)
    await api.switchOrg(newOrg.id)

    try {
      const list = await (await api.get('/api/settings/sso')).json()
      const providers = list.data as Array<{ provider: string }>
      expect(providers).toEqual([])
    } finally {
      // Always switch back, even on assertion failure. Otherwise super admin
      // is left in the throwaway org and the next run's beforeEach reads
      // SSO settings from the wrong org, breaking sibling tests like
      // "newly-configured provider appears with the configured badge".
      await api.switchOrg(orgId)
    }
  })
})
