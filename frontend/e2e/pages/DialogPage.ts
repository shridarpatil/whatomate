import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

export class DialogPage extends BasePage {
  readonly dialog: Locator
  readonly cancelButton: Locator
  readonly closeButton: Locator

  constructor(page: Page) {
    super(page)
    this.dialog = page.locator('[role="dialog"]')
    // Be specific - Cancel button (not the X close button)
    this.cancelButton = this.dialog.getByRole('button', { name: 'Cancel', exact: true })
    this.closeButton = this.dialog.locator('button[class*="absolute"]')
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

  async selectOption(label: string, value: string) {
    // Find the select trigger near the label
    const labelLocator = this.dialog.locator(`label`).filter({ hasText: label })
    const selectTrigger = labelLocator.locator('..').locator('button[role="combobox"]')

    if (await selectTrigger.count() > 0) {
      await selectTrigger.click()
    } else {
      // Try finding by data-testid or other selectors
      const container = labelLocator.locator('..')
      await container.locator('button').first().click()
    }

    // Click the option
    await this.page.locator('[role="option"]').filter({ hasText: value }).click()
  }

  async checkCheckbox(label: string) {
    // Find label, get its 'for' attribute, then find checkbox by id
    const labelLocator = this.dialog.locator('label').filter({ hasText: label })
    const labelFor = await labelLocator.getAttribute('for')

    let checkbox: Locator
    if (labelFor) {
      // Checkbox has id matching label's for attribute
      // Need to escape dots in ID for CSS selector
      const escapedId = labelFor.replace(/\./g, '\\.')
      checkbox = this.dialog.locator(`#${escapedId}`)
    } else {
      // Checkbox is sibling of label in same container
      checkbox = labelLocator.locator('..').locator('button[role="checkbox"], input[type="checkbox"]')
    }

    // Check if already checked
    const dataState = await checkbox.getAttribute('data-state')
    if (dataState !== 'checked') {
      await checkbox.click()
    }
  }

  async uncheckCheckbox(label: string) {
    const labelLocator = this.dialog.locator('label').filter({ hasText: label })
    const labelFor = await labelLocator.getAttribute('for')

    let checkbox: Locator
    if (labelFor) {
      const escapedId = labelFor.replace(/\./g, '\\.')
      checkbox = this.dialog.locator(`#${escapedId}`)
    } else {
      checkbox = labelLocator.locator('..').locator('button[role="checkbox"], input[type="checkbox"]')
    }

    const dataState = await checkbox.getAttribute('data-state')
    if (dataState === 'checked') {
      await checkbox.click()
    }
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
    const errorMessage = labelLocator.locator('..').locator('p, span').filter({ hasText: message })
    await expect(errorMessage).toBeVisible()
  }
}
