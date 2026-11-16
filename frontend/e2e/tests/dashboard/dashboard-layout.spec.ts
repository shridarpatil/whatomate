import { test, expect, type Page } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'

// These tests change the saved layout, so each one restores it before finishing.
test.describe.configure({ mode: 'serial' })

const widgets = (page: Page) => page.locator('[data-testid="dashboard-widget"]')
const names = (page: Page) => widgets(page).locator('h4').allTextContents()
const width = async (page: Page, i: number) => (await widgets(page).nth(i).boundingBox())!.width

test.describe('Dashboard edit layout', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/')
    await page.waitForLoadState('networkidle')
    await expect(widgets(page).nth(1)).toBeVisible({ timeout: 10000 })
    await page.getByRole('button', { name: 'Edit Layout' }).click()
  })

  test('dragging a widget onto another moves it there', async ({ page }) => {
    const before = await names(page)
    await widgets(page).nth(0).dragTo(widgets(page).nth(1))
    await expect.poll(() => names(page)).toEqual([before[1], before[0], ...before.slice(2)])

    await widgets(page).nth(1).dragTo(widgets(page).nth(0))
    await expect.poll(() => names(page)).toEqual(before)
  })

  test('the corner handle resizes a widget by whole columns', async ({ page }) => {
    const first = widgets(page).nth(0)
    const handle = first.locator('[data-resize-handle]')
    const w0 = await width(page, 0)

    // Pointer: drag the handle right by a bit more than one column.
    const grid = (await page.locator('[data-layout-editing]').boundingBox())!
    const box = (await handle.boundingBox())!
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2)
    await page.mouse.down()
    await page.mouse.move(box.x + box.width / 2 + grid.width / 12 * 1.2, box.y + box.height / 2, { steps: 5 })
    await page.mouse.up()
    await expect.poll(() => width(page, 0)).toBeGreaterThan(w0)

    // Keyboard: Left arrow shrinks it back by one column.
    await handle.focus()
    await page.keyboard.press('ArrowLeft')
    await expect.poll(() => width(page, 0)).toBeCloseTo(w0, 0)
  })
})
