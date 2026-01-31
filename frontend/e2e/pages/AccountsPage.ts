import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Accounts Page - WhatsApp accounts management
 */
export class AccountsPage extends BasePage {
  readonly heading: Locator
  readonly addButton: Locator
  readonly dialog: Locator
  readonly alertDialog: Locator

  constructor(page: Page) {
    super(page)
    this.heading = page.locator('h1').filter({ hasText: 'WhatsApp Accounts' })
    this.addButton = page.getByRole('button', { name: /Add Account/i }).first()
    this.dialog = page.locator('[role="dialog"][data-state="open"]')
    this.alertDialog = page.locator('[role="alertdialog"]')
  }

  get profileDialog() {
    return this.page.locator('[role="dialog"][data-state="open"]').filter({ hasText: 'Business Profile' })
  }

  async goto() {
    await this.page.goto('/settings/accounts')
    await this.page.waitForLoadState('networkidle')
  }

  async openCreateDialog() {
    await this.addButton.click()
    await this.dialog.waitFor({ state: 'visible' })
  }

  // Form helpers
  async fillAccountForm(options: {
    name: string
    phoneId: string
    businessId: string
    accessToken: string
    verifyToken?: string
  }) {
    await this.dialog.locator('input#name').fill(options.name)
    await this.dialog.locator('input#phone_id').fill(options.phoneId)
    await this.dialog.locator('input#business_id').fill(options.businessId)
    await this.dialog.locator('input#access_token').fill(options.accessToken)

    if (options.verifyToken) {
      await this.dialog.locator('input#verify_token').fill(options.verifyToken)
    }
  }

  async submitDialog(buttonText = 'Create') {
    // Button text is "Create Account" or "Update Account"
    await this.dialog.getByRole('button', { name: new RegExp(`${buttonText}`, 'i') }).click()
  }

  async cancelDialog() {
    await this.dialog.getByRole('button', { name: /Cancel/i }).click()
    await this.dialog.waitFor({ state: 'hidden' })
  }

  // Card helpers
  getAccountCard(name: string): Locator {
    return this.page.locator('.account-card').filter({ hasText: name })
  }

  async editAccount(name: string) {
    const card = this.getAccountCard(name)
    await expect(card).toBeVisible({ timeout: 10000 })
    // Edit button is the first icon-only button (svg.h-4 without span text)
    const iconButtons = card.locator('button:has(svg.h-4)').filter({ hasNot: this.page.locator('span') })
    await iconButtons.first().click()
    await this.dialog.waitFor({ state: 'visible' })
  }

  async deleteAccount(name: string) {
    const card = this.getAccountCard(name)
    await expect(card).toBeVisible({ timeout: 10000 })
    // Delete button has svg with text-destructive class
    await card.locator('button').filter({ has: this.page.locator('svg.text-destructive') }).click()
    await this.alertDialog.waitFor({ state: 'visible' })
  }

  async openBusinessProfile(name: string) {
    const card = this.getAccountCard(name)
    await expect(card).toBeVisible({ timeout: 10000 })
    // Profile button has svg with text-emerald-500 class
    await card.locator('button').filter({ has: this.page.locator('svg.text-emerald-500') }).click()
    await this.profileDialog.waitFor({ state: 'visible' })
  }

  async testConnection(name: string) {
    const card = this.getAccountCard(name)
    await card.getByRole('button', { name: /Test/i }).click()
  }

  async subscribeApp(name: string) {
    const card = this.getAccountCard(name)
    await card.getByRole('button', { name: /Subscribe/i }).click()
  }

  async copyWebhookUrl(name: string) {
    const card = this.getAccountCard(name)
    await card.locator('button[title*="Copy"]').first().click()
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
    const toast = this.page.locator('[data-sonner-toast]').filter({ hasText: text })
    await expect(toast).toBeVisible({ timeout: 5000 })
    return toast
  }

  async dismissToast(text?: string | RegExp) {
    const toast = text
      ? this.page.locator('[data-sonner-toast]').filter({ hasText: text })
      : this.page.locator('[data-sonner-toast]').first()
    if (await toast.isVisible()) {
      await toast.click()
    }
  }

  // Assertions
  async expectPageVisible() {
    await expect(this.heading).toBeVisible()
  }

  async expectDialogVisible() {
    await expect(this.dialog).toBeVisible()
  }

  async expectProfileDialogVisible() {
    await expect(this.profileDialog).toBeVisible()
    await expect(this.profileDialog.locator('input#about')).toBeVisible()
    await expect(this.profileDialog.locator('textarea#description')).toBeVisible()
  }

  async expectDialogHidden() {
    await expect(this.dialog).not.toBeVisible()
  }

  async expectAccountExists(name: string) {
    await expect(this.getAccountCard(name)).toBeVisible()
  }

  async expectAccountNotExists(name: string) {
    await expect(this.getAccountCard(name)).not.toBeVisible()
  }

  async expectEmptyState() {
    await expect(this.page.getByText('No WhatsApp accounts')).toBeVisible()
  }
}
