import { test, expect } from '@playwright/test'
import { ApiHelper, loginAsAdmin } from '../../helpers'
import { createTestScope, SUPER_ADMIN } from '../../framework'

const scope = createTestScope('tpl-header-media')

/**
 * Regression coverage for issue #355 — IMAGE/VIDEO/DOCUMENT header templates
 * lost their Meta upload handle on save, then failed publish validation.
 *
 * Two angles:
 *  1. API persistence: saving a template with a media header preserves
 *     header_content (the Meta resumable-upload handle). The original bug
 *     was a frontend payload that stripped this field — we drive it through
 *     the UI to catch frontend regressions.
 *  2. UI guard: clicking Save with an IMAGE header but no uploaded sample
 *     surfaces a toast and blocks the save instead of silently creating a
 *     dead-end template.
 */

test.describe('Template media header — issue #355', () => {
  let api: ApiHelper
  let accountName: string

  test.beforeEach(async ({ request }) => {
    api = new ApiHelper(request)
    await api.login(SUPER_ADMIN.email, SUPER_ADMIN.password)

    const accounts = await api.getWhatsAppAccounts()
    accountName = accounts[0]?.name
    if (!accountName) {
      const acc = await api.createWhatsAppAccount({
        name: scope.name('acct').toLowerCase().replace(/\s/g, '-'),
        phone_id: `phone-tpl-media-${Date.now()}`,
        business_id: `biz-tpl-media-${Date.now()}`,
        access_token: 'test-token-e2e',
      })
      accountName = acc.name
    }
  })

  test('UI save preserves IMAGE header_content (regression for #355)', async ({ page }) => {
    // Seed a template with an IMAGE header and a fake Meta handle.
    const handle = `e2e-handle-${Date.now()}`
    const tpl = await api.createTemplate({
      name: `tpl_img_${Date.now()}`,
      body_content: 'Hello image header',
      whatsapp_account: accountName,
      header_type: 'IMAGE',
      header_content: handle,
      status: 'DRAFT',
    })

    // Drive the detail page Save button. With the pre-fix code, the payload
    // would send header_content: '' because header_type !== 'TEXT', wiping
    // the handle on disk. After the fix, the loaded handle round-trips.
    await loginAsAdmin(page)
    await page.goto(`/templates/${tpl.id}`)
    await page.waitForLoadState('networkidle')

    // Expand the Details card and tweak display_name to dirty the form.
    await page.locator('text=Details').first().click()
    await page.waitForTimeout(300)

    const displayInput = page.locator('input#display-name, input[id*="display"]').first()
    if (await displayInput.isVisible({ timeout: 2000 }).catch(() => false)) {
      await displayInput.fill(`Edited ${Date.now()}`)
    } else {
      // Fallback: edit the first writable input to dirty the form.
      const firstInput = page.locator('input:visible').first()
      const original = await firstInput.inputValue()
      await firstInput.fill(`${original}_e`)
    }
    await page.waitForTimeout(200)

    const saveBtn = page.getByRole('button', { name: /^Save$/i })
    await saveBtn.click({ force: true })

    // Wait for the PUT to complete; success toast or page settled.
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(500)

    // Re-fetch via API and assert the handle survived.
    const resp = await api.get(`/api/templates/${tpl.id}`)
    expect(resp.ok(), `GET failed: ${resp.status()} ${await resp.text()}`).toBe(true)
    const reloaded = (await resp.json()).data
    expect(reloaded.header_type).toBe('IMAGE')
    expect(reloaded.header_content).toBe(handle)
  })

  test('Save is blocked with a toast when IMAGE header has no uploaded sample', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/templates/new')
    await page.waitForLoadState('networkidle')

    // Fill the minimum required fields: name, account, body. Leave the IMAGE
    // header sample unset to trigger the new save guard.
    const name = `tpl_no_media_${Date.now()}`
    const nameInput = page.locator('input').first()
    await nameInput.fill(name)

    // Select the WhatsApp account combobox (first combobox on the page).
    const accountCombo = page.locator('button[role="combobox"]').first()
    await accountCombo.click()
    await page.getByRole('option', { name: accountName }).first().click()

    // Body content.
    await page.locator('textarea').first().fill('Hello no-media')

    // Switch header type to IMAGE.
    const headerCombo = page.locator('button[role="combobox"]').filter({ hasText: /Header|TEXT|None|Type/i }).first()
    await headerCombo.click()
    await page.getByRole('option', { name: /^Image$/i }).first().click()
    await page.waitForTimeout(200)

    // Click Save — expect the new guard toast and no navigation away from /new.
    await page.getByRole('button', { name: /Create|Save/i }).first().click({ force: true })

    await expect(
      page.locator('[data-sonner-toast], [role="status"], [role="alert"]')
        .filter({ hasText: /Upload a sample/i })
    ).toBeVisible({ timeout: 5000 })

    // We must still be on /templates/new — no template was created.
    expect(page.url()).toContain('/templates/new')
  })
})
