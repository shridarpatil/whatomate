import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { CustomActionsPage } from '../../pages'

test.describe('Custom Actions Management', () => {
  let customActionsPage: CustomActionsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    customActionsPage = new CustomActionsPage(page)
    await customActionsPage.goto()
  })

  test('should display custom actions page', async () => {
    await customActionsPage.expectPageVisible()
    await expect(customActionsPage.addButton).toBeVisible()
  })

  test('should open create custom action dialog', async () => {
    await customActionsPage.openCreateDialog()
    await customActionsPage.expectDialogVisible()
    await expect(customActionsPage.dialog).toContainText('Custom Action')
  })

  test('should show validation error for empty name', async () => {
    await customActionsPage.openCreateDialog()
    await customActionsPage.submitDialog()
    await customActionsPage.expectToast('required')
  })

  test('should create a webhook custom action', async () => {
    const actionName = `Webhook Action ${Date.now()}`

    await customActionsPage.openCreateDialog()
    await customActionsPage.fillWebhookAction(actionName, 'https://api.example.com/webhook')
    await customActionsPage.submitDialog()

    await customActionsPage.expectToast('created')
    await customActionsPage.expectRowExists(actionName)
  })

  test('should create a URL custom action', async () => {
    const actionName = `URL Action ${Date.now()}`

    await customActionsPage.openCreateDialog()
    await customActionsPage.fillUrlAction(actionName, 'https://crm.example.com/contact')
    await customActionsPage.submitDialog()

    await customActionsPage.expectToast('created')
    await customActionsPage.expectRowExists(actionName)
  })

  test('should create a JavaScript custom action', async () => {
    const actionName = `JS Action ${Date.now()}`

    await customActionsPage.openCreateDialog()
    await customActionsPage.fillJsAction(actionName, 'return { clipboard: contact.phone_number }')
    await customActionsPage.submitDialog()

    await customActionsPage.expectToast('created')
    await customActionsPage.expectRowExists(actionName)
  })

  test('should edit existing custom action', async () => {
    // First create an action
    const actionName = `Edit Action ${Date.now()}`

    await customActionsPage.openCreateDialog()
    await customActionsPage.fillUrlAction(actionName, 'https://example.com')
    await customActionsPage.submitDialog()

    // Wait for create toast and dismiss it
    await customActionsPage.expectToast('created')
    await customActionsPage.dismissToast('created')

    // Wait for action to appear
    await customActionsPage.expectRowExists(actionName)

    // Edit the action
    await customActionsPage.editRow(actionName)
    await customActionsPage.getDialogInput('url').fill('https://updated.example.com')
    await customActionsPage.submitDialog('Update')

    await customActionsPage.expectToast('updated')
  })

  test('should delete custom action', async () => {
    // First create an action
    const actionName = `Delete Action ${Date.now()}`

    await customActionsPage.openCreateDialog()
    await customActionsPage.fillUrlAction(actionName, 'https://todelete.com')
    await customActionsPage.submitDialog()

    // Wait for create toast and dismiss it
    await customActionsPage.expectToast('created')
    await customActionsPage.dismissToast('created')

    // Wait for action to appear
    await customActionsPage.expectRowExists(actionName)

    // Delete the action
    await customActionsPage.deleteRow(actionName)
    await customActionsPage.confirmDelete()

    await customActionsPage.expectToast('deleted')
  })

  test('should toggle custom action status', async () => {
    // First create an action
    const actionName = `Toggle Action ${Date.now()}`

    await customActionsPage.openCreateDialog()
    await customActionsPage.fillUrlAction(actionName, 'https://toggle.com')
    await customActionsPage.submitDialog()

    // Wait for toast
    await customActionsPage.expectToast('created')

    // Wait for action to appear
    await customActionsPage.expectRowExists(actionName)

    // Toggle the switch (disabling triggers a confirmation dialog)
    await customActionsPage.toggleRowSwitch(actionName)

    // Wait for the confirm dialog and click confirm
    await customActionsPage.alertDialog.waitFor({ state: 'visible' })
    await customActionsPage.alertDialog.getByRole('button', { name: 'Confirm' }).click()

    await customActionsPage.expectToast(/(enabled|disabled)/i)
  })

  test('should cancel custom action creation', async () => {
    await customActionsPage.openCreateDialog()
    await customActionsPage.getDialogInput('name').fill('Cancelled Action')
    await customActionsPage.cancelDialog()
    await customActionsPage.expectDialogHidden()
  })
})
