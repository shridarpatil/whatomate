import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Chatbot Settings Page - Chatbot configuration
 */
export class ChatbotSettingsPage extends BasePage {
  readonly heading: Locator
  readonly messagesTab: Locator
  readonly agentsTab: Locator
  readonly hoursTab: Locator
  readonly slaTab: Locator
  readonly aiTab: Locator
  readonly saveButton: Locator

  constructor(page: Page) {
    super(page)
    this.heading = page.locator('h1').filter({ hasText: 'Chatbot Settings' })
    this.messagesTab = page.getByRole('tab', { name: /Messages/i })
    this.agentsTab = page.getByRole('tab', { name: /Agents/i })
    this.hoursTab = page.getByRole('tab', { name: /Hours/i })
    this.slaTab = page.getByRole('tab', { name: /SLA/i })
    this.aiTab = page.getByRole('tab', { name: /AI/i })
    this.saveButton = page.getByRole('button', { name: /Save Changes/i })
  }

  async goto() {
    await this.page.goto('/settings/chatbot')
    await this.page.waitForLoadState('networkidle')
  }

  // Tab navigation
  async switchToMessagesTab() {
    await this.messagesTab.click()
  }

  async switchToAgentsTab() {
    await this.agentsTab.click()
  }

  async switchToHoursTab() {
    await this.hoursTab.click()
  }

  async switchToSLATab() {
    await this.slaTab.click()
  }

  async switchToAITab() {
    await this.aiTab.click()
  }

  // Messages tab helpers
  async fillGreetingMessage(message: string) {
    await this.page.locator('textarea#greeting').fill(message)
  }

  async fillFallbackMessage(message: string) {
    await this.page.locator('textarea#fallback').fill(message)
  }

  async setSessionTimeout(minutes: number) {
    await this.page.locator('input#timeout').fill(String(minutes))
  }

  async addGreetingButton(title: string) {
    await this.page.getByRole('button', { name: /Add Button/i }).first().click()
    const inputs = this.page.locator('.flex.items-center.gap-2 input')
    await inputs.last().fill(title)
  }

  async addFallbackButton(title: string) {
    await this.page.getByRole('button', { name: /Add Button/i }).last().click()
    const inputs = this.page.locator('.flex.items-center.gap-2 input')
    await inputs.last().fill(title)
  }

  // Agents tab helpers
  async toggleAgentQueuePickup() {
    await this.page.locator('button[role="switch"]').first().click()
  }

  async toggleAssignToSameAgent() {
    await this.page.locator('button[role="switch"]').nth(1).click()
  }

  async toggleAgentCurrentConversationOnly() {
    await this.page.locator('button[role="switch"]').nth(2).click()
  }

  // Hours tab helpers
  async toggleBusinessHours() {
    await this.page.locator('button[role="switch"]').first().click()
  }

  async setDayHours(dayIndex: number, enabled: boolean, startTime?: string, endTime?: string) {
    const dayRow = this.page.locator('.flex.items-center.gap-4').nth(dayIndex)
    const switchEl = dayRow.locator('button[role="switch"]')
    const currentState = await switchEl.getAttribute('data-state')

    if ((enabled && currentState === 'unchecked') || (!enabled && currentState === 'checked')) {
      await switchEl.click()
    }

    if (enabled && startTime) {
      await dayRow.locator('input[type="time"]').first().fill(startTime)
    }
    if (enabled && endTime) {
      await dayRow.locator('input[type="time"]').last().fill(endTime)
    }
  }

  async fillOutOfHoursMessage(message: string) {
    await this.page.locator('textarea').filter({ hasText: '' }).last().fill(message)
  }

  async toggleAllowAutomatedOutsideHours() {
    await this.page.locator('button[role="switch"]').last().click()
  }

  // SLA tab helpers
  async toggleSLAEnabled() {
    await this.page.locator('button[role="switch"]').first().click()
  }

  async setSLAResponseMinutes(minutes: number) {
    await this.page.locator('input[type="number"]').first().fill(String(minutes))
  }

  async setSLAEscalationMinutes(minutes: number) {
    await this.page.locator('input[type="number"]').nth(1).fill(String(minutes))
  }

  async setSLAResolutionMinutes(minutes: number) {
    await this.page.locator('input[type="number"]').nth(2).fill(String(minutes))
  }

  async setSLAAutoCloseHours(hours: number) {
    await this.page.locator('input[type="number"]').nth(3).fill(String(hours))
  }

  async fillAutoCloseMessage(message: string) {
    await this.page.locator('textarea').first().fill(message)
  }

  async toggleClientReminder() {
    // Find the Client Inactivity Reminders switch
    const switches = this.page.locator('button[role="switch"]')
    await switches.nth(1).click() // Second switch in SLA tab
  }

  // AI tab helpers
  async toggleAIEnabled() {
    await this.page.locator('button[role="switch"]').first().click()
  }

  async selectAIProvider(provider: string) {
    await this.page.locator('button[role="combobox"]').first().click()
    await this.page.locator('[role="option"]').filter({ hasText: provider }).click()
  }

  async selectAIModel(model: string) {
    await this.page.locator('button[role="combobox"]').nth(1).click()
    await this.page.locator('[role="option"]').filter({ hasText: model }).click()
  }

  async fillAPIKey(key: string) {
    await this.page.locator('input[type="password"]').fill(key)
  }

  async setMaxTokens(tokens: number) {
    await this.page.locator('input[type="number"]').first().fill(String(tokens))
  }

  async fillSystemPrompt(prompt: string) {
    await this.page.locator('textarea').fill(prompt)
  }

  async saveSettings() {
    await this.saveButton.click()
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

  async expectMessagesTabVisible() {
    await expect(this.page.locator('textarea#greeting')).toBeVisible()
  }

  async expectAgentsTabVisible() {
    await expect(this.page.getByText('Allow Agents to Pick from Queue')).toBeVisible()
  }

  async expectHoursTabVisible() {
    await expect(this.page.getByText('Enable Business Hours')).toBeVisible()
  }

  async expectSLATabVisible() {
    await expect(this.page.getByText('Enable SLA Tracking')).toBeVisible()
  }

  async expectAITabVisible() {
    await expect(this.page.getByText('Enable AI Responses')).toBeVisible()
  }
}
