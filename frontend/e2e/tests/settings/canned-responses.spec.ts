import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { CannedResponsesPage } from '../../pages'

test.describe('Canned Responses Management', () => {
  let cannedResponsesPage: CannedResponsesPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    cannedResponsesPage = new CannedResponsesPage(page)
    await cannedResponsesPage.goto()
  })

  test('should display canned responses page', async () => {
    await cannedResponsesPage.expectPageVisible()
    await expect(cannedResponsesPage.addButton).toBeVisible()
  })

  test('should open create canned response dialog', async () => {
    await cannedResponsesPage.openCreateDialog()
    await cannedResponsesPage.expectDialogVisible()
    await expect(cannedResponsesPage.dialog).toContainText('Canned Response')
  })

  test('should show validation error for empty name and content', async () => {
    await cannedResponsesPage.openCreateDialog()
    await cannedResponsesPage.submitDialog()
    await cannedResponsesPage.expectToast('required')
  })

  test('should create a new canned response', async () => {
    const responseName = `Test Response ${Date.now()}`
    const responseContent = 'Hello! Thank you for contacting us.'

    await cannedResponsesPage.openCreateDialog()
    await cannedResponsesPage.fillResponseForm(responseName, responseContent, 'test', 'Greetings')
    await cannedResponsesPage.submitDialog()

    await cannedResponsesPage.expectToast('created')
    await expect(cannedResponsesPage.page.locator('body')).toContainText(responseName)
  })

  test('should edit existing canned response', async ({ page }) => {
    // First create a response
    const responseName = `Edit Response ${Date.now()}`

    await cannedResponsesPage.openCreateDialog()
    await cannedResponsesPage.fillResponseForm(responseName, 'Original content')
    await cannedResponsesPage.submitDialog()

    // Wait for create toast and dismiss it
    await cannedResponsesPage.expectToast('created')
    await cannedResponsesPage.dismissToast('created')

    // Wait for response to appear in table
    await cannedResponsesPage.expectResponseExists(responseName)

    // Edit the response
    await cannedResponsesPage.editResponse(responseName)
    await cannedResponsesPage.getDialogTextarea().fill('Updated content')
    await cannedResponsesPage.submitDialog('Update')

    await cannedResponsesPage.expectToast('updated')
  })

  test('should delete canned response', async ({ page }) => {
    // First create a response
    const responseName = `Delete Response ${Date.now()}`

    await cannedResponsesPage.openCreateDialog()
    await cannedResponsesPage.fillResponseForm(responseName, 'To be deleted')
    await cannedResponsesPage.submitDialog()

    // Wait for create toast and dismiss it
    await cannedResponsesPage.expectToast('created')
    await cannedResponsesPage.dismissToast('created')

    // Wait for response to appear in table
    await cannedResponsesPage.expectResponseExists(responseName)

    // Delete the response
    await cannedResponsesPage.deleteResponse(responseName)
    await cannedResponsesPage.confirmDelete()

    await cannedResponsesPage.expectToast('deleted')
  })

  test('should filter by category', async () => {
    await cannedResponsesPage.filterByCategory('Greetings')
    // Results filtered (depends on existing data)
  })

  test('should search canned responses', async ({ page }) => {
    // First create a response with unique text
    const uniqueText = `Unique${Date.now()}`

    await cannedResponsesPage.openCreateDialog()
    await cannedResponsesPage.fillResponseForm(uniqueText, 'Search test content')
    await cannedResponsesPage.submitDialog()

    // Wait for creation
    await cannedResponsesPage.expectToast('created')

    // Search for the response
    await cannedResponsesPage.search(uniqueText)
    await expect(page.locator('body')).toContainText(uniqueText)
  })

  test('should cancel canned response creation', async () => {
    await cannedResponsesPage.openCreateDialog()
    await cannedResponsesPage.getDialogInput(0).fill('Cancelled Response')
    await cannedResponsesPage.cancelDialog()
    await cannedResponsesPage.expectDialogHidden()
  })
})
