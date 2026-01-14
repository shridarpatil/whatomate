import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'

test.describe('Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/')
    await page.waitForLoadState('networkidle')
  })

  test('should display dashboard page', async ({ page }) => {
    await expect(page.locator('h1')).toContainText('Dashboard')
    await expect(page.locator('text=Overview of your messaging platform')).toBeVisible()
  })

  test('should display stat cards', async ({ page }) => {
    // Wait for stat cards to load (not skeleton) - use headings for specific matching
    const main = page.locator('main')
    await expect(main.getByRole('heading', { name: 'Total Messages' })).toBeVisible({ timeout: 15000 })
    await expect(main.getByRole('heading', { name: 'Contacts' })).toBeVisible()
    await expect(main.getByRole('heading', { name: 'Chatbot Sessions' })).toBeVisible()
    await expect(main.getByRole('heading', { name: 'Campaigns Sent' })).toBeVisible()
  })

  test('should display time range filter', async ({ page }) => {
    // Check for time range selector
    const timeRangeSelect = page.locator('button[role="combobox"]').first()
    await expect(timeRangeSelect).toBeVisible()
  })

  test('should change time range', async ({ page }) => {
    // Open time range dropdown
    await page.locator('button[role="combobox"]').first().click()

    // Select "Last 7 days"
    await page.locator('[role="option"]').filter({ hasText: 'Last 7 days' }).click()

    // Should reload data
    await page.waitForLoadState('networkidle')

    // Check that the selection is updated
    await expect(page.locator('button[role="combobox"]').first()).toContainText('Last 7 days')
  })

  test('should display recent messages section', async ({ page }) => {
    await expect(page.locator('text=Recent Messages')).toBeVisible()
    await expect(page.locator('text=Latest conversations from your contacts')).toBeVisible()
  })

  test('should display quick actions section', async ({ page }) => {
    const main = page.locator('main')
    await expect(main.locator('text=Quick Actions')).toBeVisible()
    await expect(main.locator('text=Common tasks and shortcuts')).toBeVisible()

    // Check for quick action links - scope to main to avoid sidebar duplicates
    await expect(main.locator('a[href="/chat"]')).toBeVisible()
    await expect(main.locator('a[href="/campaigns"]')).toBeVisible()
    await expect(main.locator('a[href="/templates"]')).toBeVisible()
    await expect(main.locator('a[href="/chatbot"]')).toBeVisible()
  })

  test('should navigate to chat from quick actions', async ({ page }) => {
    // Use main to scope to quick actions, not sidebar
    await page.locator('main a[href="/chat"]').click()
    await expect(page).toHaveURL(/\/chat/)
  })

  test('should navigate to campaigns from quick actions', async ({ page }) => {
    await page.locator('main a[href="/campaigns"]').click()
    await expect(page).toHaveURL(/\/campaigns/)
  })

  test('should navigate to templates from quick actions', async ({ page }) => {
    await page.locator('main a[href="/templates"]').click()
    await expect(page).toHaveURL(/\/templates/)
  })

  test('should navigate to chatbot from quick actions', async ({ page }) => {
    await page.locator('main a[href="/chatbot"]').click()
    await expect(page).toHaveURL(/\/chatbot/)
  })

  test('should show custom date range picker', async ({ page }) => {
    // Open time range dropdown
    await page.locator('button[role="combobox"]').first().click()

    // Select "Custom range"
    await page.locator('[role="option"]').filter({ hasText: 'Custom range' }).click()

    // Should show date picker button
    await expect(page.getByRole('button', { name: /Select dates|Calendar/i })).toBeVisible()
  })

  test('should display percentage change indicators', async ({ page }) => {
    // Wait for stats to load
    await expect(page.locator('text=Total Messages')).toBeVisible({ timeout: 15000 })

    // Should show percentage changes with comparison text
    await expect(page.locator('text=/from (yesterday|previous|last month)/i').first()).toBeVisible()
  })
})
