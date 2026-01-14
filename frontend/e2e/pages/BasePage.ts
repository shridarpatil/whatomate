import { Page, Locator } from '@playwright/test'

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
    const toast = this.page.locator('[data-sonner-toast]').first()
    if (await toast.isVisible()) {
      return toast.textContent()
    }
    return null
  }

  async waitForToast(text: string) {
    await this.page.locator('[data-sonner-toast]').filter({ hasText: text }).waitFor()
  }

  async dismissToast() {
    const toast = this.page.locator('[data-sonner-toast]').first()
    if (await toast.isVisible()) {
      await toast.locator('button').click()
    }
  }
}
