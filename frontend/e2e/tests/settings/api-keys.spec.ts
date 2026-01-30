import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { ApiKeysPage } from '../../pages'

test.describe('API Keys Management', () => {
  let apiKeysPage: ApiKeysPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    apiKeysPage = new ApiKeysPage(page)
    await apiKeysPage.goto()
  })

  test('should display API keys page', async () => {
    await apiKeysPage.expectPageVisible()
    await expect(apiKeysPage.addButton).toBeVisible()
  })

  test('should open create API key dialog', async () => {
    await apiKeysPage.openCreateDialog()
    await apiKeysPage.expectDialogVisible()
    await expect(apiKeysPage.dialog).toContainText('Create API Key')
  })

  test('should show validation error for empty name', async () => {
    await apiKeysPage.openCreateDialog()
    await apiKeysPage.submitDialog()
    await apiKeysPage.expectToast('required')
  })

  test('should create a new API key', async () => {
    const keyName = `Test Key ${Date.now()}`

    await apiKeysPage.openCreateDialog()
    await apiKeysPage.fillApiKeyForm(keyName)
    await apiKeysPage.submitDialog()

    // Should show the key display dialog
    await apiKeysPage.expectKeyCreatedDialog()
    await apiKeysPage.closeKeyCreatedDialog()

    // Key should appear in table
    await apiKeysPage.expectRowExists(keyName)
  })

  test('should create API key with expiration', async () => {
    const keyName = `Expiring Key ${Date.now()}`

    // Set expiration to tomorrow
    const tomorrow = new Date()
    tomorrow.setDate(tomorrow.getDate() + 1)
    const dateStr = tomorrow.toISOString().slice(0, 16)

    await apiKeysPage.openCreateDialog()
    await apiKeysPage.fillApiKeyForm(keyName, dateStr)
    await apiKeysPage.submitDialog()

    // Should show the key display dialog
    await apiKeysPage.expectKeyCreatedDialog()
    await apiKeysPage.closeKeyCreatedDialog()

    // Key should appear in table
    await apiKeysPage.expectRowExists(keyName)
  })

  test('should delete API key', async () => {
    // First create a key to delete
    const keyName = `Delete Key ${Date.now()}`

    await apiKeysPage.openCreateDialog()
    await apiKeysPage.fillApiKeyForm(keyName)
    await apiKeysPage.submitDialog()

    // Wait for key created dialog
    await apiKeysPage.expectKeyCreatedDialog()
    await apiKeysPage.closeKeyCreatedDialog()

    // Wait for table to update
    await apiKeysPage.expectRowExists(keyName)

    // Delete the key (API keys have single delete button, index 0)
    await apiKeysPage.clickRowButton(keyName, 0)
    await expect(apiKeysPage.alertDialog).toBeVisible()
    await apiKeysPage.confirmDelete()

    await apiKeysPage.expectToast('deleted')
  })

  test('should cancel API key creation', async () => {
    await apiKeysPage.openCreateDialog()
    await apiKeysPage.fillApiKeyForm('Cancelled Key')
    await apiKeysPage.cancelDialog()
    await apiKeysPage.expectDialogHidden()
  })
})

test.describe('API Keys - Table Sorting', () => {
  let apiKeysPage: ApiKeysPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    apiKeysPage = new ApiKeysPage(page)
    await apiKeysPage.goto()
  })

  test('should sort by name', async () => {
    await apiKeysPage.clickColumnHeader('Name')
    const direction = await apiKeysPage.getSortDirection('Name')
    expect(direction).not.toBeNull()
  })

  test('should sort by last used', async () => {
    await apiKeysPage.clickColumnHeader('Last Used')
    const direction = await apiKeysPage.getSortDirection('Last Used')
    expect(direction).not.toBeNull()
  })

  test('should sort by expires', async () => {
    await apiKeysPage.clickColumnHeader('Expires')
    const direction = await apiKeysPage.getSortDirection('Expires')
    expect(direction).not.toBeNull()
  })

  test('should sort by status', async () => {
    await apiKeysPage.clickColumnHeader('Status')
    const direction = await apiKeysPage.getSortDirection('Status')
    expect(direction).not.toBeNull()
  })

  test('should toggle sort direction', async () => {
    await apiKeysPage.clickColumnHeader('Name')
    const firstDirection = await apiKeysPage.getSortDirection('Name')

    await apiKeysPage.clickColumnHeader('Name')
    const secondDirection = await apiKeysPage.getSortDirection('Name')

    expect(firstDirection).not.toEqual(secondDirection)
  })
})
