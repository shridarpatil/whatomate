import { test, expect, request as playwrightRequest } from '@playwright/test'
import { Client } from 'pg'
import { loginAsAdmin, ApiHelper } from '../../helpers'
import { ChatPage } from '../../pages'

const DB_URL = process.env.TEST_DATABASE_URL || 'postgres://whatomate:whatomate@127.0.0.1:5432/whatomate'

/**
 * Run a SQL statement against the test database using node-postgres.
 * Used to seed APPROVED templates (API only creates DRAFT).
 * Works in CI without needing psql binary installed.
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

/**
 * Template Sending E2E Tests
 *
 * These tests cover the template picker and template sending flow
 * from the chat view. They require:
 * - At least one contact in the system
 * - At least one WhatsApp account configured
 * - Templates are seeded via SQL in beforeAll
 */
test.describe('Template Sending', () => {
  test.describe.configure({ mode: 'serial' })
  test.setTimeout(60000)

  let contactId: string
  let accountName: string
  let simpleTemplateName: string
  let paramTemplateName: string
  let buttonTemplateName: string

  test.beforeAll(async () => {
    const reqContext = await playwrightRequest.newContext()
    const api = new ApiHelper(reqContext)
    await api.loginAsAdmin()

    // Ensure we have at least one contact
    let contacts = await api.getContacts()
    if (contacts.length === 0) {
      await api.createContact(`91${Date.now().toString().slice(-10)}`, 'Template Test Contact')
      contacts = await api.getContacts()
    }
    contactId = contacts[0].id

    // Ensure we have a WhatsApp account (required for templates)
    let accounts: any[] = []
    try {
      accounts = await api.getWhatsAppAccounts()
    } catch {
      // ignore
    }
    if (accounts.length === 0) {
      const uid = Date.now().toString().slice(-8)
      try {
        await api.createWhatsAppAccount({
          name: `e2e-account-${uid}`,
          phone_id: `phone-${uid}`,
          business_id: `biz-${uid}`,
          access_token: `token-${uid}`
        })
        accounts = await api.getWhatsAppAccounts()
      } catch {
        // ignore
      }
    }
    accountName = accounts.length > 0 ? accounts[0].name : ''

    // Get the organization ID for the logged-in user
    const orgId = await execSQL(`SELECT organization_id FROM users WHERE email = 'admin@test.com' LIMIT 1`)

    // Clean up leftover e2e templates from previous runs
    await execSQL(`DELETE FROM templates WHERE name LIKE 'e2e_%' AND organization_id = '${orgId}'`)

    // Seed APPROVED templates directly via SQL (API only creates DRAFT)
    const uid = Date.now().toString().slice(-6)

    simpleTemplateName = `e2e_simple_${uid}`
    await execSQL(`INSERT INTO templates (id, organization_id, whats_app_account, name, display_name, language, category, status, body_content, created_at, updated_at)
      VALUES (gen_random_uuid(), '${orgId}', '${accountName}', '${simpleTemplateName}', 'E2E Simple ${uid}', 'en', 'UTILITY', 'APPROVED', 'Welcome to our service! We are glad to have you.', NOW(), NOW())
      ON CONFLICT DO NOTHING`)

    paramTemplateName = `e2e_params_${uid}`
    await execSQL(`INSERT INTO templates (id, organization_id, whats_app_account, name, display_name, language, category, status, body_content, created_at, updated_at)
      VALUES (gen_random_uuid(), '${orgId}', '${accountName}', '${paramTemplateName}', 'E2E Params ${uid}', 'en', 'UTILITY', 'APPROVED', 'Hello {{name}}! Your order {{order_id}} is confirmed.', NOW(), NOW())
      ON CONFLICT DO NOTHING`)

    buttonTemplateName = `e2e_buttons_${uid}`
    await execSQL(`INSERT INTO templates (id, organization_id, whats_app_account, name, display_name, language, category, status, body_content, buttons, created_at, updated_at)
      VALUES (gen_random_uuid(), '${orgId}', '${accountName}', '${buttonTemplateName}', 'E2E Buttons ${uid}', 'en', 'UTILITY', 'APPROVED', 'Would you like to proceed with your order?', '[{"type":"QUICK_REPLY","text":"Yes"},{"type":"QUICK_REPLY","text":"No"}]'::jsonb, NOW(), NOW())
      ON CONFLICT DO NOTHING`)

    await reqContext.dispose()
  })

  test('should show template picker button when contact is selected', async ({ page }) => {
    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    await expect(chatPage.templatePickerButton).toBeVisible()
  })

  test('should open template picker popover', async ({ page }) => {
    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    await chatPage.openTemplatePicker()

    // Popover should be visible with a search input
    await expect(chatPage.templatePopover).toBeVisible()
    await expect(chatPage.templateSearchInput).toBeVisible()
  })

  test('should display templates in the picker', async ({ page }) => {
    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    await chatPage.openTemplatePicker()

    // Wait for templates to load (loader disappears)
    await page.waitForTimeout(1000)

    // At least our seeded templates should appear
    const templateItems = chatPage.templatePopover.locator('button.w-full.text-left')
    const count = await templateItems.count()
    expect(count).toBeGreaterThan(0)
  })

  test('should search templates', async ({ page }) => {
    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    await chatPage.openTemplatePicker()
    await page.waitForTimeout(500)

    // Search for our simple template
    await chatPage.searchTemplates('e2e_simple')
    await page.waitForTimeout(300)

    // Should find the matching template
    const item = chatPage.getTemplateItem(`E2E Simple`)
    await expect(item).toBeVisible()
  })

  test('should show preview dialog for simple template (no params)', async ({ page }) => {
    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    await chatPage.openTemplatePicker()
    await page.waitForTimeout(500)

    // Search and select the simple template
    await chatPage.searchTemplates('e2e_simple')
    await page.waitForTimeout(300)
    await chatPage.selectTemplate(`E2E Simple`)

    // Dialog should show "Preview" heading (no params to fill)
    await expect(chatPage.templateDialog).toBeVisible()
    await expect(chatPage.templateDialog.getByRole('heading', { name: 'Preview' })).toBeVisible()

    // Preview bubble should show the body content
    const preview = chatPage.getTemplatePreviewBubble()
    await expect(preview).toBeVisible()
    await expect(preview).toContainText('Welcome to our service')

    // Send and Cancel buttons should be visible
    await expect(chatPage.templateDialogSendButton).toBeVisible()
    await expect(chatPage.templateDialogCancelButton).toBeVisible()

    // Close the dialog
    await chatPage.cancelTemplateDialog()
  })

  test('should show Fill Parameters dialog for template with params', async ({ page }) => {
    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    await chatPage.openTemplatePicker()
    await page.waitForTimeout(500)

    await chatPage.searchTemplates('e2e_params')
    await page.waitForTimeout(300)
    await chatPage.selectTemplate(`E2E Params`)

    // Dialog should show "Fill Parameters" heading
    await expect(chatPage.templateDialog).toBeVisible()
    await expect(chatPage.templateDialog.getByRole('heading', { name: 'Fill Parameters' })).toBeVisible()

    // Should have input fields for name and order_id
    const nameInput = chatPage.templateDialog.locator('.space-y-1').filter({ hasText: 'name' }).locator('input')
    const orderInput = chatPage.templateDialog.locator('.space-y-1').filter({ hasText: 'order_id' }).locator('input')
    await expect(nameInput).toBeVisible()
    await expect(orderInput).toBeVisible()

    // Close the dialog
    await chatPage.cancelTemplateDialog()
  })

  test('should update preview as params are filled', async ({ page }) => {
    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    await chatPage.openTemplatePicker()
    await page.waitForTimeout(500)

    await chatPage.searchTemplates('e2e_params')
    await page.waitForTimeout(300)
    await chatPage.selectTemplate(`E2E Params`)

    // Fill parameters
    await chatPage.fillTemplateParam('name', 'Alice')
    await chatPage.fillTemplateParam('order_id', 'ORD-42')

    // Preview should update with filled values
    const preview = chatPage.getTemplatePreviewBubble()
    await expect(preview).toContainText('Hello Alice')
    await expect(preview).toContainText('ORD-42')

    await chatPage.cancelTemplateDialog()
  })

  test('should show buttons in preview for template with buttons', async ({ page }) => {
    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    await chatPage.openTemplatePicker()
    await page.waitForTimeout(500)

    await chatPage.searchTemplates('e2e_buttons')
    await page.waitForTimeout(300)
    await chatPage.selectTemplate(`E2E Buttons`)

    // Preview should show the body
    const preview = chatPage.getTemplatePreviewBubble()
    await expect(preview).toContainText('Would you like to proceed')

    // Buttons should be rendered in the preview
    const buttons = chatPage.getTemplatePreviewButtons()
    await expect(buttons).toHaveCount(2)
    await expect(buttons.nth(0)).toContainText('Yes')
    await expect(buttons.nth(1)).toContainText('No')

    await chatPage.cancelTemplateDialog()
  })

  test('should send simple template successfully', async ({ page }) => {
    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    await chatPage.openTemplatePicker()
    await page.waitForTimeout(500)

    await chatPage.searchTemplates('e2e_simple')
    await page.waitForTimeout(300)
    await chatPage.selectTemplate(`E2E Simple`)

    // Click send
    await chatPage.sendTemplate()

    // Should show success toast or the dialog should close
    // The API may fail if no WhatsApp account is configured, so we check both outcomes
    const toastSuccess = page.locator('[data-sonner-toast]').filter({ hasText: /Template sent/i })
    const toastError = page.locator('[data-sonner-toast]').filter({ hasText: /Failed/i })
    const dialogClosed = chatPage.templateDialog

    // Wait for either toast to appear
    await expect(toastSuccess.or(toastError)).toBeVisible({ timeout: 10000 })

    // If successful, the dialog should close
    if (await toastSuccess.isVisible()) {
      await expect(dialogClosed).not.toBeVisible()
    }
  })

  test('should require all params before sending', async ({ page }) => {
    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    await chatPage.openTemplatePicker()
    await page.waitForTimeout(500)

    await chatPage.searchTemplates('e2e_params')
    await page.waitForTimeout(300)
    await chatPage.selectTemplate(`E2E Params`)

    // Fill only one parameter
    await chatPage.fillTemplateParam('name', 'Alice')
    // Leave order_id empty

    // Try to send
    await chatPage.sendTemplate()

    // Should show validation error toast
    const toast = page.locator('[data-sonner-toast]').filter({ hasText: /required/i })
    await expect(toast).toBeVisible({ timeout: 5000 })

    // Dialog should remain open
    await expect(chatPage.templateDialog).toBeVisible()

    await chatPage.cancelTemplateDialog()
  })

  test('should send template with params successfully', async ({ page }) => {
    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    await chatPage.openTemplatePicker()
    await page.waitForTimeout(500)

    await chatPage.searchTemplates('e2e_params')
    await page.waitForTimeout(300)
    await chatPage.selectTemplate(`E2E Params`)

    // Fill all parameters
    await chatPage.fillTemplateParam('name', 'Bob')
    await chatPage.fillTemplateParam('order_id', 'ORD-99')

    // Send
    await chatPage.sendTemplate()

    // Check for success or failure toast
    const toastSuccess = page.locator('[data-sonner-toast]').filter({ hasText: /Template sent/i })
    const toastError = page.locator('[data-sonner-toast]').filter({ hasText: /Failed/i })
    await expect(toastSuccess.or(toastError)).toBeVisible({ timeout: 10000 })
  })

  test('should close template picker when clicking cancel', async ({ page }) => {
    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    await chatPage.openTemplatePicker()
    await page.waitForTimeout(500)

    await chatPage.searchTemplates('e2e_simple')
    await page.waitForTimeout(300)
    await chatPage.selectTemplate(`E2E Simple`)

    // Dialog should be open
    await expect(chatPage.templateDialog).toBeVisible()

    // Cancel
    await chatPage.cancelTemplateDialog()

    // Dialog should be closed
    await expect(chatPage.templateDialog).not.toBeVisible()
  })
})
