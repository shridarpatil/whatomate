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
    await expect(settingsPage.dateFormatSelect).toBeVisible()
  })

  test('should have mask phone numbers toggle', async ({ page }) => {
    await expect(page.getByText('Mask Phone Numbers', { exact: true })).toBeVisible()
  })

  test('should have save button', async () => {
    await expect(settingsPage.saveButton.first()).toBeVisible()
  })

  test('should show available timezones', async () => {
    await expect(settingsPage.timezoneSelect.locator('option', { hasText: 'UTC' })).toHaveCount(1)
    await expect(settingsPage.timezoneSelect.locator('option', { hasText: 'Eastern' })).toHaveCount(1)
  })

  test('should show available date formats', async () => {
    await expect(settingsPage.dateFormatSelect.locator('option', { hasText: 'YYYY-MM-DD' })).toHaveCount(1)
    await expect(settingsPage.dateFormatSelect.locator('option', { hasText: 'DD/MM/YYYY' })).toHaveCount(1)
  })

  test('should toggle mask phone numbers', async ({ page }) => {
    const toggle = page.getByRole('switch').first()
    const initialState = await toggle.isChecked()
    await toggle.click()
    expect(await toggle.isChecked()).toBe(!initialState)
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
    await expect(page.getByText('Email Notifications', { exact: true })).toBeVisible()
  })

  test('should have email notifications toggle', async ({ page }) => {
    await expect(page.getByText('Email Notifications', { exact: true })).toBeVisible()
    const toggle = page.getByRole('switch').first()
    await expect(toggle).toBeVisible()
  })

  test('should have new message alerts toggle', async ({ page }) => {
    await expect(page.getByText('New Message Alerts', { exact: true })).toBeVisible()
  })

  test('should have campaign updates toggle', async ({ page }) => {
    await expect(page.getByText('Campaign Updates', { exact: true })).toBeVisible()
  })

  test('should toggle email notifications', async ({ page }) => {
    const toggle = page.getByRole('switch').first()
    const initialState = await toggle.isChecked()
    await toggle.click()
    expect(await toggle.isChecked()).toBe(!initialState)
  })

  test('should toggle new message alerts', async ({ page }) => {
    const toggle = page.getByRole('switch').nth(1)
    const initialState = await toggle.isChecked()
    await toggle.click()
    expect(await toggle.isChecked()).toBe(!initialState)
  })

  test('should toggle campaign updates', async ({ page }) => {
    const toggle = page.getByRole('switch').nth(2)
    const initialState = await toggle.isChecked()
    await toggle.click()
    expect(await toggle.isChecked()).toBe(!initialState)
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
    await expect(page.getByText('Email Notifications', { exact: true })).toBeVisible()
  })

  test('should switch back to General tab', async ({ page }) => {
    await settingsPage.switchToNotificationsTab()
    await settingsPage.switchToGeneralTab()
    await expect(settingsPage.orgNameInput).toBeVisible()
  })
})
