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
      // Scope to main: the sidebar user menu also has a "Language" label.
      await expect(page.locator('main').getByText('Language', { exact: true })).toBeVisible()
    })

    test('should show available languages in dropdown', async ({ page }) => {
      await page.goto('/settings')
      await page.waitForLoadState('networkidle')

      // Native language select: Spanish is among its options
      const languageSelect = page.locator('main select:has(option[value="es"])')
      await expect(languageSelect.locator('option', { hasText: 'Español' })).toHaveCount(1)
    })

    test('should switch to Spanish and update UI text', async ({ page }) => {
      await page.goto('/settings')
      await page.waitForLoadState('networkidle')

      // Open language dropdown and select Spanish
      const languageSelect = page.locator('main select:has(option[value="es"])')
      await languageSelect.selectOption('es')

      // Settings page headings should update to Spanish
      await expect(page.getByText('Configuración General')).toBeVisible()
    })

    test('should switch back to English', async ({ page }) => {
      await page.goto('/settings')
      await page.waitForLoadState('networkidle')

      // Switch to Spanish first
      const languageSelect = page.locator('main select:has(option[value="es"])')
      await languageSelect.selectOption('es')
      await expect(page.getByText('Configuración General')).toBeVisible()

      // Switch back to English
      await page.locator('main select:has(option[value="es"])').selectOption('en')
      await expect(page.getByText('General Settings')).toBeVisible()
    })
  })

  test.describe('Persistence', () => {
    test('should persist language preference across page reload', async ({ page }) => {
      await page.goto('/settings')
      await page.waitForLoadState('networkidle')

      // Switch to Spanish
      const languageSelect = page.locator('main select:has(option[value="es"])')
      await languageSelect.selectOption('es')
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

      // Open user menu (in sidebar)
      const userMenuButton = page.locator('aside').getByRole('button').filter({ hasText: /@/ }).first()
      await userMenuButton.click()

      const popoverContent = page.locator('aside [popover]:popover-open')
      await expect(popoverContent.getByText('Language', { exact: true })).toBeVisible()
    })

    test('should switch language from user menu', async ({ page }) => {
      await page.goto('/settings')
      await page.waitForLoadState('networkidle')

      // Open user menu
      const userMenuButton = page.locator('aside').getByRole('button').filter({ hasText: /@/ }).first()
      await userMenuButton.click()

      // The language switcher is a native select inside the open user menu
      const popoverContent = page.locator('aside [popover]:popover-open')
      await popoverContent.locator('select').selectOption('es')

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
      const languageSelect = page.locator('main select:has(option[value="es"])')
      await languageSelect.selectOption('es')

      // Sidebar nav items should be in Spanish
      const sidebar = page.locator('aside')
      await expect(sidebar.getByText('Panel')).toBeVisible() // Dashboard -> Panel
      await expect(sidebar.getByText('Configuración')).toBeVisible() // Settings -> Configuración
    })
  })
})
