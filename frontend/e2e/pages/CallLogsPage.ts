import { Page, Locator, expect } from '@playwright/test'

export class CallLogsPage {
  readonly page: Page
  readonly heading: Locator
  readonly table: Locator
  readonly searchInput: Locator
  readonly statusFilter: Locator
  readonly accountFilter: Locator
  readonly directionFilter: Locator
  readonly refreshButton: Locator
  readonly detailDialog: Locator

  constructor(page: Page) {
    this.page = page
    this.heading = page.getByRole('heading', { name: /Call Logs/i })
    this.table = page.locator('table')
    this.searchInput = page.getByPlaceholder(/search/i)
    this.statusFilter = page.getByLabel('Filter by status')
    this.accountFilter = page.getByLabel('Filter by account')
    this.directionFilter = page.getByLabel('Filter by direction')
    this.refreshButton = page.getByRole('button').filter({ has: page.locator('svg') }).filter({ hasText: /refresh/i }).or(
      page.getByRole('button', { name: /refresh/i })
    )
    this.detailDialog = page.getByRole('dialog')
  }

  async goto() {
    await this.page.goto('/calling/logs')
    await this.page.waitForLoadState('networkidle')
  }

  async expectPageVisible() {
    // The page should have either data or the call logs heading/content
    await expect(this.page.locator('[class*="card"], table, [class*="empty"]').first()).toBeVisible()
  }

  // Data rows only: the empty state is also rendered as a tbody row, but has no caller link.
  private get dataRows(): Locator {
    return this.page.locator('tbody tr').filter({ has: this.page.getByRole('link') })
  }

  async getRowCount(): Promise<number> {
    return this.dataRows.count()
  }

  async clickRow(index: number) {
    await this.dataRows.nth(index).getByRole('link').first().click()
  }

  async search(query: string) {
    await this.searchInput.fill(query)
    await this.page.waitForTimeout(500)
  }

  async clearSearch() {
    await this.searchInput.clear()
    await this.page.waitForTimeout(500)
  }
}
