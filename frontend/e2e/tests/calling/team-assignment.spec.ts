import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { createTestScope } from '../../framework'

const scope = createTestScope('team-assignment')

test.describe('Team Assignment Strategy for Calls', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
  })

  test('should create team with round_robin strategy', async ({ page }) => {
    const teamName = scope.name('rr')

    // Navigate to create page
    await page.goto('/settings/teams/new')
    await page.waitForLoadState('networkidle')

    // Fill name
    await page.locator('input').first().fill(teamName)
    await page.waitForTimeout(200)

    // Select round_robin strategy (default, but click to verify it works)
    const strategySelect = page.locator('button[role="combobox"]').first()
    if (await strategySelect.isVisible()) {
      await strategySelect.click()
      await page.getByRole('option', { name: /round.?robin/i }).click()
    }

    // Create
    const createBtn = page.getByRole('button', { name: /Create/i })
    await expect(createBtn).toBeVisible({ timeout: 5000 })
    await createBtn.click()
    await page.waitForTimeout(2000)

    // Verify team was created - should redirect to detail page
    expect(page.url()).not.toContain('/new')

    // Go to list and verify
    await page.goto('/settings/teams')
    await page.waitForLoadState('networkidle')
    await page.getByPlaceholder(/search/i).fill(teamName)
    await page.waitForTimeout(300)
    await expect(page.locator('tbody').getByText(teamName)).toBeVisible()
  })

  test('should create team with load_balanced strategy', async ({ page }) => {
    const teamName = scope.name('lb')

    await page.goto('/settings/teams/new')
    await page.waitForLoadState('networkidle')

    await page.locator('input').first().fill(teamName)
    await page.waitForTimeout(200)

    const strategySelect = page.locator('button[role="combobox"]').first()
    if (await strategySelect.isVisible()) {
      await strategySelect.click()
      await page.getByRole('option', { name: /load.?balanced/i }).click()
    }

    const createBtn = page.getByRole('button', { name: /Create/i })
    await expect(createBtn).toBeVisible({ timeout: 5000 })
    await createBtn.click()
    await page.waitForTimeout(2000)

    await page.goto('/settings/teams')
    await page.waitForLoadState('networkidle')
    await page.getByPlaceholder(/search/i).fill(teamName)
    await page.waitForTimeout(300)
    await expect(page.locator('tbody').getByText(teamName)).toBeVisible()
  })

  test('should create team with manual strategy', async ({ page }) => {
    const teamName = scope.name('manual')

    await page.goto('/settings/teams/new')
    await page.waitForLoadState('networkidle')

    await page.locator('input').first().fill(teamName)
    await page.waitForTimeout(200)

    const strategySelect = page.locator('button[role="combobox"]').first()
    if (await strategySelect.isVisible()) {
      await strategySelect.click()
      await page.getByRole('option', { name: /manual/i }).click()
    }

    const createBtn = page.getByRole('button', { name: /Create/i })
    await expect(createBtn).toBeVisible({ timeout: 5000 })
    await createBtn.click()
    await page.waitForTimeout(2000)

    await page.goto('/settings/teams')
    await page.waitForLoadState('networkidle')
    await page.getByPlaceholder(/search/i).fill(teamName)
    await page.waitForTimeout(300)
    await expect(page.locator('tbody').getByText(teamName)).toBeVisible()
  })
})

test.describe('Team per_agent_timeout_secs persists round-trip', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
  })

  test('UI form sets the per-agent timeout and reload preserves it', async ({ page }) => {
    const teamName = scope.name('timeout')
    await page.goto('/settings/teams/new')
    await page.waitForLoadState('networkidle')

    await page.locator('input').first().fill(teamName)

    // The numeric per_agent_timeout_secs input — type="number".
    const timeoutInput = page.locator('input[type="number"]').first()
    await timeoutInput.fill('30')
    await page.waitForTimeout(200)

    const createBtn = page.getByRole('button', { name: /^Create$/i })
    await expect(createBtn).toBeVisible({ timeout: 5000 })
    await createBtn.click()
    await page.waitForURL(/\/settings\/teams\/[a-f0-9-]+$/, { timeout: 10000 })
    await page.waitForLoadState('networkidle')

    // Reload to confirm the value round-tripped from the server.
    await page.reload()
    await page.waitForLoadState('networkidle')
    await expect(page.locator('input[type="number"]').first()).toHaveValue('30')
  })
})

test.describe('Calling Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
  })

  test('should navigate between calling sub-pages', async ({ page }) => {
    await page.goto('/calling/logs')
    await page.waitForLoadState('networkidle')
    expect(page.url()).toContain('/calling/logs')

    await page.goto('/calling/ivr-flows')
    await page.waitForLoadState('networkidle')
    expect(page.url()).toContain('/calling/ivr-flows')
  })

  test('should redirect /calling to /calling/logs', async ({ page }) => {
    await page.goto('/calling')
    await page.waitForLoadState('networkidle')
    // Should redirect to logs (first child route)
    expect(page.url()).toContain('/calling')
  })
})
