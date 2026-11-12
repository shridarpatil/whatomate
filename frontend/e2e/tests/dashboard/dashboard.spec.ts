import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'

test.describe('Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/')
    await page.waitForLoadState('networkidle')
  })

  test('should display dashboard page', async ({ page }) => {
    // PageHeader no longer renders the subtitle; the heading identifies the page.
    await expect(page.locator('h1')).toContainText('Dashboard')
    await expect(page.locator('h1')).toBeVisible()
  })

  test('should display stat cards', async ({ page }) => {
    // Wait for widget cards to load (not skeleton) - use exact text matching to avoid duplicates
    await expect(page.getByText('Total Messages', { exact: true })).toBeVisible({ timeout: 15000 })
    await expect(page.getByText('Active Contacts', { exact: true })).toBeVisible()
    await expect(page.getByText('Chatbot Sessions', { exact: true })).toBeVisible()
    await expect(page.getByText('Total Campaigns', { exact: true })).toBeVisible()
  })

  test('should display time range filter', async ({ page }) => {
    await expect(page.locator('main').getByLabel('Date Range')).toBeVisible()
  })

  test('should change time range', async ({ page }) => {
    const range = page.locator('main').getByLabel('Date Range')
    await range.selectOption({ label: 'Last 7 Days' })

    // Should reload data
    await page.waitForLoadState('networkidle')

    await expect(range).toHaveValue('7days')
  })

  test('should display recent messages widget', async ({ page }) => {
        await expect(page.getByText('Recent Messages', { exact: true })).toBeVisible({ timeout: 15000 })
    await expect(page.getByText('Latest conversations from your contacts')).toBeVisible()
  })

  test('should display quick actions widget', async ({ page }) => {
    const main = page.locator('main')
    await expect(main.getByText('Quick Actions', { exact: true })).toBeVisible({ timeout: 15000 })
    await expect(main.getByText('Common tasks and shortcuts')).toBeVisible()

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
    const main = page.locator('main')
    await main.getByLabel('Date Range').selectOption({ label: 'Custom Range' })

    await expect(main.getByLabel('From', { exact: true })).toBeVisible()
    await expect(main.getByLabel('To', { exact: true })).toBeVisible()
    await expect(main.getByRole('button', { name: 'Apply' })).toBeVisible()
  })

  test('should display percentage change indicators', async ({ page }) => {
    // Wait for stats to load
    await expect(page.locator('text=Total Messages')).toBeVisible({ timeout: 15000 })

    // Should show percentage changes with comparison text
    await expect(page.locator('text=/from (yesterday|previous|last month)/i').first()).toBeVisible()
  })
})
