import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Agent Transfers Page - Transfer queue management
 */
export class AgentTransfersPage extends BasePage {
  readonly heading: Locator
  readonly pickNextButton: Locator
  readonly myTransfersTab: Locator
  readonly queueTab: Locator
  readonly allActiveTab: Locator
  readonly historyTab: Locator
  readonly teamFilterSelect: Locator
  readonly assignDialog: Locator
  readonly loadMoreButton: Locator

  constructor(page: Page) {
    super(page)
    this.heading = page.locator('h1').filter({ hasText: 'Transfers' })
    this.pickNextButton = page.getByRole('button', { name: /Pick Next/i })
    this.myTransfersTab = page.getByRole('tab', { name: /My Transfers/i })
    this.queueTab = page.getByRole('tab', { name: /Queue/i })
    this.allActiveTab = page.getByRole('tab', { name: /All Active/i })
    this.historyTab = page.getByRole('tab', { name: /History/i })
    this.teamFilterSelect = page.locator('button[role="combobox"]')
    this.assignDialog = page.locator('[role="dialog"][data-state="open"]')
    this.loadMoreButton = page.getByRole('button', { name: /Load More/i })
  }

  async goto() {
    await this.page.goto('/chatbot/transfers')
    await this.page.waitForLoadState('networkidle')
  }

  // Tab navigation
  async switchToMyTransfers() {
    await this.myTransfersTab.click()
  }

  async switchToQueue() {
    await this.queueTab.click()
  }

  async switchToAllActive() {
    await this.allActiveTab.click()
  }

  async switchToHistory() {
    await this.historyTab.click()
  }

  // Actions
  async pickNextTransfer() {
    await this.pickNextButton.click()
  }

  async filterByTeam(teamName: string) {
    await this.teamFilterSelect.click()
    await this.page.locator('[role="option"]').filter({ hasText: teamName }).click()
  }

  // Table helpers
  getTransferRow(contactName: string): Locator {
    return this.page.locator('tr').filter({ hasText: contactName })
  }

  async viewChat(contactName: string) {
    const row = this.getTransferRow(contactName)
    await row.locator('button').filter({ has: this.page.locator('.lucide-message-square') }).click()
  }

  async resumeTransfer(contactName: string) {
    const row = this.getTransferRow(contactName)
    await row.locator('button').filter({ has: this.page.locator('.lucide-play') }).click()
  }

  async openAssignDialog(contactName: string) {
    const row = this.getTransferRow(contactName)
    await row.locator('button').filter({ has: this.page.locator('.lucide-user-plus') }).click()
    await this.assignDialog.waitFor({ state: 'visible' })
  }

  // Assign dialog helpers
  async selectTeamInDialog(teamName: string) {
    await this.assignDialog.locator('button[role="combobox"]').first().click()
    await this.page.locator('[role="option"]').filter({ hasText: teamName }).click()
  }

  async selectAgentInDialog(agentName: string) {
    await this.assignDialog.locator('button[role="combobox"]').last().click()
    await this.page.locator('[role="option"]').filter({ hasText: agentName }).click()
  }

  async saveAssignment() {
    await this.assignDialog.getByRole('button', { name: 'Save' }).click()
  }

  async cancelAssignment() {
    await this.assignDialog.getByRole('button', { name: 'Cancel' }).click()
    await this.assignDialog.waitFor({ state: 'hidden' })
  }

  async loadMoreHistory() {
    await this.loadMoreButton.click()
  }

  // Toast helpers
  async expectToast(text: string | RegExp) {
    const toast = this.page.locator('[data-sonner-toast]').filter({ hasText: text })
    await expect(toast).toBeVisible({ timeout: 5000 })
    return toast
  }

  // Assertions
  async expectPageVisible() {
    await expect(this.heading).toBeVisible()
  }

  async expectMyTransfersTabVisible() {
    await expect(this.page.getByText('Contacts transferred to you')).toBeVisible()
  }

  async expectQueueTabVisible() {
    await expect(this.page.getByText('Unassigned transfers waiting')).toBeVisible()
  }

  async expectAllActiveTabVisible() {
    await expect(this.page.getByText('All currently active')).toBeVisible()
  }

  async expectHistoryTabVisible() {
    await expect(this.page.getByText('Resumed transfers')).toBeVisible()
  }

  async expectEmptyMyTransfers() {
    await expect(this.page.getByText('No active transfers assigned to you')).toBeVisible()
  }

  async expectEmptyQueue() {
    await expect(this.page.getByText('No transfers in queue')).toBeVisible()
  }

  async expectTransferExists(contactName: string) {
    await expect(this.getTransferRow(contactName)).toBeVisible()
  }

  async expectQueueCount(count: number) {
    const badge = this.queueTab.locator('.badge, span').filter({ hasText: String(count) })
    await expect(badge).toBeVisible()
  }
}
