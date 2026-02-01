import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { TagsPage } from '../../pages'

test.describe('Tags Management', () => {
  let tagsPage: TagsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    tagsPage = new TagsPage(page)
    await tagsPage.goto()
  })

  test('should display tags page', async () => {
    await tagsPage.expectPageVisible()
    await expect(tagsPage.addButton).toBeVisible()
  })

  test('should open create tag dialog', async () => {
    await tagsPage.openCreateDialog()
    await tagsPage.expectDialogVisible()
    await expect(tagsPage.dialog).toContainText('Create Tag')
  })

  test('should show validation error for empty name', async () => {
    await tagsPage.openCreateDialog()
    await tagsPage.submitDialog()
    await tagsPage.expectToast('required')
  })

  test('should create a new tag', async ({ page }) => {
    const tagName = `Test Tag ${Date.now()}`

    await tagsPage.openCreateDialog()
    await tagsPage.fillTagForm(tagName, 'Blue')
    await tagsPage.submitDialog()

    await tagsPage.expectToast('created')
    await tagsPage.expectTagBadgeVisible(tagName)
  })

  test('should create a tag with different color', async ({ page }) => {
    const tagName = `Purple Tag ${Date.now()}`

    await tagsPage.openCreateDialog()
    await tagsPage.fillTagForm(tagName, 'Purple')
    await tagsPage.submitDialog()

    await tagsPage.expectToast('created')
    await tagsPage.expectTagBadgeVisible(tagName)
  })

  test('should edit existing tag', async ({ page }) => {
    // First create a tag
    const tagName = `Edit Tag ${Date.now()}`

    await tagsPage.openCreateDialog()
    await tagsPage.fillTagForm(tagName, 'Green')
    await tagsPage.submitDialog()

    // Wait for create toast and dismiss it
    await tagsPage.expectToast('created')
    await tagsPage.dismissToast('created')

    // Wait for tag to appear
    await tagsPage.expectTagBadgeVisible(tagName)

    // Edit the tag
    await tagsPage.editRow(tagName)
    await tagsPage.selectColor('Red')
    await tagsPage.submitDialog('Update')

    await tagsPage.expectToast('updated')
  })

  test('should delete tag', async ({ page }) => {
    // First create a tag
    const tagName = `Delete Tag ${Date.now()}`

    await tagsPage.openCreateDialog()
    await tagsPage.fillTagForm(tagName, 'Gray')
    await tagsPage.submitDialog()

    // Wait for create toast and dismiss it
    await tagsPage.expectToast('created')
    await tagsPage.dismissToast('created')

    // Wait for tag to appear
    await tagsPage.expectTagBadgeVisible(tagName)

    // Delete the tag
    await tagsPage.deleteRow(tagName)
    await tagsPage.confirmDelete()

    await tagsPage.expectToast('deleted')
  })

  test('should search tags', async ({ page }) => {
    // First create a tag with unique name
    const uniqueName = `Unique${Date.now()}`

    await tagsPage.openCreateDialog()
    await tagsPage.fillTagForm(uniqueName, 'Yellow')
    await tagsPage.submitDialog()

    // Wait for creation
    await tagsPage.expectToast('created')
    await tagsPage.dismissToast('created')

    // Search for the tag
    await page.locator('input[placeholder*="Search"]').fill(uniqueName)
    await page.waitForTimeout(500)
    await tagsPage.expectTagBadgeVisible(uniqueName)
  })

  test('should prevent duplicate tag names', async ({ page }) => {
    const tagName = `Duplicate ${Date.now()}`

    // Create first tag
    await tagsPage.openCreateDialog()
    await tagsPage.fillTagForm(tagName)
    await tagsPage.submitDialog()
    await tagsPage.expectToast('created')

    // Wait for toast to disappear before clicking Add Tag again
    await page.locator('[data-sonner-toast]').waitFor({ state: 'hidden', timeout: 10000 })

    // Try to create duplicate
    await tagsPage.openCreateDialog()
    await tagsPage.fillTagForm(tagName)
    await tagsPage.submitDialog()

    await tagsPage.expectToast(/already exists/i)
  })

  test('should cancel tag creation', async () => {
    await tagsPage.openCreateDialog()
    await tagsPage.dialog.locator('input').first().fill('Cancelled Tag')
    await tagsPage.cancelDialog()
    await tagsPage.expectDialogHidden()
  })
})
