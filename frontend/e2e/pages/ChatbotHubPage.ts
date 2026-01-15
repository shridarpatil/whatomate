import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Chatbot Hub Page - Main chatbot overview page
 */
export class ChatbotHubPage extends BasePage {
  readonly heading: Locator
  readonly enableSwitch: Locator
  readonly keywordsCard: Locator
  readonly flowsCard: Locator
  readonly aiContextsCard: Locator
  readonly transfersCard: Locator
  readonly statsCards: Locator

  constructor(page: Page) {
    super(page)
    this.heading = page.locator('h1').filter({ hasText: 'Chatbot' })
    this.enableSwitch = page.locator('button[role="switch"]')
    this.keywordsCard = page.locator('a[href="/chatbot/keywords"]')
    this.flowsCard = page.locator('a[href="/chatbot/flows"]')
    this.aiContextsCard = page.locator('a[href="/chatbot/ai"]')
    this.transfersCard = page.locator('a[href="/chatbot/transfers"]')
    this.statsCards = page.locator('.grid .rounded-lg.border')
  }

  async goto() {
    await this.page.goto('/chatbot')
    await this.page.waitForLoadState('networkidle')
  }

  async toggleChatbot() {
    await this.enableSwitch.click()
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

  async navigateToTransfers() {
    await this.transfersCard.click()
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
    await expect(this.enableSwitch).toHaveAttribute('data-state', 'checked')
  }

  async expectChatbotDisabled() {
    await expect(this.enableSwitch).toHaveAttribute('data-state', 'unchecked')
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
