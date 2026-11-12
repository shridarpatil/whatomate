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
    this.contactList = page.locator('[data-testid="contacts"]')
    this.searchInput = page.locator('main').getByRole('searchbox', { name: 'Search contacts' })
    this.messageInput = page.getByRole('textbox', { name: 'Type a message' })
    this.sendButton = page.locator('form button[type="submit"]')
    this.attachButton = page.getByRole('button', { name: 'Attach file' })
    this.emojiButton = page.getByRole('button', { name: 'Emoji', exact: true })
    this.cannedResponsesButton = page.locator('#canned-response-picker-button')
    this.contactInfoPanel = page.locator('[data-testid="contact-info"]')
    this.messageList = page.locator('[data-testid="messages"]')
    this.assignDialog = page.locator('dialog[open]')
    this.mediaDialog = page.locator('dialog[open]').filter({ hasText: /Send|Upload|Media/i })
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
    await this.page.locator('[data-testid="contact"]').filter({ hasText: name }).click()
    await this.page.waitForLoadState('networkidle')
  }

  getContactItem(name: string): Locator {
    return this.page.locator('[data-testid="contact"]').filter({ hasText: name })
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
    return this.messageList.locator('[data-testid="message"]').filter({ hasText: text })
  }

  async getLastMessage(): Promise<string> {
    const messages = this.messageList.locator('[data-testid="message"]')
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
    await this.page.locator(`[popover]:popover-open button:has-text("${emoji}")`).click()
  }

  // Canned responses helpers
  async openCannedResponses() {
    await this.cannedResponsesButton.click()
  }

  async selectCannedResponse(shortcut: string) {
    await this.getCannedPickerItem(shortcut).click()
  }

  /** Picker items are [data-picker-item] buttons inside the picker popover. */
  getCannedPickerItem(name: string): Locator {
    return this.page.locator('[data-picker="canned-responses"] [data-picker-item]').filter({ hasText: name })
  }

  async selectCannedPickerItem(name: string) {
    await this.getCannedPickerItem(name).click()
  }

  // Canned response preview dialog
  get cannedDialog(): Locator {
    return this.page.locator('#canned-response-dialog')
  }

  get cannedDialogPreview(): Locator {
    return this.page.locator('#canned-response-preview')
  }

  get cannedDialogParamInputs(): Locator {
    return this.cannedDialog.locator('input[data-canned-param]')
  }

  cannedDialogParamInput(name: string): Locator {
    return this.page.locator(`#canned-response-param-${name}`)
  }

  get cannedDialogSendButton(): Locator {
    return this.page.locator('#canned-response-send')
  }

  get cannedDialogCancelButton(): Locator {
    return this.page.locator('#canned-response-cancel')
  }

  async fillCannedParam(name: string, value: string) {
    await this.cannedDialogParamInput(name).fill(value)
  }

  async sendCannedDialog() {
    await this.cannedDialogSendButton.click()
  }

  async cancelCannedDialog() {
    await this.cannedDialogCancelButton.click()
    await this.cannedDialog.waitFor({ state: 'hidden' })
  }

  // Contact actions
  get chatHeader(): Locator {
    return this.page.locator('[data-chat-header]')
  }

  async openContactInfo() {
    await this.page.locator('#info-button').click()
  }

  async assignContact() {
    await this.chatHeader.getByRole('button', { name: 'Assign to agent' }).click()
    await this.assignDialog.waitFor({ state: 'visible' })
  }

  /** Transfer lives in the header's "Contact Options" menu. */
  async transferToAgent() {
    await this.chatHeader.getByRole('button', { name: 'Contact Options' }).click()
    await this.page.getByRole('menuitem', { name: /Transfer/i }).click()
  }

  async resumeChatbot() {
    await this.page.getByRole('button', { name: /Resume/i }).click()
  }

  // Assign dialog helpers
  /** Toggling an agent's switch on assigns immediately and closes the dialog. */
  async selectAgentInAssignDialog(agentName: string) {
    await this.assignDialog.getByRole('switch', { name: agentName }).click()
  }

  async confirmAssignment() {
    await this.assignDialog.getByRole('button', { name: /Assign|Save/i }).click()
  }

  async cancelAssignment() {
    await this.assignDialog.getByRole('button', { name: 'Close' }).click()
    await this.assignDialog.waitFor({ state: 'hidden' })
  }

  // Message reactions
  async reactToMessage(messageText: string, reaction: string) {
    const message = this.getMessageBubble(messageText)
    await message.hover()
    await message.getByRole('button', { name: 'Add reaction' }).click()
    await this.page.locator(`[popover]:popover-open button:has-text("${reaction}")`).click()
  }

  // Reply to message
  async replyToMessage(messageText: string, replyText: string) {
    const message = this.getMessageBubble(messageText)
    await message.hover()
    await message.getByRole('button', { name: 'Reply', exact: true }).click()
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
    return this.notesPanel.locator('[data-note]').filter({ hasText: content }).first()
  }

  async editNote(oldContent: string, newContent: string) {
    const noteCard = this.getNoteCard(oldContent)
    await noteCard.scrollIntoViewIfNeeded()
    await noteCard.hover()
    await noteCard.getByRole('button', { name: 'Edit' }).click()
    // After clicking edit, the text moves from <p> to textarea value,
    // so the hasText-based noteCard locator becomes stale.
    // Find the edit textarea + Save button directly in the panel.
    const editCard = this.notesPanel.locator('[data-note]').filter({ has: this.page.getByRole('button', { name: /Save/i }) })
    const editTextarea = editCard.locator('textarea')
    await editTextarea.waitFor({ state: 'visible', timeout: 5000 })
    await editTextarea.fill(newContent)
    await editCard.getByRole('button', { name: /Save/i }).click()
  }

  async deleteNote(content: string) {
    const noteCard = this.getNoteCard(content)
    await noteCard.scrollIntoViewIfNeeded()
    await noteCard.hover()
    await noteCard.getByRole('button', { name: 'Delete' }).click()
  }

  // Account tabs helpers
  get accountTabsContainer(): Locator {
    return this.page.getByRole('tablist', { name: 'WhatsApp Accounts' })
  }

  getAccountTab(accountName: string): Locator {
    return this.accountTabsContainer.getByRole('tab', { name: accountName })
  }

  get accountTabs(): Locator {
    return this.accountTabsContainer.getByRole('tab')
  }

  get activeAccountTab(): Locator {
    return this.accountTabsContainer.locator('[role="tab"][aria-selected="true"]')
  }

  get inactiveAccountTabs(): Locator {
    return this.accountTabsContainer.locator('[role="tab"][aria-selected="false"]')
  }

  async switchAccount(accountName: string) {
    await this.getAccountTab(accountName).click()
    await this.page.waitForLoadState('networkidle')
  }

  // Custom actions
  /** Custom actions are icon buttons in the chat header, named by the action. */
  async executeCustomAction(actionName: string) {
    await this.chatHeader.getByRole('button', { name: actionName }).click()
  }

  // Service window helpers
  get serviceWindowBanner(): Locator {
    return this.page.locator('text=24-hour messaging window has expired')
  }

  get serviceWindowSendTemplateButton(): Locator {
    // Scoped to the banner: the composer's template picker is also named "Send Template".
    return this.page.getByRole('alert').filter({ hasText: '24-hour messaging window has expired' }).getByRole('button', { name: /Send Template/i })
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
    return this.page.getByRole('button').filter({ has: this.page.locator('[data-icon="layout-template"]') })
  }

  get templatePopover(): Locator {
    return this.page.locator('[data-picker="templates"]')
  }

  get templateSearchInput(): Locator {
    return this.page.locator('input[placeholder*="earch template"]')
  }

  get templateDialog(): Locator {
    return this.page.locator('dialog[open]').filter({ hasText: /Preview|Fill Parameters/i })
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
    const loader = this.templatePopover.locator('[aria-busy="true"]')
    await loader.waitFor({ state: 'hidden', timeout: 15000 }).catch(() => {})
    // Wait for at least one template button to be visible
    await this.templatePopover.locator('[data-picker-item]').first().waitFor({ state: 'visible', timeout: 15000 })
  }

  async searchTemplates(term: string) {
    await this.templateSearchInput.fill(term)
    // Wait for filtered results to settle
    await this.templatePopover.locator('[data-picker-item]').first().waitFor({ state: 'visible', timeout: 10000 })
  }

  getTemplateItem(name: string): Locator {
    return this.templatePopover.locator('[data-picker-item]').filter({ hasText: name })
  }

  async selectTemplate(name: string) {
    const item = this.getTemplateItem(name)
    await item.waitFor({ state: 'visible', timeout: 10000 })
    await item.click()
    await this.templateDialog.waitFor({ state: 'visible' })
  }

  async fillTemplateParam(paramName: string, value: string) {
    await this.templateDialog.locator(`[data-param="${paramName}"] input`).fill(value)
  }

  async sendTemplate() {
    await this.templateDialogSendButton.click()
  }

  async cancelTemplateDialog() {
    await this.templateDialogCancelButton.click()
    await this.templateDialog.waitFor({ state: 'hidden' })
  }

  getTemplatePreviewBubble(): Locator {
    return this.templateDialog.locator('[data-bubble]')
  }

  getTemplatePreviewButtons(): Locator {
    return this.templateDialog.locator('[data-interactive-buttons] > *')
  }

  // Toast helpers
  async expectToast(text: string | RegExp) {
    const toast = this.page.locator('.toast-container output').filter({ hasText: text })
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
