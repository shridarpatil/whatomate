import { test, expect, request as playwrightRequest } from '@playwright/test'
import { Client } from 'pg'
import { loginAsAdmin, ApiHelper } from '../../helpers'
import { createTestScope } from '../../framework'

const scope = createTestScope('campaign-header-param')

const DB_URL =
  process.env.TEST_DATABASE_URL ||
  'postgres://whatomate:whatomate@127.0.0.1:5432/whatomate'

/**
 * Campaigns with a TEXT header parameter must:
 *   1. Show the "header" column in the manual-entry format hint and accept
 *      a value for it without eating the recipient name slot.
 *   2. Persist the value to BulkMessageRecipient.HeaderParams (not just
 *      TemplateParams) so the worker can split header_params from body
 *      params when calling Meta.
 *   3. The "Download sample CSV" button generates the right column layout.
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

test.describe('Campaign recipients — TEXT header parameter', () => {
  test.describe.configure({ mode: 'serial' })
  test.setTimeout(60000)

  let accountName: string
  let templateId: string
  let templateName: string

  test.beforeAll(async () => {
    const reqContext = await playwrightRequest.newContext()
    const api = new ApiHelper(reqContext)
    await api.loginAsAdmin()

    let accounts: any[] = []
    try {
      accounts = await api.getWhatsAppAccounts()
    } catch {
      // ignore
    }
    if (accounts.length === 0) {
      const uid = Date.now().toString().slice(-8)
      await api.createWhatsAppAccount({
        name: `e2e-camp-hdr-${uid}`,
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
    // Clean up leftover state from previous runs. Order matters: recipients
    // FK to campaigns FK to templates.
    await execSQL(`DELETE FROM bulk_message_recipients WHERE campaign_id IN (SELECT id FROM bulk_message_campaigns WHERE template_id IN (SELECT id FROM templates WHERE name LIKE 'e2e_camp_hdr_%' AND organization_id = '${orgId}'))`)
    await execSQL(`DELETE FROM bulk_message_campaigns WHERE template_id IN (SELECT id FROM templates WHERE name LIKE 'e2e_camp_hdr_%' AND organization_id = '${orgId}')`)
    await execSQL(`DELETE FROM templates WHERE name LIKE 'e2e_camp_hdr_%' AND organization_id = '${orgId}'`)

    const uid = Date.now().toString().slice(-6)
    templateName = `e2e_camp_hdr_${uid}`
    // Positional header + positional body — the case where the flat
    // TemplateParams map would collide and that this fix actually addresses.
    templateId = await execSQL(`INSERT INTO templates (id, organization_id, whats_app_account, name, display_name, language, category, status, header_type, header_content, body_content, created_at, updated_at)
      VALUES (gen_random_uuid(), '${orgId}', '${accountName}', '${templateName}', 'E2E Campaign Header ${uid}', 'en', 'UTILITY', 'APPROVED', 'TEXT', 'Order {{1}} update', 'Hi {{1}}, code {{2}}', NOW(), NOW())
      RETURNING id`)

    await reqContext.dispose()
  })

  test('manual entry parses header column into header_params and body values into template_params', async ({ page, request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    // Seed a draft campaign for this template via the API.
    const createResp = await api.post('/api/campaigns', {
      name: scope.name('campaign'),
      whatsapp_account: accountName,
      template_id: templateId,
    })
    expect(createResp.ok(), `campaign POST failed: ${createResp.status()} ${await createResp.text()}`).toBe(true)
    const campaign = (await createResp.json()).data
    const campaignId = campaign.id

    // Drive the recipient dialog through the UI.
    await loginAsAdmin(page)
    await page.goto(`/campaigns/${campaignId}`)
    await page.waitForLoadState('networkidle')

    await page.getByRole('button', { name: /Add Recipients/i }).first().click()
    const dialog = page.locator('[role="dialog"]')
    await expect(dialog).toBeVisible()

    // Format hint shows the header column explicitly so users don't conflate
    // it with the body params.
    await expect(dialog.locator('code')).toContainText('header')

    // Manual entry: phone, name, header, body_1, body_2 — header is the
    // third column. The Textarea component doesn't forward an id; the
    // dialog only has one textarea when the Manual tab is active.
    const textarea = dialog.locator('textarea')
    await textarea.fill('+15551230001, Alice, HEADER-VAL, BODY-1, BODY-2')

    // Validation passes — no error chips listed.
    await expect(dialog.getByText(/lines have errors/i)).toBeHidden()

    const addBtn = dialog.getByRole('button', { name: /Add Recipients/i }).last()
    await expect(addBtn).toBeEnabled()
    // Wait for the POST to complete before querying the DB, otherwise we
    // race the async insert.
    const addResp = page.waitForResponse((r) =>
      r.url().includes(`/api/campaigns/${campaignId}/recipients/import`) && r.request().method() === 'POST',
    )
    await addBtn.click()
    await addResp
    await expect(dialog).toBeHidden()

    // Side-channel: load the recipient row and assert HeaderParams was
    // persisted as its own JSONB field — separate from TemplateParams.
    const stored = await execSQL(
      `SELECT json_build_object('hp', header_params, 'tp', template_params)::text FROM bulk_message_recipients WHERE campaign_id = '${campaignId}' LIMIT 1`,
    )
    const parsed = JSON.parse(stored) as {
      hp: Record<string, string>
      tp: Record<string, string>
    }
    expect(parsed.hp).toEqual({ '1': 'HEADER-VAL' })
    expect(parsed.tp).toEqual({ '1': 'BODY-1', '2': 'BODY-2' })

    // Cleanup so re-runs stay deterministic.
    await execSQL(`DELETE FROM bulk_message_recipients WHERE campaign_id = '${campaignId}'`)
    await execSQL(`DELETE FROM bulk_message_campaigns WHERE id = '${campaignId}'`)
  })

  test('manual entry surfaces a column-count error when the header value is omitted', async ({ page, request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    const createResp = await api.post('/api/campaigns', {
      name: scope.name('campaign-bad'),
      whatsapp_account: accountName,
      template_id: templateId,
    })
    expect(createResp.ok()).toBe(true)
    const campaignId = (await createResp.json()).data.id

    await loginAsAdmin(page)
    await page.goto(`/campaigns/${campaignId}`)
    await page.waitForLoadState('networkidle')

    await page.getByRole('button', { name: /Add Recipients/i }).first().click()
    const dialog = page.locator('[role="dialog"]')
    await expect(dialog).toBeVisible()

    // Missing the header column — only phone, name, and two body values.
    // The tightened validator must catch the column-count mismatch instead
    // of silently mapping "BODY-1" into the header slot.
    const textarea = dialog.locator('textarea')
    await textarea.fill('+15551230002, Alice, BODY-1, BODY-2')

    await expect(dialog.getByText(/Need 3 parameter values/i)).toBeVisible()

    const addBtn = dialog.getByRole('button', { name: /Add Recipients/i }).last()
    await expect(addBtn).toBeDisabled()

    // Cleanup.
    await execSQL(`DELETE FROM bulk_message_campaigns WHERE id = '${campaignId}'`)
  })

  test('Download sample CSV produces a header row that includes the "header" column', async ({ page, request }) => {
    const api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    const createResp = await api.post('/api/campaigns', {
      name: scope.name('campaign-csv'),
      whatsapp_account: accountName,
      template_id: templateId,
    })
    expect(createResp.ok()).toBe(true)
    const campaignId = (await createResp.json()).data.id

    await loginAsAdmin(page)
    await page.goto(`/campaigns/${campaignId}`)
    await page.waitForLoadState('networkidle')

    await page.getByRole('button', { name: /Add Recipients/i }).first().click()
    const dialog = page.locator('[role="dialog"]')
    await expect(dialog).toBeVisible()

    await dialog.getByRole('tab', { name: /CSV/i }).click()

    const downloadPromise = page.waitForEvent('download')
    await dialog.getByRole('button', { name: /Download sample CSV/i }).click()
    const download = await downloadPromise

    const path = await download.path()
    expect(path).toBeTruthy()
    const fs = await import('fs/promises')
    const csv = await fs.readFile(path!, 'utf-8')
    const headerRow = csv.split('\n')[0]
    expect(headerRow.split(',')).toEqual(['phone_number', 'name', 'header', '1', '2'])

    // Cleanup.
    await execSQL(`DELETE FROM bulk_message_campaigns WHERE id = '${campaignId}'`)
  })
})
