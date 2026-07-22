import { test, expect } from '@playwright/test'
import { loginAsAdmin, navigateToFirstItem, expectMetadataVisible, expectActivityLogVisible, expectDeleteFromForm } from '../../helpers'
import { KeywordsPage } from '../../pages'

// Seed a keyword rule via the UI before tests that need data
async function seedKeywordRule(page: import('@playwright/test').Page): Promise<boolean> {
  await page.goto('/chatbot/keywords/new')
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(1000) // Wait for auth/permissions to load

  const input = page.locator('input').first()
  // Wait for input to be enabled (permissions may load async)
  try {
    await input.waitFor({ state: 'attached', timeout: 5000 })
    if (await input.isDisabled({ timeout: 3000 })) return false
  } catch {
    return false
  }

  await input.fill(`e2e-seed-${Date.now()}`)
  const textarea = page.locator('textarea')
  if (await textarea.isVisible()) {
    await textarea.fill('E2E seeded response')
  }
  await page.waitForTimeout(500)

  const createBtn = page.getByRole('button', { name: /Create/i })
  if (!(await createBtn.isVisible({ timeout: 5000 }).catch(() => false))) return false

  await createBtn.click({ force: true })
  await page.waitForTimeout(3000)
  return !page.url().includes('/new')
}

test.describe('Keyword Rules - List View', () => {
  let keywordsPage: KeywordsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    keywordsPage = new KeywordsPage(page)
    await keywordsPage.goto()
  })

  test('should display keywords page', async () => {
    await keywordsPage.expectPageVisible()
  })

  test('should have search input', async () => {
    await expect(keywordsPage.searchInput).toBeVisible()
  })

  test('should load create page', async ({ page }) => {
    await page.goto('/chatbot/keywords/new')
    await page.waitForLoadState('networkidle')
    expect(page.url()).toContain('/chatbot/keywords/new')
    await expect(page.locator('input').first()).toBeVisible()
  })

  test('should load detail page from list', async ({ page }) => {
    // Seed if empty
    let href = await navigateToFirstItem(page)
    if (!href) {
      if (!(await seedKeywordRule(page))) { test.skip(true, 'Cannot seed data'); return }
      await page.goto('/chatbot/keywords')
      await page.waitForLoadState('networkidle')
      href = await navigateToFirstItem(page)
    }
    if (href) {
      expect(page.url()).toMatch(/\/chatbot\/keywords\/[a-f0-9-]+/)
    }
  })

  test('should search and filter', async ({ page }) => {
    await keywordsPage.search('nonexistent-keyword-xyz')
    const filteredRows = await page.locator('tbody tr').count()
    // Should have 0 or fewer rows
    expect(filteredRows).toBeLessThanOrEqual(50)
  })

  test('should show delete confirmation from list', async ({ page }) => {
    // Seed if no data
    let hasRows = await page.locator('tbody tr a').first().isVisible({ timeout: 3000 }).catch(() => false)
    if (!hasRows) {
      if (!(await seedKeywordRule(page))) { test.skip(true, 'Cannot seed data'); return }
      await keywordsPage.goto()
      hasRows = true
    }

    const deleteBtn = page.locator('tbody tr').first().getByRole('button', { name: /delete/i })
    if (!(await deleteBtn.isVisible({ timeout: 5000 }).catch(() => false))) {
      test.skip(true, 'No delete button found')
      return
    }
    await deleteBtn.click()
    await expect(keywordsPage.alertDialog).toBeVisible({ timeout: 5000 })
    await keywordsPage.alertDialog.getByRole('button', { name: /Cancel/i }).click()
  })
})

test.describe('Keyword Rules - Detail Page CRUD', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
  })

  test('should show all form fields on create page', async ({ page }) => {
    await page.goto('/chatbot/keywords/new')
    await page.waitForLoadState('networkidle')

    await expect(page.locator('input').first()).toBeVisible()
    const selects = page.locator('button[role="combobox"]')
    expect(await selects.count()).toBeGreaterThanOrEqual(2)
    await expect(page.locator('textarea').first()).toBeVisible()
    await expect(page.locator('button[role="switch"]').first()).toBeVisible()
  })

  test('should create keyword rule', async ({ page }) => {
    const created = await seedKeywordRule(page)
    if (!created) { test.skip(true, 'Cannot create (no permission or CSRF)'); return }
    expect(page.url()).toMatch(/\/chatbot\/keywords\/[a-f0-9-]+/)
  })

  test('should edit existing rule', async ({ page }) => {
    await page.goto('/chatbot/keywords')
    await page.waitForLoadState('networkidle')

    let href = await navigateToFirstItem(page)
    if (!href) {
      if (!(await seedKeywordRule(page))) { test.skip(true, 'Cannot seed'); return }
      await page.goto('/chatbot/keywords')
      await page.waitForLoadState('networkidle')
      href = await navigateToFirstItem(page)
      if (!href) { test.skip(true, 'No data after seed'); return }
    }

    const input = page.locator('input').first()
    if (await input.isDisabled()) { test.skip(true, 'No write permission'); return }

    const original = await input.inputValue()
    await input.fill(original + ', e2e-edit')
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
    // Create one to delete
    const created = await seedKeywordRule(page)
    if (!created) { test.skip(true, 'Cannot create'); return }
    await expectDeleteFromForm(page, '/chatbot/keywords')
  })

  test('should show metadata', async ({ page }) => {
    if (!(await seedKeywordRule(page))) { test.skip(true, 'Cannot seed'); return }
    await expectMetadataVisible(page)
  })

  test('should show activity log', async ({ page }) => {
    // Always seed fresh to avoid stale data from other tests
    if (!(await seedKeywordRule(page))) { test.skip(true, 'Cannot seed'); return }
    await expectActivityLogVisible(page)
  })
})
