import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

export class FlowsPage extends BasePage {
  readonly heading: Locator
  readonly createButton: Locator
  readonly syncButton: Locator
  readonly accountFilter: Locator
  readonly createDialog: Locator
  readonly editDialog: Locator
  readonly alertDialog: Locator

  constructor(page: Page) {
    super(page)
    this.heading = page.getByRole('heading', { name: /WhatsApp Flows/i })
    this.createButton = page.getByRole('button', { name: /Create Flow/i }).first()
    this.syncButton = page.getByRole('button', { name: /Sync from Meta/i }).first()
    this.accountFilter = page.locator('button[role="combobox"]').first()
    // Distinguish dialogs by their title content
    this.createDialog = page.locator('[role="dialog"][data-state="open"]').filter({ hasText: 'Create WhatsApp Flow' })
    this.editDialog = page.locator('[role="dialog"][data-state="open"]').filter({ hasText: 'Edit WhatsApp Flow' })
    this.alertDialog = page.locator('[role="alertdialog"]')
  }

  async goto() {
    await this.page.goto('/flows')
    await this.page.waitForLoadState('networkidle')
  }

  async openCreateDialog() {
    await this.createButton.click()
    await this.createDialog.waitFor({ state: 'visible' })
  }

  // Flow card actions
  getFlowCard(index = 0): Locator {
    return this.page.locator('.rounded-lg.border').nth(index)
  }

  getEditButton(card?: Locator): Locator {
    const container = card || this.page
    // Use lucide class without svg prefix for broader compatibility
    return container.locator('button').filter({ has: this.page.locator('.lucide-pencil') }).first()
  }

  getDeleteButton(card?: Locator): Locator {
    const container = card || this.page
    return container.locator('button').filter({ has: this.page.locator('.lucide-trash-2') }).first()
  }

  getDuplicateButton(card?: Locator): Locator {
    const container = card || this.page
    return container.locator('button').filter({ has: this.page.locator('.lucide-copy') }).first()
  }

  getPreviewButton(card?: Locator): Locator {
    const container = card || this.page
    return container.getByRole('button', { name: /Preview/i }).first()
  }

  getSaveToMetaButton(card?: Locator): Locator {
    const container = card || this.page
    return container.getByRole('button', { name: /Save to Meta|Update on Meta/i }).first()
  }

  getPublishButton(card?: Locator): Locator {
    const container = card || this.page
    return container.getByRole('button', { name: /Publish/i }).first()
  }

  // Dialog interactions
  async clickEditButton() {
    const btn = this.getEditButton()
    if (await btn.isVisible()) {
      await btn.click()
      await this.editDialog.waitFor({ state: 'visible' })
      return true
    }
    return false
  }

  async clickDeleteButton() {
    const btn = this.getDeleteButton()
    if (await btn.isVisible()) {
      await btn.click()
      await this.alertDialog.waitFor({ state: 'visible' })
      return true
    }
    return false
  }

  async clickDuplicateButton() {
    const btn = this.getDuplicateButton()
    if (await btn.isVisible()) {
      await btn.click()
      return true
    }
    return false
  }

  // Alert dialog actions
  async confirmDelete() {
    await this.alertDialog.getByRole('button', { name: /Delete/i }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
  }

  async cancelDelete() {
    await this.alertDialog.getByRole('button', { name: /Cancel/i }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
  }

  // Form helpers
  async fillFlowName(name: string) {
    await this.createDialog.locator('input').filter({ hasText: '' }).first().fill(name)
  }

  async selectAccount(accountName: string) {
    const accountTrigger = this.createDialog.locator('button[role="combobox"]').first()
    await accountTrigger.click()
    await this.page.locator('[role="option"]').filter({ hasText: accountName }).click()
  }

  async selectCategory(category: string) {
    const categoryTriggers = this.createDialog.locator('button[role="combobox"]')
    await categoryTriggers.nth(1).click()
    await this.page.locator('[role="option"]').filter({ hasText: category }).click()
  }

  // Filter by account
  async filterByAccount(accountName: string) {
    await this.accountFilter.click()
    await this.page.locator('[role="option"]').filter({ hasText: accountName }).click()
  }

  // Sync flows
  async syncFlows() {
    await this.syncButton.click()
  }

  // Assertions
  async expectPageVisible() {
    await expect(this.heading).toBeVisible()
  }

  async expectDialogVisible() {
    await expect(this.createDialog).toBeVisible()
  }

  async expectDialogHidden() {
    await expect(this.createDialog).not.toBeVisible()
  }

  async expectAlertDialogVisible() {
    await expect(this.alertDialog).toBeVisible()
  }

  async expectAlertDialogHidden() {
    await expect(this.alertDialog).not.toBeVisible()
  }

  async expectDialogTitle(title: string | RegExp) {
    await expect(this.createDialog).toContainText(title)
  }

  async expectAlertDialogTitle(title: string | RegExp) {
    await expect(this.alertDialog).toContainText(title)
  }

  // Check if button exists
  async hasEditButton(): Promise<boolean> {
    return this.getEditButton().isVisible()
  }

  async hasDeleteButton(): Promise<boolean> {
    return this.getDeleteButton().isVisible()
  }

  async hasDuplicateButton(): Promise<boolean> {
    return this.getDuplicateButton().isVisible()
  }

  async hasPublishButton(): Promise<boolean> {
    return this.getPublishButton().isVisible()
  }

  async hasSaveToMetaButton(): Promise<boolean> {
    return this.getSaveToMetaButton().isVisible()
  }
}

export class ChatbotFlowsPage extends BasePage {
  readonly heading: Locator
  readonly createButton: Locator
  readonly backButton: Locator
  readonly alertDialog: Locator

  constructor(page: Page) {
    super(page)
    this.heading = page.getByRole('heading', { name: /Conversation Flows/i }).first()
    this.createButton = page.getByRole('button', { name: /Create Flow/i }).first()
    this.backButton = page.locator('button').filter({ has: page.locator('.lucide-arrow-left') }).first()
    this.alertDialog = page.locator('[role="alertdialog"]')
  }

  async goto() {
    await this.page.goto('/chatbot/flows')
    await this.page.waitForLoadState('networkidle')
  }

  async gotoNewFlow() {
    await this.page.goto('/chatbot/flows/new')
    await this.page.waitForLoadState('networkidle')
  }

  async clickCreateFlow() {
    await this.createButton.click()
    await expect(this.page).toHaveURL(/\/chatbot\/flows\/new/)
  }

  // Flow card actions
  getFlowCard(index = 0): Locator {
    return this.page.locator('.rounded-lg.border').nth(index)
  }

  getEditButton(card?: Locator): Locator {
    const container = card || this.page
    return container.locator('button').filter({ has: this.page.locator('.lucide-pencil') }).first()
  }

  getDeleteButton(card?: Locator): Locator {
    const container = card || this.page
    return container.locator('button').filter({ has: this.page.locator('.lucide-trash-2') }).first()
  }

  getToggleButton(card?: Locator): Locator {
    const container = card || this.page
    return container.getByRole('button', { name: /Enable|Disable/i }).first()
  }

  // Card interactions
  async clickEditButton() {
    const btn = this.getEditButton()
    if (await btn.isVisible()) {
      await btn.click()
      return true
    }
    return false
  }

  async clickDeleteButton() {
    const btn = this.getDeleteButton()
    if (await btn.isVisible()) {
      await btn.click()
      await this.alertDialog.waitFor({ state: 'visible' })
      return true
    }
    return false
  }

  async clickToggleButton() {
    const btn = this.getToggleButton()
    if (await btn.isVisible()) {
      await btn.click()
      return true
    }
    return false
  }

  // Alert dialog actions
  async confirmDelete() {
    await this.alertDialog.getByRole('button', { name: /Delete/i }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
  }

  async cancelDelete() {
    await this.alertDialog.getByRole('button', { name: /Cancel/i }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
  }

  // Assertions
  async expectPageVisible() {
    await expect(this.heading).toBeVisible()
  }

  async expectAlertDialogVisible() {
    await expect(this.alertDialog).toBeVisible()
  }

  async expectAlertDialogHidden() {
    await expect(this.alertDialog).not.toBeVisible()
  }

  async expectAlertDialogTitle(title: string | RegExp) {
    await expect(this.alertDialog).toContainText(title)
  }

  // Check for flow builder elements
  async expectFlowBuilderVisible() {
    await expect(this.page.getByRole('heading', { name: 'Flow Settings' })).toBeVisible()
  }

  // Check if button exists
  async hasEditButton(): Promise<boolean> {
    return this.getEditButton().isVisible()
  }

  async hasDeleteButton(): Promise<boolean> {
    return this.getDeleteButton().isVisible()
  }

  async hasToggleButton(): Promise<boolean> {
    return this.getToggleButton().isVisible()
  }
}

export class ChatbotFlowBuilderPage extends BasePage {
  // Left panel
  readonly stepsHeading: Locator
  readonly addStepButton: Locator

  // Right panel
  readonly propertiesHeading: Locator

  constructor(page: Page) {
    super(page)
    this.stepsHeading = page.getByRole('heading', { name: 'Steps' })
    this.addStepButton = page.getByRole('button', { name: /^Add$/ })
    this.propertiesHeading = page.getByRole('heading', { name: /Step Properties|Flow Settings/i })
  }

  async gotoNew() {
    await this.page.goto('/chatbot/flows/new')
    await this.page.waitForLoadState('networkidle')
  }

  /** Click "Add" in the steps panel to create a new step */
  async addStep() {
    await this.addStepButton.click()
  }

  /** Click on a step in the left panel by its index (1-based badge) */
  async selectStep(index: number) {
    await this.page.locator('.font-mono').filter({ hasText: String(index) }).click()
  }

  /** Click a message type button in the center preview palette */
  async selectMessageType(type: 'Text' | 'Buttons' | 'API' | 'Flow' | 'Transfer') {
    await this.page.getByRole('button', { name: type, exact: true }).click()
  }

  /** Get the step properties panel (right side) */
  get propertiesPanel() {
    return this.page.locator('div').filter({ has: this.propertiesHeading }).first()
  }

  /** Get the "Button Options" label in step properties */
  get buttonOptionsLabel() {
    return this.page.getByText(/Button Options/i)
  }

  /** Get the "Reply" add-button in the buttons config section */
  get addReplyButton() {
    return this.page.getByRole('button', { name: /^Reply$/ })
  }

  /** Get the "URL" add-button in the buttons config section */
  get addUrlButton() {
    return this.page.getByRole('button', { name: /^URL$/ })
  }

  /** Get a button title input by index (0-based) */
  getButtonTitleInput(index: number) {
    return this.page.getByPlaceholder(/Button Title/i).nth(index)
  }

  /** Get the message text textarea in step properties */
  get messageTextarea() {
    return this.page.getByPlaceholder(/Enter your message/i)
  }

  /** Get the "Go to" select for a button's conditional routing */
  getButtonGoToSelect(index: number) {
    return this.page.getByText(/Go to/i).nth(index)
  }

  /** Get all button config cards in step properties */
  get buttonCards() {
    return this.page.locator('.p-2.border.rounded-md.space-y-2')
  }

  /** Get delete button for a specific button config (0-based) */
  getButtonDeleteButton(index: number) {
    return this.buttonCards.nth(index).locator('button').filter({ has: this.page.locator('.text-destructive') })
  }
}
