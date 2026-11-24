import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'

const MOCK_ACCOUNTS = {
  data: {
    accounts: [
      { id: 'acc-1', name: 'Account Alpha', phone_id: '111', business_id: '222', status: 'active' },
      { id: 'acc-2', name: 'Account Beta', phone_id: '333', business_id: '444', status: 'active' }
    ]
  }
}

const TEMPLATES_ALPHA = [
  { id: 'tpl-1', name: 'order_confirmation', display_name: 'Order Confirmation', status: 'APPROVED', language: 'en', whats_app_account: 'Account Alpha' },
  { id: 'tpl-2', name: 'shipping_update', display_name: 'Shipping Update', status: 'APPROVED', language: 'en', whats_app_account: 'Account Alpha' }
]

const TEMPLATES_BETA = [
  { id: 'tpl-3', name: 'welcome_message', display_name: '', status: 'APPROVED', language: 'en', whats_app_account: 'Account Beta' }
]

function setupMockRoutes(page: import('@playwright/test').Page) {
  return Promise.all([
    page.route('**/api/templates*', async route => {
      if (route.request().method() !== 'GET') { await route.continue(); return }
      const url = new URL(route.request().url())
      const account = url.searchParams.get('whatsapp_account') || url.searchParams.get('account')
      let templates = [...TEMPLATES_ALPHA, ...TEMPLATES_BETA]
      if (account === 'Account Alpha') templates = TEMPLATES_ALPHA
      else if (account === 'Account Beta') templates = TEMPLATES_BETA
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { templates, total: templates.length, page: 1, limit: 50 } })
      })
    }),
    page.route('**/api/accounts', async route => {
      if (route.request().method() !== 'GET') { await route.continue(); return }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(MOCK_ACCOUNTS)
      })
    }),
    page.route('**/api/campaigns*', async route => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ data: { campaigns: [], total: 0, page: 1, limit: 50 } })
        })
      } else {
        await route.continue()
      }
    }),
    page.route('**/api/audit-logs*', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { audit_logs: [], total: 0 } })
      })
    })
  ])
}

test.describe('Campaign Create - Template Loading', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
  })

  test('should not show templates before account is selected', async ({ page }) => {
    await setupMockRoutes(page)
    await page.goto('/campaigns/new')
    await page.waitForLoadState('networkidle')

    // Template select should be disabled when no account selected
    const templateSelect = page.locator('main select').nth(1)
    await expect(templateSelect).toBeDisabled()
  })

  test('should load templates filtered by selected account', async ({ page }) => {
    await setupMockRoutes(page)
    await page.goto('/campaigns/new')
    await page.waitForLoadState('networkidle')

    // Select account
    const accountSelect = page.locator('main select').first()
    await accountSelect.selectOption({ label: 'Account Alpha' })
    await page.waitForTimeout(500)

    // Template options (native select; the placeholder option is disabled)
    const templateSelect = page.locator('main select').nth(1)

    const options = templateSelect.locator('option:not([disabled])')
    await expect(options).toHaveCount(2)
    await expect(options.nth(0)).toContainText('Order Confirmation')
    await expect(options.nth(1)).toContainText('Shipping Update')
  })

  test('should show different templates when switching accounts', async ({ page }) => {
    await setupMockRoutes(page)
    await page.goto('/campaigns/new')
    await page.waitForLoadState('networkidle')

    // Select first account
    const accountSelect = page.locator('main select').first()
    await accountSelect.selectOption({ label: 'Account Alpha' })
    await page.waitForTimeout(500)

    // Verify 2 templates
    const templateSelect = page.locator('main select').nth(1)
    await expect(templateSelect.locator('option:not([disabled])')).toHaveCount(2)

    // Switch to second account
    await accountSelect.selectOption({ label: 'Account Beta' })
    await page.waitForTimeout(500)

    // Verify 1 template
    await expect(templateSelect.locator('option:not([disabled])')).toHaveCount(1)
  })

  test('should fall back to template name when display_name is empty', async ({ page }) => {
    await setupMockRoutes(page)
    await page.goto('/campaigns/new')
    await page.waitForLoadState('networkidle')

    // Select Account Beta (has template with empty display_name)
    const accountSelect = page.locator('main select').first()
    await accountSelect.selectOption({ label: 'Account Beta' })
    await page.waitForTimeout(500)

    // Should show template name as fallback
    const templateSelect = page.locator('main select').nth(1)
    await expect(templateSelect.locator('option:not([disabled])').first()).toContainText('welcome_message')
  })
})
