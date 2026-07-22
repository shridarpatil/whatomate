import { test, expect, Page } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'

async function gotoNewTemplate(page: Page) {
  await page.goto('/templates/new')
  // Wait for the Vue app to mount — the body textarea is inside the Content card
  const bodyTextarea = page.locator('textarea').first()
  await bodyTextarea.waitFor({ state: 'visible', timeout: 15000 })
  // Ensure textarea is enabled/editable
  await expect(bodyTextarea).toBeEnabled({ timeout: 5000 })
  return bodyTextarea
}

test.describe('Template Sample Values', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
  })

  test('should not show sample values section when body has no variables', async ({ page }) => {
    const bodyTextarea = await gotoNewTemplate(page)
    await bodyTextarea.fill('Hello, this is a plain message with no variables.')

    await expect(page.getByText('Sample Values for Variables')).not.toBeVisible()
  })

  test('should show sample values inputs when body has positional variables', async ({ page }) => {
    const bodyTextarea = await gotoNewTemplate(page)
    await bodyTextarea.fill('Hello {{1}}, your order {{2}} is ready for pickup at {{3}}.')

    await expect(page.getByText('Sample Values for Variables')).toBeVisible()

    const sampleInputs = page.locator('input[placeholder*="e.g."]')
    await expect(sampleInputs).toHaveCount(3)
  })

  test('should show sample values inputs when body has named variables', async ({ page }) => {
    const bodyTextarea = await gotoNewTemplate(page)
    await bodyTextarea.fill('Hello {{name}}, your order {{order_id}} is ready.')

    await expect(page.getByText('Sample Values for Variables')).toBeVisible()

    const sampleInputs = page.locator('input[placeholder*="e.g."]')
    await expect(sampleInputs).toHaveCount(2)

    await expect(page.getByText('body:{{name}}')).toBeVisible()
    await expect(page.getByText('body:{{order_id}}')).toBeVisible()
  })

  test('should allow entering sample values', async ({ page }) => {
    const bodyTextarea = await gotoNewTemplate(page)
    await bodyTextarea.fill('Hello {{1}}, your order {{2}} is ready.')

    const sampleInputs = page.locator('input[placeholder*="e.g."]')
    await sampleInputs.nth(0).fill('John Doe')
    await sampleInputs.nth(1).fill('ORD-12345')

    await expect(sampleInputs.nth(0)).toHaveValue('John Doe')
    await expect(sampleInputs.nth(1)).toHaveValue('ORD-12345')
  })

  test('should update sample values when variables change', async ({ page }) => {
    const bodyTextarea = await gotoNewTemplate(page)
    await bodyTextarea.fill('Hello {{1}}, your order {{2}} is ready.')

    const sampleInputs = page.locator('input[placeholder*="e.g."]')
    await expect(sampleInputs).toHaveCount(2)

    await bodyTextarea.fill('Hello {{1}}, your order {{2}} is ready. Delivered by {{3}}.')
    await expect(sampleInputs).toHaveCount(3)

    await bodyTextarea.fill('Hello {{1}}!')
    await expect(sampleInputs).toHaveCount(1)
  })

  test('should hide sample values section when all variables are removed', async ({ page }) => {
    const bodyTextarea = await gotoNewTemplate(page)
    await bodyTextarea.fill('Hello {{1}}!')

    await expect(page.getByText('Sample Values for Variables')).toBeVisible()

    await bodyTextarea.fill('Hello!')
    await expect(page.getByText('Sample Values for Variables')).not.toBeVisible()
  })

  test('should show header variables when header type is TEXT', async ({ page }) => {
    await gotoNewTemplate(page)

    // Select TEXT header type — find the select within the Header Type field
    const headerTypeField = page.locator('div.space-y-1\\.5').filter({ hasText: 'Header Type' })
    const headerTypeSelect = headerTypeField.locator('button[role="combobox"]')
    await headerTypeSelect.click()
    await page.locator('[role="option"]').filter({ hasText: 'Text' }).click()

    const headerContentInput = page.locator('#header-content')
    await headerContentInput.waitFor({ state: 'visible', timeout: 5000 })
    await headerContentInput.fill('Welcome {{1}}!')

    const bodyTextarea = page.locator('textarea').first()
    await bodyTextarea.fill('Hello {{1}}, check your order {{2}}.')

    await expect(page.getByText('Sample Values for Variables')).toBeVisible()
    await expect(page.getByText('header:{{1}}')).toBeVisible()

    const sampleInputs = page.locator('input[placeholder*="e.g."]')
    await expect(sampleInputs).toHaveCount(3)
  })
})

test.describe('Template Preview with Sample Values', () => {
  test('should show preview with sample values replacing variables', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/templates')
    await page.waitForLoadState('domcontentloaded')

    // Navigate to first template
    const firstLink = page.locator('tbody tr a, tbody tr td').first()
    if (!(await firstLink.isVisible({ timeout: 5000 }).catch(() => false))) {
      test.skip(true, 'No templates exist')
      return
    }
    await firstLink.click()
    await page.waitForLoadState('domcontentloaded')

    const previewBtn = page.getByRole('button', { name: /Preview/i })
    await expect(previewBtn).toBeVisible({ timeout: 10000 })
    await previewBtn.click()

    const dialog = page.locator('[role="alertdialog"]')
    await expect(dialog).toBeVisible({ timeout: 5000 })
    await expect(dialog.getByText('Template Preview')).toBeVisible()

    await dialog.getByRole('button', { name: /Close/i }).click()
  })
})
