import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { ChatbotFlowBuilderPage } from '../../pages'

test.describe('Chatbot Flow Builder - Step Properties', () => {
  let builder: ChatbotFlowBuilderPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    builder = new ChatbotFlowBuilderPage(page)
    await builder.gotoNew()
    // Add a step and select it to open step properties
    await builder.addStep()
    await builder.selectStep(1)
  })

  test('should show step properties panel when step is selected', async () => {
    await expect(builder.page.getByRole('heading', { name: 'Step Properties' })).toBeVisible()
  })

  test('should show message type palette in center panel', async () => {
    await expect(builder.page.getByText('Message Type')).toBeVisible()
    await expect(builder.page.getByRole('button', { name: 'Text', exact: true })).toBeVisible()
    await expect(builder.page.getByRole('button', { name: 'Buttons', exact: true })).toBeVisible()
    await expect(builder.page.getByRole('button', { name: 'API', exact: true })).toBeVisible()
    await expect(builder.page.getByRole('button', { name: 'Transfer', exact: true })).toBeVisible()
  })

  test('should show message textarea for text type', async () => {
    await builder.selectMessageType('Text')
    await expect(builder.messageTextarea).toBeVisible()
  })
})

test.describe('Chatbot Flow Builder - Buttons Message Type', () => {
  let builder: ChatbotFlowBuilderPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    builder = new ChatbotFlowBuilderPage(page)
    await builder.gotoNew()
    await builder.addStep()
    await builder.selectStep(1)
    // Select "Buttons" message type
    await builder.selectMessageType('Buttons')
  })

  test('should show button options section in step properties when Buttons type selected', async () => {
    await expect(builder.buttonOptionsLabel).toBeVisible()
  })

  test('should show Reply, URL and Phone add-buttons', async () => {
    await expect(builder.addReplyButton).toBeVisible()
    await expect(builder.addUrlButton).toBeVisible()
    await expect(builder.addPhoneButton).toBeVisible()
  })

  test('should show message textarea alongside buttons config', async () => {
    await expect(builder.messageTextarea).toBeVisible()
    await expect(builder.buttonOptionsLabel).toBeVisible()
  })

  test('should show dynamic values hint without compilation errors', async () => {
    // This hint caused a vue-i18n "Not allowed nest placeholder" error
    // because of unescaped {{ }} in the translation string
    await expect(builder.page.getByText(/dynamic values/i)).toBeVisible()
  })

  test('should add a reply button', async () => {
    await builder.addReplyButton.click()
    await expect(builder.getButtonTitleInput(0)).toBeVisible()
    await expect(builder.buttonOptionsLabel).toContainText('1/10')
  })

  test('should add a URL button with /2 count', async () => {
    await builder.addUrlButton.click()
    await expect(builder.getButtonTitleInput(0)).toBeVisible()
    await expect(builder.page.getByPlaceholder(/https:\/\/example.com/i)).toBeVisible()
    await expect(builder.buttonOptionsLabel).toContainText('1/2')
  })

  test('should add a phone button', async () => {
    await builder.addPhoneButton.click()
    await expect(builder.getButtonTitleInput(0)).toBeVisible()
    await expect(builder.page.getByPlaceholder(/\+1234567890/)).toBeVisible()
    await expect(builder.buttonOptionsLabel).toContainText('1/2')
  })

  test('should add multiple reply buttons and show correct count', async () => {
    await builder.addReplyButton.click()
    await builder.addReplyButton.click()
    await builder.addReplyButton.click()
    await expect(builder.buttonOptionsLabel).toContainText('3/10')
  })

  test('should add multiple CTA buttons and show correct count', async () => {
    await builder.addUrlButton.click()
    await builder.addPhoneButton.click()
    await expect(builder.buttonOptionsLabel).toContainText('2/2')
  })

  test('should disable CTA buttons when reply buttons exist', async () => {
    await builder.addReplyButton.click()
    await expect(builder.addUrlButton).toBeDisabled()
    await expect(builder.addPhoneButton).toBeDisabled()
    // Reply button still enabled
    await expect(builder.addReplyButton).toBeEnabled()
  })

  test('should disable reply button when CTA buttons exist', async () => {
    await builder.addUrlButton.click()
    await expect(builder.addReplyButton).toBeDisabled()
    // Other CTA buttons still enabled
    await expect(builder.addPhoneButton).toBeEnabled()
  })

  test('should enforce max 2 CTA buttons', async () => {
    await builder.addUrlButton.click()
    await builder.addPhoneButton.click()
    await expect(builder.addUrlButton).toBeDisabled()
    await expect(builder.addPhoneButton).toBeDisabled()
  })

  test('should remove a button', async () => {
    await builder.addReplyButton.click()
    await expect(builder.buttonOptionsLabel).toContainText('1/10')
    await builder.getButtonDeleteButton(0).click()
    await expect(builder.buttonOptionsLabel).toContainText('0/10')
  })

  test('should show button preview in WhatsApp preview panel', async () => {
    await builder.addReplyButton.click()
    await builder.getButtonTitleInput(0).fill('Click Me')
    // The WhatsApp preview should show the button text
    await expect(builder.page.getByText('Click Me').last()).toBeVisible()
  })
})

test.describe('Chatbot Flow Builder - Switching Message Types', () => {
  let builder: ChatbotFlowBuilderPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    builder = new ChatbotFlowBuilderPage(page)
    await builder.gotoNew()
    await builder.addStep()
    await builder.selectStep(1)
  })

  test('should hide button options when switching from Buttons to Text', async () => {
    await builder.selectMessageType('Buttons')
    await expect(builder.buttonOptionsLabel).toBeVisible()

    await builder.selectMessageType('Text')
    await expect(builder.buttonOptionsLabel).not.toBeVisible()
  })

  test('should show button options when switching from Text to Buttons', async () => {
    await builder.selectMessageType('Text')
    await expect(builder.buttonOptionsLabel).not.toBeVisible()

    await builder.selectMessageType('Buttons')
    await expect(builder.buttonOptionsLabel).toBeVisible()
  })

  test('should show API config when switching to API type', async () => {
    await builder.selectMessageType('API')
    await expect(builder.page.getByText(/Method/i).first()).toBeVisible()
    await expect(builder.buttonOptionsLabel).not.toBeVisible()
  })

  test('should show transfer config when switching to Transfer type', async () => {
    await builder.selectMessageType('Transfer')
    await expect(builder.page.getByText(/Assign to Team/i)).toBeVisible()
    await expect(builder.buttonOptionsLabel).not.toBeVisible()
  })
})
