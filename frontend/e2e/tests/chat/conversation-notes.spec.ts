import { test, expect, request as playwrightRequest } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { ApiHelper } from '../../helpers/api'
import { ChatPage } from '../../pages'

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
  await superApi.login('admin@admin.com', 'admin')
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
      await api.createContact(`91${Date.now().toString().slice(-10)}`, 'Notes UI Test')
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

test.describe('Conversation Notes - API CRUD', () => {
  let api: ApiHelper
  let contactId: string

  test.beforeAll(async () => {
    const reqContext = await playwrightRequest.newContext()
    api = new ApiHelper(reqContext)
    await api.loginAsAdmin()
    let contacts = await api.getContacts()
    if (contacts.length < 2) {
      await api.createContact(`91${(Date.now() + 1).toString().slice(-10)}`, 'Notes API Test')
      contacts = await api.getContacts()
    }
    // Use a different contact than UI tests to avoid parallel conflicts
    contactId = contacts.length > 1 ? contacts[1].id : contacts[0].id
    await cleanupNotes(api, contactId)
  })

  test('should create a note via API', async () => {
    const note = await api.createNote(contactId, 'API test note')
    expect(note.id).toBeTruthy()
    expect(note.content).toBe('API test note')
    expect(note.contact_id).toBe(contactId)
    expect(note.created_by_name).toBeTruthy()
    expect(note.created_at).toBeTruthy()
  })

  test('should list notes in chronological order', async () => {
    await api.createNote(contactId, 'List test 1')
    await api.createNote(contactId, 'List test 2')

    const notes = await api.listNotes(contactId)
    expect(notes.length).toBeGreaterThanOrEqual(2)

    const listTest1 = notes.find((n: any) => n.content === 'List test 1')
    const listTest2 = notes.find((n: any) => n.content === 'List test 2')
    expect(listTest1).toBeTruthy()
    expect(listTest2).toBeTruthy()
    expect(notes.indexOf(listTest1)).toBeLessThan(notes.indexOf(listTest2))
  })

  test('should update a note via API', async () => {
    const note = await api.createNote(contactId, 'Update me via API')
    const updated = await api.updateNote(contactId, note.id, 'Updated via API')
    expect(updated.content).toBe('Updated via API')
    expect(updated.id).toBe(note.id)
  })

  test('should delete a note via API', async () => {
    const note = await api.createNote(contactId, 'Delete me via API')
    await api.deleteNote(contactId, note.id)

    const notes = await api.listNotes(contactId)
    const found = notes.find((n: any) => n.id === note.id)
    expect(found).toBeUndefined()
  })

  test('should support pagination with limit and has_more', async () => {
    await cleanupNotes(api, contactId)

    for (let i = 1; i <= 5; i++) {
      await api.createNote(contactId, `Paginate note ${i}`)
    }

    // Verify at least 5 exist (other parallel tests may add notes too)
    const allNotes = await api.listNotes(contactId)
    expect(allNotes.length).toBeGreaterThanOrEqual(5)

    // Use the same api instance to test pagination via raw request
    const BASE_URL = process.env.BASE_URL || 'http://localhost:8080'
    const reqContext = await playwrightRequest.newContext()
    const paginationApi = new ApiHelper(reqContext)
    await paginationApi.loginAsAdmin()

    const response = await reqContext.get(`${BASE_URL}/api/contacts/${contactId}/notes?limit=3`, {
      headers: { Authorization: `Bearer ${paginationApi.getToken()}` }
    })
    const data = await response.json()
    expect(data.data.notes.length).toBe(3)
    expect(data.data.has_more).toBe(true)

    await reqContext.dispose()
  })
})
