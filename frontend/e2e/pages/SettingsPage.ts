import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Base class for settings pages that use cards/grid layout
 */
export class CardGridPage extends BasePage {
  readonly heading: Locator
  readonly addButton: Locator
  readonly searchInput: Locator
  readonly categoryFilter: Locator
  readonly dialog: Locator
  readonly alertDialog: Locator

  constructor(page: Page, options: { headingText: string; addButtonText: string }) {
    super(page)
    // Use first() to handle multiple headings (PageHeader + CardTitle)
    this.heading = page.locator('h1').filter({ hasText: options.headingText }).first()
    this.addButton = page.getByRole('button', { name: new RegExp(options.addButtonText, 'i') }).first()
    this.searchInput = page.locator('input[placeholder*="Search"]')
    this.categoryFilter = page.locator('button[role="combobox"]').first()
    this.dialog = page.locator('[role="dialog"][data-state="open"]')
    this.alertDialog = page.locator('[role="alertdialog"]')
  }

  async openCreateDialog() {
    await this.addButton.click()
    await this.dialog.waitFor({ state: 'visible' })
  }

  async search(term: string) {
    await this.searchInput.fill(term)
    await this.page.waitForLoadState('networkidle')
  }

  async filterByCategory(category: string) {
    await this.categoryFilter.click()
    await this.page.locator('[role="option"]').filter({ hasText: category }).click()
    await this.page.waitForLoadState('networkidle')
  }

  // Dialog helpers
  getDialogInput(index = 0): Locator {
    return this.dialog.locator('input').nth(index)
  }

  getDialogTextarea(): Locator {
    return this.dialog.locator('textarea')
  }

  getDialogCombobox(): Locator {
    return this.dialog.locator('button[role="combobox"]')
  }

  async submitDialog(buttonText = 'Create') {
    await this.dialog.getByRole('button', { name: new RegExp(`^${buttonText}$`, 'i') }).click()
  }

  async cancelDialog() {
    await this.dialog.getByRole('button', { name: /Cancel/i }).click()
    await this.dialog.waitFor({ state: 'hidden' })
  }

  // Card helpers
  getCardByHeading(heading: string): Locator {
    return this.page.getByRole('heading', { name: heading })
      .locator('xpath=ancestor::div[contains(@class, "rounded")]').first()
  }

  async clickCardButton(heading: string, buttonIndex: number) {
    const card = this.getCardByHeading(heading)
    await card.locator('button').nth(buttonIndex).click()
  }

  // Alert dialog helpers
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
    if (await toast.isVisible()) {
      await toast.click()
    }
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
}

/**
 * Base class for settings pages that use table layout
 */
export class TableSettingsPage extends BasePage {
  readonly heading: Locator
  readonly addButton: Locator
  readonly searchInput: Locator
  readonly table: Locator
  readonly dialog: Locator
  readonly alertDialog: Locator

  constructor(page: Page, options: { headingText: string; addButtonText: string }) {
    super(page)
    // Use first() to handle multiple headings and buttons (PageHeader + CardTitle + empty state)
    this.heading = page.locator('h1').filter({ hasText: options.headingText }).first()
    this.addButton = page.getByRole('button', { name: new RegExp(options.addButtonText, 'i') }).first()
    this.searchInput = page.locator('input[placeholder*="Search"]')
    this.table = page.locator('table')
    this.dialog = page.locator('[role="dialog"][data-state="open"]')
    this.alertDialog = page.locator('[role="alertdialog"]')
  }

  async openCreateDialog() {
    await this.addButton.click()
    await this.dialog.waitFor({ state: 'visible' })
  }

  async search(term: string) {
    await this.searchInput.fill(term)
    await this.page.waitForTimeout(300)
  }

  // Table helpers
  getRow(text: string): Locator {
    return this.page.locator('tr').filter({ hasText: text })
  }

  async clickRowButton(rowText: string, buttonIndex: number) {
    const row = this.getRow(rowText)
    await row.locator('td').last().locator('button').nth(buttonIndex).click()
  }

  async editRow(rowText: string) {
    await this.clickRowButton(rowText, 0) // Edit is usually first button
    await this.dialog.waitFor({ state: 'visible' })
  }

  async deleteRow(rowText: string) {
    await this.clickRowButton(rowText, 1) // Delete is usually second button
    await this.alertDialog.waitFor({ state: 'visible' })
  }

  async toggleRowSwitch(rowText: string) {
    const row = this.getRow(rowText)
    await row.locator('button[role="switch"]').click()
  }

  // Dialog helpers
  getDialogInput(id: string): Locator {
    return this.dialog.locator(`input#${id}`)
  }

  getDialogTextarea(id: string): Locator {
    return this.dialog.locator(`textarea#${id}`)
  }

  getDialogRadio(name: string): Locator {
    return this.dialog.getByRole('radio', { name: new RegExp(name, 'i') })
  }

  async submitDialog(buttonText = 'Create') {
    await this.dialog.getByRole('button', { name: new RegExp(`^${buttonText}$`, 'i') }).click()
  }

  async cancelDialog() {
    await this.dialog.getByRole('button', { name: /Cancel/i }).click()
    await this.dialog.waitFor({ state: 'hidden' })
  }

  // Alert dialog helpers
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
    if (await toast.isVisible()) {
      await toast.click()
    }
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

  async expectRowExists(text: string) {
    await expect(this.table).toContainText(text)
  }

  async expectRowNotExists(text: string) {
    await expect(this.table).not.toContainText(text)
  }

  // Sorting helpers
  getColumnHeader(columnName: string): Locator {
    return this.page.locator('thead th').filter({ hasText: columnName })
  }

  async clickColumnHeader(columnName: string) {
    await this.getColumnHeader(columnName).click()
    await this.page.waitForTimeout(300)
  }

  async getSortDirection(columnName: string): Promise<'asc' | 'desc' | null> {
    const header = this.getColumnHeader(columnName)
    // Lucide icons render with class like 'lucide-arrow-up-icon'
    const arrowUp = header.locator('.lucide-arrow-up-icon')
    const arrowDown = header.locator('.lucide-arrow-down-icon')

    if (await arrowUp.count() > 0) return 'asc'
    if (await arrowDown.count() > 0) return 'desc'
    return null
  }

  async expectSortDirection(columnName: string, direction: 'asc' | 'desc') {
    const actual = await this.getSortDirection(columnName)
    expect(actual).toBe(direction)
  }
}

/**
 * Canned Responses Page (DataTable-based)
 */
export class CannedResponsesPage extends TableSettingsPage {
  readonly categoryFilter: Locator

  constructor(page: Page) {
    super(page, { headingText: 'Canned Responses', addButtonText: 'Add Response' })
    // Category filter is now the second combobox (after search)
    this.categoryFilter = page.locator('button[role="combobox"]').first()
  }

  async goto() {
    await this.page.goto('/settings/canned-responses')
    await this.page.waitForLoadState('networkidle')
  }

  async search(term: string) {
    await this.searchInput.fill(term)
    await this.page.waitForTimeout(300)
  }

  async filterByCategory(category: string) {
    await this.categoryFilter.click()
    await this.page.locator('[role="option"]').filter({ hasText: category }).click()
    await this.page.waitForTimeout(300)
  }

  async fillResponseForm(name: string, content: string, shortcut?: string, category?: string) {
    await this.dialog.locator('input').first().fill(name)
    if (shortcut) {
      await this.dialog.locator('input').nth(1).fill(shortcut)
    }
    await this.dialog.locator('textarea').fill(content)
    if (category) {
      await this.dialog.locator('button[role="combobox"]').click()
      await this.page.locator('[role="option"]').filter({ hasText: category }).click()
    }
  }

  // Table helpers - buttons in order: copy, edit, delete
  getResponseRow(name: string): Locator {
    return this.page.locator('tbody tr').filter({ hasText: name })
  }

  async copyResponse(name: string) {
    const row = this.getResponseRow(name)
    await expect(row).toBeVisible({ timeout: 10000 })
    // Copy is first button in actions column
    await row.locator('td:last-child button').first().click()
  }

  async editResponse(name: string) {
    const row = this.getResponseRow(name)
    await expect(row).toBeVisible({ timeout: 10000 })
    // Edit is second button in actions column
    await row.locator('td:last-child button').nth(1).click()
    await this.dialog.waitFor({ state: 'visible' })
  }

  async deleteResponse(name: string) {
    const row = this.getResponseRow(name)
    await expect(row).toBeVisible({ timeout: 10000 })
    // Delete is third button in actions column
    await row.locator('td:last-child button').nth(2).click()
    await this.alertDialog.waitFor({ state: 'visible' })
  }

  async expectResponseExists(name: string) {
    await expect(this.getResponseRow(name)).toBeVisible()
  }

  async expectResponseNotExists(name: string) {
    await expect(this.getResponseRow(name)).not.toBeVisible()
  }

  // Dialog helpers for tests
  getDialogInput(index: number): Locator {
    return this.dialog.locator('input').nth(index)
  }

  getDialogTextarea(): Locator {
    return this.dialog.locator('textarea')
  }
}

/**
 * Custom Actions Page
 */
export class CustomActionsPage extends TableSettingsPage {
  constructor(page: Page) {
    super(page, { headingText: 'Custom Actions', addButtonText: 'Add Action' })
    // Override heading to use text locator since this page uses CardTitle not h1
    this.heading = page.locator('text=Custom Actions').first()
  }

  async goto() {
    await this.page.goto('/settings/custom-actions')
    await this.page.waitForLoadState('networkidle')
  }

  async fillWebhookAction(name: string, url: string) {
    await this.getDialogInput('name').fill(name)
    await this.getDialogInput('url').fill(url)
  }

  async fillUrlAction(name: string, url: string) {
    await this.getDialogInput('name').fill(name)
    await this.getDialogRadio('Open URL').click()
    await this.getDialogInput('url').fill(url)
  }

  async fillJsAction(name: string, code: string) {
    await this.getDialogInput('name').fill(name)
    await this.getDialogRadio('JavaScript').click()
    await this.getDialogTextarea('code').fill(code)
  }
}

/**
 * API Keys Page
 */
export class ApiKeysPage extends TableSettingsPage {
  constructor(page: Page) {
    super(page, { headingText: 'API Keys', addButtonText: 'Create API Key' })
  }

  async goto() {
    await this.page.goto('/settings/api-keys')
    await this.page.waitForLoadState('networkidle')
  }

  async fillApiKeyForm(name: string, expiry?: string) {
    await this.page.locator('input#name').fill(name)
    if (expiry) {
      await this.page.locator('input#expiry').fill(expiry)
    }
  }

  async submitDialog(buttonText = 'Create Key') {
    await this.dialog.getByRole('button', { name: new RegExp(buttonText, 'i') }).click()
  }

  async expectKeyCreatedDialog() {
    await expect(this.dialog).toContainText('API Key Created')
    await expect(this.dialog).toContainText('whm_')
  }

  async closeKeyCreatedDialog() {
    await this.page.getByRole('button', { name: 'Done' }).click()
  }
}

/**
 * Tags Page
 */
export class TagsPage extends TableSettingsPage {
  readonly colorSelect: Locator

  constructor(page: Page) {
    super(page, { headingText: 'Tags', addButtonText: 'Add Tag' })
    this.colorSelect = page.locator('button[role="combobox"]')
  }

  async goto() {
    await this.page.goto('/settings/tags')
    await this.page.waitForLoadState('networkidle')
  }

  async fillTagForm(name: string, color?: string) {
    await this.dialog.locator('input').first().fill(name)
    if (color) {
      await this.dialog.locator('button[role="combobox"]').click()
      await this.page.locator('[role="option"]').filter({ hasText: color }).click()
    }
  }

  async selectColor(color: string) {
    await this.dialog.locator('button[role="combobox"]').click()
    await this.page.locator('[role="option"]').filter({ hasText: color }).click()
  }

  async expectTagBadgeVisible(name: string) {
    await expect(this.page.locator('span').filter({ hasText: name })).toBeVisible()
  }
}

/**
 * Contacts Page
 */
export class ContactsPage extends TableSettingsPage {
  readonly importExportButton: Locator
  readonly importExportDialog: Locator

  constructor(page: Page) {
    super(page, { headingText: 'Contacts', addButtonText: 'Add Contact' })
    this.importExportButton = page.getByRole('button', { name: /Import.*Export/i })
    this.importExportDialog = page.locator('[role="dialog"][data-state="open"]')
  }

  async goto() {
    await this.page.goto('/settings/contacts')
    await this.page.waitForLoadState('networkidle')
  }

  // Contact form helpers
  async fillContactForm(phoneNumber: string, name?: string, account?: string) {
    await this.dialog.locator('input').first().fill(phoneNumber)
    if (name) {
      await this.dialog.locator('input').nth(1).fill(name)
    }
    if (account) {
      await this.dialog.locator('button[role="combobox"]').first().click()
      await this.page.locator('[role="option"]').filter({ hasText: account }).click()
    }
  }

  async fillEditForm(name?: string) {
    if (name) {
      // In edit mode, phone is disabled, name is second input
      await this.dialog.locator('input').nth(1).fill(name)
    }
  }

  // Tag selection in form
  async selectTags(tags: string[]) {
    // Open tag selector popover
    const tagButton = this.dialog.locator('button[role="combobox"]').last()
    await tagButton.click()

    for (const tag of tags) {
      await this.page.locator('[role="option"]').filter({ hasText: tag }).click()
    }

    // Close by clicking outside
    await this.dialog.locator('h2').click()
  }

  // Table helpers - buttons in order: chat, edit, delete
  getContactRow(identifier: string): Locator {
    return this.page.locator('tbody tr').filter({ hasText: identifier })
  }

  async openChat(identifier: string) {
    const row = this.getContactRow(identifier)
    await expect(row).toBeVisible({ timeout: 10000 })
    // Chat is first button in actions column
    await row.locator('td:last-child button').first().click()
  }

  async editContact(identifier: string) {
    const row = this.getContactRow(identifier)
    await expect(row).toBeVisible({ timeout: 10000 })
    // Edit is second button in actions column
    await row.locator('td:last-child button').nth(1).click()
    await this.dialog.waitFor({ state: 'visible' })
  }

  async deleteContact(identifier: string) {
    const row = this.getContactRow(identifier)
    await expect(row).toBeVisible({ timeout: 10000 })
    // Delete is third button in actions column
    await row.locator('td:last-child button').nth(2).click()
    await this.alertDialog.waitFor({ state: 'visible' })
  }

  async expectContactExists(identifier: string) {
    await expect(this.getContactRow(identifier)).toBeVisible()
  }

  async expectContactNotExists(identifier: string) {
    await expect(this.getContactRow(identifier)).not.toBeVisible()
  }

  // Import/Export helpers
  async openImportExportDialog() {
    await this.importExportButton.click()
    await this.importExportDialog.waitFor({ state: 'visible' })
  }

  async switchToImportTab() {
    await this.importExportDialog.getByRole('tab', { name: /Import/i }).click()
  }

  async switchToExportTab() {
    await this.importExportDialog.getByRole('tab', { name: /Export/i }).click()
  }

  async selectExportColumn(columnName: string) {
    await this.importExportDialog.locator('label').filter({ hasText: columnName }).click()
  }

  async clickExportButton() {
    await this.importExportDialog.getByRole('button', { name: /Export CSV/i }).click()
  }

  async uploadImportFile(filePath: string) {
    await this.importExportDialog.locator('input[type="file"]').setInputFiles(filePath)
  }

  async toggleUpdateOnDuplicate() {
    await this.importExportDialog.locator('button[role="checkbox"]').click()
  }

  async clickImportButton() {
    await this.importExportDialog.getByRole('button', { name: /Import CSV/i }).click()
    // Wait for import to complete - look for "Import Complete" text
    await this.importExportDialog.locator('text=Import Complete').waitFor({ state: 'visible', timeout: 30000 })
  }

  async expectImportResult(created: number, updated: number) {
    await expect(this.importExportDialog).toContainText(`Created: ${created}`, { timeout: 10000 })
    await expect(this.importExportDialog).toContainText(`Updated: ${updated}`, { timeout: 10000 })
  }

  async closeImportExportDialog() {
    await this.importExportDialog.getByRole('button', { name: /Cancel/i }).click()
    await this.importExportDialog.waitFor({ state: 'hidden' })
  }
}
