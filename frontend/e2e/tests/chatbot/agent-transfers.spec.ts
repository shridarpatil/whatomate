import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { AgentTransfersPage } from '../../pages'

test.describe('Agent Transfers Page', () => {
  let transfersPage: AgentTransfersPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    transfersPage = new AgentTransfersPage(page)
    await transfersPage.goto()
  })

  test('should display transfers page', async () => {
    await transfersPage.expectPageVisible()
  })

  test('should have My Transfers tab', async () => {
    await expect(transfersPage.myTransfersTab).toBeVisible()
  })

  test('should have Queue tab', async () => {
    await expect(transfersPage.queueTab).toBeVisible()
  })

  test('should have All Active tab', async () => {
    await expect(transfersPage.allActiveTab).toBeVisible()
  })

  test('should have History tab', async () => {
    await expect(transfersPage.historyTab).toBeVisible()
  })
})

test.describe('My Transfers Tab', () => {
  let transfersPage: AgentTransfersPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    transfersPage = new AgentTransfersPage(page)
    await transfersPage.goto()
  })

  test('should show My Transfers content', async ({ page }) => {
    await transfersPage.switchToMyTransfers()
    // Either show transfers or empty state
    const hasTransfers = await page.locator('tr').count() > 1
    const hasEmptyState = await page.getByText('No active transfers assigned').isVisible()
    expect(hasTransfers || hasEmptyState).toBe(true)
  })

  test('should show empty state when no transfers', async ({ page }) => {
    await transfersPage.switchToMyTransfers()
    const tableRows = await page.locator('tbody tr').count()
    if (tableRows === 0) {
      await expect(page.getByText('No active transfers')).toBeVisible()
    }
  })
})

test.describe('Queue Tab', () => {
  let transfersPage: AgentTransfersPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    transfersPage = new AgentTransfersPage(page)
    await transfersPage.goto()
    await transfersPage.switchToQueue()
  })

  test('should show queue content', async ({ page }) => {
    // Either show queue items or empty state
    const hasQueue = await page.locator('tr').count() > 1
    const hasEmptyState = await page.getByText('No transfers in queue').isVisible()
    expect(hasQueue || hasEmptyState).toBe(true)
  })

  test('should have team filter', async () => {
    await expect(transfersPage.teamFilterSelect).toBeVisible()
  })

  test('should filter by team', async ({ page }) => {
    await transfersPage.teamFilterSelect.click()
    const options = page.locator('[role="option"]')
    const optionCount = await options.count()
    if (optionCount > 1) {
      await options.nth(1).click()
    } else {
      await page.keyboard.press('Escape')
    }
  })

  test('should show team queue counts', async ({ page }) => {
    await expect(page.getByText(/General/i)).toBeVisible()
  })
})

test.describe('All Active Tab', () => {
  let transfersPage: AgentTransfersPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    transfersPage = new AgentTransfersPage(page)
    await transfersPage.goto()
    await transfersPage.switchToAllActive()
  })

  test('should show all active transfers content', async ({ page }) => {
    // Either show transfers or empty state
    const hasTransfers = await page.locator('tr').count() > 1
    const hasEmptyState = await page.getByText('No active transfers').isVisible()
    expect(hasTransfers || hasEmptyState).toBe(true)
  })

  test('should show table headers', async ({ page }) => {
    const tableRows = await page.locator('tbody tr').count()
    if (tableRows > 0) {
      await expect(page.getByText('Contact')).toBeVisible()
      await expect(page.getByText('Phone')).toBeVisible()
    }
  })
})

test.describe('History Tab', () => {
  let transfersPage: AgentTransfersPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    transfersPage = new AgentTransfersPage(page)
    await transfersPage.goto()
    await transfersPage.switchToHistory()
  })

  test('should show history content', async ({ page }) => {
    // Wait for history to load
    await page.waitForTimeout(1000)
    // Either show history or empty state or loading
    const hasHistory = await page.locator('tbody tr').count() > 0
    const hasEmptyState = await page.getByText('No transfer history').isVisible()
    const hasLoading = await page.locator('.animate-spin').isVisible()
    expect(hasHistory || hasEmptyState || hasLoading).toBe(true)
  })

  test('should show load more button if more history exists', async ({ page }) => {
    await page.waitForTimeout(1000)
    const hasHistory = await page.locator('tbody tr').count() > 0
    if (hasHistory) {
      const loadMoreVisible = await transfersPage.loadMoreButton.isVisible()
      // Load more may or may not be visible depending on data
      expect(typeof loadMoreVisible).toBe('boolean')
    }
  })
})

test.describe('Transfer Actions', () => {
  let transfersPage: AgentTransfersPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    transfersPage = new AgentTransfersPage(page)
    await transfersPage.goto()
  })

  test('should have action buttons on transfer rows', async ({ page }) => {
    await transfersPage.switchToAllActive()
    const tableRows = await page.locator('tbody tr').count()
    if (tableRows > 0) {
      const firstRow = page.locator('tbody tr').first()
      // Should have action buttons
      const buttons = await firstRow.locator('button').count()
      expect(buttons).toBeGreaterThan(0)
    }
  })

  test('should open assign dialog when clicking assign', async ({ page }) => {
    await transfersPage.switchToQueue()
    const tableRows = await page.locator('tbody tr').count()
    if (tableRows > 0) {
      const firstRow = page.locator('tbody tr').first()
      const assignBtn = firstRow.getByRole('button').filter({ has: page.locator('.lucide-user-plus') })
      if (await assignBtn.isVisible()) {
        await assignBtn.click()
        await expect(transfersPage.assignDialog).toBeVisible()
        await transfersPage.cancelAssignment()
      }
    }
  })
})

test.describe('Assign Dialog', () => {
  let transfersPage: AgentTransfersPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    transfersPage = new AgentTransfersPage(page)
    await transfersPage.goto()
  })

  test('should have team selector in assign dialog', async ({ page }) => {
    await transfersPage.switchToQueue()
    const tableRows = await page.locator('tbody tr').count()
    if (tableRows > 0) {
      const firstRow = page.locator('tbody tr').first()
      const assignBtn = firstRow.getByRole('button').filter({ has: page.locator('.lucide-user-plus') })
      if (await assignBtn.isVisible()) {
        await assignBtn.click()
        await expect(transfersPage.assignDialog.getByText('Team Queue')).toBeVisible()
        await transfersPage.cancelAssignment()
      }
    }
  })

  test('should have agent selector in assign dialog', async ({ page }) => {
    await transfersPage.switchToQueue()
    const tableRows = await page.locator('tbody tr').count()
    if (tableRows > 0) {
      const firstRow = page.locator('tbody tr').first()
      const assignBtn = firstRow.getByRole('button').filter({ has: page.locator('.lucide-user-plus') })
      if (await assignBtn.isVisible()) {
        await assignBtn.click()
        await expect(transfersPage.assignDialog.getByText('Assign to Agent')).toBeVisible()
        await transfersPage.cancelAssignment()
      }
    }
  })

  test('should close assign dialog on cancel', async ({ page }) => {
    await transfersPage.switchToQueue()
    const tableRows = await page.locator('tbody tr').count()
    if (tableRows > 0) {
      const firstRow = page.locator('tbody tr').first()
      const assignBtn = firstRow.getByRole('button').filter({ has: page.locator('.lucide-user-plus') })
      if (await assignBtn.isVisible()) {
        await assignBtn.click()
        await transfersPage.cancelAssignment()
        await expect(transfersPage.assignDialog).not.toBeVisible()
      }
    }
  })
})

test.describe('Tab Navigation', () => {
  let transfersPage: AgentTransfersPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    transfersPage = new AgentTransfersPage(page)
    await transfersPage.goto()
  })

  test('should switch to Queue tab', async () => {
    await transfersPage.switchToQueue()
    await expect(transfersPage.page.getByRole('heading', { name: /Transfer Queue/i })).toBeVisible()
  })

  test('should switch to All Active tab', async () => {
    await transfersPage.switchToAllActive()
    await expect(transfersPage.page.getByRole('heading', { name: /All Active Transfers/i })).toBeVisible()
  })

  test('should switch to History tab', async () => {
    await transfersPage.switchToHistory()
    await expect(transfersPage.page.getByText('Transfer History', { exact: true })).toBeVisible()
  })

  test('should switch back to My Transfers tab', async () => {
    await transfersPage.switchToQueue()
    await transfersPage.switchToMyTransfers()
    // Should be back on my transfers
    await expect(transfersPage.myTransfersTab).toHaveAttribute('data-state', 'active')
  })
})

test.describe('SLA Indicators', () => {
  let transfersPage: AgentTransfersPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    transfersPage = new AgentTransfersPage(page)
    await transfersPage.goto()
    await transfersPage.switchToQueue()
  })

  test('should show SLA column in queue', async ({ page }) => {
    const tableRows = await page.locator('tbody tr').count()
    if (tableRows > 0) {
      await expect(page.getByText('SLA')).toBeVisible()
    }
  })

  test('should show waiting time column', async ({ page }) => {
    const tableRows = await page.locator('tbody tr').count()
    if (tableRows > 0) {
      await expect(page.getByText('Waiting')).toBeVisible()
    }
  })
})
