import { test, expect } from '@playwright/test'
import { ApiHelper } from '../../helpers'

// Admin credentials for creating test organizations
const ADMIN_EMAIL = 'admin@admin.com'
const ADMIN_PASSWORD = 'admin'
const FALLBACK_ADMIN_EMAIL = 'admin@test.com'
const FALLBACK_ADMIN_PASSWORD = 'password'

test.describe('Register', () => {
  test('should show invitation required message without org param', async ({ page }) => {
    await page.goto('/register')

    // Should NOT show the registration form fields
    await expect(page.locator('input#fullName')).not.toBeVisible()
    await expect(page.locator('input#email')).not.toBeVisible()
    await expect(page.locator('input#password')).not.toBeVisible()

    // Should show invitation required message
    await expect(page.locator('text=/invitation/i')).toBeVisible()

    // Should show sign in link (RouterLink wraps a Button, so use the link role)
    await expect(page.getByRole('link', { name: /Sign in/i })).toBeVisible()
  })

  test('should display registration form with org query param', async ({ page, request }) => {
    // Create an org to get a valid org ID
    const api = new ApiHelper(request)
    try {
      await api.login(ADMIN_EMAIL, ADMIN_PASSWORD)
    } catch {
      try {
        await api.login(FALLBACK_ADMIN_EMAIL, FALLBACK_ADMIN_PASSWORD)
      } catch {
        test.skip(true, 'No admin credentials available')
        return
      }
    }

    let orgId: string
    try {
      const org = await api.createOrganization(`Register Test Org ${Date.now()}`)
      orgId = org.id
    } catch {
      test.skip(true, 'Failed to create test organization')
      return
    }

    await page.goto(`/register?org=${orgId}`)

    // Should show the registration form
    await expect(page.locator('input#fullName')).toBeVisible()
    await expect(page.locator('input#email')).toBeVisible()
    await expect(page.locator('input#password')).toBeVisible()
    await expect(page.locator('input#confirmPassword')).toBeVisible()
    await expect(page.locator('button[type="submit"]')).toBeVisible()
  })

  test('should show error for empty fields', async ({ page, request }) => {
    const api = new ApiHelper(request)
    try {
      await api.login(ADMIN_EMAIL, ADMIN_PASSWORD)
    } catch {
      try {
        await api.login(FALLBACK_ADMIN_EMAIL, FALLBACK_ADMIN_PASSWORD)
      } catch {
        test.skip(true, 'No admin credentials available')
        return
      }
    }

    let orgId: string
    try {
      const org = await api.createOrganization(`Register Empty Test ${Date.now()}`)
      orgId = org.id
    } catch {
      test.skip(true, 'Failed to create test organization')
      return
    }

    await page.goto(`/register?org=${orgId}`)
    await page.locator('button[type="submit"]').click()

    const toast = page.locator('[data-sonner-toast]')
    await expect(toast).toBeVisible({ timeout: 5000 })
    await expect(toast).toContainText('fill in all fields')
  })

  test('should show error for mismatched passwords', async ({ page, request }) => {
    const api = new ApiHelper(request)
    try {
      await api.login(ADMIN_EMAIL, ADMIN_PASSWORD)
    } catch {
      try {
        await api.login(FALLBACK_ADMIN_EMAIL, FALLBACK_ADMIN_PASSWORD)
      } catch {
        test.skip(true, 'No admin credentials available')
        return
      }
    }

    let orgId: string
    try {
      const org = await api.createOrganization(`Register Mismatch Test ${Date.now()}`)
      orgId = org.id
    } catch {
      test.skip(true, 'Failed to create test organization')
      return
    }

    await page.goto(`/register?org=${orgId}`)
    await page.locator('input#fullName').fill('Test User')
    await page.locator('input#email').fill('newuser@test.com')
    await page.locator('input#password').fill('password123')
    await page.locator('input#confirmPassword').fill('different123')
    await page.locator('button[type="submit"]').click()

    const toast = page.locator('[data-sonner-toast]')
    await expect(toast).toBeVisible({ timeout: 5000 })
    await expect(toast).toContainText('do not match')
  })

  test('should show error for short password', async ({ page, request }) => {
    const api = new ApiHelper(request)
    try {
      await api.login(ADMIN_EMAIL, ADMIN_PASSWORD)
    } catch {
      try {
        await api.login(FALLBACK_ADMIN_EMAIL, FALLBACK_ADMIN_PASSWORD)
      } catch {
        test.skip(true, 'No admin credentials available')
        return
      }
    }

    let orgId: string
    try {
      const org = await api.createOrganization(`Register Short PW Test ${Date.now()}`)
      orgId = org.id
    } catch {
      test.skip(true, 'Failed to create test organization')
      return
    }

    await page.goto(`/register?org=${orgId}`)
    await page.locator('input#fullName').fill('Test User')
    await page.locator('input#email').fill('newuser@test.com')
    await page.locator('input#password').fill('short')
    await page.locator('input#confirmPassword').fill('short')
    await page.locator('button[type="submit"]').click()

    const toast = page.locator('[data-sonner-toast]')
    await expect(toast).toBeVisible({ timeout: 5000 })
    await expect(toast).toContainText('at least 8 characters')
  })

  test('should navigate to login page from invitation required', async ({ page }) => {
    await page.goto('/register')
    await page.getByRole('link', { name: /Sign in/i }).click()
    await expect(page).toHaveURL(/\/login/)
  })

  test('should navigate to login page from registration form', async ({ page, request }) => {
    const api = new ApiHelper(request)
    try {
      await api.login(ADMIN_EMAIL, ADMIN_PASSWORD)
    } catch {
      try {
        await api.login(FALLBACK_ADMIN_EMAIL, FALLBACK_ADMIN_PASSWORD)
      } catch {
        test.skip(true, 'No admin credentials available')
        return
      }
    }

    let orgId: string
    try {
      const org = await api.createOrganization(`Register Nav Test ${Date.now()}`)
      orgId = org.id
    } catch {
      test.skip(true, 'Failed to create test organization')
      return
    }

    await page.goto(`/register?org=${orgId}`)
    await page.locator('a').filter({ hasText: /Sign in/i }).click()
    await expect(page).toHaveURL(/\/login/)
  })
})
