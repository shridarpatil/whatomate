import { test, expect, type Page } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { createTestScope } from '../../framework'

const scope = createTestScope('audit-logs')

/**
 * Audit logs E2E.
 *
 * Drives webhook CRUD via the UI to generate audit entries, then verifies
 * those entries surface on the audit-logs list and detail pages. Webhooks
 * are convenient because their detail page is a vanilla form-based create.
 */

async function createWebhookViaUI(page: Page, name: string): Promise<void> {
  await page.goto('/settings/webhooks/new')
  await page.waitForLoadState('networkidle')
  await page.getByPlaceholder('My Helpdesk Integration').fill(name)
  await page.getByPlaceholder('https://example.com/webhook').fill('https://webhook.site/e2e-audit')
  // Tick at least one event checkbox; first one is fine.
  await page.locator('button[role="checkbox"]').first().click()
  await page.getByRole('button', { name: /^(Create|Save)$/i }).first().click()
  await page.waitForURL(/\/settings\/webhooks\/[a-f0-9-]+$/, { timeout: 10000 })
  await page.waitForLoadState('networkidle')
}

async function updateWebhookViaUI(page: Page, name: string, newUrl: string): Promise<void> {
  await page.goto('/settings/webhooks')
  await page.waitForLoadState('networkidle')
  await page.getByPlaceholder(/Search/i).first().fill(name)
  await page.waitForTimeout(400)
  await page.locator('tbody tr .font-medium').getByText(name, { exact: true }).first().click()
  await page.waitForURL(/\/settings\/webhooks\/[a-f0-9-]+$/)
  await page.waitForLoadState('networkidle')

  const urlInput = page.getByPlaceholder('https://example.com/webhook')
  await urlInput.fill(newUrl)
  await page.waitForTimeout(200)
  await page.getByRole('button', { name: /^(Save|Update)$/i }).first().click()
  await page.waitForLoadState('networkidle')
}

test.describe('Audit Logs', () => {
  test('list view shows a Created entry after a UI create', async ({ page }) => {
    await loginAsAdmin(page)
    await createWebhookViaUI(page, scope.name('list'))

    await page.goto('/settings/audit-logs')
    await page.waitForLoadState('networkidle')

    await expect(page.getByRole('heading', { level: 1 })).toContainText(/Audit/i)
    await expect(page.locator('tbody')).toBeVisible()
    await expect.poll(async () => page.locator('tbody tr').count(), { timeout: 5_000 })
      .toBeGreaterThan(0)

    const createdBadge = page.locator('tbody tr').filter({ hasText: /Created/i }).first()
    await expect(createdBadge).toBeVisible()
  })

  test('action filter narrows the list to Updated entries after a UI edit', async ({ page }) => {
    await loginAsAdmin(page)

    const name = scope.name('filter')
    await createWebhookViaUI(page, name)
    await updateWebhookViaUI(page, name, 'https://webhook.site/e2e-audit-updated')

    await page.goto('/settings/audit-logs')
    await page.waitForLoadState('networkidle')
    await expect(page.locator('tbody')).toBeVisible()

    // Open the action select and pick "Updated".
    const actionTrigger = page.locator('button[role="combobox"]')
      .filter({ hasText: /All Actions|Updated|Created|Deleted/i })
      .first()
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

  test('clicking a row opens the detail view with the change diff', async ({ page }) => {
    await loginAsAdmin(page)

    const name = scope.name('detail')
    const newUrl = `https://webhook.site/e2e-audit-${scope.prefix}`
    await createWebhookViaUI(page, name)
    await updateWebhookViaUI(page, name, newUrl)

    // Filter to the most recent Updated entry and click into it.
    await page.goto('/settings/audit-logs')
    await page.waitForLoadState('networkidle')

    const actionTrigger = page.locator('button[role="combobox"]')
      .filter({ hasText: /All Actions|Updated|Created|Deleted/i })
      .first()
    await actionTrigger.click()
    await page.getByRole('option', { name: /^Updated$/ }).click()
    await page.waitForLoadState('networkidle')

    // First row should be our just-edited webhook (sorted desc by time).
    // The user_name cell is the routerlink — click it to navigate to detail.
    await page.locator('tbody tr').first().getByRole('link').first().click()
    await page.waitForURL(/\/settings\/audit-logs\/[a-f0-9-]+$/)
    await page.waitForLoadState('networkidle')

    await expect(page.getByText(/Changes/i).first()).toBeVisible()
    await expect(page.getByText(/Url/i).first()).toBeVisible()
    await expect(page.getByText(newUrl).first()).toBeVisible()
    await expect(page.getByText(/^Updated$/).first()).toBeVisible()
  })

  test('detail view for a non-existent log shows not-found state', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/audit-logs/00000000-0000-0000-0000-000000000000')
    await page.waitForLoadState('networkidle')
    await expect(page.getByText(/No (logs|audit logs)/i).first()).toBeVisible()
  })
})
