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
    // Native select inside <main> (the app shell aside has its own selects)
    this.accountFilter = page.locator('main select').first()
    // Distinguish dialogs by their title content
    this.createDialog = page.locator('dialog[open]').filter({ hasText: 'Create WhatsApp Flow' })
    this.editDialog = page.locator('dialog[open]').filter({ hasText: 'Edit WhatsApp Flow' })
    this.alertDialog = page.getByRole('alertdialog')
  }

  async goto() {
    await this.page.goto('/flows')
    await this.page.waitForLoadState('networkidle')
  }

  async openCreateDialog() {
    await this.createButton.click()
    await this.createDialog.waitFor({ state: 'visible' })
  }

  // Flow row actions: "Edit" is the visible main action; the rest live in the
  // row's "More actions" menu. Menu items are hidden until the menu opens, so the
  // getters use includeHidden (presence checks); open the menu before clicking.
  getFlowCard(index = 0): Locator {
    return this.page.locator('main tbody tr').nth(index)
  }

  getEditButton(card?: Locator): Locator {
    const container = card || this.page.locator('main tbody')
    return container.getByRole('button', { name: 'Edit', exact: true }).first()
  }

  getMenuButton(card?: Locator): Locator {
    const container = card || this.page.locator('main tbody')
    return container.getByRole('button', { name: 'More actions' }).first()
  }

  getDeleteButton(card?: Locator): Locator {
    const container = card || this.page.locator('main tbody')
    return container.getByRole('menuitem', { name: 'Delete', exact: true, includeHidden: true }).first()
  }

  getDuplicateButton(card?: Locator): Locator {
    const container = card || this.page.locator('main tbody')
    return container.getByRole('menuitem', { name: 'Duplicate', includeHidden: true }).first()
  }

  getPreviewButton(card?: Locator): Locator {
    const container = card || this.page.locator('main tbody')
    return container.getByRole('menuitem', { name: 'Preview', includeHidden: true }).first()
  }

  getSaveToMetaButton(card?: Locator): Locator {
    const container = card || this.page.locator('main tbody')
    return container.getByRole('menuitem', { name: /Save to Meta|Update on Meta/i, includeHidden: true }).first()
  }

  getPublishButton(card?: Locator): Locator {
    const container = card || this.page.locator('main tbody')
    return container.getByRole('menuitem', { name: 'Publish', exact: true, includeHidden: true }).first()
  }

  /** Open the "More actions" menu of the first row whose item `name` is enabled. */
  async openRowMenuWith(name: string | RegExp): Promise<Locator | null> {
    const rows = this.page.locator('main tbody tr')
    const n = await rows.count()
    for (let i = 0; i < n; i++) {
      const row = rows.nth(i)
      const trigger = this.getMenuButton(row)
      if (!(await trigger.isVisible())) continue
      const item = row.getByRole('menuitem', { name, exact: typeof name === 'string', includeHidden: true }).first()
      if (await item.count() === 0 || !(await item.isEnabled())) continue
      await trigger.click()
      return item
    }
    return null
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

  /** Delete is disabled for published flows, so use the first row that allows it. */
  async clickDeleteButton() {
    const item = await this.openRowMenuWith('Delete')
    if (!item) return false
    await item.click()
    await this.alertDialog.waitFor({ state: 'visible' })
    return true
  }

  async clickDuplicateButton() {
    const item = await this.openRowMenuWith('Duplicate')
    if (!item) return false
    await item.click()
    return true
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
    await this.createDialog.getByPlaceholder(/Flow name/i).fill(name)
  }

  // Account and category are native selects (account first, then category)
  async selectAccount(accountName: string) {
    await this.createDialog.locator('select').first().selectOption({ label: accountName })
  }

  async selectCategory(category: string) {
    await this.createDialog.locator('select').nth(1).selectOption({ label: category })
  }

  // Filter by account
  async filterByAccount(accountName: string) {
    await this.accountFilter.selectOption({ label: accountName })
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
    return (await this.getDeleteButton().count()) > 0
  }

  async hasDuplicateButton(): Promise<boolean> {
    return (await this.getDuplicateButton().count()) > 0
  }

  async hasPublishButton(): Promise<boolean> {
    return (await this.getPublishButton().count()) > 0
  }

  async hasSaveToMetaButton(): Promise<boolean> {
    return (await this.getSaveToMetaButton().count()) > 0
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
    this.backButton = page.locator('button').filter({ has: page.locator('[data-icon="arrow-left"]') }).first()
    this.alertDialog = page.getByRole('alertdialog')
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

  // Flow row actions: "Edit" is the visible main action, Delete lives in the
  // row's "More actions" menu and the status column holds an enable switch.
  getFlowCard(index = 0): Locator {
    return this.page.locator('main tbody tr').nth(index)
  }

  getEditButton(card?: Locator): Locator {
    const container = card || this.page.locator('main tbody')
    return container.getByRole('button', { name: 'Edit', exact: true }).first()
  }

  getDeleteButton(card?: Locator): Locator {
    const container = card || this.page.locator('main tbody')
    return container.getByRole('menuitem', { name: 'Delete', exact: true, includeHidden: true }).first()
  }

  getToggleButton(card?: Locator): Locator {
    const container = card || this.page.locator('main tbody')
    return container.getByRole('switch').first()
  }

  // Row interactions
  async clickEditButton() {
    const btn = this.getEditButton()
    if (await btn.isVisible()) {
      await btn.click()
      return true
    }
    return false
  }

  async clickDeleteButton() {
    const row = this.getFlowCard(0)
    const trigger = row.getByRole('button', { name: 'More actions' })
    if (!(await trigger.isVisible())) return false
    await trigger.click()
    await this.getDeleteButton(row).click()
    await this.alertDialog.waitFor({ state: 'visible' })
    return true
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
    return (await this.getDeleteButton().count()) > 0
  }

  async hasToggleButton(): Promise<boolean> {
    return this.getToggleButton().isVisible()
  }
}

/**
 * Page object for the graph-native chatbot flow editor.
 *
 * Editor layout (post-refactor):
 *   header → node palette (toolbar) → canvas | right panel
 *
 * The palette adds nodes; clicking a palette button creates a new node
 * of that type *and* selects it, so the right-panel ChatNodeProperties
 * becomes visible. There is no left-side step list anymore.
 */
export class ChatbotFlowBuilderPage extends BasePage {
  readonly paletteToolbar: Locator

  constructor(page: Page) {
    super(page)
    // The palette row is a toolbar labelled by its "Add node" caption.
    this.paletteToolbar = page.getByRole('toolbar', { name: /Add node/ })
  }

  async gotoNew() {
    await this.page.goto('/chatbot/flows/new')
    await this.page.waitForLoadState('networkidle')
  }

  /** Click a palette button to add a new node (and auto-select it). */
  async addNode(type: 'Message' | 'Buttons' | 'API call' | 'WhatsApp Flow' | 'Transfer' | 'Condition' | 'Business hours' | 'Go to flow' | 'End') {
    await this.paletteToolbar.getByRole('button', { name: type, exact: true }).click()
  }

  /** "Button Options ({n}/{max})" label inside the right-panel buttons section. */
  get buttonOptionsLabel() {
    return this.page.getByText(/Button Options/i)
  }

  /** Right-panel "Body" textarea (used by buttons / prompt / transfer / etc.). */
  get bodyTextarea() {
    return this.page.getByPlaceholder(/Message shown above the buttons|Enter your message|Ask the user|Connecting you/i).first()
  }

  /** Message textarea on a Text/message node. */
  get messageTextarea() {
    return this.page.getByPlaceholder(/Enter your message/i)
  }

  /** Reply add-button in the buttons config. */
  get addReplyButton() {
    return this.page.getByRole('button', { name: /^Reply$/ })
  }

  /** URL add-button in the buttons config. */
  get addUrlButton() {
    return this.page.getByRole('button', { name: /^URL$/ })
  }

  /** Phone add-button in the buttons config. */
  get addPhoneButton() {
    return this.page.getByRole('button', { name: /^Phone$/ })
  }

  /** Per-button title input (0-based). */
  getButtonTitleInput(index: number) {
    return this.page.getByPlaceholder(/Button Title/i).nth(index)
  }

  /** All per-button config cards inside the right panel. */
  get buttonCards() {
    return this.page.locator('main [data-button-config]')
  }

  /** Delete button on a per-button card. */
  getButtonDeleteButton(index: number) {
    return this.buttonCards.nth(index).getByRole('button', { name: 'Delete' })
  }
}
