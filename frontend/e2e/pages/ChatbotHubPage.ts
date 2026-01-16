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
    // The toggle is a Button with Enable/Disable text, not a switch
    this.toggleButton = page.getByRole('button', { name: /Enable|Disable/i })
    this.statusBadge = page.locator('.border').filter({ hasText: /Active|Inactive/ }).first()
    // Target cards specifically by their full card title (not sidebar links)
    this.keywordsCard = page.getByRole('link', { name: /Keyword Rules.*rules/i })
    this.flowsCard = page.getByRole('link', { name: /Conversation Flows.*flows/i })
    this.aiContextsCard = page.getByRole('link', { name: /AI Contexts.*contexts/i })
    this.statsCards = page.locator('.grid .rounded-lg.border')
  }

  async goto() {
    await this.page.goto('/chatbot')
    await this.page.waitForLoadState('networkidle')
  }

  async toggleChatbot() {
    await this.toggleButton.click()
  }

  async navigateToKeywords() {
    await this.keywordsCard.click()
    await this.page.waitForLoadState('networkidle')
  }

  async navigateToFlows() {
    await this.flowsCard.click()
    await this.page.waitForLoadState('networkidle')
  }

  async navigateToAIContexts() {
    await this.aiContextsCard.click()
    await this.page.waitForLoadState('networkidle')
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

  async expectChatbotEnabled() {
    await expect(this.statusBadge).toContainText('Active')
    await expect(this.toggleButton).toContainText('Disable')
  }

  async expectChatbotDisabled() {
    await expect(this.statusBadge).toContainText('Inactive')
    await expect(this.toggleButton).toContainText('Enable')
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
