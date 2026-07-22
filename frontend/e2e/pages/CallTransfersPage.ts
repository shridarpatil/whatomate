import { Page, Locator, expect } from '@playwright/test'

export class CallTransfersPage {
  readonly page: Page
  readonly waitingTab: Locator
  readonly historyTab: Locator
  readonly waitingTable: Locator
  readonly historyTable: Locator

  constructor(page: Page) {
    this.page = page
    this.waitingTab = page.getByRole('tab', { name: /waiting/i })
    this.historyTab = page.getByRole('tab', { name: /history/i })
    this.waitingTable = page.locator('table').first()
    this.historyTable = page.locator('table').first()
  }

  async goto() {
    await this.page.goto('/calling/transfers')
    await this.page.waitForLoadState('networkidle')
  }

  async expectPageVisible() {
    await expect(this.page.locator('[class*="card"], [role="tablist"], table, [class*="empty"]').first()).toBeVisible()
  }

  async switchToWaiting() {
    await this.waitingTab.click()
    await this.page.waitForTimeout(300)
  }

  async switchToHistory() {
    await this.historyTab.click()
    await this.page.waitForTimeout(300)
  }

  async getWaitingCount(): Promise<number> {
    return this.page.locator('tbody tr').count()
  }

  async expectEmptyState() {
    const hasEmpty = await this.page.getByText(/no.*transfer|no.*call/i).isVisible().catch(() => false)
    const hasRows = await this.page.locator('tbody tr').count()
    expect(hasEmpty || hasRows === 0).toBe(true)
  }
}
