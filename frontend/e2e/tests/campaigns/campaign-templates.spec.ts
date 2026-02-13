import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { CampaignsPage } from '../../pages'

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

const MOCK_CAMPAIGNS = {
  data: { campaigns: [], total: 0, page: 1, limit: 50 }
}

function setupMockRoutes(page: import('@playwright/test').Page) {
  return Promise.all([
    page.route('**/api/templates*', async route => {
      if (route.request().method() !== 'GET') { await route.continue(); return }
      const url = new URL(route.request().url())
      const account = url.searchParams.get('account')
      let templates = [...TEMPLATES_ALPHA, ...TEMPLATES_BETA]
      if (account === 'Account Alpha') templates = TEMPLATES_ALPHA
      else if (account === 'Account Beta') templates = TEMPLATES_BETA
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { templates, total: templates.length, page: 1, limit: 50 } })
      })
    }),
    page.route('**/api/accounts*', async route => {
      if (route.request().method() !== 'GET') { await route.continue(); return }
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_ACCOUNTS) })
    }),
    page.route('**/api/campaigns*', async route => {
      if (route.request().method() !== 'GET') { await route.continue(); return }
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_CAMPAIGNS) })
    })
  ])
}

test.describe('Campaign Create - Template Loading', () => {
  let campaignsPage: CampaignsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    campaignsPage = new CampaignsPage(page)
  })

  test('should not show templates before account is selected', async ({ page }) => {
    await setupMockRoutes(page)
    await campaignsPage.goto()
    await campaignsPage.openCreateDialog()

    await expect(campaignsPage.getNoTemplatesMessage()).toBeVisible()
  })

  test('should load templates filtered by selected account', async ({ page }) => {
    await setupMockRoutes(page)
    await campaignsPage.goto()
    await campaignsPage.openCreateDialog()

    await campaignsPage.selectAccount('Account Alpha')
    await campaignsPage.openTemplateDropdown()

    const options = campaignsPage.getTemplateOptions()
    await expect(options).toHaveCount(2)
    await expect(options.nth(0)).toContainText('Order Confirmation')
    await expect(options.nth(1)).toContainText('Shipping Update')
  })

  test('should show different templates when switching accounts', async ({ page }) => {
    await setupMockRoutes(page)
    await campaignsPage.goto()
    await campaignsPage.openCreateDialog()

    // Select first account
    await campaignsPage.selectAccount('Account Alpha')
    await campaignsPage.openTemplateDropdown()
    await expect(campaignsPage.getTemplateOptions()).toHaveCount(2)
    // Close dropdown by pressing Escape
    await page.keyboard.press('Escape')

    // Switch to second account
    await campaignsPage.selectAccount('Account Beta')
    await campaignsPage.openTemplateDropdown()
    const options = campaignsPage.getTemplateOptions()
    await expect(options).toHaveCount(1)
    await expect(options.first()).toContainText('welcome_message')
  })

  test('should clear selected template when account changes', async ({ page }) => {
    await setupMockRoutes(page)
    await campaignsPage.goto()
    await campaignsPage.openCreateDialog()

    // Select account and template
    await campaignsPage.selectAccount('Account Alpha')
    await campaignsPage.selectTemplate('Order Confirmation')
    await expect(campaignsPage.getTemplateSelectTrigger()).toContainText('Order Confirmation')

    // Switch account — template selection should reset
    await campaignsPage.selectAccount('Account Beta')
    await expect(campaignsPage.getTemplateSelectTrigger()).not.toContainText('Order Confirmation')
  })

  test('should pass account param in template API request', async ({ page }) => {
    let capturedAccountParam: string | null = null
    await page.route('**/api/templates*', async route => {
      if (route.request().method() !== 'GET') { await route.continue(); return }
      const url = new URL(route.request().url())
      capturedAccountParam = url.searchParams.get('account')
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { templates: TEMPLATES_ALPHA, total: 2, page: 1, limit: 50 } })
      })
    })
    await page.route('**/api/accounts*', async route => {
      if (route.request().method() !== 'GET') { await route.continue(); return }
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_ACCOUNTS) })
    })
    await page.route('**/api/campaigns*', async route => {
      if (route.request().method() !== 'GET') { await route.continue(); return }
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_CAMPAIGNS) })
    })

    await campaignsPage.goto()
    await campaignsPage.openCreateDialog()
    await campaignsPage.selectAccount('Account Alpha')

    // Wait for templates to load
    await campaignsPage.openTemplateDropdown()
    await expect(campaignsPage.getTemplateOptions().first()).toBeVisible()
    await page.keyboard.press('Escape')

    expect(capturedAccountParam).toBe('Account Alpha')
  })

  test('should fall back to template name when display_name is empty', async ({ page }) => {
    await setupMockRoutes(page)
    await campaignsPage.goto()
    await campaignsPage.openCreateDialog()

    await campaignsPage.selectAccount('Account Beta')
    await campaignsPage.openTemplateDropdown()

    // Beta account template has empty display_name, should show the name field
    await expect(campaignsPage.getTemplateOptions().first()).toContainText('welcome_message')
  })

  test('should show validation error when submitting without template', async ({ page }) => {
    await setupMockRoutes(page)
    await campaignsPage.goto()
    await campaignsPage.openCreateDialog()

    await campaignsPage.fillCampaignName('Test Campaign')
    await campaignsPage.selectAccount('Account Alpha')
    // Skip template selection
    await campaignsPage.createDialog.getByRole('button', { name: /Create Campaign/i }).click()

    await expect(page.locator('[data-sonner-toast]')).toBeVisible({ timeout: 5000 })
  })
})
