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
    // Time range is a native select in the page header
    const timeRangeSelect = page.locator('main').getByLabel('Date Range')
    await expect(timeRangeSelect).toBeVisible()
  })

  test('should change time range filter', async ({ page }) => {
    const timeRangeSelect = page.locator('main').getByLabel('Date Range')
    const options = timeRangeSelect.locator('option')
    expect(await options.count()).toBeGreaterThan(1)
    await timeRangeSelect.selectOption({ index: 1 })
    await page.waitForLoadState('networkidle')
  })

  test('should display agent performance metrics', async ({ page }) => {
    // Wait for the stat tiles to finish loading (skeletons disappear)
    await page.locator('[data-testid="agent-stats"] h3').first().waitFor({ timeout: 15000 })

    // Stat card labels (exact match to avoid matching chart descriptions)
    await expect(page.getByText('Transfers Handled', { exact: true }).first()).toBeVisible()
    await expect(page.getByText('Active Conversations', { exact: true })).toBeVisible()
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
