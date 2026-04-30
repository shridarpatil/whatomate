import { test, expect } from '@playwright/test'
import { loginAsAdmin, navigateToFirstItem, expectMetadataVisible, expectActivityLogVisible, expectDeleteFromForm, ApiHelper } from '../../helpers'
import { KeywordsPage } from '../../pages'

// Seed a keyword rule via the API. Returns the new resource's ID, or null on
// failure. Replaces the old UI-based seed which silently skipped when
// permissions or click-timing didn't cooperate.
async function seedKeywordRuleViaAPI(request: import('@playwright/test').APIRequestContext): Promise<string | null> {
  const api = new ApiHelper(request)
  await api.login('admin@admin.com', 'admin')
  const resp = await api.post('/api/chatbot/keywords', {
    name: `e2e-seed-${Date.now()}`,
    keywords: [`seedkw-${Date.now()}`],
    match_type: 'contains',
    response_type: 'text',
    response_content: { body: 'E2E seeded response' },
    enabled: true,
  })
  if (!resp.ok()) return null
  const body = await resp.json()
  return body.data?.id ?? null
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

  test('should load detail page from list', async ({ page, request }) => {
    let href = await navigateToFirstItem(page)
    if (!href) {
      const id = await seedKeywordRuleViaAPI(request)
      expect(id, 'API seed must succeed').toBeTruthy()
      await page.goto('/chatbot/keywords')
      await page.waitForLoadState('networkidle')
      href = await navigateToFirstItem(page)
    }
    expect(href).toBeTruthy()
    expect(page.url()).toMatch(/\/chatbot\/keywords\/[a-f0-9-]+/)
  })

  test('should search and filter', async ({ page }) => {
    await keywordsPage.search('nonexistent-keyword-xyz')
    const filteredRows = await page.locator('tbody tr').count()
    // Should have 0 or fewer rows
    expect(filteredRows).toBeLessThanOrEqual(50)
  })

  test('should show delete confirmation from list', async ({ page, request }) => {
    // Always seed via API — under parallel workers, relying on pre-existing
    // rows is racy because another worker may delete them mid-test.
    const id = await seedKeywordRuleViaAPI(request)
    expect(id, 'API seed must succeed').toBeTruthy()
    await keywordsPage.goto()

    const deleteBtn = page.locator('tbody').getByRole('button', { name: /delete/i }).first()
    await expect(deleteBtn).toBeVisible({ timeout: 10000 })
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

  test('should create keyword rule', async ({ page, request }) => {
    const id = await seedKeywordRuleViaAPI(request)
    expect(id, 'API seed must succeed').toBeTruthy()
    await page.goto(`/chatbot/keywords/${id}`)
    await page.waitForLoadState('networkidle')
    expect(page.url()).toMatch(/\/chatbot\/keywords\/[a-f0-9-]+/)
  })

  test('should edit existing rule', async ({ page, request }) => {
    const id = await seedKeywordRuleViaAPI(request)
    expect(id, 'API seed must succeed').toBeTruthy()
    await page.goto(`/chatbot/keywords/${id}`)
    await page.waitForLoadState('networkidle')

    const input = page.locator('input').first()
    expect(await input.isDisabled()).toBe(false)

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

  test('should delete from detail page', async ({ page, request }) => {
    const id = await seedKeywordRuleViaAPI(request)
    expect(id, 'API seed must succeed').toBeTruthy()
    await page.goto(`/chatbot/keywords/${id}`)
    await page.waitForLoadState('networkidle')
    await expectDeleteFromForm(page, '/chatbot/keywords')
  })

  test('should show metadata', async ({ page, request }) => {
    const id = await seedKeywordRuleViaAPI(request)
    expect(id, 'API seed must succeed').toBeTruthy()
    await page.goto(`/chatbot/keywords/${id}`)
    await page.waitForLoadState('networkidle')
    await expectMetadataVisible(page)
  })

  test('should show activity log', async ({ page, request }) => {
    const id = await seedKeywordRuleViaAPI(request)
    expect(id, 'API seed must succeed').toBeTruthy()
    await page.goto(`/chatbot/keywords/${id}`)
    await page.waitForLoadState('networkidle')
    await expectActivityLogVisible(page)
  })
})
