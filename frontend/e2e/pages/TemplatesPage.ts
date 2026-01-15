import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Templates Page - Message templates management
 */
export class TemplatesPage extends BasePage {
  readonly heading: Locator
  readonly createButton: Locator
  readonly syncButton: Locator
  readonly searchInput: Locator
  readonly accountSelect: Locator
  readonly dialog: Locator
  readonly alertDialog: Locator
  readonly previewDialog: Locator

  constructor(page: Page) {
    super(page)
    this.heading = page.locator('h1').filter({ hasText: 'Message Templates' })
    this.createButton = page.getByRole('button', { name: /Create Template/i })
    this.syncButton = page.getByRole('button', { name: /Sync from Meta/i })
    this.searchInput = page.locator('input[placeholder*="Search templates"]')
    this.accountSelect = page.locator('button[role="combobox"]').first()
    this.dialog = page.locator('[role="dialog"][data-state="open"]')
    this.alertDialog = page.locator('[role="alertdialog"]')
    this.previewDialog = page.locator('[role="dialog"][data-state="open"]').filter({ hasText: 'Template Preview' })
  }

  async goto() {
    await this.page.goto('/templates')
    await this.page.waitForLoadState('networkidle')
  }

  async openCreateDialog() {
    await this.createButton.click()
    await this.dialog.waitFor({ state: 'visible' })
  }

  async selectAccount(accountName: string) {
    await this.accountSelect.click()
    await this.page.locator('[role="option"]').filter({ hasText: accountName }).click()
  }

  async search(term: string) {
    await this.searchInput.fill(term)
    await this.page.waitForTimeout(300)
  }

  // Form helpers
  async fillTemplateForm(options: {
    account?: string
    name: string
    displayName?: string
    language?: string
    category?: string
    headerType?: string
    headerContent?: string
    bodyContent: string
    footerContent?: string
  }) {
    if (options.account) {
      await this.dialog.locator('select').first().selectOption(options.account)
    }

    await this.dialog.locator('input').first().fill(options.name)

    if (options.displayName) {
      await this.dialog.locator('input').nth(1).fill(options.displayName)
    }

    if (options.language) {
      await this.dialog.locator('select').nth(1).selectOption(options.language)
    }

    if (options.category) {
      await this.dialog.locator('select').nth(2).selectOption(options.category)
    }

    if (options.headerType) {
      await this.dialog.locator('select').nth(3).selectOption(options.headerType)
    }

    if (options.headerContent && options.headerType === 'TEXT') {
      await this.dialog.locator('input[placeholder*="header"]').fill(options.headerContent)
    }

    await this.dialog.locator('textarea').first().fill(options.bodyContent)

    if (options.footerContent) {
      await this.dialog.locator('input[placeholder*="Thank you"]').fill(options.footerContent)
    }
  }

  async addButton(type: string, text: string, url?: string, phoneNumber?: string) {
    await this.dialog.getByRole('button', { name: /Add Button/i }).click()
    const buttonSection = this.dialog.locator('.border.rounded-lg.p-3').last()
    await buttonSection.locator('select').selectOption(type)
    await buttonSection.locator('input[placeholder="Button text"]').fill(text)

    if (type === 'URL' && url) {
      await buttonSection.locator('input[placeholder*="https"]').fill(url)
    }
    if (type === 'PHONE_NUMBER' && phoneNumber) {
      await buttonSection.locator('input[placeholder*="+123"]').fill(phoneNumber)
    }
  }

  async removeButton(index: number) {
    const buttons = this.dialog.locator('.border.rounded-lg.p-3')
    await buttons.nth(index).getByRole('button').filter({ has: this.page.locator('svg') }).click()
  }

  async submitDialog(buttonText = 'Create Template') {
    await this.dialog.getByRole('button', { name: new RegExp(buttonText, 'i') }).click()
  }

  async cancelDialog() {
    await this.dialog.getByRole('button', { name: /Cancel/i }).click()
    await this.dialog.waitFor({ state: 'hidden' })
  }

  // Card helpers
  getTemplateCard(name: string): Locator {
    return this.page.locator('.rounded-lg.border').filter({ hasText: name })
  }

  async previewTemplate(name: string) {
    const card = this.getTemplateCard(name)
    await card.getByRole('button').filter({ has: this.page.locator('svg.lucide-eye') }).click()
    await this.previewDialog.waitFor({ state: 'visible' })
  }

  async editTemplate(name: string) {
    const card = this.getTemplateCard(name)
    await card.getByRole('button').filter({ has: this.page.locator('svg.lucide-pencil') }).click()
    await this.dialog.waitFor({ state: 'visible' })
  }

  async deleteTemplate(name: string) {
    const card = this.getTemplateCard(name)
    await card.getByRole('button').filter({ has: this.page.locator('svg.lucide-trash-2') }).click()
    await this.alertDialog.waitFor({ state: 'visible' })
  }

  async publishTemplate(name: string) {
    const card = this.getTemplateCard(name)
    await card.getByRole('button').filter({ has: this.page.locator('svg.lucide-send') }).click()
    await this.alertDialog.waitFor({ state: 'visible' })
  }

  async confirmDelete() {
    await this.alertDialog.getByRole('button', { name: 'Delete' }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
  }

  async confirmPublish() {
    await this.alertDialog.getByRole('button', { name: 'Publish' }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
  }

  async cancelAlertDialog() {
    await this.alertDialog.getByRole('button', { name: 'Cancel' }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
  }

  async closePreview() {
    await this.previewDialog.getByRole('button', { name: 'Close' }).click()
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

  async expectDialogHidden() {
    await expect(this.dialog).not.toBeVisible()
  }

  async expectTemplateExists(name: string) {
    await expect(this.getTemplateCard(name)).toBeVisible()
  }

  async expectTemplateNotExists(name: string) {
    await expect(this.getTemplateCard(name)).not.toBeVisible()
  }

  async expectEmptyState() {
    await expect(this.page.getByText('No templates found')).toBeVisible()
  }
}
