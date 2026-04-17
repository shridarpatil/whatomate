import { test, expect, type Page, type Locator } from '@playwright/test'
import { TablePage } from '../../pages'
import { loginAsAdmin } from '../../helpers'

function nameInput(page: Page): Locator {
  return page.getByPlaceholder('e.g., Production Integration')
}

function saveButton(page: Page): Locator {
  return page.getByRole('button', { name: /^(Create|Saving)$/i }).first()
}

async function gotoCreateApiKey(page: Page) {
  await page.getByRole('button', { name: /Create API Key/i }).first().click()
  await page.waitForURL(/\/settings\/api-keys\/new$/)
  await page.waitForLoadState('networkidle')
}

test.describe('API Keys Management', () => {
  let tablePage: TablePage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/api-keys')
    await page.waitForLoadState('networkidle')
    tablePage = new TablePage(page)
  })

  test('should display API keys page', async ({ page }) => {
    await expect(tablePage.tableBody).toBeVisible()
  })

  test('should navigate to create API key page', async ({ page }) => {
    await gotoCreateApiKey(page)
    expect(page.url()).toContain('/settings/api-keys/new')
  })

  test('should create a new API key', async ({ page }) => {
    const keyName = `Test Key ${Date.now()}`

    await gotoCreateApiKey(page)
    await nameInput(page).fill(keyName)
    await saveButton(page).click()

    // Should show the key display dialog with the full key
    const keyDialog = page.locator('[role="dialog"]')
    await expect(keyDialog).toBeVisible({ timeout: 10000 })
    await expect(keyDialog.locator('text=whm_')).toBeVisible()

    // Close the dialog — navigates to the detail page
    await page.getByRole('button', { name: 'Done' }).click()
    await page.waitForURL(/\/settings\/api-keys\/[a-f0-9-]+$/)
    await page.waitForLoadState('networkidle')

    // Verify key name on detail page
    await expect(page.getByRole('heading', { name: keyName })).toBeVisible()
  })

  test('should create API key with expiration', async ({ page }) => {
    const keyName = `Expiring Key ${Date.now()}`
    const tomorrow = new Date()
    tomorrow.setDate(tomorrow.getDate() + 1)
    const dateStr = tomorrow.toISOString().slice(0, 16)

    await gotoCreateApiKey(page)
    await nameInput(page).fill(keyName)
    await page.locator('input[type="datetime-local"]').fill(dateStr)
    await saveButton(page).click()

    const keyDialog = page.locator('[role="dialog"]')
    await expect(keyDialog).toBeVisible({ timeout: 10000 })
    await page.getByRole('button', { name: 'Done' }).click()
    await page.waitForURL(/\/settings\/api-keys\/[a-f0-9-]+$/)

    await expect(page.getByRole('heading', { name: keyName })).toBeVisible()
  })

  test('should navigate to API key detail view', async ({ page }) => {
    const keyName = `Detail Key ${Date.now()}`

    await gotoCreateApiKey(page)
    await nameInput(page).fill(keyName)
    await saveButton(page).click()
    const keyDialog = page.locator('[role="dialog"]')
    await expect(keyDialog).toBeVisible({ timeout: 10000 })
    await page.getByRole('button', { name: 'Done' }).click()

    // Navigate back to list
    await page.goto('/settings/api-keys')
    await page.waitForLoadState('networkidle')

    // Click the name to go to detail
    await page.locator('tbody tr .font-medium').getByText(keyName, { exact: true }).first().click()
    await page.waitForURL(/\/settings\/api-keys\/[a-f0-9-]+$/)
    await page.waitForLoadState('networkidle')

    await expect(page.getByText(keyName)).toBeVisible()
    await expect(page.locator('code').filter({ hasText: 'whm_' }).first()).toBeVisible()
  })

  test('should delete API key from list', async ({ page }) => {
    const keyName = `Delete Key ${Date.now()}`

    await gotoCreateApiKey(page)
    await nameInput(page).fill(keyName)
    await saveButton(page).click()
    const keyDialog = page.locator('[role="dialog"]')
    await expect(keyDialog).toBeVisible({ timeout: 10000 })
    await page.getByRole('button', { name: 'Done' }).click()

    // Go to list and delete
    await page.goto('/settings/api-keys')
    await page.waitForLoadState('networkidle')
    await tablePage.search(keyName)
    await tablePage.expectRowExists(keyName)

    // Click the delete (last) button on the row
    const row = await tablePage.getRow(keyName)
    await row.locator('td:last-child button').last().click()
    await expect(page.locator('[role="alertdialog"]')).toBeVisible()
    await page.locator('[role="alertdialog"]').getByRole('button', { name: /delete|confirm/i }).click()

    await page.waitForTimeout(1000)
  })

  test('should cancel API key creation via back', async ({ page }) => {
    await gotoCreateApiKey(page)
    await nameInput(page).fill('Cancelled Key')
    // Go back without saving
    await page.goto('/settings/api-keys')
    await page.waitForLoadState('networkidle')
    // Key should not exist
    await tablePage.search('Cancelled Key')
    await tablePage.expectRowNotExists('Cancelled Key')
  })
})

test.describe('API Keys - Table Sorting', () => {
  let tablePage: TablePage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/api-keys')
    await page.waitForLoadState('networkidle')
    tablePage = new TablePage(page)
  })

  test('should sort by name', async () => {
    await tablePage.clickColumnHeader('Name')
    const direction = await tablePage.getSortDirection('Name')
    expect(direction).not.toBeNull()
  })

  test('should sort by last used', async () => {
    await tablePage.clickColumnHeader('Last Used')
    const direction = await tablePage.getSortDirection('Last Used')
    expect(direction).not.toBeNull()
  })

  test('should sort by expires', async () => {
    await tablePage.clickColumnHeader('Expires')
    const direction = await tablePage.getSortDirection('Expires')
    expect(direction).not.toBeNull()
  })

  test('should sort by status', async () => {
    await tablePage.clickColumnHeader('Status')
    const direction = await tablePage.getSortDirection('Status')
    expect(direction).not.toBeNull()
  })

  test('should toggle sort direction', async () => {
    await tablePage.clickColumnHeader('Name')
    const firstDirection = await tablePage.getSortDirection('Name')

    await tablePage.clickColumnHeader('Name')
    const secondDirection = await tablePage.getSortDirection('Name')

    expect(firstDirection).not.toEqual(secondDirection)
  })
})
