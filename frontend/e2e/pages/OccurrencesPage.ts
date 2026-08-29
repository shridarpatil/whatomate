import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Page object for the CRM occurrences feature: the chat-panel widget that
 * opens/lists a contact's cases (ContactOccurrencesPanel.vue), the case
 * detail view (stage + assignee selectors, timeline, note composer), and
 * the Settings > Occurrence Stages pipeline config.
 */
export class OccurrencesPage extends BasePage {
  // Chat panel
  readonly openPanelButton: Locator
  readonly panel: Locator
  readonly titleInput: Locator

  // Detail view
  readonly stageSelect: Locator
  readonly assigneeSelect: Locator
  readonly sendProtocolButton: Locator
  readonly noteInput: Locator
  readonly addNoteButton: Locator

  // Settings > Occurrence Stages
  readonly stagesHeading: Locator
  readonly alertDialog: Locator

  constructor(page: Page) {
    super(page)
    this.openPanelButton = page.locator('#occurrences-button')
    this.panel = page.locator('#occurrences-panel')
    this.titleInput = this.panel.getByPlaceholder('Title')

    // The detail view has exactly two comboboxes, in DOM order: the stage
    // selector (header actions slot, rendered first by DetailPageLayout)
    // and the assignee selector (details card, in the main slot below it).
    this.stageSelect = page.locator('button[role="combobox"]').first()
    this.assigneeSelect = page.locator('button[role="combobox"]').nth(1)
    this.sendProtocolButton = page.getByRole('button', { name: 'Send protocol' })
    this.noteInput = page.getByPlaceholder('Write a note...')
    this.addNoteButton = page.getByRole('button', { name: 'Add note' })

    this.stagesHeading = page.locator('h1').filter({ hasText: 'Occurrence Stages' })
    this.alertDialog = page.locator('[role="alertdialog"]')
  }

  // --- Chat panel ---

  async openPanel() {
    if (!(await this.panel.isVisible())) {
      await this.openPanelButton.click()
      await this.panel.waitFor({ state: 'visible' })
    }
  }

  /** Opens the panel, fills the title and submits. "New occurrence" labels
   * both the header toggle and the form's submit button, so .first()/.last()
   * disambiguate by DOM order rather than text. */
  async createOccurrence(title: string) {
    await this.openPanel()
    await this.panel.getByRole('button', { name: 'New occurrence' }).first().click()
    await this.titleInput.fill(title)
    await this.panel.getByRole('button', { name: 'New occurrence' }).last().click()
  }

  getOccurrenceCard(text: string | RegExp): Locator {
    return this.panel.locator('a').filter({ hasText: text })
  }

  /** Reads the protocol number off a panel card matched by its title. */
  async getProtocolNumber(cardText: string): Promise<string> {
    const card = this.getOccurrenceCard(cardText)
    await expect(card).toBeVisible({ timeout: 10000 })
    return (await card.locator('.font-mono').first().innerText()).trim()
  }

  async openOccurrenceDetail(cardText: string) {
    await this.getOccurrenceCard(cardText).click()
    await this.page.waitForURL(/\/crm\/occurrences\/[0-9a-f-]+$/)
    await this.page.waitForLoadState('networkidle')
  }

  // --- Detail view ---

  async changeStage(stageName: string) {
    await this.stageSelect.click()
    await this.page.locator('[role="option"]').filter({ hasText: stageName }).click()
  }

  async changeAssignee(userName: string) {
    await this.assigneeSelect.click()
    await this.page.locator('[role="option"]').filter({ hasText: userName }).click()
  }

  async removeAssignee() {
    await this.changeAssignee('Unassigned')
  }

  async addNote(content: string) {
    await this.noteInput.fill(content)
    await this.addNoteButton.click()
  }

  get timelineItems(): Locator {
    return this.page.locator('.pl-9.pb-6')
  }

  timelineEntry(text: string | RegExp): Locator {
    return this.timelineItems.filter({ hasText: text })
  }

  // --- Settings > Occurrence Stages ---

  async gotoStagesSettings() {
    await this.page.goto('/settings/occurrence-stages')
    await this.page.waitForLoadState('networkidle')
  }

  getStageRow(name: string): Locator {
    return this.page.locator('tbody tr').filter({ hasText: name })
  }

  /** Clicks the row's delete action (last button in the actions cell) and
   * confirms in the alert dialog. Leaves the dialog open on failure — the
   * app only closes it on a successful delete. */
  async deleteStage(name: string) {
    const row = this.getStageRow(name)
    await expect(row).toBeVisible({ timeout: 10000 })
    await row.locator('td:last-child button').last().click()
    await this.alertDialog.waitFor({ state: 'visible' })
    await this.alertDialog.getByRole('button', { name: /^Delete$/i }).click()
  }

  async expectToast(text: string | RegExp) {
    const toast = this.page.locator('[data-sonner-toast]').filter({ hasText: text })
    await expect(toast).toBeVisible({ timeout: 5000 })
    return toast
  }
}
