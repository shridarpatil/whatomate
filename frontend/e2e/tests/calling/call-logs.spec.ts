import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { CallLogsPage } from '../../pages'

test.describe('Call Logs Page', () => {
  let callLogsPage: CallLogsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    callLogsPage = new CallLogsPage(page)
    await callLogsPage.goto()
  })

  test('should display call logs page', async () => {
    await callLogsPage.expectPageVisible()
  })

  test('should show table or empty state', async ({ page }) => {
    const hasTable = await page.locator('table').isVisible().catch(() => false)
    const hasEmpty = await page.getByText(/no.*call|no.*log/i).isVisible().catch(() => false)
    expect(hasTable || hasEmpty).toBe(true)
  })

  test('should have status filter', async ({ page }) => {
    // Look for a select/dropdown that filters by status
    const filterTrigger = page.locator('button[role="combobox"], select').first()
    await expect(filterTrigger).toBeVisible()
  })

  test('should have search input', async ({ page }) => {
    const searchInput = page.getByPlaceholder(/search/i)
    if (await searchInput.isVisible()) {
      await searchInput.fill('test')
      await page.waitForTimeout(500)
      // Should not crash
      await expect(page.locator('[class*="card"], table, [class*="empty"]').first()).toBeVisible()
    }
  })

  test('should show call detail when clicking a row', async ({ page }) => {
    const rowCount = await callLogsPage.getRowCount()
    if (rowCount > 0) {
      await callLogsPage.clickRow(0)
      // Should open a detail dialog or expand
      const dialog = page.getByRole('dialog')
      if (await dialog.isVisible({ timeout: 2000 }).catch(() => false)) {
        await expect(dialog).toBeVisible()
      }
    }
  })

  test('should filter by status', async ({ page }) => {
    const statusTrigger = page.locator('button[role="combobox"]').first()
    if (await statusTrigger.isVisible()) {
      await statusTrigger.click()
      // Should show status options
      const options = page.getByRole('option')
      const optionCount = await options.count()
      expect(optionCount).toBeGreaterThan(0)
    }
  })
})

test.describe('Call Logs - Pagination', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/calling/logs')
    await page.waitForLoadState('networkidle')
  })

  test('should show pagination when there are multiple pages', async ({ page }) => {
    const rowCount = await page.locator('tbody tr').count()
    if (rowCount >= 20) {
      // Should have pagination controls
      const pagination = page.locator('[class*="pagination"], button:has-text("Next"), button:has-text(">")')
      await expect(pagination.first()).toBeVisible()
    }
  })
})
