import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { ChatbotSettingsPage } from '../../pages'

test.describe('Chatbot Settings Page', () => {
  let chatbotSettingsPage: ChatbotSettingsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatbotSettingsPage = new ChatbotSettingsPage(page)
    await chatbotSettingsPage.goto()
  })

  test('should display chatbot settings page', async () => {
    await chatbotSettingsPage.expectPageVisible()
  })

  test('should have Messages tab', async () => {
    await expect(chatbotSettingsPage.messagesTab).toBeVisible()
  })

  test('should have Agents tab', async () => {
    await expect(chatbotSettingsPage.agentsTab).toBeVisible()
  })

  test('should have Hours tab', async () => {
    await expect(chatbotSettingsPage.hoursTab).toBeVisible()
  })

  test('should have SLA tab', async () => {
    await expect(chatbotSettingsPage.slaTab).toBeVisible()
  })

  test('should have AI tab', async () => {
    await expect(chatbotSettingsPage.aiTab).toBeVisible()
  })
})

test.describe('Messages Tab', () => {
  let chatbotSettingsPage: ChatbotSettingsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatbotSettingsPage = new ChatbotSettingsPage(page)
    await chatbotSettingsPage.goto()
  })

  test('should show messages tab by default', async () => {
    await chatbotSettingsPage.expectMessagesTabVisible()
  })

  test('should have greeting message field', async ({ page }) => {
    await expect(page.locator('textarea#greeting')).toBeVisible()
  })

  test('should have fallback message field', async ({ page }) => {
    await expect(page.locator('textarea#fallback')).toBeVisible()
  })

  test('should have session timeout field', async ({ page }) => {
    await expect(page.locator('input#timeout')).toBeVisible()
  })

  test('should have add greeting button option', async ({ page }) => {
    await expect(page.getByRole('button', { name: /Add Button/i }).first()).toBeVisible()
  })

  test('should have add fallback button option', async ({ page }) => {
    const addButtons = page.getByRole('button', { name: /Add Button/i })
    await expect(addButtons.last()).toBeVisible()
  })

  test('should fill greeting message', async ({ page }) => {
    await chatbotSettingsPage.fillGreetingMessage('Hello! Welcome to our service.')
    await expect(page.locator('textarea#greeting')).toHaveValue('Hello! Welcome to our service.')
  })

  test('should save messages settings', async () => {
    await chatbotSettingsPage.fillGreetingMessage('Test greeting')
    await chatbotSettingsPage.saveSettings()
    await chatbotSettingsPage.expectToast(/saved|success/i)
  })
})

test.describe('Agents Tab', () => {
  let chatbotSettingsPage: ChatbotSettingsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatbotSettingsPage = new ChatbotSettingsPage(page)
    await chatbotSettingsPage.goto()
    await chatbotSettingsPage.switchToAgentsTab()
  })

  test('should show agents settings', async () => {
    await chatbotSettingsPage.expectAgentsTabVisible()
  })

  test('should have allow queue pickup toggle', async ({ page }) => {
    await expect(page.getByText('Allow Agents to Pick from Queue')).toBeVisible()
  })

  test('should have assign to same agent toggle', async ({ page }) => {
    await expect(page.getByText('Assign to Same Agent')).toBeVisible()
  })

  test('should have current conversation only toggle', async ({ page }) => {
    await expect(page.getByText('Agents See Current Conversation Only')).toBeVisible()
  })

  test('should toggle agent queue pickup', async ({ page }) => {
    const toggle = page.locator('button[role="switch"]').first()
    const initialState = await toggle.getAttribute('data-state')
    await toggle.click()
    const newState = await toggle.getAttribute('data-state')
    expect(newState).not.toBe(initialState)
  })

  test('should save agent settings', async () => {
    await chatbotSettingsPage.saveSettings()
    await chatbotSettingsPage.expectToast(/saved|success/i)
  })
})

test.describe('Business Hours Tab', () => {
  let chatbotSettingsPage: ChatbotSettingsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatbotSettingsPage = new ChatbotSettingsPage(page)
    await chatbotSettingsPage.goto()
    await chatbotSettingsPage.switchToHoursTab()
  })

  test('should show business hours settings', async () => {
    await chatbotSettingsPage.expectHoursTabVisible()
  })

  test('should have enable business hours toggle', async ({ page }) => {
    await expect(page.getByText('Enable Business Hours')).toBeVisible()
  })

  test('should toggle business hours enabled', async ({ page }) => {
    const toggle = page.locator('button[role="switch"]').first()
    const initialState = await toggle.getAttribute('data-state')
    await toggle.click()
    const newState = await toggle.getAttribute('data-state')
    expect(newState).not.toBe(initialState)
  })

  test('should show day schedule when enabled', async ({ page }) => {
    const toggle = page.locator('button[role="switch"]').first()
    const state = await toggle.getAttribute('data-state')
    if (state === 'unchecked') {
      await toggle.click()
    }
    await expect(page.getByText('Monday')).toBeVisible()
    await expect(page.getByText('Tuesday')).toBeVisible()
  })

  test('should have out of hours message field', async ({ page }) => {
    const toggle = page.locator('button[role="switch"]').first()
    const state = await toggle.getAttribute('data-state')
    if (state === 'unchecked') {
      await toggle.click()
    }
    await expect(page.getByText('Out of Hours Message')).toBeVisible()
  })

  test('should save business hours settings', async () => {
    await chatbotSettingsPage.saveSettings()
    await chatbotSettingsPage.expectToast(/saved|success/i)
  })
})

test.describe('SLA Tab', () => {
  let chatbotSettingsPage: ChatbotSettingsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatbotSettingsPage = new ChatbotSettingsPage(page)
    await chatbotSettingsPage.goto()
    await chatbotSettingsPage.switchToSLATab()
  })

  test('should show SLA settings', async () => {
    await chatbotSettingsPage.expectSLATabVisible()
  })

  test('should have enable SLA toggle', async ({ page }) => {
    await expect(page.getByText('Enable SLA Tracking')).toBeVisible()
  })

  test('should toggle SLA enabled', async ({ page }) => {
    const toggle = page.locator('button[role="switch"]').first()
    const initialState = await toggle.getAttribute('data-state')
    await toggle.click()
    const newState = await toggle.getAttribute('data-state')
    expect(newState).not.toBe(initialState)
  })

  test('should show SLA fields when enabled', async ({ page }) => {
    const toggle = page.locator('button[role="switch"]').first()
    const state = await toggle.getAttribute('data-state')
    if (state === 'unchecked') {
      await toggle.click()
    }
    // Target labels specifically to avoid matching description text
    await expect(page.locator('label').filter({ hasText: /Response Time/i })).toBeVisible()
    await expect(page.locator('label').filter({ hasText: /Escalation Time/i })).toBeVisible()
  })

  test('should have client inactivity reminders toggle', async ({ page }) => {
    await expect(page.getByText('Client Inactivity Reminders')).toBeVisible()
  })

  test('should save SLA settings', async () => {
    await chatbotSettingsPage.saveSettings()
    await chatbotSettingsPage.expectToast(/saved|success/i)
  })
})

test.describe('AI Tab', () => {
  let chatbotSettingsPage: ChatbotSettingsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatbotSettingsPage = new ChatbotSettingsPage(page)
    await chatbotSettingsPage.goto()
    await chatbotSettingsPage.switchToAITab()
  })

  test('should show AI settings', async () => {
    await chatbotSettingsPage.expectAITabVisible()
  })

  test('should have enable AI toggle', async ({ page }) => {
    await expect(page.getByText('Enable AI Responses')).toBeVisible()
  })

  test('should toggle AI enabled', async ({ page }) => {
    const toggle = page.locator('button[role="switch"]').first()
    const initialState = await toggle.getAttribute('data-state')
    await toggle.click()
    const newState = await toggle.getAttribute('data-state')
    expect(newState).not.toBe(initialState)
  })

  test('should show AI configuration when enabled', async ({ page }) => {
    const toggle = page.locator('button[role="switch"]').first()
    const state = await toggle.getAttribute('data-state')
    if (state === 'unchecked') {
      await toggle.click()
    }
    await expect(page.locator('label').filter({ hasText: /^AI Provider$/ })).toBeVisible()
    await expect(page.locator('label').filter({ hasText: /^Model$/ })).toBeVisible()
  })

  test('should have API key field', async ({ page }) => {
    const toggle = page.locator('button[role="switch"]').first()
    const state = await toggle.getAttribute('data-state')
    if (state === 'unchecked') {
      await toggle.click()
    }
    await expect(page.locator('label').filter({ hasText: /^API Key$/ })).toBeVisible()
  })

  test('should have system prompt field', async ({ page }) => {
    const toggle = page.locator('button[role="switch"]').first()
    const state = await toggle.getAttribute('data-state')
    if (state === 'unchecked') {
      await toggle.click()
    }
    await expect(page.getByText('System Prompt')).toBeVisible()
  })

  test('should show AI providers', async ({ page }) => {
    const toggle = page.locator('button[role="switch"]').first()
    const state = await toggle.getAttribute('data-state')
    if (state === 'unchecked') {
      await toggle.click()
    }
    await page.locator('button[role="combobox"]').first().click()
    await expect(page.locator('[role="option"]').filter({ hasText: 'OpenAI' })).toBeVisible()
    await expect(page.locator('[role="option"]').filter({ hasText: 'Anthropic' })).toBeVisible()
    await page.keyboard.press('Escape')
  })

  test('should save AI settings', async () => {
    await chatbotSettingsPage.saveSettings()
    await chatbotSettingsPage.expectToast(/saved|success/i)
  })
})

test.describe('Tab Navigation', () => {
  let chatbotSettingsPage: ChatbotSettingsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatbotSettingsPage = new ChatbotSettingsPage(page)
    await chatbotSettingsPage.goto()
  })

  test('should switch to Agents tab', async () => {
    await chatbotSettingsPage.switchToAgentsTab()
    await chatbotSettingsPage.expectAgentsTabVisible()
  })

  test('should switch to Hours tab', async () => {
    await chatbotSettingsPage.switchToHoursTab()
    await chatbotSettingsPage.expectHoursTabVisible()
  })

  test('should switch to SLA tab', async () => {
    await chatbotSettingsPage.switchToSLATab()
    await chatbotSettingsPage.expectSLATabVisible()
  })

  test('should switch to AI tab', async () => {
    await chatbotSettingsPage.switchToAITab()
    await chatbotSettingsPage.expectAITabVisible()
  })

  test('should switch back to Messages tab', async () => {
    await chatbotSettingsPage.switchToAITab()
    await chatbotSettingsPage.switchToMessagesTab()
    await chatbotSettingsPage.expectMessagesTabVisible()
  })
})
