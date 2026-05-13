import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Templates Page - Message templates management (DataTable + Detail Page)
 */
export class TemplatesPage extends BasePage {
  readonly heading: Locator
  readonly createButton: Locator
  readonly syncButton: Locator
  readonly searchInput: Locator
  readonly accountSelect: Locator
  readonly alertDialog: Locator
  readonly tableBody: Locator

  constructor(page: Page) {
    super(page)
    this.heading = page.locator('h1').filter({ hasText: 'Message Templates' })
    this.createButton = page.locator('a[href="/templates/new"] button').first()
    this.syncButton = page.locator('header').getByRole('button', { name: /Sync from Meta/i })
    this.searchInput = page.locator('input[placeholder*="Search templates"], input[placeholder*="search templates"]')
    this.accountSelect = page.locator('button[role="combobox"]').first()
    this.alertDialog = page.locator('[role="alertdialog"]')
    this.tableBody = page.locator('tbody')
  }

  async goto() {
    await this.page.goto('/templates')
    await this.page.waitForLoadState('networkidle')
  }

  async selectAccount(accountName: string) {
    await this.accountSelect.click()
    await this.page.locator('[role="option"]').filter({ hasText: accountName }).click()
  }

  async search(term: string) {
    await this.searchInput.fill(term)
    await this.page.waitForTimeout(300)
  }

  async deleteTemplateFromList(rowIndex = 0) {
    const row = this.page.locator('tbody tr').nth(rowIndex)
    await row.locator('button').filter({ has: this.page.locator('svg.text-destructive') }).click()
    await this.alertDialog.waitFor({ state: 'visible' })
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

  async expectTemplateInTable(name: string) {
    await expect(this.page.locator('tbody tr').filter({ hasText: name })).toBeVisible()
  }

  async expectTemplateNotInTable(name: string) {
    await expect(this.page.locator('tbody tr').filter({ hasText: name })).not.toBeVisible()
  }

  async expectEmptyState() {
    await expect(this.page.getByText('No templates found')).toBeVisible()
  }
}
