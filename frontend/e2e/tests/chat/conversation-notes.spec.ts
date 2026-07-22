import { test, expect, request as playwrightRequest } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { ApiHelper } from '../../helpers/api'
import { ChatPage } from '../../pages'
import { createTestScope, SUPER_ADMIN } from '../../framework'

const scope = createTestScope('conversation-notes')

// Helper to clean up all notes for a contact using both superadmin and test admin
// (creator-only delete means we need to try both users)
async function cleanupNotes(api: ApiHelper, contactId: string) {
  try {
    const notes = await api.listNotes(contactId)
    for (const note of notes) {
      try { await api.deleteNote(contactId, note.id) } catch { /* ignore */ }
    }
  } catch { /* ignore */ }
}

async function cleanupAllNotes(contactId: string) {
  // Try with superadmin first (for notes created via manual UI testing)
  const ctx1 = await playwrightRequest.newContext()
  const superApi = new ApiHelper(ctx1)
  await superApi.login(SUPER_ADMIN.email, SUPER_ADMIN.password)
  await cleanupNotes(superApi, contactId)
  await ctx1.dispose()
  // Then with test admin (for notes created by test user)
  const ctx2 = await playwrightRequest.newContext()
  const testApi = new ApiHelper(ctx2)
  await testApi.loginAsAdmin()
  await cleanupNotes(testApi, contactId)
  await ctx2.dispose()
}

test.describe('Conversation Notes - UI', () => {
  test.describe.configure({ mode: 'serial' }) // Tests share contact state
  test.setTimeout(60000)
  let chatPage: ChatPage
  let contactId: string

  test.beforeAll(async () => {
    const reqContext = await playwrightRequest.newContext()
    const api = new ApiHelper(reqContext)
    await api.loginAsAdmin()
    let contacts = await api.getContacts()
    if (contacts.length === 0) {
      await api.createContact(scope.phone(), scope.name('ui-contact'))
      contacts = await api.getContacts()
    }
    contactId = contacts[0].id
    await cleanupNotes(api, contactId)
    await reqContext.dispose()
  })

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatPage = new ChatPage(page)
    await chatPage.goto(contactId)
  })

  test('should show notes button when a contact is selected', async () => {
    await expect(chatPage.notesButton).toBeVisible()
  })

  test('should open and close notes panel', async () => {
    await chatPage.openNotesPanel()
    await expect(chatPage.notesPanel).toBeVisible()

    await chatPage.closeNotesPanel()
    await expect(chatPage.notesPanel).not.toBeVisible()
  })

  test('should show empty state when no notes exist', async ({ page }) => {
    await cleanupAllNotes(contactId)

    await chatPage.goto(contactId)
    await chatPage.openNotesPanel()
    await expect(page.getByText('No notes yet')).toBeVisible()
  })

  test('should create a note via Enter key', async () => {
    await chatPage.openNotesPanel()

    const noteContent = `E2E note ${Date.now()}`
    await chatPage.addNote(noteContent)

    await chatPage.expectToast('Note added')
    await expect(chatPage.getNoteCard(noteContent)).toBeVisible()
  })

  test('should edit own note', async () => {
    await cleanupAllNotes(contactId)
    const reqContext = await playwrightRequest.newContext()
    const api = new ApiHelper(reqContext)
    await api.loginAsAdmin()
    const note = await api.createNote(contactId, `Edit me ${Date.now()}`)
    await reqContext.dispose()

    await chatPage.goto(contactId)
    await chatPage.openNotesPanel()

    const updatedContent = `Updated ${Date.now()}`
    await chatPage.editNote(note.content, updatedContent)

    await chatPage.expectToast('Note updated')
    await expect(chatPage.getNoteCard(updatedContent)).toBeVisible()
  })

  test('should delete own note', async ({ page }) => {
    await cleanupAllNotes(contactId)
    const reqContext = await playwrightRequest.newContext()
    const api = new ApiHelper(reqContext)
    await api.loginAsAdmin()
    const note = await api.createNote(contactId, `Delete me ${Date.now()}`)
    await reqContext.dispose()

    await chatPage.goto(contactId)
    await chatPage.openNotesPanel()

    page.on('dialog', dialog => dialog.accept())
    await chatPage.deleteNote(note.content)

    await chatPage.expectToast('Note deleted')
    await expect(chatPage.getNoteCard(note.content)).not.toBeVisible()
  })

  test('should show badge count when panel is closed', async () => {
    await cleanupAllNotes(contactId)
    const reqContext = await playwrightRequest.newContext()
    const api = new ApiHelper(reqContext)
    await api.loginAsAdmin()
    await api.createNote(contactId, `Badge test ${Date.now()}`)
    await reqContext.dispose()

    await chatPage.goto(contactId)
    await chatPage.page.waitForTimeout(1000)

    await expect(chatPage.notesBadge).toBeVisible()
  })

  test('should not create empty note', async () => {
    await chatPage.openNotesPanel()

    await chatPage.noteInput.fill('   ')
    await chatPage.noteInput.press('Enter')

    const toast = chatPage.page.locator('[data-sonner-toast]').filter({ hasText: 'Note added' })
    await expect(toast).not.toBeVisible()
  })

  test('should persist notes across panel open/close', async () => {
    await cleanupAllNotes(contactId)
    const noteContent = `Persist ${Date.now()}`
    const reqContext = await playwrightRequest.newContext()
    const api = new ApiHelper(reqContext)
    await api.loginAsAdmin()
    await api.createNote(contactId, noteContent)
    await reqContext.dispose()

    await chatPage.goto(contactId)

    await chatPage.openNotesPanel()
    await expect(chatPage.getNoteCard(noteContent)).toBeVisible()

    await chatPage.closeNotesPanel()
    await chatPage.openNotesPanel()
    await expect(chatPage.getNoteCard(noteContent)).toBeVisible()
  })
})
