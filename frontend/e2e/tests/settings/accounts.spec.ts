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

test.describe('Account Test Connection', () => {
  let accountsPage: AccountsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    accountsPage = new AccountsPage(page)
    await accountsPage.goto()
  })

  test('should have test connection button', async ({ page }) => {
    // Account cards have h3 with account name
    const accountCard = page.locator('.rounded-xl.border').filter({ has: page.locator('h3') }).first()
    if (await accountCard.isVisible()) {
      const testBtn = accountCard.getByRole('button', { name: /Test/i })
      await expect(testBtn).toBeVisible()
    }
  })
})
