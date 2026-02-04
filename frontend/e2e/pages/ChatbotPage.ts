import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Keywords Page - Chatbot keyword rules management
 */
export class KeywordsPage extends BasePage {
  readonly heading: Locator
  readonly addButton: Locator
  readonly searchInput: Locator
  readonly dialog: Locator
  readonly alertDialog: Locator
  readonly backButton: Locator

  constructor(page: Page) {
    super(page)
    // Use first() to handle multiple headings (PageHeader + CardTitle)
    this.heading = page.getByRole('heading', { name: 'Keyword Rules' }).first()
    // Use first() since there may be button in both PageHeader and empty state
    this.addButton = page.getByRole('button', { name: /Add Rule/i }).first()
    this.searchInput = page.locator('input[placeholder*="Search"]')
    this.dialog = page.locator('[role="dialog"][data-state="open"]')
    this.alertDialog = page.locator('[role="alertdialog"]')
    this.backButton = page.locator('a[href="/chatbot"] button').first()
  }

  async goto() {
    await this.page.goto('/chatbot/keywords')
    await this.page.waitForLoadState('networkidle')
  }

  async openCreateDialog() {
    await this.addButton.click()
    await this.dialog.waitFor({ state: 'visible' })
  }

  // Form helpers
  async fillKeywordForm(options: {
    keywords: string
    matchType?: 'contains' | 'exact' | 'regex'
    responseType?: 'text' | 'transfer'
    response: string
    priority?: number
    enabled?: boolean
  }) {
    await this.dialog.locator('input#keywords').fill(options.keywords)

    if (options.matchType) {
      await this.selectOption('Match Type', options.matchType)
    }

    if (options.responseType) {
      await this.selectOption('Response Type', options.responseType === 'text' ? 'Text Response' : 'Transfer to Agent')
    }

    await this.dialog.locator('textarea#response').fill(options.response)

    if (options.priority !== undefined) {
      await this.dialog.locator('input#priority').fill(String(options.priority))
    }

    if (options.enabled === false) {
      const switchEl = this.dialog.locator('button[role="switch"]')
      const isChecked = await switchEl.getAttribute('data-state')
      if (isChecked === 'checked') {
        await switchEl.click()
      }
    }
  }

  async addButton_(id: string, title: string) {
    await this.dialog.getByRole('button', { name: /Add Button/i }).click()
    const buttonInputs = this.dialog.locator('.flex.items-center.gap-2').last()
    await buttonInputs.locator('input').first().fill(id)
    await buttonInputs.locator('input').last().fill(title)
  }

  async selectOption(label: string, value: string) {
    const labelLocator = this.dialog.locator('label').filter({ hasText: label })
    const trigger = labelLocator.locator('..').locator('button[role="combobox"]')
    await trigger.click()
    // Wait for dropdown to appear
    await this.page.locator('[role="listbox"]').waitFor({ state: 'visible', timeout: 5000 })
    const option = this.page.locator('[role="option"]').filter({ hasText: value })
    await option.click()
    // Wait for dropdown to close and dialog to still be open
    await this.page.locator('[role="listbox"]').waitFor({ state: 'hidden', timeout: 5000 })
    // Verify dialog is still open
    await this.dialog.waitFor({ state: 'visible', timeout: 5000 })
  }

  async submitDialog(buttonText = 'Create') {
    await this.dialog.getByRole('button', { name: new RegExp(`^${buttonText}$`, 'i') }).click()
  }

  async cancelDialog() {
    await this.dialog.getByRole('button', { name: /Cancel/i }).click()
    await this.dialog.waitFor({ state: 'hidden' })
  }

  // Table helpers (DataTable-based view)
  getRuleCard(keyword: string): Locator {
    // Now uses DataTable - find the row containing the keyword
    return this.page.locator('tbody tr').filter({ hasText: keyword })
  }

  getEditButton(row?: Locator): Locator {
    const container = row || this.page.locator('tbody tr').first()
    // Edit button is the first button in actions column
    return container.locator('td:last-child button').first()
  }

  getDeleteButton(row?: Locator): Locator {
    const container = row || this.page.locator('tbody tr').first()
    // Delete button is the second button in actions column
    return container.locator('td:last-child button').nth(1)
  }

  async editRule(keyword: string) {
    const row = this.getRuleCard(keyword)
    await expect(row).toBeVisible({ timeout: 10000 })
    await this.getEditButton(row).click()
    await this.dialog.waitFor({ state: 'visible' })
  }

  async deleteRule(keyword: string) {
    const row = this.getRuleCard(keyword)
    await expect(row).toBeVisible({ timeout: 10000 })
    await this.getDeleteButton(row).click()
    await this.alertDialog.waitFor({ state: 'visible' })
  }

  async search(term: string) {
    await this.searchInput.fill(term)
    // Wait for debounce (300ms) + API response + render
    await this.page.waitForTimeout(500)
  }

  // Alert dialog actions
  async confirmDelete() {
    await this.alertDialog.getByRole('button', { name: 'Delete' }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
  }

  async cancelDelete() {
    await this.alertDialog.getByRole('button', { name: 'Cancel' }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
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
    // Wait for toast to disappear (either by clicking or auto-dismiss)
    try {
      if (await toast.isVisible({ timeout: 1000 }).catch(() => false)) {
        await toast.click({ timeout: 1000 }).catch(() => {})
      }
    } catch {
      // Toast already dismissed, ignore
    }
    // Wait for toast to be fully hidden
    await toast.waitFor({ state: 'hidden', timeout: 5000 }).catch(() => {})
  }

  // Assertions
  async expectPageVisible() {
    await expect(this.heading).toBeVisible()
  }

  async expectDialogVisible() {
    await expect(this.dialog).toBeVisible()
  }

  async expectDialogHidden() {
    await expect(this.dialog).not.toBeVisible()
  }

  async expectRuleExists(keyword: string) {
    await expect(this.getRuleCard(keyword)).toBeVisible()
  }

  async expectRuleNotExists(keyword: string) {
    await expect(this.getRuleCard(keyword)).not.toBeVisible()
  }

  async expectEmptyState() {
    await expect(this.page.getByText('No keyword rules yet')).toBeVisible()
  }
}

/**
 * AI Contexts Page - Chatbot AI contexts management (DataTable-based)
 */
export class AIContextsPage extends BasePage {
  readonly heading: Locator
  readonly addButton: Locator
  readonly searchInput: Locator
  readonly dialog: Locator
  readonly alertDialog: Locator
  readonly backButton: Locator

  constructor(page: Page) {
    super(page)
    // Use first() to handle multiple headings (PageHeader + CardTitle)
    this.heading = page.getByRole('heading', { name: 'AI Contexts' }).first()
    // Use first() since there may be button in both PageHeader and empty state
    this.addButton = page.getByRole('button', { name: /Add Context/i }).first()
    this.searchInput = page.locator('input[placeholder*="Search"]')
    this.dialog = page.locator('[role="dialog"][data-state="open"]')
    this.alertDialog = page.locator('[role="alertdialog"]')
    this.backButton = page.locator('a[href="/chatbot"] button').first()
  }

  async goto() {
    await this.page.goto('/chatbot/ai')
    await this.page.waitForLoadState('networkidle')
  }

  async openCreateDialog() {
    // Wait for page to be ready and button to be actionable
    await this.page.waitForLoadState('domcontentloaded')
    await this.addButton.waitFor({ state: 'visible', timeout: 10000 })
    await expect(this.addButton).toBeEnabled()
    await this.addButton.click()
    await this.dialog.waitFor({ state: 'visible', timeout: 10000 })
  }

  // Form helpers
  async fillStaticContextForm(options: {
    name: string
    triggerKeywords?: string
    content: string
    priority?: number
    enabled?: boolean
  }) {
    await this.dialog.locator('input#name').fill(options.name)

    if (options.triggerKeywords) {
      await this.dialog.locator('input#trigger_keywords').fill(options.triggerKeywords)
    }

    await this.dialog.locator('textarea#static_content').fill(options.content)

    if (options.priority !== undefined) {
      await this.dialog.locator('input#priority').fill(String(options.priority))
    }

    if (options.enabled === false) {
      const switchEl = this.dialog.locator('button[role="switch"]')
      const isChecked = await switchEl.getAttribute('data-state')
      if (isChecked === 'checked') {
        await switchEl.click()
      }
    }
  }

  async fillApiContextForm(options: {
    name: string
    triggerKeywords?: string
    content?: string
    apiUrl: string
    apiMethod?: 'GET' | 'POST'
    apiHeaders?: string
    apiResponsePath?: string
    priority?: number
    enabled?: boolean
  }) {
    await this.dialog.locator('input#name').fill(options.name)

    // Select API type
    await this.selectOption('Type', 'API Fetch')
    // Wait for API fields to render after type selection (longer timeout for production)
    await this.dialog.locator('input#api_url').waitFor({ state: 'visible', timeout: 10000 })

    if (options.triggerKeywords) {
      await this.dialog.locator('input#trigger_keywords').fill(options.triggerKeywords)
    }

    if (options.content) {
      await this.dialog.locator('textarea#static_content').fill(options.content)
    }

    // API configuration
    if (options.apiMethod) {
      await this.selectOption('Method', options.apiMethod)
    }

    await this.dialog.locator('input#api_url').fill(options.apiUrl)

    if (options.apiHeaders) {
      await this.dialog.locator('textarea#api_headers').fill(options.apiHeaders)
    }

    if (options.apiResponsePath) {
      await this.dialog.locator('input#api_response_path').fill(options.apiResponsePath)
    }

    if (options.priority !== undefined) {
      await this.dialog.locator('input#priority').fill(String(options.priority))
    }

    if (options.enabled === false) {
      const switchEl = this.dialog.locator('button[role="switch"]')
      const isChecked = await switchEl.getAttribute('data-state')
      if (isChecked === 'checked') {
        await switchEl.click()
      }
    }
  }

  async selectOption(label: string, value: string) {
    const labelLocator = this.dialog.locator('label').filter({ hasText: label })
    const trigger = labelLocator.locator('..').locator('button[role="combobox"]')
    await trigger.click()
    // Wait for dropdown to appear
    await this.page.locator('[role="listbox"]').waitFor({ state: 'visible', timeout: 5000 })
    const option = this.page.locator('[role="option"]').filter({ hasText: value })
    await option.click()
    // Wait for dropdown to close and dialog to still be open
    await this.page.locator('[role="listbox"]').waitFor({ state: 'hidden', timeout: 5000 })
    // Verify dialog is still open
    await this.dialog.waitFor({ state: 'visible', timeout: 5000 })
  }

  async submitDialog(buttonText = 'Create') {
    await this.dialog.getByRole('button', { name: new RegExp(`^${buttonText}$`, 'i') }).click()
  }

  async cancelDialog() {
    await this.dialog.getByRole('button', { name: /Cancel/i }).click()
    await this.dialog.waitFor({ state: 'hidden' })
  }

  // Table helpers (DataTable-based view)
  getContextRow(name: string): Locator {
    // Now uses DataTable - find the row containing the context name
    return this.page.locator('tbody tr').filter({ hasText: name })
  }

  getEditButton(row?: Locator): Locator {
    const container = row || this.page.locator('tbody tr').first()
    // Edit button is the first button in actions column
    return container.locator('td:last-child button').first()
  }

  getDeleteButton(row?: Locator): Locator {
    const container = row || this.page.locator('tbody tr').first()
    // Delete button is the second button in actions column
    return container.locator('td:last-child button').nth(1)
  }

  async editContext(name: string) {
    const row = this.getContextRow(name)
    await expect(row).toBeVisible({ timeout: 10000 })
    await this.getEditButton(row).click()
    await this.dialog.waitFor({ state: 'visible' })
  }

  async deleteContext(name: string) {
    const row = this.getContextRow(name)
    await expect(row).toBeVisible({ timeout: 10000 })
    await this.getDeleteButton(row).click()
    await this.alertDialog.waitFor({ state: 'visible' })
  }

  async search(term: string) {
    await this.searchInput.fill(term)
    // Wait for debounce (300ms) + API response + render
    await this.page.waitForTimeout(500)
  }

  // Alert dialog actions
  async confirmDelete() {
    await this.alertDialog.getByRole('button', { name: 'Delete' }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
  }

  async cancelDelete() {
    await this.alertDialog.getByRole('button', { name: 'Cancel' }).click()
    await this.alertDialog.waitFor({ state: 'hidden' })
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
    // Wait for toast to disappear (either by clicking or auto-dismiss)
    try {
      if (await toast.isVisible({ timeout: 1000 }).catch(() => false)) {
        await toast.click({ timeout: 1000 }).catch(() => {})
      }
    } catch {
      // Toast already dismissed, ignore
    }
    // Wait for toast to be fully hidden
    await toast.waitFor({ state: 'hidden', timeout: 5000 }).catch(() => {})
  }

  // Assertions
  async expectPageVisible() {
    await expect(this.heading).toBeVisible()
  }

  async expectDialogVisible() {
    await expect(this.dialog).toBeVisible()
  }

  async expectDialogHidden() {
    await expect(this.dialog).not.toBeVisible()
  }

  async expectContextExists(name: string) {
    // Search for the context to handle pagination
    await this.search(name)
    await expect(this.getContextRow(name)).toBeVisible()
  }

  async expectContextNotExists(name: string) {
    // Search for the context to verify it doesn't exist
    await this.search(name)
    await expect(this.getContextRow(name)).not.toBeVisible()
  }

  async expectEmptyState() {
    await expect(this.page.getByText('No AI contexts yet')).toBeVisible()
  }
}
