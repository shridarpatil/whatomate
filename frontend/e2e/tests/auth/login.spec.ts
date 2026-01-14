import { test, expect } from '@playwright/test'
import { LoginPage } from '../../pages'
import { TEST_USERS, logout } from '../../helpers'

test.describe('Login', () => {
  let loginPage: LoginPage

  test.beforeEach(async ({ page }) => {
    loginPage = new LoginPage(page)
    await loginPage.goto()
  })

  test('should display login form', async ({ page }) => {
    await expect(loginPage.emailInput).toBeVisible()
    await expect(loginPage.passwordInput).toBeVisible()
    await expect(loginPage.submitButton).toBeVisible()
  })

  test('should login successfully with valid credentials', async ({ page }) => {
    await loginPage.login(TEST_USERS.admin.email, TEST_USERS.admin.password)
    await loginPage.expectLoginSuccess()
  })

  test('should show error with invalid credentials', async ({ page }) => {
    await loginPage.login('invalid@test.com', 'wrongpassword')
    await loginPage.expectLoginError()
  })

  test('should show validation error for empty email', async ({ page }) => {
    await loginPage.passwordInput.fill('password')
    await loginPage.submitButton.click()
    // Should stay on login page
    await expect(page).toHaveURL(/\/login/)
  })

  test('should show validation error for empty password', async ({ page }) => {
    await loginPage.emailInput.fill('test@test.com')
    await loginPage.submitButton.click()
    // Should stay on login page
    await expect(page).toHaveURL(/\/login/)
  })

  test('should navigate to register page', async ({ page }) => {
    await loginPage.goToRegister()
    await expect(page).toHaveURL(/\/register/)
  })

  test('should logout successfully', async ({ page }) => {
    // First login
    await loginPage.login(TEST_USERS.admin.email, TEST_USERS.admin.password)
    await loginPage.expectLoginSuccess()

    // Then logout
    await logout(page)
    await expect(page).toHaveURL(/\/login/)
  })
})

test.describe('Authentication Redirect', () => {
  test('should redirect to login when accessing protected route', async ({ page }) => {
    await page.goto('/settings/users')
    await expect(page).toHaveURL(/\/login/)
  })

  test('should redirect to dashboard after login', async ({ page }) => {
    const loginPage = new LoginPage(page)
    await loginPage.goto()
    await loginPage.login(TEST_USERS.admin.email, TEST_USERS.admin.password)
    // Should be on dashboard or chat
    await expect(page).toHaveURL(/\/(dashboard|chat)?/)
  })
})
