import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { GeneralSettingsPage } from '../../pages'

test.describe('General Settings Page', () => {
  let settingsPage: GeneralSettingsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    settingsPage = new GeneralSettingsPage(page)
    await settingsPage.goto()
  })

  test('should display settings page', async () => {
    await settingsPage.expectPageVisible()
  })

  test('should have General tab', async () => {
    await expect(settingsPage.generalTab).toBeVisible()
  })

  test('should have Notifications tab', async () => {
    await expect(settingsPage.notificationsTab).toBeVisible()
  })

  test('should show General tab by default', async () => {
    await expect(settingsPage.orgNameInput).toBeVisible()
  })
})

test.describe('General Tab', () => {
  let settingsPage: GeneralSettingsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    settingsPage = new GeneralSettingsPage(page)
    await settingsPage.goto()
  })

  test('should have organization name field', async () => {
    await expect(settingsPage.orgNameInput).toBeVisible()
  })

  test('should have timezone selector', async () => {
    await expect(settingsPage.timezoneSelect).toBeVisible()
  })

  test('should have date format selector', async ({ page }) => {
    await expect(page.getByText('Date Format')).toBeVisible()
  })

  test('should have mask phone numbers toggle', async ({ page }) => {
    await expect(page.getByText('Mask Phone Numbers')).toBeVisible()
  })

  test('should have save button', async () => {
    await expect(settingsPage.saveButton.first()).toBeVisible()
  })

  test('should show available timezones', async ({ page }) => {
    await settingsPage.timezoneSelect.click()
    await expect(page.locator('[role="option"]').filter({ hasText: 'UTC' })).toBeVisible()
    await expect(page.locator('[role="option"]').filter({ hasText: 'Eastern' })).toBeVisible()
    // Close dropdown
    await page.keyboard.press('Escape')
  })

  test('should show available date formats', async ({ page }) => {
    const dateFormatSelect = page.locator('button[role="combobox"]').nth(1)
    await dateFormatSelect.click()
    await expect(page.locator('[role="option"]').filter({ hasText: 'YYYY-MM-DD' })).toBeVisible()
    await expect(page.locator('[role="option"]').filter({ hasText: 'DD/MM/YYYY' })).toBeVisible()
    await page.keyboard.press('Escape')
  })

  test('should toggle mask phone numbers', async ({ page }) => {
    const toggle = page.locator('button[role="switch"]').first()
    const initialState = await toggle.getAttribute('data-state')
    await toggle.click()
    const newState = await toggle.getAttribute('data-state')
    expect(newState).not.toBe(initialState)
  })

  test('should save general settings', async () => {
    await settingsPage.fillOrgName('Test Organization')
    await settingsPage.saveGeneralSettings()
    await settingsPage.expectToast(/saved|success/i)
  })
})

test.describe('Notifications Tab', () => {
  let settingsPage: GeneralSettingsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    settingsPage = new GeneralSettingsPage(page)
    await settingsPage.goto()
    await settingsPage.switchToNotificationsTab()
  })

  test('should show notifications settings', async ({ page }) => {
    await expect(page.getByText('Email Notifications')).toBeVisible()
  })

  test('should have email notifications toggle', async ({ page }) => {
    await expect(page.getByText('Email Notifications')).toBeVisible()
    const toggle = page.locator('button[role="switch"]').first()
    await expect(toggle).toBeVisible()
  })

  test('should have new message alerts toggle', async ({ page }) => {
    await expect(page.getByText('New Message Alerts')).toBeVisible()
  })

  test('should have campaign updates toggle', async ({ page }) => {
    await expect(page.getByText('Campaign Updates')).toBeVisible()
  })

  test('should toggle email notifications', async ({ page }) => {
    const toggle = page.locator('button[role="switch"]').first()
    const initialState = await toggle.getAttribute('data-state')
    await toggle.click()
    const newState = await toggle.getAttribute('data-state')
    expect(newState).not.toBe(initialState)
  })

  test('should toggle new message alerts', async ({ page }) => {
    const toggle = page.locator('button[role="switch"]').nth(1)
    const initialState = await toggle.getAttribute('data-state')
    await toggle.click()
    const newState = await toggle.getAttribute('data-state')
    expect(newState).not.toBe(initialState)
  })

  test('should toggle campaign updates', async ({ page }) => {
    const toggle = page.locator('button[role="switch"]').nth(2)
    const initialState = await toggle.getAttribute('data-state')
    await toggle.click()
    const newState = await toggle.getAttribute('data-state')
    expect(newState).not.toBe(initialState)
  })

  test('should have save button', async () => {
    await expect(settingsPage.saveButton).toBeVisible()
  })

  test('should save notification settings', async () => {
    await settingsPage.saveNotificationSettings()
    await settingsPage.expectToast(/saved|success/i)
  })
})

test.describe('Settings Tab Navigation', () => {
  let settingsPage: GeneralSettingsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    settingsPage = new GeneralSettingsPage(page)
    await settingsPage.goto()
  })

  test('should switch to Notifications tab', async ({ page }) => {
    await settingsPage.switchToNotificationsTab()
    await expect(page.getByText('Email Notifications')).toBeVisible()
  })

  test('should switch back to General tab', async ({ page }) => {
    await settingsPage.switchToNotificationsTab()
    await settingsPage.switchToGeneralTab()
    await expect(settingsPage.orgNameInput).toBeVisible()
  })
})
