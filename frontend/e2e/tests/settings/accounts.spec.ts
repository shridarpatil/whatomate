import { test, expect } from '@playwright/test'
import { loginAsAdmin, navigateToFirstItem, expectMetadataVisible, expectActivityLogVisible, expectDeleteFromForm } from '../../helpers'
import { AccountsPage } from '../../pages'

test.describe('WhatsApp Accounts - List View', () => {
  let accountsPage: AccountsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    accountsPage = new AccountsPage(page)
    await accountsPage.goto()
  })

  test('should display accounts page', async () => {
    await accountsPage.expectPageVisible()
    await expect(accountsPage.addButton).toBeVisible()
  })

  test('should load create page', async ({ page }) => {
    await page.goto('/settings/accounts/new')
    await page.waitForLoadState('networkidle')
    expect(page.url()).toContain('/settings/accounts/new')
    await expect(page.locator('input').first()).toBeVisible()
  })

  test('should show delete confirmation from list', async ({ page }) => {
    // Find the destructive (red) delete button in the first data row
    const firstRow = page.locator('tbody tr').first()
    if (!(await firstRow.isVisible({ timeout: 3000 }).catch(() => false))) {
      test.skip(true, 'No accounts in list')
      return
    }
    const deleteBtn = firstRow.locator('button.text-destructive, button:has(svg.text-destructive)').first()
    if (!(await deleteBtn.isVisible({ timeout: 3000 }).catch(() => false))) {
      test.skip(true, 'No delete button found')
      return
    }
    await deleteBtn.click()
    await expect(accountsPage.alertDialog).toBeVisible({ timeout: 5000 })
    await accountsPage.cancelDelete()
  })

  test('should load detail page from list', async ({ page }) => {
    const href = await navigateToFirstItem(page)
    if (href) {
      expect(page.url()).toMatch(/\/settings\/accounts\/[a-f0-9-]+/)
      await expect(page.getByText('Account Details')).toBeVisible()
    }
  })
})

test.describe('WhatsApp Accounts - Detail Page CRUD', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
  })

  test('should show form fields on create page', async ({ page }) => {
    await page.goto('/settings/accounts/new')
    await page.waitForLoadState('networkidle')

    await expect(page.locator('input').first()).toBeVisible()
    await expect(page.locator('input[type="password"]').first()).toBeVisible()
  })

  test('should show validation error for empty required fields', async ({ page }) => {
    await page.goto('/settings/accounts/new')
    await page.waitForLoadState('networkidle')

    // Fill something to trigger hasChanges
    const input = page.locator('input').first()
    if (await input.isDisabled()) { test.skip(true, 'No write permission'); return }

    await input.fill('test')
    await input.clear()
    await page.waitForTimeout(300)

    const createBtn = page.getByRole('button', { name: /Create/i })
    if (await createBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await createBtn.click({ force: true })
      const toast = page.locator('[data-sonner-toast]').first()
      await expect(toast).toBeVisible({ timeout: 5000 })
    }
  })

  test('should show webhook config on existing account', async ({ page }) => {
    await page.goto('/settings/accounts')
    await page.waitForLoadState('networkidle')

    if (await navigateToFirstItem(page)) {
      await expect(page.getByText('Webhook Configuration')).toBeVisible()
    }
  })

  test('should have test connection button', async ({ page }) => {
    await page.goto('/settings/accounts')
    await page.waitForLoadState('networkidle')

    if (await navigateToFirstItem(page)) {
      await expect(page.getByRole('button', { name: /Test/i })).toBeVisible()
    }
  })

  test('should have subscribe button', async ({ page }) => {
    await page.goto('/settings/accounts')
    await page.waitForLoadState('networkidle')

    if (await navigateToFirstItem(page)) {
      await expect(page.getByRole('button', { name: /Subscribe/i })).toBeVisible()
    }
  })

  test('should have business profile button', async ({ page }) => {
    await page.goto('/settings/accounts')
    await page.waitForLoadState('networkidle')

    if (await navigateToFirstItem(page)) {
      await expect(page.getByRole('button', { name: /Profile/i })).toBeVisible()
    }
  })

  test('should delete from detail page', async ({ page }) => {
    await page.goto('/settings/accounts')
    await page.waitForLoadState('networkidle')

    if (await navigateToFirstItem(page)) {
      await expectDeleteFromForm(page, '/settings/accounts')
    }
  })

  test('should show metadata', async ({ page }) => {
    await page.goto('/settings/accounts')
    await page.waitForLoadState('networkidle')

    if (await navigateToFirstItem(page)) {
      await expectMetadataVisible(page)
    }
  })

  test('should show activity log', async ({ page }) => {
    await page.goto('/settings/accounts')
    await page.waitForLoadState('networkidle')

    if (await navigateToFirstItem(page)) {
      await expectActivityLogVisible(page)
    }
  })

  test('should show setup guide', async ({ page }) => {
    await page.goto('/settings/accounts')
    await page.waitForLoadState('networkidle')

    if (await navigateToFirstItem(page)) {
      await expect(page.getByText('Setup Guide')).toBeVisible({ timeout: 15000 })
    }
  })
})
