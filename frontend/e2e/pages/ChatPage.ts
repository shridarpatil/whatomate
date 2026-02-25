import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Chat Page - Main chat interface
 */
export class ChatPage extends BasePage {
  readonly contactList: Locator
  readonly searchInput: Locator
  readonly messageInput: Locator
  readonly sendButton: Locator
  readonly attachButton: Locator
  readonly emojiButton: Locator
  readonly cannedResponsesButton: Locator
  readonly contactInfoPanel: Locator
  readonly messageList: Locator
  readonly assignDialog: Locator
  readonly mediaDialog: Locator

  constructor(page: Page) {
    super(page)
    this.contactList = page.locator('.contacts-list, [data-testid="contacts"]').first()
    this.searchInput = page.locator('input[placeholder*="Search"]').first()
    this.messageInput = page.locator('textarea[placeholder*="message"], input[placeholder*="message"]')
    this.sendButton = page.getByRole('button').filter({ has: page.locator('.lucide-send') })
    this.attachButton = page.getByRole('button').filter({ has: page.locator('.lucide-paperclip') })
    this.emojiButton = page.getByRole('button').filter({ has: page.locator('.lucide-smile') })
    this.cannedResponsesButton = page.getByRole('button').filter({ has: page.locator('.lucide-message-square-text') })
    this.contactInfoPanel = page.locator('.contact-info, [data-testid="contact-info"]')
    this.messageList = page.locator('.messages-container, [data-testid="messages"]')
    this.assignDialog = page.locator('[role="dialog"][data-state="open"]')
    this.mediaDialog = page.locator('[role="dialog"][data-state="open"]').filter({ hasText: /Send|Upload|Media/i })
  }

  async goto(contactId?: string) {
    if (contactId) {
      await this.page.goto(`/chat/${contactId}`)
    } else {
      await this.page.goto('/chat')
    }
    await this.page.waitForLoadState('networkidle')
  }

  // Contact list helpers
  async searchContacts(term: string) {
    await this.searchInput.fill(term)
    await this.page.waitForTimeout(300)
  }

  async selectContact(name: string) {
    await this.page.locator('.contact-item, [data-testid="contact"]').filter({ hasText: name }).click()
    await this.page.waitForLoadState('networkidle')
  }

  getContactItem(name: string): Locator {
    return this.page.locator('.contact-item, [data-testid="contact"]').filter({ hasText: name })
  }

  // Message helpers
  async sendMessage(text: string) {
    await this.messageInput.fill(text)
    await this.sendButton.click()
  }

  async typeMessage(text: string) {
    await this.messageInput.fill(text)
  }

  getMessageBubble(text: string): Locator {
    return this.messageList.locator('.message-bubble, [data-testid="message"]').filter({ hasText: text })
  }

  async getLastMessage(): Promise<string> {
    const messages = this.messageList.locator('.message-bubble, [data-testid="message"]')
    const lastMessage = messages.last()
    return await lastMessage.textContent() || ''
  }

  // Attachment helpers
  async openAttachmentMenu() {
    await this.attachButton.click()
  }

  async sendImage(filePath: string) {
    const fileInput = this.page.locator('input[type="file"]')
    await fileInput.setInputFiles(filePath)
  }

  // Emoji helpers
  async openEmojiPicker() {
    await this.emojiButton.click()
  }

  async selectEmoji(emoji: string) {
    await this.page.locator(`button:has-text("${emoji}")`).click()
  }

  // Canned responses helpers
  async openCannedResponses() {
    await this.cannedResponsesButton.click()
  }

  async selectCannedResponse(shortcut: string) {
    await this.page.locator('[role="option"], .canned-item').filter({ hasText: shortcut }).click()
  }

  // Contact actions
  async openContactInfo() {
    await this.page.getByRole('button').filter({ has: this.page.locator('.lucide-info') }).click()
  }

  async assignContact() {
    await this.page.getByRole('button').filter({ has: this.page.locator('.lucide-user-plus') }).click()
    await this.assignDialog.waitFor({ state: 'visible' })
  }

  async transferToAgent() {
    await this.page.getByRole('button', { name: /Transfer/i }).click()
  }

  async resumeChatbot() {
    await this.page.getByRole('button', { name: /Resume/i }).click()
  }

  // Assign dialog helpers
  async selectAgentInAssignDialog(agentName: string) {
    await this.assignDialog.locator('button[role="combobox"]').click()
    await this.page.locator('[role="option"]').filter({ hasText: agentName }).click()
  }

  async confirmAssignment() {
    await this.assignDialog.getByRole('button', { name: /Assign|Save/i }).click()
  }

  async cancelAssignment() {
    await this.assignDialog.getByRole('button', { name: /Cancel/i }).click()
    await this.assignDialog.waitFor({ state: 'hidden' })
  }

  // Message reactions
  async reactToMessage(messageText: string, reaction: string) {
    const message = this.getMessageBubble(messageText)
    await message.hover()
    await message.locator('button').filter({ has: this.page.locator('.lucide-smile') }).click()
    await this.page.locator(`button:has-text("${reaction}")`).click()
  }

  // Reply to message
  async replyToMessage(messageText: string, replyText: string) {
    const message = this.getMessageBubble(messageText)
    await message.hover()
    await message.locator('button').filter({ has: this.page.locator('.lucide-reply') }).click()
    await this.sendMessage(replyText)
  }

  // Notes panel helpers
  get notesButton(): Locator {
    return this.page.locator('#notes-button')
  }

  get notesPanel(): Locator {
    return this.page.locator('#notes-panel')
  }

  get notesBadge(): Locator {
    return this.page.locator('#notes-badge')
  }

  get noteInput(): Locator {
    return this.notesPanel.locator('textarea').last()
  }

  async openNotesPanel() {
    await this.notesButton.click()
    await this.notesPanel.waitFor({ state: 'visible' })
  }

  async closeNotesPanel() {
    // The close button is the button with X in the panel header (first button in panel)
    const headerBtn = this.notesPanel.locator('button').first()
    await headerBtn.click()
    await this.notesPanel.waitFor({ state: 'hidden' })
  }

  async addNote(content: string) {
    await this.noteInput.fill(content)
    await this.noteInput.press('Enter')
  }

  getNoteCard(content: string): Locator {
    return this.notesPanel.locator('.group').filter({ hasText: content }).first()
  }

  async editNote(oldContent: string, newContent: string) {
    const noteCard = this.getNoteCard(oldContent)
    await noteCard.scrollIntoViewIfNeeded()
    await noteCard.hover()
    // Action buttons use group-hover opacity — force click to bypass visibility check
    const actionBtns = noteCard.locator('div.absolute button')
    await actionBtns.first().click({ force: true })
    // After clicking edit, the text moves from <p> to textarea value,
    // so the hasText-based noteCard locator becomes stale.
    // Find the edit textarea + Save button directly in the panel.
    const editCard = this.notesPanel.locator('.group').filter({ has: this.page.getByRole('button', { name: /Save/i }) })
    const editTextarea = editCard.locator('textarea')
    await editTextarea.waitFor({ state: 'visible', timeout: 5000 })
    await editTextarea.fill(newContent)
    await editCard.getByRole('button', { name: /Save/i }).click()
  }

  async deleteNote(content: string) {
    const noteCard = this.getNoteCard(content)
    await noteCard.scrollIntoViewIfNeeded()
    await noteCard.hover()
    // Action buttons use group-hover opacity — force click
    const actionBtns = noteCard.locator('div.absolute button')
    await actionBtns.last().click({ force: true })
  }

  // Account tabs helpers
  get accountTabsContainer(): Locator {
    return this.page.locator('[class*="flex-shrink-0"]').filter({ has: this.page.locator('button') }).filter({ hasText: /.+/ }).last()
  }

  getAccountTab(accountName: string): Locator {
    return this.page.locator('button.rounded-md.text-xs').filter({ hasText: accountName })
  }

  get accountTabs(): Locator {
    return this.page.locator('button.rounded-md.text-xs.font-medium')
  }

  get activeAccountTab(): Locator {
    return this.page.locator('button.rounded-md.text-xs[class*="bg-emerald"]')
  }

  get inactiveAccountTabs(): Locator {
    return this.page.locator('button.rounded-md.text-xs:not([class*="bg-emerald"])')
  }

  async switchAccount(accountName: string) {
    await this.getAccountTab(accountName).click()
    await this.page.waitForLoadState('networkidle')
  }

  // Custom actions
  async executeCustomAction(actionName: string) {
    await this.page.getByRole('button').filter({ has: this.page.locator('.lucide-zap') }).click()
    await this.page.locator('[role="menuitem"], .action-item').filter({ hasText: actionName }).click()
  }

  // Service window helpers
  get serviceWindowBanner(): Locator {
    return this.page.locator('text=24-hour messaging window has expired')
  }

  get serviceWindowSendTemplateButton(): Locator {
    return this.page.getByRole('button', { name: /Send Template/i })
  }

  async expectServiceWindowExpired() {
    await expect(this.serviceWindowBanner).toBeVisible({ timeout: 5000 })
    await expect(this.serviceWindowSendTemplateButton).toBeVisible()
  }

  async expectServiceWindowOpen() {
    await expect(this.serviceWindowBanner).not.toBeVisible()
  }

  // Template picker helpers
  get templatePickerButton(): Locator {
    return this.page.getByRole('button').filter({ has: this.page.locator('.lucide-layout-template-icon') })
  }

  get templatePopover(): Locator {
    return this.page.locator('[data-radix-popper-content-wrapper], [data-state="open"][role="dialog"]').filter({ has: this.page.locator('input[placeholder*="earch template"]') })
  }

  get templateSearchInput(): Locator {
    return this.page.locator('input[placeholder*="earch template"]')
  }

  get templateDialog(): Locator {
    return this.page.locator('[role="dialog"][data-state="open"]').filter({ hasText: /Preview|Fill Parameters/i })
  }

  get templateDialogSendButton(): Locator {
    return this.templateDialog.getByRole('button', { name: /Send/i })
  }

  get templateDialogCancelButton(): Locator {
    return this.templateDialog.getByRole('button', { name: /Cancel/i })
  }

  async openTemplatePicker() {
    await this.templatePickerButton.click()
    await this.templateSearchInput.waitFor({ state: 'visible' })
  }

  /** Wait for the loading spinner to disappear and at least one template item to appear */
  async waitForTemplatesLoaded() {
    // Wait for the loader to disappear (if it was shown)
    const loader = this.templatePopover.locator('.animate-spin')
    await loader.waitFor({ state: 'hidden', timeout: 15000 }).catch(() => {})
    // Wait for at least one template button to be visible
    await this.templatePopover.locator('button.w-full.text-left').first().waitFor({ state: 'visible', timeout: 15000 })
  }

  async searchTemplates(term: string) {
    await this.templateSearchInput.fill(term)
    // Wait for filtered results to settle
    await this.templatePopover.locator('button.w-full.text-left').first().waitFor({ state: 'visible', timeout: 10000 })
  }

  getTemplateItem(name: string): Locator {
    return this.page.locator('button.w-full.text-left').filter({ hasText: name })
  }

  async selectTemplate(name: string) {
    const item = this.getTemplateItem(name)
    await item.waitFor({ state: 'visible', timeout: 10000 })
    await item.click()
    await this.templateDialog.waitFor({ state: 'visible' })
  }

  async fillTemplateParam(paramName: string, value: string) {
    const paramField = this.templateDialog.locator('.space-y-1').filter({ hasText: paramName }).locator('input')
    await paramField.fill(value)
  }

  async sendTemplate() {
    await this.templateDialogSendButton.click()
  }

  async cancelTemplateDialog() {
    await this.templateDialogCancelButton.click()
    await this.templateDialog.waitFor({ state: 'hidden' })
  }

  getTemplatePreviewBubble(): Locator {
    return this.templateDialog.locator('.chat-bubble')
  }

  getTemplatePreviewButtons(): Locator {
    return this.templateDialog.locator('.interactive-buttons > div')
  }

  // Toast helpers
  async expectToast(text: string | RegExp) {
    const toast = this.page.locator('[data-sonner-toast]').filter({ hasText: text })
    await expect(toast).toBeVisible({ timeout: 5000 })
    return toast
  }

  // Assertions
  async expectPageVisible() {
    await expect(this.page.locator('body')).toBeVisible()
  }

  async expectContactListVisible() {
    await expect(this.contactList).toBeVisible()
  }

  async expectMessageInputVisible() {
    await expect(this.messageInput).toBeVisible()
  }

  async expectContactSelected(name: string) {
    await expect(this.page.getByText(name)).toBeVisible()
  }

  async expectMessageSent(text: string) {
    await expect(this.getMessageBubble(text)).toBeVisible()
  }

  async expectNoContactSelected() {
    await expect(this.page.getByText('Select a contact')).toBeVisible()
  }

  async expectEmptyContacts() {
    await expect(this.page.getByText('No contacts')).toBeVisible()
  }
}
