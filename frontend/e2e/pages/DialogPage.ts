import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

export class DialogPage extends BasePage {
  readonly dialog: Locator
  readonly cancelButton: Locator
  readonly closeButton: Locator

  constructor(page: Page) {
    super(page)
    // Native <dialog> (Modal) has the implicit dialog role; legacy markup uses role="dialog" explicitly.
    this.dialog = page.locator('dialog[open], [role="dialog"]').first()
    // Be specific - Cancel button (not the X close button)
    this.cancelButton = this.dialog.getByRole('button', { name: 'Cancel', exact: true })
    this.closeButton = this.dialog.getByRole('button', { name: 'Close', exact: true })
  }

  get submitButton(): Locator {
    // Look for the primary action button - usually at the bottom of dialog
    // Match buttons that START with Save/Create/Submit/Update (e.g., "Create Team", "Update User")
    return this.dialog.getByRole('button', { name: /^(Save|Create|Submit|Update)/i }).last()
  }

  async isOpen(): Promise<boolean> {
    return this.dialog.isVisible()
  }

  async waitForOpen() {
    await this.dialog.waitFor({ state: 'visible' })
  }

  async waitForClose() {
    await this.dialog.waitFor({ state: 'hidden' })
  }

  async fillField(label: string, value: string) {
    // Try multiple strategies to find the input
    const labelLocator = this.dialog.locator(`label`).filter({ hasText: label })

    // Strategy 0: Oat markup - the input is nested in <label data-field>
    const nestedInput = labelLocator.locator('input:not([type="checkbox"]):not([type="radio"]), textarea')
    if (await nestedInput.count() > 0) {
      await nestedInput.first().fill(value)
      return
    }

    // Strategy 1: Input is sibling of label
    const siblingInput = labelLocator.locator('~ input, ~ textarea')
    if (await siblingInput.count() > 0) {
      await siblingInput.fill(value)
      return
    }

    // Strategy 2: Input is inside a parent container with label
    const containerInput = labelLocator.locator('..').locator('input, textarea')
    if (await containerInput.count() > 0) {
      await containerInput.first().fill(value)
      return
    }

    // Strategy 3: Find by id matching label's for attribute
    const labelFor = await labelLocator.getAttribute('for')
    if (labelFor) {
      await this.dialog.locator(`#${labelFor}`).fill(value)
      return
    }

    // Strategy 4: Find input with placeholder matching label
    const placeholderInput = this.dialog.locator(`input[placeholder*="${label}" i], textarea[placeholder*="${label}" i]`)
    if (await placeholderInput.count() > 0) {
      await placeholderInput.fill(value)
      return
    }

    // Strategy 5: Find input with name matching label
    const nameInput = this.dialog.locator(`input[name*="${label.toLowerCase()}"], textarea[name*="${label.toLowerCase()}"]`)
    if (await nameInput.count() > 0) {
      await nameInput.fill(value)
      return
    }

    throw new Error(`Could not find input for label: ${label}`)
  }

  /** Alert dialog (Modal with `alert`). */
  get alertDialog(): Locator {
    return this.page.getByRole('alertdialog')
  }

  /**
   * Fill a select by its label: native <select> (selectOption) or Combobox
   * (click the trigger, click the option).
   */
  async fillSelect(label: string, option: string) {
    const native = this.dialog.getByLabel(label, { exact: false }).and(this.page.locator('select'))
    if (await native.count() > 0) {
      const select = native.first()
      // Native options show raw text (e.g. role "agent (System)"), so fall back to a
      // case-insensitive match ignoring a trailing "(…)" suffix when there is no exact label.
      const labels = (await select.locator('option').allTextContents()).map(l => l.trim())
      const base = (l: string) => l.replace(/\s*\([^)]*\)\s*$/, '').toLowerCase()
      const label = labels.find(l => l === option)
        ?? labels.find(l => base(l) === option.toLowerCase())
        ?? option
      await select.selectOption({ label })
      return
    }

    const combo = this.dialog.locator('button[aria-haspopup="listbox"]').and(this.dialog.getByLabel(label))
    if (await combo.count() > 0) {
      await combo.first().click()
      await this.page.getByRole('option', { name: option, exact: true }).first().click()
      return
    }

    throw new Error(`No select or combobox labelled "${label}" in dialog`)
  }

  async selectOption(label: string, value: string) {
    await this.fillSelect(label, value)
  }

  /** Native <input type="checkbox"> (Oat), located by label. */
  private async setChecked(label: string, checked: boolean) {
    await this.dialog.getByLabel(label).and(this.page.locator('input[type="checkbox"]')).first().setChecked(checked)
  }

  async checkCheckbox(label: string) {
    await this.setChecked(label, true)
  }

  async uncheckCheckbox(label: string) {
    await this.setChecked(label, false)
  }

  async submit() {
    await this.submitButton.click()
  }

  async cancel() {
    await this.cancelButton.click()
  }

  async expectValidationError(message: string) {
    await expect(this.dialog.locator('text=' + message)).toBeVisible()
  }

  async expectFieldError(label: string, message: string) {
    const labelLocator = this.dialog.locator(`label`).filter({ hasText: label })
    const errorMessage = labelLocator.locator('..').locator('p, span, .error').filter({ hasText: message })
    await expect(errorMessage).toBeVisible()
  }
}
