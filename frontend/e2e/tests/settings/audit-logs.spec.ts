import { test, expect } from '@playwright/test'
import { ApiHelper, loginAsAdmin, generateUniqueName, verifyAuditLogged } from '../../helpers'

/**
 * Audit logs E2E:
 *  - Generates predictable audit entries via the API (uses webhooks, since the
 *    contact handler currently doesn't call audit.LogAudit — see fixme in
 *    audit-trail.spec.ts).
 *  - Loads the list page, verifies entries appear.
 *  - Exercises filters (action) and the detail view.
 *  - Confirms the API filter contract by querying /api/audit-logs directly.
 *
 * NOTE on auth: Playwright's `request` fixture is shared across before/test/
 * after hooks for one test run. Calling api.login() more than once on the
 * same context fails CSRF (the access cookie is already set, but login
 * doesn't send X-CSRF-Token). We therefore log in exactly once per test, and
 * use admin@admin.com (created by Go migrations) instead of admin@test.com
 * (which depends on global-setup having succeeded).
 */
test.describe('Audit Logs', () => {
  // Helper: create + update + delete a webhook to produce three audit entries.
  async function generateAuditEntries(api: ApiHelper) {
    const createResp = await api.post('/api/webhooks', {
      name: generateUniqueName('AuditPageHook'),
      url: 'https://webhook.site/audit-page',
      events: ['message.received'],
      is_active: true,
    })
    if (!createResp.ok()) {
      throw new Error(`Failed to seed webhook: ${createResp.status()} ${await createResp.text()}`)
    }
    const wh = (await createResp.json()).data
    return wh
  }

  test('list view renders entries created via the API', async ({ page, request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    const wh = await generateAuditEntries(api)
    await verifyAuditLogged(request, 'webhook', wh.id, 'created')

    await loginAsAdmin(page)
    await page.goto('/settings/audit-logs')
    await page.waitForLoadState('networkidle')

    await expect(page.getByRole('heading', { level: 1 })).toContainText(/Audit/i)
    await expect(page.locator('tbody')).toBeVisible()
    await expect.poll(async () => page.locator('tbody tr').count(), {
      timeout: 5_000,
    }).toBeGreaterThan(0)

    const createdBadge = page.locator('tbody tr').filter({ hasText: /Created/i }).first()
    await expect(createdBadge).toBeVisible()
  })

  test('filter by action narrows the list', async ({ page, request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    const wh = await generateAuditEntries(api)
    await api.put(`/api/webhooks/${wh.id}`, { url: 'https://webhook.site/audit-page-updated' })
    await verifyAuditLogged(request, 'webhook', wh.id, 'updated')

    await loginAsAdmin(page)
    await page.goto('/settings/audit-logs')
    await page.waitForLoadState('networkidle')
    await expect(page.locator('tbody')).toBeVisible()

    // Open the action select and pick "Updated".
    const actionTrigger = page.locator('button[role="combobox"]').filter({ hasText: /All Actions|Updated|Created|Deleted/i }).first()
    await actionTrigger.click()
    await page.getByRole('option', { name: /^Updated$/ }).click()
    await page.waitForLoadState('networkidle')

    await expect.poll(async () => page.locator('tbody tr').count()).toBeGreaterThan(0)
    const badges = page.locator('tbody tr').locator('text=/^(Created|Updated|Deleted)$/i')
    const count = await badges.count()
    for (let i = 0; i < count; i++) {
      await expect(badges.nth(i)).toHaveText(/^Updated$/i)
    }
  })

  test('clicking a row navigates to the detail view with the change diff', async ({ page, request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    const wh = await generateAuditEntries(api)
    const newURL = `https://webhook.site/audit-detail-${Date.now()}`
    await api.put(`/api/webhooks/${wh.id}`, { url: newURL })
    const updateLog = await verifyAuditLogged(request, 'webhook', wh.id, 'updated')

    await loginAsAdmin(page)
    await page.goto(`/settings/audit-logs/${updateLog.id}`)
    await page.waitForLoadState('networkidle')

    await expect(page.getByText(/Changes/i).first()).toBeVisible()
    // The diff should mention the URL field that changed.
    await expect(page.getByText(/Url/i).first()).toBeVisible()
    await expect(page.getByText(newURL).first()).toBeVisible()
    // Action badge shows "Updated".
    await expect(page.getByText(/^Updated$/).first()).toBeVisible()
  })

  test('detail view for a non-existent log shows not-found state', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/audit-logs/00000000-0000-0000-0000-000000000000')
    await page.waitForLoadState('networkidle')
    await expect(page.getByText(/No (logs|audit logs)/i).first()).toBeVisible()
  })

  test('API filter by resource_type + resource_id returns scoped entries', async ({ request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    const wh = await generateAuditEntries(api)
    await verifyAuditLogged(request, 'webhook', wh.id, 'created')

    const baseURL = process.env.BASE_URL || 'http://localhost:8080'
    const resp = await request.get(`${baseURL}/api/audit-logs?resource_type=webhook&resource_id=${wh.id}&limit=10`)
    expect(resp.ok()).toBe(true)
    const body = await resp.json()
    const logs = body.data?.audit_logs ?? []
    expect(logs.length).toBeGreaterThan(0)
    for (const l of logs) {
      expect(l.resource_type).toBe('webhook')
      expect(l.resource_id).toBe(wh.id)
    }
  })
})
