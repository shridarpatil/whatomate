import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

export class CampaignsPage extends BasePage {
  readonly heading: Locator
  readonly createButton: Locator
  readonly statusFilter: Locator
  readonly timeRangeFilter: Locator
  readonly createDialog: Locator
  readonly alertDialog: Locator

  constructor(page: Page) {
    super(page)
    this.heading = page.getByRole('heading', { name: /Campaigns/i }).first()
    this.createButton = page.getByRole('button', { name: /Create Campaign/i }).first()
    this.statusFilter = page.locator('button[role="combobox"]').first()
    this.timeRangeFilter = page.locator('button[role="combobox"]').filter({ hasText: /This month|Today|days/i }).first()
    this.createDialog = page.locator('[role="dialog"][data-state="open"]')
    this.alertDialog = page.locator('[role="alertdialog"]')
  }

  async goto() {
    await this.page.goto('/campaigns')
    await this.page.waitForLoadState('networkidle')
  }

  async openCreateDialog() {
    await this.createButton.click()
    await this.createDialog.waitFor({ state: 'visible' })
  }

  // Campaign card actions
  getCampaignCard(index = 0): Locator {
    return this.page.locator('.rounded-lg.border').nth(index)
  }

  getEditButton(card?: Locator): Locator {
    const container = card || this.page
    return container.locator('button').filter({ has: this.page.locator('.lucide-pencil') }).first()
  }

  getDeleteButton(card?: Locator): Locator {
    const container = card || this.page
    return container.locator('button').filter({ has: this.page.locator('.lucide-trash-2') }).first()
  }

  getViewRecipientsButton(card?: Locator): Locator {
    const container = card || this.page
    return container.locator('button').filter({ has: this.page.locator('.lucide-eye') }).first()
  }

  getAddRecipientsButton(card?: Locator): Locator {
    const container = card || this.page
    return container.locator('button').filter({ has: this.page.locator('.lucide-user-plus') }).first()
  }

  getStartButton(card?: Locator): Locator {
    const container = card || this.page
    return container.getByRole('button', { name: /^Start$/i }).first()
  }

  getPauseButton(card?: Locator): Locator {
    const container = card || this.page
    return container.getByRole('button', { name: /Pause/i }).first()
  }

  getResumeButton(card?: Locator): Locator {
    const container = card || this.page
    return container.getByRole('button', { name: /Resume/i }).first()
  }

  getCancelButton(card?: Locator): Locator {
    const container = card || this.page
    return container.getByRole('button', { name: /^Cancel$/i }).first()
  }

  getRetryFailedButton(card?: Locator): Locator {
    const container = card || this.page
    return container.getByRole('button', { name: /Retry Failed/i }).first()
  }

  // Dialog interactions
  async clickEditButton() {
    const btn = this.getEditButton()
    if (await btn.isVisible()) {
      await btn.click()
      await this.createDialog.waitFor({ state: 'visible' })
      return true
    }
    return false
  }

  async clickDeleteButton() {
    const btn = this.getDeleteButton()
    if (await btn.isVisible()) {
      await btn.click()
      await this.alertDialog.waitFor({ state: 'visible' })
      return true
    }
    return false
  }

  async clickViewRecipientsButton() {
    const btn = this.getViewRecipientsButton()
    if (await btn.isVisible()) {
      await btn.click()
      await this.createDialog.waitFor({ state: 'visible' })
      return true
    }
    return false
  }

  async clickAddRecipientsButton() {
    const btn = this.getAddRecipientsButton()
    if (await btn.isVisible()) {
      await btn.click()
      await this.createDialog.waitFor({ state: 'visible' })
      return true
    }
    return false
  }

  // Alert dialog actions
  async confirmDelete() {
    await this.alertDialog.getByRole('button', { name: /Delete/i }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
  }

  async cancelDelete() {
    await this.alertDialog.getByRole('button', { name: /Cancel/i }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
  }

  async confirmCancelCampaign() {
    await this.alertDialog.getByRole('button', { name: /Cancel Campaign/i }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
  }

  async keepRunning() {
    await this.alertDialog.getByRole('button', { name: /Keep Running/i }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
  }

  // Form helpers
  async fillCampaignName(name: string) {
    await this.createDialog.locator('input#name').fill(name)
  }

  async selectAccount(accountName: string) {
    const accountTrigger = this.createDialog.locator('button[role="combobox"]').first()
    await accountTrigger.click()
    await this.page.locator('[role="option"]').filter({ hasText: accountName }).click()
  }

  async selectTemplate(templateName: string) {
    const templateTriggers = this.createDialog.locator('button[role="combobox"]')
    // Template is typically the second combobox (after account)
    await templateTriggers.nth(1).click()
    await this.page.locator('[role="option"]').filter({ hasText: templateName }).click()
  }

  // Template dropdown helpers
  getTemplateSelectTrigger(): Locator {
    return this.createDialog.locator('button[role="combobox"]').nth(1)
  }

  getTemplateOptions(): Locator {
    return this.page.locator('[role="option"]')
  }

  getNoTemplatesMessage(): Locator {
    return this.createDialog.getByText(/No templates found/i)
  }

  async openTemplateDropdown() {
    await this.getTemplateSelectTrigger().click()
  }

  // Recipients dialog helpers
  getManualEntryTab(): Locator {
    return this.createDialog.getByRole('tab', { name: /Manual Entry/i })
  }

  getCsvUploadTab(): Locator {
    return this.createDialog.getByRole('tab', { name: /Upload CSV/i })
  }

  getRecipientsTextarea(): Locator {
    return this.createDialog.locator('textarea#recipients')
  }

  getCsvFileInput(): Locator {
    return this.createDialog.locator('input[type="file"]')
  }

  // Assertions
  async expectPageVisible() {
    await expect(this.heading).toBeVisible()
  }

  async expectDialogVisible() {
    await expect(this.createDialog).toBeVisible()
  }

  async expectDialogHidden() {
    await expect(this.createDialog).not.toBeVisible()
  }

  async expectAlertDialogVisible() {
    await expect(this.alertDialog).toBeVisible()
  }

  async expectAlertDialogHidden() {
    await expect(this.alertDialog).not.toBeVisible()
  }

  async expectDialogTitle(title: string | RegExp) {
    await expect(this.createDialog).toContainText(title)
  }

  async expectAlertDialogTitle(title: string | RegExp) {
    await expect(this.alertDialog).toContainText(title)
  }

  // Check if button exists
  async hasEditButton(): Promise<boolean> {
    return this.getEditButton().isVisible()
  }

  async hasDeleteButton(): Promise<boolean> {
    return this.getDeleteButton().isVisible()
  }

  async hasViewRecipientsButton(): Promise<boolean> {
    return this.getViewRecipientsButton().isVisible()
  }

  async hasAddRecipientsButton(): Promise<boolean> {
    return this.getAddRecipientsButton().isVisible()
  }
}
