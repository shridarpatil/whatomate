import { test, expect } from '@playwright/test'

test.describe('Register', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/register')
  })

  test('should display registration form', async ({ page }) => {
    await expect(page.locator('input#fullName')).toBeVisible()
    await expect(page.locator('input#email')).toBeVisible()
    await expect(page.locator('input#organizationName')).toBeVisible()
    await expect(page.locator('input#password')).toBeVisible()
    await expect(page.locator('input#confirmPassword')).toBeVisible()
    await expect(page.locator('button[type="submit"]')).toBeVisible()
  })

  test('should show error for empty fields', async ({ page }) => {
    await page.locator('button[type="submit"]').click()

    // Should show toast error
    const toast = page.locator('[data-sonner-toast]')
    await expect(toast).toBeVisible({ timeout: 5000 })
    await expect(toast).toContainText('fill in all fields')
  })

  test('should show error for mismatched passwords', async ({ page }) => {
    await page.locator('input#fullName').fill('Test User')
    await page.locator('input#email').fill('newuser@test.com')
    await page.locator('input#organizationName').fill('Test Org')
    await page.locator('input#password').fill('password123')
    await page.locator('input#confirmPassword').fill('different123')
    await page.locator('button[type="submit"]').click()

    const toast = page.locator('[data-sonner-toast]')
    await expect(toast).toBeVisible({ timeout: 5000 })
    await expect(toast).toContainText('do not match')
  })

  test('should show error for short password', async ({ page }) => {
    await page.locator('input#fullName').fill('Test User')
    await page.locator('input#email').fill('newuser@test.com')
    await page.locator('input#organizationName').fill('Test Org')
    await page.locator('input#password').fill('short')
    await page.locator('input#confirmPassword').fill('short')
    await page.locator('button[type="submit"]').click()

    const toast = page.locator('[data-sonner-toast]')
    await expect(toast).toBeVisible({ timeout: 5000 })
    await expect(toast).toContainText('at least 8 characters')
  })

  test('should navigate to login page', async ({ page }) => {
    await page.locator('a').filter({ hasText: /Sign in/i }).click()
    await expect(page).toHaveURL(/\/login/)
  })

  test('should show error for existing email', async ({ page }) => {
    // Try to register with an email that already exists
    await page.locator('input#fullName').fill('Admin User')
    await page.locator('input#email').fill('admin@test.com')
    await page.locator('input#organizationName').fill('Test Org')
    await page.locator('input#password').fill('password123')
    await page.locator('input#confirmPassword').fill('password123')
    await page.locator('button[type="submit"]').click()

    const toast = page.locator('[data-sonner-toast]')
    await expect(toast).toBeVisible({ timeout: 5000 })
    // Should show some kind of error (email exists, registration failed, etc.)
  })
})
