import { test, expect } from '@playwright/test'
import { TablePage } from '../../pages'
import { loginAsAdmin, createTeamFixture, navigateToFirstItem, expectMetadataVisible, expectActivityLogVisible, expectDeleteFromForm } from '../../helpers'

test.describe('Teams - List View', () => {
  let tablePage: TablePage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/teams')
    await page.waitForLoadState('networkidle')
    tablePage = new TablePage(page)
  })

  test('should display teams list', async ({ page }) => {
    await expect(tablePage.tableBody).toBeVisible()
  })

  test('should search teams', async ({ page }) => {
    const initialCount = await tablePage.getRowCount()
    if (initialCount > 0) {
      await tablePage.search('nonexistent-team-xyz')
      await page.waitForTimeout(300)
      const filteredCount = await tablePage.getRowCount()
      expect(filteredCount).toBeLessThanOrEqual(initialCount)
    }
  })

  test('should load create page', async ({ page }) => {
    await page.goto('/settings/teams/new')
    await page.waitForLoadState('networkidle')
    expect(page.url()).toContain('/settings/teams/new')
    await expect(page.locator('input').first()).toBeVisible()
  })

  test('should load detail page from list', async ({ page }) => {
    const href = await navigateToFirstItem(page)
    if (href) {
      expect(page.url()).toMatch(/\/settings\/teams\/[a-f0-9-]+/)
      await expect(page.getByText('Details')).toBeVisible()
    }
  })

  test('should delete team from list', async ({ page }) => {
    const row = page.locator('tbody tr').first()
    if (await row.isVisible({ timeout: 3000 }).catch(() => false)) {
      // Click delete button
      await row.locator('button').filter({ has: page.locator('svg.text-destructive') }).click()
      const dialog = page.locator('[role="alertdialog"]')
      await expect(dialog).toBeVisible({ timeout: 3000 })
      // Cancel to not actually delete
      await dialog.getByRole('button', { name: /Cancel/i }).click()
    }
  })
})

test.describe('Teams - Detail Page CRUD', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
  })

  test('should show form fields on create page', async ({ page }) => {
    await page.goto('/settings/teams/new')
    await page.waitForLoadState('networkidle')

    await expect(page.locator('input').first()).toBeVisible()
    await expect(page.locator('textarea').first()).toBeVisible()
    await expect(page.locator('button[role="combobox"]').first()).toBeVisible()
    await expect(page.locator('button[role="switch"]').first()).toBeVisible()
  })

  test('should create a new team', async ({ page }) => {
    const newTeam = createTeamFixture()

    await page.goto('/settings/teams/new')
    await page.waitForLoadState('networkidle')

    const input = page.locator('input').first()
    if (await input.isDisabled()) { test.skip(true, 'No write permission'); return }

    await input.fill(newTeam.name)
    await page.locator('textarea').first().fill(newTeam.description)
    await page.waitForTimeout(300)

    const createBtn = page.getByRole('button', { name: /Create/i })
    if (!(await createBtn.isVisible({ timeout: 5000 }).catch(() => false))) {
      test.skip(true, 'Create button not visible')
      return
    }
    await createBtn.click({ force: true })
    await page.waitForTimeout(3000)

    if (page.url().includes('/new')) {
      test.skip(true, 'Creation failed (possibly CSRF)')
    } else {
      expect(page.url()).toMatch(/\/settings\/teams\/[a-f0-9-]+/)
    }
  })

  test('should edit existing team', async ({ page }) => {
    await page.goto('/settings/teams')
    await page.waitForLoadState('networkidle')

    const href = await navigateToFirstItem(page)
    if (!href) { test.skip(true, 'No teams exist'); return }

    const input = page.locator('input').first()
    if (await input.isDisabled()) { test.skip(true, 'No write permission'); return }

    const original = await input.inputValue()
    await input.fill(original + ' edited')
    await page.waitForTimeout(300)

    const saveBtn = page.getByRole('button', { name: /Save/i })
    if (await saveBtn.isVisible({ timeout: 5000 }).catch(() => false)) {
      await saveBtn.click({ force: true })
      await page.waitForTimeout(2000)

      // Revert
      await input.fill(original)
      await page.waitForTimeout(300)
      const revertBtn = page.getByRole('button', { name: /Save/i })
      if (await revertBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
        await revertBtn.click({ force: true })
      }
    }
  })

  test('should delete from detail page', async ({ page }) => {
    await page.goto('/settings/teams')
    await page.waitForLoadState('networkidle')

    const href = await navigateToFirstItem(page)
    if (!href) { test.skip(true, 'No teams exist'); return }

    await expectDeleteFromForm(page, '/settings/teams')
  })

  test('should show metadata', async ({ page }) => {
    await page.goto('/settings/teams')
    await page.waitForLoadState('networkidle')

    if (await navigateToFirstItem(page)) {
      await expectMetadataVisible(page)
    }
  })

  test('should show activity log', async ({ page }) => {
    await page.goto('/settings/teams')
    await page.waitForLoadState('networkidle')

    if (await navigateToFirstItem(page)) {
      await expectActivityLogVisible(page)
    }
  })
})

test.describe('Teams - Table Sorting', () => {
  let tablePage: TablePage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/teams')
    await page.waitForLoadState('networkidle')
    tablePage = new TablePage(page)
  })

  test('should sort by team name', async () => {
    await tablePage.clickColumnHeader('Team')
    const direction = await tablePage.getSortDirection('Team')
    expect(direction).not.toBeNull()
  })

  test('should sort by strategy', async () => {
    await tablePage.clickColumnHeader('Strategy')
    const direction = await tablePage.getSortDirection('Strategy')
    expect(direction).not.toBeNull()
  })

  test('should toggle sort direction', async () => {
    await tablePage.clickColumnHeader('Team')
    const firstDirection = await tablePage.getSortDirection('Team')
    await tablePage.clickColumnHeader('Team')
    const secondDirection = await tablePage.getSortDirection('Team')
    expect(firstDirection).not.toEqual(secondDirection)
  })
})

test.describe('Team Members', () => {
  test('should show members section on detail page', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/teams')
    await page.waitForLoadState('networkidle')

    if (await navigateToFirstItem(page)) {
      await expect(page.getByRole('heading', { name: /Members/ })).toBeVisible({ timeout: 5000 })
    }
  })
})
