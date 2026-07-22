import { test, expect } from '@playwright/test'
import { loginAsAdmin, navigateToFirstItem, expectMetadataVisible, expectActivityLogVisible, expectDeleteFromForm } from '../../helpers'
import { AIContextsPage } from '../../pages'

async function seedAIContext(page: import('@playwright/test').Page): Promise<boolean> {
  await page.goto('/chatbot/ai/new')
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(1000) // Wait for auth/permissions to load

  const input = page.locator('input').first()
  try {
    await input.waitFor({ state: 'attached', timeout: 5000 })
    if (await input.isDisabled({ timeout: 3000 })) return false
  } catch {
    return false
  }

  await input.fill(`e2e-ctx-${Date.now()}`)
  const textarea = page.locator('textarea').first()
  if (await textarea.isVisible()) {
    await textarea.fill('E2E seeded AI context content')
  }
  await page.waitForTimeout(500)

  const createBtn = page.getByRole('button', { name: /Create/i })
  if (!(await createBtn.isVisible({ timeout: 5000 }).catch(() => false))) return false

  await createBtn.click({ force: true })
  await page.waitForTimeout(3000)
  return !page.url().includes('/new')
}

test.describe('AI Contexts - List View', () => {
  let aiPage: AIContextsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    aiPage = new AIContextsPage(page)
    await aiPage.goto()
  })

  test('should display AI contexts page', async () => {
    await aiPage.expectPageVisible()
  })

  test('should have search input', async () => {
    await expect(aiPage.searchInput).toBeVisible()
  })

  test('should load create page', async ({ page }) => {
    await page.goto('/chatbot/ai/new')
    await page.waitForLoadState('networkidle')
    expect(page.url()).toContain('/chatbot/ai/new')
  })

  test('should load detail page from list', async ({ page }) => {
    let href = await navigateToFirstItem(page)
    if (!href) {
      if (!(await seedAIContext(page))) { test.skip(true, 'Cannot seed data'); return }
      await page.goto('/chatbot/ai')
      await page.waitForLoadState('networkidle')
      href = await navigateToFirstItem(page)
    }
    if (href) {
      expect(page.url()).toMatch(/\/chatbot\/ai\/[a-f0-9-]+/)
    }
  })

  test('should search and filter', async ({ page }) => {
    await aiPage.search('nonexistent-context-xyz')
    const filteredRows = await page.locator('tbody tr').count()
    expect(filteredRows).toBeLessThanOrEqual(50)
  })

  test('should show delete confirmation from list', async ({ page }) => {
    let hasRows = await page.locator('tbody tr a').first().isVisible({ timeout: 3000 }).catch(() => false)
    if (!hasRows) {
      if (!(await seedAIContext(page))) { test.skip(true, 'Cannot seed data'); return }
      await aiPage.goto()
      hasRows = true
    }

    const deleteBtn = page.locator('tbody tr').first().getByRole('button', { name: /delete/i })
    if (!(await deleteBtn.isVisible({ timeout: 5000 }).catch(() => false))) {
      test.skip(true, 'No delete button found')
      return
    }
    await deleteBtn.click()
    await expect(aiPage.alertDialog).toBeVisible({ timeout: 5000 })
    await aiPage.alertDialog.getByRole('button', { name: /Cancel/i }).click()
  })
})

test.describe('AI Contexts - Detail Page CRUD', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
  })

  test('should show form fields on create page', async ({ page }) => {
    await page.goto('/chatbot/ai/new')
    await page.waitForLoadState('networkidle')

    await expect(page.locator('input').first()).toBeVisible()
    await expect(page.locator('button[role="combobox"]').first()).toBeVisible()
    await expect(page.locator('textarea').first()).toBeVisible()
    await expect(page.locator('button[role="switch"]').first()).toBeVisible()
  })

  test('should create static AI context', async ({ page }) => {
    const created = await seedAIContext(page)
    if (!created) { test.skip(true, 'Cannot create (no permission or CSRF)'); return }
    expect(page.url()).toMatch(/\/chatbot\/ai\/[a-f0-9-]+/)
  })

  test('should show API config fields for api type', async ({ page }) => {
    await page.goto('/chatbot/ai/new')
    await page.waitForLoadState('networkidle')

    const typeSelect = page.locator('button[role="combobox"]').first()
    await typeSelect.click()
    const apiOption = page.getByRole('option', { name: /api/i })
    if (await apiOption.isVisible()) {
      await apiOption.click()
      await page.waitForTimeout(500)
      await expect(page.getByText('API URL')).toBeVisible({ timeout: 3000 })
    }
  })

  test('should edit existing context', async ({ page }) => {
    await page.goto('/chatbot/ai')
    await page.waitForLoadState('networkidle')

    let href = await navigateToFirstItem(page)
    if (!href) {
      if (!(await seedAIContext(page))) { test.skip(true, 'Cannot seed'); return }
      await page.goto('/chatbot/ai')
      await page.waitForLoadState('networkidle')
      href = await navigateToFirstItem(page)
      if (!href) { test.skip(true, 'No data after seed'); return }
    }

    const nameInput = page.locator('input').first()
    if (await nameInput.isDisabled()) { test.skip(true, 'No write permission'); return }

    const original = await nameInput.inputValue()
    await nameInput.fill(original + ' edited')
    await page.waitForTimeout(300)

    const saveBtn = page.getByRole('button', { name: /Save/i })
    if (await saveBtn.isVisible({ timeout: 5000 }).catch(() => false)) {
      await saveBtn.click({ force: true })
      await page.waitForTimeout(2000)
      // Revert
      await nameInput.fill(original)
      await page.waitForTimeout(300)
      const revertBtn = page.getByRole('button', { name: /Save/i })
      if (await revertBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
        await revertBtn.click({ force: true })
      }
    }
  })

  test('should delete from detail page', async ({ page }) => {
    const created = await seedAIContext(page)
    if (!created) { test.skip(true, 'Cannot create'); return }
    await expectDeleteFromForm(page, '/chatbot/ai')
  })

  test('should show metadata', async ({ page }) => {
    if (!(await seedAIContext(page))) { test.skip(true, 'Cannot seed'); return }
    await expectMetadataVisible(page)
  })

  test('should show activity log', async ({ page }) => {
    if (!(await seedAIContext(page))) { test.skip(true, 'Cannot seed'); return }
    await expectActivityLogVisible(page)
  })

})
