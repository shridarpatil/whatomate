import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { DialogPage, TablePage } from '../../pages'

test.describe('API Keys Management', () => {
  let dialogPage: DialogPage
  let tablePage: TablePage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/api-keys')
    await page.waitForLoadState('networkidle')
    dialogPage = new DialogPage(page)
    tablePage = new TablePage(page)
  })

  test('should display API keys page', async ({ page }) => {
    await expect(page.locator('h1')).toContainText('API Keys')
    await expect(page.getByRole('button', { name: /Create API Key/i })).toBeVisible()
  })

  test('should open create API key dialog', async ({ page }) => {
    await page.getByRole('button', { name: /Create API Key/i }).click()
    await dialogPage.waitForOpen()
    await expect(page.locator('[role="dialog"]')).toBeVisible()
    await expect(page.locator('[role="dialog"]')).toContainText('Create API Key')
  })

  test('should show validation error for empty name', async ({ page }) => {
    await page.getByRole('button', { name: /Create API Key/i }).click()
    await dialogPage.waitForOpen()

    // Try to submit without name - button text is "Create Key"
    await page.locator('[role="dialog"]').getByRole('button', { name: /Create Key/i }).click()

    const toast = page.locator('[data-sonner-toast]')
    await expect(toast).toBeVisible({ timeout: 5000 })
    await expect(toast).toContainText('required')
  })

  test('should create a new API key', async ({ page }) => {
    const keyName = `Test Key ${Date.now()}`

    await page.getByRole('button', { name: /Create API Key/i }).click()
    await dialogPage.waitForOpen()

    await page.locator('input#name').fill(keyName)
    // Button text is "Create Key"
    await page.locator('[role="dialog"][data-state="open"]').getByRole('button', { name: /Create Key/i }).click()

    // Should show the key display dialog - use data-state="open" to get the visible one
    await expect(page.locator('[role="dialog"][data-state="open"]')).toContainText('API Key Created')
    await expect(page.locator('[role="dialog"][data-state="open"]')).toContainText('whm_')

    // Close the dialog
    await page.getByRole('button', { name: 'Done' }).click()

    // Key should appear in table
    await expect(page.locator('table')).toContainText(keyName)
  })

  test('should create API key with expiration', async ({ page }) => {
    const keyName = `Expiring Key ${Date.now()}`

    await page.getByRole('button', { name: /Create API Key/i }).click()
    await dialogPage.waitForOpen()

    await page.locator('input#name').fill(keyName)
    // Set expiration to tomorrow
    const tomorrow = new Date()
    tomorrow.setDate(tomorrow.getDate() + 1)
    const dateStr = tomorrow.toISOString().slice(0, 16)
    await page.locator('input#expiry').fill(dateStr)

    // Button text is "Create Key"
    await page.locator('[role="dialog"][data-state="open"]').getByRole('button', { name: /Create Key/i }).click()

    // Should show the key display dialog - use data-state="open" to get the visible one
    await expect(page.locator('[role="dialog"][data-state="open"]')).toContainText('API Key Created')
    await page.getByRole('button', { name: 'Done' }).click()

    // Key should appear in table with expiration
    await expect(page.locator('table')).toContainText(keyName)
  })

  test('should delete API key', async ({ page }) => {
    // First create a key to delete
    const keyName = `Delete Key ${Date.now()}`

    await page.getByRole('button', { name: /Create API Key/i }).click()
    await dialogPage.waitForOpen()
    await page.locator('input#name').fill(keyName)
    await page.locator('[role="dialog"][data-state="open"]').getByRole('button', { name: /Create Key/i }).click()

    // Wait for key created dialog - use data-state="open"
    await expect(page.locator('[role="dialog"][data-state="open"]')).toContainText('API Key Created')
    await page.getByRole('button', { name: 'Done' }).click()

    // Wait for table to update
    await expect(page.locator('table')).toContainText(keyName)

    // Find the row with our key and click delete (last button in the row's Actions cell)
    const row = page.locator('tr').filter({ hasText: keyName })
    await row.locator('td').last().locator('button').click()

    // Confirm deletion
    await expect(page.locator('[role="alertdialog"]')).toBeVisible()
    await page.getByRole('button', { name: 'Delete' }).click()

    // Key should be removed - filter for delete toast
    const deleteToast = page.locator('[data-sonner-toast]').filter({ hasText: 'deleted' })
    await expect(deleteToast).toBeVisible({ timeout: 5000 })
  })

  test('should cancel API key creation', async ({ page }) => {
    await page.getByRole('button', { name: /Create API Key/i }).click()
    await dialogPage.waitForOpen()

    await page.locator('input#name').fill('Cancelled Key')
    await dialogPage.cancel()

    await dialogPage.waitForClose()
    await expect(page.locator('[role="dialog"]')).not.toBeVisible()
  })
})
