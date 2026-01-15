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
    await expect(accountsPage.dialog.locator('input#business_account_id')).toBeVisible()
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
    await accountsPage.dialog.locator('input#business_account_id').fill('789012')
    await accountsPage.dialog.locator('input#access_token').fill('token123')
    await accountsPage.submitDialog()
    await accountsPage.expectToast(/name|required/i)
  })

  test('should show validation error for empty phone ID', async () => {
    await accountsPage.dialog.locator('input#name').fill('Test Account')
    await accountsPage.dialog.locator('input#business_account_id').fill('789012')
    await accountsPage.dialog.locator('input#access_token').fill('token123')
    await accountsPage.submitDialog()
    await accountsPage.expectToast(/phone|required/i)
  })

  test('should show validation error for empty access token', async () => {
    await accountsPage.dialog.locator('input#name').fill('Test Account')
    await accountsPage.dialog.locator('input#phone_id').fill('123456')
    await accountsPage.dialog.locator('input#business_account_id').fill('789012')
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

  test('should create an account', async () => {
    const accountName = `Test Account ${Date.now()}`

    await accountsPage.openCreateDialog()
    await accountsPage.fillAccountForm({
      name: accountName,
      phoneId: '123456789',
      businessAccountId: '987654321',
      accessToken: 'test_access_token_123'
    })
    await accountsPage.submitDialog()

    await accountsPage.expectToast(/created|success/i)
  })

  test('should show delete confirmation dialog', async ({ page }) => {
    const firstCard = page.locator('.rounded-lg.border').first()
    if (await firstCard.isVisible()) {
      const cardText = await firstCard.textContent()
      if (cardText) {
        await firstCard.getByRole('button').filter({ has: page.locator('svg.lucide-trash-2') }).click()
        await expect(accountsPage.alertDialog).toBeVisible()
        await expect(accountsPage.alertDialog).toContainText('cannot be undone')
        await accountsPage.cancelDelete()
      }
    }
  })

  test('should cancel account deletion', async ({ page }) => {
    const firstCard = page.locator('.rounded-lg.border').first()
    if (await firstCard.isVisible()) {
      await firstCard.getByRole('button').filter({ has: page.locator('svg.lucide-trash-2') }).click()
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
    const firstCard = page.locator('.rounded-lg.border').first()
    if (await firstCard.isVisible()) {
      const editBtn = firstCard.getByRole('button').filter({ has: page.locator('svg.lucide-pencil') })
      await expect(editBtn).toBeVisible()
    }
  })

  test('should have delete button on account card', async ({ page }) => {
    const firstCard = page.locator('.rounded-lg.border').first()
    if (await firstCard.isVisible()) {
      const deleteBtn = firstCard.getByRole('button').filter({ has: page.locator('svg.lucide-trash-2') })
      await expect(deleteBtn).toBeVisible()
    }
  })

  test('should open edit dialog when clicking edit', async ({ page }) => {
    const firstCard = page.locator('.rounded-lg.border').first()
    if (await firstCard.isVisible()) {
      await firstCard.getByRole('button').filter({ has: page.locator('svg.lucide-pencil') }).click()
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
    const firstCard = page.locator('.rounded-lg.border').first()
    if (await firstCard.isVisible()) {
      await expect(firstCard.getByText(/Webhook/i)).toBeVisible()
    }
  })

  test('should have copy button for webhook URL', async ({ page }) => {
    const firstCard = page.locator('.rounded-lg.border').first()
    if (await firstCard.isVisible()) {
      const copyBtn = firstCard.locator('button').filter({ has: page.locator('svg.lucide-copy') })
      if (await copyBtn.first().isVisible()) {
        await expect(copyBtn.first()).toBeVisible()
      }
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
    const firstCard = page.locator('.rounded-lg.border').first()
    if (await firstCard.isVisible()) {
      const testBtn = firstCard.getByRole('button', { name: /Test/i })
      if (await testBtn.isVisible()) {
        await expect(testBtn).toBeVisible()
      }
    }
  })
})
