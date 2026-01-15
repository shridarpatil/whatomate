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

  test('should have chatbot enable/disable switch', async () => {
    await expect(chatbotHubPage.enableSwitch).toBeVisible()
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
    const initialState = await chatbotHubPage.enableSwitch.getAttribute('data-state')
    await chatbotHubPage.toggleChatbot()
    const newState = await chatbotHubPage.enableSwitch.getAttribute('data-state')
    expect(newState).not.toBe(initialState)
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

  test('should navigate to transfers page', async ({ page }) => {
    await chatbotHubPage.navigateToTransfers()
    await expect(page).toHaveURL(/\/chatbot\/transfers/)
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

  test('should show keyword rules count', async ({ page }) => {
    await expect(page.getByText(/Keyword Rules|Keywords/i)).toBeVisible()
  })

  test('should show flows count', async ({ page }) => {
    await expect(page.getByText(/Flows/i)).toBeVisible()
  })

  test('should show AI contexts count', async ({ page }) => {
    await expect(page.getByText(/AI Context/i)).toBeVisible()
  })
})

test.describe('Chatbot Hub Card Content', () => {
  let chatbotHubPage: ChatbotHubPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatbotHubPage = new ChatbotHubPage(page)
    await chatbotHubPage.goto()
  })

  test('should show keywords card description', async ({ page }) => {
    await expect(chatbotHubPage.keywordsCard.getByText(/keyword|trigger/i)).toBeVisible()
  })

  test('should show flows card description', async ({ page }) => {
    await expect(chatbotHubPage.flowsCard.getByText(/flow|conversation/i)).toBeVisible()
  })

  test('should show AI contexts card description', async ({ page }) => {
    await expect(chatbotHubPage.aiContextsCard.getByText(/AI|context/i)).toBeVisible()
  })
})
