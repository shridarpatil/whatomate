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
    this.createButton = page.getByRole('button', { name: /Create Template/i }).first()
    this.syncButton = page.locator('header').getByRole('button', { name: /Sync from Meta/i })
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

  // Form field selectors using placeholders/labels for stability
  private get dialogAccountSelect() {
    return this.dialog.locator('label').filter({ hasText: 'WhatsApp Account' }).locator('..').locator('select')
  }

  private get templateNameInput() {
    return this.dialog.locator('input[placeholder="order_confirmation"]')
  }

  private get displayNameInput() {
    return this.dialog.locator('input[placeholder="Order Confirmation"]')
  }

  private get languageSelect() {
    return this.dialog.locator('label').filter({ hasText: /^Language/ }).locator('..').locator('select')
  }

  private get categorySelect() {
    return this.dialog.locator('label').filter({ hasText: /^Category/ }).locator('..').locator('select')
  }

  private get headerTypeSelect() {
    return this.dialog.locator('label').filter({ hasText: 'Header Type' }).locator('..').locator('select')
  }

  private get headerTextInput() {
    return this.dialog.locator('input[placeholder="Enter header text..."]')
  }

  private get bodyTextarea() {
    return this.dialog.locator('textarea[placeholder*="Hi {{1}}"]')
  }

  private get footerInput() {
    return this.dialog.locator('input[placeholder="Thank you for your business!"]')
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
      await this.dialogAccountSelect.selectOption(options.account)
    }

    await this.templateNameInput.fill(options.name)

    if (options.displayName) {
      await this.displayNameInput.fill(options.displayName)
    }

    if (options.language) {
      await this.languageSelect.selectOption(options.language)
    }

    if (options.category) {
      await this.categorySelect.selectOption(options.category)
    }

    if (options.headerType) {
      await this.headerTypeSelect.selectOption(options.headerType)
    }

    if (options.headerContent && options.headerType === 'TEXT') {
      await this.headerTextInput.fill(options.headerContent)
    }

    await this.bodyTextarea.fill(options.bodyContent)

    if (options.footerContent) {
      await this.footerInput.fill(options.footerContent)
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
    // X button is in the flex justify-between header row
    await buttons.nth(index).locator('.flex.items-center.justify-between button').click()
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
    await card.locator('button').filter({ has: this.page.locator('.lucide-eye') }).click()
    await this.previewDialog.waitFor({ state: 'visible' })
  }

  async editTemplate(name: string) {
    const card = this.getTemplateCard(name)
    await card.locator('button').filter({ has: this.page.locator('.lucide-pencil') }).click()
    await this.dialog.waitFor({ state: 'visible' })
  }

  async deleteTemplate(name: string) {
    const card = this.getTemplateCard(name)
    await card.locator('button').filter({ has: this.page.locator('.lucide-trash-2') }).click()
    await this.alertDialog.waitFor({ state: 'visible' })
  }

  async publishTemplate(name: string) {
    const card = this.getTemplateCard(name)
    await card.locator('button').filter({ has: this.page.locator('.lucide-send') }).click()
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
