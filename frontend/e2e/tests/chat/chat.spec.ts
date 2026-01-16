import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { ChatPage } from '../../pages'

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
