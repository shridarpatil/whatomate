import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'

// An 8x8 emerald PNG.
const PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAgAAAAICAYAAADED76LAAAAEklEQVR4nGMQ2Nn4Hx9mGBkKABvNkkHg+UoaAAAAAElFTkSuQmCC',
  'base64',
)

// Changes the organization's logo, so it removes it again before finishing.
test.describe.configure({ mode: 'serial' })

test.describe('Organization logo', () => {
  test('uploading a logo shows it in the sidebar; removing it restores the default', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings')
    const sidebarLogo = page.locator('aside [data-testid="org-switcher"] [data-org-trigger] [data-org-logo]')
    await expect(sidebarLogo).toHaveAttribute('src', /favicon\.svg$/)

    await page.getByLabel('Organization logo').setInputFiles({ name: 'logo.png', mimeType: 'image/png', buffer: PNG })
    await expect(sidebarLogo).toHaveAttribute('src', /^data:image\/(webp|png);base64,/, { timeout: 10000 })

    // Persisted: still there after a reload.
    await page.reload()
    await expect(sidebarLogo).toHaveAttribute('src', /^data:image\//, { timeout: 10000 })

    await page.getByRole('button', { name: 'Remove' }).click()
    await expect(sidebarLogo).toHaveAttribute('src', /favicon\.svg$/, { timeout: 10000 })
  })

  test('rejects an SVG file', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings')
    await page.getByLabel('Organization logo').setInputFiles({ name: 'logo.svg', mimeType: 'image/svg+xml', buffer: Buffer.from('<svg xmlns="http://www.w3.org/2000/svg"/>') })
    await expect(page.getByText('Use a PNG, JPEG or WebP image')).toBeVisible()
  })
})
