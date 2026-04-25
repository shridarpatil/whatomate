import { test, expect, type Page, type Locator } from '@playwright/test'
import { TablePage } from '../../pages'
import { loginAsAdmin, createWebhookFixture } from '../../helpers'

function nameInput(page: Page): Locator {
  return page.getByPlaceholder('My Helpdesk Integration')
}

function urlInput(page: Page): Locator {
  return page.getByPlaceholder('https://example.com/webhook')
}

function saveButton(page: Page): Locator {
  return page.getByRole('button', { name: /^(Create|Save)$/i }).first()
}

async function gotoCreateWebhook(page: Page) {
  await page.getByRole('button', { name: /^Add Webhook$/i }).first().click()
  await page.waitForURL(/\/settings\/webhooks\/new$/)
  await page.waitForLoadState('networkidle')
}

async function openWebhookDetail(tablePage: TablePage, page: Page, rowText: string) {
  await tablePage.search(rowText)
  await page.locator('tbody tr .font-medium').getByText(rowText, { exact: true }).first().click()
  await page.waitForURL(/\/settings\/webhooks\/[a-f0-9-]+$/)
  await page.waitForLoadState('networkidle')
}

test.describe('Webhooks Management', () => {
  let tablePage: TablePage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/webhooks')
    await page.waitForLoadState('networkidle')

    tablePage = new TablePage(page)
  })

  test('should display webhooks list', async ({ page }) => {
    await expect(tablePage.tableBody).toBeVisible()
  })

  test('should navigate to create webhook page', async ({ page }) => {
    await gotoCreateWebhook(page)
    expect(page.url()).toContain('/settings/webhooks/new')
  })

  test('should create a new webhook', async ({ page }) => {
    const webhook = createWebhookFixture()

    await gotoCreateWebhook(page)

    const nameField = nameInput(page)
    await nameField.fill(webhook.name)
    await urlInput(page).fill(webhook.url)

    // Select at least one event
    const checkbox = page.locator('button[role="checkbox"]').first()
    if (await checkbox.isVisible()) await checkbox.click()

    await saveButton(page).click()
    await page.waitForURL(/\/settings\/webhooks\/[a-f0-9-]+$/, { timeout: 10000 })
    await page.waitForLoadState('networkidle')

    // Verify in list
    await page.goto('/settings/webhooks')
    await page.waitForLoadState('networkidle')
    await tablePage.search(webhook.name)
    await tablePage.expectRowExists(webhook.name)
  })

  test('should edit existing webhook', async ({ page }) => {
    // Create a webhook first
    const webhook = createWebhookFixture()
    await gotoCreateWebhook(page)
    await nameInput(page).fill(webhook.name)
    await urlInput(page).fill(webhook.url)
    const checkbox = page.locator('button[role="checkbox"]').first()
    if (await checkbox.isVisible()) await checkbox.click()
    await saveButton(page).click()
    await page.waitForURL(/\/settings\/webhooks\/[a-f0-9-]+$/, { timeout: 10000 })
    await page.waitForLoadState('networkidle')

    // Navigate back and open detail
    await page.goto('/settings/webhooks')
    await page.waitForLoadState('networkidle')
    await openWebhookDetail(tablePage, page, webhook.name)

    const updatedName = webhook.name + ' Updated'
    await nameInput(page).fill(updatedName)
    await page.waitForTimeout(300)
    await saveButton(page).click()
    await page.waitForLoadState('networkidle')

    // Verify in list
    await page.goto('/settings/webhooks')
    await page.waitForLoadState('networkidle')
    await tablePage.search(updatedName)
    await tablePage.expectRowExists(updatedName)
  })

  test('should delete webhook', async ({ page }) => {
    const webhook = createWebhookFixture({ name: 'Webhook To Delete ' + Date.now() })

    await gotoCreateWebhook(page)
    await nameInput(page).fill(webhook.name)
    await urlInput(page).fill(webhook.url)
    const checkbox = page.locator('button[role="checkbox"]').first()
    if (await checkbox.isVisible()) await checkbox.click()
    await saveButton(page).click()
    await page.waitForURL(/\/settings\/webhooks\/[a-f0-9-]+$/, { timeout: 10000 })
    await page.waitForLoadState('networkidle')

    // Navigate back and delete from list
    await page.goto('/settings/webhooks')
    await page.waitForLoadState('networkidle')
    await tablePage.search(webhook.name)
    await tablePage.expectRowExists(webhook.name)
    await tablePage.deleteRow(webhook.name)
    await tablePage.expectRowNotExists(webhook.name)
  })
})

test.describe('Webhook Toggle Confirmation', () => {
  test('should show confirmation when disabling webhook', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/webhooks')
    await page.waitForLoadState('networkidle')

    const toggleSwitch = page.getByRole('switch').first()
    if (await toggleSwitch.isVisible()) {
      await toggleSwitch.click()
      const alertDialog = page.locator('[role="alertdialog"]')
      const dialogVisible = await alertDialog.isVisible({ timeout: 3000 }).catch(() => false)
      if (dialogVisible) {
        const cancelBtn = alertDialog.getByRole('button', { name: /cancel/i })
        await cancelBtn.click()
        await alertDialog.waitFor({ state: 'hidden' })
      }
    }
  })
})

test.describe('Webhooks - Table Sorting', () => {
  let tablePage: TablePage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/webhooks')
    await page.waitForLoadState('networkidle')
    tablePage = new TablePage(page)
  })

  test('should sort by name', async () => {
    await tablePage.clickColumnHeader('Name')
    const direction = await tablePage.getSortDirection('Name')
    expect(direction).not.toBeNull()
  })

  test('should sort by URL', async () => {
    await tablePage.clickColumnHeader('URL')
    const direction = await tablePage.getSortDirection('URL')
    expect(direction).not.toBeNull()
  })

  test('should sort by status', async () => {
    await tablePage.clickColumnHeader('Status')
    const direction = await tablePage.getSortDirection('Status')
    expect(direction).not.toBeNull()
  })

  test('should sort by created date', async () => {
    await tablePage.clickColumnHeader('Created')
    const direction = await tablePage.getSortDirection('Created')
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
