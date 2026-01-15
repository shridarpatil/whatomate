import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * General Settings Page - Organization settings
 */
export class GeneralSettingsPage extends BasePage {
  readonly heading: Locator
  readonly generalTab: Locator
  readonly notificationsTab: Locator
  readonly orgNameInput: Locator
  readonly timezoneSelect: Locator
  readonly dateFormatSelect: Locator
  readonly maskPhoneSwitch: Locator
  readonly emailNotificationsSwitch: Locator
  readonly newMessageAlertsSwitch: Locator
  readonly campaignUpdatesSwitch: Locator
  readonly saveButton: Locator

  constructor(page: Page) {
    super(page)
    this.heading = page.getByRole('heading', { name: 'Settings' })
    this.generalTab = page.getByRole('tab', { name: /General/i })
    this.notificationsTab = page.getByRole('tab', { name: /Notifications/i })
    this.orgNameInput = page.locator('input#org_name')
    this.timezoneSelect = page.locator('button[role="combobox"]').first()
    this.dateFormatSelect = page.locator('button[role="combobox"]').nth(1)
    this.maskPhoneSwitch = page.locator('button[role="switch"]').first()
    this.emailNotificationsSwitch = page.locator('button[role="switch"]').first()
    this.newMessageAlertsSwitch = page.locator('button[role="switch"]').nth(1)
    this.campaignUpdatesSwitch = page.locator('button[role="switch"]').nth(2)
    this.saveButton = page.getByRole('button', { name: /Save Changes/i })
  }

  async goto() {
    await this.page.goto('/settings')
    await this.page.waitForLoadState('networkidle')
  }

  async switchToGeneralTab() {
    await this.generalTab.click()
  }

  async switchToNotificationsTab() {
    await this.notificationsTab.click()
  }

  // General settings helpers
  async fillOrgName(name: string) {
    await this.orgNameInput.fill(name)
  }

  async selectTimezone(value: string) {
    await this.timezoneSelect.click()
    await this.page.locator('[role="option"]').filter({ hasText: value }).click()
  }

  async selectDateFormat(value: string) {
    await this.dateFormatSelect.click()
    await this.page.locator('[role="option"]').filter({ hasText: value }).click()
  }

  async toggleMaskPhone() {
    await this.maskPhoneSwitch.click()
  }

  async saveGeneralSettings() {
    await this.saveButton.first().click()
  }

  // Notification settings helpers
  async toggleEmailNotifications() {
    await this.emailNotificationsSwitch.click()
  }

  async toggleNewMessageAlerts() {
    await this.newMessageAlertsSwitch.click()
  }

  async toggleCampaignUpdates() {
    await this.campaignUpdatesSwitch.click()
  }

  async saveNotificationSettings() {
    await this.saveButton.click()
  }

  // Toast helpers
  async expectToast(text: string | RegExp) {
    const toast = this.page.locator('[data-sonner-toast]').filter({ hasText: text })
    await expect(toast).toBeVisible({ timeout: 5000 })
    return toast
  }

  // Assertions
  async expectPageVisible() {
    await expect(this.heading).toBeVisible()
  }

  async expectGeneralTabVisible() {
    await expect(this.orgNameInput).toBeVisible()
  }

  async expectNotificationsTabVisible() {
    await expect(this.emailNotificationsSwitch).toBeVisible()
  }
}
