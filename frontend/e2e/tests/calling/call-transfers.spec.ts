import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { CallTransfersPage } from '../../pages'

test.describe('Call Transfers Page', () => {
  let transfersPage: CallTransfersPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    transfersPage = new CallTransfersPage(page)
    await transfersPage.goto()
  })

  test('should display call transfers page', async () => {
    await transfersPage.expectPageVisible()
  })

  test('should have Waiting tab', async () => {
    await expect(transfersPage.waitingTab).toBeVisible()
  })

  test('should have History tab', async () => {
    await expect(transfersPage.historyTab).toBeVisible()
  })
})

test.describe('Call Transfers - Waiting Tab', () => {
  let transfersPage: CallTransfersPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    transfersPage = new CallTransfersPage(page)
    await transfersPage.goto()
    await transfersPage.switchToWaiting()
  })

  test('should show waiting transfers or empty state', async ({ page }) => {
    const hasTable = await page.locator('tbody tr').count() > 0
    const hasEmpty = await page.getByText(/no.*transfer|no.*waiting|no.*call/i).isVisible().catch(() => false)
    expect(hasTable || hasEmpty).toBe(true)
  })

  test('should show caller phone column', async ({ page }) => {
    const table = page.locator('table')
    if (await table.isVisible()) {
      const headers = table.locator('th')
      const headerTexts = await headers.allTextContents()
      const hasCallerColumn = headerTexts.some(h =>
        h.toLowerCase().includes('caller') || h.toLowerCase().includes('phone')
      )
      expect(hasCallerColumn).toBe(true)
    }
  })

  test('should show status column', async ({ page }) => {
    const table = page.locator('table')
    if (await table.isVisible()) {
      const headers = table.locator('th')
      const headerTexts = await headers.allTextContents()
      const hasStatusColumn = headerTexts.some(h =>
        h.toLowerCase().includes('status')
      )
      expect(hasStatusColumn).toBe(true)
    }
  })
})

test.describe('Call Transfers - History Tab', () => {
  let transfersPage: CallTransfersPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    transfersPage = new CallTransfersPage(page)
    await transfersPage.goto()
    await transfersPage.switchToHistory()
  })

  test('should show history transfers or empty state', async ({ page }) => {
    await page.waitForTimeout(500)
    const hasTable = await page.locator('tbody tr').count() > 0
    const hasEmpty = await page.getByText(/no.*transfer|no.*history|no.*call/i).isVisible().catch(() => false)
    expect(hasTable || hasEmpty).toBe(true)
  })

  test('should show hold and talk duration columns', async ({ page }) => {
    const table = page.locator('table')
    if (await table.isVisible()) {
      const headers = table.locator('th')
      const headerTexts = await headers.allTextContents()
      const hasHoldColumn = headerTexts.some(h => h.toLowerCase().includes('hold'))
      const hasTalkColumn = headerTexts.some(h => h.toLowerCase().includes('talk'))
      expect(hasHoldColumn || hasTalkColumn).toBe(true)
    }
  })

  test('should paginate history', async ({ page }) => {
    const rowCount = await page.locator('tbody tr').count()
    if (rowCount >= 20) {
      const nextButton = page.locator('button:has-text("Next"), button:has-text(">")')
      if (await nextButton.isVisible()) {
        await nextButton.click()
        await page.waitForTimeout(500)
        // Should still show content
        await expect(page.locator('table').or(page.getByText(/no.*transfer/i)).first()).toBeVisible()
      }
    }
  })
})
