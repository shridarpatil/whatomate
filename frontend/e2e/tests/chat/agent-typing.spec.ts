import { test, expect, request as playwrightRequest } from '@playwright/test'
import { loginAsAdmin, loginAsManager, ApiHelper } from '../../helpers'
import { ChatPage } from '../../pages'
import { createTestScope } from '../../framework'

const scope = createTestScope('agent-typing')

/**
 * Agent Typing E2E Tests
 *
 * Two browser contexts share one contact: one agent types, the other must see
 * the indicator appear and then expire on its own after ~3s.
 */
test.describe('Agent Typing', () => {
  test.describe.configure({ mode: 'serial' })
  test.setTimeout(90000)

  let contactId: string
  let accountName: string

  test.beforeAll(async () => {
    const reqContext = await playwrightRequest.newContext()
    const api = new ApiHelper(reqContext)
    await api.loginAsAdmin()
    const contact = await api.createContact(scope.phone(), scope.name('contact'))
    contactId = contact.id

    // A send is rejected with "Failed to resolve WhatsApp account" unless the
    // org has one, so the outgoing-bubble test needs one to exist. The
    // upstream Meta call fails with these dummy credentials, but the message
    // row is created and broadcast before the send is attempted, which is all
    // this spec asserts on.
    let accounts: any[] = []
    try {
      accounts = await api.getWhatsAppAccounts()
    } catch {
      // ignore
    }
    if (accounts.length === 0) {
      const uid = Date.now().toString().slice(-8)
      await api.createWhatsAppAccount({
        name: `e2e-typing-${uid}`,
        phone_id: `phone-${uid}`,
        business_id: `biz-${uid}`,
        access_token: `token-${uid}`
      })
      accounts = await api.getWhatsAppAccounts()
    }
    accountName = accounts[0].name

    await reqContext.dispose()
  })

  test('shows and expires the typing indicator for another agent', async ({ browser }) => {
    const watcherCtx = await browser.newContext()
    const typistCtx = await browser.newContext()

    try {
      const watcher = await watcherCtx.newPage()
      const typist = await typistCtx.newPage()

      // Two distinct agent accounts: applyAgentTyping() on the frontend
      // intentionally drops an agent's own typing event, so watcher and
      // typist must be different users or the indicator never renders.
      await loginAsManager(watcher)
      await loginAsAdmin(typist)

      // Land on the conversation list first (not a direct deep link into the
      // contact) so the WebSocket — opened asynchronously on app mount — has
      // settled before the client sends set_contact. A deep link to
      // /chat/:id fires set_contact from onMounted while the socket may
      // still be CONNECTING; that message is silently dropped (no queueing
      // in wsService.send) and never retried, so the server never learns
      // this session is viewing the contact. Clicking in from the list is
      // also how a real agent normally opens a conversation.
      await new ChatPage(watcher).goto()
      await new ChatPage(typist).goto()

      const contactName = scope.name('contact')
      await watcher.locator('[data-testid="conversation-item"]').filter({ hasText: contactName }).click()
      await typist.locator('[data-testid="conversation-item"]').filter({ hasText: contactName }).click()

      // Give both sockets time to send set_contact and the server to register it
      await watcher.waitForTimeout(1000)

      await typist.locator('textarea').first().fill('escrevendo uma resposta')

      const indicator = watcher.getByText(/está digitando|is typing/)
      await expect(indicator).toBeVisible({ timeout: 5000 })

      // No further events: it must disappear on its own
      await expect(indicator).toBeHidden({ timeout: 8000 })
    } finally {
      await watcherCtx.close()
      await typistCtx.close()
    }
  })

  test('shows the sending agent name on an outgoing bubble', async ({ page }) => {
    // Deliberately watch as the *manager* while the *admin* sends. The sidebar
    // user menu always renders the logged-in user's full name, so an assertion
    // on the viewer's own name passes even with the bubble label deleted.
    // Asserting a different agent's name, scoped to .chat-bubble-outgoing,
    // cannot be satisfied by anything outside the bubble.
    await loginAsManager(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contactId)

    // The sidebar starts collapsed by default now, and the viewer's name in
    // UserMenu.vue is behind `v-if="!collapsed"` — not just CSS-hidden, it
    // isn't in the DOM at all when collapsed. Expand it so the sanity check
    // below has something to find.
    await page.getByRole('button', { name: /expand sidebar|expandir/i }).click()

    // Sanity: the sidebar shows the viewer, not the sender.
    await expect(page.getByText('Test Manager').first()).toBeVisible({ timeout: 10000 })

    const body = scope.name('bubble-msg')
    const reqContext = await playwrightRequest.newContext()
    try {
      const api = new ApiHelper(reqContext)
      await api.loginAsAdmin()
      const res = await api.post(`/api/contacts/${contactId}/messages`, {
        type: 'text',
        content: { body },
        whatsapp_account: accountName,
      })
      expect(res.status()).toBe(200)
    } finally {
      await reqContext.dispose()
    }

    // The name has to arrive on the live websocket broadcast — no reload here.
    const bubble = page.locator('.chat-bubble-outgoing').filter({ hasText: body })
    await expect(bubble).toBeVisible({ timeout: 15000 })
    await expect(bubble.getByText('Test Admin')).toBeVisible({ timeout: 15000 })
  })
})
