import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Profile Page - User profile and password management
 */
export class ProfilePage extends BasePage {
  readonly heading: Locator
  readonly accountInfoCard: Locator
  readonly changePasswordCard: Locator
  readonly currentPasswordInput: Locator
  readonly newPasswordInput: Locator
  readonly confirmPasswordInput: Locator
  readonly changePasswordButton: Locator

  constructor(page: Page) {
    super(page)
    this.heading = page.locator('h1').filter({ hasText: 'Profile' })
    this.accountInfoCard = page.locator('.rounded-xl.border').filter({ hasText: 'Account Information' })
    this.changePasswordCard = page.locator('.rounded-xl.border').filter({ hasText: 'Change Password' })
    this.currentPasswordInput = page.locator('input#current_password')
    this.newPasswordInput = page.locator('input#new_password')
    this.confirmPasswordInput = page.locator('input#confirm_password')
    this.changePasswordButton = page.getByRole('button', { name: /Change Password/i })
  }

  async goto() {
    await this.page.goto('/profile')
    await this.page.waitForLoadState('networkidle')
  }

  // Password helpers
  async fillPasswordForm(currentPassword: string, newPassword: string, confirmPassword: string) {
    await this.currentPasswordInput.fill(currentPassword)
    await this.newPasswordInput.fill(newPassword)
    await this.confirmPasswordInput.fill(confirmPassword)
  }

  async changePassword(currentPassword: string, newPassword: string, confirmPassword: string) {
    await this.fillPasswordForm(currentPassword, newPassword, confirmPassword)
    await this.changePasswordButton.click()
  }

  async togglePasswordVisibility(field: 'current' | 'new' | 'confirm') {
    const container = field === 'current'
      ? this.currentPasswordInput.locator('..')
      : field === 'new'
        ? this.newPasswordInput.locator('..')
        : this.confirmPasswordInput.locator('..')
    await container.locator('button').click()
  }

  // Toast helpers
  async expectToast(text: string | RegExp) {
    const toast = this.page.locator('[data-sonner-toast]').filter({ hasText: text })
    await expect(toast).toBeVisible({ timeout: 5000 })
    return toast
  }

  // Assertions
  async expectPageVisible() {
    await expect(this.heading).toBeVisible()
  }

  async expectAccountInfo(name: string, email: string, role: string) {
    await expect(this.accountInfoCard).toContainText(name)
    await expect(this.accountInfoCard).toContainText(email)
    await expect(this.accountInfoCard).toContainText(role)
  }

  async expectPasswordFieldsVisible() {
    await expect(this.currentPasswordInput).toBeVisible()
    await expect(this.newPasswordInput).toBeVisible()
    await expect(this.confirmPasswordInput).toBeVisible()
  }
}
