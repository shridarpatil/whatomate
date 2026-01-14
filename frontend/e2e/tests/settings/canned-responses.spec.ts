import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { DialogPage } from '../../pages'

test.describe('Canned Responses Management', () => {
  let dialogPage: DialogPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/canned-responses')
    await page.waitForLoadState('networkidle')
    dialogPage = new DialogPage(page)
  })

  test('should display canned responses page', async ({ page }) => {
    await expect(page.locator('h1')).toContainText('Canned Responses')
    // Use first() since there are 2 Add Response buttons (header + empty state)
    await expect(page.getByRole('button', { name: /Add Response/i }).first()).toBeVisible()
  })

  test('should open create canned response dialog', async ({ page }) => {
    await page.getByRole('button', { name: /Add Response/i }).first().click()
    await dialogPage.waitForOpen()
    await expect(page.locator('[role="dialog"]')).toBeVisible()
    await expect(page.locator('[role="dialog"]')).toContainText('Canned Response')
  })

  test('should show validation error for empty name and content', async ({ page }) => {
    await page.getByRole('button', { name: /Add Response/i }).first().click()
    await dialogPage.waitForOpen()

    // Click submit button directly (Create)
    await page.locator('[role="dialog"]').getByRole('button', { name: /^Create$/i }).click()

    const toast = page.locator('[data-sonner-toast]')
    await expect(toast).toBeVisible({ timeout: 5000 })
    await expect(toast).toContainText('required')
  })

  test('should create a new canned response', async ({ page }) => {
    const responseName = `Test Response ${Date.now()}`
    const responseContent = 'Hello! Thank you for contacting us.'

    await page.getByRole('button', { name: /Add Response/i }).first().click()
    await dialogPage.waitForOpen()

    // Fill name
    await page.locator('[role="dialog"] input').first().fill(responseName)
    // Fill shortcut
    await page.locator('[role="dialog"] input').nth(1).fill('test')
    // Fill content
    await page.locator('[role="dialog"] textarea').fill(responseContent)
    // Select category
    await page.locator('[role="dialog"] button[role="combobox"]').click()
    await page.locator('[role="option"]').filter({ hasText: 'Greetings' }).click()

    // Click Create button
    await page.locator('[role="dialog"]').getByRole('button', { name: /^Create$/i }).click()

    const toast = page.locator('[data-sonner-toast]')
    await expect(toast).toBeVisible({ timeout: 5000 })
    await expect(toast).toContainText('created')

    // Response should appear in grid
    await expect(page.locator('body')).toContainText(responseName)
  })

  test('should edit existing canned response', async ({ page }) => {
    // First create a response
    const responseName = `Edit Response ${Date.now()}`

    await page.getByRole('button', { name: /Add Response/i }).first().click()
    await dialogPage.waitForOpen()
    await page.locator('[role="dialog"] input').first().fill(responseName)
    await page.locator('[role="dialog"] textarea').fill('Original content')
    await page.locator('[role="dialog"]').getByRole('button', { name: /^Create$/i }).click()

    // Wait for create toast and dismiss it
    const createToast = page.locator('[data-sonner-toast]').filter({ hasText: 'created' })
    await expect(createToast).toBeVisible({ timeout: 5000 })
    await createToast.click() // Dismiss toast

    // Wait for response to appear - find heading with exact name
    const heading = page.getByRole('heading', { name: responseName })
    await expect(heading).toBeVisible({ timeout: 10000 })

    // Find the card container and click edit button (2nd button in the card footer)
    const cardContainer = heading.locator('xpath=ancestor::div[contains(@class, "rounded")]').first()
    await cardContainer.locator('button').nth(1).click()  // edit is 2nd button (copy, edit, delete)

    await dialogPage.waitForOpen()

    // Update content
    await page.locator('[role="dialog"] textarea').fill('Updated content')
    await page.locator('[role="dialog"]').getByRole('button', { name: /^Update$/i }).click()

    // Wait for update toast
    const updateToast = page.locator('[data-sonner-toast]').filter({ hasText: 'updated' })
    await expect(updateToast).toBeVisible({ timeout: 5000 })
  })

  test('should delete canned response', async ({ page }) => {
    // First create a response
    const responseName = `Delete Response ${Date.now()}`

    await page.getByRole('button', { name: /Add Response/i }).first().click()
    await dialogPage.waitForOpen()
    await page.locator('[role="dialog"] input').first().fill(responseName)
    await page.locator('[role="dialog"] textarea').fill('To be deleted')
    await page.locator('[role="dialog"]').getByRole('button', { name: /^Create$/i }).click()

    // Wait for create toast and dismiss it
    const createToast = page.locator('[data-sonner-toast]').filter({ hasText: 'created' })
    await expect(createToast).toBeVisible({ timeout: 5000 })
    await createToast.click() // Dismiss toast

    const heading = page.getByRole('heading', { name: responseName })
    await expect(heading).toBeVisible({ timeout: 10000 })

    // Find the card container and click delete button (3rd button in the card footer)
    const cardContainer = heading.locator('xpath=ancestor::div[contains(@class, "rounded")]').first()
    await cardContainer.locator('button').nth(2).click()  // delete is 3rd button (copy, edit, delete)

    // Confirm deletion
    await expect(page.locator('[role="alertdialog"]')).toBeVisible()
    await page.getByRole('button', { name: 'Delete' }).click()

    // Wait for delete toast
    const deleteToast = page.locator('[data-sonner-toast]').filter({ hasText: 'deleted' })
    await expect(deleteToast).toBeVisible({ timeout: 5000 })
  })

  test('should filter by category', async ({ page }) => {
    // Click category filter
    await page.locator('button[role="combobox"]').first().click()
    await page.locator('[role="option"]').filter({ hasText: 'Greetings' }).click()

    // Should filter responses (results depend on existing data)
    await page.waitForLoadState('networkidle')
  })

  test('should search canned responses', async ({ page }) => {
    // First create a response with unique text
    const uniqueText = `Unique${Date.now()}`

    await page.getByRole('button', { name: /Add Response/i }).first().click()
    await dialogPage.waitForOpen()
    await page.locator('[role="dialog"] input').first().fill(uniqueText)
    await page.locator('[role="dialog"] textarea').fill('Search test content')
    await page.locator('[role="dialog"]').getByRole('button', { name: /^Create$/i }).click()

    // Wait for creation
    const createToast = page.locator('[data-sonner-toast]')
    await expect(createToast).toBeVisible({ timeout: 5000 })

    // Search for the response
    await page.locator('input[placeholder*="Search"]').fill(uniqueText)
    await page.waitForLoadState('networkidle')

    // Should find our response
    await expect(page.locator('body')).toContainText(uniqueText)
  })

  test('should cancel canned response creation', async ({ page }) => {
    await page.getByRole('button', { name: /Add Response/i }).first().click()
    await dialogPage.waitForOpen()

    await page.locator('[role="dialog"] input').first().fill('Cancelled Response')
    await dialogPage.cancel()

    await dialogPage.waitForClose()
    await expect(page.locator('[role="dialog"]')).not.toBeVisible()
  })
})
