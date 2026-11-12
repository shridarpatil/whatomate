import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Accounts Page - WhatsApp accounts management (DataTable + Detail Page)
 */
export class AccountsPage extends BasePage {
  readonly heading: Locator
  readonly addButton: Locator
  readonly alertDialog: Locator
  readonly tableBody: Locator

  constructor(page: Page) {
    super(page)
    this.heading = page.locator('h1').filter({ hasText: 'WhatsApp Accounts' })
    this.addButton = page.getByRole('button', { name: /Add Account/i }).first()
    this.dialog = page.locator('dialog[open]')
    this.alertDialog = page.locator('[role="alertdialog"]')
    this.tableBody = page.locator('tbody')
  }

  get profileDialog() {
    return this.page.locator('dialog[open]').filter({ hasText: 'Business Profile' })
  }

  async goto() {
    await this.page.goto('/settings/accounts')
    await this.page.waitForLoadState('networkidle')
  }

  async navigateToCreate() {
    await this.addButton.click()
    await this.page.waitForLoadState('networkidle')
  }

  async navigateToAccount(name: string) {
    const row = this.page.locator('tr').filter({ hasText: name })
    await row.locator('a').first().click()
    await this.page.waitForLoadState('networkidle')
  }

  // Detail page form helpers
  async fillAccountForm(options: {
    name: string
    phoneId: string
    businessId: string
    accessToken: string
  }) {
    // Fields are inputs nested in <label data-field>, so they resolve by label
    await this.page.getByLabel(/Account Name/).fill(options.name)
    await this.page.getByLabel(/Phone Number ID/).fill(options.phoneId)
    await this.page.getByLabel(/Business Account ID/).fill(options.businessId)
    await this.page.getByLabel(/Access Token/).fill(options.accessToken)
  }

  async saveAccount() {
    await this.page.getByRole('button', { name: /Create|Save/i }).first().click()
    await this.page.waitForLoadState('networkidle')
  }

  async deleteAccount(name: string) {
    const row = this.page.locator('tr').filter({ hasText: name })
    await row.getByRole('button', { name: 'Delete' }).click()
    await this.alertDialog.waitFor({ state: 'visible' })
  }

  async testConnection() {
    await this.page.getByRole('button', { name: /Test/i }).click()
  }

  async subscribeApp() {
    await this.page.getByRole('button', { name: /Subscribe/i }).click()
  }

  async openBusinessProfile() {
    await this.page.getByRole('button', { name: /Profile/i }).click()
    await this.profileDialog.waitFor({ state: 'visible' })
  }

  async confirmDelete() {
    await this.alertDialog.getByRole('button', { name: 'Delete' }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
  }

  async cancelDelete() {
    await this.alertDialog.getByRole('button', { name: 'Cancel' }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
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

  async expectProfileDialogVisible() {
    await expect(this.profileDialog).toBeVisible()
    await expect(this.profileDialog.locator('input#about')).toBeVisible()
    await expect(this.profileDialog.locator('textarea#description')).toBeVisible()
  }

  async expectAccountExists(name: string) {
    await expect(this.page.locator('tr').filter({ hasText: name })).toBeVisible()
  }

  async expectAccountNotExists(name: string) {
    await expect(this.page.locator('tr').filter({ hasText: name })).not.toBeVisible()
  }

  async expectEmptyState() {
    await expect(this.page.getByText('No WhatsApp accounts')).toBeVisible()
  }
}
