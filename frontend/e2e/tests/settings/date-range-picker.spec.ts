import { test, expect, Page } from '@playwright/test'

/**
 * Regression: a custom date range saved to localStorage was being restored
 * as plain {year, month, day} objects, which RangeCalendar (reka-ui) can't
 * render. Result: re-opening the date picker after a previous Apply showed
 * only the Apply button — no calendar grid. Now restored as CalendarDate
 * instances.
 */
async function loginAsSuperAdmin(page: Page) {
  await page.goto('/login')
  await page.locator('input[name="email"], input[type="email"]').fill('admin@admin.com')
  await page.locator('input[name="password"], input[type="password"]').fill('admin')
  await page.locator('button[type="submit"]').click()
  await page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 10000 })
}

test.describe('DateRangePicker — re-open after Apply', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsSuperAdmin(page)

    // Seed a previously-applied custom range (simulates the user having
    // hit Apply before). Without the fix, this state breaks the next open.
    await page.addInitScript(() => {
      localStorage.setItem('dashboard_range', 'custom')
      localStorage.setItem(
        'dashboard_custom',
        JSON.stringify({
          start: { year: 2026, month: 4, day: 1 },
          end: { year: 2026, month: 4, day: 30 },
        }),
      )
    })
  })

  test.afterEach(async ({ page }) => {
    await page.evaluate(() => {
      localStorage.removeItem('dashboard_range')
      localStorage.removeItem('dashboard_custom')
    })
  })

  test('calendar grid renders on second open with previously applied range', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('networkidle')

    // Confirm the seeded localStorage actually got read by the SPA. If not,
    // the rest of the test is meaningless.
    const storageState = await page.evaluate(() => ({
      range: localStorage.getItem('dashboard_range'),
      custom: localStorage.getItem('dashboard_custom'),
    }))
    expect(storageState.range).toBe('custom')
    expect(storageState.custom).toBeTruthy()

    // The date button is rendered by formatDateRangeDisplay; with the
    // seeded range it shows month/day/year separated by " - ". Match a
    // small portion to stay robust against locale formatting differences.
    const dateButton = page.getByRole('button').filter({ hasText: '/2026' })
    await expect(dateButton).toBeVisible({ timeout: 10000 })
    await dateButton.click()

    // The grid is what was missing before the fix.
    const grid = page.locator('[role="grid"]').first()
    await expect(grid).toBeVisible({ timeout: 5000 })

    // Sanity: at least one date cell is present.
    await expect(page.locator('[role="gridcell"]').first()).toBeVisible()
  })
})
