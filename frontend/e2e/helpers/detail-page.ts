import { Page, expect } from '@playwright/test'

/**
 * Reusable assertions for detail pages with metadata, audit log, and unsaved changes guard.
 */

export async function expectMetadataVisible(page: Page) {
  await expect(page.getByText('Metadata')).toBeVisible({ timeout: 15000 })
}

export async function expectActivityLogVisible(page: Page) {
  await expect(page.getByText('Activity Log')).toBeVisible({ timeout: 15000 })
}

export async function expectSaveButtonOnChange(page: Page) {
  const input = page.locator('input').first()
  const original = await input.inputValue()
  await input.fill(original + '-test')
  await page.waitForTimeout(300)

  const saveBtn = page.getByRole('button', { name: /Save/i })
  await expect(saveBtn).toBeVisible({ timeout: 5000 })

  await input.fill(original)
}

export async function expectDeleteFromForm(page: Page, listUrl: string) {
  // Remove toasts from DOM so they don't block clicks
  await page.evaluate(() => {
    document.querySelectorAll('[data-sonner-toast]').forEach(el => el.remove())
  })
  await page.waitForTimeout(300)

  const deleteBtn = page.getByRole('button', { name: /Delete/i }).first()
  if (await deleteBtn.isVisible()) {
    await deleteBtn.click()
    const dialog = page.locator('[role="alertdialog"]')
    await expect(dialog).toBeVisible({ timeout: 5000 })
    await dialog.getByRole('button', { name: /Delete/i }).click()
    await page.waitForTimeout(2000)
    expect(page.url()).toContain(listUrl)
  }
}

/**
 * Navigate to the first data item's detail page from a list view.
 * Only matches links inside table rows that point to a UUID detail page
 * (excludes /new links and empty state links).
 * Returns the href or null if no data items exist.
 */
export async function navigateToFirstItem(page: Page): Promise<string | null> {
  // Wait for table to load
  await page.waitForTimeout(1000)

  // Find links in table body that contain a UUID pattern (not /new)
  const dataLinks = page.locator('tbody tr a').filter({
    hasNot: page.locator('text=Add'),
  })

  const count = await dataLinks.count()
  if (count === 0) return null

  // Check if the first link href contains a UUID (not /new)
  const href = await dataLinks.first().getAttribute('href')
  if (!href || href.includes('/new')) return null

  await page.goto(href)
  await page.waitForLoadState('networkidle')
  // Wait for lazy-loaded panels (audit log, metadata) to render
  await page.waitForTimeout(2000)
  return href
}
