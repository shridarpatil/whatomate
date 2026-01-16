import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { ChatbotHubPage } from '../../pages'

test.describe('Chatbot Hub Page', () => {
  let chatbotHubPage: ChatbotHubPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatbotHubPage = new ChatbotHubPage(page)
    await chatbotHubPage.goto()
  })

  test('should display chatbot hub page', async () => {
    await chatbotHubPage.expectPageVisible()
  })

  test('should have chatbot toggle button', async () => {
    await expect(chatbotHubPage.toggleButton).toBeVisible()
  })

  test('should have status badge', async () => {
    await expect(chatbotHubPage.statusBadge).toBeVisible()
  })

  test('should have keywords navigation card', async () => {
    await expect(chatbotHubPage.keywordsCard).toBeVisible()
  })

  test('should have flows navigation card', async () => {
    await expect(chatbotHubPage.flowsCard).toBeVisible()
  })

  test('should have AI contexts navigation card', async () => {
    await expect(chatbotHubPage.aiContextsCard).toBeVisible()
  })
})

test.describe('Chatbot Toggle', () => {
  let chatbotHubPage: ChatbotHubPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatbotHubPage = new ChatbotHubPage(page)
    await chatbotHubPage.goto()
  })

  test('should toggle chatbot on/off', async () => {
    const initialText = await chatbotHubPage.toggleButton.textContent()
    await chatbotHubPage.toggleChatbot()
    // Wait for the button text to change
    await expect(chatbotHubPage.toggleButton).not.toHaveText(initialText!)
  })

  test('should show toast on toggle', async () => {
    await chatbotHubPage.toggleChatbot()
    await chatbotHubPage.expectToast(/enabled|disabled/i)
  })
})

test.describe('Navigation Cards', () => {
  let chatbotHubPage: ChatbotHubPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatbotHubPage = new ChatbotHubPage(page)
    await chatbotHubPage.goto()
  })

  test('should navigate to keywords page', async ({ page }) => {
    await chatbotHubPage.navigateToKeywords()
    await expect(page).toHaveURL(/\/chatbot\/keywords/)
  })

  test('should navigate to flows page', async ({ page }) => {
    await chatbotHubPage.navigateToFlows()
    await expect(page).toHaveURL(/\/chatbot\/flows/)
  })

  test('should navigate to AI contexts page', async ({ page }) => {
    await chatbotHubPage.navigateToAIContexts()
    await expect(page).toHaveURL(/\/chatbot\/ai/)
  })
})

test.describe('Stats Cards', () => {
  let chatbotHubPage: ChatbotHubPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatbotHubPage = new ChatbotHubPage(page)
    await chatbotHubPage.goto()
  })

  test('should display stats cards', async ({ page }) => {
    // Stats cards should be visible
    const cards = page.locator('.rounded-lg.border')
    await expect(cards.first()).toBeVisible()
  })

  test('should show total sessions stat', async ({ page }) => {
    await expect(page.getByText('Total Sessions')).toBeVisible()
  })

  test('should show active sessions stat', async ({ page }) => {
    await expect(page.getByText('Active Sessions')).toBeVisible()
  })

  test('should show messages handled stat', async ({ page }) => {
    await expect(page.getByText('Messages Handled')).toBeVisible()
  })
})

test.describe('Chatbot Hub Card Content', () => {
  let chatbotHubPage: ChatbotHubPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatbotHubPage = new ChatbotHubPage(page)
    await chatbotHubPage.goto()
  })

  test('should show keywords card with title', async () => {
    // Title is in h3 element
    await expect(chatbotHubPage.keywordsCard.locator('h3')).toContainText('Keyword Rules')
  })

  test('should show flows card with title', async () => {
    await expect(chatbotHubPage.flowsCard.locator('h3')).toContainText('Conversation Flows')
  })

  test('should show AI contexts card with title', async () => {
    await expect(chatbotHubPage.aiContextsCard.locator('h3')).toContainText('AI Contexts')
  })

  test('should show keywords card description', async () => {
    // Description is the second p element (first is count)
    await expect(chatbotHubPage.keywordsCard.locator('p').last()).toContainText('automated responses')
  })

  test('should show flows card description', async () => {
    // Description is the second p element (first is count)
    await expect(chatbotHubPage.flowsCard.locator('p').last()).toContainText('multi-step conversation')
  })

  test('should show AI contexts card description', async () => {
    // Description is the second p element (first is count)
    await expect(chatbotHubPage.aiContextsCard.locator('p').last()).toContainText('AI-powered responses')
  })
})
