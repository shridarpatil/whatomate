import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { CampaignsPage } from '../../pages'

const MOCK_ACCOUNTS = {
  data: {
    accounts: [
      { id: 'acc-1', name: 'Test WhatsApp Account', phone_id: '111', business_id: '222', status: 'active' }
    ]
  }
}

const MOCK_TEMPLATES = {
  data: {
    templates: [
      { id: 'tpl-1', name: 'order_confirmation', display_name: 'Order Confirmation', status: 'APPROVED', language: 'en' },
      { id: 'tpl-2', name: 'shipping_update', display_name: 'Shipping Update', status: 'APPROVED', language: 'en' },
      { id: 'tpl-3', name: 'welcome_message', display_name: '', status: 'APPROVED', language: 'en' }
    ],
    total: 3,
    page: 1,
    limit: 50
  }
}

const MOCK_CAMPAIGNS = {
  data: {
    campaigns: [],
    total: 0,
    page: 1,
    limit: 50
  }
}

function setupMockRoutes(page: import('@playwright/test').Page, options?: { templates?: typeof MOCK_TEMPLATES }) {
  const templates = options?.templates ?? MOCK_TEMPLATES
  return Promise.all([
    page.route('**/api/templates*', async route => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(templates)
        })
      } else {
        await route.continue()
      }
    }),
    page.route('**/api/accounts*', async route => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_ACCOUNTS)
        })
      } else {
        await route.continue()
      }
    }),
    page.route('**/api/campaigns*', async route => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_CAMPAIGNS)
        })
      } else {
        await route.continue()
      }
    })
  ])
}

test.describe('Campaign Create - Template Loading', () => {
  let campaignsPage: CampaignsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    campaignsPage = new CampaignsPage(page)
  })

  test('should load and display templates in create dialog dropdown', async ({ page }) => {
    await setupMockRoutes(page)
    await campaignsPage.goto()
    await campaignsPage.openCreateDialog()

    await campaignsPage.openTemplateDropdown()

    const options = campaignsPage.getTemplateOptions()
    await expect(options).toHaveCount(3)
    await expect(options.nth(0)).toContainText('Order Confirmation')
    await expect(options.nth(1)).toContainText('Shipping Update')
  })

  test('should fall back to template name when display_name is empty', async ({ page }) => {
    await setupMockRoutes(page)
    await campaignsPage.goto()
    await campaignsPage.openCreateDialog()

    await campaignsPage.openTemplateDropdown()

    // Third template has empty display_name, should show the name field
    const options = campaignsPage.getTemplateOptions()
    await expect(options.nth(2)).toContainText('welcome_message')
  })

  test('should allow selecting a template from the dropdown', async ({ page }) => {
    await setupMockRoutes(page)
    await campaignsPage.goto()
    await campaignsPage.openCreateDialog()

    await campaignsPage.selectTemplate('Order Confirmation')

    const trigger = campaignsPage.getTemplateSelectTrigger()
    await expect(trigger).toContainText('Order Confirmation')
  })

  test('should show no templates message when API returns empty list', async ({ page }) => {
    const emptyTemplates = {
      data: {
        templates: [],
        total: 0,
        page: 1,
        limit: 50
      }
    }
    await setupMockRoutes(page, { templates: emptyTemplates })
    await campaignsPage.goto()
    await campaignsPage.openCreateDialog()

    await expect(campaignsPage.getNoTemplatesMessage()).toBeVisible()
  })

  test('should correctly parse envelope-wrapped template response', async ({ page }) => {
    // This specifically tests the fix: the API wraps data in { data: { templates: [...] } }
    // and the frontend must unwrap it correctly
    let interceptedTemplateRequest = false
    await page.route('**/api/templates*', async route => {
      if (route.request().method() === 'GET') {
        interceptedTemplateRequest = true
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              templates: [
                { id: 'tpl-envelope', name: 'envelope_test', display_name: 'Envelope Test Template', status: 'APPROVED', language: 'en' }
              ],
              total: 1,
              page: 1,
              limit: 50
            }
          })
        })
      } else {
        await route.continue()
      }
    })
    await page.route('**/api/accounts*', async route => {
      if (route.request().method() === 'GET') {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_ACCOUNTS) })
      } else {
        await route.continue()
      }
    })
    await page.route('**/api/campaigns*', async route => {
      if (route.request().method() === 'GET') {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_CAMPAIGNS) })
      } else {
        await route.continue()
      }
    })

    await campaignsPage.goto()
    await campaignsPage.openCreateDialog()

    // Verify API was called
    expect(interceptedTemplateRequest).toBe(true)

    // Verify template appears — this would have failed before the fix
    await campaignsPage.openTemplateDropdown()
    const options = campaignsPage.getTemplateOptions()
    await expect(options).toHaveCount(1)
    await expect(options.first()).toContainText('Envelope Test Template')
  })

  test('should show validation error when submitting without template', async ({ page }) => {
    await setupMockRoutes(page)
    await campaignsPage.goto()
    await campaignsPage.openCreateDialog()

    await campaignsPage.fillCampaignName('Test Campaign')
    // Select account but skip template
    await campaignsPage.selectAccount('Test WhatsApp Account')
    await campaignsPage.createDialog.getByRole('button', { name: /Create Campaign/i }).click()

    // Should show validation toast for missing template
    await expect(page.locator('[data-sonner-toast]')).toBeVisible({ timeout: 5000 })
  })
})
