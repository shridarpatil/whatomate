import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Keywords Page - Chatbot keyword rules management (DataTable + Detail Page)
 */
export class KeywordsPage extends BasePage {
  readonly heading: Locator
  readonly searchInput: Locator
  readonly alertDialog: Locator

  constructor(page: Page) {
    super(page)
    this.heading = page.getByRole('heading', { name: 'Keyword Rules' }).first()
    this.searchInput = page.locator('input[placeholder*="Search"]')
    this.alertDialog = page.locator('[role="alertdialog"]')
  }

  async goto() {
    await this.page.goto('/chatbot/keywords')
    await this.page.waitForLoadState('networkidle')
  }

  async search(term: string) {
    await this.searchInput.fill(term)
    await this.page.waitForTimeout(500)
  }

  async confirmDelete() {
    await this.alertDialog.getByRole('button', { name: 'Delete' }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
  }

  async expectPageVisible() {
    await expect(this.heading).toBeVisible()
  }
}

/**
 * AI Contexts Page - Chatbot AI contexts management (DataTable + Detail Page)
 */
export class AIContextsPage extends BasePage {
  readonly heading: Locator
  readonly searchInput: Locator
  readonly alertDialog: Locator

  constructor(page: Page) {
    super(page)
    this.heading = page.getByRole('heading', { name: 'AI Contexts' }).first()
    this.searchInput = page.locator('input[placeholder*="Search"]')
    this.alertDialog = page.locator('[role="alertdialog"]')
  }

  async goto() {
    await this.page.goto('/chatbot/ai')
    await this.page.waitForLoadState('networkidle')
  }

  async search(term: string) {
    await this.searchInput.fill(term)
    await this.page.waitForTimeout(500)
  }

  async confirmDelete() {
    await this.alertDialog.getByRole('button', { name: 'Delete' }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
  }

  async expectPageVisible() {
    await expect(this.heading).toBeVisible()
  }
}
