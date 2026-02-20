import { test, expect, request as playwrightRequest } from '@playwright/test'
import { Client } from 'pg'
import { loginAsAdmin, ApiHelper } from '../../helpers'
import { ChatPage } from '../../pages'

const DB_URL = process.env.TEST_DATABASE_URL || 'postgres://whatomate:whatomate@127.0.0.1:5432/whatomate'

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
 * 24-Hour Service Window E2E Tests
 *
 * These tests verify the service window expired banner and related UI
 * in the chat view. The WhatsApp API only allows freeform messages
 * within 24 hours of the customer's last inbound message.
 */
test.describe('24-Hour Service Window', () => {
  test.describe.configure({ mode: 'serial' })
  test.setTimeout(60000)

  let contactId: string

  test.beforeAll(async () => {
    const reqContext = await playwrightRequest.newContext()
    const api = new ApiHelper(reqContext)
    await api.loginAsAdmin()

    // Create a dedicated contact for service window tests
    const phone = `91${Date.now().toString().slice(-10)}`
    const contact = await api.createContact(phone, 'Service Window Test')
    contactId = contact.id

    await reqContext.dispose()
  })

  test('should show expired-window banner when last_inbound_at is older than 24 hours', async ({ page }) => {
    // Set last_inbound_at to 25 hours ago
    await execSQL(`UPDATE contacts SET last_inbound_at = NOW() - INTERVAL '25 hours' WHERE id = '${contactId}'`)

    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    // The red service window expired banner should be visible
    const banner = page.locator('text=24-hour messaging window has expired')
    await expect(banner).toBeVisible({ timeout: 5000 })

    // The "Send Template" button should be in the banner
    const sendTemplateBtn = page.getByRole('button', { name: /Send Template/i })
    await expect(sendTemplateBtn).toBeVisible()
  })

  test('should NOT show expired-window banner when last_inbound_at is within 24 hours', async ({ page }) => {
    // Set last_inbound_at to 1 hour ago
    await execSQL(`UPDATE contacts SET last_inbound_at = NOW() - INTERVAL '1 hour' WHERE id = '${contactId}'`)

    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    // Wait for the page to load fully
    await page.waitForTimeout(1000)

    // The banner should NOT be visible
    const banner = page.locator('text=24-hour messaging window has expired')
    await expect(banner).not.toBeVisible()
  })

  test('should show expired-window banner when last_inbound_at is NULL', async ({ page }) => {
    // Set last_inbound_at to NULL (no inbound messages ever)
    await execSQL(`UPDATE contacts SET last_inbound_at = NULL WHERE id = '${contactId}'`)

    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    // The banner should be visible (NULL = window expired, safe default)
    const banner = page.locator('text=24-hour messaging window has expired')
    await expect(banner).toBeVisible({ timeout: 5000 })
  })

  test('should open template picker when clicking Send Template in banner', async ({ page }) => {
    // Ensure window is expired
    await execSQL(`UPDATE contacts SET last_inbound_at = NOW() - INTERVAL '25 hours' WHERE id = '${contactId}'`)

    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    // Click the "Send Template" button in the banner
    const sendTemplateBtn = page.getByRole('button', { name: /Send Template/i })
    await expect(sendTemplateBtn).toBeVisible({ timeout: 5000 })
    await sendTemplateBtn.click()

    // The template picker popover should open (search input becomes visible)
    await expect(chatPage.templateSearchInput).toBeVisible({ timeout: 5000 })
  })

  test('should show error_message on failed outgoing messages', async ({ page }) => {
    const orgId = await execSQL(`SELECT organization_id FROM contacts WHERE id = '${contactId}'`)
    const errorMessage = 'Re-engagement message: This message was not delivered to maintain healthy ecosystem engagement.'

    // Insert a failed outgoing message with an error_message
    await execSQL(`INSERT INTO messages (id, organization_id, whats_app_account, contact_id, direction, message_type, content, status, error_message, created_at, updated_at)
      VALUES (gen_random_uuid(), '${orgId}', 'test-account', '${contactId}', 'outgoing', 'text', 'Hello after window expired', 'failed', '${errorMessage}', NOW(), NOW())`)

    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    // Wait for messages to load
    await page.waitForTimeout(1000)

    // The error message should be displayed on the failed message bubble
    const errorText = page.locator('text=This message was not delivered')
    await expect(errorText).toBeVisible({ timeout: 5000 })
  })

  test('should display full error text on failed messages', async ({ page }) => {
    // Re-use the contact which already has a failed message from the previous test
    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    await page.waitForTimeout(1000)

    // The full error message (Title + Details) should be visible on the failed message
    const errorText = page.locator('text=This message was not delivered to maintain healthy ecosystem engagement')
    await expect(errorText).toBeVisible({ timeout: 5000 })
  })
})
