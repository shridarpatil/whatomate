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

    // Strategy 3: Icon button with tooltip (hover to show tooltip, then find by tooltip content)
    // For Edit - look for Pencil icon button
    // For Delete - look for Trash icon button
    if (action.toLowerCase() === 'edit') {
      const editButton = row.locator('button').filter({ has: this.page.locator('svg.lucide-pencil, svg[class*="pencil"]') })
      if (await editButton.count() > 0) {
        await editButton.click()
        return
      }
      // Fallback: first small icon button that's not delete
      const iconButtons = row.locator('button[class*="icon"], button:has(svg)').first()
      await iconButtons.click()
      return
    }

    if (action.toLowerCase() === 'delete') {
      const deleteButton = row.locator('button').filter({ has: this.page.locator('svg.lucide-trash, svg[class*="trash"]') })
      if (await deleteButton.count() > 0) {
        await deleteButton.click()
        return
      }
      // Fallback: look for button with destructive styling
      const destructiveButton = row.locator('button:has(svg[class*="destructive"]), button svg.text-destructive').locator('..')
      if (await destructiveButton.count() > 0) {
        await destructiveButton.click()
        return
      }
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
}
