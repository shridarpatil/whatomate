import { test, expect } from '@playwright/test'
import { TablePage, DialogPage } from '../../pages'
import { loginAsAdmin, createWebhookFixture } from '../../helpers'

test.describe('Webhooks Management', () => {
  let tablePage: TablePage
  let dialogPage: DialogPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/webhooks')
    await page.waitForLoadState('networkidle')

    tablePage = new TablePage(page)
    dialogPage = new DialogPage(page)
  })

  test('should display webhooks list', async ({ page }) => {
    await expect(tablePage.tableBody).toBeVisible()
  })

  test('should open create webhook dialog', async ({ page }) => {
    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await expect(dialogPage.dialog).toBeVisible()
  })

  test('should create a new webhook', async ({ page }) => {
    const webhook = createWebhookFixture()

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()

    await dialogPage.fillField('Name', webhook.name)
    await dialogPage.fillField('URL', webhook.url)

    // Select at least one event using checkbox
    await dialogPage.checkCheckbox('Message Incoming')

    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Verify webhook appears in list
    await tablePage.expectRowExists(webhook.name)
  })

  test('should show validation error for invalid URL', async ({ page }) => {
    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()

    await dialogPage.fillField('Name', 'Test Webhook')
    await dialogPage.fillField('URL', 'not-a-valid-url')
    await dialogPage.checkCheckbox('Message Incoming')

    await dialogPage.submit()

    // Should show validation error and stay open
    await expect(dialogPage.dialog).toBeVisible()
  })

  test('should show validation error when no events selected', async ({ page }) => {
    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()

    await dialogPage.fillField('Name', 'Test Webhook')
    await dialogPage.fillField('URL', 'https://webhook.site/test')
    // Don't select any events

    await dialogPage.submit()

    // Should show validation error and stay open
    await expect(dialogPage.dialog).toBeVisible()
  })

  test('should edit existing webhook', async ({ page }) => {
    // First create a webhook to edit
    const webhook = createWebhookFixture()

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await dialogPage.fillField('Name', webhook.name)
    await dialogPage.fillField('URL', webhook.url)
    await dialogPage.checkCheckbox('Message Incoming')
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Now edit the webhook
    await tablePage.editRow(webhook.name)
    await dialogPage.waitForOpen()

    const updatedName = webhook.name + ' Updated'
    await dialogPage.fillField('Name', updatedName)
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Verify update
    await tablePage.expectRowExists(updatedName)
  })

  test('should delete webhook', async ({ page }) => {
    // First create a webhook to delete
    const webhook = createWebhookFixture({ name: 'Webhook To Delete ' + Date.now() })

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await dialogPage.fillField('Name', webhook.name)
    await dialogPage.fillField('URL', webhook.url)
    await dialogPage.checkCheckbox('Message Incoming')
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Verify it exists
    await tablePage.expectRowExists(webhook.name)

    // Delete the webhook
    await tablePage.deleteRow(webhook.name)

    // Verify deletion
    await tablePage.expectRowNotExists(webhook.name)
  })

  test('should toggle webhook events', async ({ page }) => {
    const webhook = createWebhookFixture()

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()

    await dialogPage.fillField('Name', webhook.name)
    await dialogPage.fillField('URL', webhook.url)

    // Check multiple events
    await dialogPage.checkCheckbox('Message Incoming')
    await dialogPage.checkCheckbox('Message Sent')

    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Verify webhook was created
    await tablePage.expectRowExists(webhook.name)
  })

  test('should cancel webhook creation', async ({ page }) => {
    const webhookName = 'Cancelled Webhook ' + Date.now()

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()

    await dialogPage.fillField('Name', webhookName)
    await dialogPage.fillField('URL', 'https://test.com')
    await dialogPage.cancel()

    await dialogPage.waitForClose()

    // Webhook should not be created
    await tablePage.expectRowNotExists(webhookName)
  })
})

test.describe('Webhook Testing', () => {
  test('should test webhook endpoint', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/webhooks')
    await page.waitForLoadState('networkidle')

    const tablePage = new TablePage(page)
    const dialogPage = new DialogPage(page)

    // Create a webhook first
    const webhook = createWebhookFixture()

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await dialogPage.fillField('Name', webhook.name)
    await dialogPage.fillField('URL', webhook.url)
    await dialogPage.checkCheckbox('Message Incoming')
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Find test button for the webhook
    const row = await tablePage.getRow(webhook.name)

    const testButton = row.locator('button').filter({ hasText: /Test/i })
    if (await testButton.count() > 0) {
      await testButton.click()
      // Wait for test result (toast or inline message)
      await page.waitForTimeout(2000)
    }
  })
})
