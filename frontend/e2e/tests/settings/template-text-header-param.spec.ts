import { test, expect } from '@playwright/test'
import { ApiHelper, loginAsAdmin } from '../../helpers'
import { createTestScope, SUPER_ADMIN } from '../../framework'

const scope = createTestScope('tpl-text-header-param')

/**
 * Meta restricts TEXT headers to one variable. The template editor must
 * surface this constraint inline and block save before the backend has to
 * 400 the request — and before publishing a template Meta will reject.
 *
 * See:
 *   - internal/templateutil.ValidateHeaderParamCount
 *   - frontend/src/views/settings/TemplateDetailView.vue (hasTooManyHeaderVariables)
 */

test.describe('Template editor — TEXT header parameter', () => {
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
        phone_id: `phone-tpl-text-${Date.now()}`,
        business_id: `biz-tpl-text-${Date.now()}`,
        access_token: 'test-token-e2e',
      })
      accountName = acc.name
    }
  })

  test('editor surfaces an inline error when the TEXT header has 2+ variables', async ({ page, request }) => {
    // Seed a clean DRAFT template with a single-variable TEXT header.
    const tpl = await api.createTemplate({
      name: scope.name('tpl').toLowerCase().replace(/\s/g, '_'),
      body_content: 'Hi {{1}}',
      whatsapp_account: accountName,
      header_type: 'TEXT',
      header_content: 'Hi {{1}}',
      status: 'DRAFT',
    })

    await loginAsAdmin(page)
    await page.goto(`/templates/${tpl.id}`)
    await page.waitForLoadState('networkidle')

    // Edit the header to two variables. The inline destructive hint should
    // appear right below the input.
    const headerInput = page.locator('#header-content')
    await headerInput.fill('Order {{1}} for {{2}}')

    await expect(
      page.getByText(/at most one variable in a TEXT header/i),
    ).toBeVisible()

    // Backing this up with the API negative path — the handler also enforces
    // the same constraint, so a direct PUT must 400 with the same wording.
    const resp = await api.put(`/api/templates/${tpl.id}`, {
      header_type: 'TEXT',
      header_content: 'Order {{1}} for {{2}}',
      body_content: 'Hi {{1}}',
    })
    expect(resp.status()).toBe(400)
    expect(await resp.text()).toMatch(/at most one variable/i)
  })

  test('editor accepts a single-variable header and persists it', async ({ page, request }) => {
    const tpl = await api.createTemplate({
      name: scope.name('tpl-single').toLowerCase().replace(/\s/g, '_'),
      body_content: 'Hi {{1}}',
      whatsapp_account: accountName,
      header_type: 'TEXT',
      header_content: 'Welcome',
      status: 'DRAFT',
    })

    await loginAsAdmin(page)
    await page.goto(`/templates/${tpl.id}`)
    await page.waitForLoadState('networkidle')

    await page.locator('#header-content').fill('Our {{1}} sale')

    // Destructive hint must NOT appear with one variable.
    await expect(
      page.getByText(/at most one variable in a TEXT header/i),
    ).toBeHidden()

    // API side-channel: PUT with one variable succeeds.
    const resp = await api.put(`/api/templates/${tpl.id}`, {
      header_type: 'TEXT',
      header_content: 'Our {{1}} sale',
      body_content: 'Hi {{1}}',
    })
    expect(resp.ok(), `PUT failed: ${resp.status()} ${await resp.text()}`).toBe(true)
  })
})
