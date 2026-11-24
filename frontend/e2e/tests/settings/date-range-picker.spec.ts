import { test, expect } from '@playwright/test'
import { loginAsSuperAdmin } from '../../framework'

/**
 * Regression: a custom date range saved to localStorage must be restored
 * (as CalendarDate instances) and shown in the From/To date inputs.
 */

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

  test('date inputs show the previously applied range', async ({ page }) => {
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

    // A custom range renders native From/To date inputs seeded from storage.
    // Exact + scoped to main: a substring "To" also matches "Toggle menu" etc.
    const main = page.locator('main')
    await expect(main.getByLabel('From', { exact: true })).toHaveValue('2026-04-01', { timeout: 10000 })
    await expect(main.getByLabel('To', { exact: true })).toHaveValue('2026-04-30')
  })
})
