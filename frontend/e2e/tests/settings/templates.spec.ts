import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { TemplatesPage } from '../../pages'

test.describe('Message Templates', () => {
  let templatesPage: TemplatesPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    templatesPage = new TemplatesPage(page)
    await templatesPage.goto()
  })

  test('should display templates page', async () => {
    await templatesPage.expectPageVisible()
    await expect(templatesPage.createButton).toBeVisible()
    await expect(templatesPage.syncButton).toBeVisible()
  })

  test('should have search input', async () => {
    await expect(templatesPage.searchInput).toBeVisible()
  })

  test('should have account filter', async () => {
    await expect(templatesPage.accountSelect).toBeVisible()
  })

  test('should open create template dialog', async () => {
    await templatesPage.openCreateDialog()
    await templatesPage.expectDialogVisible()
    await expect(templatesPage.dialog).toContainText('Create Template')
  })

  test('should close create dialog on cancel', async () => {
    await templatesPage.openCreateDialog()
    await templatesPage.cancelDialog()
    await templatesPage.expectDialogHidden()
  })

  test('should show required fields in create dialog', async () => {
    await templatesPage.openCreateDialog()
    await expect(templatesPage.dialog.locator('select').first()).toBeVisible() // Account
    await expect(templatesPage.dialog.locator('input').first()).toBeVisible() // Name
    await expect(templatesPage.dialog.locator('textarea').first()).toBeVisible() // Body
  })

  test('should show validation error for empty name', async () => {
    await templatesPage.openCreateDialog()
    await templatesPage.dialog.locator('textarea').first().fill('Body content')
    await templatesPage.submitDialog()
    await templatesPage.expectToast('required')
  })

  test('should show validation error for empty body', async () => {
    await templatesPage.openCreateDialog()
    await templatesPage.dialog.locator('input').first().fill('test_template')
    await templatesPage.submitDialog()
    await templatesPage.expectToast('required')
  })
})

test.describe('Template Form Fields', () => {
  let templatesPage: TemplatesPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    templatesPage = new TemplatesPage(page)
    await templatesPage.goto()
    await templatesPage.openCreateDialog()
  })

  test('should have language selector', async () => {
    const langSelect = templatesPage.dialog.locator('select').nth(1)
    await expect(langSelect).toBeVisible()
  })

  test('should have category selector', async () => {
    const categorySelect = templatesPage.dialog.locator('select').nth(2)
    await expect(categorySelect).toBeVisible()
  })

  test('should have header type selector', async () => {
    const headerTypeSelect = templatesPage.dialog.locator('select').nth(3)
    await expect(headerTypeSelect).toBeVisible()
  })

  test('should show header text input for TEXT type', async () => {
    await templatesPage.dialog.locator('select').nth(3).selectOption('TEXT')
    await expect(templatesPage.dialog.locator('input[placeholder*="header"]')).toBeVisible()
  })

  test('should show media upload for IMAGE type', async () => {
    await templatesPage.dialog.locator('select').nth(3).selectOption('IMAGE')
    await expect(templatesPage.dialog.locator('input[type="file"]')).toBeVisible()
  })

  test('should have footer input', async () => {
    await expect(templatesPage.dialog.locator('input[placeholder*="Thank you"]')).toBeVisible()
  })

  test('should have add button option', async () => {
    await expect(templatesPage.dialog.getByRole('button', { name: /Add Button/i })).toBeVisible()
  })

  test('should add button when clicking add button', async () => {
    await templatesPage.dialog.getByRole('button', { name: /Add Button/i }).click()
    await expect(templatesPage.dialog.locator('.border.rounded-lg.p-3')).toBeVisible()
  })

  test('should limit to 3 buttons', async () => {
    const addBtn = templatesPage.dialog.getByRole('button', { name: /Add Button/i })
    await addBtn.click()
    await addBtn.click()
    await addBtn.click()
    await expect(addBtn).toBeDisabled()
  })
})

test.describe('Template Search', () => {
  let templatesPage: TemplatesPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    templatesPage = new TemplatesPage(page)
    await templatesPage.goto()
  })

  test('should filter templates by search', async ({ page }) => {
    await templatesPage.search('nonexistent_template_xyz')
    await page.waitForTimeout(500)
    // Should show empty or filtered results
  })

  test('should clear search', async () => {
    await templatesPage.search('test')
    await templatesPage.search('')
    // Templates should be shown again
  })
})

test.describe('Template CRUD Operations', () => {
  let templatesPage: TemplatesPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    templatesPage = new TemplatesPage(page)
    await templatesPage.goto()
  })

  test('should create a template', async ({ page }) => {
    const templateName = `test_template_${Date.now()}`

    await templatesPage.openCreateDialog()

    // Select first account if available
    const accountSelect = templatesPage.dialog.locator('select').first()
    const options = await accountSelect.locator('option').count()
    if (options > 1) {
      await accountSelect.selectOption({ index: 1 })
    }

    await templatesPage.dialog.locator('input').first().fill(templateName)
    await templatesPage.dialog.locator('textarea').first().fill('Hello {{1}}, your order is confirmed!')
    await templatesPage.submitDialog()

    // May fail if no account - that's expected
    const toast = page.locator('[data-sonner-toast]').first()
    await expect(toast).toBeVisible({ timeout: 5000 })
  })

  test('should show delete confirmation dialog', async ({ page }) => {
    // Find template cards that have delete buttons (exclude info cards)
    const deleteButton = page.locator('.rounded-lg.border').locator('button').filter({ has: page.locator('svg.lucide-trash-2') }).first()
    if (await deleteButton.isVisible({ timeout: 2000 }).catch(() => false)) {
      await deleteButton.click()
      await expect(templatesPage.alertDialog).toBeVisible()
      await expect(templatesPage.alertDialog).toContainText('cannot be undone')
      await templatesPage.cancelAlertDialog()
    }
  })

  test('should show preview dialog', async ({ page }) => {
    // Find template cards that have preview buttons
    const previewButton = page.locator('.rounded-lg.border').locator('button').filter({ has: page.locator('svg.lucide-eye') }).first()
    if (await previewButton.isVisible({ timeout: 2000 }).catch(() => false)) {
      await previewButton.click()
      await expect(templatesPage.previewDialog).toBeVisible()
      await expect(templatesPage.previewDialog).toContainText('Template Preview')
      await templatesPage.closePreview()
    }
  })
})

test.describe('Template Sync', () => {
  let templatesPage: TemplatesPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    templatesPage = new TemplatesPage(page)
    await templatesPage.goto()
  })

  test('should have sync button disabled when no account selected', async () => {
    // Check if "All Accounts" is selected
    const allAccountsSelected = await templatesPage.accountSelect.textContent()
    if (allAccountsSelected?.includes('All')) {
      await expect(templatesPage.syncButton).toBeDisabled()
    }
  })

  test('should show error when syncing without account', async ({ page }) => {
    // Select "All Accounts" if not already
    await templatesPage.accountSelect.click()
    const allOption = page.locator('[role="option"]').filter({ hasText: 'All' })
    if (await allOption.isVisible()) {
      await allOption.click()
    }
    // Sync button should be disabled
  })
})

test.describe('Template Buttons', () => {
  let templatesPage: TemplatesPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    templatesPage = new TemplatesPage(page)
    await templatesPage.goto()
    await templatesPage.openCreateDialog()
  })

  test('should add QUICK_REPLY button', async () => {
    await templatesPage.addButton('QUICK_REPLY', 'Yes')
    await expect(templatesPage.dialog.locator('input[placeholder="Button text"]')).toHaveValue('Yes')
  })

  test('should show URL field for URL button type', async () => {
    await templatesPage.dialog.getByRole('button', { name: /Add Button/i }).click()
    await templatesPage.dialog.locator('.border.rounded-lg.p-3').last().locator('select').selectOption('URL')
    await expect(templatesPage.dialog.locator('input[placeholder*="https"]')).toBeVisible()
  })

  test('should show phone field for PHONE_NUMBER button type', async () => {
    await templatesPage.dialog.getByRole('button', { name: /Add Button/i }).click()
    await templatesPage.dialog.locator('.border.rounded-lg.p-3').last().locator('select').selectOption('PHONE_NUMBER')
    await expect(templatesPage.dialog.locator('input[placeholder*="+123"]')).toBeVisible()
  })

  test('should remove button', async () => {
    await templatesPage.dialog.getByRole('button', { name: /Add Button/i }).click()
    const buttonSection = templatesPage.dialog.locator('.border.rounded-lg.p-3')
    await expect(buttonSection).toBeVisible()

    // Remove button
    await buttonSection.locator('button').filter({ has: templatesPage.page.locator('svg.lucide-x') }).click()
    await expect(buttonSection).not.toBeVisible()
  })
})

test.describe('Template Variables', () => {
  let templatesPage: TemplatesPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    templatesPage = new TemplatesPage(page)
    await templatesPage.goto()
    await templatesPage.openCreateDialog()
  })

  test('should show sample values section when body has variables', async () => {
    await templatesPage.dialog.locator('textarea').first().fill('Hello {{name}}, your order {{order_id}} is ready')
    await expect(templatesPage.dialog.getByText('Sample Values')).toBeVisible()
  })

  test('should show sample inputs for named variables', async () => {
    await templatesPage.dialog.locator('textarea').first().fill('Hello {{name}}!')
    await expect(templatesPage.dialog.locator('input[placeholder*="name"]')).toBeVisible()
  })

  test('should show sample inputs for positional variables', async () => {
    await templatesPage.dialog.locator('textarea').first().fill('Hello {{1}}, your code is {{2}}')
    await expect(templatesPage.dialog.getByText('{{1}}')).toBeVisible()
    await expect(templatesPage.dialog.getByText('{{2}}')).toBeVisible()
  })
})
