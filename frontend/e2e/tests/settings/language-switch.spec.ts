import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'

test.describe('Language Switching', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    // Clear any saved locale preference
    await page.evaluate(() => localStorage.removeItem('locale'))
  })

  test.afterEach(async ({ page }) => {
    // Reset to English after each test
    await page.evaluate(() => {
      localStorage.setItem('locale', 'en')
    })
  })

  test.describe('Settings Page Language Selector', () => {
    test('should display language selector on settings page', async ({ page }) => {
      await page.goto('/settings')
      await page.waitForLoadState('networkidle')
      await expect(page.getByText('Language', { exact: true })).toBeVisible()
    })

    test('should show available languages in dropdown', async ({ page }) => {
      await page.goto('/settings')
      await page.waitForLoadState('networkidle')

      // Find the language switcher select (the one with Globe icon nearby)
      const languageSelect = page.locator('button[role="combobox"]').filter({ hasText: /English/ })
      await languageSelect.click()

      await expect(page.locator('[role="option"]').filter({ hasText: 'Español' })).toBeVisible()
      await page.keyboard.press('Escape')
    })

    test('should switch to Spanish and update UI text', async ({ page }) => {
      await page.goto('/settings')
      await page.waitForLoadState('networkidle')

      // Open language dropdown and select Spanish
      const languageSelect = page.locator('button[role="combobox"]').filter({ hasText: /English/ })
      await languageSelect.click()
      await page.locator('[role="option"]').filter({ hasText: 'Español' }).click()

      // Settings page headings should update to Spanish
      await expect(page.getByText('Configuración General')).toBeVisible()
    })

    test('should switch back to English', async ({ page }) => {
      await page.goto('/settings')
      await page.waitForLoadState('networkidle')

      // Switch to Spanish first
      const languageSelect = page.locator('button[role="combobox"]').filter({ hasText: /English/ })
      await languageSelect.click()
      await page.locator('[role="option"]').filter({ hasText: 'Español' }).click()
      await expect(page.getByText('Configuración General')).toBeVisible()

      // Switch back to English
      const spanishSelect = page.locator('button[role="combobox"]').filter({ hasText: /Español/ })
      await spanishSelect.click()
      await page.locator('[role="option"]').filter({ hasText: 'English' }).click()
      await expect(page.getByText('General Settings')).toBeVisible()
    })
  })

  test.describe('Persistence', () => {
    test('should persist language preference across page reload', async ({ page }) => {
      await page.goto('/settings')
      await page.waitForLoadState('networkidle')

      // Switch to Spanish
      const languageSelect = page.locator('button[role="combobox"]').filter({ hasText: /English/ })
      await languageSelect.click()
      await page.locator('[role="option"]').filter({ hasText: 'Español' }).click()
      await expect(page.getByText('Configuración General')).toBeVisible()

      // Verify localStorage was set
      const savedLocale = await page.evaluate(() => localStorage.getItem('locale'))
      expect(savedLocale).toBe('es')

      // Reload page
      await page.reload()
      await page.waitForLoadState('networkidle')

      // Should still be in Spanish
      await expect(page.getByText('Configuración General')).toBeVisible()
    })
  })

  test.describe('User Menu Language Switcher', () => {
    test('should display language switcher in user menu', async ({ page }) => {
      await page.goto('/settings')
      await page.waitForLoadState('networkidle')

      // Open user menu popover (in sidebar)
      const userMenuButton = page.locator('aside').getByRole('button').filter({ hasText: /@/ }).first()
      await userMenuButton.click()

      // The popover is portaled by Radix outside aside
      const popoverContent = page.locator('[data-state="open"][data-side]')
      await expect(popoverContent.getByText('Language', { exact: true })).toBeVisible()
    })

    test('should switch language from user menu', async ({ page }) => {
      await page.goto('/settings')
      await page.waitForLoadState('networkidle')

      // Open user menu
      const userMenuButton = page.locator('aside').getByRole('button').filter({ hasText: /@/ }).first()
      await userMenuButton.click()

      // The language switcher is in the popover that appears in the sidebar area
      // Find the combobox within the popover content
      const popoverContent = page.locator('[data-state="open"][data-side]')
      const languageSelect = popoverContent.locator('button[role="combobox"]')
      await languageSelect.click()

      // Select Spanish
      await page.locator('[role="option"]').filter({ hasText: 'Español' }).click()

      // Close user menu by pressing Escape
      await page.keyboard.press('Escape')

      // Navigate to settings to verify the switch persisted
      await page.goto('/settings')
      await page.waitForLoadState('networkidle')
      await expect(page.getByText('Configuración General')).toBeVisible()
    })
  })

  test.describe('Navigation Labels', () => {
    test('should update sidebar navigation when language changes', async ({ page }) => {
      await page.goto('/settings')
      await page.waitForLoadState('networkidle')

      // Switch to Spanish via settings page
      const languageSelect = page.locator('button[role="combobox"]').filter({ hasText: /English/ })
      await languageSelect.click()
      await page.locator('[role="option"]').filter({ hasText: 'Español' }).click()

      // Sidebar nav items should be in Spanish
      const sidebar = page.locator('aside')
      await expect(sidebar.getByText('Panel')).toBeVisible() // Dashboard -> Panel
      await expect(sidebar.getByText('Configuración')).toBeVisible() // Settings -> Configuración
    })
  })
})
