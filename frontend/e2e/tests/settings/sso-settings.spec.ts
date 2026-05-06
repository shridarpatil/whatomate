import { test, expect } from '@playwright/test'
import { ApiHelper, loginAsAdmin } from '../../helpers'
import { SUPER_ADMIN } from '../../framework'

/**
 * SSO settings E2E.
 *
 * Drives the provider cards + edit dialog through the UI. Backend security
 * properties (secret never leaks, validation errors, cross-org isolation)
 * are covered by Go tests — those don't need a browser to assert.
 *
 * IMPORTANT: Playwright's `request` fixture is the same APIRequestContext
 * for the whole test (beforeEach + body + afterEach). Calling api.login()
 * more than once on it would fail — the second call has the whm_access
 * cookie set, triggering CSRF middleware, which login itself doesn't
 * satisfy. So we log in exactly once in beforeEach and reuse the same
 * ApiHelper for cleanup.
 */
test.describe('SSO Settings', () => {
  let api: ApiHelper
  // Each test pushes the providers it touched here; afterEach cleans only
  // those, so parallel tests using different provider keys don't race on
  // a shared org-wide reset.
  let touchedProviders: string[] = []

  test.beforeEach(async ({ request }) => {
    api = new ApiHelper(request)
    await api.login(SUPER_ADMIN.email, SUPER_ADMIN.password)
    // Reset to default org so we operate on the same org admin@test.com
    // lives in (the UI is driven as that user).
    const memberships = await api.getMyOrganizations()
    const defaultOrg = memberships.find(m => m.is_default) ?? memberships[0]
    if (defaultOrg) {
      await api.switchOrg(defaultOrg.organization_id)
    }
    touchedProviders = []
  })

  test.afterEach(async () => {
    for (const p of touchedProviders) {
      await api.del(`/api/settings/sso/${p}`).catch(() => {})
    }
  })

  test('settings page renders all provider cards', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/sso')
    await page.waitForLoadState('networkidle')

    await expect(page.getByText(/Google/).first()).toBeVisible()
    await expect(page.getByText(/Microsoft/).first()).toBeVisible()
    await expect(page.getByText(/GitHub/).first()).toBeVisible()
    await expect(page.getByText(/Facebook/).first()).toBeVisible()
    await expect(page.getByText(/Custom OIDC/).first()).toBeVisible()
  })

  test('configuring a provider via dialog shows the Enabled badge', async ({ page }) => {
    touchedProviders.push('github')
    await loginAsAdmin(page)
    await page.goto('/settings/sso')
    await page.waitForLoadState('networkidle')

    // Open the GitHub card's "Set Up" button. The action label is "Set Up"
    // when the provider is unconfigured, "Configure" once it has a row.
    const ghCard = page
      .getByRole('heading', { name: 'GitHub', exact: true })
      .locator('xpath=ancestor::*[contains(@class, "rounded-xl")][1]')
    await ghCard.getByRole('button', { name: /Set Up|Configure/i }).click()

    // Edit dialog opens.
    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()
    await expect(dialog.getByRole('heading', { name: /Configure GitHub/i })).toBeVisible()

    // Fill required fields and toggle Enable on.
    await dialog.locator('#client_id').fill('gh-client-id-e2e')
    await dialog.locator('#client_secret').fill('gh-secret-must-not-be-exposed')

    // The Enable switch lives in a row labeled by sso.enableProvider.
    const enableSwitch = dialog.getByRole('switch').first()
    if ((await enableSwitch.getAttribute('data-state')) !== 'checked') {
      await enableSwitch.click()
    }

    // Save — the dialog closes once the PUT settles.
    await dialog.getByRole('button', { name: /^Save$/i }).click()
    await expect(dialog).not.toBeVisible({ timeout: 10000 })

    // Card now shows the Enabled badge.
    await expect(
      ghCard.getByText('Enabled', { exact: true }),
    ).toBeVisible({ timeout: 10000 })
  })

  test('removing a provider clears the configured state', async ({ page }) => {
    // Use Microsoft so this test doesn't share a provider key with the
    // parallel "configuring a provider" test (which uses GitHub). The
    // afterEach cleans only the providers each test touched.
    touchedProviders.push('microsoft')
    await loginAsAdmin(page)
    await page.goto('/settings/sso')
    await page.waitForLoadState('networkidle')

    const card = page
      .getByRole('heading', { name: 'Microsoft', exact: true })
      .locator('xpath=ancestor::*[contains(@class, "rounded-xl")][1]')
    await card.getByRole('button', { name: /Set Up|Configure/i }).click()

    const dialog = page.getByRole('dialog')
    await dialog.locator('#client_id').fill('ms-id')
    await dialog.locator('#client_secret').fill('ms-secret')
    const enableSwitch = dialog.getByRole('switch').first()
    if ((await enableSwitch.getAttribute('data-state')) !== 'checked') {
      await enableSwitch.click()
    }
    await dialog.getByRole('button', { name: /^Save$/i }).click()
    await expect(dialog).not.toBeVisible({ timeout: 10000 })
    await expect(card.getByText('Enabled', { exact: true })).toBeVisible({ timeout: 10000 })

    // Now remove it. Edit dialog → Remove → confirm in alert dialog.
    await card.getByRole('button', { name: /Configure/i }).click()
    await expect(dialog).toBeVisible()
    await dialog.getByRole('button', { name: /Remove/i }).click()
    const confirm = page.locator('[role="alertdialog"]')
    await expect(confirm).toBeVisible({ timeout: 5000 })
    await confirm.getByRole('button', { name: /Remove|Delete|Confirm/i }).click()

    // No strict response assertion — even under parallel runs the visible
    // end state (Set Up button back, Enabled badge gone) is the actual
    // contract under test.
    await expect(
      card.getByRole('button', { name: /Set Up/i }),
    ).toBeVisible({ timeout: 10000 })
    await expect(card.getByText('Enabled', { exact: true })).toHaveCount(0)
  })

  test('custom provider dialog reveals OIDC URL fields', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/sso')
    await page.waitForLoadState('networkidle')

    const customCard = page
      .getByRole('heading', { name: /Custom OIDC/i })
      .locator('xpath=ancestor::*[contains(@class, "rounded-xl")][1]')
    await customCard.getByRole('button', { name: /Set Up|Configure/i }).click()

    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()

    // Custom provider has three extra inputs only it shows.
    await expect(dialog.locator('#auth_url')).toBeVisible()
    await expect(dialog.locator('#token_url')).toBeVisible()
    await expect(dialog.locator('#user_info_url')).toBeVisible()
  })
})
