import { test, expect, request as playwrightRequest } from '@playwright/test'
import { Client } from 'pg'
import { loginAsAdmin, ApiHelper } from '../../helpers'
import { ChatPage } from '../../pages'
import { createTestScope } from '../../framework'

const scope = createTestScope('chat-tpl-header-param')

const DB_URL =
  process.env.TEST_DATABASE_URL ||
  'postgres://whatomate:whatomate@127.0.0.1:5432/whatomate'

/**
 * Templates with a TEXT header containing a {{var}} need a dedicated input
 * slot in the chat composer's Fill Parameters dialog, and the value must be
 * sent to the backend under `header_params` (separate from `template_params`)
 * so positional {{1}} in the header doesn't collide with positional {{1}} in
 * the body.
 *
 * Pattern mirrors template-sending.spec.ts — seed APPROVED templates via SQL
 * because the API only ever creates DRAFT.
 */

async function execSQL(sql: string): Promise<string> {
  const client = new Client({ connectionString: DB_URL })
  await client.connect()
  try {
    const result = await client.query(sql)
    return result.rows.length > 0 ? String(Object.values(result.rows[0])[0]) : ''
  } finally {
    await client.end()
  }
}

test.describe('Chat composer — TEXT header parameter', () => {
  test.describe.configure({ mode: 'serial' })
  test.setTimeout(60000)

  let contactId: string
  let accountName: string
  let positionalTplName: string
  let namedTplName: string

  test.beforeAll(async () => {
    const reqContext = await playwrightRequest.newContext()
    const api = new ApiHelper(reqContext)
    await api.loginAsAdmin()

    const phone = scope.phone()
    await api.createContact(phone, scope.name('contact'))
    const contacts = await api.getContacts()
    const contact = contacts.find((c: any) => c.phone_number === phone) || contacts[0]
    contactId = contact.id

    let accounts: any[] = []
    try {
      accounts = await api.getWhatsAppAccounts()
    } catch {
      // ignore
    }
    if (accounts.length === 0) {
      const uid = Date.now().toString().slice(-8)
      await api.createWhatsAppAccount({
        name: `e2e-hdr-account-${uid}`,
        phone_id: `phone-${uid}`,
        business_id: `biz-${uid}`,
        access_token: `token-${uid}`,
      })
      accounts = await api.getWhatsAppAccounts()
    }
    accountName = accounts[0].name

    const orgId = await execSQL(
      `SELECT organization_id FROM users WHERE email = 'admin@test.com' LIMIT 1`,
    )
    await execSQL(`DELETE FROM templates WHERE name LIKE 'e2e_hdr_%' AND organization_id = '${orgId}'`)

    const uid = Date.now().toString().slice(-6)

    // Positional template — header {{1}} AND body {{1}}, the case that was
    // collapsing to a single input before this work.
    positionalTplName = `e2e_hdr_pos_${uid}`
    await execSQL(`INSERT INTO templates (id, organization_id, whats_app_account, name, display_name, language, category, status, header_type, header_content, body_content, created_at, updated_at)
      VALUES (gen_random_uuid(), '${orgId}', '${accountName}', '${positionalTplName}', 'E2E Header Pos ${uid}', 'en', 'UTILITY', 'APPROVED', 'TEXT', 'Order {{1}} update', 'Hi {{1}}, code {{2}}', NOW(), NOW())`)

    // Named template — separate var names; covers the "header_params is
    // optional, falls back to template_params" path.
    namedTplName = `e2e_hdr_named_${uid}`
    await execSQL(`INSERT INTO templates (id, organization_id, whats_app_account, name, display_name, language, category, status, header_type, header_content, body_content, created_at, updated_at)
      VALUES (gen_random_uuid(), '${orgId}', '${accountName}', '${namedTplName}', 'E2E Header Named ${uid}', 'en', 'UTILITY', 'APPROVED', 'TEXT', 'Our {{season}} sale', 'Hi {{name}}', NOW(), NOW())`)

    await execSQL(`UPDATE contacts SET whats_app_account = '${accountName}' WHERE id = '${contactId}'`)
    await reqContext.dispose()
  })

  test('positional template renders distinct header + body inputs', async ({ page }) => {
    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    await chatPage.openTemplatePicker()
    await chatPage.waitForTemplatesLoaded()
    await chatPage.searchTemplates('e2e_hdr_pos')
    await chatPage.selectTemplate('E2E Header Pos')

    await expect(chatPage.templateDialog).toBeVisible()

    // Header slot is labeled with a "Header" badge — the dedicated input lives
    // in its own .space-y-1 block.
    const headerBadge = chatPage.templateDialog.getByText('Header', { exact: true })
    await expect(headerBadge).toBeVisible()

    // Two inputs must exist for {{1}}: one above the badge slot, one below.
    // (Pre-fix, dedupe collapsed them to a single input.)
    const allInputs = chatPage.templateDialog.locator('input[type="text"], input:not([type])')
    expect(await allInputs.count()).toBeGreaterThanOrEqual(2)

    await chatPage.cancelTemplateDialog()
  })

  test('sending a positional template emits header_params separately from template_params', async ({ page }) => {
    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    await chatPage.openTemplatePicker()
    await chatPage.waitForTemplatesLoaded()
    await chatPage.searchTemplates('e2e_hdr_pos')
    await chatPage.selectTemplate('E2E Header Pos')

    await expect(chatPage.templateDialog).toBeVisible()

    // The header slot is the .space-y-1 block containing the "Header" badge.
    const headerSlot = chatPage.templateDialog
      .locator('.space-y-1')
      .filter({ has: page.getByText('Header', { exact: true }) })
    await headerSlot.locator('input').fill('HEADER-VAL')

    // Body inputs follow templateParamNames order — "1" then "2".
    const bodyInputs = chatPage.templateDialog
      .locator('.space-y-1')
      .filter({ hasNotText: 'Header' })
      .locator('input')
    await bodyInputs.nth(0).fill('BODY-VAL-1')
    await bodyInputs.nth(1).fill('BODY-VAL-2')

    // Capture the outgoing POST /api/messages/template payload to verify the
    // split. The send is async on the backend so the request goes out as soon
    // as we click Send.
    const sendRequest = page.waitForRequest((r) =>
      r.method() === 'POST' && r.url().includes('/api/messages/template'),
    )
    await chatPage.sendTemplate()
    const req = await sendRequest

    const body = req.postDataJSON() as {
      template_params?: Record<string, string>
      header_params?: Record<string, string>
    }
    expect(body.header_params).toEqual({ '1': 'HEADER-VAL' })
    expect(body.template_params).toEqual({ '1': 'BODY-VAL-1', '2': 'BODY-VAL-2' })
  })

  test('named template forwards header value (no collision case)', async ({ page }) => {
    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    await chatPage.openTemplatePicker()
    await chatPage.waitForTemplatesLoaded()
    await chatPage.searchTemplates('e2e_hdr_named')
    await chatPage.selectTemplate('E2E Header Named')

    await expect(chatPage.templateDialog).toBeVisible()

    const headerSlot = chatPage.templateDialog
      .locator('.space-y-1')
      .filter({ has: page.getByText('Header', { exact: true }) })
    await headerSlot.locator('input').fill('Summer')

    const nameInput = chatPage.templateDialog
      .locator('.space-y-1')
      .filter({ hasText: 'name' })
      .filter({ hasNotText: 'Header' })
      .locator('input')
    await nameInput.fill('Alice')

    const sendRequest = page.waitForRequest((r) =>
      r.method() === 'POST' && r.url().includes('/api/messages/template'),
    )
    await chatPage.sendTemplate()
    const req = await sendRequest

    const body = req.postDataJSON() as {
      template_params?: Record<string, string>
      header_params?: Record<string, string>
    }
    expect(body.header_params).toEqual({ season: 'Summer' })
    expect(body.template_params).toEqual({ name: 'Alice' })
  })
})
