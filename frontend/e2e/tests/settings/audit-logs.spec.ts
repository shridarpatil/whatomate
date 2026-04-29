import { test, expect } from '@playwright/test'
import { ApiHelper, loginAsAdmin, generateUniqueName, verifyAuditLogged } from '../../helpers'

/**
 * Audit logs E2E:
 *  - Generates a few audit entries via the API (create/update/delete a contact),
 *    so we have predictable rows to assert on.
 *  - Loads the list page, verifies entries appear.
 *  - Exercises filters (resource_type, action) and the detail view.
 *  - Confirms the API filter contract by calling /api/audit-logs directly via the
 *    shared verifyAuditLogged helper.
 */
test.describe('Audit Logs', () => {
  test('list view renders entries created via the API', async ({ page, request }) => {
    const api = new ApiHelper(request)
    await api.loginAsAdmin()

    // Create test data so the list isn't empty / dependent on prior runs.
    const contact = await api.createContact(`+1555${Date.now().toString().slice(-7)}`, generateUniqueName('AuditCt'))
    await verifyAuditLogged(request, 'contact', contact.id, 'created')

    await loginAsAdmin(page)
    await page.goto('/settings/audit-logs')
    await page.waitForLoadState('networkidle')

    // Page header + table skeleton present.
    await expect(page.getByRole('heading', { level: 1 })).toContainText(/Audit/i)
    await expect(page.locator('tbody')).toBeVisible()
    await expect.poll(async () => page.locator('tbody tr').count(), {
      timeout: 5_000,
    }).toBeGreaterThan(0)

    // The contact-create row we just produced should appear (admin user is the actor).
    const createdBadge = page.locator('tbody tr').filter({ hasText: /Created/i }).first()
    await expect(createdBadge).toBeVisible()
  })

  test('filter by action narrows the list', async ({ page, request }) => {
    const api = new ApiHelper(request)
    await api.loginAsAdmin()

    // Generate one of each action.
    const contact = await api.createContact(`+1555${Date.now().toString().slice(-7)}`, generateUniqueName('AuditCt'))
    await api.updateContact(contact.id, { profile_name: generateUniqueName('AuditCtUpd') })
    await verifyAuditLogged(request, 'contact', contact.id, 'updated')

    await loginAsAdmin(page)
    await page.goto('/settings/audit-logs')
    await page.waitForLoadState('networkidle')
    await expect(page.locator('tbody')).toBeVisible()

    // Open the action select and pick "Updated".
    const actionTrigger = page.locator('button[role="combobox"]').filter({ hasText: /All Actions|Updated|Created|Deleted/i }).first()
    await actionTrigger.click()
    await page.getByRole('option', { name: /^Updated$/ }).click()
    await page.waitForLoadState('networkidle')

    // Every visible action badge should be the filtered one.
    await expect.poll(async () => page.locator('tbody tr').count()).toBeGreaterThan(0)
    const badges = page.locator('tbody tr').locator('text=/^(Created|Updated|Deleted)$/i')
    const count = await badges.count()
    for (let i = 0; i < count; i++) {
      await expect(badges.nth(i)).toHaveText(/^Updated$/i)
    }
  })

  test('clicking a row navigates to the detail view with the change diff', async ({ page, request }) => {
    const api = new ApiHelper(request)
    await api.loginAsAdmin()

    // Create a contact then update its profile_name to produce a diff entry.
    const contact = await api.createContact(`+1555${Date.now().toString().slice(-7)}`, generateUniqueName('Before'))
    const newName = generateUniqueName('After')
    await api.updateContact(contact.id, { profile_name: newName })
    const updateLog = await verifyAuditLogged(request, 'contact', contact.id, 'updated', {
      expectedFields: ['profile_name'],
    })

    await loginAsAdmin(page)
    await page.goto(`/settings/audit-logs/${updateLog.id}`)
    await page.waitForLoadState('networkidle')

    // Detail view shows the changed field.
    await expect(page.getByText(/Changes/i).first()).toBeVisible()
    await expect(page.getByText(/Profile Name/i).first()).toBeVisible()
    await expect(page.getByText(newName).first()).toBeVisible()
    // Action badge shows "Updated".
    await expect(page.getByText(/^Updated$/).first()).toBeVisible()
  })

  test('detail view for a non-existent log shows not-found state', async ({ page }) => {
    await loginAsAdmin(page)
    // Random UUID — no such log exists.
    await page.goto('/settings/audit-logs/00000000-0000-0000-0000-000000000000')
    await page.waitForLoadState('networkidle')
    // The DetailPageLayout shows the not-found title from the view.
    await expect(page.getByText(/No (logs|audit logs)/i).first()).toBeVisible()
  })

  test('API filter by resource_type + resource_id returns scoped entries', async ({ request }) => {
    const api = new ApiHelper(request)
    await api.loginAsAdmin()

    const contact = await api.createContact(`+1555${Date.now().toString().slice(-7)}`, generateUniqueName('ScopedCt'))
    await verifyAuditLogged(request, 'contact', contact.id, 'created')

    // Calling the same endpoint with a wildcard resource_id (none specified) should
    // return many entries; with the specific contact id, only ours.
    const resp = await request.get(`${process.env.BASE_URL || 'http://localhost:8080'}/api/audit-logs?resource_type=contact&resource_id=${contact.id}&limit=10`)
    expect(resp.ok()).toBe(true)
    const body = await resp.json()
    const logs = body.data?.audit_logs ?? []
    expect(logs.length).toBeGreaterThan(0)
    for (const l of logs) {
      expect(l.resource_type).toBe('contact')
      expect(l.resource_id).toBe(contact.id)
    }
  })
})
