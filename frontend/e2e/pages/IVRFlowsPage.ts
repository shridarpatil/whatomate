import { Page, Locator, expect } from '@playwright/test'

export class IVRFlowsPage {
  readonly page: Page
  readonly table: Locator
  readonly addButton: Locator
  readonly searchInput: Locator

  constructor(page: Page) {
    this.page = page
    this.table = page.locator('table')
    this.addButton = page.getByRole('button', { name: /create.*flow/i }).first()
    this.searchInput = page.getByPlaceholder(/search/i)
  }

  async goto() {
    await this.page.goto('/calling/ivr-flows')
    await this.page.waitForLoadState('networkidle')
  }

  async expectPageVisible() {
    await expect(this.page.locator('[class*="card"], table, [class*="empty"]').first()).toBeVisible()
  }

  async getRowCount(): Promise<number> {
    return this.page.locator('tbody tr').count()
  }

  async clickFlow(name: string) {
    await this.page.locator('tbody tr').filter({ hasText: name }).click()
  }
}
