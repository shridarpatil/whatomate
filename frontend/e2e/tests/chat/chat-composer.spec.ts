import { test, expect, request as playwrightRequest } from '@playwright/test'
import { Client } from 'pg'
import { loginAsAdmin, ApiHelper } from '../../helpers'
import { ChatPage } from '../../pages'
import { createTestScope } from '../../framework'

/**
 * Chat composer interactions — reply, send via Enter, empty-disabled.
 *
 * Sends go through POST /api/contacts/:id/messages. The test WhatsApp
 * account has a fake access_token, so the backend will mark the message
 * status='failed' after persisting the row — but the optimistic bubble
 * still renders (the composer flow is what's under test, not delivery).
 *
 * Setup pins last_inbound_at to a recent timestamp so the 24-hour
 * service window is open and the freeform composer is visible (the
 * "Send Template" banner replaces it otherwise).
 */

const DB_URL = process.env.TEST_DATABASE_URL || 'postgres://whatomate:whatomate@127.0.0.1:5432/whatomate'

async function execSQL(sql: string): Promise<Record<string, unknown>[]> {
  const client = new Client({ connectionString: DB_URL })
  await client.connect()
  try {
    const result = await client.query(sql)
    return result.rows as Record<string, unknown>[]
  } finally {
    await client.end()
  }
}

const scope = createTestScope('chat-composer')

test.describe('Chat composer interactions', () => {
  test.describe.configure({ mode: 'serial' })
  test.setTimeout(60_000)

  let contactId: string
  let orgId: string
  let accountName: string
  let incomingMessageId: string

  test.beforeAll(async () => {
    const ctx = await playwrightRequest.newContext()
    const api = new ApiHelper(ctx)
    await api.loginAsAdmin()

    const phone = scope.phone()
    const contact = await api.createContact(phone, scope.name('contact'))
    contactId = contact.id

    const orgRows = await execSQL(
      `SELECT organization_id FROM contacts WHERE id = '${contactId}'`,
    )
    orgId = orgRows[0]!.organization_id as string

    const accounts = await api.getWhatsAppAccounts().catch(() => [] as { name: string }[])
    if (accounts.length > 0) {
      accountName = accounts[0]!.name
    } else {
      const acc = await api.createWhatsAppAccount({
        name: scope.name('acc').toLowerCase().replace(/\s/g, '-'),
        phone_id: `phone-${Date.now()}`,
        business_id: `biz-${Date.now()}`,
        access_token: 'test-token-composer',
      })
      accountName = acc.name
    }

    // Pin contact to the account + open the service window.
    await execSQL(`
      UPDATE contacts
      SET whats_app_account = '${accountName}',
          last_inbound_at = NOW() - INTERVAL '1 hour'
      WHERE id = '${contactId}'
    `)

    // Seed an incoming message we can reply to. Customers' incoming
    // messages can't be created via the API (they only arrive through
    // WhatsApp webhooks), so we insert directly.
    const rows = await execSQL(`
      INSERT INTO messages (id, organization_id, whats_app_account, contact_id, direction, message_type, content, status, created_at, updated_at)
      VALUES (gen_random_uuid(), '${orgId}', '${accountName}', '${contactId}', 'incoming', 'text', 'Original from customer', 'delivered', NOW() - INTERVAL '5 minutes', NOW())
      RETURNING id::text AS id
    `)
    incomingMessageId = rows[0]!.id as string

    await ctx.dispose()
  })

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)
    // Wait for the seeded incoming bubble to mount before each test.
    await expect(page.locator(`#message-${incomingMessageId}`)).toBeVisible({ timeout: 10_000 })
  })

  test('Send button is disabled when composer is empty', async ({ page }) => {
    const composer = page.getByPlaceholder(/Type a message/i)
    await expect(composer).toBeVisible()
    await expect(composer).toHaveValue('')

    const sendBtn = page.locator('form button[type="submit"]').last()
    await expect(sendBtn).toBeDisabled()
  })

  test('Send button stays disabled for whitespace-only input', async ({ page }) => {
    const composer = page.getByPlaceholder(/Type a message/i)
    await composer.fill('     ')

    // Submit handler trims before sending; the button uses the same
    // !messageInput.trim() check so it should stay disabled.
    const sendBtn = page.locator('form button[type="submit"]').last()
    await expect(sendBtn).toBeDisabled()
  })

  test('pressing Enter in the composer sends the message', async ({ page }) => {
    const messageText = `Sent via Enter ${Date.now()}`
    const composer = page.getByPlaceholder(/Type a message/i)
    await composer.fill(messageText)
    // The textarea binds @keydown.enter.exact.prevent="sendMessage", so a
    // press here exercises that path (vs. clicking the submit button).
    await composer.press('Enter')

    const bubble = page.locator('.chat-bubble-outgoing').filter({ hasText: messageText }).last()
    await expect(bubble).toBeVisible({ timeout: 10_000 })
    await expect(composer).toHaveValue('')
  })

  test('clicking Reply on a bubble shows the reply indicator', async ({ page }) => {
    // Hover the message group container so the reply button (visible only
    // on hover via opacity-0 group-hover:opacity-100) becomes interactive.
    const wrap = page.locator(`#message-${incomingMessageId}`)
    await wrap.hover()

    // Click the Reply icon button next to the bubble.
    const replyBtn = wrap.locator('xpath=..').locator('button').filter({
      has: page.locator('[class*="lucide-reply"]'),
    }).first()
    await expect(replyBtn).toBeVisible({ timeout: 5_000 })
    await replyBtn.click()

    // The reply indicator above the composer should appear with
    // "Replying to <Customer>" and the original text.
    await expect(page.getByText(/Replying to /i)).toBeVisible({ timeout: 5_000 })
    await expect(page.getByText('Original from customer').last()).toBeVisible()
  })

  test('cancelling a reply (X) hides the reply indicator', async ({ page }) => {
    const wrap = page.locator(`#message-${incomingMessageId}`)
    await wrap.hover()
    await wrap.locator('xpath=..').locator('button').filter({
      has: page.locator('[class*="lucide-reply"]'),
    }).first().click()

    const indicator = page.getByText(/Replying to /i)
    await expect(indicator).toBeVisible({ timeout: 5_000 })

    // Cancel — the X button to the right of the indicator.
    await page.locator('button').filter({
      has: page.locator('[class*="lucide-x"]'),
    }).filter({ hasNot: page.locator('main *') }).first().click().catch(async () => {
      // Fallback: scope to the sibling within the indicator container.
      await indicator.locator('xpath=ancestor::div[contains(@class, "border-t")][1]')
        .locator('button').click()
    })

    await expect(indicator).toHaveCount(0)
  })

  test('sending a reply produces a bubble with the reply preview', async ({ page }) => {
    const wrap = page.locator(`#message-${incomingMessageId}`)
    await wrap.hover()
    await wrap.locator('xpath=..').locator('button').filter({
      has: page.locator('[class*="lucide-reply"]'),
    }).first().click()

    await expect(page.getByText(/Replying to /i)).toBeVisible({ timeout: 5_000 })

    const replyText = `My reply ${Date.now()}`
    const composer = page.getByPlaceholder(/Type a message/i)
    await composer.fill(replyText)
    await page.locator('form button[type="submit"]').last().click()

    // The new outgoing bubble must contain the reply text AND a
    // .reply-preview block referencing the original incoming message.
    const newBubble = page.locator('.chat-bubble-outgoing').filter({ hasText: replyText }).last()
    await expect(newBubble).toBeVisible({ timeout: 10_000 })
    await expect(newBubble.locator('.reply-preview')).toBeVisible()
    await expect(newBubble.locator('.reply-preview')).toContainText('Original from customer')

    // Reply indicator goes away on successful send.
    await expect(page.getByText(/Replying to /i)).toHaveCount(0)
  })
})
