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
  readonly saveButton: Locator

  constructor(page: Page) {
    super(page)
    this.heading = page.locator('h1').filter({ hasText: 'Settings' })
    this.generalTab = page.getByRole('tab', { name: /General/i })
    this.notificationsTab = page.getByRole('tab', { name: /Notifications/i })
    this.orgNameInput = page.locator('input#org_name')
    // Native selects labelled by their wrapping <label data-field>
    this.timezoneSelect = page.getByLabel('Default Timezone')
    this.dateFormatSelect = page.getByLabel('Date Format')
    this.saveButton = page.getByRole('button', { name: /Save Changes/i })
  }

  // Helper to get switch by its label text
  private getSwitchByLabel(labelText: string): Locator {
    return this.page.getByRole('switch', { name: new RegExp(labelText) })
  }

  // Switch locators using label-based approach
  get maskPhoneSwitch(): Locator {
    return this.getSwitchByLabel('Mask Phone Numbers')
  }

  get emailNotificationsSwitch(): Locator {
    return this.getSwitchByLabel('Email Notifications')
  }

  get newMessageAlertsSwitch(): Locator {
    return this.getSwitchByLabel('New Message Alerts')
  }

  get campaignUpdatesSwitch(): Locator {
    return this.getSwitchByLabel('Campaign Updates')
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
    await this.timezoneSelect.selectOption({ label: value })
  }

  async selectDateFormat(value: string) {
    await this.dateFormatSelect.selectOption({ label: value })
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
    const toast = this.page.locator('.toast-container output').filter({ hasText: text })
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
