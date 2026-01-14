import { test, expect } from '@playwright/test'
import { loginAsAdmin, loginAsAgent } from '../../helpers'

test.describe('Agent Analytics', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/analytics/agents')
    await page.waitForLoadState('networkidle')
  })

  test('should display agent analytics page', async ({ page }) => {
    // Check for page header
    await expect(page.locator('h1')).toContainText('Agent Analytics')
  })

  test('should display time range filter', async ({ page }) => {
    // Check for time range selector
    const timeRangeSelect = page.locator('button[role="combobox"]').first()
    await expect(timeRangeSelect).toBeVisible()
  })

  test('should change time range filter', async ({ page }) => {
    // Open time range dropdown
    await page.locator('button[role="combobox"]').first().click()

    // Select different option
    const options = page.locator('[role="option"]')
    if (await options.count() > 1) {
      await options.nth(1).click()
      await page.waitForLoadState('networkidle')
    }
  })

  test('should display agent performance metrics', async ({ page }) => {
    // Wait for stats cards to load - use headings for specific matching
    await expect(page.getByRole('heading', { name: 'Transfers Handled' })).toBeVisible({ timeout: 10000 })
    await expect(page.getByRole('heading', { name: 'Active Conversations' })).toBeVisible()
  })
})

test.describe('Agent Analytics - Agent Role', () => {
  test('should allow agents to view their own analytics', async ({ page }) => {
    await loginAsAgent(page)
    await page.goto('/analytics/agents')
    await page.waitForLoadState('networkidle')

    // Agents should be able to see the analytics page (with limited data)
    await expect(page).toHaveURL(/\/analytics\/agents/)
  })
})
