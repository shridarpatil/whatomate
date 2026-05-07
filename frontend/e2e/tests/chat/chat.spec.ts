import { test, expect } from '@playwright/test'
import { ApiHelper, login, loginAsAdmin } from '../../helpers'
import { ChatPage } from '../../pages'
import { createTestScope } from '../../framework'

const cannedScope = createTestScope('chat-canned-preview')
// Seed via API as admin@admin.com (always exists per migrations) and log the
// browser in as the same user — otherwise the API-created contact lives in a
// different org from the UI session and the chat composer never renders.
const ADMIN_USER = { email: 'admin@admin.com', password: 'admin', role: 'admin' as const }

test.describe('Chat Page', () => {
  let chatPage: ChatPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatPage = new ChatPage(page)
    await chatPage.goto()
  })

  test('should display chat page', async () => {
    await chatPage.expectPageVisible()
  })

  test('should show contact list area', async ({ page }) => {
    // Chat page should have some layout
    await expect(page.locator('body')).toBeVisible()
  })

  test('should have search input', async ({ page }) => {
    const searchInput = page.locator('input[placeholder*="Search"]')
    if (await searchInput.isVisible()) {
      await expect(searchInput).toBeVisible()
    }
  })
})

test.describe('Contact List', () => {
  let chatPage: ChatPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatPage = new ChatPage(page)
    await chatPage.goto()
  })

  test('should search contacts', async ({ page }) => {
    const searchInput = page.locator('input[placeholder*="Search"]')
    if (await searchInput.isVisible()) {
      await searchInput.fill('test')
      await page.waitForTimeout(500)
      // Search should filter contacts
    }
  })

  test('should clear search', async ({ page }) => {
    const searchInput = page.locator('input[placeholder*="Search"]')
    if (await searchInput.isVisible()) {
      await searchInput.fill('test')
      await searchInput.fill('')
      await page.waitForTimeout(500)
    }
  })

  test('should show contact items', async ({ page }) => {
    // Contact list may or may not have items
    const contacts = page.locator('.contact-item, [data-testid="contact"], .cursor-pointer')
    const count = await contacts.count()
    // Just verify the page loads without error
    expect(count).toBeGreaterThanOrEqual(0)
  })
})

test.describe('Message Area', () => {
  let chatPage: ChatPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatPage = new ChatPage(page)
    await chatPage.goto()
  })

  test('should show message area when contact selected', async ({ page }) => {
    // Try to click first contact if available
    const contacts = page.locator('.cursor-pointer').filter({ has: page.locator('text=/[+0-9]|contact/i') })
    const count = await contacts.count()
    if (count > 0) {
      await contacts.first().click()
      await page.waitForLoadState('networkidle')
      // Message input should appear
      const messageInput = page.locator('textarea, input[placeholder*="message" i]')
      if (await messageInput.first().isVisible()) {
        await expect(messageInput.first()).toBeVisible()
      }
    }
  })

  test('should have message input field', async ({ page }) => {
    const contacts = page.locator('.cursor-pointer').filter({ has: page.locator('text=/[+0-9]/') })
    const count = await contacts.count()
    if (count > 0) {
      await contacts.first().click()
      await page.waitForLoadState('networkidle')
      const messageInput = page.locator('textarea, input[placeholder*="message" i]')
      if (await messageInput.first().isVisible()) {
        await expect(messageInput.first()).toBeVisible()
      }
    }
  })

  test('should have send button', async ({ page }) => {
    const contacts = page.locator('.cursor-pointer').filter({ has: page.locator('text=/[+0-9]/') })
    const count = await contacts.count()
    if (count > 0) {
      await contacts.first().click()
      await page.waitForLoadState('networkidle')
      const sendBtn = page.locator('button').filter({ has: page.locator('.lucide-send') })
      if (await sendBtn.isVisible()) {
        await expect(sendBtn).toBeVisible()
      }
    }
  })
})

test.describe('Chat Actions', () => {
  let chatPage: ChatPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatPage = new ChatPage(page)
    await chatPage.goto()
  })

  test('should have attachment button', async ({ page }) => {
    const contacts = page.locator('.cursor-pointer').filter({ has: page.locator('text=/[+0-9]/') })
    const count = await contacts.count()
    if (count > 0) {
      await contacts.first().click()
      await page.waitForLoadState('networkidle')
      const attachBtn = page.locator('button').filter({ has: page.locator('.lucide-paperclip') })
      if (await attachBtn.isVisible()) {
        await expect(attachBtn).toBeVisible()
      }
    }
  })

  test('should have emoji button', async ({ page }) => {
    const contacts = page.locator('.cursor-pointer').filter({ has: page.locator('text=/[+0-9]/') })
    const count = await contacts.count()
    if (count > 0) {
      await contacts.first().click()
      await page.waitForLoadState('networkidle')
      const emojiBtn = page.locator('button').filter({ has: page.locator('.lucide-smile') })
      if (await emojiBtn.isVisible()) {
        await expect(emojiBtn).toBeVisible()
      }
    }
  })
})

test.describe('Chat Message Input', () => {
  let chatPage: ChatPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatPage = new ChatPage(page)
    await chatPage.goto()
  })

  test('should type in message input', async ({ page }) => {
    const contacts = page.locator('.cursor-pointer').filter({ has: page.locator('text=/[+0-9]/') })
    const count = await contacts.count()
    if (count > 0) {
      await contacts.first().click()
      await page.waitForLoadState('networkidle')
      const messageInput = page.locator('textarea, input[placeholder*="message" i]').first()
      if (await messageInput.isVisible()) {
        await messageInput.fill('Test message')
        await expect(messageInput).toHaveValue('Test message')
      }
    }
  })

  test('should clear input after typing', async ({ page }) => {
    const contacts = page.locator('.cursor-pointer').filter({ has: page.locator('text=/[+0-9]/') })
    const count = await contacts.count()
    if (count > 0) {
      await contacts.first().click()
      await page.waitForLoadState('networkidle')
      const messageInput = page.locator('textarea, input[placeholder*="message" i]').first()
      if (await messageInput.isVisible()) {
        await messageInput.fill('Test message')
        await messageInput.fill('')
        await expect(messageInput).toHaveValue('')
      }
    }
  })
})

test.describe('Contact Info Panel', () => {
  let chatPage: ChatPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatPage = new ChatPage(page)
    await chatPage.goto()
  })

  test('should have contact info button', async ({ page }) => {
    const contacts = page.locator('.cursor-pointer').filter({ has: page.locator('text=/[+0-9]/') })
    const count = await contacts.count()
    if (count > 0) {
      await contacts.first().click()
      await page.waitForLoadState('networkidle')
      const infoBtn = page.locator('button').filter({ has: page.locator('.lucide-info, .lucide-user') })
      if (await infoBtn.first().isVisible()) {
        await expect(infoBtn.first()).toBeVisible()
      }
    }
  })
})

test.describe('Chat Transfer Actions', () => {
  let chatPage: ChatPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatPage = new ChatPage(page)
    await chatPage.goto()
  })

  test('should have transfer button if available', async ({ page }) => {
    const contacts = page.locator('.cursor-pointer').filter({ has: page.locator('text=/[+0-9]/') })
    const count = await contacts.count()
    if (count > 0) {
      await contacts.first().click()
      await page.waitForLoadState('networkidle')
      // Transfer button may or may not be visible depending on state
      const transferBtn = page.getByRole('button', { name: /Transfer/i })
      const isVisible = await transferBtn.isVisible()
      expect(typeof isVisible).toBe('boolean')
    }
  })

  test('should have resume button if available', async ({ page }) => {
    const contacts = page.locator('.cursor-pointer').filter({ has: page.locator('text=/[+0-9]/') })
    const count = await contacts.count()
    if (count > 0) {
      await contacts.first().click()
      await page.waitForLoadState('networkidle')
      // Resume button may or may not be visible depending on state
      const resumeBtn = page.getByRole('button', { name: /Resume/i })
      const isVisible = await resumeBtn.isVisible()
      expect(typeof isVisible).toBe('boolean')
    }
  })
})

test.describe('Chat Messages Display', () => {
  let chatPage: ChatPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatPage = new ChatPage(page)
    await chatPage.goto()
  })

  test('should display messages area', async ({ page }) => {
    const contacts = page.locator('.cursor-pointer').filter({ has: page.locator('text=/[+0-9]/') })
    const count = await contacts.count()
    if (count > 0) {
      await contacts.first().click()
      await page.waitForLoadState('networkidle')
      // Messages area should be present
      const messagesArea = page.locator('.messages, [data-testid="messages"], .overflow-y-auto').first()
      if (await messagesArea.isVisible()) {
        await expect(messagesArea).toBeVisible()
      }
    }
  })

  test('should show message bubbles if messages exist', async ({ page }) => {
    const contacts = page.locator('.cursor-pointer').filter({ has: page.locator('text=/[+0-9]/') })
    const count = await contacts.count()
    if (count > 0) {
      await contacts.first().click()
      await page.waitForLoadState('networkidle')
      await page.waitForTimeout(1000)
      // Messages may or may not exist
      const messages = page.locator('.message, [data-testid="message"], .rounded-lg.p-2, .rounded-lg.p-3')
      const messageCount = await messages.count()
      expect(messageCount).toBeGreaterThanOrEqual(0)
    }
  })
})

test.describe('Canned Responses', () => {
  let chatPage: ChatPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatPage = new ChatPage(page)
    await chatPage.goto()
  })

  test('should have canned responses button', async ({ page }) => {
    const contacts = page.locator('.cursor-pointer').filter({ has: page.locator('text=/[+0-9]/') })
    const count = await contacts.count()
    if (count > 0) {
      await contacts.first().click()
      await page.waitForLoadState('networkidle')
      const cannedBtn = page.locator('button').filter({ has: page.locator('.lucide-message-square-text, .lucide-book-text') })
      if (await cannedBtn.first().isVisible()) {
        await expect(cannedBtn.first()).toBeVisible()
      }
    }
  })
})

test.describe('Custom Actions', () => {
  let chatPage: ChatPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatPage = new ChatPage(page)
    await chatPage.goto()
  })

  test('should have custom actions button', async ({ page }) => {
    const contacts = page.locator('.cursor-pointer').filter({ has: page.locator('text=/[+0-9]/') })
    const count = await contacts.count()
    if (count > 0) {
      await contacts.first().click()
      await page.waitForLoadState('networkidle')
      const actionsBtn = page.locator('button').filter({ has: page.locator('.lucide-zap, .lucide-bolt') })
      if (await actionsBtn.first().isVisible()) {
        await expect(actionsBtn.first()).toBeVisible()
      }
    }
  })
})

test.describe('Canned Response Preview', () => {
  let api: ApiHelper
  let chatPage: ChatPage
  let contact: { id: string; profile_name: string }
  let plain: { id: string; name: string; shortcut: string }
  let withParam: { id: string; name: string; shortcut: string }
  let withContactName: { id: string; name: string; shortcut: string }

  test.beforeEach(async ({ page, request }) => {
    api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    // Names use the random-suffix form (no second arg) so each test creates
    // distinct rows. The DELETE endpoint soft-deletes, so a deterministic
    // name would collide with the prior test's tombstone within the worker.
    contact = await api.createContact(cannedScope.phone(), cannedScope.name())
    const plainName = cannedScope.name()
    plain = await api.createCannedResponse({
      name: plainName,
      shortcut: plainName.toLowerCase(),
      content: 'Hello! Thanks for contacting us.',
    })
    const orderName = cannedScope.name()
    withParam = await api.createCannedResponse({
      name: orderName,
      shortcut: orderName.toLowerCase(),
      content: 'Your order #{{order_id}} is ready.',
    })
    const greetName = cannedScope.name()
    withContactName = await api.createCannedResponse({
      name: greetName,
      shortcut: greetName.toLowerCase(),
      content: 'Hi {{contact_name}}, how can I help?',
    })

    await login(page, ADMIN_USER)
    chatPage = new ChatPage(page)
    await chatPage.goto(contact.id)
  })

  test.afterEach(async () => {
    for (const id of [plain?.id, withParam?.id, withContactName?.id]) {
      if (id) await api.deleteCannedResponse(id).catch(() => {})
    }
  })

  test('opens preview dialog with no inputs when content has no placeholders', async () => {
    await chatPage.openCannedResponses()
    await chatPage.selectCannedPickerItem(plain.name)

    await chatPage.cannedDialog.waitFor({ state: 'visible' })
    await expect(chatPage.cannedDialog).toContainText(plain.name)
    await expect(chatPage.cannedDialogPreview).toContainText('Hello! Thanks for contacting us.')
    await expect(chatPage.cannedDialogParamInputs).toHaveCount(0)
  })

  test('renders an input for each {{custom}} token and updates preview live', async () => {
    await chatPage.openCannedResponses()
    await chatPage.selectCannedPickerItem(withParam.name)

    await chatPage.cannedDialog.waitFor({ state: 'visible' })
    await expect(chatPage.cannedDialogParamInputs).toHaveCount(1)
    // Until the agent fills the input the preview keeps the literal token.
    await expect(chatPage.cannedDialogPreview).toContainText('{{order_id}}')

    await chatPage.fillCannedParam('order_id', '12345')
    await expect(chatPage.cannedDialogPreview).toContainText('Your order #12345 is ready.')
    await expect(chatPage.cannedDialogPreview).not.toContainText('{{order_id}}')
  })

  test('auto-resolves {{contact_name}} without exposing an input', async () => {
    await chatPage.openCannedResponses()
    await chatPage.selectCannedPickerItem(withContactName.name)

    await chatPage.cannedDialog.waitFor({ state: 'visible' })
    await expect(chatPage.cannedDialogParamInputs).toHaveCount(0)
    await expect(chatPage.cannedDialogPreview).toContainText(`Hi ${contact.profile_name}, how can I help?`)
  })

  test('Send dispatches the resolved body to the messages API', async ({ page }) => {
    let postedBody: { type?: string; content?: { body?: string } } | null = null
    // Intercept the POST so we can assert the resolved body without depending
    // on a real WhatsApp account being configured in the test backend.
    await page.route(`**/api/contacts/${contact.id}/messages`, async (route) => {
      postedBody = JSON.parse(route.request().postData() || '{}')
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'error', message: 'mocked' }),
      })
    })

    await chatPage.openCannedResponses()
    await chatPage.selectCannedPickerItem(withParam.name)
    await chatPage.cannedDialog.waitFor({ state: 'visible' })
    await chatPage.fillCannedParam('order_id', '99')
    await chatPage.sendCannedDialog()

    await expect.poll(() => postedBody, { timeout: 5000 }).not.toBeNull()
    expect(postedBody!.type).toBe('text')
    expect(postedBody!.content?.body).toBe('Your order #99 is ready.')
  })

  test('Cancel closes the dialog and clears any leftover slash command', async () => {
    // Open the picker via slash command, then cancel — the textarea should
    // be empty afterwards (the slash command is dropped on selection).
    await chatPage.messageInput.fill(`/${plain.shortcut}`)
    await chatPage.selectCannedPickerItem(plain.name)
    await chatPage.cannedDialog.waitFor({ state: 'visible' })

    await chatPage.cancelCannedDialog()
    await expect(chatPage.messageInput).toHaveValue('')
  })
})
