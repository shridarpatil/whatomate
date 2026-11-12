import { Page, Locator, expect } from '@playwright/test'

export class BasePage {
  constructor(protected page: Page) {}

  async navigateTo(path: string) {
    await this.page.goto(path)
  }

  async waitForLoad() {
    await this.page.waitForLoadState('networkidle')
  }

  async waitForNavigation() {
    await this.page.waitForLoadState('domcontentloaded')
  }

  async clickNavItem(name: string) {
    await this.page.locator(`nav`).getByText(name).click()
  }

  async getToastMessage(): Promise<string | null> {
    const toast = this.page.locator('.toast-container output').first()
    if (await toast.isVisible()) {
      return toast.textContent()
    }
    return null
  }

  async waitForToast(text: string) {
    await this.page.locator('.toast-container output').filter({ hasText: text }).waitFor()
  }

  // Oat toasts have no close button or click-to-dismiss, so remove them from the DOM.
  async dismissToast(text?: string | RegExp) {
    const toasts = this.page.locator('.toast-container output')
    const target = text ? toasts.filter({ hasText: text }) : toasts
    await target.evaluateAll(els => els.forEach(el => el.remove()))
    await expect(target).toHaveCount(0)
  }
}
