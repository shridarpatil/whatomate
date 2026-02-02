import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

export class TablePage extends BasePage {
  readonly searchInput: Locator
  readonly tableBody: Locator
  readonly tableRows: Locator

  constructor(page: Page) {
    super(page)
    this.searchInput = page.locator('input[placeholder*="Search"], input[placeholder*="search"]')
    this.tableBody = page.locator('tbody')
    this.tableRows = page.locator('tbody tr')
  }

  get addButton(): Locator {
    // Look for Add/Create button - prefer the header one (first)
    return this.page.getByRole('button', { name: /^(Add|Create|New)\s/i }).first()
  }

  async search(query: string) {
    await this.searchInput.fill(query)
    // Wait for search to take effect
    await this.page.waitForTimeout(300)
  }

  async clearSearch() {
    await this.searchInput.clear()
    await this.page.waitForTimeout(300)
  }

  async clickAddButton() {
    await this.addButton.click()
  }

  async getRowCount(): Promise<number> {
    return this.tableRows.count()
  }

  async getRow(text: string): Promise<Locator> {
    return this.tableRows.filter({ hasText: text })
  }

  async rowExists(text: string): Promise<boolean> {
    const row = await this.getRow(text)
    return row.count().then(count => count > 0)
  }

  async clickRowAction(rowText: string, action: string) {
    const row = await this.getRow(rowText)

    // Try multiple strategies to find the action button
    // Strategy 1: Dropdown menu
    const dropdownTrigger = row.locator('button[aria-haspopup="menu"]')
    if (await dropdownTrigger.count() > 0) {
      await dropdownTrigger.click()
      await this.page.locator('[role="menuitem"]').filter({ hasText: action }).click()
      return
    }

    // Strategy 2: Button with text
    const textButton = row.locator('button').filter({ hasText: action })
    if (await textButton.count() > 0) {
      await textButton.click()
      return
    }

    // Strategy 3: Position-based selectors in last column (actions column)
    const actionButtons = row.locator('td:last-child button')
    const buttonCount = await actionButtons.count()

    if (action.toLowerCase() === 'edit') {
      // Edit is typically second-to-last button (before delete), or first if only 2 buttons
      // Common patterns: [edit, delete] or [extra, edit, delete] or [extra1, extra2, edit, delete]
      const editIndex = buttonCount <= 2 ? 0 : buttonCount - 2
      await actionButtons.nth(editIndex).click()
      return
    }

    if (action.toLowerCase() === 'delete') {
      // Delete is typically the last button in actions column
      await actionButtons.last().click()
      return
    }

    throw new Error(`Could not find action button: ${action}`)
  }

  async deleteRow(rowText: string) {
    await this.clickRowAction(rowText, 'Delete')
    // Confirm deletion in alert dialog
    await this.page.locator('[role="alertdialog"] button').filter({ hasText: /Delete|Confirm|Yes/ }).click()
  }

  async editRow(rowText: string) {
    await this.clickRowAction(rowText, 'Edit')
  }

  async expectRowExists(text: string) {
    await expect(this.tableRows.filter({ hasText: text })).toBeVisible()
  }

  async expectRowNotExists(text: string) {
    await expect(this.tableRows.filter({ hasText: text })).not.toBeVisible()
  }

  async expectRowCount(count: number) {
    await expect(this.tableRows).toHaveCount(count)
  }

  async expectEmptyState() {
    // Check for empty state message or no rows
    const emptyMessage = this.page.locator('text=/No .* found|No results|Empty/i')
    const hasEmptyMessage = await emptyMessage.count() > 0
    const rowCount = await this.getRowCount()
    expect(hasEmptyMessage || rowCount === 0).toBeTruthy()
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

  async getColumnValues(columnIndex: number): Promise<string[]> {
    const cells = this.page.locator(`tbody tr td:nth-child(${columnIndex + 1})`)
    const count = await cells.count()
    const values: string[] = []
    for (let i = 0; i < count; i++) {
      const text = await cells.nth(i).textContent()
      values.push(text?.trim() || '')
    }
    return values
  }

  async expectColumnSorted(columnIndex: number, direction: 'asc' | 'desc') {
    const values = await this.getColumnValues(columnIndex)
    const sorted = [...values].sort((a, b) => {
      const comparison = a.localeCompare(b, undefined, { sensitivity: 'base' })
      return direction === 'asc' ? comparison : -comparison
    })
    expect(values).toEqual(sorted)
  }
}
