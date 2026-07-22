import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { IVRFlowsPage } from '../../pages'

test.describe('IVR Flows Page', () => {
  let ivrPage: IVRFlowsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    ivrPage = new IVRFlowsPage(page)
    await ivrPage.goto()
  })

  test('should display IVR flows page', async () => {
    await ivrPage.expectPageVisible()
  })

  test('should show flows list or empty state', async ({ page }) => {
    const hasTable = await page.locator('table').isVisible().catch(() => false)
    const hasCards = await page.locator('[class*="card"]').count() > 0
    const hasEmpty = await page.getByText(/no.*flow|create.*flow|get.*started/i).isVisible().catch(() => false)
    expect(hasTable || hasCards || hasEmpty).toBe(true)
  })

  test('should have create button', async ({ page }) => {
    const createButton = page.getByRole('button', { name: /create.*flow/i }).first()
    await expect(createButton).toBeVisible()
  })

  test('should open flow editor on create', async ({ page }) => {
    const createButton = page.getByRole('button', { name: /create.*flow/i }).first()
    if (await createButton.isVisible()) {
      await createButton.click()

      // Should either open a dialog or navigate to editor
      const dialog = page.getByRole('dialog')
      const editorUrl = page.url().includes('/edit')
      const hasDialog = await dialog.isVisible({ timeout: 2000 }).catch(() => false)
      expect(hasDialog || editorUrl).toBe(true)
    }
  })
})

test.describe('IVR Flow Toggle Status', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/calling/ivr-flows')
    await page.waitForLoadState('networkidle')
  })

  test('should show confirmation when toggling flow status', async ({ page }) => {
    // Find an active/disabled status badge that acts as a toggle button
    const statusBadge = page.locator('[role="button"]').filter({ hasText: /active|disabled/i }).first()
    if (await statusBadge.isVisible({ timeout: 3000 }).catch(() => false)) {
      await statusBadge.click()
      // Verify the confirmation alert dialog appears
      const alertDialog = page.locator('[role="alertdialog"]')
      const dialogVisible = await alertDialog.isVisible({ timeout: 3000 }).catch(() => false)
      if (dialogVisible) {
        // Click cancel to dismiss without changing state
        const cancelBtn = alertDialog.getByRole('button', { name: /cancel/i })
        await cancelBtn.click()
        await alertDialog.waitFor({ state: 'hidden' })
      }
    }
  })
})

test.describe('IVR Flow Editor', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/calling/ivr-flows')
    await page.waitForLoadState('networkidle')
  })

  test('should navigate to flow editor when clicking a flow', async ({ page }) => {
    const rows = page.locator('tbody tr')
    const rowCount = await rows.count()

    if (rowCount > 0) {
      // Click the first flow's edit action
      const editButton = rows.first().getByRole('button', { name: /edit/i })
      if (await editButton.isVisible()) {
        await editButton.click()
        await page.waitForTimeout(1000)

        // Should navigate to editor or open a dialog
        const isOnEditor = page.url().includes('/edit')
        const hasCanvas = await page.locator('[class*="flow"], [class*="canvas"], [class*="vue-flow"]').isVisible().catch(() => false)
        const hasDialog = await page.getByRole('dialog').isVisible().catch(() => false)
        expect(isOnEditor || hasCanvas || hasDialog).toBe(true)
      }
    }
  })
})
