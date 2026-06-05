import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'

const MOCK_ACCOUNTS = {
  data: {
    accounts: [
      { id: 'acc-1', name: 'Account Alpha', phone_id: '111', business_id: '222', status: 'active' }
    ]
  }
}

const TEMPLATES = [
  { id: 'tpl-mkt', name: 'marketing_offer', display_name: 'Special Discount Offer', category: 'MARKETING', status: 'APPROVED', language: 'en', whats_app_account: 'Account Alpha' },
  { id: 'tpl-utl', name: 'order_receipt', display_name: 'Receipt Update', category: 'UTILITY', status: 'APPROVED', language: 'en', whats_app_account: 'Account Alpha' }
]

function setupMockRoutes(page: import('@playwright/test').Page, marketingStatusPayload: any) {
  return Promise.all([
    page.route('**/api/templates*', async route => {
      if (route.request().method() !== 'GET') { await route.continue(); return }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { templates: TEMPLATES, total: TEMPLATES.length, page: 1, limit: 50 } })
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
    page.route('**/api/accounts/acc-1/marketing-status', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(marketingStatusPayload)
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

test.describe('Campaign Creation - Marketing Terms Verification', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
  })

  test('should not show Smart Optimization for non-marketing templates', async ({ page }) => {
    await setupMockRoutes(page, { status: 'ONBOARDED' })
    await page.goto('/campaigns/new')
    await page.waitForLoadState('networkidle')

    // Select Account Alpha
    const accountSelect = page.locator('button[role="combobox"]').first()
    await accountSelect.click()
    await page.getByRole('option', { name: 'Account Alpha' }).click()
    await page.waitForTimeout(500)

    // Select UTILITY template
    const templateSelect = page.locator('button[role="combobox"]').nth(1)
    await templateSelect.click()
    await page.getByRole('option', { name: 'Receipt Update' }).click()
    await page.waitForTimeout(500)

    // Verify Smart Optimization toggle is not visible
    await expect(page.getByText('Smart Marketing Optimization')).not.toBeVisible()
  })

  test('should show enabled Smart Optimization toggle and active state warning when status is ONBOARDED', async ({ page }) => {
    await setupMockRoutes(page, { status: 'ONBOARDED' })
    await page.goto('/campaigns/new')
    await page.waitForLoadState('networkidle')

    // Select Account Alpha
    const accountSelect = page.locator('button[role="combobox"]').first()
    await accountSelect.click()
    await page.getByRole('option', { name: 'Account Alpha' }).click()
    await page.waitForTimeout(500)

    // Select MARKETING template
    const templateSelect = page.locator('button[role="combobox"]').nth(1)
    await templateSelect.click()
    await page.getByRole('option', { name: 'Special Discount Offer' }).click()
    await page.waitForTimeout(500)

    // Verify Smart Optimization toggle is visible and active text exists
    await expect(page.getByText('Smart Marketing Optimization')).toBeVisible()
    await expect(page.getByText('Marketing Messages: Active')).toBeVisible()
    
    // Checkbox should not be disabled
    const checkbox = page.locator('#optimize_delivery')
    await expect(checkbox).toBeEnabled()
    await expect(checkbox).toBeChecked()
  })

  test('should disable Smart Optimization toggle and show warning when status is pending or not onboarded', async ({ page }) => {
    await setupMockRoutes(page, { status: 'PENDING' })
    await page.goto('/campaigns/new')
    await page.waitForLoadState('networkidle')

    // Select Account Alpha
    const accountSelect = page.locator('button[role="combobox"]').first()
    await accountSelect.click()
    await page.getByRole('option', { name: 'Account Alpha' }).click()
    await page.waitForTimeout(500)

    // Select MARKETING template
    const templateSelect = page.locator('button[role="combobox"]').nth(1)
    await templateSelect.click()
    await page.getByRole('option', { name: 'Special Discount Offer' }).click()
    await page.waitForTimeout(500)

    // Verify Smart Optimization toggle is visible but disabled, and unchecked by default
    await expect(page.getByText('Smart Marketing Optimization')).toBeVisible()
    await expect(page.getByText('Smart Optimization requires accepting the WhatsApp Marketing Messages Terms of Service.')).toBeVisible()
    
    const checkbox = page.locator('#optimize_delivery')
    await expect(checkbox).toBeDisabled()
    await expect(checkbox).not.toBeChecked()
  })

  test('should disable toggle and show terms warning when API check fails', async ({ page }) => {
    // API error response
    await setupMockRoutes(page, { status: '', api_error: true, api_error_message: 'Graph API error' })
    await page.goto('/campaigns/new')
    await page.waitForLoadState('networkidle')

    // Select Account Alpha
    const accountSelect = page.locator('button[role="combobox"]').first()
    await accountSelect.click()
    await page.getByRole('option', { name: 'Account Alpha' }).click()
    await page.waitForTimeout(500)

    // Select MARKETING template
    const templateSelect = page.locator('button[role="combobox"]').nth(1)
    await templateSelect.click()
    await page.getByRole('option', { name: 'Special Discount Offer' }).click()
    await page.waitForTimeout(500)

    // Verify Smart Optimization toggle is visible but disabled, and standard warning shows up
    await expect(page.getByText('Smart Marketing Optimization')).toBeVisible()
    await expect(page.getByText('Could not verify Marketing Messages Terms status. Smart Optimization may still be available.')).not.toBeVisible()
    await expect(page.getByText('Smart Optimization requires accepting the WhatsApp Marketing Messages Terms of Service.')).toBeVisible()
    
    const checkbox = page.locator('#optimize_delivery')
    await expect(checkbox).toBeDisabled()
    await expect(checkbox).not.toBeChecked()
  })
})
