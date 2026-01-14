import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

export class LoginPage extends BasePage {
  readonly emailInput: Locator
  readonly passwordInput: Locator
  readonly submitButton: Locator
  readonly errorMessage: Locator
  readonly registerLink: Locator

  constructor(page: Page) {
    super(page)
    this.emailInput = page.locator('input[name="email"], input[type="email"]')
    this.passwordInput = page.locator('input[name="password"], input[type="password"]')
    this.submitButton = page.locator('button[type="submit"]')
    this.errorMessage = page.locator('[role="alert"], .error-message, [data-error]')
    this.registerLink = page.locator('a').filter({ hasText: /Register|Sign up/i })
  }

  async goto() {
    await this.page.goto('/login')
  }

  async login(email: string, password: string) {
    await this.emailInput.fill(email)
    await this.passwordInput.fill(password)
    await this.submitButton.click()
  }

  async expectLoginSuccess() {
    // Should redirect away from login page
    await expect(this.page).not.toHaveURL(/\/login/)
  }

  async expectLoginError(message?: string) {
    // Errors are shown as toast notifications
    const toastLocator = this.page.locator('[data-sonner-toast]')
    await expect(toastLocator).toBeVisible({ timeout: 10000 })
    if (message) {
      await expect(toastLocator.filter({ hasText: message })).toBeVisible()
    }
  }

  async goToRegister() {
    await this.registerLink.click()
  }
}
