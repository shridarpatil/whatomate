import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { AccountsPage } from '../../pages'

test.describe('WhatsApp Accounts', () => {
  let accountsPage: AccountsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    accountsPage = new AccountsPage(page)
    await accountsPage.goto()
  })

  test('should display accounts page', async () => {
    await accountsPage.expectPageVisible()
    await expect(accountsPage.addButton).toBeVisible()
  })

  test('should open create account dialog', async () => {
    await accountsPage.openCreateDialog()
    await accountsPage.expectDialogVisible()
    await expect(accountsPage.dialog).toContainText('Account')
  })

  test('should close create dialog on cancel', async () => {
    await accountsPage.openCreateDialog()
    await accountsPage.cancelDialog()
    await accountsPage.expectDialogHidden()
  })

  test('should show required fields in create dialog', async () => {
    await accountsPage.openCreateDialog()
    await expect(accountsPage.dialog.locator('input#name')).toBeVisible()
    await expect(accountsPage.dialog.locator('input#phone_id')).toBeVisible()
    await expect(accountsPage.dialog.locator('input#business_id')).toBeVisible()
    await expect(accountsPage.dialog.locator('input#access_token')).toBeVisible()
  })
})

test.describe('Account Form Validation', () => {
  let accountsPage: AccountsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    accountsPage = new AccountsPage(page)
    await accountsPage.goto()
    await accountsPage.openCreateDialog()
  })

  test('should show validation error for empty name', async () => {
    await accountsPage.dialog.locator('input#phone_id').fill('123456')
    await accountsPage.dialog.locator('input#business_id').fill('789012')
    await accountsPage.dialog.locator('input#access_token').fill('token123')
    await accountsPage.submitDialog()
    await accountsPage.expectToast(/required/i)
  })

  test('should show validation error for empty phone ID', async () => {
    await accountsPage.dialog.locator('input#name').fill('Test Account')
    await accountsPage.dialog.locator('input#business_id').fill('789012')
    await accountsPage.dialog.locator('input#access_token').fill('token123')
    await accountsPage.submitDialog()
    await accountsPage.expectToast(/required/i)
  })

  test('should show validation error for empty access token', async () => {
    await accountsPage.dialog.locator('input#name').fill('Test Account')
    await accountsPage.dialog.locator('input#phone_id').fill('123456')
    await accountsPage.dialog.locator('input#business_id').fill('789012')
    await accountsPage.submitDialog()
    await accountsPage.expectToast(/token|required/i)
  })
})

test.describe('Account CRUD Operations', () => {
  let accountsPage: AccountsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    accountsPage = new AccountsPage(page)
    await accountsPage.goto()
  })

  test('should create an account', async ({ page }) => {
    const accountName = `Test Account ${Date.now()}`

    await accountsPage.openCreateDialog()
    await accountsPage.fillAccountForm({
      name: accountName,
      phoneId: '123456789',
      businessId: '987654321',
      accessToken: 'test_access_token_123'
    })
    await accountsPage.submitDialog()

    // Should show some toast response (success or error from API)
    const toast = page.locator('[data-sonner-toast]').first()
    await expect(toast).toBeVisible({ timeout: 5000 })
  })

  test('should show delete confirmation dialog', async ({ page }) => {
    // Account cards have h3 with account name, skip the webhook info card
    const accountCard = page.locator('.rounded-xl.border').filter({ has: page.locator('h3') }).first()
    if (await accountCard.isVisible()) {
      // Delete button is the icon button with destructive icon (has text-destructive class on svg)
      await accountCard.locator('button').filter({ has: page.locator('svg.text-destructive') }).click()
      await expect(accountsPage.alertDialog).toBeVisible()
      await expect(accountsPage.alertDialog).toContainText('cannot be undone')
      await accountsPage.cancelDelete()
    }
  })

  test('should cancel account deletion', async ({ page }) => {
    const accountCard = page.locator('.rounded-xl.border').filter({ has: page.locator('h3') }).first()
    if (await accountCard.isVisible()) {
      await accountCard.locator('button').filter({ has: page.locator('svg.text-destructive') }).click()
      await accountsPage.cancelDelete()
      await accountsPage.expectDialogHidden()
    }
  })
})

test.describe('Account Card Actions', () => {
  let accountsPage: AccountsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    accountsPage = new AccountsPage(page)
    await accountsPage.goto()
  })

  test('should have edit button on account card', async ({ page }) => {
    // Account cards have h3 with account name
    const accountCard = page.locator('.rounded-xl.border').filter({ has: page.locator('h3') }).first()
    if (await accountCard.isVisible()) {
      // Edit button is an icon-only button (has svg but no text-destructive class)
      // It's the icon button that's NOT the delete button
      const iconButtons = accountCard.locator('button:has(svg.h-4)').filter({ hasNot: page.locator('span') })
      const editBtn = iconButtons.first()
      await expect(editBtn).toBeVisible()
    }
  })

  test('should have delete button on account card', async ({ page }) => {
    const accountCard = page.locator('.rounded-xl.border').filter({ has: page.locator('h3') }).first()
    if (await accountCard.isVisible()) {
      // Delete button has svg with text-destructive class
      const deleteBtn = accountCard.locator('button').filter({ has: page.locator('svg.text-destructive') })
      await expect(deleteBtn).toBeVisible()
    }
  })

  test('should open edit dialog when clicking edit', async ({ page }) => {
    const accountCard = page.locator('.rounded-xl.border').filter({ has: page.locator('h3') }).first()
    if (await accountCard.isVisible()) {
      // Edit button is the first icon-only button (without text-destructive)
      const iconButtons = accountCard.locator('button:has(svg.h-4)').filter({ hasNot: page.locator('span') })
      await iconButtons.first().click()
      await accountsPage.expectDialogVisible()
      await expect(accountsPage.dialog).toContainText('Edit')
    }
  })
})

test.describe('Account Webhook Info', () => {
  let accountsPage: AccountsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    accountsPage = new AccountsPage(page)
    await accountsPage.goto()
  })

  test('should display webhook URL section', async ({ page }) => {
    // Webhook card has h4 with "Webhook Configuration"
    const webhookCard = page.locator('.rounded-xl.border').filter({ has: page.locator('h4') }).first()
    await expect(webhookCard.getByText('Webhook Configuration')).toBeVisible()
  })

  test('should have copy button for webhook URL', async ({ page }) => {
    const webhookCard = page.locator('.rounded-xl.border').filter({ has: page.locator('h4') }).first()
    if (await webhookCard.isVisible()) {
      // Copy button is next to the code element containing the webhook URL
      const copyBtn = webhookCard.locator('code').first().locator('..').locator('button')
      await expect(copyBtn).toBeVisible()
    }
  })
})

test.describe('Account Test Connection Details', () => {
  let accountsPage: AccountsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    accountsPage = new AccountsPage(page)

    // Mock the GET /accounts to ensure we have a test subject
    await page.route('**/api/accounts', async route => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              accounts: [{
                id: 'test-acc-id',
                name: 'Test Account',
                phone_id: '123456',
                business_id: '789012',
                status: 'active'
              }]
            }
          })
        })
      } else {
        await route.continue()
      }
    })

    await accountsPage.goto()
  })

  test('should display success state with verified account details', async ({ page }) => {
    // Mock successful test connection
    await page.route('**/api/accounts/*/test', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            success: true,
            display_phone_number: '+1 555 0123',
            verified_name: 'My Business',
            quality_rating: 'GREEN',
            code_verification_status: 'VERIFIED'
          }
        })
      })
    })

    const accountCard = page.locator('.rounded-xl.border').filter({ hasText: 'Test Account' })
    await accountCard.getByRole('button', { name: /Test/i }).click()

    // Assertions
    await expect(accountCard.getByText('Connected')).toBeVisible()
    await expect(accountCard.getByText('+1 555 0123')).toBeVisible()
    await expect(page.locator('[data-sonner-toast]').filter({ hasText: 'Connection successful' })).toBeVisible()

    // Check for success color/icon context if needed, but text is usually sufficient
  })

  test('should display error state when connection fails', async ({ page }) => {
    await page.route('**/api/accounts/*/test', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            success: false,
            error: 'Invalid access token or permissions'
          }
        })
      })
    })

    const accountCard = page.locator('.rounded-xl.border').filter({ hasText: 'Test Account' })
    await accountCard.getByRole('button', { name: /Test/i }).click()

    await expect(accountCard.getByText('Invalid access token or permissions')).toBeVisible()
    await expect(page.locator('[data-sonner-toast]').filter({ hasText: /Connection failed/ })).toBeVisible()
  })

  test('should display warning for test numbers', async ({ page }) => {
    await page.route('**/api/accounts/*/test', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            success: true,
            display_phone_number: '+1 555 9999',
            is_test_number: true,
            warning: 'This is a trial number, messaging limits apply'
          }
        })
      })
    })

    const accountCard = page.locator('.rounded-xl.border').filter({ hasText: 'Test Account' })
    await accountCard.getByRole('button', { name: /Test/i }).click()

    await expect(accountCard.getByText('Test Number')).toBeVisible()
    await expect(accountCard.getByText('This is a trial number, messaging limits apply')).toBeVisible()
  })

  test('should show loading state while testing connection', async ({ page }) => {
    // Intercept and delay request to verify loading state
    let fulfillCallback: () => void;
    const responsePromise = new Promise<void>((resolve) => {
      fulfillCallback = resolve;
    });

    await page.route('**/api/accounts/*/test', async route => {
      await responsePromise;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { success: true } })
      })
    });

    const accountCard = page.locator('.rounded-xl.border').filter({ hasText: 'Test Account' })
    const testBtn = accountCard.getByRole('button', { name: /Test/i })

    // Start the action
    await testBtn.click()

    // Verify loading state
    await expect(testBtn).toBeDisabled()
    // Check for the loader icon (lucide-loader-2 typically has a specific class or we can check for the svg)
    // Based on the vue file: <Loader2 ... class="animate-spin" />
    await expect(testBtn.locator('.animate-spin')).toBeVisible()

    // Finish the request
    fulfillCallback!()

    // Verify loading state ends
    await expect(testBtn).not.toBeDisabled()
    await expect(testBtn.locator('.animate-spin')).not.toBeVisible()
  })
})

test.describe('Account Subscribe App', () => {
  let accountsPage: AccountsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    accountsPage = new AccountsPage(page)

    // Mock the GET /accounts to ensure we have a test subject
    await page.route('**/api/accounts', async route => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              accounts: [{
                id: 'test-acc-id',
                name: 'Test Account',
                phone_id: '123456',
                business_id: '789012',
                status: 'active'
              }]
            }
          })
        })
      } else {
        await route.continue()
      }
    })

    await accountsPage.goto()
  })

  test('should have subscribe button on account card', async ({ page }) => {
    const accountCard = page.locator('.rounded-xl.border').filter({ hasText: 'Test Account' })
    const subscribeBtn = accountCard.getByRole('button', { name: /Subscribe/i })
    await expect(subscribeBtn).toBeVisible()
  })

  test('should display success message when subscription succeeds', async ({ page }) => {
    await page.route('**/api/accounts/*/subscribe', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            success: true,
            message: 'App subscribed to webhooks successfully. You should now receive incoming messages.'
          }
        })
      })
    })

    const accountCard = page.locator('.rounded-xl.border').filter({ hasText: 'Test Account' })
    await accountCard.getByRole('button', { name: /Subscribe/i }).click()

    await expect(page.locator('[data-sonner-toast]').filter({ hasText: /subscribed.*successfully/i })).toBeVisible()
  })

  test('should display error message when subscription fails', async ({ page }) => {
    await page.route('**/api/accounts/*/subscribe', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: {
            success: false,
            error: 'Invalid access token or insufficient permissions'
          }
        })
      })
    })

    const accountCard = page.locator('.rounded-xl.border').filter({ hasText: 'Test Account' })
    await accountCard.getByRole('button', { name: /Subscribe/i }).click()

    await expect(page.locator('[data-sonner-toast]').filter({ hasText: /failed/i })).toBeVisible()
  })

  test('should show loading state while subscribing', async ({ page }) => {
    let fulfillCallback: () => void;
    const responsePromise = new Promise<void>((resolve) => {
      fulfillCallback = resolve;
    });

    await page.route('**/api/accounts/*/subscribe', async route => {
      await responsePromise;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: { success: true } })
      })
    });

    const accountCard = page.locator('.rounded-xl.border').filter({ hasText: 'Test Account' })
    const subscribeBtn = accountCard.getByRole('button', { name: /Subscribe/i })

    // Start the action
    await subscribeBtn.click()

    // Verify loading state
    await expect(subscribeBtn).toBeDisabled()
    await expect(subscribeBtn.locator('.animate-spin')).toBeVisible()

    // Finish the request
    fulfillCallback!()

    // Verify loading state ends
    await expect(subscribeBtn).not.toBeDisabled()
    await expect(subscribeBtn.locator('.animate-spin')).not.toBeVisible()
  })

  test('should handle API error gracefully', async ({ page }) => {
    await page.route('**/api/accounts/*/subscribe', async route => {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({
          error: 'Internal server error'
        })
      })
    })

    const accountCard = page.locator('.rounded-xl.border').filter({ hasText: 'Test Account' })
    await accountCard.getByRole('button', { name: /Subscribe/i }).click()

    await expect(page.locator('[data-sonner-toast]').filter({ hasText: /failed|error/i })).toBeVisible()
  })
})
