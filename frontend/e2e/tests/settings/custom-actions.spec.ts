import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { DialogPage } from '../../pages'

test.describe('Custom Actions Management', () => {
  let dialogPage: DialogPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/custom-actions')
    await page.waitForLoadState('networkidle')
    dialogPage = new DialogPage(page)
  })

  test('should display custom actions page', async ({ page }) => {
    await expect(page.locator('text=Custom Actions').first()).toBeVisible()
    await expect(page.getByRole('button', { name: /Add Action/i })).toBeVisible()
  })

  test('should open create custom action dialog', async ({ page }) => {
    await page.getByRole('button', { name: /Add Action/i }).click()
    await dialogPage.waitForOpen()
    await expect(page.locator('[role="dialog"]')).toBeVisible()
    await expect(page.locator('[role="dialog"]')).toContainText('Custom Action')
  })

  test('should show validation error for empty name', async ({ page }) => {
    await page.getByRole('button', { name: /Add Action/i }).click()
    await dialogPage.waitForOpen()

    // Try to submit without filling anything - click Create button
    await page.locator('[role="dialog"]').getByRole('button', { name: /^Create$/i }).click()

    const toast = page.locator('[data-sonner-toast]')
    await expect(toast).toBeVisible({ timeout: 5000 })
    await expect(toast).toContainText('required')
  })

  test('should create a webhook custom action', async ({ page }) => {
    const actionName = `Webhook Action ${Date.now()}`

    await page.getByRole('button', { name: /Add Action/i }).click()
    await dialogPage.waitForOpen()

    // Fill name
    await page.locator('[role="dialog"] input#name').fill(actionName)

    // Webhook type is default, fill webhook URL
    await page.locator('[role="dialog"] input#url').fill('https://api.example.com/webhook')

    // Click Create button
    await page.locator('[role="dialog"]').getByRole('button', { name: /^Create$/i }).click()

    const toast = page.locator('[data-sonner-toast]')
    await expect(toast).toBeVisible({ timeout: 5000 })
    await expect(toast).toContainText('created')

    // Action should appear in table
    await expect(page.locator('table')).toContainText(actionName)
  })

  test('should create a URL custom action', async ({ page }) => {
    const actionName = `URL Action ${Date.now()}`

    await page.getByRole('button', { name: /Add Action/i }).click()
    await dialogPage.waitForOpen()

    // Fill name
    await page.locator('[role="dialog"] input#name').fill(actionName)

    // Select URL type
    await page.locator('[role="dialog"] input#type-url').check()

    // Fill URL
    await page.locator('[role="dialog"] input#url').fill('https://crm.example.com/contact')

    // Click Create button
    await page.locator('[role="dialog"]').getByRole('button', { name: /^Create$/i }).click()

    const toast = page.locator('[data-sonner-toast]')
    await expect(toast).toBeVisible({ timeout: 5000 })
    await expect(toast).toContainText('created')

    // Action should appear in table with URL badge
    await expect(page.locator('table')).toContainText(actionName)
  })

  test('should create a JavaScript custom action', async ({ page }) => {
    const actionName = `JS Action ${Date.now()}`

    await page.getByRole('button', { name: /Add Action/i }).click()
    await dialogPage.waitForOpen()

    // Fill name
    await page.locator('[role="dialog"] input#name').fill(actionName)

    // Select JavaScript type
    await page.locator('[role="dialog"] input#type-javascript').check()

    // Fill code
    await page.locator('[role="dialog"] textarea#code').fill('return { clipboard: contact.phone_number }')

    // Click Create button
    await page.locator('[role="dialog"]').getByRole('button', { name: /^Create$/i }).click()

    const toast = page.locator('[data-sonner-toast]')
    await expect(toast).toBeVisible({ timeout: 5000 })
    await expect(toast).toContainText('created')

    // Action should appear in table with JavaScript badge
    await expect(page.locator('table')).toContainText(actionName)
  })

  test('should edit existing custom action', async ({ page }) => {
    // First create an action
    const actionName = `Edit Action ${Date.now()}`

    await page.getByRole('button', { name: /Add Action/i }).click()
    await dialogPage.waitForOpen()
    await page.locator('[role="dialog"] input#name').fill(actionName)
    await page.locator('[role="dialog"] input#type-url').check()
    await page.locator('[role="dialog"] input#url').fill('https://example.com')
    await page.locator('[role="dialog"]').getByRole('button', { name: /^Create$/i }).click()

    // Wait for toast
    const createToast = page.locator('[data-sonner-toast]')
    await expect(createToast).toBeVisible({ timeout: 5000 })

    // Wait for action to appear
    await expect(page.locator('table')).toContainText(actionName)

    // Find the row with our action and click edit
    const row = page.locator('tr').filter({ hasText: actionName })
    await row.locator('button').filter({ has: page.locator('svg.lucide-pencil') }).click()

    await dialogPage.waitForOpen()

    // Update URL
    await page.locator('[role="dialog"] input#url').fill('https://updated.example.com')
    await page.locator('[role="dialog"]').getByRole('button', { name: /^Update$/i }).click()

    const toast = page.locator('[data-sonner-toast]')
    await expect(toast).toBeVisible({ timeout: 5000 })
    await expect(toast).toContainText('updated')
  })

  test('should delete custom action', async ({ page }) => {
    // First create an action
    const actionName = `Delete Action ${Date.now()}`

    await page.getByRole('button', { name: /Add Action/i }).click()
    await dialogPage.waitForOpen()
    await page.locator('[role="dialog"] input#name').fill(actionName)
    await page.locator('[role="dialog"] input#type-url').check()
    await page.locator('[role="dialog"] input#url').fill('https://todelete.com')
    await page.locator('[role="dialog"]').getByRole('button', { name: /^Create$/i }).click()

    // Wait for toast
    const createToast = page.locator('[data-sonner-toast]')
    await expect(createToast).toBeVisible({ timeout: 5000 })

    // Wait for action to appear
    await expect(page.locator('table')).toContainText(actionName)

    // Find the row with our action and click delete
    const row = page.locator('tr').filter({ hasText: actionName })
    await row.locator('button').filter({ has: page.locator('svg.lucide-trash-2') }).click()

    // Confirm deletion
    await expect(page.locator('[role="alertdialog"]')).toBeVisible()
    await page.getByRole('button', { name: 'Delete' }).click()

    const toast = page.locator('[data-sonner-toast]')
    await expect(toast).toBeVisible({ timeout: 5000 })
    await expect(toast).toContainText('deleted')
  })

  test('should toggle custom action status', async ({ page }) => {
    // First create an action
    const actionName = `Toggle Action ${Date.now()}`

    await page.getByRole('button', { name: /Add Action/i }).click()
    await dialogPage.waitForOpen()
    await page.locator('[role="dialog"] input#name').fill(actionName)
    await page.locator('[role="dialog"] input#type-url').check()
    await page.locator('[role="dialog"] input#url').fill('https://toggle.com')
    await page.locator('[role="dialog"]').getByRole('button', { name: /^Create$/i }).click()

    // Wait for toast
    const createToast = page.locator('[data-sonner-toast]')
    await expect(createToast).toBeVisible({ timeout: 5000 })

    // Wait for action to appear
    await expect(page.locator('table')).toContainText(actionName)

    // Find the row with our action and toggle the switch
    const row = page.locator('tr').filter({ hasText: actionName })
    await row.locator('button[role="switch"]').click()

    const toast = page.locator('[data-sonner-toast]')
    await expect(toast).toBeVisible({ timeout: 5000 })
    // Should show disabled or enabled message
  })

  test('should cancel custom action creation', async ({ page }) => {
    await page.getByRole('button', { name: /Add Action/i }).click()
    await dialogPage.waitForOpen()

    await page.locator('[role="dialog"] input#name').fill('Cancelled Action')
    await dialogPage.cancel()

    await dialogPage.waitForClose()
    await expect(page.locator('[role="dialog"]')).not.toBeVisible()
  })
})
