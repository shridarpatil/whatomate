import { test, expect, request as playwrightRequest } from '@playwright/test'
import { Client } from 'pg'
import { loginAsAdmin, ApiHelper } from '../../helpers'
import { ChatPage } from '../../pages'
import { createTestScope } from '../../framework'

/**
 * Chat bubble rendering — driven through the chat UI.
 *
 * Inbound messages can't normally be created through the API (they only
 * arrive via WhatsApp webhooks), so we seed a contact + messages of every
 * shape directly into the DB. The test then opens the chat for that
 * contact and asserts each bubble renders with the right class, content,
 * and affordances.
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

const scope = createTestScope('message-bubbles')

test.describe('Chat message bubbles', () => {
  test.describe.configure({ mode: 'serial' })
  test.setTimeout(60_000)

  let contactId: string
  let orgId: string
  let outgoingTextId: string
  let incomingTextId: string
  let failedOutgoingId: string
  let replyTextId: string

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

    // Anchor every test bubble in the past so the messages list (sorted ASC)
    // shows them in a deterministic order regardless of clock drift between
    // the seed step and the page mount.
    const seed = async (
      direction: 'incoming' | 'outgoing',
      text: string,
      status: string,
      extra: { errorMessage?: string; replyToId?: string; minutesAgo?: number } = {},
    ): Promise<string> => {
      const minutesAgo = extra.minutesAgo ?? 5
      const errorClause = extra.errorMessage
        ? `, error_message = '${extra.errorMessage.replace(/'/g, "''")}'`
        : ''
      const replyCols = extra.replyToId ? `, is_reply = true, reply_to_message_id = '${extra.replyToId}'` : ''
      const rows = await execSQL(`
        INSERT INTO messages (id, organization_id, whats_app_account, contact_id, direction, message_type, content, status, created_at, updated_at)
        VALUES (gen_random_uuid(), '${orgId}', 'test-account', '${contactId}', '${direction}', 'text', '${text.replace(/'/g, "''")}', '${status}', NOW() - INTERVAL '${minutesAgo} minutes', NOW())
        RETURNING id::text AS id
      `)
      const id = rows[0]!.id as string
      if (errorClause || replyCols) {
        await execSQL(`UPDATE messages SET id = '${id}'${errorClause}${replyCols} WHERE id = '${id}'`)
      }
      return id
    }

    incomingTextId = await seed('incoming', 'Hi from the customer', 'delivered', { minutesAgo: 30 })
    outgoingTextId = await seed('outgoing', 'Hello from the agent', 'read', { minutesAgo: 25 })
    failedOutgoingId = await seed('outgoing', 'This one failed', 'failed', {
      errorMessage: 'Re-engagement message: not delivered',
      minutesAgo: 20,
    })
    replyTextId = await seed('outgoing', 'Replying to your earlier message', 'delivered', {
      replyToId: incomingTextId,
      minutesAgo: 10,
    })

    await ctx.dispose()
  })

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
  })

  test('outgoing text bubble renders content, status icon, and timestamp', async ({ page }) => {
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    const bubble = page.locator(`#message-${outgoingTextId} .chat-bubble`)
    await expect(bubble).toBeVisible({ timeout: 10_000 })
    await expect(bubble).toHaveClass(/chat-bubble-outgoing/)
    await expect(bubble).toContainText('Hello from the agent')

    // Outgoing bubbles include a timestamp + status icon.
    await expect(bubble.locator('.chat-bubble-time')).toBeVisible()
    await expect(bubble.locator('.status-icon')).toBeVisible()
  })

  test('incoming text bubble renders without status icon', async ({ page }) => {
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    const bubble = page.locator(`#message-${incomingTextId} .chat-bubble`)
    await expect(bubble).toBeVisible({ timeout: 10_000 })
    await expect(bubble).toHaveClass(/chat-bubble-incoming/)
    await expect(bubble).toContainText('Hi from the customer')

    // Status icon only renders for outgoing direction.
    await expect(bubble.locator('.status-icon')).toHaveCount(0)
  })

  test('failed outgoing bubble shows the error message', async ({ page }) => {
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    const bubble = page.locator(`#message-${failedOutgoingId} .chat-bubble`)
    await expect(bubble).toBeVisible({ timeout: 10_000 })
    await expect(bubble).toContainText('This one failed')

    // The error_message text from the DB should be visible somewhere on the
    // bubble — the exact location is a sibling block in ChatView.
    const messageWrap = page.locator(`#message-${failedOutgoingId}`)
    await expect(messageWrap).toContainText(/not delivered/i)
  })

  test('reply preview links the bubble to the original incoming message', async ({ page }) => {
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    const bubble = page.locator(`#message-${replyTextId} .chat-bubble`)
    await expect(bubble).toBeVisible({ timeout: 10_000 })

    // Reply preview block sits inside the bubble and shows the original text.
    const preview = bubble.locator('.reply-preview')
    await expect(preview).toBeVisible()
    await expect(preview).toContainText('Hi from the customer')
  })

  test('clicking the reply preview scrolls to the original message', async ({ page }) => {
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    const replyBubble = page.locator(`#message-${replyTextId} .chat-bubble`)
    await expect(replyBubble).toBeVisible({ timeout: 10_000 })

    await replyBubble.locator('.reply-preview').click()

    // The original incoming bubble should be scrolled into view.
    await expect(page.locator(`#message-${incomingTextId} .chat-bubble`)).toBeInViewport()
  })

  test('all seeded bubbles render in chronological order', async ({ page }) => {
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    // Wait for at least one seeded bubble to mount before counting.
    await expect(page.locator(`#message-${incomingTextId}`)).toBeVisible({ timeout: 10_000 })

    const ids = [incomingTextId, outgoingTextId, failedOutgoingId, replyTextId]
    const positions = await Promise.all(
      ids.map(async id => {
        const box = await page.locator(`#message-${id}`).boundingBox()
        return box ? box.y : -1
      }),
    )

    // Each position should be strictly greater than the previous (newer
    // messages are below older ones in chronological view).
    for (let i = 1; i < positions.length; i++) {
      expect(positions[i]).toBeGreaterThan(positions[i - 1])
    }
  })
})

test.describe('Chat template bubble', () => {
  test.describe.configure({ mode: 'serial' })
  test.setTimeout(60_000)

  let contactId: string
  let orgId: string
  let accountName: string

  test.beforeAll(async () => {
    const ctx = await playwrightRequest.newContext()
    const api = new ApiHelper(ctx)
    await api.loginAsAdmin()

    const phone = scope.phone()
    const contact = await api.createContact(phone, scope.name('tpl-contact'))
    contactId = contact.id

    const orgRows = await execSQL(
      `SELECT organization_id FROM contacts WHERE id = '${contactId}'`,
    )
    orgId = orgRows[0]!.organization_id as string

    // Reuse an existing account or create one — templates require it.
    const accounts = await api.getWhatsAppAccounts().catch(() => [] as { name: string }[])
    if (accounts.length > 0) {
      accountName = accounts[0]!.name
    } else {
      const acc = await api.createWhatsAppAccount({
        name: scope.name('acc').toLowerCase().replace(/\s/g, '-'),
        phone_id: `phone-${Date.now()}`,
        business_id: `biz-${Date.now()}`,
        access_token: 'test-token-bubbles',
      })
      accountName = acc.name
    }
    await execSQL(
      `UPDATE contacts SET whats_app_account = '${accountName}' WHERE id = '${contactId}'`,
    )

    await ctx.dispose()
  })

  test('template message renders as an outgoing bubble with body text', async ({ page }) => {
    // Seed a delivered template message directly so the bubble is visible
    // without depending on a successful Meta API roundtrip.
    const rows = await execSQL(`
      INSERT INTO messages (id, organization_id, whats_app_account, contact_id, direction, message_type, content, template_name, status, created_at, updated_at)
      VALUES (gen_random_uuid(), '${orgId}', '${accountName}', '${contactId}', 'outgoing', 'template', 'Welcome Alice! Your order ORD-1 is confirmed.', 'welcome_tpl', 'delivered', NOW() - INTERVAL '5 minutes', NOW())
      RETURNING id::text AS id
    `)
    const tplMessageId = rows[0]!.id as string

    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    const bubble = page.locator(`#message-${tplMessageId} .chat-bubble`)
    await expect(bubble).toBeVisible({ timeout: 10_000 })
    await expect(bubble).toHaveClass(/chat-bubble-outgoing/)
    await expect(bubble).toContainText('Welcome Alice')
    await expect(bubble).toContainText('ORD-1')
    await expect(bubble.locator('.status-icon')).toBeVisible()
  })
})

test.describe('Sending a text message via the UI', () => {
  test.describe.configure({ mode: 'serial' })
  test.setTimeout(60_000)

  let contactId: string

  test.beforeAll(async () => {
    const ctx = await playwrightRequest.newContext()
    const api = new ApiHelper(ctx)
    await api.loginAsAdmin()

    const contact = await api.createContact(scope.phone(), scope.name('send-contact'))
    contactId = contact.id

    // Reuse / create a WhatsApp account and pin the contact to it. Without
    // this the message input falls back to the "no account" state and the
    // composer is hidden.
    const accounts = await api.getWhatsAppAccounts().catch(() => [] as { name: string }[])
    let accountName: string
    if (accounts.length > 0) {
      accountName = accounts[0]!.name
    } else {
      const acc = await api.createWhatsAppAccount({
        name: scope.name('send-acc').toLowerCase().replace(/\s/g, '-'),
        phone_id: `phone-${Date.now()}`,
        business_id: `biz-${Date.now()}`,
        access_token: 'test-token-bubbles',
      })
      accountName = acc.name
    }
    // Open the 24-hour service window so the freeform composer is enabled
    // (NULL last_inbound_at would render the "expired" banner instead).
    await execSQL(`
      UPDATE contacts
      SET whats_app_account = '${accountName}',
          last_inbound_at = NOW() - INTERVAL '1 hour'
      WHERE id = '${contactId}'
    `)

    await ctx.dispose()
  })

  test('typing into the composer and clicking Send creates an outgoing bubble', async ({ page }) => {
    await loginAsAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    const messageText = `Hello via UI ${Date.now()}`

    // Composer is the textarea with the Type a message... placeholder.
    const composer = page.getByPlaceholder(/Type a message/i)
    await expect(composer).toBeVisible({ timeout: 10_000 })
    await composer.fill(messageText)

    // Submit through the form. The textarea also binds Enter to sendMessage,
    // but pressing Enter via .press() works too — submitting via the button
    // exercises the same handler and is closer to a click-driven user flow.
    await page.locator('form button[type="submit"]').last().click()

    // The send hits POST /api/contacts/:id/messages — the backend persists
    // the message and the store appends it to the list. The bubble must
    // appear with our typed text and the outgoing class. Status (read /
    // delivered / failed) depends on whether the test WhatsApp account
    // accepts the send; we only assert the bubble renders.
    const bubble = page.locator('.chat-bubble-outgoing').filter({ hasText: messageText }).last()
    await expect(bubble).toBeVisible({ timeout: 10_000 })
    await expect(bubble).toContainText(messageText)
    await expect(bubble.locator('.chat-bubble-time')).toBeVisible()

    // The composer should be cleared after a successful send.
    await expect(composer).toHaveValue('')
  })
})
