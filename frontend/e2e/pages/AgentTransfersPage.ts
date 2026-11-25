import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'
import { selectOptionMatching } from '../helpers/select'

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
    this.heading = page.locator('h1').filter({ hasText: /Handover to agents/i })
    this.pickNextButton = page.getByRole('button', { name: /Pick Next/i })
    this.myTransfersTab = page.getByRole('tab', { name: /My Transfers/i })
    this.queueTab = page.getByRole('tab', { name: /Queue/i })
    this.allActiveTab = page.getByRole('tab', { name: /All Active/i })
    this.historyTab = page.getByRole('tab', { name: /History/i })
    this.teamFilterSelect = page.locator('main').getByRole('combobox', { name: /Filter by team/i })
    this.assignDialog = page.locator('dialog[open]')
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
    await selectOptionMatching(this.teamFilterSelect, new RegExp(teamName))
  }

  // Table helpers
  getTransferRow(contactName: string): Locator {
    return this.page.locator('tr').filter({ hasText: contactName })
  }

  /** Rows' "More actions" menu; its items are hidden until it is opened. */
  async openRowMenu(row: Locator) {
    await row.getByRole('button', { name: 'More actions' }).click()
  }

  /** "Chat" is the main row action (My transfers, All active); in Queue it is the "View chat" menu item. */
  async viewChat(contactName: string) {
    const row = this.getTransferRow(contactName)
    const chatButton = row.getByRole('button', { name: 'Chat', exact: true })
    if (await chatButton.count() > 0) {
      await chatButton.click()
    } else {
      await this.openRowMenu(row)
      await row.getByRole('menuitem', { name: 'View chat' }).click()
    }
  }

  async resumeTransfer(contactName: string) {
    const row = this.getTransferRow(contactName)
    await this.openRowMenu(row)
    await row.getByRole('menuitem', { name: 'Resume chatbot' }).click()
    // Handle the ConfirmDialog that appears after clicking resume
    const alertDialog = this.page.getByRole('alertdialog')
    await alertDialog.waitFor({ state: 'visible' })
    await alertDialog.getByRole('button', { name: /confirm|resume|yes/i }).click()
    await alertDialog.waitFor({ state: 'hidden' })
  }

  /** "Assign" is the main row action in Queue and a menu item in All active. */
  async openAssignDialog(contactName: string) {
    const row = this.getTransferRow(contactName)
    const assignButton = row.getByRole('button', { name: 'Assign', exact: true })
    if (await assignButton.count() > 0) {
      await assignButton.click()
    } else {
      await this.openRowMenu(row)
      await row.getByRole('menuitem', { name: 'Assign', exact: true }).click()
    }
    await this.assignDialog.waitFor({ state: 'visible' })
  }

  // Assign dialog helpers
  async selectTeamInDialog(teamName: string) {
    await selectOptionMatching(this.assignDialog.getByRole('combobox').first(), new RegExp(teamName))
  }

  async selectAgentInDialog(agentName: string) {
    await selectOptionMatching(this.assignDialog.getByRole('combobox').last(), new RegExp(agentName))
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
    const toast = this.page.locator('.toast-container output').filter({ hasText: text })
    await expect(toast).toBeVisible({ timeout: 5000 })
    return toast
  }

  // Assertions
  async expectPageVisible() {
    await expect(this.heading).toBeVisible()
  }

  async expectMyTransfersTabVisible() {
    await expect(this.page.getByRole('tab', { name: /My transfers/i, selected: true })).toBeVisible()
  }

  async expectQueueTabVisible() {
    await expect(this.page.getByRole('tab', { name: /Queue/i, selected: true })).toBeVisible()
  }

  async expectAllActiveTabVisible() {
    await expect(this.page.getByRole('tab', { name: /All active/i, selected: true })).toBeVisible()
  }

  async expectHistoryTabVisible() {
    await expect(this.page.getByRole('tab', { name: /History/i, selected: true })).toBeVisible()
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
