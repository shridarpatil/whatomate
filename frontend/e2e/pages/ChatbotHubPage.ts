import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Chatbot Hub Page - Main chatbot overview page
 */
export class ChatbotHubPage extends BasePage {
  readonly heading: Locator
  readonly toggleButton: Locator
  readonly statusBadge: Locator
  readonly keywordsCard: Locator
  readonly flowsCard: Locator
  readonly aiContextsCard: Locator
  readonly statsCards: Locator

  constructor(page: Page) {
    super(page)
    this.heading = page.locator('h1').filter({ hasText: 'Chatbot' })
    // Off: a "Turn on chatbot" button in the warning banner. On: an "Active" badge and "Disable" in the header.
    this.toggleButton = page.locator('main').getByRole('button', { name: /Turn on chatbot|Disable/i })
    this.statusBadge = page.locator('main header .badge').filter({ hasText: /^\s*Active\s*$/ }).first()
    // The "Get started" checklist: one step per area (title, count summary, "Open ..." link)
    const step = (link: RegExp) => page.locator('main [data-steps] li').filter({ has: page.getByRole('link', { name: link }) })
    this.keywordsCard = step(/^Open keyword rules$/i)
    this.flowsCard = step(/^Open bot flows$/i)
    this.aiContextsCard = step(/^Open AI contexts$/i)
    this.statsCards = page.locator('main article.card')
  }

  async goto() {
    await this.page.goto('/chatbot')
    await this.page.waitForLoadState('networkidle')
  }

  async toggleChatbot() {
    await this.toggleButton.click()
    // A ConfirmDialog (AlertDialog) appears before the toggle takes effect.
    const dialog = this.page.getByRole('alertdialog')
    await expect(dialog).toBeVisible()
    // The confirm dialog's button is labelled "Enable" or "Disable".
    await dialog.getByRole('button', { name: /Enable|Disable/i }).click()
    // Wait for the dialog to close after the action completes.
    await expect(dialog).toBeHidden()
  }

  async navigateToKeywords() {
    await this.keywordsCard.getByRole('link').click()
    await this.page.waitForLoadState('networkidle')
  }

  async navigateToFlows() {
    await this.flowsCard.getByRole('link').click()
    await this.page.waitForLoadState('networkidle')
  }

  async navigateToAIContexts() {
    await this.aiContextsCard.getByRole('link').click()
    await this.page.waitForLoadState('networkidle')
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

  async expectChatbotEnabled() {
    await expect(this.statusBadge).toContainText('Active')
    await expect(this.toggleButton).toContainText('Disable')
  }

  async expectChatbotDisabled() {
    await expect(this.page.locator('main [role="alert"]')).toContainText('The chatbot is off')
    await expect(this.toggleButton).toContainText('Turn on chatbot')
  }

  async expectNavigationCardsVisible() {
    await expect(this.keywordsCard).toBeVisible()
    await expect(this.flowsCard).toBeVisible()
    await expect(this.aiContextsCard).toBeVisible()
  }

  async expectStatsVisible() {
    await expect(this.statsCards.first()).toBeVisible()
  }
}
